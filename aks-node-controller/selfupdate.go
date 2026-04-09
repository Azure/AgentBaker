package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	// hotfixVersionPath is written by cloud-init during node bootstrap. The control plane sets the target version in the node's CustomData when a hotfix is active
	hotfixVersionPath = "/etc/aks-node-controller/hotfix-version"
	maxInstallRetries = 5
	retryBackoff      = 3 * time.Second
)

// selfUpdate checks for a hotfix version and installs it from PMC if needed.
// It is called before command dispatch for provision and provision-wait commands.
// On successful install, it re-execs the process with the new binary and never returns.
// On failure, it logs a warning and returns nil so the VHD-baked binary proceeds.
func (a *App) selfUpdate(ctx context.Context) error {
	hotfixVersion, err := readHotfixVersion(hotfixVersionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		slog.Warn("failed to read hotfix version, proceeding with VHD-baked version",
			"path", hotfixVersionPath, "error", err)
		return nil
	}

	if hotfixVersion == "" {
		return nil
	}
	if Version == hotfixVersion {
		slog.Info("ANC already at hotfix version, skipping self-update", "version", Version)
		return nil
	}

	slog.Info("ANC self-update triggered", "current", Version, "target", hotfixVersion)

	installErr := a.installFromPMC(ctx, hotfixVersion)
	if installErr != nil {
		slog.Warn("failed to install hotfix, proceeding with VHD-baked version",
			"target", hotfixVersion, "error", installErr)
		return nil
	}

	return a.reExec()
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
// It reads /etc/os-release to determine whether to use apt-get (Ubuntu) or dnf (AzureLinux/Mariner).
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
			case "azurelinux":
				return "dnf", nil
			default:
				return "", fmt.Errorf("unsupported OS: %s", id)
			}
		}
	}
	return "", fmt.Errorf("ID not found in /etc/os-release")
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
	case "dnf":
		return a.installWithDnf(ctx, version)
	default:
		return fmt.Errorf("unsupported package manager: %s", pkgMgr)
	}
}

// installWithApt refreshes the PMC repo index and installs the package via apt-get.
func (a *App) installWithApt(ctx context.Context, version string) error {
	// Ensure any interrupted dpkg state is reconciled before running apt operations.
	if err := a.retryCommand(ctx, "env", "DEBIAN_FRONTEND=noninteractive",
		"dpkg", "--configure", "-a", "--force-confdef", "--force-confold"); err != nil {
		return fmt.Errorf("dpkg --configure -a failed: %w", err)
	}

	// Refresh only the microsoft-prod repo to minimize time.
	if err := a.retryCommand(ctx, "env", "DEBIAN_FRONTEND=noninteractive",
		"apt-get", "update",
		"-o", "Dpkg::Options::=--force-confold",
		"-o", "Dir::Etc::sourcelist=/etc/apt/sources.list.d/microsoft-prod.list",
		"-o", "Dir::Etc::sourceparts=-"); err != nil {
		return fmt.Errorf("apt-get update failed: %w", err)
	}
	// Install with --allow-downgrades in case the hotfix is older than the VHD-baked version.
	return a.retryCommand(ctx, "env", "DEBIAN_FRONTEND=noninteractive",
		"apt-get", "install", "-y", "--allow-downgrades",
		"-o", "Dpkg::Options::=--force-confold",
		fmt.Sprintf("aks-node-controller=%s*", version))
}

// installWithDnf installs the package via dnf (repo index refreshed automatically).
func (a *App) installWithDnf(ctx context.Context, version string) error {
	return a.retryCommand(ctx, "dnf", "install", "-y", "--allowerasing",
		fmt.Sprintf("aks-node-controller-%s", version))
}

// retryCommand runs a command with retries and backoff.
// This handles transient failures like dpkg lock contention from concurrent cloud-init apt operations.
func (a *App) retryCommand(ctx context.Context, name string, args ...string) error {
	var lastErr error
	for attempt := 1; attempt <= maxInstallRetries; attempt++ {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		lastErr = a.cmdRun(cmd)
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

// reExec replaces the current process with the newly installed binary.
// After package install, the new binary is at /usr/bin/aks-node-controller which
// takes precedence in PATH over the VHD-baked /opt/azure/containers/ location.
func (a *App) reExec() error {
	binary, err := exec.LookPath("aks-node-controller")
	if err != nil {
		return fmt.Errorf("finding aks-node-controller in PATH after install: %w", err)
	}
	slog.Info("re-executing with updated binary", "path", binary, "args", os.Args)
	return syscall.Exec(binary, os.Args, os.Environ())
}
