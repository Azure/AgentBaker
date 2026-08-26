package e2e

import (
	"os"
	"path/filepath"

	"github.com/Azure/agentbaker/e2e/config"
)

// testDir is the directory holding the artifacts of the test named testName.
func testDir(testName string) string {
	return filepath.Join(config.Config.E2ELoggingDir, testName)
}

func writeToFile(testName, fileName, content string) error {
	dirPath := testDir(testName)
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	fullPath := filepath.Join(dirPath, fileName)
	return os.WriteFile(fullPath, []byte(content), 0600)
}

func dumpFileMapToDir(testName string, files map[string]string) error {
	for fileName, contents := range files {
		fileName = filepath.Base(fileName)
		if err := writeToFile(testName, fileName, contents); err != nil {
			return err
		}
	}

	return nil
}
