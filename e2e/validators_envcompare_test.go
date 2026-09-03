package e2e

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
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
		name string
		// vhdBuiltFromSourceUnderTest mirrors config.Configuration.VHDBuiltFromSourceUnderTest:
		// true in the VHD builder pipelines (VHD_BUILD_ID set), false for the standalone e2e check.
		vhdBuiltFromSourceUnderTest bool
		diffVars                    []string
		expected                    []string
	}{
		{
			name:                        "GPU vars alone are explained by VHD skew when the VHD is not from this source",
			vhdBuiltFromSourceUnderTest: false,
			diffVars:                    []string{"GPU_DRIVER_VERSION", "GPU_IMAGE_SHA"},
			expected:                    nil,
		},
		{
			name:                        "GPU vars are NOT tolerated when the VHD was built from this source",
			vhdBuiltFromSourceUnderTest: true,
			diffVars:                    []string{"GPU_DRIVER_VERSION", "GPU_IMAGE_SHA"},
			expected:                    []string{"GPU_DRIVER_VERSION", "GPU_IMAGE_SHA"},
		},
		{
			name:                        "a non-GPU var is still reported",
			vhdBuiltFromSourceUnderTest: false,
			diffVars:                    []string{"GPU_DRIVER_VERSION", "KUBELET_FLAGS"},
			expected:                    []string{"KUBELET_FLAGS"},
		},
		{
			name:                        "only non-GPU vars are reported",
			vhdBuiltFromSourceUnderTest: false,
			diffVars:                    []string{"KUBELET_FLAGS", "NETWORK_PLUGIN"},
			expected:                    []string{"KUBELET_FLAGS", "NETWORK_PLUGIN"},
		},
		{
			name:                        "non-GPU vars are reported regardless of VHD provenance",
			vhdBuiltFromSourceUnderTest: true,
			diffVars:                    []string{"KUBELET_FLAGS", "NETWORK_PLUGIN"},
			expected:                    []string{"KUBELET_FLAGS", "NETWORK_PLUGIN"},
		},
		{
			name:                        "no diffs yields none",
			vhdBuiltFromSourceUnderTest: false,
			diffVars:                    nil,
			expected:                    nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, unexpectedEnvCompareDiffVars(tc.diffVars, tc.vhdBuiltFromSourceUnderTest))
		})
	}
}

// TestVHDBuiltFromSourceUnderTest asserts the provenance signal matches what
// .pipelines/scripts/e2e_run.sh exports: SIG_VERSION_TAG_NAME=buildId only when VHD_BUILD_ID
// is set, which the VHD builder pipelines set to $(Build.BuildId).
func TestVHDBuiltFromSourceUnderTest(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tagName  string
		expected bool
	}{
		{
			name:     "buildId tag means the VHD came from this pipeline run",
			tagName:  "buildId",
			expected: true,
		},
		{
			name:     "default branch tag means the VHD came from main, not this source",
			tagName:  "branch",
			expected: false,
		},
		{
			name:     "empty tag is not treated as same-source",
			tagName:  "",
			expected: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &config.Configuration{SIGVersionTagName: tc.tagName}
			assert.Equal(t, tc.expected, c.VHDBuiltFromSourceUnderTest())
		})
	}
}

// TestEnvCompareGPUBumpIsTolerated asserts the end-to-end decision for the exact failure that
// blocked the aks-gpu-grid 570.237 bump: on the standalone e2e check the diff parses to only
// VHD-sourced vars, so it must not fail the scenario. The same diff must still fail when the
// VHD was built from the source under test, where the two sides are expected to agree.
func TestEnvCompareGPUBumpIsTolerated(t *testing.T) {
	diffVars := parseEnvCompareDiffVars(realGPUBumpLogLine)
	assert.Equal(t, []string{"GPU_DRIVER_VERSION", "GPU_IMAGE_SHA"}, diffVars)

	assert.Empty(t, unexpectedEnvCompareDiffVars(diffVars, false),
		"a GPU driver version bump must not fail phase 3 when the VHD is built from a different source")

	assert.Equal(t, diffVars, unexpectedEnvCompareDiffVars(diffVars, true),
		"the same diff must still fail once the VHD is built from the source under test")
}
