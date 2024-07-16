package e2e

import (
	"bufio"
	"os"
	"path/filepath"

	"github.com/Azure/agentbakere2e/config"
)

func createDirIfNeeded(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

func createE2ELoggingDir() error {
	return createDirIfNeeded(config.E2ELoggingDir)
}

func createScenarioLogsDir(name string) (string, error) {
	logDir := filepath.Join(config.E2ELoggingDir, name)
	return logDir, createDirIfNeeded(logDir)
}

func writeToFile(fileName, content string) error {
	outputFile, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	w := bufio.NewWriter(outputFile)
	defer w.Flush()

	contentBytes := []byte(content)

	if _, err := w.Write(contentBytes); err != nil {
		return err
	}

	return nil
}

func dumpFileMapToDir(baseDir string, files map[string]string) error {
	for path, contents := range files {
		path = filepath.Base(path)
		fullPath := filepath.Join(baseDir, path)
		if err := writeToFile(fullPath, contents); err != nil {
			return err
		}
	}

	return nil
}
