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
	maxInstallRetries        = 5
	retryBackoff             = 3 * time.Second
	commandTimeout           = 60 * time.Second
	defaultAptSourcesDir     = "/etc/apt/sources.list.d"
	// vhdBinaryPath is where packer installs the VHD-baked binary.
	vhdBinaryPath = "/opt/azure/containers/aks-node-controller"
	// hotfixBinaryPath is where the hotfix binary is placed alongside the VHD-baked binary.
	// The wrapper script checks for this path and prefers it over the VHD-baked binary.
	hotfixBinaryPath = "/opt/azure/containers/aks-node-controller-hotfix"
	// pkgBinaryPath is where apt/dnf package installs the binary.
	pkgBinaryPath = "/usr/bin/aks-node-controller"
)

// downloadHotfix installs the requested hotfix and stages it alongside the VHD-baked binary.
// The wrapper script decides which binary to execute after this command returns.
func (a *App) downloadHotfix(ctx context.Context) error {
	hotfixPath := a.hotfixVersionPath
	if hotfixPath == "" {
		hotfixPath = defaultHotfixVersionPath
	}
	cfg, err := readHotfixConfig(hotfixPath)
	if err != nil {
		return fmt.Errorf("read hotfix config from %s: %w", hotfixPath, err)
	}
	hotfixVersion := cfg.resolveVersion(Version)

	if hotfixVersion == "" {
		slog.Info("hotfix config does not request a version for this base, skipping download",
			"path", hotfixPath, "current", Version)
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

// hotfixConfig is the JSON structure of the hotfix configuration file.
// Using JSON allows future extension (e.g., adding checksum, source URL) without format changes.
type hotfixConfig struct {
	// Version is the legacy single-version pointer. It is still honored when Hotfixes
	// is empty, preserving backward compatibility with the original config shape.
	Version string `json:"version,omitempty"`

	// Hotfixes maps an ANC version base ("YYYYMM.DD") to the hotfix version
	// ("YYYYMM.DD.PATCH") to apply to nodes whose baked ANC version shares that base.
	// A single config can thus pin hotfixes for multiple VHD bases at once; a base
	// whose key is absent gets no hotfix (default deny). When non-empty, this map
	// takes precedence over Version.
	Hotfixes map[string]string `json:"hotfixes,omitempty"`
}

// hotfixBaseFromVersion extracts the "YYYYMM.DD" base from an ANC version string of
// the form "YYYYMM.DD.PATCH". It splits on "." rather than parsing semver so the literal
// day segment — including any leading zero such as "01" — is preserved to match map keys
// exactly (semver parsing would drop the leading zero, e.g. "202604.01" -> minor 1).
func hotfixBaseFromVersion(version string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(version), ".", 3)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("version %q is not in YYYYMM.DD.PATCH form", version)
	}
	return parts[0] + "." + parts[1], nil
}

// resolveVersion picks the hotfix version that applies to the given current ANC version.
// When the base->version map is populated it takes precedence: the entry matching the
// current version's "YYYYMM.DD" base is returned, while an absent base (or an unparseable
// current version) yields "" so provisioning proceeds with no hotfix. When the map is
// empty it falls back to the legacy single Version field. The returned version is still
// subject to shouldUpgradeToHotfix's patch-only-strictly-higher gating in the caller.
func (cfg hotfixConfig) resolveVersion(current string) string {
	if len(cfg.Hotfixes) > 0 {
		base, err := hotfixBaseFromVersion(current)
		if err != nil {
			slog.Warn("cannot derive hotfix base from current version, skipping hotfix",
				"current", current, "error", err)
			return ""
		}
		return strings.TrimSpace(cfg.Hotfixes[base])
	}
	return strings.TrimSpace(cfg.Version)
}

// readHotfixConfig reads and parses the JSON hotfix config from the given path.
// Returns a zero-value config if the file doesn't exist or is empty.
func readHotfixConfig(path string) (hotfixConfig, error) {
	var cfg hotfixConfig
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing hotfix config %s: %w", path, err)
	}
	return cfg, nil
}

// readHotfixVersion reads the legacy single-version field from the hotfix config.
// Retained for backward compatibility; map-aware callers should use readHotfixConfig
// together with hotfixConfig.resolveVersion.
func readHotfixVersion(path string) (string, error) {
	cfg, err := readHotfixConfig(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.Version), nil
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
	cv, err := semver.NewVersion(strings.TrimSpace(current))
	if err != nil {
		return false, fmt.Errorf("parsing current version %q: %w", current, err)
	}
	hv, err := semver.NewVersion(strings.TrimSpace(hotfix))
	if err != nil {
		return false, fmt.Errorf("parsing hotfix version %q: %w", hotfix, err)
	}
	return cv.Major() == hv.Major() && cv.Minor() == hv.Minor() && hv.Patch() > cv.Patch(), nil
}
