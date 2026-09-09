package e2e

import (
	"os"
	"path/filepath"

	"github.com/Azure/agentbaker/e2e/config"
)

// artifactDir is the directory holding artifacts from one scenario run.
func artifactDir(artifactName string) string {
	return filepath.Join(config.Config.E2ELoggingDir, artifactName)
}

func writeToFile(artifactName, fileName, content string) error {
	dirPath := artifactDir(artifactName)
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	fullPath := filepath.Join(dirPath, fileName)
	return os.WriteFile(fullPath, []byte(content), 0600)
}

func dumpFileMapToDir(artifactName string, files map[string]string) error {
	for fileName, contents := range files {
		fileName = filepath.Base(fileName)
		if err := writeToFile(artifactName, fileName, contents); err != nil {
			return err
		}
	}

	return nil
}
