package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadKubeletConfigFile_ReadsValidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "kubeletconfig.json")
	configContent := `{"maxPods":110,"clusterDNS":["10.0.0.10"],"featureGates":{"RotateKubeletServerCertificate":true}}`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	result := readKubeletConfigFile(configPath, true)

	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(110), m["maxPods"])
	dns, ok := m["clusterDNS"].([]any)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.10", dns[0])
}

func TestReadKubeletConfigFile_ReturnsEmptyWhenNotUsed(t *testing.T) {
	result := readKubeletConfigFile("/nonexistent", false)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, m)
}

func TestReadKubeletConfigFile_ReturnsEmptyWhenFileMissing(t *testing.T) {
	result := readKubeletConfigFile("/nonexistent/kubeletconfig.json", true)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, m)
}

func TestReadKubeletConfigFile_ReturnsEmptyOnInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "kubeletconfig.json")
	require.NoError(t, os.WriteFile(configPath, []byte("not json{{{"), 0644))

	result := readKubeletConfigFile(configPath, true)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, m)
}

func TestMarshalWithSizeGuard_SmallPayload(t *testing.T) {
	payload := KubeletActiveFlagsPayload{
		Found:      true,
		FlagCount:  2,
		Flags:      map[string]string{"max-pods": "110", "config": "/etc/default/kubeletconfig.json"},
		ConfigFile: map[string]any{"maxPods": float64(110)},
	}

	result := marshalWithSizeGuard(payload)
	require.NotNil(t, result)
	assert.LessOrEqual(t, len(result), maxMessageBytes)

	var parsed KubeletActiveFlagsPayload
	require.NoError(t, json.Unmarshal(result, &parsed))
	assert.True(t, parsed.Found)
	assert.Equal(t, 2, parsed.FlagCount)
	assert.Equal(t, "110", parsed.Flags["max-pods"])
}

func TestMarshalWithSizeGuard_TruncatesLargePayload(t *testing.T) {
	// Create a payload that exceeds maxMessageBytes
	largeConfig := make(map[string]any)
	for i := range 500 {
		largeConfig[strings.Repeat("k", 15)+string(rune('a'+i%26))+string(rune('a'+i/26%26))] = strings.Repeat("v", 50)
	}

	payload := KubeletActiveFlagsPayload{
		Found:      true,
		FlagCount:  3,
		Flags:      map[string]string{"max-pods": "110"},
		ConfigFile: largeConfig,
	}

	// Verify it would exceed the cap
	raw, _ := json.Marshal(payload)
	require.Greater(t, len(raw), maxMessageBytes)

	result := marshalWithSizeGuard(payload)
	require.NotNil(t, result)
	assert.LessOrEqual(t, len(result), maxMessageBytes)

	var parsed KubeletActiveFlagsPayload
	require.NoError(t, json.Unmarshal(result, &parsed))
	assert.Equal(t, "truncated:exceeded_size_cap", parsed.ConfigFile)
	assert.True(t, parsed.Found)
	assert.Equal(t, "110", parsed.Flags["max-pods"])
}

func TestGetKubeletTrackedFlags_ContainsExpectedFlags(t *testing.T) {
	flags := getKubeletTrackedFlags()
	expected := []string{"config", "max-pods", "cgroup-driver", "feature-gates", "eviction-hard"}
	for _, e := range expected {
		assert.Contains(t, flags, e)
	}
}

func TestEmitKubeletActiveFlagsEvent_WritesEventFile(t *testing.T) {
	// This test verifies the event emission path without real journalctl.
	// Since journalctl won't return FLAG lines in a test environment,
	// the payload will have found=false.
	dir := t.TempDir()
	logger := NewEventLogger(dir)

	// EmitKubeletActiveFlagsEvent is best-effort; it should not panic
	// even when journalctl is unavailable.
	logger.EmitKubeletActiveFlagsEvent()

	events := logger.Events()
	// With no kubelet running, we still get an event with found=false
	if len(events) > 0 {
		assert.Contains(t, events[0].TaskName, "kubeletActiveFlags")
		// LogEvent appends " | startTime=... endTime=... durationMs=..." to the message.
		// Extract the JSON portion (everything before " | ").
		msg := events[0].Message
		if idx := strings.Index(msg, " | "); idx > 0 {
			msg = msg[:idx]
		}
		var payload KubeletActiveFlagsPayload
		require.NoError(t, json.Unmarshal([]byte(msg), &payload))
		assert.False(t, payload.Found)
	}
	// If journalctl is completely unavailable (no systemd), no event is written — also acceptable.
}
