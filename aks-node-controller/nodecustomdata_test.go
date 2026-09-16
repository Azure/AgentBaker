package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyNodeCustomData(t *testing.T) {
	tempDir := t.TempDir()
	plainPath := filepath.Join(tempDir, "plain.txt")
	gzipPath := filepath.Join(tempDir, "gzip.txt")
	renderedPath := filepath.Join(tempDir, "nodecustomdata.yml")

	var gzipBuffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzipBuffer)
	_, err := gzipWriter.Write([]byte("gzip-content"))
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	rendered := fmt.Sprintf(`#cloud-config
write_files:
- path: %s
  permissions: "0600"
  owner: root
  content: |
    plain-content
- path: %s
  permissions: "0644"
  owner: root
  encoding: gzip
  content: !!binary |
    %s
`, plainPath, gzipPath, base64.StdEncoding.EncodeToString(gzipBuffer.Bytes()))
	require.NoError(t, os.WriteFile(renderedPath, []byte(rendered), 0o600))

	require.NoError(t, applyNodeCustomData(renderedPath))

	plainContent, err := os.ReadFile(plainPath)
	require.NoError(t, err)
	assert.Equal(t, "plain-content\n", string(plainContent))

	gzipContent, err := os.ReadFile(gzipPath)
	require.NoError(t, err)
	assert.Equal(t, "gzip-content", string(gzipContent))
}

// TestApplyNodeCustomDataCommandAppliesRenderedWriteFilesWhenScriptsVersionMatches verifies that
// the direct fallback applies nodecustomdata only when the config targets the current ANC
// version's YYYYMM.DD base with a strictly higher patch.
func TestApplyNodeCustomDataCommandAppliesRenderedWriteFilesWhenScriptsVersionMatches(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "marker.txt")
	renderedPath := filepath.Join(tempDir, "nodecustomdata.yml")
	hotfixPath := filepath.Join(tempDir, "hotfix-config.json")

	rendered := fmt.Sprintf(`#cloud-config
write_files:
- path: %s
  permissions: "0644"
  owner: root
  content: |
    rendered-marker
`, markerPath)
	require.NoError(t, os.WriteFile(renderedPath, []byte(rendered), 0o600))
	require.NoError(t, os.WriteFile(hotfixPath, []byte(`{"scripts_version": "202604.01.1"}`), 0o644))

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.nodeCustomDataPath = renderedPath
	tt.App.hotfixVersionPath = hotfixPath
	tt.App.osReleasePath = filepath.Join(tempDir, "os-release")
	require.NoError(t, os.WriteFile(tt.App.osReleasePath, []byte("ID=ubuntu\n"), 0o600))

	require.NoError(t, tt.App.runApplyNodeCustomDataCommand())

	markerContent, err := os.ReadFile(markerPath)
	require.NoError(t, err)
	assert.Equal(t, "rendered-marker\n", string(markerContent))
}

func TestApplyNodeCustomDataCommandSkipsIneligibleAndUnsupportedNodes(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		osRelease  string
		wantMarker bool
	}{
		{name: "different base", version: "202605.30.1", osRelease: "ID=ubuntu\n"},
		{name: "same version", version: "202604.01.0", osRelease: "ID=ubuntu\n"},
		{name: "OS Guard", version: "202604.01.1", osRelease: "ID=azurelinux\nVARIANT_ID=osguard\n"},
		{name: "ACL", version: "202604.01.1", osRelease: "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n"},
		{name: "Flatcar", version: "202604.01.1", osRelease: "ID=flatcar\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			origVersion := Version
			Version = "202604.01.0"
			defer func() { Version = origVersion }()

			dir := t.TempDir()
			markerPath := filepath.Join(dir, "marker")
			customDataPath := filepath.Join(dir, "nodecustomdata.yml")
			hotfixPath := filepath.Join(dir, "hotfix.json")
			osReleasePath := filepath.Join(dir, "os-release")
			require.NoError(t, os.WriteFile(customDataPath, []byte(fmt.Sprintf("write_files:\n- path: %s\n  content: marker\n", markerPath)), 0o600))
			require.NoError(t, os.WriteFile(hotfixPath, []byte(fmt.Sprintf(`{"scripts_version": %q}`, test.version)), 0o600))
			require.NoError(t, os.WriteFile(osReleasePath, []byte(test.osRelease), 0o600))

			tt := NewTestApp(t, TestAppConfig{})
			tt.App.nodeCustomDataPath = customDataPath
			tt.App.hotfixVersionPath = hotfixPath
			tt.App.osReleasePath = osReleasePath
			require.NoError(t, tt.App.runApplyNodeCustomDataCommand())
			_, err := os.Stat(markerPath)
			assert.Equal(t, test.wantMarker, err == nil)
		})
	}
}

// TestDownloadHotfixSkipsRenderedWriteFilesWhenScriptsVersionAbsent verifies that no
// nodecustomdata is applied when the hotfix config does not set scripts_version.
func TestDownloadHotfixSkipsRenderedWriteFilesWhenScriptsVersionAbsent(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "marker.txt")
	renderedPath := filepath.Join(tempDir, "nodecustomdata.yml")
	hotfixPath := filepath.Join(tempDir, "hotfix-config.json")

	rendered := fmt.Sprintf(`#cloud-config
write_files:
- path: %s
  permissions: "0644"
  owner: root
  content: |
    rendered-marker
`, markerPath)
	require.NoError(t, os.WriteFile(renderedPath, []byte(rendered), 0o600))
	require.NoError(t, os.WriteFile(hotfixPath, []byte(`{}`), 0o644))

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.nodeCustomDataPath = renderedPath
	tt.App.hotfixVersionPath = hotfixPath

	require.NoError(t, tt.App.downloadHotfix(context.Background()))

	_, err := os.Stat(markerPath)
	assert.True(t, os.IsNotExist(err), "marker file should not be written when scripts_version is absent")
}

// TestDownloadHotfixSkipsRenderedWriteFilesWhenBaseDiffers verifies that a scripts_version
// targeting a different YYYYMM.DD base than the current ANC version is not applied.
func TestDownloadHotfixSkipsRenderedWriteFilesWhenBaseDiffers(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "marker.txt")
	renderedPath := filepath.Join(tempDir, "nodecustomdata.yml")
	hotfixPath := filepath.Join(tempDir, "hotfix-config.json")

	rendered := fmt.Sprintf(`#cloud-config
write_files:
- path: %s
  permissions: "0644"
  owner: root
  content: |
    rendered-marker
`, markerPath)
	require.NoError(t, os.WriteFile(renderedPath, []byte(rendered), 0o600))
	require.NoError(t, os.WriteFile(hotfixPath, []byte(`{"scripts_version": "202605.30.1"}`), 0o644))

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.nodeCustomDataPath = renderedPath
	tt.App.hotfixVersionPath = hotfixPath

	require.NoError(t, tt.App.downloadHotfix(context.Background()))

	_, err := os.Stat(markerPath)
	assert.True(t, os.IsNotExist(err), "marker file should not be written when scripts_version base differs from current")
}
