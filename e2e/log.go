package e2e_test

import (
	"bufio"
	"os"
	"path/filepath"
)

const (
	clusterParamsDir = "cluster-parameters"
	vmLogsDir        = "vm-logs"
)

func createDirIfNeeded(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

func createClusterParamsDir() error {
	return createDirIfNeeded(clusterParamsDir)
}

func createVMLogsDir(caseName string) (string, error) {
	logDir := filepath.Join(vmLogsDir, caseName)
	return logDir, createDirIfNeeded(logDir)
}

func writeToFile(fileName, content string) error {
	outputFile, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	w := bufio.NewWriter(outputFile)

	if _, err := w.Write([]byte(content)); err != nil {
		return err
	}

	return nil
}

func dumpFileMapToDir(baseDir string, files map[string]string) error {
	for path, contents := range files {
		fullPath := filepath.Join(baseDir, path)
		if err := writeToFile(fullPath, contents); err != nil {
			return err
		}
	}

	return nil
}
