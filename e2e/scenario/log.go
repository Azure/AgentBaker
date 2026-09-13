package scenario

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/Azure/agentbaker/e2e/config"
)

func collectCommandLogs(ctx context.Context, artifactName string, commands map[string]string, exec func(context.Context, string) (*podExecResult, error)) error {
	files := slices.Sorted(maps.Keys(commands))
	errs := make([]error, len(files))
	var group sync.WaitGroup
	for i, file := range files {
		group.Go(func() {
			errs[i] = collectCommandLog(ctx, artifactName, file, commands[file], exec)
		})
	}
	group.Wait()
	return errors.Join(errs...)
}

func collectCommandLog(ctx context.Context, artifactName, file, command string, exec func(context.Context, string) (*podExecResult, error)) error {
	var content string
	err := runWithPanicRecovery(ctx, func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		result, err := exec(ctx, command)
		if err != nil {
			return err
		}
		content = result.String()
		return nil
	})
	var collectionErr error
	if err != nil {
		content = fmt.Sprintf("log collection failed: %v\n", err)
		collectionErr = fmt.Errorf("could not collect %s; see artifact for details", file)
	}
	if err := writeToFile(artifactName, file, content); err != nil {
		return errors.Join(collectionErr, fmt.Errorf("write log %s: %w", file, err))
	}
	return collectionErr
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
