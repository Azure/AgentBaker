// Package generalize prepares a Linux VM for image capture. Packer and Azure Image Builder use image
// mode. VHD caching (PIS) uses keep-agent mode.
package generalize

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/Azure/agentbaker/aks-node-controller/config"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
)

const aclLegacyUKIName = "acl.efi"

// nodeStatePaths are per-node provisioning outputs that must not reach a captured image.
// basePrep outputs (for example base_prep.complete) are intentionally absent so clones skip basePrep.
//
//nolint:gochecknoglobals // read-only list
var nodeStatePaths = []string{
	config.Linux().ProvisionCompleteFilePath,
	nodeconfigutils.AKSNodeConfigFilePath,
	config.NBCCmdPath,
	"/opt/azure/containers/boothook.sh",
	"/opt/bin/boothook.sh",
	config.Linux().ProvisionJSONFilePath,
}

// buildArtifactPaths are VHD build leftovers. A provisioned node does not create them again,
// so only image mode removes them.
//
//nolint:gochecknoglobals // read-only list
var buildArtifactPaths = []string{
	// Packer SSH keys.
	"/root/.ssh/authorized_keys",
	"/home/packer/.ssh/authorized_keys",
	"/var/log/cloud-init.log",
	"/var/log/cloud-init-output.log",
	// aznfs pulls in stunnel4, which writes to this log dir; aznfs configures stunnel to log elsewhere.
	"/var/log/stunnel4",
	"/etc/logrotate.d/stunnel4",
	// Written by generate-disk-usage.sh.
	"/opt/azure/disk-usage.txt",
	// Used only to pull images during the build.
	"/opt/azure/containers/image-fetcher",
	"/opt/azure/containers/imds_instance_metadata_cache.json",
	// Prevents build-time extension logs and telemetry from shipping on the OS disk.
	"/var/log/azure/Microsoft.Azure.Extensions.CustomScript",
}

// nvpcrAnchorPaths and nvpcrCredentialDirs hold the systemd NvPCR anchor. The anchor is bound to
// the TPM of the build VM, so a clone must create its own.
//
//nolint:gochecknoglobals // read-only list
var (
	nvpcrAnchorPaths = []string{
		"/var/lib/systemd/nvpcr/nvpcr-anchor.cred",
		"/run/systemd/nvpcr/nvpcr-anchor.cred",
	}
	nvpcrCredentialDirs = []string{
		"/boot/loader/credentials",
		"/boot/efi/loader/credentials",
		"/efi/loader/credentials",
	}
)

type generalizeStep struct {
	name string
	// imageOnly steps run only without KeepAgent: they stop or replace the guest agent, or they
	// remove build leftovers that a provisioned node does not have.
	imageOnly bool
	run       func(ctx context.Context, osRelease map[string]string) error
}

func (g *Generalizer) steps() []generalizeStep {
	return []generalizeStep{
		{name: "DownloadPinnedAgent", imageOnly: true, run: g.downloadPinnedWALinuxAgent},
		{name: "DisableContainerd", imageOnly: true, run: g.disableContainerd},
		{name: "RemoveBuildArtifacts", imageOnly: true, run: g.removeBuildArtifacts},
		{name: "ResetMachineID", run: g.resetMachineID},
		{name: "RemoveNodeState", run: g.removeNodeState},
		{name: "ResetACLFirstBoot", run: g.resetACLFirstBoot},
		{name: "DeprovisionAgent", imageOnly: true, run: g.deprovisionAgent},
		{name: "InstallPinnedAgent", imageOnly: true, run: g.installPinnedWALinuxAgent},
		{name: "RemoveNvPCRAnchors", imageOnly: true, run: g.removeNvPCRAnchors},
		{name: "Sync", run: func(context.Context, map[string]string) error { syncFilesystems(); return nil }},
	}
}

// Generalizer prepares a Linux VM for image capture.
type Generalizer struct {
	// KeepAgent removes only per-node state. It is for captures driven through the running guest agent
	// (VHD caching/PIS RunCommand): it does not stop or deprovision WALinuxAgent, because RunCommand
	// reports its status through it. /var/lib/waagent/provisioned is kept on purpose: on a clone, the
	// agent sees a changed VM unique ID and cleans its own goal-state, extension, and DHCP state before
	// it handles the clone's goal state.
	KeepAgent bool
	// RunCmd runs external commands. If nil, Run uses (*exec.Cmd).Run.
	RunCmd func(*exec.Cmd) error

	// rootDir prefixes all paths for testing.
	rootDir string
	// installer overrides the wireserver client for testing.
	installer *walinuxagentInstaller
}

// Run runs the generalize steps in order and stops at the first failure.
func (g *Generalizer) Run(ctx context.Context) error {
	if runtime.GOOS == "windows" {
		return errors.New("generalize is not supported on Windows")
	}
	slog.Info("generalize started", "keepAgent", g.KeepAgent)

	osReleasePath := g.path(config.OSReleasePath)
	data, err := os.ReadFile(osReleasePath)
	if err != nil {
		return fmt.Errorf("read OS release %s: %w", osReleasePath, err)
	}
	osRelease := config.ParseOSRelease(data)

	for _, step := range g.steps() {
		if g.KeepAgent && step.imageOnly {
			continue
		}
		slog.Info("generalize step started", "step", step.name)
		if err := step.run(ctx, osRelease); err != nil {
			return fmt.Errorf("generalize step %s: %w", step.name, err)
		}
	}

	slog.Info("generalize finished")
	return nil
}

// runCmd runs a command, copies its output to stderr, and returns the output.
func (g *Generalizer) runCmd(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var output bytes.Buffer
	w := io.MultiWriter(os.Stderr, &output)
	cmd.Stdout = w
	cmd.Stderr = w
	run := g.RunCmd
	if run == nil {
		run = (*exec.Cmd).Run
	}
	if err := run(cmd); err != nil {
		return output.Bytes(), fmt.Errorf("run %s: %w", cmd.String(), err)
	}
	return output.Bytes(), nil
}

// disableContainerd makes clones start containerd only after CSE configures it.
func (g *Generalizer) disableContainerd(ctx context.Context, _ map[string]string) error {
	if _, err := g.runCmd(ctx, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	_, err := g.runCmd(ctx, "systemctl", "disable", "--now", "containerd")
	return err
}

func (g *Generalizer) removeBuildArtifacts(context.Context, map[string]string) error {
	for _, p := range buildArtifactPaths {
		path := g.path(p)
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

// resetMachineID truncates instead of removing: an empty machine-id is not treated as first boot by
// systemd, and the running VM keeps a valid file. The clone gets a new ID on boot.
func (g *Generalizer) resetMachineID(context.Context, map[string]string) error {
	machineID := g.path("/etc/machine-id")
	f, err := os.OpenFile(machineID, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("reset %s: %w", machineID, err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("reset %s: %w", machineID, err)
	}
	if err = os.Chmod(machineID, 0o644); err != nil {
		return fmt.Errorf("chmod %s: %w", machineID, err)
	}
	slog.Info("reset machine-id", "path", machineID)
	return nil
}

func (g *Generalizer) removeNodeState(context.Context, map[string]string) error {
	for _, p := range nodeStatePaths {
		path := g.path(p)
		if err := os.Remove(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("remove %s: %w", path, err)
		}
		slog.Info("removed", "path", path)
	}
	return nil
}

// deprovisionAgent runs the standard Azure Linux generalization. It also removes the last provisioned
// user (the Packer build user).
func (g *Generalizer) deprovisionAgent(ctx context.Context, _ map[string]string) error {
	// Azure Policy installs the Guest Configuration extension on build VMs. Its gcd.service workers write
	// to /var/lib/waagent/events and survive "stop" (KillMode=process), so kill them first. waagent stops
	// only its own service; extensions and the log collector run in other units. All best effort.
	for _, args := range [][]string{
		{"kill", "--signal=SIGKILL", "gcd.service"},
		{"disable", "--now", "gcd.service"},
		{"stop", "walinuxagent.service", "waagent.service", "azure-walinuxagent-logcollector.slice", "azure-vmextensions.slice"},
	} {
		if _, stopErr := g.runCmd(ctx, "systemctl", args...); stopErr != nil {
			slog.Info("stop WALinuxAgent state writers", "error", stopErr)
		}
	}

	// waagent stops at the first failed cleanup action but still exits 0. Do not retry: a partial run can
	// delete ovf-env.xml, and waagent then skips build-user deletion.
	output, err := g.runCmd(ctx, "waagent", "-force", "-deprovision+user")
	if err == nil && bytes.Contains(output, []byte("Failed to run 'deprovision")) {
		err = errors.New("waagent deprovision logged \"Failed to run 'deprovision\" but exited 0")
	}
	return err
}

// removeNvPCRAnchors removes systemd NvPCR anchors. Only builds with a vTPM have them. The SRK, NV
// indices, disk-encryption keys, and other system credentials stay intact.
func (g *Generalizer) removeNvPCRAnchors(context.Context, map[string]string) error {
	for _, p := range nvpcrAnchorPaths {
		path := g.path(p)
		if err := os.Remove(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("remove %s: %w", path, err)
		}
		slog.Info("removed NvPCR anchor", "path", path)
	}
	for _, d := range nvpcrCredentialDirs {
		if err := removeNvPCRCredentials(g.path(d)); err != nil {
			return err
		}
	}
	// /run/credentials/@encrypted is a RAM-only boot import. Removing its boot source above prevents
	// a reimport.
	return nil
}

// removeNvPCRCredentials removes top-level anchor files and symlinks in dir. A missing dir is not an error.
func removeNvPCRCredentials(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if !e.Type().IsRegular() && e.Type()&fs.ModeSymlink == 0 {
			continue
		}
		if ok, _ := filepath.Match("nvpcr-anchor.*.cred", strings.ToLower(e.Name())); !ok {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err = removeIfExists(path); err != nil {
			return err
		}
		slog.Info("removed NvPCR anchor", "path", path)
	}
	return nil
}

func (g *Generalizer) path(p string) string {
	if g.rootDir == "" {
		return p
	}
	return filepath.Join(g.rootDir, p)
}

// resetACLFirstBoot makes clones of an ACL image run Ignition again.
func (g *Generalizer) resetACLFirstBoot(_ context.Context, osRelease map[string]string) error {
	if strings.ToLower(osRelease["ID"]) != config.OSReleaseIDAzureContainerLinux &&
		strings.ToLower(osRelease["VARIANT_ID"]) != config.OSReleaseIDAzureContainerLinux {
		return nil
	}
	if err := g.restoreACLFirstBootAddon(); err != nil {
		return err
	}

	marker := g.path("/boot/flatcar/first_boot")
	f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", marker, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("create %s: %w", marker, err)
	}
	slog.Info("set ACL first boot marker", "path", marker)
	return nil
}

// restoreACLFirstBootAddon puts back the UKI firstboot addon, which Ignition consumes on the first
// boot. Without it, the kernel cmdline of a clone does not get flatcar.first_boot=detected.
func (g *Generalizer) restoreACLFirstBootAddon() error {
	source := g.path("/boot/acl/uki-addons/firstboot.addon.efi")
	if _, err := os.Stat(source); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", source, err)
	}
	uki, err := g.findACLUKI()
	if err != nil {
		return err
	}
	addon := filepath.Join(uki+".extra.d", "firstboot.addon.efi")
	if _, err = os.Stat(addon); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", addon, err)
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	if err = os.MkdirAll(filepath.Dir(addon), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(addon), err)
	}
	//nolint:gosec // addon is under the fixed ACL UKI directory; the name comes from a directory listing filtered by pattern
	if err = os.WriteFile(addon, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", addon, err)
	}
	slog.Info("restored ACL firstboot addon", "path", addon)
	return nil
}

// findACLUKI returns the active UKI. Newer ACL images use UAPI naming (vmlinuz-<version>.efi);
// older images use acl.efi.
func (g *Generalizer) findACLUKI() (string, error) {
	dir := g.path("/boot/EFI/Linux")
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() {
			continue
		}
		if name == aclLegacyUKIName || (strings.HasPrefix(name, "vmlinuz-") && strings.HasSuffix(name, ".efi")) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no ACL UKI found under %s (expected %s or vmlinuz-*.efi)", dir, aclLegacyUKIName)
	}
	sort.Strings(names)
	return filepath.Join(dir, names[0]), nil
}
