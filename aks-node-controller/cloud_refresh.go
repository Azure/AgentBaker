package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode"
)

const cloudRefreshTimeout = 15 * time.Minute

func (a *App) runCloudCertificateRefresh(ctx context.Context, location string) error {
	if strings.TrimSpace(location) == "" {
		return errors.New("cloud certificate refresh requires a location")
	}
	var err error
	if a.refreshCloudCertificates != nil {
		err = a.refreshCloudCertificates(ctx, location)
	} else {
		initializer := newCloudInitializer(a)
		defer initializer.close()
		err = initializer.refreshCertificates(ctx, location)
	}
	if err != nil {
		slog.Error("Azure cloud certificate refresh failed", "error", err)
	}
	return err
}

func (c *cloudInitializer) refreshCertificates(ctx context.Context, location string) error {
	refreshCtx, cancel := context.WithTimeout(ctx, cloudRefreshTimeout)
	defer cancel()
	return c.withEvent("CloudCertificateRefresh", func() error {
		normalizedLocation, err := normalizeCloudLocation(location)
		if err != nil {
			return err
		}
		platform, err := c.platform()
		if err != nil {
			return err
		}
		_, err = c.refresh(refreshCtx, platform, normalizedLocation)
		return err
	})
}

func normalizeCloudLocation(location string) (string, error) {
	location = strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, location))
	if strings.IndexFunc(location, func(r rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789-", r)
	}) != -1 {
		return "", errors.New("cloud location contains unsupported characters")
	}
	if location == "" {
		slog.Warn("Azure cloud location is empty, using rcv1p certificate endpoints")
	}
	return location, nil
}
