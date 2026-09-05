package main

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

func TestClassifyNodeCustomDataPlatform(t *testing.T) {
	tests := []struct {
		name     string
		release  string
		expected nodeCustomDataPlatform
	}{
		{name: "Ubuntu", release: "ID=ubuntu\n", expected: nodeCustomDataPlatformUbuntu},
		{name: "Mariner", release: "ID=mariner\n", expected: nodeCustomDataPlatformMariner},
		{name: "Azure Linux", release: "ID=azurelinux\n", expected: nodeCustomDataPlatformMariner},
		{
			name:     "OS Guard variant wins over Azure Linux ID",
			release:  "ID=azurelinux\nVARIANT_ID=osguard\n",
			expected: nodeCustomDataPlatformAzlOSGuard,
		},
		{
			name:     "ACL variant wins over Azure Linux ID",
			release:  "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n",
			expected: nodeCustomDataPlatformACL,
		},
		{name: "ACL dedicated ID", release: "ID=azurecontainerlinux\n", expected: nodeCustomDataPlatformACL},
		{name: "Flatcar", release: "ID=flatcar\n", expected: nodeCustomDataPlatformFlatcar},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			releasePath := filepath.Join(t.TempDir(), "os-release")
			require.NoError(t, os.WriteFile(releasePath, []byte(test.release), 0o600))

			actual, err := classifyNodeCustomDataPlatform(releasePath)

			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}

	t.Run("unsupported ID fails explicitly", func(t *testing.T) {
		releasePath := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(releasePath, []byte("ID=other\n"), 0o600))

		_, err := classifyNodeCustomDataPlatform(releasePath)

		require.ErrorContains(t, err, "unsupported OS ID")
	})
}

func TestApplyEmbeddedNodeCustomDataIfActiveSkipsInactivePayload(t *testing.T) {
	original := generatedNodeCustomData
	generatedNodeCustomData = fstest.MapFS{
		"scripthotfix/generated/active": &fstest.MapFile{Data: []byte("false\n")},
	}
	t.Cleanup(func() {
		generatedNodeCustomData = original
	})

	result, err := applyEmbeddedNodeCustomDataIfActive(filepath.Join(t.TempDir(), "missing-os-release"))

	require.NoError(t, err)
	assert.Equal(t, nodeCustomDataApplyResult{}, result)
}

func TestApplyEmbeddedNodeCustomDataIfActiveSelectsPlatformPayload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rename cannot atomically replace an existing destination")
	}

	directory := t.TempDir()
	destination := filepath.Join(directory, "provision.sh")
	require.NoError(t, os.WriteFile(destination, []byte("old"), 0o600))
	releasePath := filepath.Join(directory, "os-release")
	require.NoError(t, os.WriteFile(releasePath, []byte("ID=ubuntu\n"), 0o600))
	payload := []byte("#!/bin/sh\necho fixed\n")

	original := generatedNodeCustomData
	generatedNodeCustomData = fstest.MapFS{
		"scripthotfix/generated/active": &fstest.MapFile{Data: []byte("true\n")},
		embeddedRenderedPath(nodeCustomDataPlatformUbuntu): &fstest.MapFile{Data: marshalNodeCustomData(t, []nodeCustomDataWriteFile{{
			Path:        destination,
			Permissions: "0744",
			Encoding:    encodingBase64,
			Owner:       "root",
			Content:     base64.StdEncoding.EncodeToString(payload),
		}})},
	}
	t.Cleanup(func() {
		generatedNodeCustomData = original
	})

	result, err := applyEmbeddedNodeCustomDataIfActive(releasePath)

	require.NoError(t, err)
	assert.Equal(t, nodeCustomDataApplyResult{Applied: 1}, result)
	actual, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, payload, actual)
}

func TestApplyEmbeddedNodeCustomDataIsReplaceOnlyAndIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rename cannot atomically replace an existing destination")
	}

	directory := t.TempDir()
	destination := filepath.Join(directory, "provision.sh")
	missing := filepath.Join(directory, "missing.sh")
	require.NoError(t, os.WriteFile(destination, []byte("old"), 0o600))
	files := embeddedRenderedFS(t, nodeCustomDataPlatformUbuntu, []nodeCustomDataWriteFile{
		{
			Path:        destination,
			Permissions: "0744",
			Owner:       "root",
			Content:     "hotfix",
		},
		{
			Path:        missing,
			Permissions: "0744",
			Owner:       "root",
			Content:     "not-created",
		},
	})

	first, err := applyEmbeddedNodeCustomDataFS(files, nodeCustomDataPlatformUbuntu)

	require.NoError(t, err)
	assert.Equal(t, nodeCustomDataApplyResult{Applied: 1, Skipped: 1}, first)
	actual, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, []byte("hotfix"), actual)
	info, err := os.Stat(destination)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o744), info.Mode().Perm())
	_, statErr := os.Stat(missing)
	assert.True(t, os.IsNotExist(statErr))

	second, err := applyEmbeddedNodeCustomDataFS(files, nodeCustomDataPlatformUbuntu)
	require.NoError(t, err)
	assert.Equal(t, nodeCustomDataApplyResult{Skipped: 2}, second)
}

func TestEmbeddedNodeCustomDataStrictValidation(t *testing.T) {
	validDestination := filepath.Join(t.TempDir(), "provision.sh")
	valid := nodeCustomDataWriteFile{
		Path:        validDestination,
		Permissions: "0744",
		Owner:       "root",
		Content:     "hotfix",
	}

	tests := []struct {
		name        string
		files       []nodeCustomDataWriteFile
		expectedErr string
	}{
		{
			name: "unsafe destination",
			files: []nodeCustomDataWriteFile{{
				Path:        "../provision.sh",
				Permissions: "0744",
				Owner:       "root",
				Content:     "hotfix",
			}},
			expectedErr: "unsafe destination",
		},
		{
			name: "destination with embedded backslash",
			files: []nodeCustomDataWriteFile{{
				Path:        `/opt/provision\script.sh`,
				Permissions: "0744",
				Owner:       "root",
				Content:     "hotfix",
			}},
			expectedErr: "backslashes are not allowed",
		},
		{
			name: "absent mode",
			files: []nodeCustomDataWriteFile{{
				Path:    validDestination,
				Owner:   "root",
				Content: "hotfix",
			}},
			expectedErr: "invalid mode",
		},
		{
			name: "invalid mode",
			files: []nodeCustomDataWriteFile{{
				Path:        validDestination,
				Permissions: "0999",
				Owner:       "root",
				Content:     "hotfix",
			}},
			expectedErr: "invalid mode",
		},
		{
			name: "unsupported owner",
			files: []nodeCustomDataWriteFile{{
				Path:        validDestination,
				Permissions: "0744",
				Owner:       "nobody",
				Content:     "hotfix",
			}},
			expectedErr: "unsupported owner",
		},
		{
			name: "unsupported encoding",
			files: []nodeCustomDataWriteFile{{
				Path:        validDestination,
				Permissions: "0744",
				Owner:       "root",
				Encoding:    "rot13",
				Content:     "hotfix",
			}},
			expectedErr: "unsupported encoding",
		},
		{
			name: "empty decoded content",
			files: []nodeCustomDataWriteFile{{
				Path:        validDestination,
				Permissions: "0744",
				Owner:       "root",
				Encoding:    encodingBase64,
				Content:     "",
			}},
			expectedErr: "is empty",
		},
		{
			name:        "duplicate destination",
			files:       []nodeCustomDataWriteFile{valid, valid},
			expectedErr: "duplicate destination",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := applyEmbeddedNodeCustomDataFS(
				embeddedRenderedFS(t, nodeCustomDataPlatformUbuntu, test.files),
				nodeCustomDataPlatformUbuntu,
			)
			require.ErrorContains(t, err, test.expectedErr)
		})
	}

	t.Run("unknown YAML field", func(t *testing.T) {
		files := fstest.MapFS{
			embeddedRenderedPath(nodeCustomDataPlatformUbuntu): &fstest.MapFile{
				Data: []byte("write_files: []\nunknown: true\n"),
			},
		}
		_, err := applyEmbeddedNodeCustomDataFS(files, nodeCustomDataPlatformUbuntu)
		require.ErrorContains(t, err, "field unknown not found")
	})

	t.Run("trailing YAML document", func(t *testing.T) {
		files := fstest.MapFS{
			embeddedRenderedPath(nodeCustomDataPlatformUbuntu): &fstest.MapFile{
				Data: []byte("write_files: []\n---\nwrite_files: []\n"),
			},
		}
		_, err := applyEmbeddedNodeCustomDataFS(files, nodeCustomDataPlatformUbuntu)
		require.ErrorContains(t, err, "trailing content")
	})
}

func TestNodeCustomDataSharedDecoderHandlesGzip(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte("rendered payload"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	decoded, err := decodeNodeCustomDataWriteFileContent(nodeCustomDataWriteFile{
		Encoding: encodingGZIP,
		Content:  compressed.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, []byte("rendered payload"), decoded)
}

func TestEmbeddedNodeCustomDataStagesAllBeforeCommit(t *testing.T) {
	directory := t.TempDir()
	firstDestination := filepath.Join(directory, "first.sh")
	require.NoError(t, os.WriteFile(firstDestination, []byte("original"), 0o700))
	files := embeddedRenderedFS(t, nodeCustomDataPlatformUbuntu, []nodeCustomDataWriteFile{
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

	_, err := applyEmbeddedNodeCustomDataFS(files, nodeCustomDataPlatformUbuntu)

	require.ErrorContains(t, err, "read destination")
	actual, readErr := os.ReadFile(firstDestination)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("original"), actual)
	staged, globErr := filepath.Glob(filepath.Join(directory, ".aks-node-controller-nodecustomdata-*"))
	require.NoError(t, globErr)
	assert.Empty(t, staged)
}

func TestCommitStagedNodeCustomDataRollsBackEarlierReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows rename cannot atomically replace an existing destination")
	}
	directory := t.TempDir()
	firstDestination := filepath.Join(directory, "first.sh")
	secondDestination := filepath.Join(directory, "second.sh")
	require.NoError(t, os.WriteFile(firstDestination, []byte("first original"), 0o700))
	require.NoError(t, os.WriteFile(secondDestination, []byte("second original"), 0o711))
	first, changed, err := stageNodeCustomDataEntry(
		nodeCustomDataEntry{destination: firstDestination, content: []byte("first hotfix"), mode: 0o744},
		true,
	)
	require.NoError(t, err)
	require.True(t, changed)
	second, changed, err := stageNodeCustomDataEntry(
		nodeCustomDataEntry{destination: secondDestination, content: []byte("second hotfix"), mode: 0o755},
		true,
	)
	require.NoError(t, err)
	require.True(t, changed)

	err = commitStagedNodeCustomDataWithRename(
		[]*stagedNodeCustomDataEntry{&first, &second},
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

func embeddedRenderedFS(
	t *testing.T,
	platform nodeCustomDataPlatform,
	files []nodeCustomDataWriteFile,
) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		embeddedRenderedPath(platform): &fstest.MapFile{Data: marshalNodeCustomData(t, files)},
	}
}

func marshalNodeCustomData(t *testing.T, files []nodeCustomDataWriteFile) []byte {
	t.Helper()
	data, err := yaml.Marshal(nodeCustomData{WriteFiles: files})
	require.NoError(t, err)
	return data
}

func embeddedRenderedPath(platform nodeCustomDataPlatform) string {
	return "scripthotfix/generated/rendered_nodecustomdata_" + string(platform) + ".yml"
}
