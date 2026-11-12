package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
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
	layers, err := imageLayers(parseNodeCustomDataOSRelease(release))
	if err != nil {
		return err
	}
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
		layers = append(layers, "overlays/"+overlay)
	}
	if err := unpackRoot(parts.Root, layers, destination); err != nil {
		return err
	}
	// Hotfixes must use the image's feature selections, not the build host's environment.
	if len(overlays) > 0 {
		if err := writeRootFile(selection, []byte(strings.Join(overlays, "\n")+"\n"), 0o644); err != nil {
			return fmt.Errorf("record image overlays: %w", err)
		}
	}
	slog.Info("unpacked embedded image", "layers", layers, "destination", destination)
	return nil
}

func imageLayers(release map[string]string) ([]string, error) {
	id, variant := strings.ToLower(release["ID"]), strings.ToLower(release["VARIANT_ID"])
	layers := []string{"common"}
	if id != osReleaseIDFlatcar {
		layers = append(layers, "current")
		if variant != osVariantIDOSGuard {
			layers = append(layers, "hosts")
		}
	}
	switch {
	case variant == osVariantIDOSGuard:
		return append(layers, "rpm", "azlosguard"), nil
	case id == osReleaseIDAzureContainerLinux || variant == osReleaseIDAzureContainerLinux:
		return append(layers, "linux", "rpm", "flatcar-notice", "acl"), nil
	case id == osReleaseIDUbuntu:
		layers = append(layers, "linux", "packages", "observability", "ubuntu")
		if release["VERSION_ID"] != "18.04" && release["VERSION_ID"] != "20.04" {
			layers = append(layers, "ubuntu-modern")
		}
	case id == osReleaseIDAzureLinux || id == "mariner" || id == "azurelinuxkata" || id == "marinerkata":
		layers = append(layers, "linux", "rpm", "packages", "mariner")
		if id == osReleaseIDAzureLinux || id == "azurelinuxkata" {
			layers = append(layers, "observability")
			if release["VERSION_ID"] == "3.0" {
				layers = append(layers, "azurelinux")
			}
		}
	case id == osReleaseIDFlatcar:
		layers = append(layers, "linux", "flatcar-notice", "flatcar")
	default:
		return nil, fmt.Errorf("unsupported OS: ID=%q VARIANT_ID=%q", id, variant)
	}
	return layers, nil
}

func unpackRoot(source fs.FS, layers []string, destination string) error {
	for _, layer := range layers {
		root := "root/" + layer
		err := fs.WalkDir(source, root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			relative := strings.TrimPrefix(name, root+"/")
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
			content, err := fs.ReadFile(source, name)
			if err != nil {
				return err
			}
			return writeRootFile(output, content, mode)
		})
		if err != nil {
			return fmt.Errorf("unpack %s: %w", layer, err)
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
