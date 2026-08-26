package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
			result.Status = statusSkipped
			e.addResult(result, !isFilteredSkip(attemptResult.Message))
			return
		case statusFailed:
			hadFailure = true
			if e.ctx.Err() != nil {
				break
			}
		}
	}
	if result.Status == "" {
		result.Status = statusFailed
	}
	e.addResult(result, true)
}

func (e *executor) executeAttempt(name string, attempt int, original *Scenario) attemptResult {
	started := time.Now()
	logPath := filepath.Join(e.opts.logDir, name, fmt.Sprintf("attempt-%d.log", attempt))
	logger, err := newScenarioLogger(e, name, logPath)
	if err != nil {
		return attemptResult{Attempt: attempt, Status: statusFailed, Duration: time.Since(started), Message: err.Error(), LogPath: logPath}
	}
	defer logger.Close()

	s := freshScenario(original)
	status := statusPassed
	message := ""
	if s.SkipIf != nil {
		message = s.SkipIf(e.ctx)
	}
	if message != "" {
		status = statusSkipped
		logger.Log("SKIP:", message)
	} else {
		err = e.runScenario(e.ctx, name, logger, s)
		var skip *skipError
		if errors.As(err, &skip) {
			status = statusSkipped
			message = skip.Error()
		} else if err != nil {
			status = statusFailed
			message = err.Error()
			logger.Log("🔴 FAIL:", err)
		}
	}
	duration := time.Since(started)
	logger.FlushConsole(string(status))
	return attemptResult{
		Attempt:  attempt,
		Status:   status,
		Duration: duration,
		Message:  message,
		LogPath:  logPath,
		Checks:   append([]scenarioCheck(nil), s.checks...),
	}
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
	file     *os.File
	buffer   bytes.Buffer
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
	return &scenarioLogger{executor: executor, name: name, started: time.Now(), file: file}, nil
}

func (l *scenarioLogger) Log(args ...any) {
	l.write(strings.TrimSuffix(fmt.Sprintln(args...), "\n"))
}

func (l *scenarioLogger) Logf(format string, args ...any) {
	l.write(fmt.Sprintf(format, args...))
}

func (l *scenarioLogger) write(message string) {
	line := fmt.Sprintf("[%.3fs] %s\n", time.Since(l.started).Seconds(), message)
	l.mu.Lock()
	if l.file != nil {
		_, _ = l.file.WriteString(line)
	}
	l.buffer.WriteString(line)
	l.mu.Unlock()
	if l.executor.stream {
		l.executor.consoleMu.Lock()
		_, _ = fmt.Fprintf(l.executor.stdout, "[%s] %s", l.name, line)
		l.executor.consoleMu.Unlock()
	}
}

func (l *scenarioLogger) FlushConsole(label string) {
	if l.executor.stream || (label == string(statusPassed) && l.executor.opts.hidePassed) {
		return
	}
	l.mu.Lock()
	output := l.buffer.String()
	l.mu.Unlock()
	if output == "" {
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
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func isFilteredSkip(message string) bool {
	return strings.HasPrefix(message, "filtered:")
}
