package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func main() {
	// defer calls are not executed on os.Exit
	logCleanup := configureLogging()
	app := App{cmdRunner: cmdRunner}
	exitCode := app.Run(context.Background(), os.Args)
	logCleanup()
	os.Exit(exitCode)
}

func configureLogging() func() {
	logPath := setupLogPath()

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		//nolint:forbidigo // there is no other way to communicate the error
		fmt.Printf("failed to create log directory: %s\n", err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		//nolint:forbidigo // there is no other way to communicate the error
		fmt.Printf("failed to open log file: %s\n", err)
		os.Exit(1)
	}
	mw := io.MultiWriter(logFile, os.Stderr)
	logger := slog.New(slog.NewJSONHandler(mw, nil))
	slog.SetDefault(logger)
	return func() {
		err := logFile.Close()
		if err != nil {
			// stdout is important, don't pollute with non-important warnings
			_, _ = fmt.Fprintf(os.Stderr, "failed to close log file: %s\n", err)
		}
	}
}

func setupLogPath() string {
	// Try to create production directory first
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err == nil {
		return logPath
	}
	// If directory creation fails, fallback to current directory
	return "aks-node-controller.log"
}
