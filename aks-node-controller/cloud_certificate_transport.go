package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/common"
)

const cloudRequestAttempts = 10

func (c *cloudInitializer) requestCertificates(ctx context.Context, path string) ([]byte, error) {
	attempt := 0
	delay, nextDelay := 3*time.Second, time.Duration(0)
	return common.RetryFetch(ctx, cloudRequestAttempts, nil, func(ctx context.Context) ([]byte, error) {
		if attempt > 0 {
			if err := c.wait(ctx, nextDelay); err != nil {
				return nil, err
			}
		}
		attempt++
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.wireEndpoint+path, nil)
		if err != nil {
			return nil, err
		}
		nextDelay = delay
		resp, err := c.wireClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := readCloudResponse(resp)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}
		if resp.StatusCode == http.StatusForbidden && bytes.Contains(body, []byte("RequestRateLimitExceeded")) {
			delay *= 2
		}
		return nil, fmt.Errorf("WireServer certificate request %s returned HTTP %d", path, resp.StatusCode)
	})
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
