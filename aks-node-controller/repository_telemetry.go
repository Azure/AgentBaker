package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
)

type repositoryTargetKey struct{}

// The acquisition context carries the target into parallel metadata/package downloads
// without mutable App state or changing the generic retry helper.
func repositoryTarget(ctx context.Context) string {
	target, _ := ctx.Value(repositoryTargetKey{}).(string)
	return target
}

type repositoryEvent struct {
	Target      string `json:"target,omitempty"`
	File        string `json:"file,omitempty"`
	Attempt     int    `json:"attempt,omitempty"`
	MaxAttempts int    `json:"maxAttempts,omitempty"`
	Error       string `json:"error,omitempty"`
	Route       string `json:"route"`
	Outcome     string `json:"outcome,omitempty"`
	Reason      string `json:"reason,omitempty"`
	DurationMs  int64  `json:"durationMs"`
}

// Preserve repository URLs in diagnostics, excluding potential query/fragment tokens.
var repositoryTelemetryQuery = regexp.MustCompile(`[?#][^\s"'<>]+`)

func boundedRepositoryTelemetryText(text string, limit int) string {
	// Bound JSON-encoded bytes, not just characters (control characters expand).
	if len(text) > limit {
		text = text[:limit]
	}
	text = strings.ToValidUTF8(text, "")
	for {
		encoded, _ := json.Marshal(text)
		if len(encoded) <= limit {
			return text
		}
		_, size := utf8.DecodeLastRuneInString(text)
		text = text[:len(text)-size]
	}
}

func (a *App) logRepositoryEvent(name string, event repositoryEvent, level helpers.EventLevel, start, end time.Time) {
	event.Target = boundedRepositoryTelemetryText(event.Target, 128)
	event.File = boundedRepositoryTelemetryText(event.File, 768)
	event.Error = boundedRepositoryTelemetryText(
		repositoryTelemetryQuery.ReplaceAllString(event.Error, "[query redacted]"), 1536)
	event.DurationMs = end.Sub(start).Milliseconds()
	message, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal repository telemetry", "error", err)
		return
	}
	a.logEvent(name, string(message), level, start, end)
}
