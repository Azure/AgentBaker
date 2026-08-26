package toolkit

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestLoggerDoesNotExposeTestControl(t *testing.T) {
	var logger Logger = &stateLogger{}
	if _, ok := logger.(interface{ Error(args ...any) }); ok {
		t.Error("Logger must not expose Error")
	}
	if _, ok := logger.(interface{ Skip(args ...any) }); ok {
		t.Error("Logger must not expose Skip")
	}
	if _, ok := logger.(interface{ Cleanup(func()) }); ok {
		t.Error("Logger must not expose Cleanup")
	}
}

func TestContextLoggerRoundTrip(t *testing.T) {
	logger := &stateLogger{}
	ctx := ContextWithLogger(context.Background(), logger)

	if LoggerFromContext(ctx) != logger {
		t.Fatal("LoggerFromContext did not return the stored logger")
	}
	Log(ctx, "from context")
	Logf(ctx, "formatted %s", "value")
	if got := strings.Join(logger.logs, "\n"); !strings.Contains(got, "from context") || !strings.Contains(got, "formatted value") {
		t.Fatalf("unexpected log output:\n%s", got)
	}
}

func TestLoggerFromContextFallsBackToStandardLogger(t *testing.T) {
	if _, ok := LoggerFromContext(context.Background()).(stdLogger); !ok {
		t.Fatal("expected the standard logger fallback")
	}
}

type stateLogger struct {
	logs []string
}

func (l *stateLogger) Log(args ...any) {
	l.logs = append(l.logs, fmt.Sprint(args...))
}

func (l *stateLogger) Logf(format string, args ...any) {
	l.logs = append(l.logs, fmt.Sprintf(format, args...))
}

func TestLogStepReportsDuration(t *testing.T) {
	logger := &stateLogger{}
	done := LogStep(logger, "creating firewall")
	done()
	if len(logger.logs) != 2 || !strings.Contains(logger.logs[1], "finished") {
		t.Fatalf("unexpected step logs: %v", logger.logs)
	}
}
