package scenario

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"golang.org/x/crypto/ssh"
)

const logCollectionConcurrency = 8

func collectCommandLogs(ctx context.Context, artifactName string, commands map[string]string, exec func(context.Context, string) (*podExecResult, error)) error {
	batch := &scenarioCleanup{}
	slots := make(chan struct{}, logCollectionConcurrency)
	for file, command := range commands {
		batch.add(func(ctx context.Context) error {
			slots <- struct{}{}
			defer func() { <-slots }()

			var result *podExecResult
			err := runCleanup(ctx, func(ctx context.Context) error {
				for attempt := 0; ; attempt++ {
					if err := ctx.Err(); err != nil {
						return err
					}
					var err error
					result, err = exec(ctx, command)
					var rejected *ssh.OpenChannelError
					if attempt >= 4 || !errors.As(err, &rejected) || rejected.Reason != ssh.ConnectionFailed {
						return err
					}
					select {
					case <-time.After(200 * time.Millisecond):
					case <-ctx.Done():
						return errors.Join(ctx.Err(), err)
					}
				}
			})
			var content string
			var collectionErr error
			if err != nil {
				content = fmt.Sprintf("log collection failed: %v\n", err)
				collectionErr = fmt.Errorf("could not collect %s; see artifact for details", file)
			} else {
				content = result.String()
			}
			if err := writeToFile(artifactName, file, content); err != nil {
				return errors.Join(collectionErr, fmt.Errorf("write log %s: %w", file, err))
			}
			return collectionErr
		})
	}
	return batch.runCleanups(ctx)
}

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
