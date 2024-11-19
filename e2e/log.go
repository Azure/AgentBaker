package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

func testDir(t *testing.T) string {
	return filepath.Join(config.Config.E2ELoggingDir, t.Name())
}

func cleanTestDir(t *testing.T) {
	dirPath := testDir(t)
	err := os.RemoveAll(dirPath)
	require.NoError(t, err)
}

func writeToFile(t *testing.T, fileName, content string) error {
	dirPath := testDir(t)
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	fullPath := filepath.Join(dirPath, fileName)
	return os.WriteFile(fullPath, []byte(content), 0644)
}

func dumpFileMapToDir(t *testing.T, files map[string]string) error {
	for fileName, contents := range files {
		fileName = filepath.Base(fileName)
		if err := writeToFile(t, fileName, contents); err != nil {
			return err
		}
	}

	return nil
}
