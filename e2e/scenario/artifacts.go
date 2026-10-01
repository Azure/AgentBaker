package scenario

import (
	"archive/tar"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
)

//go:embed collect_logs.sh
var collectLogsScript string

func collectCommandLogs(ctx context.Context, artifactName string, commands map[string]string, exec func(context.Context, string) (*podExecResult, error)) error {
	files := slices.Sorted(maps.Keys(commands))
	if len(files) == 0 {
		return nil
	}
	var result *podExecResult
	batchErr := runWithPanicRecovery(ctx, func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		result, err = exec(ctx, logCollectionCommand(ctx, files, commands))
		return err
	})
	var entries map[string]string
	if result != nil {
		if result.exitCode != "0" {
			batchErr = errors.Join(batchErr, fmt.Errorf("log collection script exited with code %s: %s", result.exitCode, result.stderr))
		}
		var archiveErr error
		entries, archiveErr = readLogArchive(result.stdout, len(files))
		batchErr = errors.Join(batchErr, archiveErr)
	} else if batchErr == nil {
		batchErr = errors.New("log collection returned no result")
	}

	var errs []error
	for i, file := range files {
		prefix := strconv.Itoa(i)
		stdout, stdoutOK := entries[prefix+".stdout"]
		stderr, stderrOK := entries[prefix+".stderr"]
		exitCode, exitOK := entries[prefix+".exit"]
		collectionErr := batchErr
		if !stdoutOK || !stderrOK || !exitOK {
			collectionErr = errors.Join(collectionErr, errors.New("incomplete diagnostic archive"))
		}
		if code, err := strconv.Atoi(exitCode); err != nil || code < 0 || code > 255 {
			collectionErr = errors.Join(collectionErr, fmt.Errorf("invalid or missing diagnostic exit code %q", exitCode))
		} else if code == 124 || code == 137 {
			collectionErr = errors.Join(collectionErr, fmt.Errorf("diagnostic command timed out or was killed (exit %s)", exitCode))
		}
		content := (&podExecResult{exitCode: exitCode, stdout: stdout, stderr: stderr}).String()
		if collectionErr != nil {
			content = fmt.Sprintf("log collection failed: %v\n%s", collectionErr, content)
			errs = append(errs, fmt.Errorf("could not collect %s; see artifact for details", file))
		}
		if err := writeToFile(artifactName, file, content); err != nil {
			errs = append(errs, fmt.Errorf("write log %s: %w", file, err))
		}
	}
	return errors.Join(errs...)
}

func logCollectionCommand(ctx context.Context, files []string, commands map[string]string) string {
	duration, killAfter := 3*time.Minute, 5*time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := max(time.Until(deadline), time.Millisecond)
		duration = min(duration, remaining*3/4)
		killAfter = min(killAfter, remaining/8)
	}
	var script strings.Builder
	fmt.Fprintf(&script, "set -- %.9f %.9f", duration.Seconds(), killAfter.Seconds())
	for _, file := range files {
		fmt.Fprintf(&script, " '%s'", strings.ReplaceAll(commands[file], "'", "'\"'\"'"))
	}
	script.WriteByte('\n')
	script.WriteString(collectLogsScript)
	encoded := base64.StdEncoding.EncodeToString([]byte(script.String()))
	return `bash -c "$(printf %s '` + encoded + `' | base64 -d)"`
}

func readLogArchive(data string, fileCount int) (map[string]string, error) {
	entries := make(map[string]string, 3*fileCount)
	reader, err := gzip.NewReader(strings.NewReader(data))
	if err != nil {
		return entries, fmt.Errorf("read diagnostic archive: %w", err)
	}
	defer reader.Close()
	archive := tar.NewReader(reader)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			_, err = io.Copy(io.Discard, reader)
			if err != nil {
				return entries, fmt.Errorf("verify diagnostic archive: %w", err)
			}
			return entries, nil
		}
		if err != nil {
			return entries, fmt.Errorf("read diagnostic archive entry: %w", err)
		}
		if header.Typeflag == tar.TypeDir && header.Name == "./" {
			continue
		}
		name := strings.TrimPrefix(header.Name, "./")
		index, suffix, _ := strings.Cut(name, ".")
		number, err := strconv.Atoi(index)
		if header.Typeflag != tar.TypeReg || err != nil || number < 0 || number >= fileCount ||
			index != strconv.Itoa(number) || (suffix != "stdout" && suffix != "stderr" && suffix != "exit") {
			return entries, fmt.Errorf("unexpected diagnostic archive entry %q", header.Name)
		}
		if _, exists := entries[name]; exists {
			return entries, fmt.Errorf("duplicate diagnostic archive entry %q", header.Name)
		}
		content, err := io.ReadAll(archive)
		entries[name] = string(content)
		if err != nil {
			return entries, fmt.Errorf("read diagnostic archive entry %q: %w", name, err)
		}
	}
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
