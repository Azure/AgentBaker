// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package scripthotfix

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyPlatform(t *testing.T) {
	tests := []struct {
		name     string
		release  string
		expected Platform
	}{
		{name: "Ubuntu", release: "ID=ubuntu\n", expected: PlatformUbuntu},
		{name: "Mariner", release: "ID=mariner\n", expected: PlatformMariner},
		{name: "Azure Linux", release: "ID=azurelinux\n", expected: PlatformMariner},
		{
			name:     "OS Guard uses variant before Azure Linux ID",
			release:  "ID=azurelinux\nVARIANT_ID=osguard\n",
			expected: PlatformAzlOSGuard,
		},
		{
			name:     "ACL uses variant before Azure Linux ID",
			release:  "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n",
			expected: PlatformACL,
		},
		{
			name:     "ACL dedicated ID",
			release:  "ID=azurecontainerlinux\n",
			expected: PlatformACL,
		},
		{name: "Flatcar", release: "ID=flatcar\n", expected: PlatformFlatcar},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			releasePath := filepath.Join(t.TempDir(), "os-release")
			require.NoError(t, os.WriteFile(releasePath, []byte(test.release), 0o600))

			actual, err := ClassifyPlatform(releasePath)

			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}

	t.Run("unsupported ID fails explicitly", func(t *testing.T) {
		releasePath := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(releasePath, []byte("ID=other\n"), 0o600))

		_, err := ClassifyPlatform(releasePath)

		require.ErrorContains(t, err, "unsupported OS ID")
	})
}

func TestManifestValidation(t *testing.T) {
	payload := []byte("hotfix")
	destination := filepath.Join(t.TempDir(), "provision.sh")
	valid := Entry{
		Source:      "cse_main.sh",
		Payload:     "payloads/cse_main.sh",
		Destination: destination,
		Mode:        "0744",
		Platforms:   concretePlatforms(),
	}

	t.Run("valid", func(t *testing.T) {
		manifest, payloads, err := loadAndValidate(testFS(t, []Entry{valid}, map[string][]byte{
			valid.Payload: payload,
		}))

		require.NoError(t, err)
		assert.Equal(t, []Entry{valid}, manifest.Entries)
		assert.Equal(t, payload, payloads[valid.Payload])
	})

	t.Run("unsupported schema", func(t *testing.T) {
		files := testFS(t, nil, nil)
		files[manifestPath] = &fstest.MapFile{Data: []byte(`{"schema_version":2,"entries":[]}`)}

		_, _, err := loadAndValidate(files)

		require.ErrorContains(t, err, "unsupported embedded manifest schema")
	})

	t.Run("unsafe payload path", func(t *testing.T) {
		entry := valid
		entry.Payload = "../cse_main.sh"

		_, _, err := loadAndValidate(testFS(t, []Entry{entry}, nil))

		require.ErrorContains(t, err, "unsafe payload path")
	})

	t.Run("invalid mode", func(t *testing.T) {
		entry := valid
		entry.Mode = "0999"

		_, _, err := loadAndValidate(testFS(t, []Entry{entry}, nil))

		require.ErrorContains(t, err, "invalid mode")
	})

	t.Run("duplicate destination on overlapping platform", func(t *testing.T) {
		ubuntu := valid
		ubuntu.Source = "ubuntu/cse_main.sh"
		ubuntu.Payload = "payloads/ubuntu/cse_main.sh"
		ubuntu.Platforms = []Platform{PlatformUbuntu}

		_, _, err := loadAndValidate(testFS(
			t,
			[]Entry{valid, ubuntu},
			map[string][]byte{
				valid.Payload:  payload,
				ubuntu.Payload: payload,
			},
		))

		require.ErrorContains(t, err, "duplicate destination")
	})
}

func TestSelectEntries(t *testing.T) {
	common := Entry{Source: "common", Destination: "/opt/common", Platforms: concretePlatforms()}
	ubuntu := Entry{Source: "ubuntu", Destination: "/opt/distro", Platforms: []Platform{PlatformUbuntu}}
	mariner := Entry{Source: "mariner", Destination: "/opt/distro", Platforms: []Platform{PlatformMariner}}
	manifest := Manifest{SchemaVersion: supportedSchema, Entries: []Entry{common, ubuntu, mariner}}

	selected, err := selectEntries(manifest, PlatformUbuntu)

	require.NoError(t, err)
	assert.Equal(t, []Entry{common, ubuntu}, selected)
}

func TestApplyEmbeddedEmptyManifestDoesNotRequireOSRelease(t *testing.T) {
	result, err := ApplyEmbedded(filepath.Join(t.TempDir(), "missing-os-release"))

	require.NoError(t, err)
	assert.Equal(t, Result{}, result)
}

func TestApplyFSIsAtomicAndIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rename cannot atomically replace an existing destination")
	}

	directory := t.TempDir()
	destination := filepath.Join(directory, "provision.sh")
	require.NoError(t, os.WriteFile(destination, []byte("old"), 0o600))
	payload := []byte("#!/bin/sh\necho fixed\n")
	entry := Entry{
		Source:      "cse_main.sh",
		Payload:     "payloads/cse_main.sh",
		Destination: destination,
		Mode:        "0744",
		Platforms:   concretePlatforms(),
	}
	files := testFS(t, []Entry{entry}, map[string][]byte{entry.Payload: payload})

	first, err := applyFS(files, PlatformUbuntu)

	require.NoError(t, err)
	assert.Equal(t, Result{Applied: 1}, first)
	actual, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, payload, actual)
	info, err := os.Stat(destination)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o744), info.Mode().Perm())

	second, err := applyFS(files, PlatformUbuntu)

	require.NoError(t, err)
	assert.Equal(t, Result{Skipped: 1}, second)
	matches, err := filepath.Glob(filepath.Join(directory, ".aks-node-controller-hotfix-*"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestApplyFSSkipsMissingDestinationAndReplacesExistingDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rename cannot atomically replace an existing destination")
	}
	directory := t.TempDir()
	existingDestination := filepath.Join(directory, "existing.sh")
	missingDestination := filepath.Join(directory, "missing", "gated.sh")
	require.NoError(t, os.WriteFile(existingDestination, []byte("original"), 0o700))
	existingPayload := []byte("existing hotfix")
	missingPayload := []byte("gated hotfix")
	entries := []Entry{
		{
			Source:      "existing.sh",
			Payload:     "payloads/existing.sh",
			Destination: existingDestination,
			Mode:        "0744",
			Platforms:   concretePlatforms(),
		},
		{
			Source:      "gated.sh",
			Payload:     "payloads/gated.sh",
			Destination: missingDestination,
			Mode:        "0744",
			Platforms:   concretePlatforms(),
		},
	}

	result, err := applyFS(
		testFS(t, entries, map[string][]byte{
			entries[0].Payload: existingPayload,
			entries[1].Payload: missingPayload,
		}),
		PlatformMariner,
	)

	require.NoError(t, err)
	assert.Equal(t, Result{Applied: 1, Skipped: 1}, result)
	actual, readErr := os.ReadFile(existingDestination)
	require.NoError(t, readErr)
	assert.Equal(t, existingPayload, actual)
	_, statErr := os.Stat(missingDestination)
	require.Error(t, statErr)
	assert.True(t, os.IsNotExist(statErr))
}

func TestApplyFSDoesNotCommitWhenLaterEntryCannotBeStaged(t *testing.T) {
	directory := t.TempDir()
	firstDestination := filepath.Join(directory, "first.sh")
	require.NoError(t, os.WriteFile(firstDestination, []byte("original"), 0o700))
	firstPayload := []byte("first hotfix")
	secondPayload := []byte("second hotfix")
	entries := []Entry{
		{
			Source:      "first.sh",
			Payload:     "payloads/first.sh",
			Destination: firstDestination,
			Mode:        "0744",
			Platforms:   concretePlatforms(),
		},
		{
			Source:      "second.sh",
			Payload:     "payloads/second.sh",
			Destination: directory,
			Mode:        "0744",
			Platforms:   concretePlatforms(),
		},
	}

	_, err := applyFS(
		testFS(t, entries, map[string][]byte{
			entries[0].Payload: firstPayload,
			entries[1].Payload: secondPayload,
		}),
		PlatformUbuntu,
	)

	require.ErrorContains(t, err, "read destination")
	actual, readErr := os.ReadFile(firstDestination)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("original"), actual)
}

func TestCommitStagedRollsBackEarlierReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rename cannot atomically replace an existing destination")
	}
	directory := t.TempDir()
	firstDestination := filepath.Join(directory, "first.sh")
	secondDestination := filepath.Join(directory, "second.sh")
	require.NoError(t, os.WriteFile(firstDestination, []byte("first original"), 0o700))
	require.NoError(t, os.WriteFile(secondDestination, []byte("second original"), 0o711))
	first, changed, err := stageEntry(
		firstDestination,
		[]byte("first hotfix"),
		0o744,
	)
	require.NoError(t, err)
	require.True(t, changed)
	second, changed, err := stageEntry(
		secondDestination,
		[]byte("second hotfix"),
		0o755,
	)
	require.NoError(t, err)
	require.True(t, changed)

	err = commitStagedWithRename(
		[]*stagedEntry{&first, &second},
		func(source string, destination string) error {
			if source == second.stagedPath {
				return errors.New("injected rename failure")
			}
			return os.Rename(source, destination)
		},
	)

	require.ErrorContains(t, err, "injected rename failure")
	firstActual, readErr := os.ReadFile(firstDestination)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("first original"), firstActual)
	firstInfo, statErr := os.Stat(firstDestination)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o700), firstInfo.Mode().Perm())
	secondActual, readErr := os.ReadFile(secondDestination)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("second original"), secondActual)
	secondInfo, statErr := os.Stat(secondDestination)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o711), secondInfo.Mode().Perm())
}

func TestCommitStagedPreservesBackupWhenRollbackFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rename cannot atomically replace an existing destination")
	}
	directory := t.TempDir()
	firstDestination := filepath.Join(directory, "first.sh")
	secondDestination := filepath.Join(directory, "second.sh")
	require.NoError(t, os.WriteFile(firstDestination, []byte("first original"), 0o700))
	require.NoError(t, os.WriteFile(secondDestination, []byte("second original"), 0o700))
	first, changed, err := stageEntry(
		firstDestination,
		[]byte("first hotfix"),
		0o744,
	)
	require.NoError(t, err)
	require.True(t, changed)
	second, changed, err := stageEntry(
		secondDestination,
		[]byte("second hotfix"),
		0o744,
	)
	require.NoError(t, err)
	require.True(t, changed)
	firstBackup := first.backupPath

	err = commitStagedWithRename(
		[]*stagedEntry{&first, &second},
		func(source string, destination string) error {
			if source == second.stagedPath || source == first.backupPath {
				return errors.New("injected rename failure")
			}
			return os.Rename(source, destination)
		},
	)

	require.ErrorContains(t, err, "rollback failed")
	require.ErrorContains(t, err, firstBackup)
	backup, readErr := os.ReadFile(firstBackup)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("first original"), backup)
}

func testFS(
	t *testing.T,
	entries []Entry,
	payloads map[string][]byte,
) fstest.MapFS {
	t.Helper()
	manifest, err := json.Marshal(Manifest{
		SchemaVersion: supportedSchema,
		Entries:       entries,
	})
	require.NoError(t, err)
	files := fstest.MapFS{
		manifestPath: &fstest.MapFile{Data: manifest},
	}
	for payloadPath, payload := range payloads {
		files[filepath.ToSlash(filepath.Join("generated", payloadPath))] = &fstest.MapFile{
			Data: payload,
		}
	}
	return files
}
