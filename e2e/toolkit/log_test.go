package toolkit

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// recordingTB records what an implementation writes to a testing.TB. The
// embedded testing.TB is nil on purpose: any call to a method the tests do not
// expect panics instead of silently passing.
type recordingTB struct {
	testing.TB
	logs   []string
	errors []string
	failed bool
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Log(args ...any) {
	r.logs = append(r.logs, strings.TrimSuffix(fmt.Sprintln(args...), "\n"))
}

func (r *recordingTB) Logf(format string, args ...any) {
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}

func (r *recordingTB) Error(args ...any) {
	r.errors = append(r.errors, strings.TrimSuffix(fmt.Sprintln(args...), "\n"))
	r.failed = true
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
	r.failed = true
}

func (r *recordingTB) Failed() bool { return r.failed }

var elapsedPrefix = regexp.MustCompile(`^\[\d+\.\d{3}s\] `)

func TestTestLoggerPrefixesElapsedTime(t *testing.T) {
	rec := &recordingTB{}
	logger := NewTestLogger(rec)

	logger.Log("hello", "world")
	logger.Logf("count %d", 3)

	if len(rec.logs) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %v", len(rec.logs), rec.logs)
	}
	for _, line := range rec.logs {
		if !elapsedPrefix.MatchString(line) {
			t.Errorf("line %q does not start with an elapsed time prefix", line)
		}
	}
	if got, want := elapsedPrefix.ReplaceAllString(rec.logs[0], ""), "hello world"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := elapsedPrefix.ReplaceAllString(rec.logs[1], ""), "count 3"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestLoggerDoesNotExposeTestControl locks in the boundary this package exists
// for: implementation code holding a Logger must not be able to reach any
// test-control capability.
func TestLoggerDoesNotExposeTestControl(t *testing.T) {
	logger := NewTestLogger(&recordingTB{})

	if _, ok := logger.(testing.TB); ok {
		t.Error("Logger must not satisfy testing.TB")
	}
	if _, ok := logger.(interface{ Error(args ...any) }); ok {
		t.Error("Logger must not expose Error")
	}
	if _, ok := logger.(interface{ Errorf(string, ...any) }); ok {
		t.Error("Logger must not expose Errorf")
	}
	if _, ok := logger.(interface{ Fatal(args ...any) }); ok {
		t.Error("Logger must not expose Fatal")
	}
	if _, ok := logger.(interface{ Skip(args ...any) }); ok {
		t.Error("Logger must not expose Skip")
	}
	if _, ok := logger.(interface{ Failed() bool }); ok {
		t.Error("Logger must not expose Failed")
	}
	if _, ok := logger.(interface{ Name() string }); ok {
		t.Error("Logger must not expose Name")
	}
	if _, ok := logger.(interface{ Cleanup(func()) }); ok {
		t.Error("Logger must not expose Cleanup")
	}
	if _, ok := logger.(interface{ Helper() }); ok {
		t.Error("Logger must not expose Helper")
	}
}

func TestContextLoggerRoundTrip(t *testing.T) {
	rec := &recordingTB{}
	logger := NewTestLogger(rec)
	ctx := ContextWithLogger(context.Background(), logger)

	if LoggerFromContext(ctx) != logger {
		t.Fatal("LoggerFromContext did not return the logger stored in the context")
	}

	Log(ctx, "from context")
	Logf(ctx, "formatted %s", "value")

	if len(rec.logs) != 2 {
		t.Fatalf("expected 2 log lines, got %v", rec.logs)
	}
	if got, want := elapsedPrefix.ReplaceAllString(rec.logs[0], ""), "from context"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := elapsedPrefix.ReplaceAllString(rec.logs[1], ""), "formatted value"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLoggerFromContextFallsBackToStandardLogger(t *testing.T) {
	logger := LoggerFromContext(context.Background())
	if logger == nil {
		t.Fatal("LoggerFromContext returned nil")
	}
	if _, ok := logger.(stdLogger); !ok {
		t.Fatalf("expected the standard logger fallback, got %T", logger)
	}
	// must not panic without a test in the context
	Log(context.Background(), "no logger in context")
	Logf(context.Background(), "no logger in %s", "context")
	LogStepCtx(context.Background(), "step without a logger")()
	LogStepCtxf(context.Background(), "step %d without a logger", 2)()
}

func TestContextWithNilLoggerKeepsContext(t *testing.T) {
	ctx := ContextWithLogger(context.Background(), nil)
	if _, ok := LoggerFromContext(ctx).(stdLogger); !ok {
		t.Fatal("a nil logger must leave the context without a logger")
	}
}

func TestLogStepReportsSuccess(t *testing.T) {
	rec := &recordingTB{}
	LogStep(NewTestLogger(rec), "creating firewall")()

	if len(rec.logs) != 2 {
		t.Fatalf("expected 2 log lines, got %v", rec.logs)
	}
	if got, want := elapsedPrefix.ReplaceAllString(rec.logs[0], ""), "→ creating firewall..."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := elapsedPrefix.ReplaceAllString(rec.logs[1], ""); !strings.HasPrefix(got, "✓ creating firewall done (") {
		t.Errorf("got %q, want a success marker", got)
	}
}

func TestLogStepReportsFailureRaisedDuringTheStep(t *testing.T) {
	rec := &recordingTB{}
	done := LogStepf(NewTestLogger(rec), "creating %s", "firewall")
	rec.failed = true
	done()

	if got := elapsedPrefix.ReplaceAllString(rec.logs[1], ""); !strings.HasPrefix(got, "✗ creating firewall failed (") {
		t.Errorf("got %q, want a failure marker", got)
	}
}

func TestLogStepIgnoresFailureRaisedBeforeTheStep(t *testing.T) {
	rec := &recordingTB{failed: true}
	LogStep(NewTestLogger(rec), "collecting logs")()

	if got := elapsedPrefix.ReplaceAllString(rec.logs[1], ""); !strings.HasPrefix(got, "✓ collecting logs done (") {
		t.Errorf("got %q, want a success marker for a failure that predates the step", got)
	}
}

func TestFailureFormattingDecoratesFailures(t *testing.T) {
	rec := &recordingTB{}
	tb := WithFailureFormatting(rec)

	tb.Error("something broke")
	tb.Errorf("%s broke", "something else")

	if len(rec.errors) != 2 {
		t.Fatalf("expected 2 errors, got %v", rec.errors)
	}
	for _, line := range rec.errors {
		if !elapsedPrefix.MatchString(line) {
			t.Errorf("error %q does not start with an elapsed time prefix", line)
		}
		if !strings.Contains(line, "🔴 FAIL:") {
			t.Errorf("error %q does not carry the failure marker", line)
		}
	}
}
