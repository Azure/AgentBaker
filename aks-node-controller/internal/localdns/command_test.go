package localdns

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedEvent struct {
	task, message string
	level         helpers.EventLevel
}

type fakeEventLogger struct{ events []recordedEvent }

func (f *fakeEventLogger) LogEvent(taskName, message string, eventLevel helpers.EventLevel, _, _ time.Time) {
	f.events = append(f.events, recordedEvent{task: taskName, message: message, level: eventLevel})
}

func TestFetchConfig(t *testing.T) {
	// fetch runs the command for a node in pool1 and returns its stdout lines and events.
	fetch := func(t *testing.T, fetcher Fetcher) ([]string, []recordedEvent, string, error) {
		t.Helper()
		src := newTestSources(t)
		writeTestNodeConfig(t, src)
		writeTestBakedCorefile(t, src, bakedTestCorefile(false))
		events := &fakeEventLogger{}
		var stdout bytes.Buffer
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")
		err := FetchConfig(context.Background(), FetchParams{
			Fetch:        fetcher,
			NotPublished: func(err error) bool { return errors.Is(err, errNotPublishedForTest) },
			Events:       events,
			OutputPath:   out,
			Sources:      *src,
			Stdout:       &stdout,
		})
		return strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n"), events.events, out, err
	}

	t.Run("prints the version and outcome and reports the outcome", func(t *testing.T) {
		lines, events, out, err := fetch(t, fetched(testPayload(map[string]string{"pool1": testPoolEntry("v1", 7200)})))
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", "rendered"}, lines)
		assert.Equal(t, []recordedEvent{{task: "FetchLocalDNSConfig", message: "localdns fetch-config outcome=rendered",
			level: helpers.EventLevelInformational}}, events)
		assert.FileExists(t, out)
	})

	t.Run("a fetch failure fails open", func(t *testing.T) {
		lines, events, out, err := fetch(t, func(context.Context) (string, error) {
			return "", assert.AnError
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"failed"}, lines)
		assert.Equal(t, []recordedEvent{{task: "FetchLocalDNSConfig", message: "localdns fetch-config outcome=failed error=" + assert.AnError.Error(),
			level: helpers.EventLevelError}}, events)
		assert.NoFileExists(t, out)
	})

	t.Run("nothing published is not found", func(t *testing.T) {
		lines, events, out, err := fetch(t, func(context.Context) (string, error) {
			return "", fmt.Errorf("get component config: %w", errNotPublishedForTest)
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"notFound"}, lines)
		assert.Equal(t, []recordedEvent{{task: "FetchLocalDNSConfig", message: "localdns fetch-config outcome=notFound",
			level: helpers.EventLevelInformational}}, events)
		assert.NoFileExists(t, out)
	})

	t.Run("a panic fails open", func(t *testing.T) {
		lines, events, _, err := fetch(t, func(context.Context) (string, error) {
			panic("boom")
		})
		require.NoError(t, err)
		assert.Equal(t, []string{""}, lines, "nothing is printed")
		assert.Equal(t, []recordedEvent{{task: "FetchLocalDNSConfig", message: "localdns fetch-config outcome=failed panic=boom",
			level: helpers.EventLevelError}}, events)
	})

	t.Run("events are optional", func(t *testing.T) {
		var stdout bytes.Buffer
		err := FetchConfig(context.Background(), FetchParams{
			Fetch:      func(context.Context) (string, error) { return "", assert.AnError },
			OutputPath: filepath.Join(t.TempDir(), "out"),
			Sources:    *newTestSources(t),
			Stdout:     &stdout,
		})
		require.NoError(t, err)
		assert.Equal(t, "failed\n", stdout.String())
	})
}

// errNotPublishedForTest stands in for LPS answering that it has nothing for this node.
var errNotPublishedForTest = errors.New("nothing published")

// TestApplyConfig pins the output contract the localDNS handler parses: the outcome is always the
// last line, and on failure the reason precedes it as "error: <reason>".
func TestApplyConfig(t *testing.T) {
	run := func(t *testing.T, src *Sources, configPath, agentPool string) ([]string, error) {
		t.Helper()
		var stdout bytes.Buffer
		err := ApplyConfig(context.Background(), ApplyParams{
			ConfigPath: configPath,
			OutputPath: filepath.Join(t.TempDir(), "livepatched.localdns.corefile"),
			AgentPool:  agentPool,
			Sources:    *src,
			Stdout:     &stdout,
		})
		return strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n"), err
	}
	configFile := func(t *testing.T) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "localdns-config.json")
		require.NoError(t, os.WriteFile(p, []byte(testPayload(map[string]string{"pool1": testPoolEntry("v1", 7200)})), 0o600))
		return p
	}

	t.Run("failure prints the reason before the outcome", func(t *testing.T) {
		lines, err := run(t, newTestSources(t), configFile(t), "")
		require.Error(t, err)
		require.Len(t, lines, 2)
		assert.Equal(t, "error: "+err.Error(), lines[0])
		assert.Equal(t, string(OutcomeFailed), lines[1])
	})

	t.Run("success prints the version before the outcome", func(t *testing.T) {
		src := newTestSources(t)
		writeTestBakedCorefile(t, src, bakedTestCorefile(false))

		lines, err := run(t, src, configFile(t), "pool1")
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", string(OutcomeRendered)}, lines)
	})

	t.Run("an unreadable config file is a failure", func(t *testing.T) {
		lines, err := run(t, newTestSources(t), filepath.Join(t.TempDir(), "missing.json"), "pool1")
		require.Error(t, err)
		assert.Equal(t, []string{"error: " + err.Error(), "failed"}, lines)
	})
}
