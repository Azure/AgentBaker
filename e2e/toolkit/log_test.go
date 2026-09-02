package toolkit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerDoesNotExposeTestControl(t *testing.T) {
	var logger Logger = &stateLogger{}
	assert.NotImplements(t, (*interface{ Error(args ...any) })(nil), logger)
	assert.NotImplements(t, (*interface{ Skip(args ...any) })(nil), logger)
	assert.NotImplements(t, (*interface{ Cleanup(func()) })(nil), logger)
}

func TestContextLoggerRoundTrip(t *testing.T) {
	logger := &stateLogger{}
	ctx := ContextWithLogger(context.Background(), logger)

	assert.Same(t, logger, LoggerFromContext(ctx))
	Log(ctx, "from context")
	Logf(ctx, "formatted %s", "value")
	got := strings.Join(logger.logs, "\n")
	assert.Contains(t, got, "from context")
	assert.Contains(t, got, "formatted value")
}

func TestLoggerFromContextFallsBackToStandardLogger(t *testing.T) {
	assert.IsType(t, stdLogger{}, LoggerFromContext(context.Background()))
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
	require.Len(t, logger.logs, 2)
	assert.Contains(t, logger.logs[1], "finished")
}
