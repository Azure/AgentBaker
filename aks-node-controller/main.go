package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// Some options are intentionally non-configurable to avoid customization by users
// it will help us to avoid introducing any breaking changes in the future.
const (
	LogFile = "/var/log/azure/node-bootstrapper.log"
)

func main() {
	logFile, err := os.OpenFile(LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
