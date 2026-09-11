package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/scenario"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitionScenariosKeepsRegisteredScenariosUnchanged(t *testing.T) {
	scenarios := []*scenario.Scenario{
		{Name: "Excluded"},
		{Name: "Kept"},
	}

	runnable, filtered, err := partitionScenarios(scenarios, tagFilter{skip: "Name=Excluded"})
	require.NoError(t, err)
	require.Len(t, runnable, 1)
	assert.Equal(t, "Kept", runnable[0].Name)
	require.Len(t, filtered, 1)
	assert.Equal(t, "Excluded", filtered[0].Name)
	assert.Equal(t, statusSkipped, filtered[0].Status)
	require.Len(t, filtered[0].Attempts, 1)
	assert.True(t, strings.HasPrefix(filtered[0].Attempts[0].Message, "filtered: "), "filtered reason lost its prefix: %q", filtered[0].Attempts[0].Message)
	for _, scenario := range scenarios {
		assert.Empty(t, scenario.Tags, "filtering mutated the registered scenario")
	}
}

func TestPartitionScenariosAcceptsLegacyTestNameFilter(t *testing.T) {
	scenarios := []*scenario.Scenario{{Name: "AzureLinuxV2"}, {Name: "Ubuntu2204"}}

	runnable, filtered, err := partitionScenarios(scenarios, tagFilter{run: "Name=Test_AzureLinuxV2"})

	require.NoError(t, err)
	require.Len(t, runnable, 1)
	assert.Equal(t, "AzureLinuxV2", runnable[0].Name)
	require.Len(t, filtered, 1)
	assert.Equal(t, "Ubuntu2204", filtered[0].Name)
}

func TestPartitionScenariosRejectsInvalidFilters(t *testing.T) {
	scenarios := []*scenario.Scenario{{Name: "Only"}}
	for _, filter := range []tagFilter{{run: "not-a-pair"}, {skip: "unknownKey=true"}} {
		_, _, err := partitionScenarios(scenarios, filter)
		require.Error(t, err, "invalid filter %+v was accepted", filter)
	}
}

// azureInitProbe returns a value that changes whenever config.Initialize runs.
func azureInitProbe() string {
	return config.VMSSHPrivateKeyFileName
}

func TestAppFailsBeforeInitializationWhenFiltersMatchNothing(t *testing.T) {
	restoreRunnerConfig(t)
	before := azureInitProbe()
	junitFile := filepath.Join(t.TempDir(), "report.xml")

	var stderr bytes.Buffer
	app := NewApp(&bytes.Buffer{}, &stderr)
	code := app.Run(context.Background(), []string{
		"e2e", "run", "--log-dir", t.TempDir(), "--junit-file", junitFile, "--tags", "Name=DoesNotExist", "Ubuntu2204_CustomLinuxOSConfig_Taints_ANC",
	})

	assert.Equal(t, exitUsage, code, "stderr: %s", stderr.String())
	assert.Contains(t, stderr.String(), "no scenarios matched the configured filters")
	assert.Equal(t, before, azureInitProbe(), "configuration was initialized before the filters were evaluated")
	report, err := os.ReadFile(junitFile)
	require.NoError(t, err)
	assert.Contains(t, string(report), `<skipped message="filtered:`, "JUnit report dropped the filtered scenario")
}

func TestAppFailsFastOnInvalidTagFilter(t *testing.T) {
	restoreRunnerConfig(t)
	before := azureInitProbe()

	var stderr bytes.Buffer
	app := NewApp(&bytes.Buffer{}, &stderr)
	code := app.Run(context.Background(), []string{
		"e2e", "run", "--log-dir", t.TempDir(), "--tags", "not-a-pair", "Ubuntu2204_CustomLinuxOSConfig_Taints_ANC",
	})

	assert.Equal(t, exitFailure, code, "stderr: %s", stderr.String())
	assert.Contains(t, stderr.String(), "invalid filter format")
	assert.Equal(t, before, azureInitProbe(), "configuration was initialized before the filters were validated")
}

func restoreRunnerConfig(t *testing.T) {
	t.Helper()
	saved := *config.Config
	t.Cleanup(func() { *config.Config = saved })
}

func TestMatchFiltersPreservesTagPolicy(t *testing.T) {
	tags := scenario.Tags{Name: "Ubuntu2204", OS: "linux", GPU: true}
	for _, test := range []struct {
		filter string
		all    bool
		match  bool
	}{
		{filter: "Name=Other,Name=Test_Ubuntu2204", all: true, match: true},
		{filter: "Name=Ubuntu2204,GPU=false", all: true, match: false},
		{filter: "Name=Ubuntu2204,GPU=false", all: false, match: true},
		{filter: " os = LINUX , gpu = true ", all: true, match: true},
		{filter: "", all: true, match: true},
	} {
		t.Run(test.filter, func(t *testing.T) {
			got, err := matchFilters(tags, test.filter, test.all)
			require.NoError(t, err)
			assert.Equal(t, test.match, got)
		})
	}
	_, err := matchFilters(tags, "GPU=invalid", true)
	require.ErrorContains(t, err, "invalid boolean")
}
