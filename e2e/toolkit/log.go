package toolkit

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"
)

// Logger is the minimal logging capability used by e2e implementation code.
//
// It deliberately offers logging only. It has no test-control methods (Fatal,
// Error, Skip, Failed, Name, Cleanup) and does not embed testing.TB, so
// implementation code cannot change the outcome of a test through a Logger.
// testing.T stays at the test entry points and the runner adapter.
type Logger interface {
	Log(args ...any)
	Logf(format string, args ...any)
}

// tbBacked is implemented by loggers that write to a testing.TB. It is
// unexported on purpose: only this package can reach the testing.TB behind a
// Logger, and it does so for two reasons only:
//   - marking a frame as a test helper so log lines keep pointing at the caller,
//   - reading Failed() to pick the step status marker in LogStep.
type tbBacked interface {
	testingTB() testing.TB
}

// tbOf returns the testing.TB backing logger, or nil when the logger is not
// backed by one. It returns the testing.TB instead of marking the frame itself
// because testing.TB.Helper marks the frame that calls it, so callers have to
// invoke Helper from their own frame.
func tbOf(logger Logger) testing.TB {
	if backed, ok := logger.(tbBacked); ok {
		return backed.testingTB()
	}
	return nil
}

// loggerKey is a private key so the logger can only be read through
// LoggerFromContext.
type loggerKey struct{}

// ContextWithLogger returns a context carrying logger. Implementation code
// reads it back with LoggerFromContext, Log, Logf or the LogStep helpers.
func ContextWithLogger(ctx context.Context, logger Logger) context.Context {
	if logger == nil {
		log.Println("WARNING: no logger provided, falling back to the standard logger")
		return ctx
	}
	return context.WithValue(ctx, loggerKey{}, logger)
}

// LoggerFromContext returns the Logger stored in ctx. When ctx carries no
// logger it returns a logger backed by the standard library, so logging still
// works outside a test.
func LoggerFromContext(ctx context.Context) Logger {
	if logger, ok := ctx.Value(loggerKey{}).(Logger); ok && logger != nil {
		return logger
	}
	return stdLogger{}
}

func Logf(ctx context.Context, format string, args ...any) {
	logger := LoggerFromContext(ctx)
	if tb := tbOf(logger); tb != nil {
		tb.Helper()
	}
	logger.Logf(format, args...)
}

func Log(ctx context.Context, args ...any) {
	logger := LoggerFromContext(ctx)
	if tb := tbOf(logger); tb != nil {
		tb.Helper()
	}
	logger.Log(args...)
}

// stdLogger writes to the standard library logger. It is the fallback used when
// no logger is attached to the context.
type stdLogger struct{}

func (stdLogger) Log(args ...any) {
	log.Println(args...)
}

func (stdLogger) Logf(format string, args ...any) {
	log.Printf(format, args...)
}

// testLogger writes to a testing.TB and prefixes every line with the time
// elapsed since the logger was created. It holds the testing.TB in an
// unexported field rather than embedding it, so none of the test-control
// methods leak to callers.
type testLogger struct {
	tb    testing.TB
	start time.Time
}

// NewTestLogger returns a Logger that writes to tb. Call it at the test runner
// boundary and pass the returned Logger down to implementation code.
func NewTestLogger(tb testing.TB) Logger {
	return &testLogger{tb: tb, start: time.Now()}
}

func (l *testLogger) testingTB() testing.TB {
	return l.tb
}

func (l *testLogger) elapsed() string {
	return fmt.Sprintf("[%.3fs]", time.Since(l.start).Seconds())
}

func (l *testLogger) Log(args ...any) {
	l.tb.Helper()
	args = append([]any{l.elapsed()}, args...)
	l.tb.Log(args...)
}

func (l *testLogger) Logf(format string, args ...any) {
	l.tb.Helper()
	l.tb.Logf(l.elapsed()+" "+format, args...)
}

// failureFormatter decorates the failure reporting of a testing.TB with the
// elapsed time and a visible marker. Reporting failures is test control, not
// logging, so this stays a testing.TB and belongs to the runner boundary.
type failureFormatter struct {
	testing.TB
	start time.Time
}

// WithFailureFormatting wraps t so reported failures carry the elapsed time and
// a visible marker. Use it only at the test runner boundary.
func WithFailureFormatting(t testing.TB) testing.TB {
	return &failureFormatter{TB: t, start: time.Now()}
}

func (t *failureFormatter) elapsed() string {
	return fmt.Sprintf("[%.3fs]", time.Since(t.start).Seconds())
}

// formatError formats the ERROR prefix with emoji
func (t *failureFormatter) formatError() string {
	return "🔴 FAIL:"
}

func (t *failureFormatter) Fatal(args ...any) {
	t.Helper()
	args = append([]any{t.elapsed(), t.formatError()}, args...)
	t.TB.Fatal(args...)
}

func (t *failureFormatter) Fatalf(format string, args ...any) {
	t.Helper()
	t.TB.Fatalf(t.elapsed()+" "+t.formatError()+" "+format, args...)
}

func (t *failureFormatter) Error(args ...any) {
	t.Helper()
	args = append([]any{t.elapsed(), t.formatError()}, args...)
	t.TB.Error(args...)
}

func (t *failureFormatter) Errorf(format string, args ...any) {
	t.Helper()
	t.TB.Errorf(t.elapsed()+" "+t.formatError()+" "+format, args...)
}

func (t *failureFormatter) FailNow() {
	t.Helper()
	t.TB.Log(t.elapsed(), t.formatError())
	t.TB.FailNow()
}

func (t *failureFormatter) Fail() {
	t.Helper()
	t.TB.Log(t.elapsed(), t.formatError())
	t.TB.Fail()
}

// LogStep logs "→ msg..." at the start and "✓ msg done (Xs)" or "✗ msg failed (Xs)"
// when the returned function is called. Intended for use with defer:
//
//	defer toolkit.LogStep(logger, "creating firewall")()
func LogStep(logger Logger, msg string) func() {
	if tb := tbOf(logger); tb != nil {
		tb.Helper()
	}
	logger.Log("→", msg+"...")
	start := time.Now()
	alreadyFailed := failed(logger)
	return func() {
		if tb := tbOf(logger); tb != nil {
			tb.Helper()
		}
		elapsed := time.Since(start).Seconds()
		if !alreadyFailed && failed(logger) {
			logger.Logf("✗ %s failed (%.1fs)", msg, elapsed)
		} else {
			logger.Logf("✓ %s done (%.1fs)", msg, elapsed)
		}
	}
}

// LogStepf is like LogStep but accepts a format string.
func LogStepf(logger Logger, format string, args ...any) func() {
	if tb := tbOf(logger); tb != nil {
		tb.Helper()
	}
	return LogStep(logger, fmt.Sprintf(format, args...))
}

// LogStepCtx is like LogStep but takes the logger from the context.
func LogStepCtx(ctx context.Context, msg string) func() {
	logger := LoggerFromContext(ctx)
	if tb := tbOf(logger); tb != nil {
		tb.Helper()
	}
	return LogStep(logger, msg)
}

// LogStepCtxf is like LogStepCtx but accepts a format string.
func LogStepCtxf(ctx context.Context, format string, args ...any) func() {
	logger := LoggerFromContext(ctx)
	if tb := tbOf(logger); tb != nil {
		tb.Helper()
	}
	return LogStep(logger, fmt.Sprintf(format, args...))
}

// failed reports whether the test behind logger has already failed. It is used
// only to pick the step status marker; failure state is never exposed through
// the Logger interface.
func failed(logger Logger) bool {
	if tb := tbOf(logger); tb != nil {
		return tb.Failed()
	}
	return false
}
