package scenario

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCARotationScriptsIncludeRegistryDependencies(t *testing.T) {
	scripts, err := caRotationScripts()
	require.NoError(t, err)
	dir := t.TempDir()
	for _, script := range scripts {
		require.NotEmpty(t, script.content, script.name)
		require.NoError(t, os.WriteFile(filepath.Join(dir, script.name), script.content, 0600))
	}
	_, err = os.Stat(filepath.Join(dir, "init-aks-cloud.sh"))
	require.NoError(t, err)

	// Source definitions only: no node configuration or runtime actions.
	cmd := exec.CommandContext(t.Context(), "bash", "-ec", `
. "$1/cse_config.sh"
declare -F configureContainerdRegistryHost >/dev/null
declare -F configureContainerdLegacyMooncakeMcrHost >/dev/null
`, "fixture-source", dir)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
}

func TestCARotationBuildResolvesFromModuleAndScenarioDirectories(t *testing.T) {
	root, err := findRepoRoot()
	require.NoError(t, err)
	binary := filepath.Join(t.TempDir(), "fixture")
	for _, directory := range []string{"e2e", filepath.Join("e2e", "scenario")} {
		t.Run(directory, func(t *testing.T) {
			t.Chdir(filepath.Join(root, directory))
			cmd, err := caRotationBuildCommand(context.Background(), binary)
			require.NoError(t, err)
			require.Equal(t, filepath.Join(root, "e2e"), cmd.Dir)
			require.Equal(t, []string{"go", "build", "-o", binary, "./cmd/ca-rotation-fixture"}, cmd.Args)
			require.Contains(t, cmd.Env, "GOOS=linux")
			require.Contains(t, cmd.Env, "GOARCH=amd64")
			require.Contains(t, cmd.Env, "CGO_ENABLED=0")
			_, err = os.Stat(filepath.Join(cmd.Dir, "cmd", "ca-rotation-fixture", "main.go"))
			require.NoError(t, err)
		})
	}
}
