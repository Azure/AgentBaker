package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/agentbaker/parts"
	"gopkg.in/yaml.v3"
)

type rcv1pRefreshArtifact struct {
	sha256 string
	origin string
}

// Inspect the actual AgentBaker render before test-only customData injections.
// CSE removes comments and templates scripts before delivery. Reimplementing
// that transformation (or hashing raw source) would drift from production.
// Only the digest and delivery origin are retained, not customData secrets.
func expectedRCV1PRefreshArtifact(customData string) (*rcv1pRefreshArtifact, error) {
	content, err := rcv1pRefreshPayload(customData)
	if err != nil {
		return nil, err
	}
	origin := "production-rendered customData"
	if content == nil {
		// Scriptless payloads normally leave this file on the VHD. In that
		// case require the exact current baked source, not an arbitrary image.
		origin = "current VHD source (not delivered by customData)"
		content, err = parts.Templates.ReadFile("linux/cloud-init/artifacts/init-aks-cloud.sh")
		if err != nil {
			return nil, err
		}
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("empty production refresh script")
	}
	return &rcv1pRefreshArtifact{sha256: fmt.Sprintf("%x", sha256.Sum256(content)), origin: origin}, nil
}

// nil content means a recognized production payload does not write the script.
// Unknown, malformed, or ambiguous payloads are errors, not a VHD fallback.
func rcv1pRefreshPayload(customData string) ([]byte, error) {
	payload, err := base64.StdEncoding.DecodeString(customData)
	if err != nil {
		return nil, fmt.Errorf("decode customData: %w", err)
	}
	if bytes.HasPrefix(payload, []byte{0x1f, 0x8b}) {
		payload, err = rcv1pGunzip(payload)
		if err != nil {
			return nil, fmt.Errorf("decompress customData: %w", err)
		}
	}
	switch {
	case bytes.HasPrefix(payload, []byte("#cloud-config\n")):
		var config struct {
			WriteFiles []struct {
				Path     string    `yaml:"path"`
				Encoding string    `yaml:"encoding"`
				Content  yaml.Node `yaml:"content"`
			} `yaml:"write_files"`
		}
		if err := yaml.Unmarshal(payload, &config); err != nil {
			return nil, fmt.Errorf("parse cloud-config: %w", err)
		}
		var script []byte
		for _, file := range config.WriteFiles {
			if file.Path != installedRCV1PScript {
				continue
			}
			if script != nil {
				return nil, fmt.Errorf("duplicate refresh script in cloud-config")
			}
			if file.Encoding != "gzip" || file.Content.Tag != "!!binary" {
				return nil, fmt.Errorf("unexpected refresh script cloud-config encoding")
			}
			compressed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(file.Content.Value))
			if err != nil {
				return nil, fmt.Errorf("decode rendered refresh script: %w", err)
			}
			script, err = rcv1pGunzip(compressed)
			if err != nil || len(script) == 0 {
				return nil, fmt.Errorf("invalid compressed refresh script")
			}
		}
		return script, nil
	case bytes.HasPrefix(payload, []byte("#cloud-boothook\n")):
		if bytes.Contains(payload, []byte(installedRCV1PScript)) {
			return nil, fmt.Errorf("unsupported boothook refresh script override")
		}
		return nil, nil
	case bytes.HasPrefix(payload, []byte("{")):
		return rcv1pRefreshIgnitionPayload(payload)
	default:
		return nil, fmt.Errorf("unrecognized production customData format")
	}
}

func rcv1pGunzip(compressed []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// ACL/Flatcar scripted CSE packages cloud-init files in the production Ignition
// tarball. Read that existing payload; never fetch external content or extract
// it to the filesystem.
func rcv1pRefreshIgnitionPayload(payload []byte) ([]byte, error) {
	const tarPath = "/var/lib/ignition/ignition-files.tar"
	var config struct {
		Ignition struct{ Version string }
		Storage  struct {
			Files []struct {
				Path     string
				Contents struct{ Source, Compression string }
			}
		}
	}
	if err := json.Unmarshal(payload, &config); err != nil || config.Ignition.Version == "" {
		return nil, fmt.Errorf("invalid production Ignition customData")
	}
	var script []byte
	foundTar := false
	for _, file := range config.Storage.Files {
		if file.Path == installedRCV1PScript {
			return nil, fmt.Errorf("unsupported direct Ignition refresh script override")
		}
		if file.Path != tarPath {
			continue
		}
		if foundTar {
			return nil, fmt.Errorf("duplicate production Ignition tarball")
		}
		foundTar = true
		const prefix = "data:;base64,"
		if !strings.HasPrefix(file.Contents.Source, prefix) || file.Contents.Compression != "gzip" {
			return nil, fmt.Errorf("unexpected production Ignition tarball encoding")
		}
		compressed, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(file.Contents.Source, prefix))
		if err != nil {
			return nil, fmt.Errorf("invalid production Ignition tarball base64")
		}
		tarball, err := rcv1pGunzip(compressed)
		if err != nil {
			return nil, fmt.Errorf("invalid compressed production Ignition tarball")
		}
		reader := tar.NewReader(bytes.NewReader(tarball))
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("read production Ignition tar: %w", err)
			}
			if "/"+strings.TrimPrefix(header.Name, "/") != installedRCV1PScript {
				continue
			}
			if script != nil || header.Typeflag != tar.TypeReg || header.Size == 0 {
				return nil, fmt.Errorf("duplicate or invalid Ignition refresh script")
			}
			script, err = io.ReadAll(reader)
			if err != nil {
				return nil, fmt.Errorf("read Ignition refresh script: %w", err)
			}
		}
	}
	return script, nil
}
