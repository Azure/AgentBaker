package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/agentbaker/aks-node-controller/config"
)

const embeddedNodeCustomDataPath = "/opt/azure/containers/embedded-nodecustomdata.yml"

type nodeCustomDataPlatform string

const (
	nodeCustomDataPlatformUbuntu      nodeCustomDataPlatform = "ubuntu"
	nodeCustomDataPlatformMariner     nodeCustomDataPlatform = "mariner"
	nodeCustomDataPlatformUnsupported nodeCustomDataPlatform = "unsupported"
)

//go:embed generated
var embeddedGeneratedNodeCustomData embed.FS

func applyEmbeddedNodeCustomData(payloadFS fs.FS, osReleasePath, outputPath string) error {
	if osReleasePath == "" {
		osReleasePath = config.OSReleasePath
	}
	platform, err := classifyNodeCustomDataPlatform(osReleasePath)
	if err != nil {
		return err
	}
	if platform == nodeCustomDataPlatformUnsupported {
		slog.Info("embedded script hotfix is not supported on this OS, skipping", "osReleasePath", osReleasePath)
		return nil
	}
	renderedPath := fmt.Sprintf("generated/rendered_nodecustomdata_%s.yml", platform)
	data, err := fs.ReadFile(payloadFS, renderedPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read embedded nodecustomdata %s: %w", renderedPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create embedded nodecustomdata directory: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return fmt.Errorf("write embedded nodecustomdata %s: %w", outputPath, err)
	}
	if err := applyNodeCustomData(outputPath); err != nil {
		return err
	}
	slog.Info("applied embedded hotfix payload", "source", renderedPath, "path", outputPath)
	return nil
}

func classifyNodeCustomDataPlatform(osReleasePath string) (nodeCustomDataPlatform, error) {
	data, err := os.ReadFile(osReleasePath)
	if err != nil {
		return "", fmt.Errorf("read OS release %s: %w", osReleasePath, err)
	}
	values := config.ParseOSRelease(data)
	id := strings.ToLower(values["ID"])
	variant := strings.ToLower(values["VARIANT_ID"])

	switch {
	// Exclude immutable variants before matching their shared Azure Linux ID.
	case variant == config.OSVariantIDOSGuard, variant == config.OSReleaseIDAzureContainerLinux,
		id == config.OSReleaseIDAzureContainerLinux, id == config.OSReleaseIDFlatcar:
		return nodeCustomDataPlatformUnsupported, nil
	case id == config.OSReleaseIDUbuntu:
		return nodeCustomDataPlatformUbuntu, nil
	case id == config.OSReleaseIDAzureLinux:
		return nodeCustomDataPlatformMariner, nil
	case id == "":
		return "", fmt.Errorf("ID is missing from %s", osReleasePath)
	default:
		return "", fmt.Errorf("unsupported OS ID %q in %s", id, osReleasePath)
	}
}
