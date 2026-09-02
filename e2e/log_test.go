package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactHelpersUseTheGivenArtifactName(t *testing.T) {
	loggingDir := t.TempDir()
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = loggingDir
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })

	const artifactName = "TestScenario/vhd-provision"

	assert.Equal(t, filepath.Join(loggingDir, artifactName), artifactDir(artifactName))

	require.NoError(t, writeToFile(artifactName, "single.log", "single"))
	require.NoError(t, dumpFileMapToDir(artifactName, map[string]string{"/var/log/nested/mapped.log": "mapped"}))

	for name, want := range map[string]string{"single.log": "single", "mapped.log": "mapped"} {
		got, err := os.ReadFile(filepath.Join(artifactDir(artifactName), name))
		require.NoError(t, err, "read %s", name)
		assert.Equal(t, want, string(got), name)
	}
}

func TestGenerateVMSSNameLinuxUsesTheGivenArtifactName(t *testing.T) {
	name := generateVMSSNameLinux("TestScenario_Ubuntu2204/vhd_caching")

	assert.LessOrEqual(t, len(name), 57, "name %q exceeds the VMSS limit", name)
	assert.NotContains(t, name, "_")
	assert.NotContains(t, name, "/")
	assert.NotContains(t, name, "Test")
	assert.Equal(t, strings.ToLower(name), name, "name is not lowercase")
	assert.Contains(t, name, "scenarioubuntu2204", "name does not carry the test name")
}
