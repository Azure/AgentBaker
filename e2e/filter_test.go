package e2e

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitionScenariosKeepsRegisteredScenariosUnchanged(t *testing.T) {
	scenarios := []*Scenario{
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

func TestPartitionScenariosRejectsInvalidFilters(t *testing.T) {
	scenarios := []*Scenario{{Name: "Only"}}
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
		"e2e", "run", "--log-dir", t.TempDir(), "--junit-file", junitFile, "--tags", "Name=DoesNotExist", "Ubuntu2204",
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
		"e2e", "run", "--log-dir", t.TempDir(), "--tags", "not-a-pair", "Ubuntu2204",
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
