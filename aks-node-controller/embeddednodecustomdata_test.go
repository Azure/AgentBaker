package main

import (
	"encoding/base64"
	"io/fs"
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
		{"Ubuntu", "ID=ubuntu\n", nodeCustomDataPlatformUbuntu},
		{"Azure Linux", "ID=azurelinux\n", nodeCustomDataPlatformMariner},
		{"OS Guard", "ID=azurelinux\nVARIANT_ID=osguard\n", nodeCustomDataPlatformUnsupported},
		{"ACL variant", "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n", nodeCustomDataPlatformUnsupported},
		{"ACL ID", "ID=azurecontainerlinux\n", nodeCustomDataPlatformUnsupported},
		{"Flatcar", "ID=flatcar\n", nodeCustomDataPlatformUnsupported},
		{"Quoted OS Guard", "ID=\"AZURELINUX\"\nVARIANT_ID=\"OSGUARD\"\n", nodeCustomDataPlatformUnsupported},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			releasePath := filepath.Join(t.TempDir(), "os-release")
			require.NoError(t, os.WriteFile(releasePath, []byte(test.release), 0o600))
			actual, err := classifyNodeCustomDataPlatform(releasePath)
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
			if actual == nodeCustomDataPlatformUnsupported {
				outputPath := filepath.Join(t.TempDir(), "embedded-nodecustomdata.yml")
				require.NoError(t, applyEmbeddedNodeCustomData(fstest.MapFS{}, releasePath, outputPath),
					"unsupported platforms must skip without reading any payload")
				assert.NoFileExists(t, outputPath)
			}
		})
	}
}

func TestApplyEmbeddedNodeCustomData(t *testing.T) {
	for _, id := range []string{"ubuntu", "azurelinux"} {
		t.Run(id, func(t *testing.T) {
			directory := t.TempDir()
			outputPath := filepath.Join(directory, "containers", "embedded-nodecustomdata.yml")
			legacyPath := filepath.Join(directory, "containers", "nodecustomdata.yml")
			require.NoError(t, os.MkdirAll(filepath.Dir(legacyPath), 0o755))
			require.NoError(t, os.WriteFile(legacyPath, []byte("legacy payload"), 0o600))
			require.NoError(t, os.WriteFile(outputPath, []byte("previous embedded payload"), 0o600))
			releasePath := filepath.Join(directory, "os-release")
			require.NoError(t, os.WriteFile(releasePath, []byte("ID="+id+"\n"), 0o600))
			existing := filepath.Join(directory, "existing.sh")
			missing := filepath.Join(directory, "new", "script.sh")
			require.NoError(t, os.WriteFile(existing, []byte("old"), 0o600))
			payload := "echo hotfixed\n"
			data, err := yaml.Marshal(nodeCustomData{WriteFiles: []nodeCustomDataWriteFile{
				{Path: existing, Encoding: encodingBase64, Content: base64.StdEncoding.EncodeToString([]byte(payload))},
				{Path: missing, Content: payload},
			}})
			require.NoError(t, err)
			platform := id
			if id == osReleaseIDAzureLinux {
				platform = string(nodeCustomDataPlatformMariner)
			}
			files := fstest.MapFS{
				"generated/rendered_nodecustomdata_" + platform + ".yml": &fstest.MapFile{Data: data},
			}
			require.NoError(t, applyEmbeddedNodeCustomData(files, releasePath, outputPath))
			for _, destination := range []string{existing, missing} {
				actual, readErr := os.ReadFile(destination)
				require.NoError(t, readErr)
				assert.Equal(t, payload, string(actual))
			}
			retained, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			assert.Equal(t, data, retained)
			legacy, err := os.ReadFile(legacyPath)
			require.NoError(t, err)
			assert.Equal(t, "legacy payload", string(legacy))
			require.NoError(t, applyEmbeddedNodeCustomData(fstest.MapFS{}, releasePath, outputPath))
			retained, err = os.ReadFile(outputPath)
			require.NoError(t, err)
			assert.Equal(t, data, retained, "a skipped application must preserve the last payload")
			if runtime.GOOS != "windows" {
				info, statErr := os.Stat(missing)
				require.NoError(t, statErr)
				assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
			}
		})
	}
}

func TestApplyEmbeddedNodeCustomDataErrorsAndRetention(t *testing.T) {
	tests := []struct {
		name      string
		release   string
		payload   string
		missing   string
		wantError string
		retained  bool
	}{
		{name: "missing release", missing: "release", wantError: "read OS release"},
		{name: "unknown OS", release: "ID=other", wantError: "unsupported OS ID"},
		{name: "legacy mariner", release: "ID=mariner", wantError: "unsupported OS ID"},
		{name: "missing ID", release: "VERSION_ID=3.0", wantError: "ID is missing"},
		{name: "missing payload", release: "ID=ubuntu", missing: "payload"},
		{name: "other platform payload only", release: "ID=azurelinux", payload: "write_files: ["},
		{name: "unreadable payload directory", release: "ID=ubuntu", missing: "payload-file", wantError: "read embedded nodecustomdata"},
		{name: "malformed YAML", release: "ID=ubuntu", payload: "write_files: [", wantError: "unmarshal nodecustomdata", retained: true},
		{name: "invalid entry", release: "ID=ubuntu", payload: "write_files:\n- content: invalid\n", wantError: "path is required", retained: true},
		{name: "empty payload", release: "ID=ubuntu", payload: "write_files: []\n", retained: true},
		{name: "output directory blocked", release: "ID=ubuntu", payload: "write_files: []\n", missing: "output-dir", wantError: "create embedded nodecustomdata directory"},
		{name: "output file blocked", release: "ID=ubuntu", payload: "write_files: []\n", missing: "output-file", wantError: "write embedded nodecustomdata"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			outputPath := filepath.Join(directory, "containers", "embedded-nodecustomdata.yml")
			if test.missing == "output-dir" {
				require.NoError(t, os.WriteFile(filepath.Dir(outputPath), []byte("blocked"), 0o600))
			}
			if test.missing == "output-file" {
				require.NoError(t, os.MkdirAll(outputPath, 0o755))
			}
			releasePath := filepath.Join(directory, "os-release")
			if test.missing != "release" {
				require.NoError(t, os.WriteFile(releasePath, []byte(test.release), 0o600))
			}
			files := fstest.MapFS{
				"generated/README": &fstest.MapFile{Data: []byte("placeholder")},
			}
			if test.missing != "payload" {
				files["generated/rendered_nodecustomdata_ubuntu.yml"] = &fstest.MapFile{Data: []byte(test.payload)}
			}
			if test.missing == "payload-file" {
				files["generated/rendered_nodecustomdata_ubuntu.yml"].Mode = fs.ModeDir
			}
			err := applyEmbeddedNodeCustomData(files, releasePath, outputPath)
			if test.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantError)
			}
			assertRetainedEmbeddedPayload(t, outputPath, test.payload, test.retained)
		})
	}
}

func assertRetainedEmbeddedPayload(t *testing.T, outputPath, payload string, retained bool) {
	t.Helper()
	if !retained {
		assert.NoFileExists(t, outputPath)
		return
	}
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, payload, string(data))
	if runtime.GOOS != "windows" {
		info, err := os.Stat(outputPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}
