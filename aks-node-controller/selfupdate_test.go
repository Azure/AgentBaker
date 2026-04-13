package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadHotfixVersion(t *testing.T) {
	t.Run("file does not exist", func(t *testing.T) {
		version, err := readHotfixVersion("/nonexistent/path")
		assert.NoError(t, err)
		assert.Equal(t, "", version)
	})

	t.Run("file is empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte(""), 0644))
		version, err := readHotfixVersion(path)
		assert.NoError(t, err)
		assert.Equal(t, "", version)
	})

	t.Run("file has version", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"version": "202603.30.0-hotfix1"}`), 0644))
		version, err := readHotfixVersion(path)
		assert.NoError(t, err)
		assert.Equal(t, "202603.30.0-hotfix1", version)
	})

	t.Run("file has empty version field", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"version": ""}`), 0644))
		version, err := readHotfixVersion(path)
		assert.NoError(t, err)
		assert.Equal(t, "", version)
	})

	t.Run("file has invalid JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))
		_, err := readHotfixVersion(path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parsing hotfix config")
	})

	t.Run("file has extra fields (forward compat)", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"version": "1.0.0", "sha256": "abc123"}`), 0644))
		version, err := readHotfixVersion(path)
		assert.NoError(t, err)
		assert.Equal(t, "1.0.0", version)
	})
}

func TestDetectPackageManager(t *testing.T) {
	// This test reads the real /etc/os-release so it's OS-dependent.
	// We just verify it doesn't error on the current host.
	pkgMgr, err := detectPackageManager()
	if err != nil {
		t.Skipf("skipping on unsupported OS: %v", err)
	}
	assert.Contains(t, []packageManager{pkgMgrApt, pkgMgrDnf, pkgMgrTdnf}, pkgMgr)
}

func TestResolveMicrosoftProdSourceListPath(t *testing.T) {
	t.Run("prefers legacy .list when both exist", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "microsoft-prod.list"), []byte("deb ..."), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "microsoft-prod.sources"), []byte("Types: deb"), 0644))

		path, err := resolveMicrosoftProdSourceListPath(dir)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "microsoft-prod.list"), path)
	})

	t.Run("falls back to deb822 .sources when .list is absent", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "microsoft-prod.sources"), []byte("Types: deb"), 0644))

		path, err := resolveMicrosoftProdSourceListPath(dir)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "microsoft-prod.sources"), path)
	})

	t.Run("returns error when neither file exists", func(t *testing.T) {
		dir := t.TempDir()

		_, err := resolveMicrosoftProdSourceListPath(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "neither")
	})
}

func TestSelfUpdate_NoHotfixFile(t *testing.T) {
	// When no hotfix-version file exists, selfUpdate should be a no-op.
	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "nonexistent")
	tt.App.selfUpdate(context.Background()) // should not panic
}

func TestSelfUpdate_VersionMatch(t *testing.T) {
	// When the compiled version matches the hotfix version, selfUpdate should skip.
	origVersion := Version
	Version = "202603.30.0-hotfix1"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": "202603.30.0-hotfix1"}`), 0644))

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = path
	tt.App.selfUpdate(context.Background()) // should not panic
}

func TestSelfUpdate_UnreadableFile(t *testing.T) {
	// When the hotfix file exists but is unreadable, selfUpdate should log warning and continue.
	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": "1.0.0"}`), 0644))
	require.NoError(t, os.Chmod(path, 0000))
	t.Cleanup(func() { _ = os.Chmod(path, 0644) })

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = path
	tt.App.selfUpdate(context.Background()) // should not panic, logs warning
}

func TestRetryCommand_SuccessOnFirstAttempt(t *testing.T) {
	callCount := 0
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(*exec.Cmd) error {
			callCount++
			return nil
		},
	})
	err := tt.App.retryCommand(context.Background(), "echo", "hello")
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRetryCommand_SuccessAfterRetries(t *testing.T) {
	callCount := 0
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(*exec.Cmd) error {
			callCount++
			if callCount < 3 {
				return &testExitError{Code: 100}
			}
			return nil
		},
	})
	err := tt.App.retryCommand(context.Background(), "echo", "hello")
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestRetryCommand_AllAttemptsFail(t *testing.T) {
	callCount := 0
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(*exec.Cmd) error {
			callCount++
			return &testExitError{Code: 100}
		},
	})
	err := tt.App.retryCommand(context.Background(), "false")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 5 attempts")
	assert.Equal(t, maxInstallRetries, callCount)
}

func TestRetryCommand_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(*exec.Cmd) error {
			callCount++
			cancel() // cancel after first attempt
			return &testExitError{Code: 100}
		},
	})
	err := tt.App.retryCommand(ctx, "false")
	assert.Error(t, err)
	assert.Equal(t, 1, callCount)
}

func TestCopyBinaryAlongside(t *testing.T) {
	t.Run("copies hotfix alongside and preserves VHD binary permissions", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "pkg-binary")
		vhd := filepath.Join(dir, "aks-node-controller")
		hotfix := filepath.Join(dir, "aks-node-controller-hotfix")

		require.NoError(t, os.WriteFile(vhd, []byte("original"), 0755))
		require.NoError(t, os.WriteFile(src, []byte("new-hotfix"), 0644))

		err := copyBinaryAlongside(src, hotfix, vhd)
		require.NoError(t, err)

		// Hotfix binary has the new content.
		data, err := os.ReadFile(hotfix)
		require.NoError(t, err)
		assert.Equal(t, "new-hotfix", string(data))

		// Hotfix binary has VHD binary's permissions.
		info, err := os.Stat(hotfix)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0755), info.Mode().Perm())

		// Original VHD binary is untouched.
		origData, err := os.ReadFile(vhd)
		require.NoError(t, err)
		assert.Equal(t, "original", string(origData))

		// Verify no temp files left behind.
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		for _, e := range entries {
			assert.False(t, strings.HasPrefix(e.Name(), ".aks-node-controller-update-"),
				"temp file should be cleaned up: %s", e.Name())
		}
	})

	t.Run("returns error when src missing", func(t *testing.T) {
		dir := t.TempDir()
		vhd := filepath.Join(dir, "aks-node-controller")
		require.NoError(t, os.WriteFile(vhd, []byte("original"), 0755))

		err := copyBinaryAlongside(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "hotfix"), vhd)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reading")
	})

	t.Run("returns error when refPath missing and cleans up temp", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "pkg-binary")
		require.NoError(t, os.WriteFile(src, []byte("new"), 0644))

		err := copyBinaryAlongside(src, filepath.Join(dir, "hotfix"), filepath.Join(dir, "nonexistent"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "stat")

		// No temp files should remain.
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		for _, e := range entries {
			assert.False(t, strings.HasPrefix(e.Name(), ".aks-node-controller-update-"),
				"temp file should be cleaned up: %s", e.Name())
		}
	})
}
