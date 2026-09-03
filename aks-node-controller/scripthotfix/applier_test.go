// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package scripthotfix

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
			name:     "OS Guard variant wins over Azure Linux ID",
			release:  "ID=azurelinux\nVARIANT_ID=osguard\n",
			expected: PlatformAzlOSGuard,
		},
		{
			name:     "ACL variant wins over Azure Linux ID",
			release:  "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n",
			expected: PlatformACL,
		},
		{name: "ACL dedicated ID", release: "ID=azurecontainerlinux\n", expected: PlatformACL},
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

func TestApplyEmbeddedInactivePayloadDoesNotReadOSRelease(t *testing.T) {
	original := generatedFiles
	generatedFiles = fstest.MapFS{
		"generated/active": &fstest.MapFile{Data: []byte("false\n")},
	}
	t.Cleanup(func() {
		generatedFiles = original
	})

	result, err := ApplyEmbedded(filepath.Join(t.TempDir(), "missing-os-release"))

	require.NoError(t, err)
	assert.Equal(t, Result{}, result)
}

func TestApplyFSUsesSelectedRenderedNodeCustomData(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rename cannot atomically replace an existing destination")
	}

	directory := t.TempDir()
	destination := filepath.Join(directory, "provision.sh")
	require.NoError(t, os.WriteFile(destination, []byte("old"), 0o600))
	payload := []byte("#!/bin/sh\necho fixed\n")
	files := renderedFS(t, PlatformUbuntu, []writeFile{{
		Path:        destination,
		Permissions: "0744",
		Encoding:    "base64",
		Owner:       "root",
		Content:     base64.StdEncoding.EncodeToString(payload),
	}})

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
}

func TestApplyFSSkipsMissingDestination(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "missing.sh")
	files := renderedFS(t, PlatformMariner, []writeFile{{
		Path:        destination,
		Permissions: "0744",
		Owner:       "root",
		Content:     "hotfix",
	}})

	result, err := applyFS(files, PlatformMariner)

	require.NoError(t, err)
	assert.Equal(t, Result{Skipped: 1}, result)
	_, statErr := os.Stat(destination)
	assert.True(t, os.IsNotExist(statErr))
}

func TestRenderedNodeCustomDataValidation(t *testing.T) {
	validDestination := filepath.Join(t.TempDir(), "provision.sh")
	valid := writeFile{
		Path:        validDestination,
		Permissions: "0744",
		Owner:       "root",
		Content:     "hotfix",
	}

	tests := []struct {
		name        string
		files       []writeFile
		expectedErr string
	}{
		{
			name: "unsafe destination",
			files: []writeFile{{
				Path:        "../provision.sh",
				Permissions: "0744",
				Owner:       "root",
				Content:     "hotfix",
			}},
			expectedErr: "unsafe destination",
		},
		{
			name: "destination with embedded backslash",
			files: []writeFile{{
				Path:        `/tmp/provision\script.sh`,
				Permissions: "0744",
				Owner:       "root",
				Content:     "hotfix",
			}},
			expectedErr: "backslashes are not allowed",
		},
		{
			name: "invalid mode",
			files: []writeFile{{
				Path:        validDestination,
				Permissions: "0999",
				Owner:       "root",
				Content:     "hotfix",
			}},
			expectedErr: "invalid mode",
		},
		{
			name: "unsupported owner",
			files: []writeFile{{
				Path:        validDestination,
				Permissions: "0744",
				Owner:       "nobody",
				Content:     "hotfix",
			}},
			expectedErr: "unsupported owner",
		},
		{
			name:        "duplicate destination",
			files:       []writeFile{valid, valid},
			expectedErr: "duplicate destination",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadAndValidate(renderedFS(t, PlatformUbuntu, test.files), PlatformUbuntu)
			require.ErrorContains(t, err, test.expectedErr)
		})
	}

	t.Run("unknown YAML field", func(t *testing.T) {
		files := fstest.MapFS{
			renderedPath(PlatformUbuntu): &fstest.MapFile{
				Data: []byte("write_files: []\nunknown: true\n"),
			},
		}
		_, err := loadAndValidate(files, PlatformUbuntu)
		require.ErrorContains(t, err, "field unknown not found")
	})
}

func TestDecodeContentGzip(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte("rendered payload"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	decoded, err := decodeContent(writeFile{
		Encoding: "gzip",
		Content:  compressed.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, []byte("rendered payload"), decoded)
}

func TestApplyFSDoesNotCommitWhenLaterEntryCannotBeStaged(t *testing.T) {
	directory := t.TempDir()
	firstDestination := filepath.Join(directory, "first.sh")
	require.NoError(t, os.WriteFile(firstDestination, []byte("original"), 0o700))
	files := renderedFS(t, PlatformUbuntu, []writeFile{
		{
			Path:        firstDestination,
			Permissions: "0744",
			Owner:       "root",
			Content:     "first hotfix",
		},
		{
			Path:        directory,
			Permissions: "0744",
			Owner:       "root",
			Content:     "second hotfix",
		},
	})

	_, err := applyFS(files, PlatformUbuntu)

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
	first, changed, err := stageEntry(firstDestination, []byte("first hotfix"), 0o744)
	require.NoError(t, err)
	require.True(t, changed)
	second, changed, err := stageEntry(secondDestination, []byte("second hotfix"), 0o755)
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
	secondActual, readErr := os.ReadFile(secondDestination)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("second original"), secondActual)
}

func renderedFS(t *testing.T, platform Platform, files []writeFile) fstest.MapFS {
	t.Helper()
	data, err := yaml.Marshal(nodeCustomData{WriteFiles: files})
	require.NoError(t, err)
	return fstest.MapFS{
		renderedPath(platform): &fstest.MapFile{Data: data},
	}
}

func renderedPath(platform Platform) string {
	return "generated/rendered_nodecustomdata_" + string(platform) + ".yml"
}
