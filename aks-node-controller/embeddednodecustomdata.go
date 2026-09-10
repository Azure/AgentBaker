package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
)

const defaultOSReleasePath = "/etc/os-release"

// os-release ID values that appear in more than one classification path.
const (
	osReleaseIDAzureContainerLinux = "azurecontainerlinux"
	osReleaseIDAzureLinux          = "azurelinux"
	osReleaseIDFlatcar             = "flatcar"
)

type nodeCustomDataPlatform string

const (
	nodeCustomDataPlatformUbuntu      nodeCustomDataPlatform = "ubuntu"
	nodeCustomDataPlatformMariner     nodeCustomDataPlatform = "mariner"
	nodeCustomDataPlatformUnsupported nodeCustomDataPlatform = "unsupported"
)

//go:embed scripthotfix/generated
var embeddedGeneratedNodeCustomData embed.FS

func applyEmbeddedNodeCustomData(payloadFS fs.FS, osReleasePath string) error {
	if osReleasePath == "" {
		osReleasePath = defaultOSReleasePath
	}
	platform, err := classifyNodeCustomDataPlatform(osReleasePath)
	if err != nil {
		return err
	}
	if platform == nodeCustomDataPlatformUnsupported {
		slog.Info("embedded script hotfix is not supported on this OS, skipping", "osReleasePath", osReleasePath)
		return nil
	}
	renderedPath := fmt.Sprintf("scripthotfix/generated/rendered_nodecustomdata_%s.yml", platform)
	data, err := fs.ReadFile(payloadFS, renderedPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read embedded nodecustomdata %s: %w", renderedPath, err)
	}
	temp, err := os.CreateTemp("", "aks-node-controller-nodecustomdata-*.yml")
	if err != nil {
		return fmt.Errorf("create temporary nodecustomdata: %w", err)
	}
	defer func() {
		if err := os.Remove(temp.Name()); err != nil {
			slog.Warn("failed to remove temporary nodecustomdata", "path", temp.Name(), "error", err)
		}
	}()
	_, writeErr := temp.Write(data)
	closeErr := temp.Close()
	if writeErr != nil {
		return fmt.Errorf("write temporary nodecustomdata: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close temporary nodecustomdata: %w", closeErr)
	}
	if err := applyNodeCustomData(temp.Name()); err != nil {
		return err
	}
	slog.Info("applied embedded hotfix payload", "source", renderedPath)
	return nil
}

func classifyNodeCustomDataPlatform(osReleasePath string) (nodeCustomDataPlatform, error) {
	data, err := os.ReadFile(osReleasePath)
	if err != nil {
		return "", fmt.Errorf("read OS release %s: %w", osReleasePath, err)
	}
	values := parseNodeCustomDataOSRelease(data)
	id := strings.ToLower(values["ID"])
	variant := strings.ToLower(values["VARIANT_ID"])

	switch {
	// Exclude immutable variants before matching their shared Azure Linux ID.
	case variant == "osguard", variant == osReleaseIDAzureContainerLinux,
		id == osReleaseIDAzureContainerLinux, id == osReleaseIDFlatcar:
		return nodeCustomDataPlatformUnsupported, nil
	case id == "ubuntu":
		return nodeCustomDataPlatformUbuntu, nil
	case id == osReleaseIDAzureLinux:
		return nodeCustomDataPlatformMariner, nil
	case id == "":
		return "", fmt.Errorf("ID is missing from %s", osReleasePath)
	default:
		return "", fmt.Errorf("unsupported OS ID %q in %s", id, osReleasePath)
	}
}

func parseNodeCustomDataOSRelease(data []byte) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values
}
