package main

import (
	"context"
	"fmt"
	"github.com/Azure/agentbaker/aks-node-controller/nodeoperator_os"
	"log/slog"
	"os"
	"path/filepath"
)

// Some options are intentionally non-configurable to avoid customization by users
// it will help us to avoid introducing any breaking changes in the future.
const (
	LogFileName                 = "aks-node-controller.log"
	ReadOnlyWorld   os.FileMode = 0644
	ExecutableWorld os.FileMode = 0755
)

func getLogFile() (*os.File, error) {
	osInfo := nodeoperator_os.GetOperatingSystemInfo()
	logFilePath := osInfo.LogFilePath()

	err := os.MkdirAll(logFilePath, ExecutableWorld)
	if err != nil {
		return nil, err
	}

	logFile := filepath.Join(logFilePath, LogFileName)

	return os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, ReadOnlyWorld)
}

func main() {
	logFile, err := getLogFile()
	if err != nil {
		//nolint:forbidigo // there is no other way to communicate the error
		fmt.Printf("failed to open log file: %s\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(logFile, nil))
	slog.SetDefault(logger)

	app := App{cmdRunner: cmdRunner}
	exitCode := app.Run(context.Background(), os.Args)
	_ = logFile.Close()
	os.Exit(exitCode)
}
