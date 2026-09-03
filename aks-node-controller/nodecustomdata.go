package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultNodeCustomDataPath = "/opt/azure/containers/nodecustomdata.yml"
	encodingGZIP              = "gzip"
	encodingBase64            = "base64"
)

type nodeCustomDataWriteFile struct {
	Path        string `yaml:"path"`
	Permissions string `yaml:"permissions"`
	Encoding    string `yaml:"encoding,omitempty"`
	Owner       string `yaml:"owner"`
	Content     string `yaml:"content"`
}

type nodeCustomData struct {
	WriteFiles []nodeCustomDataWriteFile `yaml:"write_files"`
}

type nodeCustomDataApplyOptions struct {
	source             string
	strict             bool
	replaceOnly        bool
	requirePermissions bool
	rejectUnsafePaths  bool
	rejectEmptyContent bool
}

type nodeCustomDataApplyResult struct {
	Applied int
	Skipped int
}

type nodeCustomDataEntry struct {
	destination string
	mode        os.FileMode
	content     []byte
}

type stagedNodeCustomDataEntry struct {
	destination    string
	stagedPath     string
	backupPath     string
	originalExist  bool
	preserveBackup bool
}

func applyNodeCustomData(nodeCustomDataPath string) error {
	data, err := os.ReadFile(nodeCustomDataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read nodecustomdata %s: %w", nodeCustomDataPath, err)
	}

	if _, err := applyNodeCustomDataPayload(data, nodeCustomDataApplyOptions{source: nodeCustomDataPath}); err != nil {
		return fmt.Errorf("apply nodecustomdata %s: %w", nodeCustomDataPath, err)
	}
	return nil
}

func applyNodeCustomDataPayload(data []byte, options nodeCustomDataApplyOptions) (nodeCustomDataApplyResult, error) {
	customData, err := parseNodeCustomData(data, options)
	if err != nil {
		return nodeCustomDataApplyResult{}, err
	}

	entries, err := validateNodeCustomData(customData, options)
	if err != nil {
		return nodeCustomDataApplyResult{}, err
	}

	result := nodeCustomDataApplyResult{}
	var staged []*stagedNodeCustomDataEntry
	for _, entry := range entries {
		pending, changed, err := stageNodeCustomDataEntry(entry, options.replaceOnly)
		if err != nil {
			cleanupStagedNodeCustomData(staged)
			return result, fmt.Errorf("stage nodecustomdata destination %s: %w", entry.destination, err)
		}
		if !changed {
			result.Skipped++
			continue
		}
		staged = append(staged, &pending)
	}

	if err := commitStagedNodeCustomData(staged); err != nil {
		return nodeCustomDataApplyResult{}, err
	}
	result.Applied = len(staged)
	return result, nil
}

func parseNodeCustomData(data []byte, options nodeCustomDataApplyOptions) (nodeCustomData, error) {
	var customData nodeCustomData
	if !options.strict {
		if err := yaml.Unmarshal(data, &customData); err != nil {
			return nodeCustomData{}, fmt.Errorf("unmarshal nodecustomdata %s: %w", options.source, err)
		}
		return customData, nil
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&customData); err != nil {
		return nodeCustomData{}, fmt.Errorf("decode nodecustomdata %s: %w", options.source, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nodeCustomData{}, fmt.Errorf("nodecustomdata %s has trailing content", options.source)
	}
	return customData, nil
}

func validateNodeCustomData(customData nodeCustomData, options nodeCustomDataApplyOptions) ([]nodeCustomDataEntry, error) {
	entries := make([]nodeCustomDataEntry, 0, len(customData.WriteFiles))
	destinations := make(map[string]struct{}, len(customData.WriteFiles))
	for index, file := range customData.WriteFiles {
		entry, err := validateNodeCustomDataWriteFile(file, options)
		if err != nil {
			return nil, fmt.Errorf("validate write_files entry %d: %w", index, err)
		}
		if options.strict {
			if _, exists := destinations[entry.destination]; exists {
				return nil, fmt.Errorf("duplicate destination %s", entry.destination)
			}
			destinations[entry.destination] = struct{}{}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func validateNodeCustomDataWriteFile(file nodeCustomDataWriteFile, options nodeCustomDataApplyOptions) (nodeCustomDataEntry, error) {
	if file.Path == "" {
		return nodeCustomDataEntry{}, fmt.Errorf("path is required")
	}
	if options.rejectUnsafePaths {
		if (!strings.HasPrefix(file.Path, "/") && !filepath.IsAbs(file.Path)) ||
			(strings.HasPrefix(file.Path, "/") && path.Clean(file.Path) != file.Path) ||
			(!strings.HasPrefix(file.Path, "/") && filepath.Clean(file.Path) != file.Path) {
			return nodeCustomDataEntry{}, fmt.Errorf("unsafe destination %q", file.Path)
		}
		if strings.HasPrefix(file.Path, "/") && strings.Contains(file.Path, `\`) {
			return nodeCustomDataEntry{}, fmt.Errorf("unsafe destination %q: backslashes are not allowed", file.Path)
		}
	}
	if file.Owner != "" && file.Owner != "root" {
		return nodeCustomDataEntry{}, fmt.Errorf("unsupported owner %q", file.Owner)
	}

	mode, err := parseNodeCustomDataMode(file.Permissions, options.requirePermissions)
	if err != nil {
		return nodeCustomDataEntry{}, err
	}
	content, err := decodeNodeCustomDataWriteFileContent(file)
	if err != nil {
		return nodeCustomDataEntry{}, err
	}
	if options.rejectEmptyContent && len(content) == 0 {
		return nodeCustomDataEntry{}, fmt.Errorf("content for %s is empty", file.Path)
	}
	return nodeCustomDataEntry{destination: file.Path, mode: mode, content: content}, nil
}

func parseNodeCustomDataMode(value string, required bool) (os.FileMode, error) {
	if value == "" && !required {
		return 0o644, nil
	}
	parsed, err := strconv.ParseUint(value, 8, 32)
	if err != nil {
		if required {
			return 0, fmt.Errorf("invalid mode %q", value)
		}
		return 0, fmt.Errorf("parse permissions: %w", err)
	}
	if required && (parsed == 0 || parsed > 0o777) {
		return 0, fmt.Errorf("invalid mode %q", value)
	}
	return os.FileMode(parsed), nil
}

func decodeNodeCustomDataWriteFileContent(file nodeCustomDataWriteFile) ([]byte, error) {
	switch file.Encoding {
	case "":
		return []byte(file.Content), nil
	case encodingGZIP:
		reader, err := gzip.NewReader(bytes.NewReader([]byte(file.Content)))
		if err != nil {
			return nil, fmt.Errorf("create gzip reader for %s: %w", file.Path, err)
		}
		defer reader.Close()

		decoded, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read gzip content for %s: %w", file.Path, err)
		}
		return decoded, nil
	case encodingBase64:
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return nil, fmt.Errorf("decode base64 content for %s: %w", file.Path, err)
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", file.Encoding)
	}
}

func stageNodeCustomDataEntry(entry nodeCustomDataEntry, replaceOnly bool) (stagedNodeCustomDataEntry, bool, error) {
	current, err := os.ReadFile(entry.destination)
	originalExists := err == nil
	var originalMode os.FileMode
	switch {
	case err == nil:
		info, statErr := os.Stat(entry.destination)
		if statErr != nil {
			return stagedNodeCustomDataEntry{}, false, fmt.Errorf("stat destination: %w", statErr)
		}
		originalMode = info.Mode().Perm()
		if bytes.Equal(current, entry.content) && originalMode == entry.mode.Perm() {
			return stagedNodeCustomDataEntry{}, false, nil
		}
	case os.IsNotExist(err):
		if replaceOnly {
			return stagedNodeCustomDataEntry{}, false, nil
		}
	default:
		return stagedNodeCustomDataEntry{}, false, fmt.Errorf("read destination: %w", err)
	}

	directory := filepath.Dir(entry.destination)
	if originalExists {
		info, statErr := os.Stat(directory)
		if statErr != nil {
			return stagedNodeCustomDataEntry{}, false, fmt.Errorf("stat destination directory %s: %w", directory, statErr)
		}
		if !info.IsDir() {
			return stagedNodeCustomDataEntry{}, false, fmt.Errorf("destination parent %s is not a directory", directory)
		}
	} else if mkErr := os.MkdirAll(directory, 0o755); mkErr != nil {
		return stagedNodeCustomDataEntry{}, false, fmt.Errorf("create parent directory: %w", mkErr)
	}

	stagedPath, err := writeNodeCustomDataTempFile(directory, ".aks-node-controller-nodecustomdata-stage-*", entry.content, entry.mode)
	if err != nil {
		return stagedNodeCustomDataEntry{}, false, err
	}
	staged := stagedNodeCustomDataEntry{
		destination:   entry.destination,
		stagedPath:    stagedPath,
		originalExist: originalExists,
	}
	if !originalExists {
		return staged, true, nil
	}

	backupPath, err := writeNodeCustomDataTempFile(
		directory,
		".aks-node-controller-nodecustomdata-backup-*",
		current,
		originalMode,
	)
	if err != nil {
		_ = os.Remove(stagedPath)
		return stagedNodeCustomDataEntry{}, false, fmt.Errorf("stage destination backup: %w", err)
	}
	staged.backupPath = backupPath
	return staged, true, nil
}

func writeNodeCustomDataTempFile(directory, pattern string, content []byte, mode os.FileMode) (string, error) {
	temp, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}
	if _, err := temp.Write(content); err != nil {
		cleanup()
		return "", fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return "", fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		cleanup()
		return "", fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		cleanup()
		return "", fmt.Errorf("chmod temporary file: %w", err)
	}
	return tempPath, nil
}

func commitStagedNodeCustomData(staged []*stagedNodeCustomDataEntry) error {
	return commitStagedNodeCustomDataWithRename(staged, os.Rename)
}

func commitStagedNodeCustomDataWithRename(
	staged []*stagedNodeCustomDataEntry,
	rename func(string, string) error,
) error {
	committed := 0
	defer cleanupStagedNodeCustomData(staged)
	for index, entry := range staged {
		if err := rename(entry.stagedPath, entry.destination); err != nil {
			rollbackErr := rollbackStagedNodeCustomData(staged[:committed], rename)
			if rollbackErr != nil {
				return fmt.Errorf(
					"commit nodecustomdata destination %s: %w; rollback failed: %w",
					entry.destination,
					err,
					rollbackErr,
				)
			}
			return fmt.Errorf("commit nodecustomdata destination %s: %w", entry.destination, err)
		}
		staged[index].stagedPath = ""
		committed++
	}
	return nil
}

func rollbackStagedNodeCustomData(
	committed []*stagedNodeCustomDataEntry,
	rename func(string, string) error,
) error {
	var rollbackErrors []error
	for index := len(committed) - 1; index >= 0; index-- {
		entry := committed[index]
		var err error
		if entry.originalExist {
			err = rename(entry.backupPath, entry.destination)
			if err == nil {
				entry.backupPath = ""
			}
		} else {
			err = os.Remove(entry.destination)
		}
		if err != nil {
			entry.preserveBackup = true
			rollbackErrors = append(
				rollbackErrors,
				fmt.Errorf(
					"restore %s from preserved backup %s: %w",
					entry.destination,
					entry.backupPath,
					err,
				),
			)
		}
	}
	return errors.Join(rollbackErrors...)
}

func cleanupStagedNodeCustomData(staged []*stagedNodeCustomDataEntry) {
	for _, entry := range staged {
		if entry.stagedPath != "" {
			_ = os.Remove(entry.stagedPath)
		}
		if entry.backupPath != "" && !entry.preserveBackup {
			_ = os.Remove(entry.backupPath)
		}
	}
}
