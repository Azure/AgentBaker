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
	app := App{
		cmdRun: cmdRunner,
	}
	exitCode := app.Run(context.Background(), os.Args)
	os.Exit(exitCode)
}

func configureLogging(logPath string) func() {
	resolvedPath := resolveLogPath(logPath)

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0755); err != nil {
		//nolint:forbidigo // there is no other way to communicate the error
		fmt.Printf("failed to create log directory: %s\n", err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(resolvedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

func resolveLogPath(logPath string) string {
	// Try to create the requested log directory; fall back to current directory on failure.
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err == nil {
		return logPath
	}
	return "aks-node-controller.log"
}
