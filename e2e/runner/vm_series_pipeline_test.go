package runner

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNormalPipelineDoesNotEnableVMSeries(t *testing.T) {
	data, err := os.ReadFile("../../.pipelines/e2e.yaml")
	require.NoError(t, err)
	var pipeline struct {
		Variables map[string]any `yaml:"variables"`
	}
	require.NoError(t, yaml.Unmarshal(data, &pipeline))
	for _, key := range []string{"DEFAULT_VM_SKU", "TAGS_TO_RUN", "ADHOC_SKU_VALIDATION", "VM_SERIES_COVERAGE", "E2E_PARALLEL", "E2E_GO_TEST_TIMEOUT"} {
		require.NotContains(t, pipeline.Variables, key, "normal pipeline inherited an ad-hoc/series override")
	}
}
