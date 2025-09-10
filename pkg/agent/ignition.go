package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// createBaseFlatcarIgnitionConfig creates the base Flatcar configuration using Go structs
func createBaseFlatcarIgnitionConfig() IgnitionConfig {
	protocolsContent := "C /etc/protocols - - - - /usr/share/baselayout/protocols\n"
	updateCaServiceContent := `[Unit]
Description=Update CA certificates if missing or symlink
DefaultDependencies=no
After=local-fs.target
ExecCondition=/bin/sh -c '[ ! -e /etc/ssl/certs/ca-certificates.crt ] || [ -L /etc/ssl/certs/ca-certificates.crt ]'

[Service]
Type=oneshot
ExecStartPre=/usr/bin/rm -f /etc/ssl/certs/ca-certificates.crt
ExecStart=/usr/sbin/update-ca-certificates
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
`

	return IgnitionConfig{
		Ignition: IgnitionSection{
			Version: "3.4.0",
		},
		Storage: &StorageSection{
			Files: []File{
				{
					Path: "/etc/tmpfiles.d/protocols.conf",
					Contents: &FileContents{
						Source: fmt.Sprintf("data:,%s", url.QueryEscape(protocolsContent)),
					},
					Mode: toPointer(420),
				},
			},
		},
		Systemd: &SystemdSection{
			Units: []Unit{
				{
					Name:     "update-ca.service",
					Enabled:  toPointer(true),
					Contents: toPointer(updateCaServiceContent),
				},
			},
		},
	}
}

// toPointer creates a pointer to the given value
func toPointer[T any](v T) *T {
	return &v
}

// Ignition v3.4 structures for direct JSON generation, replacing butane dependency

// IgnitionConfig represents the root Ignition configuration
type IgnitionConfig struct {
	Ignition IgnitionSection `json:"ignition"`
	Storage  *StorageSection `json:"storage,omitempty"`
	Systemd  *SystemdSection `json:"systemd,omitempty"`
}

// IgnitionSection contains metadata about the Ignition configuration
type IgnitionSection struct {
	Version string                 `json:"version"`
	Config  *IgnitionConfigSection `json:"config,omitempty"`
}

// IgnitionConfigSection represents config replacement/merge in Ignition
type IgnitionConfigSection struct {
	Replace *IgnitionResource  `json:"replace,omitempty"`
	Merge   []IgnitionResource `json:"merge,omitempty"`
}

// IgnitionResource represents a remote configuration resource
type IgnitionResource struct {
	Source      string `json:"source"`
	Compression string `json:"compression,omitempty"`
}

// StorageSection contains storage-related configuration
type StorageSection struct {
	Files []File `json:"files,omitempty"`
}

// File represents a file to be created
type File struct {
	Path      string        `json:"path"`
	Contents  *FileContents `json:"contents,omitempty"`
	Mode      *int          `json:"mode,omitempty"`
	User      *FileUser     `json:"user,omitempty"`
	Overwrite *bool         `json:"overwrite,omitempty"`
}

// FileContents represents the contents of a file
type FileContents struct {
	Source      string  `json:"source,omitempty"`
	Compression string  `json:"compression,omitempty"`
	Inline      *string `json:"inline,omitempty"`
}

// FileUser represents file ownership information
type FileUser struct {
	Name *string `json:"name,omitempty"`
}

// SystemdSection contains systemd-related configuration
type SystemdSection struct {
	Units []Unit `json:"units,omitempty"`
}

// Unit represents a systemd unit
type Unit struct {
	Name     string  `json:"name"`
	Enabled  *bool   `json:"enabled,omitempty"`
	Contents *string `json:"contents,omitempty"`
}

// toIgnitionFile converts a cloud-init write file to an Ignition file
func toIgnitionFile(file cloudInitWriteFile) (File, error) {
	ignFile := File{
		Path:      file.Path,
		Overwrite: toPointer(true),
	}

	if file.Owner != "" {
		ignFile.User = &FileUser{
			Name: toPointer(file.Owner),
		}
	}

	if file.Permissions != "" {
		mode, err := strconv.ParseInt(file.Permissions, 8, 32)
		if err != nil {
			return ignFile, fmt.Errorf("failed to parse file mode: %w", err)
		}
		ignFile.Mode = toPointer(int(mode))
	}

	if file.Content != "" {
		contents := FileContents{}
		switch file.Encoding {
		case "gzip", "base64":
			contents.Inline = &file.Content
			contents.Compression = file.Encoding
		default:
			contents.Source = fmt.Sprintf("data:,%s", url.QueryEscape(file.Content))
		}
		ignFile.Contents = &contents
	}

	return ignFile, nil
}

// createGzippedDataURL creates a gzipped, base64-encoded data URL for the given content
func createGzippedDataURL(content string) (string, error) {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)

	if _, err := gzWriter.Write([]byte(content)); err != nil {
		return "", fmt.Errorf("failed to gzip content: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close gzip writer: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return fmt.Sprintf("data:;base64,%s", encoded), nil
}

// buildFlatcarIgnitionConfig builds a complete Ignition configuration for Flatcar.
// It takes custom data files, converts them to Ignition format, and merges them
// with a base Flatcar configuration. The final result is a gzipped, base64-encoded
// Ignition config wrapped in an "envelope" config.
func buildFlatcarIgnitionConfig(customDataFiles []cloudInitWriteFile) (*IgnitionConfig, error) {
	ignitionFiles := make([]File, 0, len(customDataFiles))
	for _, file := range customDataFiles {
		ignFile, err := toIgnitionFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to convert file %s: %w", file.Path, err)
		}
		ignitionFiles = append(ignitionFiles, ignFile)
	}

	baseConfig := createBaseFlatcarIgnitionConfig()
	if baseConfig.Storage == nil {
		baseConfig.Storage = &StorageSection{}
	}
	baseConfig.Storage.Files = append(ignitionFiles, baseConfig.Storage.Files...)

	innerConfigJSON, err := json.Marshal(baseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal inner Ignition config: %w", err)
	}

	gzippedDataURL, err := createGzippedDataURL(string(innerConfigJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzipped data URL: %w", err)
	}

	return &IgnitionConfig{
		Ignition: IgnitionSection{
			Version: "3.4.0",
			Config: &IgnitionConfigSection{
				Replace: &IgnitionResource{
					Source:      gzippedDataURL,
					Compression: "gzip",
				},
			},
		},
	}, nil
}
