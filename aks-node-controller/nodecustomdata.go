package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

const (
	defaultNodeCustomDataPath = "/opt/azure/containers/nodecustomdata.yml"
	encodingGZIP              = "gzip"
	encodingBase64            = "base64"
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

func applyNodeCustomData(path string) error {
	// nodecustomdata.yml is required input for the NBCCmd provisioning path, so a missing
	// file must be a hard error. applyWriteFiles itself treats absence as a no-op for
	// optional payloads (e.g. the script hotfix), so guard existence here before delegating.
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("read nodecustomdata %s: %w", path, err)
	}
	return applyWriteFiles(path)
}

// applyWriteFiles reads a #cloud-config write_files document and materializes each
// entry to disk. A missing file is treated as a no-op so callers can apply optional
// payloads (e.g. the script hotfix payload) without checking existence first.
func applyWriteFiles(path string) error {
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
