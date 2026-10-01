package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryFilesResolveFromModuleAndScenarioDirectories(t *testing.T) {
	root, err := findRepoRoot()
	require.NoError(t, err)
	for _, directory := range []string{"e2e", filepath.Join("e2e", "scenario")} {
		t.Run(directory, func(t *testing.T) {
			t.Chdir(filepath.Join(root, directory))
			resolved, err := findRepoRoot()
			require.NoError(t, err)
			assert.Equal(t, root, resolved)
			settings, err := getWindowsSettingsJson()
			require.NoError(t, err)
			require.True(t, json.Valid(settings))
			assert.Contains(t, string(settings), "WindowsBaseVersions")
			_, err = os.Stat(filepath.Join(resolved, "aks-node-controller", "go.mod"))
			require.NoError(t, err)
		})
	}
}
