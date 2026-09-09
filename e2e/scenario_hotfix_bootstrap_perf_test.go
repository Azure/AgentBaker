package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHotfixTiming(t *testing.T) {
	t.Run("fast path", func(t *testing.T) {
		log := `time=2026-09-03T01:00:00Z level=INFO msg="downloading ANC hotfix" current=202608.21.0 target=202608.21.1
time=2026-09-03T01:00:01Z level=INFO msg="downloaded ANC hotfix through authenticated repository fast path" target=202608.21.1 format=deb path=/opt/azure/containers/aks-node-controller-hotfix durationMs=1843`
		timing, err := parseHotfixTiming(log)
		require.NoError(t, err)
		assert.True(t, timing.FastPath)
		assert.Equal(t, int64(1843), timing.Duration)
	})

	// The fast-path message contains the package-manager message as a prefix, so a naive
	// substring check would report a fast-path run as a package-manager run.
	t.Run("package manager path is not confused with fast path", func(t *testing.T) {
		log := `time=2026-09-03T01:00:00Z level=WARN msg="safe repository download unavailable, falling back to package manager" version=202608.21.1
time=2026-09-03T01:00:13Z level=INFO msg="downloaded ANC hotfix" target=202608.21.1 path=/opt/azure/containers/aks-node-controller-hotfix durationMs=13204`
		timing, err := parseHotfixTiming(log)
		require.NoError(t, err)
		assert.False(t, timing.FastPath, "this run fell back and must not be reported as the fast path")
		assert.Equal(t, int64(13204), timing.Duration)
	})

	t.Run("no hotfix ran", func(t *testing.T) {
		_, err := parseHotfixTiming("time=... msg=\"ANC version not targeted by hotfix, skipping download\"")
		assert.Error(t, err)
	})

	// A completion line without durationMs means the build predates the instrumentation;
	// reporting zero would look like an impossibly fast run.
	t.Run("completion without durationMs is an error, not a zero", func(t *testing.T) {
		_, err := parseHotfixTiming(`msg="downloaded ANC hotfix" target=202608.21.1`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no durationMs")
	})
}
