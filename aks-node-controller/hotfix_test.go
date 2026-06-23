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
		require.NoError(t, os.WriteFile(path, []byte(`{"version": "1.0.0", "target_version":"202604.01.0", "sha256": "abc123"}`), 0644))
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

func TestDownloadHotfix_NoHotfixFile(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "nonexistent")
	require.NoError(t, tt.App.downloadHotfix(context.Background()))
}

func TestDownloadHotfix_MaterializesHotfixConfigFromNodeCustomData(t *testing.T) {
	origVersion := Version
	Version = "202604.01.1"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	hotfixPath := filepath.Join(dir, "hotfix-config.json")
	nodeCustomDataPath := filepath.Join(dir, "nodecustomdata.yml")
	require.NoError(t, os.WriteFile(nodeCustomDataPath, []byte(
		`#cloud-config
write_files:
- path: `+hotfixPath+`
  permissions: "0644"
  owner: root
  content: |
    {"version":"202604.01.1","target_version":"202604.01.1"}
`), 0o644))

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = hotfixPath
	tt.App.nodeCustomDataPath = nodeCustomDataPath
	require.NoError(t, tt.App.downloadHotfix(context.Background()))

	data, err := os.ReadFile(hotfixPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"version":"202604.01.1"`)
}

func TestDownloadHotfix_VersionMatch(t *testing.T) {
	origVersion := Version
	Version = "202604.01.1"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": "202604.01.1"}`), 0o644))

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = path
	require.NoError(t, tt.App.downloadHotfix(context.Background()))
}

func TestDownloadHotfix_DifferentBaseSkips(t *testing.T) {
	origVersion := Version
	Version = "202605.01.0"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": "202604.01.1"}`), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(cmd *exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	require.NoError(t, tt.App.downloadHotfix(context.Background()))
	assert.False(t, installCalled, "should skip when version base doesn't match")
}

func TestDownloadHotfix_DevVersionSkips(t *testing.T) {
	origVersion := Version
	Version = "dev"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": "202604.01.1"}`), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(cmd *exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	require.NoError(t, tt.App.downloadHotfix(context.Background()))
	assert.False(t, installCalled, "should skip when Version is 'dev' (parse error)")
}

func TestDownloadHotfix_MatchingBaseUpgrades(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": "202604.01.1"}`), 0o644))

	aptDir := filepath.Join(dir, "sources.list.d")
	require.NoError(t, os.MkdirAll(aptDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(aptDir, "microsoft-prod.list"), []byte("deb ..."), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(cmd *exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	tt.App.aptSourcesDir = aptDir
	// Will fail at copyBinaryAlongside since pkgBinaryPath doesn't exist in test,
	// but install should have been called.
	err := tt.App.downloadHotfix(context.Background())
	require.Error(t, err)
	assert.True(t, installCalled, "should proceed when base matches and hotfix patch is higher")
}

func TestDownloadHotfix_UnreadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": "1.0.0"}`), 0o644))
	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = path
	require.Error(t, tt.App.downloadHotfix(context.Background()))
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

func TestShouldUpgradeToHotfix(t *testing.T) {
	tests := []struct {
		name    string
		current string
		hotfix  string
		want    bool
		wantErr bool
	}{
		// Positive: same base, hotfix has higher patch
		{"base .0 → hotfix .1", "202604.01.0", "202604.01.1", true, false},
		{"base .0 → hotfix .2", "202604.01.0", "202604.01.2", true, false},
		{"hotfix .1 → hotfix .2", "202604.01.1", "202604.01.2", true, false},

		// Negative: same version
		{"same version .0", "202604.01.0", "202604.01.0", false, false},
		{"same version .1", "202604.01.1", "202604.01.1", false, false},

		// Negative: different base (different YYYYMM)
		{"different month", "202603.15.0", "202604.01.1", false, false},
		{"newer month", "202605.01.0", "202604.01.1", false, false},

		// Negative: different base (different DD)
		{"different day", "202604.15.0", "202604.01.1", false, false},

		// Negative: current patch higher than hotfix
		{"current patch higher", "202604.01.2", "202604.01.1", false, false},

		// Error cases
		{"dev current", "dev", "202604.01.1", false, true},
		{"dev hotfix", "202604.01.0", "dev", false, true},
		{"both dev", "dev", "dev", false, true},
		{"empty current", "", "202604.01.1", false, true},
		{"empty hotfix", "202604.01.0", "", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := shouldUpgradeToHotfix(tc.current, tc.hotfix)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got, "current=%s hotfix=%s", tc.current, tc.hotfix)
			}
		})
	}
}

func TestShouldApplyTargetVersion(t *testing.T) {
	tests := []struct {
		name    string
		current string
		target  string
		want    bool
	}{
		{"target omitted applies", "202604.01.1", "", true},
		{"base target applies any patch", "202604.01.1", "202604.01", true},
		{"base target applies patch zero", "202604.01.0", "202604.01", true},
		{"base target mismatch skips", "202604.02.1", "202604.01", false},
		{"exact match applies", "202604.01.1", "202604.01.1", true},
		{"trimmed exact match applies", " 202604.01.1 ", "202604.01.1", true},
		{"mismatch skips", "202604.01.1", "202604.01.0", false},
		{"invalid target skips", "202604.01.1", "202604", false},
		{"invalid current skips", "dev", "202604.01", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, shouldApplyTargetVersion(tc.current, tc.target))
		})
	}
}
