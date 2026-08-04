// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

// Package scripthotfix applies generated provisioning script hotfixes embedded
// in the aks-node-controller binary.
package scripthotfix

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	manifestPath         = "generated/manifest.json"
	supportedSchema      = 1
	defaultOSReleasePath = "/etc/os-release"
)

type Platform string

const (
	PlatformUbuntu     Platform = "ubuntu"
	PlatformMariner    Platform = "mariner"
	PlatformAzlOSGuard Platform = "azlosguard"
	PlatformFlatcar    Platform = "flatcar"
	PlatformACL        Platform = "acl"
)

//go:embed generated
var generatedFiles embed.FS

type Manifest struct {
	SchemaVersion int     `json:"schema_version"`
	Entries       []Entry `json:"entries"`
}

type Entry struct {
	Source      string     `json:"source"`
	Payload     string     `json:"payload"`
	Destination string     `json:"destination"`
	Mode        string     `json:"mode"`
	Platforms   []Platform `json:"platforms"`
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

// ApplyEmbedded applies the payload compiled into this ANC binary.
func ApplyEmbedded(osReleasePath string) (Result, error) {
	manifest, payloads, err := loadAndValidate(generatedFiles)
	if err != nil {
		return Result{}, err
	}
	if len(manifest.Entries) == 0 {
		return Result{}, nil
	}
	if osReleasePath == "" {
		osReleasePath = defaultOSReleasePath
	}
	platform, err := ClassifyPlatform(osReleasePath)
	if err != nil {
		return Result{}, err
	}
	return applyValidated(manifest, payloads, platform)
}

// ClassifyPlatform maps /etc/os-release to the existing nodecustomdata distro
// branch semantics used by the generator.
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
	case variant == "azurecontainerlinux":
		return PlatformACL, nil
	case id == "azurecontainerlinux":
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
		values[strings.TrimSpace(key)] = strings.Trim(
			strings.TrimSpace(value),
			`"'`,
		)
	}
	return values
}

func applyFS(payloadFS fs.FS, platform Platform) (Result, error) {
	manifest, payloads, err := loadAndValidate(payloadFS)
	if err != nil {
		return Result{}, err
	}
	return applyValidated(manifest, payloads, platform)
}

func applyValidated(
	manifest Manifest,
	payloads map[string][]byte,
	platform Platform,
) (Result, error) {
	entries, err := selectEntries(manifest, platform)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	var staged []*stagedEntry
	for _, entry := range entries {
		mode, err := parseMode(entry.Mode)
		if err != nil {
			return result, fmt.Errorf("parse mode for %s: %w", entry.Source, err)
		}
		pending, changed, err := stageEntry(
			entry.Destination,
			payloads[entry.Payload],
			mode,
		)
		if err != nil {
			cleanupStaged(staged)
			return result, fmt.Errorf(
				"apply embedded hotfix %s to %s: %w",
				entry.Source,
				entry.Destination,
				err,
			)
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

func loadAndValidate(payloadFS fs.FS) (Manifest, map[string][]byte, error) {
	var manifest Manifest
	data, err := fs.ReadFile(payloadFS, manifestPath)
	if err != nil {
		return manifest, nil, fmt.Errorf("read embedded manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, nil, fmt.Errorf("decode embedded manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return manifest, nil, fmt.Errorf("embedded manifest has trailing content")
	}
	if manifest.SchemaVersion != supportedSchema {
		return manifest, nil, fmt.Errorf(
			"unsupported embedded manifest schema %d",
			manifest.SchemaVersion,
		)
	}

	payloads := make(map[string][]byte, len(manifest.Entries))
	destinations := make(map[Platform]map[string]string, len(concretePlatforms()))
	for _, platform := range concretePlatforms() {
		destinations[platform] = make(map[string]string)
	}

	for index, entry := range manifest.Entries {
		if err := validateEntry(entry); err != nil {
			return manifest, nil, fmt.Errorf("validate manifest entry %d: %w", index, err)
		}
		for _, platform := range entry.Platforms {
			if previous := destinations[platform][entry.Destination]; previous != "" {
				return manifest, nil, fmt.Errorf(
					"duplicate destination %s for platform %s: %s and %s",
					entry.Destination,
					platform,
					previous,
					entry.Source,
				)
			}
			destinations[platform][entry.Destination] = entry.Source
		}

		payloadPath := path.Join("generated", entry.Payload)
		payload, err := fs.ReadFile(payloadFS, payloadPath)
		if err != nil {
			return manifest, nil, fmt.Errorf(
				"read payload %s for %s: %w",
				entry.Payload,
				entry.Source,
				err,
			)
		}
		payloads[entry.Payload] = payload
	}
	return manifest, payloads, nil
}

func validateEntry(entry Entry) error {
	if strings.TrimSpace(entry.Source) == "" {
		return fmt.Errorf("source is required")
	}
	if entry.Payload == "" ||
		!strings.HasPrefix(entry.Payload, "payloads/") ||
		path.Clean(entry.Payload) != entry.Payload ||
		strings.Contains(entry.Payload, `\`) {
		return fmt.Errorf("unsafe payload path %q", entry.Payload)
	}
	if entry.Destination == "" ||
		(!strings.HasPrefix(entry.Destination, "/") && !filepath.IsAbs(entry.Destination)) ||
		(strings.HasPrefix(entry.Destination, "/") && path.Clean(entry.Destination) != entry.Destination) ||
		(!strings.HasPrefix(entry.Destination, "/") && filepath.Clean(entry.Destination) != entry.Destination) {
		return fmt.Errorf("unsafe destination %q", entry.Destination)
	}
	if _, err := parseMode(entry.Mode); err != nil {
		return err
	}
	if len(entry.Platforms) == 0 {
		return fmt.Errorf("at least one platform is required")
	}
	seenPlatforms := make(map[Platform]struct{}, len(entry.Platforms))
	for _, platform := range entry.Platforms {
		if !isConcretePlatform(platform) {
			return fmt.Errorf("unsupported platform %q", platform)
		}
		if _, exists := seenPlatforms[platform]; exists {
			return fmt.Errorf("duplicate platform %q", platform)
		}
		seenPlatforms[platform] = struct{}{}
	}
	return nil
}

func parseMode(value string) (os.FileMode, error) {
	parsed, err := strconv.ParseUint(value, 8, 32)
	if err != nil || parsed == 0 || parsed > 0o777 {
		return 0, fmt.Errorf("invalid mode %q", value)
	}
	return os.FileMode(parsed), nil
}

func isConcretePlatform(platform Platform) bool {
	for _, supported := range concretePlatforms() {
		if platform == supported {
			return true
		}
	}
	return false
}

func concretePlatforms() []Platform {
	return []Platform{
		PlatformUbuntu,
		PlatformMariner,
		PlatformAzlOSGuard,
		PlatformFlatcar,
		PlatformACL,
	}
}

func selectEntries(manifest Manifest, platform Platform) ([]Entry, error) {
	if !isConcretePlatform(platform) {
		return nil, fmt.Errorf("unsupported concrete platform %q", platform)
	}
	var selected []Entry
	destinations := make(map[string]string)
	for _, entry := range manifest.Entries {
		if !containsPlatform(entry.Platforms, platform) {
			continue
		}
		if previous := destinations[entry.Destination]; previous != "" {
			return nil, fmt.Errorf(
				"duplicate selected destination %s: %s and %s",
				entry.Destination,
				previous,
				entry.Source,
			)
		}
		destinations[entry.Destination] = entry.Source
		selected = append(selected, entry)
	}
	return selected, nil
}

func containsPlatform(platforms []Platform, target Platform) bool {
	for _, platform := range platforms {
		if platform == target {
			return true
		}
	}
	return false
}

func stageEntry(
	destination string,
	payload []byte,
	mode os.FileMode,
) (stagedEntry, bool, error) {
	current, err := os.ReadFile(destination)
	var originalMode os.FileMode
	switch {
	case err == nil:
		info, statErr := os.Stat(destination)
		if statErr != nil {
			return stagedEntry{}, false, fmt.Errorf("stat destination: %w", statErr)
		}
		originalMode = info.Mode().Perm()
		if bytes.Equal(current, payload) && info.Mode().Perm() == mode.Perm() {
			return stagedEntry{}, false, nil
		}
	case os.IsNotExist(err):
		// Phase 1 replaces scripts selected by the VHD/CustomData path. A
		// missing destination can reflect a non-platform template gate, so
		// creating it here would change behavior the manifest cannot describe.
		return stagedEntry{}, false, nil
	default:
		return stagedEntry{}, false, fmt.Errorf("read destination: %w", err)
	}

	directory := filepath.Dir(destination)
	info, err := os.Stat(directory)
	if err != nil {
		return stagedEntry{}, false, fmt.Errorf(
			"stat destination directory %s: %w",
			directory,
			err,
		)
	}
	if !info.IsDir() {
		return stagedEntry{}, false, fmt.Errorf(
			"destination parent %s is not a directory",
			directory,
		)
	}

	stagedPath, err := writeTempFile(
		directory,
		".aks-node-controller-hotfix-stage-*",
		payload,
		mode,
	)
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
		return stagedEntry{}, false, fmt.Errorf(
			"stage destination backup: %w",
			err,
		)
	}
	staged.backupPath = backupPath
	return staged, true, nil
}

func writeTempFile(
	directory string,
	pattern string,
	content []byte,
	mode os.FileMode,
) (string, error) {
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

func commitStagedWithRename(
	staged []*stagedEntry,
	rename func(string, string) error,
) error {
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
			return fmt.Errorf(
				"commit hotfix destination %s: %w",
				entry.destination,
				err,
			)
		}
		staged[index].stagedPath = ""
		committed++
	}
	return nil
}

func rollbackStaged(
	committed []*stagedEntry,
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
