package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func repositoryEvents(t *testing.T, app *App, name string) []repositoryEvent {
	t.Helper()
	var result []repositoryEvent
	for _, event := range app.eventLogger.Events() {
		if event.TaskName != "AKS.AKSNodeController."+name {
			continue
		}
		require.LessOrEqual(t, len(event.Message), 3*1024, "includes EventLogger timing suffix")
		message, _, found := strings.Cut(event.Message, " | startTime=")
		require.True(t, found)
		var fields repositoryEvent
		require.NoError(t, json.Unmarshal([]byte(message), &fields))
		assert.GreaterOrEqual(t, fields.DurationMs, int64(0))
		expectedLevel := helpers.EventLevelInformational
		if fields.Reason == "integrity" || fields.Outcome == "failed" {
			expectedLevel = helpers.EventLevelError
		}
		assert.Equal(t, string(expectedLevel), event.EventLevel)
		result = append(result, fields)
	}
	return result
}

func assertRepositoryOutcome(t *testing.T, app *App, route, outcome string) {
	t.Helper()
	events := repositoryEvents(t, app, "RepositoryDownloadOutcome")
	require.Len(t, events, 1)
	assert.Equal(t, route, events[0].Route)
	assert.Equal(t, outcome, events[0].Outcome)
	assert.Equal(t, "202608.21.1", events[0].Target)
	if outcome == "failed" {
		assert.NotEmpty(t, events[0].Error)
	} else {
		assert.Empty(t, events[0].Error)
	}
}

func TestRepositoryTelemetryBoundedAndSanitized(t *testing.T) {
	app := NewTestApp(t, TestAppConfig{}).App
	start := time.Now().Add(-25 * time.Millisecond)
	rawURL := "https://packages.example/pool/anc.deb?sig=secret#credential"
	app.logRepositoryEvent("RepositoryDownloadRetry", repositoryEvent{
		Target: strings.Repeat("\x00\u754c", 2000),
		File:   rawURL, Attempt: 2, MaxAttempts: 2,
		Error: "Get \"" + rawURL + "\": failure for relative/path?token=hidden " +
			strings.Repeat("\x00\u754c", 2000),
		Route: "fastpath",
	}, helpers.EventLevelInformational, start, start.Add(25*time.Millisecond))
	events := repositoryEvents(t, app, "RepositoryDownloadRetry")
	require.Len(t, events, 1)
	assert.Equal(t, rawURL, events[0].File)
	assert.Equal(t, int64(25), events[0].DurationMs)
	message := app.eventLogger.Events()[0].Message
	assert.True(t, utf8.ValidString(message))
	assert.Contains(t, events[0].Error, "https://packages.example/pool/anc.deb")
	for _, secret := range []string{"password", "secret", "credential", "hidden", "user:"} {
		assert.NotContains(t, events[0].Error, secret)
	}

	app.logRepositoryEvent("RepositoryDownloadRetry", repositoryEvent{
		File:   "https://example/" + strings.Repeat("x", 5000),
		Target: strings.Repeat("x", 5000), Error: strings.Repeat("\x00", 5000),
		Route: "fastpath", Attempt: 2, MaxAttempts: 2,
	}, helpers.EventLevelInformational, start, start)
	require.Len(t, repositoryEvents(t, app, "RepositoryDownloadRetry"), 2)
}

func TestRepositoryTelemetryNilLogger(t *testing.T) {
	app := &App{}
	now := time.Now()
	require.NotPanics(t, func() {
		app.logEvent("test", "message", helpers.EventLevelInformational, now, now)
		app.logRepositoryEvent("RepositoryDownloadOutcome", repositoryEvent{
			Route: "fastpath", Outcome: "success",
		}, helpers.EventLevelInformational, now, now)
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	downloadApp, origin := newRepositoryDownloadTestApp(t, server.URL)
	downloadApp.eventLogger = nil
	_, err := downloadApp.downloadRepositoryFile(context.Background(), server.URL+"/anc.deb",
		origin, repositoryPackageMaxBytes)
	require.Error(t, err)
}

func TestRepositoryTelemetryConcurrentFileRetries(t *testing.T) {
	var mu sync.Mutex
	attempts := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts[r.URL.Path]++
		attempt := attempts[r.URL.Path]
		mu.Unlock()
		if attempt == 1 {
			http.Error(w, "retry", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("body"))
	}))
	defer server.Close()
	app, origin := newRepositoryDownloadTestApp(t, server.URL)
	ctx := context.WithValue(context.Background(), repositoryTargetKey{}, "202608.21.1")
	file, _, err := app.fetchPackageAndMetadata(ctx, repositoryDownloadPlan{
		packageURL: server.URL + "/pool/anc.deb", trustedOrigin: origin,
		resolveMetadata: func(ctx context.Context) (repositoryPackageMetadata, error) {
			for _, path := range []string{"/dists/jammy/InRelease", "/dists/jammy/Packages.gz"} {
				metadata, err := app.downloadRepositoryFile(ctx, server.URL+path, origin, repositoryMetadataMaxBytes)
				if err != nil {
					return repositoryPackageMetadata{}, err
				}
				_ = os.Remove(metadata.path)
			}
			return repositoryPackageMetadata{}, nil
		},
	})
	require.NoError(t, err)
	defer func() { _ = os.Remove(file.path) }()
	events := repositoryEvents(t, app, "RepositoryDownloadRetry")
	require.Len(t, events, 3)
	var files []string
	for _, event := range events {
		assert.Equal(t, "202608.21.1", event.Target)
		assert.Equal(t, "fastpath", event.Route)
		assert.Equal(t, 2, event.Attempt)
		assert.Equal(t, 2, event.MaxAttempts)
		assert.Contains(t, event.Error, "HTTP 503")
		assert.NotContains(t, event.Error, "secret")
		files = append(files, event.File)
	}
	assert.ElementsMatch(t, []string{
		server.URL + "/pool/anc.deb",
		server.URL + "/dists/jammy/InRelease",
		server.URL + "/dists/jammy/Packages.gz",
	}, files)
}

func TestRepositoryTelemetryFirstAttemptSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	defer server.Close()
	app, origin := newRepositoryDownloadTestApp(t, server.URL)
	file, err := app.downloadRepositoryFile(context.Background(), server.URL+"/InRelease",
		origin, repositoryMetadataMaxBytes)
	require.NoError(t, err)
	defer func() { _ = os.Remove(file.path) }()
	assert.Empty(t, repositoryEvents(t, app, "RepositoryDownloadRetry"))
}

func TestRepositoryTelemetryPackageManagerInstallFailure(t *testing.T) {
	app := configuredUbuntuRepositoryApp(t, t.TempDir(), "http://127.0.0.1:1",
		func(*exec.Cmd) error { return context.Canceled })
	// Reject the fast path before network access, then fail package-manager installation.
	require.NoError(t, os.RemoveAll(app.aptSourcesDir))
	original := Version
	Version = "202608.21.0"
	t.Cleanup(func() { Version = original })
	err := app.downloadBinaryHotfixIfNeeded(context.Background(), &hotfixConfig{Version: "202608.21.1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "install hotfix version")
	assertRepositoryOutcome(t, app, "packageManager", "failed")
	fallback := repositoryEvents(t, app, "RepositoryDownloadFallback")
	require.Len(t, fallback, 1)
	assert.Equal(t, "unavailable", fallback[0].Reason)
	assert.Equal(t, "packageManager", fallback[0].Route)
	assert.False(t, strings.Contains(fallback[0].Error, "SHA-256"))
	assert.False(t, strings.Contains(fallback[0].Error, "tamper"))
}
