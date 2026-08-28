package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
)

type resultStatus string

const (
	statusPassed  resultStatus = "passed"
	statusFailed  resultStatus = "failed"
	statusSkipped resultStatus = "skipped"
	statusFlaky   resultStatus = "flaky"
)

type attemptResult struct {
	Attempt  int
	Status   resultStatus
	Duration time.Duration
	Message  string
	LogPath  string
	Checks   []scenarioCheck
}

type scenarioResult struct {
	Name     string
	Status   resultStatus
	Attempts []attemptResult
}

type runSummary struct {
	Total   int
	Passed  int
	Failed  int
	Skipped int
	Flaky   int
}

type executor struct {
	ctx         context.Context
	stdout      io.Writer
	opts        runOptions
	stream      bool
	sem         chan struct{}
	consoleMu   sync.Mutex
	resultsMu   sync.Mutex
	results     []scenarioResult
	scheduled   []string
	finalized   bool
	scenarios   sync.WaitGroup
	runScenario func(context.Context, string, toolkit.Logger, *Scenario) error
}

func newExecutor(ctx context.Context, stdout io.Writer, opts runOptions, runnable int) *executor {
	return &executor{
		ctx:         ctx,
		stdout:      stdout,
		opts:        opts,
		stream:      opts.outputMode == "stream" || (opts.outputMode == "auto" && runnable <= 3),
		sem:         make(chan struct{}, opts.parallel),
		runScenario: runScenarioFlow,
	}
}

func (e *executor) schedule(name string, original *Scenario) {
	e.resultsMu.Lock()
	e.scheduled = append(e.scheduled, name)
	e.resultsMu.Unlock()
	e.scenarios.Add(1)
	go func() {
		defer e.scenarios.Done()
		e.execute(name, original)
	}()
}

func (e *executor) wait(gracePeriod time.Duration) error {
	done := make(chan struct{})
	go func() {
		e.scenarios.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-e.ctx.Done():
	}

	timer := time.NewTimer(gracePeriod)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		err := fmt.Errorf("scenarios did not stop within %s after suite cancellation: %w", gracePeriod, e.ctx.Err())
		e.failUnfinished(err)
		return err
	}
}

func (e *executor) failUnfinished(err error) {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()
	e.finalized = true

	finished := make(map[string]struct{}, len(e.results))
	for _, result := range e.results {
		finished[result.Name] = struct{}{}
	}
	for _, name := range e.scheduled {
		if _, ok := finished[name]; ok {
			continue
		}
		e.results = append(e.results, scenarioResult{
			Name:     name,
			Status:   statusFailed,
			Attempts: []attemptResult{{Attempt: 1, Status: statusFailed, Message: err.Error()}},
		})
	}
}

func (e *executor) execute(name string, original *Scenario) {
	result := scenarioResult{Name: name}
	hadFailure := false
attempts:
	for attempt := 1; attempt <= e.opts.retries+1; attempt++ {
		if err := e.acquire(); err != nil {
			result.Status = statusFailed
			result.Attempts = append(result.Attempts, attemptResult{Attempt: attempt, Status: statusFailed, Message: err.Error()})
			break
		}
		attemptResult := e.executeAttempt(name, attempt, original)
		e.release()
		result.Attempts = append(result.Attempts, attemptResult)
		switch attemptResult.Status {
		case statusPassed:
			if hadFailure {
				result.Status = statusFlaky
			} else {
				result.Status = statusPassed
			}
			e.addResult(result)
			return
		case statusSkipped:
			if hadFailure {
				result.Status = statusFailed
				e.addResult(result)
				return
			}
			result.Status = statusSkipped
			e.addResult(result)
			return
		case statusFailed:
			hadFailure = true
			if e.ctx.Err() != nil {
				break attempts
			}
		}
	}
	if result.Status == "" {
		result.Status = statusFailed
	}
	e.addResult(result)
}

func (e *executor) executeAttempt(name string, attempt int, original *Scenario) (result attemptResult) {
	started := time.Now()
	attemptCtx, cancel := context.WithTimeout(e.ctx, config.Config.TestTimeout)
	defer cancel()
	logPath := filepath.Join(e.opts.logDir, name, fmt.Sprintf("attempt-%d.log", attempt))
	logger, err := newScenarioLogger(e, name, logPath)
	if err != nil {
		return attemptResult{Attempt: attempt, Status: statusFailed, Duration: time.Since(started), Message: err.Error(), LogPath: logPath}
	}
	result = attemptResult{Attempt: attempt, LogPath: logPath}
	var scenario *Scenario
	var runErr error
	// Every stage in an attempt shares one cleanup stack.
	cleanup := &scenarioCleanup{}
	defer func() {
		if recovered := recover(); recovered != nil {
			runErr = fmt.Errorf("panic: %v\n%s", recovered, debug.Stack())
		}
		runErr = addFailure(runErr, runScenarioCleanup(e.ctx, cleanup))
		logErr := logger.Err()
		runErr = addFailure(runErr, logErr)
		switch status, message := classifyAttempt(runErr); status {
		case statusSkipped:
			logger.Log("SKIP:", message)
		case statusFailed:
			logger.Log("🔴 FAIL:", message)
		}
		if closeErr := logger.Close(); logErr == nil {
			logErr = closeErr
			runErr = addFailure(runErr, closeErr)
		}
		result.Status, result.Message = classifyAttempt(runErr)
		result.Duration = time.Since(started)
		if scenario != nil {
			result.Checks = append([]scenarioCheck(nil), scenario.checks...)
		}
		if result.Status != statusSkipped {
			logger.FlushConsole(string(result.Status))
		}
		if logErr != nil && !e.stream {
			logger.PrintConsoleFailure(result.Message)
		}
	}()

	scenario = freshScenario(original)
	scenario.cleanup = cleanup
	if scenario.SkipIf != nil {
		if message := scenario.SkipIf(attemptCtx); message != "" {
			runErr = &skipError{message: message}
			return result
		}
	}
	runErr = e.runScenario(attemptCtx, name, logger, scenario)
	return result
}

func classifyAttempt(err error) (resultStatus, string) {
	if err == nil {
		return statusPassed, ""
	}
	var skip *skipError
	if errors.As(err, &skip) {
		return statusSkipped, skip.Error()
	}
	return statusFailed, err.Error()
}

func (e *executor) acquire() error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	select {
	case e.sem <- struct{}{}:
		return nil
	case <-e.ctx.Done():
		return e.ctx.Err()
	}
}

func (e *executor) release() {
	<-e.sem
}

func (e *executor) addResult(result scenarioResult) {
	e.resultsMu.Lock()
	if e.finalized {
		e.resultsMu.Unlock()
		return
	}
	e.results = append(e.results, result)
	e.resultsMu.Unlock()
}

func (e *executor) summary() runSummary {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()
	summary := runSummary{Total: len(e.results)}
	for _, result := range e.results {
		switch result.Status {
		case statusPassed:
			summary.Passed++
		case statusFailed:
			summary.Failed++
		case statusSkipped:
			summary.Skipped++
		case statusFlaky:
			summary.Flaky++
		}
	}
	return summary
}

func (e *executor) printSummary() runSummary {
	e.resultsMu.Lock()
	summary := runSummary{Total: len(e.results)}
	var skipped, flaky, failed []scenarioResult
	for _, result := range e.results {
		switch result.Status {
		case statusPassed:
			summary.Passed++
		case statusSkipped:
			summary.Skipped++
			skipped = append(skipped, result)
		case statusFlaky:
			summary.Flaky++
			flaky = append(flaky, result)
		case statusFailed:
			summary.Failed++
			failed = append(failed, result)
		}
	}
	e.resultsMu.Unlock()
	for _, results := range [][]scenarioResult{skipped, flaky, failed} {
		sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	}

	e.consoleMu.Lock()
	defer e.consoleMu.Unlock()
	_, _ = fmt.Fprintf(e.stdout, "\nDONE %d scenarios: %d passed, %d flaky, %d skipped, %d failed\n",
		summary.Total, summary.Passed, summary.Flaky, summary.Skipped, summary.Failed)
	if len(skipped) > 0 {
		_, _ = fmt.Fprintln(e.stdout, "\nSkipped:")
	}
	for _, result := range skipped {
		last := result.Attempts[len(result.Attempts)-1]
		_, _ = fmt.Fprintf(e.stdout, "- %s: %s\n", result.Name, summaryMessage(last.Message))
	}
	if len(flaky) > 0 {
		_, _ = fmt.Fprintln(e.stdout, "\nFlaky:")
	}
	for _, result := range flaky {
		failedAttempt := lastAttemptWithStatus(result, statusFailed)
		_, _ = fmt.Fprintf(e.stdout, "- %s (passed on attempt %d): %s\n",
			result.Name, len(result.Attempts), summaryMessage(failedAttempt.Message))
	}
	if len(failed) > 0 {
		_, _ = fmt.Fprintln(e.stdout, "\nFailed:")
	}
	for _, result := range failed {
		attempt := lastAttemptWithStatus(result, statusFailed)
		_, _ = fmt.Fprintf(e.stdout, "- %s: %s\n", result.Name, summaryMessage(attempt.Message))
	}
	return summary
}

func lastAttemptWithStatus(result scenarioResult, status resultStatus) attemptResult {
	for i := len(result.Attempts) - 1; i >= 0; i-- {
		if result.Attempts[i].Status == status {
			return result.Attempts[i]
		}
	}
	return result.Attempts[len(result.Attempts)-1]
}

func summaryMessage(message string) string {
	const maxLength = 500
	message = strings.TrimSpace(message)
	if line, _, found := strings.Cut(message, "\n"); found {
		message = strings.TrimSuffix(line, "\r")
	}
	if len(message) > maxLength {
		message = message[:maxLength] + "..."
	}
	return message
}

type scenarioLogger struct {
	executor *executor
	name     string
	started  time.Time
	path     string
	file     *os.File
	err      error
	mu       sync.Mutex
}

func newScenarioLogger(executor *executor, name, path string) (*scenarioLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create scenario log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open scenario log: %w", err)
	}
	return &scenarioLogger{executor: executor, name: name, started: time.Now(), path: path, file: file}, nil
}

func (l *scenarioLogger) Log(args ...any) {
	l.write(strings.TrimSuffix(fmt.Sprintln(args...), "\n"))
}

func (l *scenarioLogger) Logf(format string, args ...any) {
	l.write(fmt.Sprintf(format, args...))
}

func (l *scenarioLogger) write(message string) {
	prefix := fmt.Sprintf("[%.3fs] ", time.Since(l.started).Seconds())
	var output strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(message, "\n"), "\n") {
		output.WriteString(prefix)
		output.WriteString(line)
		output.WriteByte('\n')
	}
	formatted := output.String()
	l.mu.Lock()
	if l.file != nil {
		if _, err := l.file.WriteString(formatted); err != nil {
			l.recordErr(fmt.Errorf("write scenario log %s: %w", l.path, err))
		}
	}
	l.mu.Unlock()
	if l.executor.stream {
		l.executor.consoleMu.Lock()
		for _, line := range strings.Split(strings.TrimSuffix(formatted, "\n"), "\n") {
			_, _ = fmt.Fprintf(l.executor.stdout, "[%s] %s\n", l.name, line)
		}
		l.executor.consoleMu.Unlock()
	}
}

func (l *scenarioLogger) FlushConsole(label string) {
	if l.executor.stream || (label == string(statusPassed) && l.executor.opts.hidePassed) {
		return
	}
	l.mu.Lock()
	if l.file != nil {
		if err := l.file.Sync(); err != nil {
			l.recordErr(fmt.Errorf("sync scenario log %s: %w", l.path, err))
		}
	}
	l.mu.Unlock()
	output, err := os.ReadFile(l.path)
	if err != nil || len(output) == 0 {
		return
	}
	l.executor.consoleMu.Lock()
	defer l.executor.consoleMu.Unlock()
	if l.executor.opts.outputMode == "grouped" {
		_, _ = fmt.Fprintf(l.executor.stdout, "##[group]%s (%s)\n%s##[endgroup]\n", l.name, label, output)
		return
	}
	_, _ = fmt.Fprintf(l.executor.stdout, "=== %s (%s) ===\n%s", l.name, label, output)
}

func (l *scenarioLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		if err := l.file.Sync(); err != nil {
			l.recordErr(fmt.Errorf("sync scenario log %s: %w", l.path, err))
		}
		if err := l.file.Close(); err != nil {
			l.recordErr(fmt.Errorf("close scenario log %s: %w", l.path, err))
		}
		l.file = nil
	}
	return l.err
}

func (l *scenarioLogger) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

func (l *scenarioLogger) PrintConsoleFailure(message string) {
	l.executor.consoleMu.Lock()
	defer l.executor.consoleMu.Unlock()
	_, _ = fmt.Fprintf(l.executor.stdout, "[%s] [%.3fs] 🔴 FAIL: %s\n", l.name, time.Since(l.started).Seconds(), message)
}

func (l *scenarioLogger) recordErr(err error) {
	if l.err == nil {
		l.err = err
	}
}
