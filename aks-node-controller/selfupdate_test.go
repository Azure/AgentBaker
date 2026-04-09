package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		path := filepath.Join(t.TempDir(), "hotfix-version")
		require.NoError(t, os.WriteFile(path, []byte(""), 0644))
		version, err := readHotfixVersion(path)
		assert.NoError(t, err)
		assert.Equal(t, "", version)
	})

	t.Run("file has version with newline", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-version")
		require.NoError(t, os.WriteFile(path, []byte("202603.30.0-hotfix1\n"), 0644))
		version, err := readHotfixVersion(path)
		assert.NoError(t, err)
		assert.Equal(t, "202603.30.0-hotfix1", version)
	})

	t.Run("file has version with whitespace", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-version")
		require.NoError(t, os.WriteFile(path, []byte("  202603.30.0-hotfix1  \n"), 0644))
		version, err := readHotfixVersion(path)
		assert.NoError(t, err)
		assert.Equal(t, "202603.30.0-hotfix1", version)
	})
}

func TestDetectPackageManager(t *testing.T) {
	// This test reads the real /etc/os-release so it's OS-dependent.
	// We just verify it doesn't error on the current host.
	pkgMgr, err := detectPackageManager()
	if err != nil {
		t.Skipf("skipping on unsupported OS: %v", err)
	}
	assert.Contains(t, []string{"apt-get", "dnf", "tdnf"}, pkgMgr)
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
	err := tt.App.selfUpdate(context.Background())
	assert.NoError(t, err)
}

func TestSelfUpdate_VersionMatch(t *testing.T) {
	// When the compiled version matches the hotfix version, selfUpdate should skip.
	origVersion := Version
	Version = "202603.30.0-hotfix1"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-version")
	require.NoError(t, os.WriteFile(path, []byte("202603.30.0-hotfix1\n"), 0644))

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = path
	err := tt.App.selfUpdate(context.Background())
	assert.NoError(t, err)
}

func TestSelfUpdate_UnreadableFile(t *testing.T) {
	// When the hotfix file exists but is unreadable, selfUpdate should log warning and continue.
	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-version")
	require.NoError(t, os.WriteFile(path, []byte("1.0.0\n"), 0644))
	require.NoError(t, os.Chmod(path, 0000))
	t.Cleanup(func() { os.Chmod(path, 0644) })

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = path
	err := tt.App.selfUpdate(context.Background())
	assert.NoError(t, err) // best-effort: returns nil on error
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
