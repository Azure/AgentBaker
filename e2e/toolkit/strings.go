package toolkit

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func StrToInt32(s string) int32 {
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		panic(err)
	}
	return int32(i)
}

func LogDuration(ctx context.Context, duration time.Duration, warningDuration time.Duration, message string) {
	if t, ok := ctx.Value(testLoggerKey{}).(testing.TB); ok {
		// exclude this function from log stack trace
		t.Helper()
	}
	if duration > warningDuration {
		Logf(ctx, "⚠️ ##vso[task.logissue type=warning;] %s", message)
	} else {
		Log(ctx, message)
	}
}
