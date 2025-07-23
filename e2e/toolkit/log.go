package toolkit

import (
	"context"
	"log"
	"testing"
)

// testLoggerKey is a private key to prevent *testing.T from being easily accessed
type testLoggerKey struct{}

func ContextWithLog(ctx context.Context, t *testing.T) context.Context {
	if t == nil {
		log.Println("WARNING: No *testing.T provided, this function should only be called from a test")
		return ctx
	}
	return context.WithValue(ctx, testLoggerKey{}, t)
}

func Logf(ctx context.Context, format string, args ...any) {
	t, ok := ctx.Value(testLoggerKey{}).(*testing.T)
	if !ok || t == nil {
		log.Printf(format+"WARNING: No *testing.T in Context, this function should only be called from ", args...)
	}
	t.Helper()
	t.Logf(format, args...)
}

func Log(ctx context.Context, args ...any) {
	t, ok := ctx.Value(testLoggerKey{}).(*testing.T)
	if !ok || t == nil {
		log.Println("WARNING: No *testing.T in Context, this function should only be called from a test")
		return
	}
	t.Helper()
	t.Log(args...)
}
