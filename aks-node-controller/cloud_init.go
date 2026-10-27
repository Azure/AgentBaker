package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type cloudInitConfig struct {
	Location          string
	RepoDepotEndpoint string
	Timeout           time.Duration
}

func (a *App) initializeAKSCloud(ctx context.Context, env map[string]string) error {
	timeout, err := time.ParseDuration(env["CSE_TIMEOUT"] + "s")
	if err != nil || timeout <= 0 {
		return fmt.Errorf("invalid cloud initialization timeout %q", env["CSE_TIMEOUT"])
	}
	config := cloudInitConfig{
		Location:          env["LOCATION"],
		RepoDepotEndpoint: env["REPO_DEPOT_ENDPOINT"],
		Timeout:           timeout,
	}
	if a.initializeCloud != nil {
		return a.initializeCloud(ctx, config)
	}
	initializer := newCloudInitializer(a)
	defer initializer.close()
	return initializer.initialize(ctx, config)
}

func (c *cloudInitializer) initialize(ctx context.Context, config cloudInitConfig) error {
	initCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	return c.withEvent("CloudInitialization", func() error {
		location, err := normalizeCloudLocation(config.Location)
		if err != nil {
			return err
		}
		platform, err := c.platform()
		if err != nil {
			return err
		}
		eligible, err := c.refresh(initCtx, platform, location)
		if err != nil {
			return err
		}
		if eligible {
			if err := c.installRefreshSchedule(initCtx, platform, location); err != nil {
				return fmt.Errorf("schedule Azure cloud certificate refresh: %w", err)
			}
		}
		repositoryErr := c.withEvent("CloudRepositoryInitialization", func() error {
			return c.initializeRepositories(initCtx, platform, config.RepoDepotEndpoint)
		})
		if errors.Is(repositoryErr, errCloudRepositoryCacheRefresh) {
			if contextErr := initCtx.Err(); contextErr != nil {
				return contextErr
			}
			// The shell reports cache-refresh failures without propagating them to CSE.
			slog.Warn("RepoDepot package cache refresh failed, continuing cloud initialization", "error", repositoryErr)
			return nil
		}
		return repositoryErr
	})
}
