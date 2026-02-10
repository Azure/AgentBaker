package toolkit

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"
)

// testLoggerKey is a private key to prevent *testing.T from being easily accessed
type testLoggerKey struct{}

func ContextWithT(ctx context.Context, t testing.TB) context.Context {
	if t == nil {
		log.Println("WARNING: No *testing.T provided, this function should only be called from a test")
		return ctx
	}
	return context.WithValue(ctx, testLoggerKey{}, t)
}

func Logf(ctx context.Context, format string, args ...any) {
	t, ok := ctx.Value(testLoggerKey{}).(testing.TB)
	if !ok || t == nil {
		log.Printf(format+"WARNING: No *testing.T in Context, this function should only be called from ", args...)
	}
	t.Helper()
	t.Logf(format, args...)
}

func Log(ctx context.Context, args ...any) {
	t, ok := ctx.Value(testLoggerKey{}).(testing.TB)
	if !ok || t == nil {
		log.Println("WARNING: No *testing.T in Context, this function should only be called from a test")
		return
	}
	t.Helper()
	t.Log(args...)
}

type testLogger struct {
	testing.TB
	start time.Time
}

func (t *testLogger) elapsed() string {
	return fmt.Sprintf("[%.3fs]", time.Since(t.start).Seconds())
}

func (t *testLogger) Log(args ...any) {
	t.Helper()
	args = append([]any{t.elapsed()}, args...)
	t.TB.Log(args...)
}

func (t *testLogger) Logf(format string, args ...any) {
	t.Helper()
	t.TB.Logf(t.elapsed()+" "+format, args...)
}

// formatError formats the ERROR prefix with emoji
func (t *testLogger) formatError() string {
	return "🔴 FAIL:"
}

func (t *testLogger) Fatal(args ...any) {
	t.Helper()
	args = append([]any{t.elapsed(), t.formatError()}, args...)
	t.TB.Fatal(args...)
}

func (t *testLogger) Fatalf(format string, args ...any) {
	t.Helper()
	t.TB.Fatalf(t.elapsed()+" "+t.formatError()+" "+format, args...)
}

func (t *testLogger) Error(args ...any) {
	t.Helper()
	args = append([]any{t.elapsed(), t.formatError()}, args...)
	t.TB.Error(args...)
}

func (t *testLogger) Errorf(format string, args ...any) {
	t.Helper()
	t.TB.Errorf(t.elapsed()+" "+t.formatError()+" "+format, args...)
}

func (t *testLogger) FailNow() {
	t.Helper()
	t.Log(t.formatError())
	t.TB.FailNow()
}

func (t *testLogger) Fail() {
	t.Helper()
	t.Log(t.formatError())
	t.TB.Fail()
}

func WithTestLogger(t testing.TB) testing.TB {
	return &testLogger{TB: t, start: time.Now()}
}
