package logging

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

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
	ctx := WithLogger(context.Background(), logger)

	assert.Same(t, logger, FromContext(ctx))
	Log(ctx, "from context")
	Logf(ctx, "formatted %s", "value")
	assert.Equal(t, []string{"from context", "formatted value"}, logger.logs)
}

func TestFromContextFallsBackToStandardLogger(t *testing.T) {
	var output strings.Builder
	original := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(original) })

	ctx := context.Background()
	Log(ctx, "fallback", "message")
	Logf(ctx, "formatted %s", "fallback")
	assert.Contains(t, output.String(), "fallback message\n")
	assert.Contains(t, output.String(), "formatted fallback\n")
}

func TestWithNilLoggerPreservesContext(t *testing.T) {
	var output strings.Builder
	original := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(original) })

	logger := &stateLogger{}
	ctx := WithLogger(t.Context(), logger)
	assert.Same(t, ctx, WithLogger(ctx, nil))
	Log(ctx, "retained")
	assert.Equal(t, []string{"retained"}, logger.logs)
	assert.Contains(t, output.String(), "WARNING: no logger provided")
}

type stateLogger struct {
	logs []string
}

func (l *stateLogger) Log(args ...any) {
	l.logs = append(l.logs, strings.TrimSuffix(fmt.Sprintln(args...), "\n"))
}

func (l *stateLogger) Logf(format string, args ...any) {
	l.logs = append(l.logs, fmt.Sprintf(format, args...))
}

func TestLogStepReportsDuration(t *testing.T) {
	for _, formatted := range []bool{false, true} {
		t.Run(fmt.Sprintf("formatted=%t", formatted), func(t *testing.T) {
			logger := &stateLogger{}
			ctx, cancel := context.WithCancel(WithLogger(t.Context(), logger))
			defer cancel()
			var done func()
			if formatted {
				done = LogStepf(ctx, "creating %s", "firewall")
			} else {
				done = LogStep(ctx, "creating firewall")
			}
			require.Equal(t, []string{"→ creating firewall..."}, logger.logs)
			cancel()
			done()
			require.Len(t, logger.logs, 2)
			assert.Regexp(t, `^← creating firewall finished \(\d+\.\ds\)$`, logger.logs[1])
		})
	}
}

func TestLogDurationWarningThreshold(t *testing.T) {
	for _, test := range []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{name: "below", duration: time.Second, want: "operation"},
		{name: "equal", duration: 2 * time.Second, want: "operation"},
		{name: "above", duration: 3 * time.Second, want: "⚠️ ##vso[task.logissue type=warning;] operation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			logger := &stateLogger{}
			LogDuration(WithLogger(t.Context(), logger), test.duration, 2*time.Second, "operation")
			assert.Equal(t, []string{test.want}, logger.logs)
		})
	}
}
