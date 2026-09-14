package logging

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Logger interface {
	Log(args ...any)
	Logf(format string, args ...any)
}

type loggerKey struct{}

func WithLogger(ctx context.Context, logger Logger) context.Context {
	if logger == nil {
		log.Println("WARNING: no logger provided, falling back to the standard logger")
		return ctx
	}
	return context.WithValue(ctx, loggerKey{}, logger)
}

func FromContext(ctx context.Context) Logger {
	if logger, ok := ctx.Value(loggerKey{}).(Logger); ok && logger != nil {
		return logger
	}
	return stdLogger{}
}

func Logf(ctx context.Context, format string, args ...any) {
	FromContext(ctx).Logf(format, args...)
}

func Log(ctx context.Context, args ...any) {
	FromContext(ctx).Log(args...)
}

type stdLogger struct{}

func (stdLogger) Log(args ...any) {
	log.Println(args...)
}

func (stdLogger) Logf(format string, args ...any) {
	log.Printf(format, args...)
}

func LogStep(ctx context.Context, msg string) func() {
	logger := FromContext(ctx)
	logger.Log("→", msg+"...")
	start := time.Now()
	return func() {
		elapsed := time.Since(start).Seconds()
		logger.Logf("← %s finished (%.1fs)", msg, elapsed)
	}
}

func LogStepf(ctx context.Context, format string, args ...any) func() {
	return LogStep(ctx, fmt.Sprintf(format, args...))
}

func LogDuration(ctx context.Context, duration time.Duration, warningDuration time.Duration, message string) {
	if duration > warningDuration {
		Logf(ctx, "⚠️ ##vso[task.logissue type=warning;] %s", message)
	} else {
		Log(ctx, message)
	}
}
