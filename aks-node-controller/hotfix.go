package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/common"
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

	// HTTP download settings.
	downloadTimeout = 30 * time.Second
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
	return a.downloadBinaryHotfixIfNeeded(ctx, cfg)
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

func (a *App) downloadBinaryHotfixIfNeeded(ctx context.Context, cfg *hotfixConfig) error {
	hotfixVersion := cfg.resolveVersion(Version)

	if hotfixVersion == "" {
		slog.Info("hotfix config does not request a version for this base, skipping download", "current", Version)
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

	// Try direct HTTP download if an artifact descriptor is available for this version + OS/arch.
	if err := a.tryDirectDownload(ctx, cfg, hotfixVersion); err == nil {
		return nil
	} else if isIntegrityError(err) {
		return err
	}

	// Fallback: install via package manager (apt-get or dnf/tdnf).
	if err := a.installFromPMC(ctx, hotfixVersion); err != nil {
		return fmt.Errorf("install hotfix version %s: %w", hotfixVersion, err)
	}

	if err := copyBinaryAlongside(pkgBinaryPath, hotfixBinaryPath, vhdBinaryPath); err != nil {
		return fmt.Errorf("stage hotfix binary: %w", err)
	}

	slog.Info("downloaded ANC hotfix", "target", hotfixVersion, "path", hotfixBinaryPath)
	return nil
}

// artifactInfo describes a directly-downloadable package artifact with its integrity digest.
type artifactInfo struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
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

	// Artifacts maps a hotfix version to per-OS/arch artifact descriptors for direct
	// HTTP download. When present and matching, the download path bypasses the package
	// manager entirely. The outer key is the hotfix version (e.g. "202607.02.2"), the
	// inner key is "ID-VERSION_ID-GOARCH" (e.g. "ubuntu-22.04-amd64").
	Artifacts map[string]map[string]artifactInfo `json:"artifacts,omitempty"`
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

// platformInfo holds the parsed OS identity and architecture for the current host.
type platformInfo struct {
	ID        string // e.g. "ubuntu", "azurelinux", "mariner"
	VersionID string // e.g. "22.04", "3.0"
	Arch      string // e.g. "amd64", "arm64"
}

// parsePlatformInfo reads /etc/os-release (or a.osReleasePath override for testing)
// and combines it with the build architecture to describe the current platform.
func (a *App) parsePlatformInfo() (platformInfo, error) {
	osReleasePath := a.osReleasePath
	if osReleasePath == "" {
		osReleasePath = "/etc/os-release"
	}
	data, err := os.ReadFile(osReleasePath)
	if err != nil {
		return platformInfo{}, fmt.Errorf("reading %s: %w", osReleasePath, err)
	}
	info := platformInfo{Arch: runtime.GOARCH}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			info.ID = strings.ToLower(strings.Trim(strings.TrimPrefix(line, "ID="), `"`))
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

// detectPackageManager returns the package manager for the current OS.
func (a *App) detectPackageManager() (packageManager, error) {
	info, err := a.parsePlatformInfo()
	if err != nil {
		return "", err
	}
	switch info.ID {
	case "ubuntu":
		return pkgMgrApt, nil
	case "azurelinux", "mariner":
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

// tryDirectDownload attempts to download the hotfix binary directly via HTTP using the
// artifact descriptor. Returns nil on success, an integrityError on validation failure
// (caller must NOT fallback), or a regular error on network/transient failure (caller may fallback).
// Returns a non-nil non-integrity error when no artifact is available (signals fallback).
func (a *App) tryDirectDownload(ctx context.Context, cfg *hotfixConfig, hotfixVersion string) error {
	artifact, artifactKey := a.resolveArtifact(cfg, hotfixVersion)
	if artifact == nil {
		return fmt.Errorf("no artifact descriptor available")
	}

	slog.Info("artifact descriptor found, attempting direct HTTP download",
		"version", hotfixVersion, "key", artifactKey, "url", artifact.URL)

	tmpPath, err := a.downloadAndVerify(ctx, artifact.URL, artifact.SHA256)
	if err != nil {
		if isIntegrityError(err) {
			// Remove any previously staged hotfix binary so the wrapper falls back to the
			// VHD-baked ANC — a stale hotfix binary must not run after an integrity failure.
			if removeErr := os.Remove(hotfixBinaryPath); removeErr != nil && !os.IsNotExist(removeErr) {
				slog.Warn("failed to remove stale hotfix binary on integrity error",
					"path", hotfixBinaryPath, "error", removeErr)
			}
			return fmt.Errorf("artifact integrity check failed for %s: %w", hotfixVersion, err)
		}
		slog.Warn("direct HTTP download failed, falling back to package manager",
			"version", hotfixVersion, "error", err)
		return err
	}

	if err := copyBinaryAlongside(tmpPath, hotfixBinaryPath, vhdBinaryPath); err != nil {
		os.Remove(tmpPath)
		// Staging failure after successful download+verify is a hard error — do not fallback
		// to package manager since we already verified the binary integrity.
		return newIntegrityError("stage hotfix binary from artifact: %v", err)
	}
	os.Remove(tmpPath)
	slog.Info("downloaded ANC hotfix via direct HTTP", "target", hotfixVersion, "path", hotfixBinaryPath)
	return nil
}

// integrityError marks errors where the downloaded content failed validation.
// These must NOT fallback to the package manager — the node should keep its VHD-baked ANC.
type integrityError struct {
	msg string
}

func (e *integrityError) Error() string { return e.msg }

func newIntegrityError(format string, args ...any) error {
	return &integrityError{msg: fmt.Sprintf(format, args...)}
}

func isIntegrityError(err error) bool {
	var ie *integrityError
	return errors.As(err, &ie)
}

// resolveArtifact looks up the artifact descriptor for the given hotfix version and current
// OS/architecture. Returns nil if no artifact is available (caller should fallback to pkg mgr).
func (a *App) resolveArtifact(cfg *hotfixConfig, hotfixVersion string) (*artifactInfo, string) {
	if len(cfg.Artifacts) == 0 {
		return nil, ""
	}
	perArch, ok := cfg.Artifacts[hotfixVersion]
	if !ok || len(perArch) == 0 {
		return nil, ""
	}
	key, err := a.buildArtifactKey()
	if err != nil {
		slog.Warn("cannot build artifact key, skipping direct download", "error", err)
		return nil, ""
	}
	ai, ok := perArch[key]
	if !ok {
		return nil, key
	}
	return &ai, key
}

// buildArtifactKey constructs the OS/arch lookup key for the artifacts map.
// Format: "ID-VERSION_ID-GOARCH" (e.g. "ubuntu-22.04-amd64").
func (a *App) buildArtifactKey() (string, error) {
	info, err := a.parsePlatformInfo()
	if err != nil {
		return "", err
	}
	if info.VersionID == "" {
		return "", fmt.Errorf("VERSION_ID not found in os-release")
	}
	return fmt.Sprintf("%s-%s-%s", info.ID, info.VersionID, info.Arch), nil
}

// validateArtifactURL ensures the URL is HTTPS and the host is in the PMC allowlist.
func validateArtifactURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return newIntegrityError("invalid artifact URL %q: %v", rawURL, err)
	}
	if u.Scheme != "https" {
		return newIntegrityError("artifact URL must be HTTPS, got %q", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "packages.microsoft.com":
		return nil
	default:
		return newIntegrityError("artifact URL host %q not in allowlist", host)
	}
}

// downloadAndVerify downloads the artifact from the given URL, streams it to a temp file
// while computing its SHA-256 digest, and compares to the expected value. Returns the path
// to a temp file containing the verified binary. The caller is responsible for removing or
// renaming the temp file after staging.
func (a *App) downloadAndVerify(ctx context.Context, artifactURL, expectedSHA256 string) (string, error) {
	if err := validateArtifactURL(artifactURL); err != nil {
		return "", err
	}
	expectedSHA256 = strings.TrimSpace(strings.ToLower(expectedSHA256))
	if expectedSHA256 == "" {
		return "", newIntegrityError("artifact SHA-256 is empty")
	}

	// Create temp file for streaming.
	dir := a.downloadDir
	if dir == "" {
		dir = filepath.Dir(hotfixBinaryPath)
	}
	tmp, err := os.CreateTemp(dir, ".aks-node-controller-download-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()

	// Cleanup on any failure path.
	success := false
	defer func() {
		if !success {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	// Get a reader for the artifact content.
	reader, err := a.getArtifactReader(ctx, artifactURL)
	if err != nil {
		return "", fmt.Errorf("HTTP download %s: %w", artifactURL, err)
	}
	defer reader.Close()

	// Stream through SHA-256 hasher into temp file in a single pass — no full-body buffer.
	hasher := sha256.New()
	if _, err := io.Copy(tmp, io.TeeReader(reader, hasher)); err != nil {
		return "", fmt.Errorf("writing temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("closing temp file %s: %w", tmpPath, err)
	}

	// Verify SHA-256.
	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))
	if actualSHA256 != expectedSHA256 {
		os.Remove(tmpPath)
		return "", newIntegrityError("SHA-256 mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
	}

	success = true
	return tmpPath, nil
}

// getArtifactReader returns a ReadCloser for the artifact content. It uses the injectable
// httpDownload hook for testing (wrapping []byte in a reader), or performs a real streaming
// HTTP GET.
func (a *App) getArtifactReader(ctx context.Context, artifactURL string) (io.ReadCloser, error) {
	if a.httpDownload != nil {
		data, err := a.httpDownload(ctx, artifactURL)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return a.doHTTPStream(ctx, artifactURL)
}

// doHTTPStream performs a real streaming HTTP GET and returns the response body.
// The caller must close the returned ReadCloser.
func (a *App) doHTTPStream(ctx context.Context, artifactURL string) (io.ReadCloser, error) {
	dlCtx, cancel := context.WithTimeout(ctx, downloadTimeout)

	transport := common.NewBaseTransport(common.HTTPTransportOptions{
		DialTimeout:           10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	})
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if err := validateArtifactURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect to disallowed host: %w", err)
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, artifactURL, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, artifactURL)
	}
	// Wrap body to cancel context on close.
	return &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}, nil
}

// cancelOnClose wraps an io.ReadCloser to call a cancel func on Close.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
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
