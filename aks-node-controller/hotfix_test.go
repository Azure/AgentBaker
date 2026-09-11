package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadHotfixConfig(t *testing.T) {
	t.Run("file does not exist returns zero config", func(t *testing.T) {
		cfg, err := readHotfixConfig("/nonexistent/path")
		assert.NoError(t, err)
		assert.Equal(t, &hotfixConfig{}, cfg)
	})

	t.Run("empty file returns zero config", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte("  \n"), 0644))
		cfg, err := readHotfixConfig(path)
		assert.NoError(t, err)
		assert.Equal(t, &hotfixConfig{}, cfg)
	})

	t.Run("parses legacy version field", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"version": "202604.01.1"}`), 0644))
		cfg, err := readHotfixConfig(path)
		require.NoError(t, err)
		assert.Equal(t, "202604.01.1", cfg.Version)
		assert.Empty(t, cfg.Hotfixes)
	})

	t.Run("parses base->version map", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"hotfixes": {"202604.01": "202604.01.1", "202605.30": "202605.30.2"}}`), 0644))
		cfg, err := readHotfixConfig(path)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"202604.01": "202604.01.1", "202605.30": "202605.30.2"}, cfg.Hotfixes)
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))
		_, err := readHotfixConfig(path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parsing hotfix config")
	})

	t.Run("ignores extra fields (forward compat)", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"version": "202604.01.1", "sha256": "abc123"}`), 0644))
		cfg, err := readHotfixConfig(path)
		require.NoError(t, err)
		assert.Equal(t, "202604.01.1", cfg.Version)
	})
}

func TestHotfixBaseFromVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
		wantErr bool
	}{
		{"standard version", "202604.01.1", "202604.01", false},
		{"preserves leading zero day", "202604.01.0", "202604.01", false},
		{"two-digit day", "202605.30.2", "202605.30", false},
		{"trims whitespace", "  202604.01.1  ", "202604.01", false},
		{"too few segments", "202604.01", "", true},
		{"empty patch segment", "202604.01.", "", true},
		{"single segment", "dev", "", true},
		{"empty", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := hotfixBaseFromVersion(tc.version)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestHotfixConfigResolveVersion(t *testing.T) {
	// requireResolves asserts a successful resolution and returns the resolved version.
	requireResolves := func(t *testing.T, cfg hotfixConfig, current string) string {
		t.Helper()
		resolved, err := cfg.resolveVersion(current)
		require.NoError(t, err)
		return resolved
	}

	t.Run("empty map falls back to legacy Version field", func(t *testing.T) {
		cfg := hotfixConfig{Version: "202604.01.1"}
		assert.Equal(t, "202604.01.1", requireResolves(t, cfg, "202604.01.0"))
	})

	t.Run("empty config resolves to empty", func(t *testing.T) {
		cfg := hotfixConfig{}
		assert.Equal(t, "", requireResolves(t, cfg, "202604.01.0"))
	})

	t.Run("map hit returns matching base entry", func(t *testing.T) {
		cfg := hotfixConfig{Hotfixes: map[string]string{
			"202604.01": "202604.01.1",
			"202605.30": "202605.30.2",
		}}
		assert.Equal(t, "202604.01.1", requireResolves(t, cfg, "202604.01.0"))
		assert.Equal(t, "202605.30.2", requireResolves(t, cfg, "202605.30.0"))
	})

	t.Run("map miss returns empty (default deny for unlisted base)", func(t *testing.T) {
		cfg := hotfixConfig{Hotfixes: map[string]string{"202604.01": "202604.01.1"}}
		assert.Equal(t, "", requireResolves(t, cfg, "202606.09.0"))
	})

	t.Run("map preserves leading-zero day matching", func(t *testing.T) {
		cfg := hotfixConfig{Hotfixes: map[string]string{"202604.01": "202604.01.1"}}
		assert.Equal(t, "202604.01.1", requireResolves(t, cfg, "202604.01.0"))
	})

	t.Run("map takes precedence over legacy Version field", func(t *testing.T) {
		cfg := hotfixConfig{
			Version:  "202604.01.9",
			Hotfixes: map[string]string{"202604.01": "202604.01.1"},
		}
		assert.Equal(t, "202604.01.1", requireResolves(t, cfg, "202604.01.0"))
	})

	t.Run("unparseable current version with map reports an error", func(t *testing.T) {
		cfg := hotfixConfig{Hotfixes: map[string]string{"202604.01": "202604.01.1"}}
		resolved, err := cfg.resolveVersion("dev")
		require.Error(t, err)
		assert.Equal(t, "", resolved)
	})
}

func TestDetectPackageManager(t *testing.T) {
	t.Run("real host os-release", func(t *testing.T) {
		// This exercises the real /etc/os-release so it's OS-dependent.
		// We just verify it doesn't error on the current host.
		a := &App{}
		pkgMgr, err := a.detectPackageManager()
		if err != nil {
			t.Skipf("skipping on unsupported OS: %v", err)
		}
		assert.Contains(t, []packageManager{pkgMgrApt, pkgMgrDnf, pkgMgrTdnf}, pkgMgr)
	})

	t.Run("ubuntu returns apt", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(path, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\n"), 0644))
		a := &App{osReleasePath: path}
		pkgMgr, err := a.detectPackageManager()
		require.NoError(t, err)
		assert.Equal(t, pkgMgrApt, pkgMgr)
	})

	t.Run("azurelinux returns dnf or tdnf", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(path, []byte(`ID="azurelinux"`+"\n"), 0644))
		a := &App{osReleasePath: path}
		pkgMgr, err := a.detectPackageManager()
		require.NoError(t, err)
		assert.Contains(t, []packageManager{pkgMgrDnf, pkgMgrTdnf}, pkgMgr)
	})

	t.Run("unsupported OS errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(path, []byte("ID=windows\n"), 0644))
		a := &App{osReleasePath: path}
		_, err := a.detectPackageManager()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported OS")
	})

	t.Run("legacy mariner is unsupported", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(path, []byte("ID=mariner\n"), 0644))
		a := &App{osReleasePath: path}
		_, err := a.detectPackageManager()
		require.ErrorContains(t, err, "unsupported OS: mariner")
	})

	t.Run("ACL azurelinux variant reports self-update unsupported", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(
			path,
			[]byte("ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n"),
			0644,
		))
		a := &App{osReleasePath: path}

		_, err := a.detectPackageManager()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not supported on image-based OS")
		assert.Contains(t, err.Error(), "azurecontainerlinux")
	})

	t.Run("missing ID line errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(path, []byte("VERSION_ID=1\n"), 0644))
		a := &App{osReleasePath: path}
		_, err := a.detectPackageManager()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ID not found")
	})

	t.Run("unreadable os-release errors", func(t *testing.T) {
		a := &App{osReleasePath: filepath.Join(t.TempDir(), "does-not-exist")}
		_, err := a.detectPackageManager()
		require.Error(t, err)
	})
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

func TestDownloadHotfix_MatchingBaseWithArtifactsUsesPackageManager(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"version": "202604.01.1",
		"artifacts": {
			"202604.01.1": {
				"linux-ubuntu-22.04-amd64": {
					"url": "https://packages.microsoft.com/fake.deb",
					"sha256": "abc123"
				}
			}
		}
	}`), 0o644))

	aptDir := filepath.Join(dir, "sources.list.d")
	require.NoError(t, os.MkdirAll(aptDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(aptDir, "microsoft-prod.list"), []byte("deb ..."), 0o644))

	// Provide a fake /etc/os-release so detectPackageManager resolves deterministically
	// to apt-get regardless of the host OS running this test (e.g. macOS dev machines).
	osReleasePath := filepath.Join(dir, "os-release")
	require.NoError(t, os.WriteFile(osReleasePath, []byte("ID=ubuntu\n"), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(cmd *exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	tt.App.aptSourcesDir = aptDir
	tt.App.osReleasePath = osReleasePath
	// Will fail at copyBinaryAlongside since pkgBinaryPath doesn't exist in test,
	// but install should have been called.
	err := tt.App.downloadHotfix(context.Background())
	require.Error(t, err)
	assert.True(t, installCalled, "artifacts must be ignored and the package manager must install the hotfix")
}

func TestDownloadHotfix_UnreadableFileFailsOpen(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": "202604.01.1"}`), 0o644))
	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if _, err := os.ReadFile(path); err == nil {
		require.NoError(t, os.Remove(path))
		require.NoError(t, os.Mkdir(path, 0o755))
	}

	aptDir := filepath.Join(dir, "sources.list.d")
	require.NoError(t, os.MkdirAll(aptDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(aptDir, "microsoft-prod.list"), []byte("deb ..."), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(*exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	tt.App.aptSourcesDir = aptDir
	// Fail-open: an unreadable config must skip the hotfix without erroring,
	// so download-hotfix never blocks provisioning.
	require.NoError(t, tt.App.downloadHotfix(context.Background()))
	assert.False(t, installCalled, "should skip install when the config cannot be read")

	events := tt.eventLogger.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "AKS.AKSNodeController.Hotfix.BinaryOperation", events[0].TaskName)
	assert.Equal(t, "Error", events[0].EventLevel)
	assert.Contains(t, events[0].Message, "current=202604.01.0")
	assert.Contains(t, events[0].Message, "target=")
	assert.Contains(t, events[0].Message, "route=none")
	assert.Contains(t, events[0].Message, "outcome=skipped-config-error")
	assert.Contains(t, events[0].Message, "configPath="+path)
	assert.Contains(t, events[0].Message, "error=")
	assert.Contains(t, events[0].Message, "durationMs=")
}

func TestDownloadHotfix_InvalidJSONFailsOpen(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))

	aptDir := filepath.Join(dir, "sources.list.d")
	require.NoError(t, os.MkdirAll(aptDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(aptDir, "microsoft-prod.list"), []byte("deb ..."), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(*exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	tt.App.aptSourcesDir = aptDir
	// Fail-open: malformed JSON must skip the hotfix without erroring.
	require.NoError(t, tt.App.downloadHotfix(context.Background()))
	assert.False(t, installCalled, "should skip install when the config is invalid JSON")

	events := tt.eventLogger.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "AKS.AKSNodeController.Hotfix.BinaryOperation", events[0].TaskName)
	assert.Equal(t, "Error", events[0].EventLevel)
	assert.Contains(t, events[0].Message, "current=202604.01.0")
	assert.Contains(t, events[0].Message, "target=")
	assert.Contains(t, events[0].Message, "route=none")
	assert.Contains(t, events[0].Message, "outcome=skipped-config-error")
	assert.Contains(t, events[0].Message, "configPath="+path)
	assert.Contains(t, events[0].Message, "error=")
	assert.Contains(t, events[0].Message, "durationMs=")
}

func TestDownloadHotfix_MapBaseNotPresentSkips(t *testing.T) {
	origVersion := Version
	Version = "202606.09.0"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"hotfixes": {"202604.01": "202604.01.1"}}`), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(cmd *exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	require.NoError(t, tt.App.downloadHotfix(context.Background()))
	assert.False(t, installCalled, "should skip when VHD base is not a key in the hotfixes map")
}

func TestDownloadHotfix_MapMatchingBaseUpgrades(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"hotfixes": {"202604.01": "202604.01.1", "202605.30": "202605.30.2"}}`), 0o644))

	aptDir := filepath.Join(dir, "sources.list.d")
	require.NoError(t, os.MkdirAll(aptDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(aptDir, "microsoft-prod.list"), []byte("deb ..."), 0o644))

	// Provide a fake /etc/os-release so detectPackageManager resolves deterministically
	// to apt-get regardless of the host OS running this test (e.g. macOS dev machines).
	osReleasePath := filepath.Join(dir, "os-release")
	require.NoError(t, os.WriteFile(osReleasePath, []byte("ID=ubuntu\n"), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(cmd *exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	tt.App.aptSourcesDir = aptDir
	tt.App.osReleasePath = osReleasePath
	// Will fail at copyBinaryAlongside since pkgBinaryPath doesn't exist in test,
	// but install should have been called for the matching base entry.
	err := tt.App.downloadHotfix(context.Background())
	require.Error(t, err)
	assert.True(t, installCalled, "should install the hotfix for the matching base entry")
}

func TestDownloadHotfix_MapPatchNotHigherSkips(t *testing.T) {
	origVersion := Version
	Version = "202604.01.2"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	// Map entry resolves for this base, but patch (1) is not strictly higher than baked (2).
	require.NoError(t, os.WriteFile(path, []byte(`{"hotfixes": {"202604.01": "202604.01.1"}}`), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(cmd *exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	require.NoError(t, tt.App.downloadHotfix(context.Background()))
	assert.False(t, installCalled, "should skip when resolved hotfix patch is not strictly higher")
}

func TestDownloadHotfix_MapMisconfiguredValueBaseSkips(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	dir := t.TempDir()
	path := filepath.Join(dir, "hotfix-config.json")
	// Misconfigured entry: the value's base (202605.30) does not match its key (202604.01).
	// resolveVersion selects it by key for a node on base 202604.01, but shouldUpgradeToHotfix
	// must reject it because the YYYYMM.DD bases differ, so no wrong-base binary is installed.
	require.NoError(t, os.WriteFile(path, []byte(`{"hotfixes": {"202604.01": "202605.30.2"}}`), 0o644))

	installCalled := false
	tt := NewTestApp(t, TestAppConfig{
		RunFunc: func(cmd *exec.Cmd) error {
			installCalled = true
			return nil
		},
	})
	tt.App.hotfixVersionPath = path
	require.NoError(t, tt.App.downloadHotfix(context.Background()))
	assert.False(t, installCalled, "should skip when the map value's base does not match the node's base")
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
		{"base .0 -> hotfix .1", "202604.01.0", "202604.01.1", true, false},
		{"base .0 -> hotfix .2", "202604.01.0", "202604.01.2", true, false},
		{"hotfix .1 -> hotfix .2", "202604.01.1", "202604.01.2", true, false},
		{"hotfix .0 -> hotfix .1-1", "202604.01.0", "202604.01.1-1", true, false},

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

func TestDownloadHotfixGuestAgentTimingEvents(t *testing.T) {
	tests := []struct {
		name          string
		current       string
		setup         func(t *testing.T, app *App, dir string) *hotfixConfig
		wantErr       bool
		wantTaskNames []string
		wantLevel     string
		wantMessage   []string
	}{
		{
			name:    "no hotfix version skips",
			current: "202604.01.0",
			setup: func(t *testing.T, app *App, dir string) *hotfixConfig {
				return &hotfixConfig{}
			},
			wantTaskNames: []string{"AKS.AKSNodeController.Hotfix.BinaryOperation"},
			wantLevel:     "Informational",
			wantMessage:   []string{"current=202604.01.0", "route=none", "outcome=skipped-no-version"},
		},
		{
			name:    "non-targeted version skips",
			current: "202604.01.2",
			setup: func(t *testing.T, app *App, dir string) *hotfixConfig {
				return &hotfixConfig{Version: "202604.01.1"}
			},
			wantTaskNames: []string{"AKS.AKSNodeController.Hotfix.BinaryOperation"},
			wantLevel:     "Informational",
			wantMessage:   []string{"target=202604.01.1", "route=none", "outcome=skipped-not-targeted"},
		},
		{
			name:    "malformed current version skips",
			current: "dev",
			setup: func(t *testing.T, app *App, dir string) *hotfixConfig {
				return &hotfixConfig{Version: "202604.01.1"}
			},
			wantTaskNames: []string{"AKS.AKSNodeController.Hotfix.BinaryOperation"},
			wantLevel:     "Informational",
			wantMessage:   []string{"current=dev", "route=none", "outcome=skipped-version-compare-error"},
		},
		{
			name:    "map-based malformed current version skips",
			current: "dev",
			setup: func(t *testing.T, app *App, dir string) *hotfixConfig {
				return &hotfixConfig{Hotfixes: map[string]string{"202604.01": "202604.01.1"}}
			},
			wantTaskNames: []string{"AKS.AKSNodeController.Hotfix.BinaryOperation"},
			wantLevel:     "Informational",
			wantMessage:   []string{"current=dev", "route=none", "outcome=skipped-version-compare-error"},
		},
		{
			name:    "package manager success",
			current: "202604.01.0",
			setup: func(t *testing.T, app *App, dir string) *hotfixConfig {
				app.osReleasePath = writeOSRelease(t, dir, "ID=ubuntu\n")
				app.aptSourcesDir = writeAptSourcesDir(t, dir)
				app.vhdBinaryPath = writeExecutable(t, dir, "vhd-anc", "original")
				app.pkgBinaryPath = writeExecutable(t, dir, "pkg-anc", "package-manager-hotfix")
				app.hotfixBinaryPath = filepath.Join(dir, "hotfix-anc")
				return &hotfixConfig{Version: "202604.01.1"}
			},
			wantTaskNames: []string{
				"AKS.AKSNodeController.Hotfix.AptDpkgConfigure",
				"AKS.AKSNodeController.Hotfix.AptUpdate",
				"AKS.AKSNodeController.Hotfix.AptInstall",
				"AKS.AKSNodeController.Hotfix.PackageManagerInstall",
				"AKS.AKSNodeController.Hotfix.BinaryStaging",
				"AKS.AKSNodeController.Hotfix.BinaryOperation",
			},
			wantLevel:   "Informational",
			wantMessage: []string{"target=202604.01.1", "route=package-manager", "outcome=success"},
		},
		{
			name:    "rpm package manager success",
			current: "202604.01.0",
			setup: func(t *testing.T, app *App, dir string) *hotfixConfig {
				app.osReleasePath = writeOSRelease(t, dir, "ID=azurelinux\n")
				app.vhdBinaryPath = writeExecutable(t, dir, "vhd-anc", "original")
				app.pkgBinaryPath = writeExecutable(t, dir, "pkg-anc", "package-manager-hotfix")
				app.hotfixBinaryPath = filepath.Join(dir, "hotfix-anc")
				return &hotfixConfig{Version: "202604.01.1"}
			},
			wantTaskNames: []string{
				"AKS.AKSNodeController.Hotfix.RpmInstall",
				"AKS.AKSNodeController.Hotfix.PackageManagerInstall",
				"AKS.AKSNodeController.Hotfix.BinaryStaging",
				"AKS.AKSNodeController.Hotfix.BinaryOperation",
			},
			wantLevel: "Informational",
		},
		{
			name:    "package manager install failure",
			current: "202604.01.0",
			setup: func(t *testing.T, app *App, dir string) *hotfixConfig {
				app.osReleasePath = writeOSRelease(t, dir, "ID=unsupported\n")
				return &hotfixConfig{Version: "202604.01.1"}
			},
			wantErr: true,
			wantTaskNames: []string{
				"AKS.AKSNodeController.Hotfix.PackageManagerInstall",
				"AKS.AKSNodeController.Hotfix.BinaryOperation",
			},
			wantLevel:   "Error",
			wantMessage: []string{"target=202604.01.1", "route=package-manager", "outcome=failed"},
		},
		{
			name:    "package manager staging failure",
			current: "202604.01.0",
			setup: func(t *testing.T, app *App, dir string) *hotfixConfig {
				app.osReleasePath = writeOSRelease(t, dir, "ID=ubuntu\n")
				app.aptSourcesDir = writeAptSourcesDir(t, dir)
				app.vhdBinaryPath = writeExecutable(t, dir, "vhd-anc", "original")
				app.pkgBinaryPath = filepath.Join(dir, "missing-pkg-anc")
				app.hotfixBinaryPath = filepath.Join(dir, "hotfix-anc")
				return &hotfixConfig{Version: "202604.01.1"}
			},
			wantErr: true,
			wantTaskNames: []string{
				"AKS.AKSNodeController.Hotfix.AptDpkgConfigure",
				"AKS.AKSNodeController.Hotfix.AptUpdate",
				"AKS.AKSNodeController.Hotfix.AptInstall",
				"AKS.AKSNodeController.Hotfix.PackageManagerInstall",
				"AKS.AKSNodeController.Hotfix.BinaryStaging",
				"AKS.AKSNodeController.Hotfix.BinaryOperation",
			},
			wantLevel:   "Error",
			wantMessage: []string{"target=202604.01.1", "route=package-manager", "outcome=failed"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			origVersion := Version
			Version = tc.current
			t.Cleanup(func() { Version = origVersion })

			dir := t.TempDir()
			tt := NewTestApp(t, TestAppConfig{})
			cfg := tc.setup(t, tt.App, dir)
			configData, err := json.Marshal(cfg)
			require.NoError(t, err)
			tt.App.hotfixVersionPath = filepath.Join(dir, "hotfix-config.json")
			tt.App.hotfixTimingPath = filepath.Join(dir, "hotfix-timing.json")
			require.NoError(t, os.WriteFile(tt.App.hotfixVersionPath, configData, 0o644))

			err = tt.App.downloadHotfix(context.Background())
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			events := tt.eventLogger.Events()
			require.Len(t, events, len(tc.wantTaskNames))
			for i, taskName := range tc.wantTaskNames {
				assert.Equal(t, taskName, events[i].TaskName)
				assert.Contains(t, events[i].Message, "durationMs=")
				assert.Contains(t, events[i].Message, "outcome=")
				switch {
				case strings.Contains(taskName, "Apt"):
					assert.Contains(t, events[i].Message, "version=202604.01.1")
				case strings.Contains(taskName, "PackageManagerInstall"):
					assert.Contains(t, events[i].Message, "target=202604.01.1")
					assert.Contains(t, events[i].Message, "route=package-manager")
				case strings.Contains(taskName, "BinaryStaging"):
					assert.Contains(t, events[i].Message, "target=202604.01.1")
					assert.Contains(t, events[i].Message, "source=")
					assert.Contains(t, events[i].Message, "destination=")
				}
			}
			assert.Equal(t, tc.wantLevel, events[len(events)-1].EventLevel)
			for _, expected := range tc.wantMessage {
				assert.Contains(t, events[len(events)-1].Message, expected)
			}

			var timing hotfixTiming
			timingBytes, err := os.ReadFile(tt.App.hotfixTimingPath)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(timingBytes, &timing))
			assert.Equal(t, tc.current, timing.Current)
			assert.NotEqual(t, hotfixOutcomeStarted, timing.Outcome)
			assert.GreaterOrEqual(t, timing.DurationMs, int64(0))
			if tc.wantErr {
				assert.NotEmpty(t, timing.Error)
			} else {
				assert.Empty(t, timing.Error)
			}
		})
	}
}

func writeOSRelease(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "os-release")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func writeAptSourcesDir(t *testing.T, dir string) string {
	t.Helper()
	aptDir := filepath.Join(dir, "sources.list.d")
	require.NoError(t, os.MkdirAll(aptDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(aptDir, "microsoft-prod.list"), []byte("deb ..."), 0o644))
	return aptDir
}

func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}
