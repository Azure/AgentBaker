package main

import (
	"context"
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

func configureLogging(logPath string) {
	logFile, err := openLogFile(logPath)
	if err != nil {
		// Fall back to stderr-only logging if we can't open any log file.
		slog.Warn("failed to open log file, logging to stderr only", "error", err)
		return
	}
	mw := io.MultiWriter(logFile, os.Stderr)
	logger := slog.New(slog.NewJSONHandler(mw, nil))
	slog.SetDefault(logger)
}

// openLogFile tries to open logPath, falling back to a local file if the path is not writable.
func openLogFile(logPath string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err == nil {
		if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			return f, nil
		}
	}
	// Fall back to current directory.
	fallback := "aks-node-controller.log"
	return os.OpenFile(fallback, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}
