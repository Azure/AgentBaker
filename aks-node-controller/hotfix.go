package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	defaultHotfixVersionPath = "/opt/azure/containers/aks-node-controller-hotfix.json"
	// defaultScriptsHotfixFilesPath is the script-hotfix write_files payload shipped by the
	// boothook only when a script hotfix is active. download-hotfix applies it when the
	// running ANC/VHD version is targeted by scripts_version.
	defaultScriptsHotfixFilesPath = "/opt/azure/containers/anc-scripts-hotfix-files.yml"
	maxInstallRetries             = 5
	retryBackoff                  = 3 * time.Second
	commandTimeout                = 60 * time.Second
	defaultAptSourcesDir          = "/etc/apt/sources.list.d"
	// vhdBinaryPath is where packer installs the VHD-baked binary.
	vhdBinaryPath = "/opt/azure/containers/aks-node-controller"
	// hotfixBinaryPath is where the hotfix binary is placed alongside the VHD-baked binary.
	// The wrapper script checks for this path and prefers it over the VHD-baked binary.
	hotfixBinaryPath = "/opt/azure/containers/aks-node-controller-hotfix"
	// pkgBinaryPath is where apt/dnf package installs the binary.
	pkgBinaryPath = "/usr/bin/aks-node-controller"
)

// downloadHotfix applies any active hotfix for the running ANC/VHD version. It is the
// single place where version-gated remediation happens:
//   - the ANC binary hotfix (gated by `version`), staged alongside the VHD-baked binary
//   - the script hotfix write_files (gated by `scripts_version`), materialized to disk
//
// Running both here means that by the time `provision` runs, the correct binary and the
// correct scripts are already in place — no marker-based stripping of nodecustomdata.yml.
func (a *App) downloadHotfix(ctx context.Context) error {
	hotfixPath := a.hotfixVersionPath
	if hotfixPath == "" {
		hotfixPath = defaultHotfixVersionPath
	}

	hotfixCfg, exists, err := readHotfixConfig(hotfixPath)
	if err != nil {
		return fmt.Errorf("read hotfix config from %s: %w", hotfixPath, err)
	}
	if !exists {
		return nil
	}

	if err := a.downloadHotfixBinary(ctx, hotfixCfg); err != nil {
		return err
	}
	return a.applyScriptHotfix(hotfixCfg)
}

// downloadHotfixBinary installs the requested ANC binary hotfix and stages it alongside
// the VHD-baked binary. The wrapper script decides which binary to execute afterwards.
// It is a no-op when no binary version is requested or the running version isn't targeted.
func (a *App) downloadHotfixBinary(ctx context.Context, hotfixCfg hotfixConfig) error {
	hotfixVersion := hotfixCfg.Version

	if hotfixVersion == "" {
		slog.Debug("hotfix config does not request ANC version, skipping ANC binary download")
		return nil
	}

	// Patch-only matching: only upgrade if same YYYYMM.DD base and hotfix has
	// a strictly higher PATCH. Parse errors (e.g. "dev" builds) result in skip.
	shouldUpgrade, err := shouldUpgradeToHotfix(Version, hotfixVersion)
	if err != nil {
		slog.Warn("failed to compare versions, skipping hotfix download",
			"current", Version, "hotfix", hotfixVersion, "error", err)
		return nil
	}
	if !shouldUpgrade {
		slog.Info("ANC version not targeted by hotfix, skipping download",
			"current", Version, "hotfix", hotfixVersion)
		return nil
	}

	slog.Info("downloading ANC hotfix", "current", Version, "target", hotfixVersion)

	if err := a.installFromPMC(ctx, hotfixVersion); err != nil {
		return fmt.Errorf("install hotfix version %s: %w", hotfixVersion, err)
	}

	if err := copyBinaryAlongside(pkgBinaryPath, hotfixBinaryPath, vhdBinaryPath); err != nil {
		return fmt.Errorf("stage hotfix binary: %w", err)
	}

	slog.Info("downloaded ANC hotfix", "target", hotfixVersion, "path", hotfixBinaryPath)
	return nil
}

// applyScriptHotfix materializes the script hotfix write_files when the running ANC/VHD
// version is targeted by scripts_version. The payload file is shipped by the boothook only
// when a script hotfix is active, so a missing file is a no-op.
func (a *App) applyScriptHotfix(hotfixCfg hotfixConfig) error {
	filesPath := a.scriptsHotfixFilesPath
	if filesPath == "" {
		filesPath = defaultScriptsHotfixFilesPath
	}

	if !shouldApplyScriptsVersion(Version, hotfixCfg.ScriptsVersion) {
		if _, err := os.Stat(filesPath); err == nil {
			slog.Info("skipping script hotfix due to scripts_version mismatch",
				"current", Version, "target", hotfixCfg.ScriptsVersion)
		}
		return nil
	}

	if err := applyWriteFiles(filesPath); err != nil {
		return fmt.Errorf("apply script hotfix write files: %w", err)
	}
	return nil
}

// hotfixConfig is the JSON structure of the hotfix configuration file.
// Using JSON allows future extension (e.g., adding checksum, source URL) without format changes.
type hotfixConfig struct {
	Version        string `json:"version"`         // ANC binary hotfix version to download/install.
	ScriptsVersion string `json:"scripts_version"` // ANC/VHD version base that should receive the script hotfix write_files.
}

// readHotfixVersion reads and parses the JSON hotfix config from the given path.
// Returns empty string if the file doesn't exist or contains an empty version.
func readHotfixVersion(path string) (string, error) {
	cfg, _, err := readHotfixConfig(path)
	return cfg.Version, err
}

func readHotfixConfig(path string) (hotfixConfig, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return hotfixConfig{}, false, nil
		}
		return hotfixConfig{}, false, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return hotfixConfig{}, true, nil
	}
	var cfg hotfixConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return hotfixConfig{}, true, fmt.Errorf("parsing hotfix config %s: %w", path, err)
	}
	// Normalize once at this boundary so downstream comparisons (shouldUpgradeToHotfix,
	// shouldApplyScriptsVersion) can trust the values without re-trimming. The config is
	// generated by our tooling, but an operator-edited source could leave incidental
	// whitespace inside a quoted value.
	cfg.Version = strings.TrimSpace(cfg.Version)
	cfg.ScriptsVersion = strings.TrimSpace(cfg.ScriptsVersion)
	return cfg, true, nil
}

func shouldApplyScriptsVersion(currentVersion, targetVersion string) bool {
	// scripts_version supports:
	// - YYYYMM.DD       => match any patch under the same base
	// - YYYYMM.DD.PATCH => exact match
	// Empty scripts_version means no scoping (applies to all versions).
	if targetVersion == "" {
		return true
	}
	cv, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false
	}

	switch strings.Count(targetVersion, ".") {
	case 1:
		tv, err := semver.NewVersion(targetVersion + ".0")
		if err != nil {
			return false
		}
		return cv.Major() == tv.Major() && cv.Minor() == tv.Minor()
	case 2:
		tv, err := semver.NewVersion(targetVersion)
		if err != nil {
			return false
		}
		return cv.Equal(tv)
	default:
		return false
	}
}

// packageManager represents a supported system package manager.
type packageManager string

const (
	pkgMgrApt  packageManager = "apt-get"
	pkgMgrDnf  packageManager = "dnf"
	pkgMgrTdnf packageManager = "tdnf"
)

// detectPackageManager returns the package manager for the current OS.
// It reads /etc/os-release to determine whether to use apt-get (Ubuntu) or dnf/tdnf (AzureLinux/Mariner).
func detectPackageManager() (packageManager, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", fmt.Errorf("reading /etc/os-release: %w", err)
	}
	content := string(data)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			id := strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
			id = strings.ToLower(id)
			switch id {
			case "ubuntu":
				return pkgMgrApt, nil
			case "azurelinux", "mariner":
				return preferredRpmManager(), nil
			default:
				return "", fmt.Errorf("unsupported OS: %s", id)
			}
		}
	}
	return "", fmt.Errorf("ID not found in /etc/os-release")
}

// preferredRpmManager returns dnf if available, falling back to tdnf (used by OS Guard).
func preferredRpmManager() packageManager {
	if _, err := exec.LookPath("dnf"); err == nil {
		return pkgMgrDnf
	}
	return pkgMgrTdnf
}

// installFromPMC installs the hotfix package from PMC using the system package manager.
func (a *App) installFromPMC(ctx context.Context, version string) error {
	pkgMgr, err := detectPackageManager()
	if err != nil {
		return err
	}

	switch pkgMgr {
	case pkgMgrApt:
		return a.installWithApt(ctx, version)
	case pkgMgrDnf, pkgMgrTdnf:
		return a.installWithRpm(ctx, string(pkgMgr), version)
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgMgr)
	}
}

// installWithApt refreshes the PMC repo index and installs the package via apt-get.
func (a *App) installWithApt(ctx context.Context, version string) error {
	sourcesDir := a.aptSourcesDir
	if sourcesDir == "" {
		sourcesDir = defaultAptSourcesDir
	}
	microsoftProdSourceListPath, err := resolveMicrosoftProdSourceListPath(sourcesDir)
	if err != nil {
		return err
	}

	// Ensure any interrupted dpkg state is reconciled before running apt operations.
	if err := a.retryCommand(ctx, "env", "DEBIAN_FRONTEND=noninteractive",
		"dpkg", "--configure", "-a", "--force-confdef", "--force-confold"); err != nil {
		return fmt.Errorf("dpkg --configure -a failed: %w", err)
	}

	// Refresh only the microsoft-prod repo to minimize time.
	if err := a.retryCommand(ctx, "env", "DEBIAN_FRONTEND=noninteractive",
		"apt-get", "update",
		"-o", "Dpkg::Options::=--force-confold",
		"-o", fmt.Sprintf("Dir::Etc::sourcelist=%s", microsoftProdSourceListPath),
		"-o", "Dir::Etc::sourceparts=-"); err != nil {
		return fmt.Errorf("apt-get update failed: %w", err)
	}
	// Install with --allow-downgrades in case the hotfix is older than the VHD-baked version.
	return a.retryCommand(ctx, "env", "DEBIAN_FRONTEND=noninteractive",
		"apt-get", "install", "-y", "--allow-downgrades",
		"-o", "Dpkg::Options::=--force-confold",
		fmt.Sprintf("aks-node-controller=%s*", version))
}

func resolveMicrosoftProdSourceListPath(sourcesDir string) (string, error) {
	legacyListPath := filepath.Join(sourcesDir, "microsoft-prod.list")
	if _, err := os.Stat(legacyListPath); err == nil {
		return legacyListPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking %s: %w", legacyListPath, err)
	}

	deb822SourcesPath := filepath.Join(sourcesDir, "microsoft-prod.sources")
	if _, err := os.Stat(deb822SourcesPath); err == nil {
		return deb822SourcesPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking %s: %w", deb822SourcesPath, err)
	}

	return "", fmt.Errorf("neither %s nor %s exists", legacyListPath, deb822SourcesPath)
}

// installWithRpm installs the package via dnf or tdnf (repo index refreshed automatically).
func (a *App) installWithRpm(ctx context.Context, pkgMgr string, version string) error {
	return a.retryCommand(ctx, pkgMgr, "install", "-y", "--refresh", "--allowerasing",
		fmt.Sprintf("aks-node-controller-%s", version))
}

// retryCommand runs a command with retries, per-attempt timeout, and backoff.
// Each attempt is capped at commandTimeout to prevent hung package managers from
// blocking provisioning indefinitely (the parent ctx from main.go is context.Background).
func (a *App) retryCommand(ctx context.Context, name string, args ...string) error {
	var lastErr error
	for attempt := 1; attempt <= maxInstallRetries; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, commandTimeout)
		cmd := exec.CommandContext(attemptCtx, name, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		lastErr = a.cmdRun(cmd)
		cancel()
		if lastErr == nil {
			return nil
		}
		slog.Warn("command failed, retrying",
			"command", name, "args", args,
			"attempt", attempt, "maxRetries", maxInstallRetries,
			"error", lastErr)
		if attempt < maxInstallRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryBackoff):
			}
		}
	}
	return fmt.Errorf("command %s failed after %d attempts: %w", name, maxInstallRetries, lastErr)
}

// copyBinaryAlongside atomically copies src to dst (the hotfix path) without touching the
// original VHD-baked binary. It derives permissions from refPath (the VHD binary) so the
// hotfix is executable with the same mode. Writing to a temp file first then renaming ensures
// concurrent readers (e.g., provision-wait) never see a partial binary.
func copyBinaryAlongside(src, dst, refPath string) error {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	info, err := os.Stat(refPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", refPath, err)
	}

	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".aks-node-controller-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(srcData); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Chmod(info.Mode()); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming %s to %s: %w", tmpPath, dst, err)
	}
	slog.Info("installed hotfix binary alongside VHD binary", "src", src, "hotfixPath", dst)
	return nil
}

// shouldUpgradeToHotfix returns true when the current ANC version should be upgraded
// to the hotfix version. This is true only when both versions share the same YYYYMM.DD
// base and the hotfix has a strictly higher PATCH number (patch-only matching).
//
// ANC versions use the format YYYYMM.DD.PATCH which is valid semver (Major.Minor.Patch).
//
// This ensures the hotfix only targets the specific VHD it was built for:
//   - Older VHDs (different base) are skipped — remediated via VHD republish
//   - Newer VHDs (different base) are skipped — fix is already baked in
//   - Same version is skipped — already at hotfix
//   - Unparseable versions (e.g. "dev") return an error — caller should skip
func shouldUpgradeToHotfix(current, hotfix string) (bool, error) {
	cv, err := semver.NewVersion(current)
	if err != nil {
		return false, fmt.Errorf("parsing current version %q: %w", current, err)
	}
	hv, err := semver.NewVersion(hotfix)
	if err != nil {
		return false, fmt.Errorf("parsing hotfix version %q: %w", hotfix, err)
	}
	return cv.Major() == hv.Major() && cv.Minor() == hv.Minor() && hv.Patch() > cv.Patch(), nil
}
