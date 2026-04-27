package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"
)

const (
	defaultNodeCustomDataPath         = "/opt/azure/containers/nodecustomdata.yml"
	defaultScriptHotfixManifestPath   = "/opt/azure/hotfix/scripts/manifest.json"
	encodingGZIP                      = "gzip"
	encodingBase64                    = "base64"
)

type nodeCustomDataWriteFile struct {
	Path        string `yaml:"path"`
	Permissions string `yaml:"permissions"`
	Encoding    string `yaml:"encoding,omitempty"`
	Owner       string `yaml:"owner"`
	Content     string `yaml:"content"`
}

type nodeCustomData struct {
	WriteFiles []nodeCustomDataWriteFile `yaml:"write_files"`
}

// scriptHotfixManifest describes version-scoped script hotfix files staged by cloud-init.
// ANC reads this manifest, checks the target version against its own VHD version,
// and copies staged files to their real destinations only when the version matches.
type scriptHotfixManifest struct {
	TargetVersion string              `json:"targetVersion"`
	Files         []scriptHotfixFile  `json:"files"`
}

type scriptHotfixFile struct {
	Staging     string `json:"staging"`
	Destination string `json:"destination"`
}

func applyNodeCustomData(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read nodecustomdata %s: %w", path, err)
	}

	var customData nodeCustomData
	if err := yaml.Unmarshal(data, &customData); err != nil {
		return fmt.Errorf("unmarshal nodecustomdata %s: %w", path, err)
	}

	for _, file := range customData.WriteFiles {
		if err := applyNodeCustomDataWriteFile(file); err != nil {
			return fmt.Errorf("apply nodecustomdata write file %s: %w", file.Path, err)
		}
	}

	return nil
}

func applyNodeCustomDataWriteFile(file nodeCustomDataWriteFile) error {
	if file.Path == "" {
		return fmt.Errorf("path is required")
	}
	if file.Owner != "" && file.Owner != "root" {
		return fmt.Errorf("unsupported owner %q", file.Owner)
	}

	mode := os.FileMode(0o644)
	if file.Permissions != "" {
		parsedMode, err := strconv.ParseUint(file.Permissions, 8, 32)
		if err != nil {
			return fmt.Errorf("parse permissions: %w", err)
		}
		mode = os.FileMode(parsedMode)
	}

	contents, err := decodeNodeCustomDataWriteFileContent(file)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	if err := os.WriteFile(file.Path, contents, mode); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func decodeNodeCustomDataWriteFileContent(file nodeCustomDataWriteFile) ([]byte, error) {
	switch file.Encoding {
	case "":
		return []byte(file.Content), nil
	case encodingGZIP:
		reader, err := gzip.NewReader(bytes.NewReader([]byte(file.Content)))
		if err != nil {
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer reader.Close()

		decoded, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read gzip content: %w", err)
		}
		return decoded, nil
	case encodingBase64:
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return nil, fmt.Errorf("decode base64 content: %w", err)
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", file.Encoding)
	}
}

// applyScriptHotfix reads the script hotfix manifest, checks if the target version
// matches the current ANC/VHD version (same YYYYMM.DD base), and copies staged
// hotfix scripts to their real destinations. Returns nil if no manifest exists.
func applyScriptHotfix(manifestPath, currentVersion string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("no script hotfix manifest found, skipping", "path", manifestPath)
			return nil
		}
		return fmt.Errorf("read script hotfix manifest %s: %w", manifestPath, err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	var manifest scriptHotfixManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse script hotfix manifest %s: %w", manifestPath, err)
	}

	if manifest.TargetVersion == "" || len(manifest.Files) == 0 {
		slog.Info("script hotfix manifest is empty, skipping", "path", manifestPath)
		return nil
	}

	shouldApply, err := isScriptHotfixTargeted(currentVersion, manifest.TargetVersion)
	if err != nil {
		slog.Warn("failed to compare versions for script hotfix, skipping",
			"current", currentVersion, "target", manifest.TargetVersion, "error", err)
		return nil
	}
	if !shouldApply {
		slog.Info("script hotfix not targeted for this VHD, skipping",
			"current", currentVersion, "target", manifest.TargetVersion)
		return nil
	}

	slog.Info("applying script hotfix",
		"current", currentVersion, "target", manifest.TargetVersion, "fileCount", len(manifest.Files))

	for _, f := range manifest.Files {
		if err := copyHotfixFile(f.Staging, f.Destination); err != nil {
			return fmt.Errorf("apply script hotfix %s → %s: %w", f.Staging, f.Destination, err)
		}
		slog.Info("applied script hotfix file", "staging", f.Staging, "destination", f.Destination)
	}

	return nil
}

// isScriptHotfixTargeted returns true when the current ANC/VHD version matches the
// hotfix target version's YYYYMM.DD base (any patch). This allows a hotfix targeting
// "202604.27.0" to apply on nodes running "202604.27.0" or "202604.27.1" (ANC-hotfixed).
func isScriptHotfixTargeted(current, target string) (bool, error) {
	cv, err := semver.NewVersion(strings.TrimSpace(current))
	if err != nil {
		return false, fmt.Errorf("parsing current version %q: %w", current, err)
	}
	tv, err := semver.NewVersion(strings.TrimSpace(target))
	if err != nil {
		return false, fmt.Errorf("parsing target version %q: %w", target, err)
	}
	return cv.Major() == tv.Major() && cv.Minor() == tv.Minor(), nil
}

// copyHotfixFile copies a staged hotfix file to its destination, preserving the
// source file's permissions. Uses atomic write (temp + rename) for safety.
func copyHotfixFile(src, dst string) error {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading staged file %s: %w", src, err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat staged file %s: %w", src, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", dst, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".script-hotfix-*")
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", dst, err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(srcData); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Chmod(srcInfo.Mode()); err != nil {
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
	return nil
}
