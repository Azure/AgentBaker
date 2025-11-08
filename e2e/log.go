package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
)

func testDir(t testing.TB) string {
	return filepath.Join(config.Config.E2ELoggingDir, t.Name())
}

func writeToFile(t testing.TB, fileName, content string) error {
	dirPath := testDir(t)
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	fullPath := filepath.Join(dirPath, fileName)
	return os.WriteFile(fullPath, []byte(content), 0644)
}

func dumpFileMapToDir(t testing.TB, files map[string]string) error {
	for fileName, contents := range files {
		fileName = filepath.Base(fileName)
		if err := writeToFile(t, fileName, contents); err != nil {
			return err
		}
	}

	return nil
}
