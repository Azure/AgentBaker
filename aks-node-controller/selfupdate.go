package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultHotfixVersionPath = "/etc/aks-node-controller/hotfix-version"
	maxInstallRetries        = 5
	retryBackoff             = 3 * time.Second
	commandTimeout           = 60 * time.Second
	defaultAptSourcesDir     = "/etc/apt/sources.list.d"
	// vhdBinaryPath is where packer installs the VHD-baked binary and where
	// the wrapper script (aks-node-controller-wrapper.sh) expects to find it.
	vhdBinaryPath = "/opt/azure/containers/aks-node-controller"
	// pkgBinaryPath is where apt/dnf package installs the binary.
	pkgBinaryPath = "/usr/bin/aks-node-controller"
)

// selfUpdate checks for a hotfix version and installs it from PMC if needed.
// It is called before command dispatch for provision and provision-wait commands.
// On successful install, it re-execs the process with the new binary and never returns.
// On any failure, it logs a warning so the VHD-baked binary proceeds.
func (a *App) selfUpdate(ctx context.Context) {
	hotfixPath := a.hotfixVersionPath
	if hotfixPath == "" {
		hotfixPath = defaultHotfixVersionPath
	}
	hotfixVersion, err := readHotfixVersion(hotfixPath)
	if err != nil {
		slog.Warn("failed to read hotfix version, proceeding with VHD-baked version",
			"path", hotfixPath, "error", err)
		return
	}

	if hotfixVersion == "" {
		return
	}
	if Version == hotfixVersion {
		slog.Info("ANC already at hotfix version, skipping self-update", "version", Version)
		return
	}

	slog.Info("ANC self-update triggered", "current", Version, "target", hotfixVersion)

	installErr := a.installFromPMC(ctx, hotfixVersion)
	if installErr != nil {
		slog.Warn("failed to install hotfix, proceeding with VHD-baked version",
			"target", hotfixVersion, "error", installErr)
		return
	}

	// Overwrite the VHD-baked binary so the hotfix persists across service restarts and reboots.
	if err := replaceBinary(pkgBinaryPath, vhdBinaryPath); err != nil {
		slog.Warn("failed to replace VHD binary with hotfix, proceeding with current binary",
			"error", err)
		return
	}

	if err := a.reExec(); err != nil {
		slog.Warn("failed to re-exec after hotfix install, proceeding with current binary",
			"error", err)
	}
}

// readHotfixVersion reads and trims the hotfix version from the given path.
// Returns empty string if the file doesn't exist or is empty.
func readHotfixVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// detectPackageManager returns the package manager command for the current OS.
// It reads /etc/os-release to determine whether to use apt-get (Ubuntu) or tdnf/dnf (AzureLinux/Mariner).
func detectPackageManager() (string, error) {
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
				return "apt-get", nil
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
func preferredRpmManager() string {
	if _, err := exec.LookPath("dnf"); err == nil {
		return "dnf"
	}
	return "tdnf"
}

// installFromPMC installs the hotfix package from PMC using the system package manager.
func (a *App) installFromPMC(ctx context.Context, version string) error {
	pkgMgr, err := detectPackageManager()
	if err != nil {
		return err
	}

	switch pkgMgr {
	case "apt-get":
		return a.installWithApt(ctx, version)
	case "dnf", "tdnf":
		return a.installWithRpm(ctx, pkgMgr, version)
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
	return a.retryCommand(ctx, pkgMgr, "install", "-y", "--allowerasing",
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

// replaceBinary atomically replaces dst with the contents of src, preserving dst's permissions.
// It writes to a temp file in the same directory, then renames — ensuring readers never see a
// truncated or partially-written binary even if another process is running concurrently.
func replaceBinary(src, dst string) error {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("stat %s: %w", dst, err)
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
	slog.Info("replaced VHD binary with hotfix", "src", src, "dst", dst)
	return nil
}

// reExec replaces the current process with the updated binary at the VHD path.
func (a *App) reExec() error {
	args := append([]string{vhdBinaryPath}, os.Args[1:]...)
	slog.Info("re-executing with updated binary", "path", vhdBinaryPath, "args", args)
	return syscall.Exec(vhdBinaryPath, args, os.Environ())
}
