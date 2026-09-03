// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

// Package scripthotfix applies rendered provisioning script hotfixes embedded
// in the aks-node-controller binary.
package scripthotfix

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultOSReleasePath = "/etc/os-release"

type Platform string

const (
	PlatformUbuntu     Platform = "ubuntu"
	PlatformMariner    Platform = "mariner"
	PlatformACL        Platform = "acl"
	PlatformAzlOSGuard Platform = "azlosguard"
	PlatformFlatcar    Platform = "flatcar"
)

//go:embed generated
var embeddedGeneratedFiles embed.FS

var generatedFiles fs.FS = embeddedGeneratedFiles

type nodeCustomData struct {
	WriteFiles []writeFile `yaml:"write_files"`
}

type writeFile struct {
	Path        string `yaml:"path"`
	Permissions string `yaml:"permissions"`
	Encoding    string `yaml:"encoding,omitempty"`
	Owner       string `yaml:"owner"`
	Content     string `yaml:"content"`
}

type payloadEntry struct {
	destination string
	mode        os.FileMode
	content     []byte
}

type Result struct {
	Applied int
	Skipped int
}

type stagedEntry struct {
	destination    string
	stagedPath     string
	backupPath     string
	originalExist  bool
	preserveBackup bool
}

// ApplyEmbedded applies the rendered payload compiled into this ANC binary.
func ApplyEmbedded(osReleasePath string) (Result, error) {
	active, err := fs.ReadFile(generatedFiles, "generated/active")
	if err != nil {
		return Result{}, fmt.Errorf("read embedded hotfix state: %w", err)
	}
	if strings.TrimSpace(string(active)) != "true" {
		return Result{}, nil
	}
	if osReleasePath == "" {
		osReleasePath = defaultOSReleasePath
	}
	platform, err := ClassifyPlatform(osReleasePath)
	if err != nil {
		return Result{}, err
	}
	return applyFS(generatedFiles, platform)
}

// ClassifyPlatform maps /etc/os-release to rendered nodecustomdata variants.
func ClassifyPlatform(osReleasePath string) (Platform, error) {
	data, err := os.ReadFile(osReleasePath)
	if err != nil {
		return "", fmt.Errorf("read OS release %s: %w", osReleasePath, err)
	}
	values := parseOSRelease(data)
	id := strings.ToLower(values["ID"])
	variant := strings.ToLower(values["VARIANT_ID"])

	switch {
	case variant == "osguard":
		return PlatformAzlOSGuard, nil
	case variant == "azurecontainerlinux", id == "azurecontainerlinux":
		return PlatformACL, nil
	case id == "ubuntu":
		return PlatformUbuntu, nil
	case id == "flatcar":
		return PlatformFlatcar, nil
	case id == "mariner", id == "azurelinux":
		return PlatformMariner, nil
	case id == "":
		return "", fmt.Errorf("ID is missing from %s", osReleasePath)
	default:
		return "", fmt.Errorf("unsupported OS ID %q in %s", id, osReleasePath)
	}
}

func parseOSRelease(data []byte) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values
}

func applyFS(payloadFS fs.FS, platform Platform) (Result, error) {
	entries, err := loadAndValidate(payloadFS, platform)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	var staged []*stagedEntry
	for _, entry := range entries {
		pending, changed, err := stageEntry(entry.destination, entry.content, entry.mode)
		if err != nil {
			cleanupStaged(staged)
			return result, fmt.Errorf("apply embedded hotfix to %s: %w", entry.destination, err)
		}
		if !changed {
			result.Skipped++
			continue
		}
		staged = append(staged, &pending)
	}
	if err := commitStaged(staged); err != nil {
		return Result{}, err
	}
	result.Applied = len(staged)
	return result, nil
}

func loadAndValidate(payloadFS fs.FS, platform Platform) ([]payloadEntry, error) {
	if !isConcretePlatform(platform) {
		return nil, fmt.Errorf("unsupported concrete platform %q", platform)
	}
	renderedPath := fmt.Sprintf(
		"generated/rendered_nodecustomdata_%s.yml",
		platform,
	)
	data, err := fs.ReadFile(payloadFS, renderedPath)
	if err != nil {
		return nil, fmt.Errorf("read embedded nodecustomdata %s: %w", renderedPath, err)
	}

	var customData nodeCustomData
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&customData); err != nil {
		return nil, fmt.Errorf("decode embedded nodecustomdata %s: %w", renderedPath, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("embedded nodecustomdata %s has trailing content", renderedPath)
	}

	entries := make([]payloadEntry, 0, len(customData.WriteFiles))
	destinations := make(map[string]struct{}, len(customData.WriteFiles))
	for index, file := range customData.WriteFiles {
		entry, err := validateWriteFile(file)
		if err != nil {
			return nil, fmt.Errorf("validate embedded write_files entry %d: %w", index, err)
		}
		if _, exists := destinations[entry.destination]; exists {
			return nil, fmt.Errorf("duplicate destination %s", entry.destination)
		}
		destinations[entry.destination] = struct{}{}
		entries = append(entries, entry)
	}
	return entries, nil
}

func validateWriteFile(file writeFile) (payloadEntry, error) {
	if file.Path == "" ||
		(!strings.HasPrefix(file.Path, "/") && !filepath.IsAbs(file.Path)) ||
		(strings.HasPrefix(file.Path, "/") && path.Clean(file.Path) != file.Path) ||
		(!strings.HasPrefix(file.Path, "/") && filepath.Clean(file.Path) != file.Path) {
		return payloadEntry{}, fmt.Errorf("unsafe destination %q", file.Path)
	}
	if strings.HasPrefix(file.Path, "/") && strings.Contains(file.Path, `\`) {
		return payloadEntry{}, fmt.Errorf("unsafe destination %q: backslashes are not allowed", file.Path)
	}
	if file.Owner != "" && file.Owner != "root" {
		return payloadEntry{}, fmt.Errorf("unsupported owner %q", file.Owner)
	}
	mode, err := parseMode(file.Permissions)
	if err != nil {
		return payloadEntry{}, err
	}
	content, err := decodeContent(file)
	if err != nil {
		return payloadEntry{}, err
	}
	if len(content) == 0 {
		return payloadEntry{}, fmt.Errorf("content for %s is empty", file.Path)
	}
	return payloadEntry{
		destination: file.Path,
		mode:        mode,
		content:     content,
	}, nil
}

func parseMode(value string) (os.FileMode, error) {
	parsed, err := strconv.ParseUint(value, 8, 32)
	if err != nil || parsed == 0 || parsed > 0o777 {
		return 0, fmt.Errorf("invalid mode %q", value)
	}
	return os.FileMode(parsed), nil
}

func decodeContent(file writeFile) ([]byte, error) {
	switch file.Encoding {
	case "":
		return []byte(file.Content), nil
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return nil, fmt.Errorf("decode base64 content for %s: %w", file.Path, err)
		}
		return decoded, nil
	case "gzip":
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
	default:
		return nil, fmt.Errorf("unsupported encoding %q", file.Encoding)
	}
}

func isConcretePlatform(platform Platform) bool {
	switch platform {
	case PlatformUbuntu, PlatformMariner, PlatformACL, PlatformAzlOSGuard, PlatformFlatcar:
		return true
	default:
		return false
	}
}

func stageEntry(destination string, payload []byte, mode os.FileMode) (stagedEntry, bool, error) {
	current, err := os.ReadFile(destination)
	var originalMode os.FileMode
	switch {
	case err == nil:
		info, statErr := os.Stat(destination)
		if statErr != nil {
			return stagedEntry{}, false, fmt.Errorf("stat destination: %w", statErr)
		}
		originalMode = info.Mode().Perm()
		if bytes.Equal(current, payload) && originalMode == mode.Perm() {
			return stagedEntry{}, false, nil
		}
	case os.IsNotExist(err):
		return stagedEntry{}, false, nil
	default:
		return stagedEntry{}, false, fmt.Errorf("read destination: %w", err)
	}

	directory := filepath.Dir(destination)
	info, err := os.Stat(directory)
	if err != nil {
		return stagedEntry{}, false, fmt.Errorf("stat destination directory %s: %w", directory, err)
	}
	if !info.IsDir() {
		return stagedEntry{}, false, fmt.Errorf("destination parent %s is not a directory", directory)
	}

	stagedPath, err := writeTempFile(directory, ".aks-node-controller-hotfix-stage-*", payload, mode)
	if err != nil {
		return stagedEntry{}, false, err
	}
	staged := stagedEntry{
		destination:   destination,
		stagedPath:    stagedPath,
		originalExist: true,
	}
	backupPath, err := writeTempFile(
		directory,
		".aks-node-controller-hotfix-backup-*",
		current,
		originalMode,
	)
	if err != nil {
		_ = os.Remove(stagedPath)
		return stagedEntry{}, false, fmt.Errorf("stage destination backup: %w", err)
	}
	staged.backupPath = backupPath
	return staged, true, nil
}

func writeTempFile(directory, pattern string, content []byte, mode os.FileMode) (string, error) {
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

func commitStaged(staged []*stagedEntry) error {
	return commitStagedWithRename(staged, os.Rename)
}

func commitStagedWithRename(staged []*stagedEntry, rename func(string, string) error) error {
	committed := 0
	defer cleanupStaged(staged)
	for index, entry := range staged {
		if err := rename(entry.stagedPath, entry.destination); err != nil {
			rollbackErr := rollbackStaged(staged[:committed], rename)
			if rollbackErr != nil {
				return fmt.Errorf(
					"commit hotfix destination %s: %w; rollback failed: %w",
					entry.destination,
					err,
					rollbackErr,
				)
			}
			return fmt.Errorf("commit hotfix destination %s: %w", entry.destination, err)
		}
		staged[index].stagedPath = ""
		committed++
	}
	return nil
}

func rollbackStaged(committed []*stagedEntry, rename func(string, string) error) error {
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

func cleanupStaged(staged []*stagedEntry) {
	for _, entry := range staged {
		if entry.stagedPath != "" {
			_ = os.Remove(entry.stagedPath)
		}
		if entry.backupPath != "" && !entry.preserveBackup {
			_ = os.Remove(entry.backupPath)
		}
	}
}
