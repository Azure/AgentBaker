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
	"unicode"
)

func (c *cloudInitializer) refresh(ctx context.Context, platform cloudPlatform, location string) (bool, error) {
	legacy := strings.HasPrefix(location, "ussec") || strings.HasPrefix(location, "usnat")
	slog.Info("selecting Azure cloud certificate endpoint", "legacy", legacy, "location", location)
	if !legacy {
		body, err := c.requestCertificates(ctx, "/acms/isOptedInForRootCerts")
		if err != nil {
			return false, fmt.Errorf("check Azure cloud root certificate opt-in: %w", err)
		}
		var response struct {
			OptedIn *bool `json:"IsOptedInForRootCerts"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return false, fmt.Errorf("decode Azure cloud root certificate opt-in: %w", err)
		}
		if response.OptedIn == nil {
			return false, errors.New("cloud root certificate opt-in response is missing IsOptedInForRootCerts")
		}
		if !*response.OptedIn {
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
		return false, fmt.Errorf("install Azure cloud CA certificates: %w", err)
	}
	slog.Info("installed Azure cloud CA certificates", "count", len(certificates))
	return true, nil
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
		!strings.Contains(name, `\`) && strings.IndexFunc(name, unicode.IsControl) == -1
}

func (c *cloudInitializer) legacyCertificates(ctx context.Context) (map[string][]byte, error) {
	body, err := c.requestCertificates(ctx, certificatePackagePath("cacertificates", "json"))
	if err != nil {
		return nil, fmt.Errorf("retrieve legacy Azure cloud certificates: %w", err)
	}
	var response struct {
		Certificates []struct {
			Name string
			Body string `json:"CertBody"`
		}
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode legacy Azure cloud certificates: %w", err)
	}
	certificates := make(map[string][]byte)
	for _, cert := range response.Certificates {
		if !validCloudCertificateName(cert.Name) {
			slog.Warn("rejecting invalid Azure cloud certificate filename")
			continue
		}
		name := strings.ReplaceAll(cert.Name, ".cer", ".crt")
		data, err := publicCloudCertificates([]byte(cert.Body))
		if err != nil {
			return nil, fmt.Errorf("decode legacy Azure cloud certificate %s: %w", name, err)
		}
		if strings.HasSuffix(name, ".crt") {
			certificates[name] = data
		}
	}
	if len(certificates) == 0 {
		return nil, errors.New("legacy Azure cloud response contains no installable CA certificates")
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
			Name string `json:"ResouceFileName"`
		} `json:"OperationsInfo"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode Azure cloud %s: %w", kind, err)
	}
	certificates := make(map[string][]byte)
	for _, operation := range response.Operations {
		if !validCloudCertificateName(operation.Name) {
			slog.Warn("rejecting invalid Azure cloud certificate filename")
			continue
		}
		data, err := c.operationCertificate(ctx, operation.Name)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(operation.Name, ".crt") {
			certificates[operation.Name] = data
		}
	}
	if len(certificates) == 0 {
		return nil, fmt.Errorf("cloud %s contains no installable CA certificates", kind)
	}
	return certificates, nil
}

func (c *cloudInitializer) operationCertificate(ctx context.Context, filename string) ([]byte, error) {
	extension := filepath.Ext(filename)
	if extension == "" {
		return nil, errors.New("cloud certificate filename is missing an extension")
	}
	name := strings.TrimSuffix(filename, extension)
	body, err := c.requestCertificates(ctx, certificatePackagePath(name, strings.TrimPrefix(extension, ".")))
	if err != nil {
		return nil, fmt.Errorf("retrieve Azure cloud certificate %s: %w", filename, err)
	}
	data, err := publicCloudCertificates(body)
	if err != nil {
		return nil, fmt.Errorf("decode Azure cloud certificate %s: %w", filename, err)
	}
	return data, nil
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
	for _, name := range names {
		data := certificates[name]
		if err := c.writeFile("/root/AzureCACertificates/"+name, data); err != nil {
			return err
		}
		targetName := name
		if platform.ID == osReleaseIDFlatcar {
			targetName = strings.TrimSuffix(name, ".crt") + ".pem"
		}
		if err := c.writeFile(filepath.Join(destination, targetName), data); err != nil {
			return err
		}
		if os.Getenv("AKS_CLOUD_LOG_CERTIFICATES") == "true" {
			// Only re-encoded public certificates reach this opt-in diagnostic.
			slog.Info("Azure cloud public CA certificate", "name", name, "certificate", string(data))
		}
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
