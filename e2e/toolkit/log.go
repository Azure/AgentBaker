package toolkit

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Logger is the minimal logging capability used by e2e implementation code.
//
// It deliberately offers logging only, so implementation code cannot change
// the outcome of a scenario through a Logger.
type Logger interface {
	Log(args ...any)
	Logf(format string, args ...any)
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
	logger.Logf(format, args...)
}

func Log(ctx context.Context, args ...any) {
	logger := LoggerFromContext(ctx)
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

// LogStep logs "→ msg..." at the start and "← msg finished (Xs)" when the returned function is called.
//
//	defer toolkit.LogStep(logger, "creating firewall")()
func LogStep(logger Logger, msg string) func() {
	logger.Log("→", msg+"...")
	start := time.Now()
	return func() {
		elapsed := time.Since(start).Seconds()
		logger.Logf("← %s finished (%.1fs)", msg, elapsed)
	}
}

// LogStepf is like LogStep but accepts a format string.
func LogStepf(logger Logger, format string, args ...any) func() {
	return LogStep(logger, fmt.Sprintf(format, args...))
}

// LogStepCtx is like LogStep but takes the logger from the context.
func LogStepCtx(ctx context.Context, msg string) func() {
	logger := LoggerFromContext(ctx)
	return LogStep(logger, msg)
}

// LogStepCtxf is like LogStepCtx but accepts a format string.
func LogStepCtxf(ctx context.Context, format string, args ...any) func() {
	logger := LoggerFromContext(ctx)
	return LogStep(logger, fmt.Sprintf(format, args...))
}
