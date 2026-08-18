package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// realGPUBumpLogLine is the exact line emitted by aks-node-controller's compareEnvs during
// Agentbaker GPU E2E build 176997071 (Test_Ubuntu2204_GPUA10), where the only differences were
// caused by bumping the aks-gpu-grid driver version in parts/common/components.json while the
// VHD under test was still built from main.
const realGPUBumpLogLine = `{"time":"2026-08-17T22:12:04.882319563Z","level":"INFO","msg":"env var differences (2): differs: GPU_DRIVER_VERSION; differs: GPU_IMAGE_SHA"}`

func TestParseEnvCompareDiffVars(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty output yields no vars",
			input:    "",
			expected: nil,
		},
		{
			name:     "real GPU driver bump log line",
			input:    realGPUBumpLogLine,
			expected: []string{"GPU_DRIVER_VERSION", "GPU_IMAGE_SHA"},
		},
		{
			name:     "all three diff kinds are parsed",
			input:    `env var differences (3): differs: FOO; only-in-pc: BAR; only-in-nbc: BAZ`,
			expected: []string{"BAR", "BAZ", "FOO"},
		},
		{
			name:     "duplicate entries across repeated log lines are de-duplicated",
			input:    realGPUBumpLogLine + "\n" + realGPUBumpLogLine,
			expected: []string{"GPU_DRIVER_VERSION", "GPU_IMAGE_SHA"},
		},
		{
			name:     "unrelated log noise is ignored",
			input:    "some unrelated line mentioning differs but no entry\nanother line",
			expected: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, parseEnvCompareDiffVars(tc.input))
		})
	}
}

func TestUnexpectedEnvCompareDiffVars(t *testing.T) {
	for _, tc := range []struct {
		name     string
		diffVars []string
		expected []string
	}{
		{
			name:     "GPU vars alone are explained by VHD skew",
			diffVars: []string{"GPU_DRIVER_VERSION", "GPU_IMAGE_SHA"},
			expected: nil,
		},
		{
			name:     "a non-GPU var is still reported",
			diffVars: []string{"GPU_DRIVER_VERSION", "KUBELET_FLAGS"},
			expected: []string{"KUBELET_FLAGS"},
		},
		{
			name:     "only non-GPU vars are reported",
			diffVars: []string{"KUBELET_FLAGS", "NETWORK_PLUGIN"},
			expected: []string{"KUBELET_FLAGS", "NETWORK_PLUGIN"},
		},
		{
			name:     "no diffs yields none",
			diffVars: nil,
			expected: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, unexpectedEnvCompareDiffVars(tc.diffVars))
		})
	}
}

// TestEnvCompareGPUBumpIsTolerated asserts the end-to-end decision for the exact failure that
// blocked the aks-gpu-grid 570.237 bump: the diff parses to only VHD-sourced vars, so it must
// not fail the scenario.
func TestEnvCompareGPUBumpIsTolerated(t *testing.T) {
	diffVars := parseEnvCompareDiffVars(realGPUBumpLogLine)
	assert.Equal(t, []string{"GPU_DRIVER_VERSION", "GPU_IMAGE_SHA"}, diffVars)
	assert.Empty(t, unexpectedEnvCompareDiffVars(diffVars),
		"a GPU driver version bump must not fail phase 3 when the VHD is built from a different source")
}
