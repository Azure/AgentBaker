package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	e2eLogsDir = "scenario-logs"
)

func writeToFile(t *testing.T, fileName, content string) error {
	dirPath := filepath.Join(e2eLogsDir, t.Name())

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
