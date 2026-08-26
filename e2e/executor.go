package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

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
	Total    int
	Selected int
	Passed   int
	Failed   int
	Skipped  int
	Flaky    int
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
	selected    int
	scenarios   sync.WaitGroup
	runScenario func(context.Context, string, toolkit.Logger, *Scenario) error
}

func newExecutor(ctx context.Context, stdout io.Writer, opts runOptions, selectedEntries int) *executor {
	return &executor{
		ctx:         ctx,
		stdout:      stdout,
		opts:        opts,
		stream:      opts.outputMode == "stream" || (opts.outputMode == "auto" && selectedEntries <= 3),
		sem:         make(chan struct{}, opts.parallel),
		runScenario: runScenarioFlow,
	}
}

func (e *executor) schedule(name string, original *Scenario) {
	e.scenarios.Add(1)
	go func() {
		defer e.scenarios.Done()
		e.execute(name, original)
	}()
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
			e.addResult(result, true)
			return
		case statusSkipped:
			if hadFailure {
				result.Status = statusFailed
				e.addResult(result, true)
				return
			}
			result.Status = statusSkipped
			e.addResult(result, !isFilteredSkip(attemptResult.Message))
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
	e.addResult(result, true)
}

func (e *executor) executeAttempt(name string, attempt int, original *Scenario) (result attemptResult) {
	started := time.Now()
	logPath := filepath.Join(e.opts.logDir, name, fmt.Sprintf("attempt-%d.log", attempt))
	logger, err := newScenarioLogger(e, name, logPath)
	if err != nil {
		return attemptResult{Attempt: attempt, Status: statusFailed, Duration: time.Since(started), Message: err.Error(), LogPath: logPath}
	}
	defer logger.Close()
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
		switch status, message := classifyAttempt(runErr); status {
		case statusSkipped:
			logger.Log("SKIP:", message)
		case statusFailed:
			logger.Log("🔴 FAIL:", message)
		}
		runErr = addFailure(runErr, logger.Close())
		result.Status, result.Message = classifyAttempt(runErr)
		result.Duration = time.Since(started)
		if scenario != nil {
			result.Checks = append([]scenarioCheck(nil), scenario.checks...)
		}
		logger.FlushConsole(string(result.Status))
	}()

	scenario = freshScenario(original)
	scenario.cleanup = cleanup
	if runErr = filterScenario(name, scenario); runErr != nil {
		return result
	}
	if scenario.SkipIf != nil {
		if message := scenario.SkipIf(e.ctx); message != "" {
			runErr = &skipError{message: message}
			return result
		}
	}
	runErr = e.runScenario(e.ctx, name, logger, scenario)
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

func (e *executor) addResult(result scenarioResult, selected bool) {
	e.resultsMu.Lock()
	e.results = append(e.results, result)
	if selected {
		e.selected++
	}
	e.resultsMu.Unlock()
}

func (e *executor) summary() runSummary {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()
	summary := runSummary{Total: len(e.results), Selected: e.selected}
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

func (e *executor) printSummary() {
	summary := e.summary()
	e.consoleMu.Lock()
	defer e.consoleMu.Unlock()
	_, _ = fmt.Fprintf(e.stdout, "\nDONE %d scenarios: %d passed, %d flaky, %d skipped, %d failed\n",
		summary.Total, summary.Passed, summary.Flaky, summary.Skipped, summary.Failed)
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

func (l *scenarioLogger) recordErr(err error) {
	if l.err == nil {
		l.err = err
	}
}

func isFilteredSkip(message string) bool {
	return strings.HasPrefix(message, "filtered:")
}
