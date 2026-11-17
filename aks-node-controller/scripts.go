package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Azure/agentbaker/parts"
)

const (
	imageOverlaysPath   = "etc/aks/image-overlays"
	logrotateDropInPath = "etc/systemd/system/logrotate.timer.d/override.conf"
)

func unpackScripts(osReleasePath, destination string, overlays []string) error {
	if !filepath.IsAbs(destination) {
		return fmt.Errorf("destination must be absolute: %q", destination)
	}
	if osReleasePath == "" {
		osReleasePath = defaultOSReleasePath
	}
	release, err := os.ReadFile(osReleasePath)
	if err != nil {
		return fmt.Errorf("read OS release: %w", err)
	}
	osRoot, err := imageRoot(parseNodeCustomDataOSRelease(release))
	if err != nil {
		return err
	}
	roots := []string{"common", osRoot}
	selection := filepath.Join(destination, imageOverlaysPath)
	if len(overlays) == 0 {
		data, err := os.ReadFile(selection)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("read image overlays: %w", err)
		}
		overlays = strings.Fields(string(data))
	}
	for _, overlay := range overlays {
		if !fs.ValidPath(overlay) || strings.Contains(overlay, "/") || overlay == "." {
			return fmt.Errorf("invalid image overlay %q", overlay)
		}
		roots = append(roots, "overlays/"+overlay)
	}
	if err := unpackRoot(parts.Root, roots, destination); err != nil {
		return err
	}
	// Hotfixes must use the image's feature selections, not the build host's environment.
	if len(overlays) > 0 {
		if err := writeRootFile(selection, []byte(strings.Join(overlays, "\n")+"\n"), 0o644); err != nil {
			return fmt.Errorf("record image overlays: %w", err)
		}
	}
	slog.Info("unpacked embedded image", "roots", roots, "destination", destination)
	return nil
}

func imageRoot(release map[string]string) (string, error) {
	id, variant := strings.ToLower(release["ID"]), strings.ToLower(release["VARIANT_ID"])
	switch {
	case variant == osVariantIDOSGuard:
		return "osguard", nil
	case id == osReleaseIDAzureContainerLinux || variant == osReleaseIDAzureContainerLinux:
		return "acl", nil
	case id == osReleaseIDUbuntu:
		if release["VERSION_ID"] == "18.04" || release["VERSION_ID"] == "20.04" {
			return "ubuntu-" + release["VERSION_ID"], nil
		}
		return "ubuntu", nil
	case id == osReleaseIDAzureLinux || id == "azurelinuxkata":
		if release["VERSION_ID"] == "3.0" {
			return "azurelinux-3.0", nil
		}
		return "azurelinux", nil
	case id == "mariner" || id == "marinerkata":
		return "mariner", nil
	case id == osReleaseIDFlatcar:
		return "flatcar", nil
	default:
		return "", fmt.Errorf("unsupported OS: ID=%q VARIANT_ID=%q", id, variant)
	}
}

type imageFile struct {
	source  string
	content []byte
}

func readImageFile(source fs.FS, root, name string) (string, imageFile, error) {
	relative := strings.TrimPrefix(name, root+"/")
	content, err := fs.ReadFile(source, name)
	if err != nil {
		return "", imageFile{}, err
	}
	if strings.HasSuffix(relative, ".link") {
		relative = strings.TrimSuffix(relative, ".link")
		target := strings.TrimSpace(string(content))
		if !fs.ValidPath(target) || !strings.HasPrefix(target, "shared/") ||
			strings.ContainsAny(target, "\\\r\n") || strings.HasSuffix(target, ".link") {
			return "", imageFile{}, fmt.Errorf("%s: invalid shared reference %q", name, target)
		}
		info, err := fs.Stat(source, "root/"+target)
		if err != nil {
			return "", imageFile{}, fmt.Errorf("%s: resolve %s: %w", name, target, err)
		}
		if !info.Mode().IsRegular() {
			return "", imageFile{}, fmt.Errorf("%s: reference %s is not a regular file", name, target)
		}
		content, err = fs.ReadFile(source, "root/"+target)
		if err != nil {
			return "", imageFile{}, fmt.Errorf("%s: read %s: %w", name, target, err)
		}
	}
	if !fs.ValidPath(relative) || relative == "." {
		return "", imageFile{}, fmt.Errorf("%s: invalid destination %q", name, relative)
	}
	return relative, imageFile{source: name, content: content}, nil
}

func addImageTree(source fs.FS, directory string, files map[string]imageFile, overrides map[string]string) error {
	root := "root/" + directory
	seen := make(map[string]string)
	return fs.WalkDir(source, root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", name)
		}
		relative, file, err := readImageFile(source, root, name)
		if err != nil {
			return err
		}
		if previous, ok := seen[relative]; ok {
			return fmt.Errorf("destination %s conflicts between %s and %s", relative, previous, name)
		}
		seen[relative] = name
		if strings.HasPrefix(directory, "overlays/") {
			if previous, ok := overrides[relative]; ok {
				return fmt.Errorf("destination %s conflicts between %s and %s", relative, previous, name)
			}
			overrides[relative] = name
		}
		files[relative] = file
		return nil
	})
}

// Resolve the complete plan first: a broken reference or ambiguous destination
// must not leave the node with only part of its image updated.
func resolveImageFiles(source fs.FS, roots []string) (map[string]imageFile, error) {
	files := make(map[string]imageFile)
	overrides := make(map[string]string)
	for _, directory := range roots {
		if err := addImageTree(source, directory, files, overrides); err != nil {
			return nil, fmt.Errorf("unpack %s: %w", directory, err)
		}
	}
	if extra, ok := files["NOTICE_FLATCAR.txt"]; ok {
		notice, ok := files["NOTICE.txt"]
		if !ok {
			return nil, errors.New("flatcar notices require the common NOTICE.txt")
		}
		// Existing ACL/Flatcar images contain both licenses in this single file.
		notice.content = bytes.Join([][]byte{notice.content, extra.content}, []byte("\n"))
		files["NOTICE.txt"] = notice
		delete(files, "NOTICE_FLATCAR.txt")
	}
	if slices.Contains(roots, "overlays/logrotate") {
		// The bundled timer already has the schedule; the drop-in is only for distro timers.
		delete(files, logrotateDropInPath)
	}
	for name, file := range files {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if previous, ok := files[parent]; ok {
				return nil, fmt.Errorf("file/directory conflict between %s and %s", previous.source, file.source)
			}
		}
	}
	return files, nil
}

// Keep the installed permissions from Packer and ImageCustomizer. Git modes and
// embedded filesystems cannot represent the original 0444/0544/0600/0744 distinctions.
func imageFileMode(name string, osguard bool) fs.FileMode {
	if osguard {
		if path.Dir(name) == "etc/systemd/system" &&
			(strings.HasSuffix(name, ".service") || strings.HasSuffix(name, ".timer") ||
				strings.HasSuffix(name, ".path") || strings.HasSuffix(name, ".slice")) &&
			name != "etc/systemd/system/disk_queue.service" &&
			name != "etc/systemd/system/localdns-exporter@.service" {
			return 0o600
		}
		switch name {
		case "opt/azure/containers/cloud-init-status-check.sh", "opt/azure/containers/setup-custom-search-domains.sh":
			return 0o755
		case "opt/scripts/ipv6_nftables.sh":
			return 0o744
		}
	}
	switch name {
	case "NOTICE.txt":
		return 0o444
	case "etc/ssh/sshd_config", "etc/security/pwquality.conf", "etc/security/faillock.conf",
		"etc/systemd/system/bind-mount.service", "etc/systemd/system/dhcpv6.service",
		"etc/systemd/system/ensure-no-dup.service", "etc/systemd/system/kubelet.service",
		"etc/systemd/system/mig-partition.service", "etc/systemd/system/reconcile-private-hosts.service",
		"etc/systemd/system/secure-tls-bootstrap.service", "etc/systemd/system/sync-container-logs.service",
		"etc/systemd/system/format-mount-nvme-root.service", "etc/systemd/system/kubelet.service.d/11-fmtmount.conf",
		"etc/systemd/system/trident-acl-agent.path", "etc/systemd/system/trident-acl-agent.service.d/10-aks.conf":
		return 0o600
	case "opt/azure/containers/bind-mount.sh", "opt/azure/containers/enable-dhcpv6.sh",
		"opt/azure/containers/sync-container-logs.sh", "opt/azure/containers/mig-partition.sh",
		"opt/azure/containers/security-update.sh", "opt/azure/containers/npd-update.sh",
		"opt/azure/containers/ubuntu-snapshot-update.sh", "opt/azure/containers/mariner-package-update.sh",
		"opt/azure/containers/format-mount-nvme-root.sh", "opt/bin/logrotate.sh":
		return 0o544
	case "opt/azure/containers/cleanup-vhd.sh", "opt/azure/containers/install_walinuxagent.py",
		"opt/scripts/cgroup-telemetry-events.sh":
		return 0o644
	case "opt/azure/containers/cloud-init-status-check.sh", "opt/azure/containers/fetch_provision_config.py",
		"opt/azure/containers/init-aks-cloud.sh", "opt/azure/containers/reconcilePrivateHosts.sh",
		"opt/azure/containers/setup-custom-search-domains.sh":
		return 0o744
	}
	if strings.HasPrefix(name, "opt/azure/containers/provision") {
		return 0o744
	}
	if strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".py") || strings.HasPrefix(name, "etc/grub.d/") {
		return 0o755
	}
	return 0o644
}

func imageOutputPath(destination, relative string) (string, error) {
	output := filepath.Join(destination, filepath.FromSlash(relative))
	info, err := os.Lstat(output)
	if errors.Is(err, fs.ErrNotExist) {
		return output, nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return output, nil
	}
	// Packer's copy preserved distro links such as /etc/issue -> ../usr/lib/issue.
	target, err := filepath.EvalSymlinks(output)
	if err != nil {
		return "", fmt.Errorf("resolve image destination %s: %w", relative, err)
	}
	root, err := filepath.EvalSymlinks(destination)
	if err != nil {
		return "", err
	}
	withinRoot, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if withinRoot == ".." || strings.HasPrefix(withinRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("image destination %s escapes destination root", relative)
	}
	return target, nil
}

func unpackRoot(source fs.FS, roots []string, destination string) error {
	files, err := resolveImageFiles(source, roots)
	if err != nil {
		return err
	}
	basePrepared := false
	if info, err := os.Stat(filepath.Join(destination, "opt/azure/containers/base_prep.complete")); err == nil {
		basePrepared = info.Mode().IsRegular()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect base preparation marker: %w", err)
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, relative := range names {
		output, err := imageOutputPath(destination, relative)
		if err != nil {
			return err
		}
		mode := imageFileMode(relative, slices.Contains(roots, "osguard"))
		if info, err := os.Stat(output); err == nil {
			// basePrep owns the SSH policy and will not run again on a prepared image.
			if basePrepared && relative == "etc/ssh/sshd_config" {
				slog.Info("preserving prepared image SSH configuration")
				continue
			}
			mode = info.Mode().Perm()
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := writeRootFile(output, files[relative].content, mode); err != nil {
			return err
		}
	}
	if slices.Contains(roots, "overlays/logrotate") {
		if err := removeLogrotateDropIn(destination); err != nil {
			return err
		}
	}
	return nil
}

func removeLogrotateDropIn(destination string) error {
	// The initial unpack can install this before the build selects its timer.
	dropIn := filepath.Join(destination, logrotateDropInPath)
	if err := os.Remove(dropIn); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove distro logrotate drop-in: %w", err)
	}
	directory := filepath.Dir(dropIn)
	entries, err := os.ReadDir(directory)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect logrotate drop-ins: %w", err)
	}
	if err == nil && len(entries) == 0 {
		if err := os.Remove(directory); err != nil {
			return fmt.Errorf("remove empty logrotate drop-in directory: %w", err)
		}
	}
	return nil
}

func writeRootFile(destination string, content []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	// Atomic replacement lets a hotfix replace scripts without truncating running readers.
	file, err := os.CreateTemp(filepath.Dir(destination), ".anc-file-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), destination)
}
