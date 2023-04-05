package main

import (
	"bufio"
	"os"
	"path/filepath"
)

const (
	e2eLogsDir = "scenario-logs"
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
	return createDirIfNeeded(e2eLogsDir)
}

func createVMLogsDir(caseName string) (string, error) {
	logDir := filepath.Join(e2eLogsDir, caseName)
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
