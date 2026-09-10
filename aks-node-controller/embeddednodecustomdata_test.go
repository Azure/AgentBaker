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
				require.NoError(t, applyEmbeddedNodeCustomData(fstest.MapFS{}, releasePath),
					"unsupported platforms must skip without reading any payload")
			}
		})
	}
}

func TestApplyEmbeddedNodeCustomData(t *testing.T) {
	for _, id := range []string{"ubuntu", "azurelinux"} {
		t.Run(id, func(t *testing.T) {
			directory := t.TempDir()
			t.Setenv("TMPDIR", directory)
			t.Setenv("TMP", directory)
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
				"scripthotfix/generated/rendered_nodecustomdata_" + platform + ".yml": &fstest.MapFile{Data: data},
			}
			require.NoError(t, applyEmbeddedNodeCustomData(files, releasePath))
			for _, destination := range []string{existing, missing} {
				actual, readErr := os.ReadFile(destination)
				require.NoError(t, readErr)
				assert.Equal(t, payload, string(actual))
			}
			temporary, err := filepath.Glob(filepath.Join(directory, "aks-node-controller-nodecustomdata-*.yml"))
			require.NoError(t, err)
			assert.Empty(t, temporary)
			if runtime.GOOS != "windows" {
				info, statErr := os.Stat(missing)
				require.NoError(t, statErr)
				assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
			}
		})
	}
}

func TestApplyEmbeddedNodeCustomDataErrorsAndCleanup(t *testing.T) {
	tests := []struct {
		name      string
		release   string
		payload   string
		missing   string
		wantError string
	}{
		{name: "missing release", missing: "release", wantError: "read OS release"},
		{name: "unknown OS", release: "ID=other", wantError: "unsupported OS ID"},
		{name: "legacy mariner", release: "ID=mariner", wantError: "unsupported OS ID"},
		{name: "missing ID", release: "VERSION_ID=3.0", wantError: "ID is missing"},
		{name: "missing payload", release: "ID=ubuntu", missing: "payload"},
		{name: "other platform payload only", release: "ID=azurelinux", payload: "write_files: ["},
		{name: "unreadable payload directory", release: "ID=ubuntu", missing: "payload-file", wantError: "read embedded nodecustomdata"},
		{name: "malformed YAML", release: "ID=ubuntu", payload: "write_files: [", wantError: "unmarshal nodecustomdata"},
		{name: "invalid entry", release: "ID=ubuntu", payload: "write_files:\n- content: invalid\n", wantError: "path is required"},
		{name: "empty payload", release: "ID=ubuntu", payload: "write_files: []\n"},
		{name: "temporary directory unavailable", release: "ID=ubuntu", payload: "write_files: []\n", missing: "temp", wantError: "create temporary nodecustomdata"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			tempDir := directory
			if test.missing == "temp" {
				tempDir = filepath.Join(directory, "missing")
			}
			t.Setenv("TMPDIR", tempDir)
			t.Setenv("TMP", tempDir)
			releasePath := filepath.Join(directory, "os-release")
			if test.missing != "release" {
				require.NoError(t, os.WriteFile(releasePath, []byte(test.release), 0o600))
			}
			files := fstest.MapFS{
				"scripthotfix/generated/README": &fstest.MapFile{Data: []byte("placeholder")},
			}
			if test.missing != "payload" {
				files["scripthotfix/generated/rendered_nodecustomdata_ubuntu.yml"] = &fstest.MapFile{Data: []byte(test.payload)}
			}
			if test.missing == "payload-file" {
				files["scripthotfix/generated/rendered_nodecustomdata_ubuntu.yml"].Mode = fs.ModeDir
			}
			err := applyEmbeddedNodeCustomData(files, releasePath)
			if test.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantError)
			}
			temporary, err := filepath.Glob(filepath.Join(directory, "aks-node-controller-nodecustomdata-*.yml"))
			require.NoError(t, err)
			assert.Empty(t, temporary)
		})
	}
}
