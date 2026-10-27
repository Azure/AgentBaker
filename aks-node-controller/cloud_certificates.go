package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
