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
