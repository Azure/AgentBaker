package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const defaultOSReleasePath = "/etc/os-release"

// os-release ID values that appear in more than one classification path.
const (
	osReleaseIDAzureContainerLinux = "azurecontainerlinux"
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

//nolint:gochecknoglobals // indirection point so tests can inject an alternate filesystem
var generatedNodeCustomData fs.FS = embeddedGeneratedNodeCustomData

func applyEmbeddedNodeCustomDataIfActive(osReleasePath string) (nodeCustomDataApplyResult, error) {
	active, err := fs.ReadFile(generatedNodeCustomData, "scripthotfix/generated/active")
	if err != nil {
		return nodeCustomDataApplyResult{}, fmt.Errorf("read embedded hotfix state: %w", err)
	}
	if strings.TrimSpace(string(active)) != "true" {
		return nodeCustomDataApplyResult{}, nil
	}
	if osReleasePath == "" {
		osReleasePath = defaultOSReleasePath
	}
	platform, err := classifyNodeCustomDataPlatform(osReleasePath)
	if err != nil {
		return nodeCustomDataApplyResult{}, err
	}
	if platform == nodeCustomDataPlatformUnsupported {
		slog.Info("embedded script hotfix is not supported on this OS, skipping", "osReleasePath", osReleasePath)
		return nodeCustomDataApplyResult{}, nil
	}
	return applyEmbeddedNodeCustomDataFS(generatedNodeCustomData, platform)
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
	case id == "mariner", id == "azurelinux":
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

func applyEmbeddedNodeCustomDataFS(
	payloadFS fs.FS,
	platform nodeCustomDataPlatform,
) (nodeCustomDataApplyResult, error) {
	if !isConcreteNodeCustomDataPlatform(platform) {
		return nodeCustomDataApplyResult{}, fmt.Errorf("unsupported concrete platform %q", platform)
	}
	renderedPath := filepath.ToSlash(filepath.Join(
		"scripthotfix",
		"generated",
		fmt.Sprintf("rendered_nodecustomdata_%s.yml", platform),
	))
	data, err := fs.ReadFile(payloadFS, renderedPath)
	if err != nil {
		return nodeCustomDataApplyResult{}, fmt.Errorf("read embedded nodecustomdata %s: %w", renderedPath, err)
	}
	return applyNodeCustomDataPayload(data, nodeCustomDataApplyOptions{
		source:             renderedPath,
		strict:             true,
		replaceOnly:        true,
		requirePermissions: true,
		rejectUnsafePaths:  true,
		rejectEmptyContent: true,
	})
}

func isConcreteNodeCustomDataPlatform(platform nodeCustomDataPlatform) bool {
	switch platform {
	case nodeCustomDataPlatformUbuntu,
		nodeCustomDataPlatformMariner:
		return true
	default:
		return false
	}
}
