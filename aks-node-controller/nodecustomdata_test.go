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

func TestProvisionAppliesRenderedWriteFilesBeforeNBCCmd(t *testing.T) {
	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "marker.txt")
	renderedPath := filepath.Join(tempDir, "nodecustomdata.yml")
	scriptPath := filepath.Join(tempDir, "test_nbccmd.sh")

	rendered := fmt.Sprintf(`#cloud-config
write_files:
- path: %s
  permissions: "0644"
  owner: root
  content: |
    rendered-marker
`, markerPath)
	require.NoError(t, os.WriteFile(renderedPath, []byte(rendered), 0o600))

	script := fmt.Sprintf("#!/bin/bash\ngrep -qx 'rendered-marker' %s\n", markerPath)
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o700))

	tt := NewTestApp(t, TestAppConfig{RunFunc: cmdRunner})
	tt.App.nodeCustomDataPath = renderedPath

	result, err := tt.App.Provision(context.Background(), ProvisionFlags{NBCCmd: scriptPath})
	require.NoError(t, err)
	assert.Equal(t, "0", result.ExitCode)
}

func TestApplyScriptHotfix_VersionMatch(t *testing.T) {
	tempDir := t.TempDir()

	// Create staging directory and a staged script
	stagingDir := filepath.Join(tempDir, "hotfix", "scripts")
	require.NoError(t, os.MkdirAll(stagingDir, 0o755))

	stagingPath := filepath.Join(stagingDir, "provision_installs.sh")
	require.NoError(t, os.WriteFile(stagingPath, []byte("#!/bin/bash\n# hotfixed script"), 0o744))

	destPath := filepath.Join(tempDir, "dest", "provision_installs.sh")

	manifest := fmt.Sprintf(`{"targetVersion":"202604.27.0","files":[{"staging":"%s","destination":"%s"}]}`,
		stagingPath, destPath)
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifest), 0o644))

	// Current version matches target base (same YYYYMM.DD)
	err := applyScriptHotfix(manifestPath, "202604.27.0")
	require.NoError(t, err)

	content, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/bash\n# hotfixed script", string(content))
}

func TestApplyScriptHotfix_VersionMatch_HigherPatch(t *testing.T) {
	tempDir := t.TempDir()

	stagingDir := filepath.Join(tempDir, "hotfix", "scripts")
	require.NoError(t, os.MkdirAll(stagingDir, 0o755))

	stagingPath := filepath.Join(stagingDir, "provision_installs.sh")
	require.NoError(t, os.WriteFile(stagingPath, []byte("#!/bin/bash\n# hotfixed"), 0o744))

	destPath := filepath.Join(tempDir, "dest", "provision_installs.sh")

	manifest := fmt.Sprintf(`{"targetVersion":"202604.27.0","files":[{"staging":"%s","destination":"%s"}]}`,
		stagingPath, destPath)
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifest), 0o644))

	// Node has ANC-hotfixed version (patch 1) but same base — should still apply
	err := applyScriptHotfix(manifestPath, "202604.27.1")
	require.NoError(t, err)

	content, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/bash\n# hotfixed", string(content))
}

func TestApplyScriptHotfix_VersionMismatch(t *testing.T) {
	tempDir := t.TempDir()

	stagingDir := filepath.Join(tempDir, "hotfix", "scripts")
	require.NoError(t, os.MkdirAll(stagingDir, 0o755))

	stagingPath := filepath.Join(stagingDir, "provision_installs.sh")
	require.NoError(t, os.WriteFile(stagingPath, []byte("#!/bin/bash\n# hotfixed"), 0o744))

	destPath := filepath.Join(tempDir, "dest", "provision_installs.sh")

	manifest := fmt.Sprintf(`{"targetVersion":"202604.27.0","files":[{"staging":"%s","destination":"%s"}]}`,
		stagingPath, destPath)
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifest), 0o644))

	// Node is on a different VHD version — should NOT apply
	err := applyScriptHotfix(manifestPath, "202602.19.0")
	require.NoError(t, err)

	// Destination should not exist
	_, err = os.ReadFile(destPath)
	assert.True(t, os.IsNotExist(err))
}

func TestApplyScriptHotfix_NoManifest(t *testing.T) {
	err := applyScriptHotfix("/nonexistent/path/manifest.json", "202604.27.0")
	require.NoError(t, err)
}

func TestApplyScriptHotfix_EmptyManifest(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte("{}"), 0o644))

	err := applyScriptHotfix(manifestPath, "202604.27.0")
	require.NoError(t, err)
}

func TestApplyScriptHotfix_DevVersion(t *testing.T) {
	tempDir := t.TempDir()

	stagingDir := filepath.Join(tempDir, "hotfix", "scripts")
	require.NoError(t, os.MkdirAll(stagingDir, 0o755))

	stagingPath := filepath.Join(stagingDir, "test.sh")
	require.NoError(t, os.WriteFile(stagingPath, []byte("#!/bin/bash"), 0o744))

	destPath := filepath.Join(tempDir, "dest", "test.sh")

	manifest := fmt.Sprintf(`{"targetVersion":"202604.27.0","files":[{"staging":"%s","destination":"%s"}]}`,
		stagingPath, destPath)
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifest), 0o644))

	// Dev version can't be parsed — should skip gracefully
	err := applyScriptHotfix(manifestPath, "dev")
	require.NoError(t, err)

	_, err = os.ReadFile(destPath)
	assert.True(t, os.IsNotExist(err))
}

func TestIsScriptHotfixTargeted(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		target   string
		expected bool
		wantErr  bool
	}{
		{"exact match", "202604.27.0", "202604.27.0", true, false},
		{"same base higher patch", "202604.27.1", "202604.27.0", true, false},
		{"same base lower patch", "202604.27.0", "202604.27.1", true, false},
		{"different base", "202602.19.0", "202604.27.0", false, false},
		{"different minor", "202604.24.0", "202604.27.0", false, false},
		{"dev current", "dev", "202604.27.0", false, true},
		{"dev target", "202604.27.0", "dev", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isScriptHotfixTargeted(tt.current, tt.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}
