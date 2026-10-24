package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (c *cloudInitializer) refresh(ctx context.Context, platform cloudPlatform, location string) (bool, error) {
	c.clearCertificateCache()
	legacy := strings.HasPrefix(location, "ussec") || strings.HasPrefix(location, "usnat")
	slog.Info("selecting Azure cloud certificate endpoint", "legacy", legacy, "location", location)
	if !legacy {
		body, err := c.requestCertificates(ctx, "/acms/isOptedInForRootCerts")
		if err != nil {
			return false, fmt.Errorf("check Azure cloud root certificate opt-in: %w", err)
		}
		if len(bytes.TrimRight(body, "\n")) == 0 {
			return false, errors.New("cloud root certificate opt-in response is empty")
		}
		var response map[string]json.RawMessage
		if err := json.Unmarshal(body, &response); err != nil {
			slog.Warn("could not decode Azure cloud root certificate opt-in, skipping certificate installation", "error", err)
			return false, nil
		}
		if !bytes.Equal(bytes.TrimSpace(response["IsOptedInForRootCerts"]), []byte("true")) {
			slog.Info("Azure cloud root certificates are not opted in, skipping certificate installation")
			return false, nil
		}
	}
	var certificates map[string][]byte
	var err error
	if legacy {
		certificates, err = c.legacyCertificates(ctx)
	} else {
		certificates, err = c.operationCertificates(ctx)
	}
	if err != nil {
		return false, err
	}
	if err := c.installCertificates(ctx, platform, certificates); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if legacy {
			slog.Warn("legacy Azure cloud CA certificate installation failed, continuing cloud initialization", "error", err)
			return true, nil
		}
		return false, fmt.Errorf("install Azure cloud CA certificates: %w", err)
	}
	slog.Info("installed Azure cloud CA certificates")
	return true, nil
}

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
	for _, entry := range entries {
		// Match the shell's non-recursive cleanup, which excludes hidden files.
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.IsDir() {
			slog.Warn("leaving directory in Azure cloud certificate cache", "name", entry.Name())
			continue
		}
		if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil {
			slog.Warn("could not remove cached Azure cloud certificate", "name", entry.Name(), "error", err)
		}
	}
}

func certificatePackagePath(kind, extension string) string {
	return "/machine?" + url.Values{
		"comp": {"acmspackage"},
		"type": {kind},
		"ext":  {extension},
	}.Encode()
}

func validCloudCertificateName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name &&
		!strings.ContainsRune(name, 0)
}

func (c *cloudInitializer) legacyCertificates(ctx context.Context) (map[string][]byte, error) {
	body, err := c.requestCertificates(ctx, certificatePackagePath("cacertificates", "json"))
	if err != nil {
		return nil, fmt.Errorf("retrieve legacy Azure cloud certificates: %w", err)
	}
	var response struct {
		Certificates []struct {
			Name json.RawMessage
			Body json.RawMessage `json:"CertBody"`
		}
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode legacy Azure cloud certificates: %w", err)
	}
	if response.Certificates == nil {
		return nil, errors.New("legacy Azure cloud response is missing a Certificates array")
	}
	certificates := make(map[string][]byte)
	for _, cert := range response.Certificates {
		var name string
		if err := json.Unmarshal(cert.Name, &name); err != nil || !validCloudCertificateName(name) {
			slog.Warn("rejecting invalid Azure cloud certificate filename")
			continue
		}
		var certBody *string
		if err := json.Unmarshal(cert.Body, &certBody); err != nil {
			return nil, fmt.Errorf("decode legacy Azure cloud certificate body %s: %w", name, err)
		}
		if certBody == nil {
			return nil, fmt.Errorf("legacy Azure cloud certificate %s is missing CertBody", name)
		}
		// Shell command substitution trims trailing LFs before CRLF normalization.
		data := strings.ReplaceAll(strings.TrimRight(*certBody, "\n"), "\r\n", "\n") + "\n"
		name = strings.ReplaceAll(name, ".cer", ".crt")
		if err := c.writeFile("/root/AzureCACertificates/"+name, []byte(data)); err != nil {
			return nil, fmt.Errorf("cache legacy Azure cloud certificate %s: %w", name, err)
		}
		certificates[name] = []byte(data)
	}
	return certificates, nil
}

func (c *cloudInitializer) operationCertificates(ctx context.Context) (map[string][]byte, error) {
	certificates := make(map[string][]byte)
	for _, kind := range []string{"operationrequestsroot", "operationrequestsintermediate"} {
		group, err := c.operationCertificateGroup(ctx, kind)
		if err != nil {
			return nil, err
		}
		maps.Copy(certificates, group)
	}
	return certificates, nil
}

func (c *cloudInitializer) operationCertificateGroup(ctx context.Context, kind string) (map[string][]byte, error) {
	body, err := c.requestCertificates(ctx, certificatePackagePath(kind, "json"))
	if err != nil {
		return nil, fmt.Errorf("retrieve Azure cloud %s: %w", kind, err)
	}
	var response struct {
		Operations []struct {
			// ResouceFileName is the spelling used by the WireServer protocol.
			Name json.RawMessage `json:"ResouceFileName"`
		} `json:"OperationsInfo"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode Azure cloud %s: %w", kind, err)
	}
	certificates := make(map[string][]byte)
	found := false
	for _, operation := range response.Operations {
		var name string
		if err := json.Unmarshal(operation.Name, &name); err != nil || name == "" {
			slog.Warn("ignoring missing or non-string Azure cloud certificate filename")
			continue
		}
		found = true
		if !validCloudCertificateName(name) {
			slog.Warn("rejecting invalid Azure cloud certificate filename")
			continue
		}
		data, err := c.operationCertificate(ctx, name)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			slog.Warn("could not retrieve Azure cloud certificate, continuing with remaining certificates", "name", name, "error", err)
			continue
		}
		if err := c.writeFile("/root/AzureCACertificates/"+name, data); err != nil {
			return nil, fmt.Errorf("cache Azure cloud certificate %s: %w", name, err)
		}
		certificates[name] = data
	}
	if !found {
		return nil, fmt.Errorf("cloud %s contains no certificate filenames", kind)
	}
	return certificates, nil
}

func (c *cloudInitializer) operationCertificate(ctx context.Context, filename string) ([]byte, error) {
	name, extension := filename, filename
	if suffix := filepath.Ext(filename); suffix != "" {
		name = strings.TrimSuffix(filename, suffix)
		extension = strings.TrimPrefix(suffix, ".")
	}
	body, err := c.requestCertificates(ctx, certificatePackagePath(name, extension))
	if err != nil {
		return nil, fmt.Errorf("retrieve Azure cloud certificate %s: %w", filename, err)
	}
	body = bytes.TrimRight(body, "\n")
	if len(body) == 0 {
		return nil, fmt.Errorf("cloud certificate %s response is empty", filename)
	}
	return append(body, '\n'), nil
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
