package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (c *cloudInitializer) clearCertificateCache() {
	directory := c.path("/root/AzureCACertificates")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		slog.Warn("could not create Azure cloud certificate cache", "error", err)
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		slog.Warn("could not read Azure cloud certificate cache", "error", err)
		return
	}
	// Certificate downloads are written directly here as files, without subdirectories.
	for _, entry := range entries {
		if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil {
			slog.Warn("could not remove cached Azure cloud certificate", "name", entry.Name(), "error", err)
		}
	}
}

func publicCloudCertificates(data []byte) ([]byte, error) {
	var result bytes.Buffer
	for {
		block, rest := pem.Decode(data)
		if block == nil {
			break
		}
		data = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return nil, fmt.Errorf("invalid CA certificate: %w", err)
		}
		if err := pem.Encode(&result, &pem.Block{Type: "CERTIFICATE", Bytes: block.Bytes}); err != nil {
			return nil, err
		}
	}
	if result.Len() == 0 {
		return nil, errors.New("response contains no PEM certificates")
	}
	return result.Bytes(), nil
}

func (c *cloudInitializer) installCertificates(ctx context.Context, platform cloudPlatform, certificates map[string][]byte) error {
	destination, update := "/usr/local/share/ca-certificates", "update-ca-certificates"
	switch platform.ID {
	case osReleaseIDAzureLinux, osReleaseIDMariner, osReleaseIDAzureContainerLinux:
		destination, update = "/etc/pki/ca-trust/source/anchors", "update-ca-trust"
	case osReleaseIDFlatcar:
		destination = "/etc/ssl/certs"
	}
	names := make([]string, 0, len(certificates))
	for name := range certificates {
		names = append(names, name)
	}
	sort.Strings(names)
	installed := 0
	for _, name := range names {
		data := certificates[name]
		if !strings.HasSuffix(name, ".crt") {
			continue
		}
		targetName := name
		if platform.ID == osReleaseIDFlatcar {
			targetName = strings.TrimSuffix(name, ".crt") + ".pem"
		}
		if err := c.writeFile(filepath.Join(destination, targetName), data); err != nil {
			return err
		}
		installed++
		if os.Getenv("AKS_CLOUD_LOG_CERTIFICATES") == "true" {
			public, err := publicCloudCertificates(data)
			if err != nil {
				slog.Warn("could not decode public Azure cloud CA certificates for logging", "name", name, "error", err)
				continue
			}
			slog.Info("Azure cloud public CA certificate", "name", name, "certificate", string(public))
		}
	}
	if installed == 0 {
		return errors.New("no *.crt files in Azure cloud certificate cache to install")
	}
	if _, err := c.command(ctx, nil, update); err != nil {
		return err
	}
	if platform.ID == osReleaseIDUbuntu {
		return c.copyOpenSSLBundle()
	}
	return nil
}

func (c *cloudInitializer) copyOpenSSLBundle() error {
	source, target := c.path("/etc/ssl/certs/ca-certificates.crt"), c.path("/usr/lib/ssl/cert.pem")
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	targetInfo, err := os.Stat(target)
	if err == nil && os.SameFile(sourceInfo, targetInfo) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return c.writeFile("/usr/lib/ssl/cert.pem", data)
}
