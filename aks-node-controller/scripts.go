package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Azure/agentbaker/parts"
)

const imageOverlaysPath = "etc/aks/image-overlays"

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

// Resolve the complete plan first: a broken reference or ambiguous destination
// must not leave the node with only part of its image updated.
func resolveImageFiles(source fs.FS, roots []string) (map[string]imageFile, error) {
	files := make(map[string]imageFile)
	overrides := make(map[string]string)
	for _, directory := range roots {
		root := "root/" + directory
		seen := make(map[string]string)
		err := fs.WalkDir(source, root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("%s is not a regular file", name)
			}
			relative := strings.TrimPrefix(name, root+"/")
			content, err := fs.ReadFile(source, name)
			if err != nil {
				return err
			}
			if strings.HasSuffix(relative, ".link") {
				relative = strings.TrimSuffix(relative, ".link")
				target := strings.TrimSpace(string(content))
				if !fs.ValidPath(target) || !strings.HasPrefix(target, "shared/") ||
					strings.ContainsAny(target, "\\\r\n") || strings.HasSuffix(target, ".link") {
					return fmt.Errorf("%s: invalid shared reference %q", name, target)
				}
				info, err := fs.Stat(source, "root/"+target)
				if err != nil {
					return fmt.Errorf("%s: resolve %s: %w", name, target, err)
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("%s: reference %s is not a regular file", name, target)
				}
				content, err = fs.ReadFile(source, "root/"+target)
				if err != nil {
					return fmt.Errorf("%s: read %s: %w", name, target, err)
				}
			}
			if !fs.ValidPath(relative) || relative == "." {
				return fmt.Errorf("%s: invalid destination %q", name, relative)
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
			files[relative] = imageFile{source: name, content: content}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("unpack %s: %w", directory, err)
		}
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

func unpackRoot(source fs.FS, roots []string, destination string) error {
	files, err := resolveImageFiles(source, roots)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, relative := range names {
		output := filepath.Join(destination, filepath.FromSlash(relative))
		mode := fs.FileMode(0o644)
		if strings.HasSuffix(relative, ".sh") || strings.HasSuffix(relative, ".py") || strings.HasPrefix(relative, "etc/grub.d/") {
			mode = 0o755
		}
		if info, err := os.Stat(output); err == nil {
			mode = info.Mode().Perm()
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := writeRootFile(output, files[relative].content, mode); err != nil {
			return err
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
