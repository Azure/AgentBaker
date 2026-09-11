package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		// Fail-open: an unreadable or malformed hotfix config must never block
		// provisioning. Log and skip so the node boots on its VHD-baked binary.
		slog.Warn("failed to read hotfix config, skipping hotfix download",
			"path", hotfixPath, "error", err)
		return nil
	}
	// Applying node custom data is best-effort/fail-open: it must never block the
	// binary hotfix download below, or provisioning as a whole.
	if err := a.applyNodeCustomDataIfNeeded(cfg); err != nil {
		slog.Warn("failed to apply node custom data", "path", hotfixPath, "error", err)
	}
	return a.eventLogger.RunTimedOperation("Hotfix.BinaryOperation", func() (string, error) {
		return a.downloadBinaryHotfixIfNeeded(ctx, cfg)
	})
}

func (a *App) applyNodeCustomDataIfNeeded(cfg *hotfixConfig) error {
	hotfixVersion := strings.TrimSpace(cfg.ScriptsVersion)
	if hotfixVersion == "" {
		slog.Info("hotfix config does not request a scripts version for this base, skipping nodecustomdata apply", "current", Version)
		return nil
	}

	// Patch-only matching: only upgrade if same YYYYMM.DD base and hotfix has
	// a strictly higher PATCH. Parse errors (e.g. "dev" builds) result in skip.
	shouldUpgrade, err := shouldUpgradeToHotfix(Version, hotfixVersion)
	if err != nil {
		slog.Warn("failed to compare versions, skipping nodecustomdata apply",
			"current", Version, "hotfix", hotfixVersion, "error", err)
		return nil
	}
	if !shouldUpgrade {
		slog.Info("CSE scripts version not targeted by hotfix, skipping nodecustomdata apply",
			"current", Version, "hotfix", hotfixVersion)
		return nil
	}

	return applyNodeCustomData(a.getNodeCustomDataPath())
}

// hotfixProgress tracks which install route was taken and how the attempt ended, so the
// caller can emit a single summary event no matter which branch returns.
type hotfixProgress struct {
	route   string
	outcome string
}

func (a *App) downloadBinaryHotfixIfNeeded(ctx context.Context, cfg *hotfixConfig) (string, error) {
	hotfixVersion, resolveErr := cfg.resolveVersion(Version)
	progress := &hotfixProgress{route: hotfixRouteNone, outcome: hotfixOutcomeStarted}
	slog.Info("ANC hotfix binary operation started", "current", Version, "target", hotfixVersion)

	err := a.runBinaryHotfix(ctx, cfg, hotfixVersion, resolveErr, progress)
	logHotfixBinaryOperationFinished(Version, hotfixVersion, progress.route, progress.outcome, err)
	return hotfixOperationMessage(Version, hotfixVersion, progress.route, progress.outcome), err
}

func (a *App) runBinaryHotfix(ctx context.Context, cfg *hotfixConfig, hotfixVersion string, resolveErr error, progress *hotfixProgress) error {
	// An unparseable running version is a fail-open skip: never block provisioning on a
	// version we cannot interpret.
	if resolveErr != nil {
		progress.outcome = hotfixOutcomeSkippedVersionCompareError
		slog.Warn("cannot resolve hotfix version for current build, skipping download",
			"current", Version, "error", resolveErr)
		return nil
	}
	if hotfixVersion == "" {
		progress.outcome = hotfixOutcomeSkippedNoVersion
		slog.Info("hotfix config does not request a version for this base, skipping download", "current", Version)
		return nil
	}

	// Patch-only matching: only upgrade if same YYYYMM.DD base and hotfix has
	// a strictly higher PATCH. Parse errors (e.g. "dev" builds) result in skip.
	shouldUpgrade, err := shouldUpgradeToHotfix(Version, hotfixVersion)
	if err != nil {
		progress.outcome = hotfixOutcomeSkippedVersionCompareError
		slog.Warn("failed to compare versions, skipping hotfix download",
			"current", Version, "hotfix", hotfixVersion, "error", err)
		return nil
	}
	if !shouldUpgrade {
		progress.outcome = hotfixOutcomeSkippedNotTargeted
		slog.Info("ANC version not targeted by hotfix, skipping download",
			"current", Version, "hotfix", hotfixVersion)
		return nil
	}

	slog.Info("downloading ANC hotfix", "current", Version, "target", hotfixVersion)

	// Install via package manager (apt-get or dnf/tdnf). A future direct-download path must
	// resolve the package from the node's configured repository and extract the ANC binary;
	// package artifacts cannot be staged directly as executables.
	progress.route = hotfixRoutePackageManager
	slog.Info("ANC hotfix package-manager install started", "target", hotfixVersion)
	if err := a.eventLogger.RunTimedOperation("Hotfix.PackageManagerInstall", func() (string, error) {
		installErr := a.installFromPMC(ctx, hotfixVersion)
		return fmt.Sprintf("target=%s route=%s outcome=%s", hotfixVersion, progress.route, hotfixOutcome(installErr)), installErr
	}); err != nil {
		progress.outcome = string(outcomeFailed)
		slog.Warn("ANC hotfix package-manager install failed", "target", hotfixVersion, "error", err)
		return fmt.Errorf("install hotfix version %s: %w", hotfixVersion, err)
	}
	slog.Info("ANC hotfix package-manager install finished", "target", hotfixVersion)

	slog.Info("ANC hotfix binary staging started", "target", hotfixVersion, "src", a.pkgPath(), "dst", a.hotfixPath())
	if err := a.eventLogger.RunTimedOperation("Hotfix.BinaryStaging", func() (string, error) {
		stageErr := copyBinaryAlongside(a.pkgPath(), a.hotfixPath(), a.vhdPath())
		return fmt.Sprintf("target=%s source=%s destination=%s outcome=%s",
			hotfixVersion, a.pkgPath(), a.hotfixPath(), hotfixOutcome(stageErr)), stageErr
	}); err != nil {
		progress.outcome = string(outcomeFailed)
		slog.Warn("ANC hotfix binary staging failed", "target", hotfixVersion, "error", err)
		return fmt.Errorf("stage hotfix binary: %w", err)
	}
	slog.Info("ANC hotfix binary staging finished", "target", hotfixVersion)

	slog.Info("downloaded ANC hotfix", "target", hotfixVersion, "path", a.hotfixPath())
	progress.outcome = hotfixOutcomeSuccess
	return nil
}

func hotfixOperationMessage(current, target, route, outcome string) string {
	return fmt.Sprintf("current=%s target=%s route=%s outcome=%s", current, target, route, outcome)
}

func hotfixOutcome(err error) string {
	if err != nil {
		return string(outcomeFailed)
	}
	return hotfixOutcomeSuccess
}

func logHotfixBinaryOperationFinished(current, target, route, outcome string, err error) {
	attrs := []any{
		"current", current,
		"target", target,
		"route", route,
		"outcome", outcome,
	}
	if err != nil {
		attrs = append(attrs, "error", err)
		slog.Warn("ANC hotfix binary operation finished", attrs...)
		return
	}
	slog.Info("ANC hotfix binary operation finished", attrs...)
}

func (a *App) vhdPath() string {
	if a.vhdBinaryPath != "" {
		return a.vhdBinaryPath
	}
	return vhdBinaryPath
}

func (a *App) hotfixPath() string {
	if a.hotfixBinaryPath != "" {
		return a.hotfixBinaryPath
	}
	return hotfixBinaryPath
}

func (a *App) pkgPath() string {
	if a.pkgBinaryPath != "" {
		return a.pkgBinaryPath
	}
	return pkgBinaryPath
}

// hotfixConfig is the JSON structure of the hotfix configuration file.
// Using JSON allows future extension (e.g., adding checksum, source URL) without format changes.
type hotfixConfig struct {
	// Version is the legacy single-version pointer. It is still honored when Hotfixes
	// is empty, preserving backward compatibility with the original config shape.
	Version string `json:"version,omitempty"`

	// ScriptsVersion is override version for cse scripts
	ScriptsVersion string `json:"scripts_version,omitempty"`

	// Hotfixes maps an ANC version base ("YYYYMM.DD") to the hotfix version
	// ("YYYYMM.DD.PATCH") to apply to nodes whose baked ANC version shares that base.
	// A single config can thus pin hotfixes for multiple VHD bases at once; a base
	// whose key is absent gets no hotfix (default deny). When non-empty, this map
	// takes precedence over Version.
	Hotfixes map[string]string `json:"hotfixes,omitempty"`
}

// hotfixBaseFromVersion extracts the "YYYYMM.DD" base from an ANC version string of
// the form "YYYYMM.DD.PATCH". It splits on "." rather than parsing semver so the literal
// day segment - including any leading zero such as "01" - is preserved to match map keys
// exactly (semver parsing would drop the leading zero, e.g. "202604.01" -> minor 1).
// All three segments must be non-empty; a present-but-empty patch (e.g. "202604.01.")
// is rejected so an obviously malformed current version never selects a map entry.
func hotfixBaseFromVersion(version string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(version), ".", 3)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("version %q is not in YYYYMM.DD.PATCH form", version)
	}
	return parts[0] + "." + parts[1], nil
}

// resolveVersion picks the hotfix ANC version that applies to the given current ANC version.
// When the base->version map is populated it takes precedence: the entry matching the
// current version's "YYYYMM.DD" base is returned, while an absent base yields "" so
// provisioning proceeds with no hotfix. When the map is empty it falls back to the legacy
// single Version field. A non-nil error means current could not be parsed, which callers
// treat as a fail-open skip distinct from "no hotfix configured". The returned version is
// still subject to shouldUpgradeToHotfix's patch-only-strictly-higher gating in the caller.
func (cfg hotfixConfig) resolveVersion(current string) (string, error) {
	if len(cfg.Hotfixes) > 0 {
		base, err := hotfixBaseFromVersion(current)
		if err != nil {
			return "", fmt.Errorf("deriving hotfix base from %q: %w", current, err)
		}
		return strings.TrimSpace(cfg.Hotfixes[base]), nil
	}
	return strings.TrimSpace(cfg.Version), nil
}

// readHotfixConfig reads and parses the JSON hotfix config from the given path.
// Returns a zero-value config if the file doesn't exist or is empty.
func readHotfixConfig(path string) (*hotfixConfig, error) {
	var cfg hotfixConfig
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return &cfg, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return &cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &cfg, fmt.Errorf("parsing hotfix config %s: %w", path, err)
	}
	return &cfg, nil
}

// platformInfo holds the OS family, platform identity, and architecture for the current host.
type platformInfo struct {
	OS        string // e.g. "linux", "windows"
	ID        string // e.g. "ubuntu", "azurelinux"
	VariantID string // e.g. "azurecontainerlinux", "osguard"
	VersionID string // e.g. "22.04", "3.0"
	Arch      string // e.g. "amd64", "arm64"
}

// parseLinuxPlatformInfo currently reads Linux /etc/os-release (or a.osReleasePath override
// for testing) and combines it with the build architecture to describe the platform.
func (a *App) parseLinuxPlatformInfo() (platformInfo, error) {
	osReleasePath := a.osReleasePath
	if osReleasePath == "" {
		osReleasePath = "/etc/os-release"
	}
	data, err := os.ReadFile(osReleasePath)
	if err != nil {
		return platformInfo{}, fmt.Errorf("reading %s: %w", osReleasePath, err)
	}
	info := platformInfo{OS: "linux", Arch: runtime.GOARCH}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			info.ID = strings.ToLower(strings.Trim(strings.TrimPrefix(line, "ID="), `"`))
		}
		if strings.HasPrefix(line, "VARIANT_ID=") {
			info.VariantID = strings.ToLower(strings.Trim(strings.TrimPrefix(line, "VARIANT_ID="), `"`))
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			info.VersionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
		}
	}
	if info.ID == "" {
		return platformInfo{}, fmt.Errorf("ID not found in %s", osReleasePath)
	}
	return info, nil
}

// packageManager represents a supported system package manager.
type packageManager string

const (
	pkgMgrApt  packageManager = "apt-get"
	pkgMgrDnf  packageManager = "dnf"
	pkgMgrTdnf packageManager = "tdnf"
)

const (
	hotfixRouteNone           = "none"
	hotfixRoutePackageManager = "package-manager"

	hotfixOutcomeSuccess                    = "success"
	hotfixOutcomeStarted                    = "started"
	hotfixOutcomeSkippedNoVersion           = "skipped-no-version"
	hotfixOutcomeSkippedVersionCompareError = "skipped-version-compare-error"
	hotfixOutcomeSkippedNotTargeted         = "skipped-not-targeted"
)

// detectPackageManager returns the package manager for the current OS.
func (a *App) detectPackageManager() (packageManager, error) {
	info, err := a.parseLinuxPlatformInfo()
	if err != nil {
		return "", err
	}
	if info.ID == osReleaseIDAzureLinux &&
		(info.VariantID == osReleaseIDAzureContainerLinux || info.VariantID == "osguard") {
		return "", fmt.Errorf(
			"PMC package-based ANC self-update is not supported on image-based OS %q variant %q",
			info.ID,
			info.VariantID,
		)
	}
	switch info.ID {
	case "ubuntu":
		return pkgMgrApt, nil
	case osReleaseIDAzureLinux:
		return preferredRpmManager(), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", info.ID)
	}
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
	pkgMgr, err := a.detectPackageManager()
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
	slog.Info("ANC hotfix apt dpkg configure started", "version", version)
	if err := a.eventLogger.RunTimedOperation("Hotfix.AptDpkgConfigure", func() (string, error) {
		err := a.retryCommand(ctx, "env", "DEBIAN_FRONTEND=noninteractive",
			"dpkg", "--configure", "-a", "--force-confdef", "--force-confold")
		return fmt.Sprintf("version=%s outcome=%s", version, hotfixOutcome(err)), err
	}); err != nil {
		slog.Warn("ANC hotfix apt dpkg configure failed", "version", version, "error", err)
		return fmt.Errorf("dpkg --configure -a failed: %w", err)
	}
	slog.Info("ANC hotfix apt dpkg configure finished", "version", version)

	// Refresh only the microsoft-prod repo to minimize time.
	slog.Info("ANC hotfix apt update started", "version", version, "sourceList", microsoftProdSourceListPath)
	if err := a.eventLogger.RunTimedOperation("Hotfix.AptUpdate", func() (string, error) {
		err := a.retryCommand(ctx, "env", "DEBIAN_FRONTEND=noninteractive",
			"apt-get", "update",
			"-o", "Dpkg::Options::=--force-confold",
			"-o", fmt.Sprintf("Dir::Etc::sourcelist=%s", microsoftProdSourceListPath),
			"-o", "Dir::Etc::sourceparts=-")
		return fmt.Sprintf("version=%s sourceList=%s outcome=%s",
			version, microsoftProdSourceListPath, hotfixOutcome(err)), err
	}); err != nil {
		slog.Warn("ANC hotfix apt update failed", "version", version, "sourceList", microsoftProdSourceListPath, "error", err)
		return fmt.Errorf("apt-get update failed: %w", err)
	}
	slog.Info("ANC hotfix apt update finished", "version", version, "sourceList", microsoftProdSourceListPath)

	// Install with --allow-downgrades in case the hotfix is older than the VHD-baked version.
	slog.Info("ANC hotfix apt install started", "version", version)
	if err := a.eventLogger.RunTimedOperation("Hotfix.AptInstall", func() (string, error) {
		err := a.retryCommand(ctx, "env", "DEBIAN_FRONTEND=noninteractive",
			"apt-get", "install", "-y", "--allow-downgrades",
			"-o", "Dpkg::Options::=--force-confold",
			fmt.Sprintf("aks-node-controller=%s*", version))
		return fmt.Sprintf("version=%s outcome=%s", version, hotfixOutcome(err)), err
	}); err != nil {
		slog.Warn("ANC hotfix apt install failed", "version", version, "error", err)
		return err
	}
	slog.Info("ANC hotfix apt install finished", "version", version)
	return nil
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
	slog.Info("ANC hotfix rpm install started", "packageManager", pkgMgr, "version", version)
	if err := a.eventLogger.RunTimedOperation("Hotfix.RpmInstall", func() (string, error) {
		err := a.retryCommand(ctx, pkgMgr, "install", "-y", "--refresh", "--allowerasing",
			fmt.Sprintf("aks-node-controller-%s", version))
		return fmt.Sprintf("packageManager=%s version=%s outcome=%s",
			pkgMgr, version, hotfixOutcome(err)), err
	}); err != nil {
		slog.Warn("ANC hotfix rpm install failed", "packageManager", pkgMgr, "version", version, "error", err)
		return err
	}
	slog.Info("ANC hotfix rpm install finished", "packageManager", pkgMgr, "version", version)
	return nil
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
//   - Older VHDs (different base) are skipped - remediated via VHD republish
//   - Newer VHDs (different base) are skipped - fix is already baked in
//   - Same version is skipped - already at hotfix
//   - Unparseable versions (e.g. "dev") return an error - caller should skip
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
