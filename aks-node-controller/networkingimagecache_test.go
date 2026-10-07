package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchNetworkingImageCacheConfig(t *testing.T) {
	app := &App{
		networkingImageCacheFetcher: func(context.Context) ([]byte, error) {
			return []byte(`{
				"schemaVersion": 1,
				"agentPool": "pool1",
				"images": [
					{"reference":"mcr.microsoft.com/z:v1","workload":"cilium"},
					{"reference":"mcr.microsoft.com/a:v2","workload":"azure-cns"},
					{"reference":"mcr.microsoft.com/z:v1","workload":"duplicate"}
				]
			}`), nil
		},
	}
	config, err := app.fetchNetworkingImageCacheConfig(context.Background())
	require.NoError(t, err)
	require.Len(t, config.Images, 2)
	assert.Equal(t, "mcr.microsoft.com/a:v2", config.Images[0].Reference)
	assert.Equal(t, "mcr.microsoft.com/z:v1", config.Images[1].Reference)
	assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(`{
				"schemaVersion": 1,
				"agentPool": "pool1",
				"images": [
					{"reference":"mcr.microsoft.com/z:v1","workload":"cilium"},
					{"reference":"mcr.microsoft.com/a:v2","workload":"azure-cns"},
					{"reference":"mcr.microsoft.com/z:v1","workload":"duplicate"}
				]
			}`))), config.Fingerprint)
}

func TestFetchNetworkingImageCacheConfigRejectsFetchOnlyShape(t *testing.T) {
	app := &App{
		networkingImageCacheFetcher: func(context.Context) ([]byte, error) {
			return []byte(`{"schemaVersion":1,"agentPool":"pool1","images":[{"reference":"not-a-full-reference"}]}`), nil
		},
	}
	_, err := app.fetchNetworkingImageCacheConfig(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry, repository, and tag or digest")
}

func TestFetchNetworkingImageCacheConfigRejectsIncompletePayloads(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "missing pool",
			payload: `{"schemaVersion":1,"images":[{"reference":"mcr.microsoft.com/a:v1"}]}`,
			want:    "no agentPool",
		},
		{
			name:    "empty images",
			payload: `{"schemaVersion":1,"agentPool":"pool1","images":[]}`,
			want:    "no images",
		},
		{
			name:    "trailing JSON",
			payload: `{"schemaVersion":1,"agentPool":"pool1","images":[{"reference":"mcr.microsoft.com/a:v1"}]} {}`,
			want:    "trailing JSON data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				networkingImageCacheFetcher: func(context.Context) ([]byte, error) {
					return []byte(tt.payload), nil
				},
			}
			_, err := app.fetchNetworkingImageCacheConfig(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestFetchNetworkingImageCacheConfigRetriesTransientFailures(t *testing.T) {
	var attempts atomic.Int32
	var waits atomic.Int32
	app := &App{
		networkingImageCacheFetcher: func(context.Context) ([]byte, error) {
			if attempts.Add(1) < networkingLPSFetchAttempts {
				return nil, errors.New("not ready")
			}
			return []byte(`{"schemaVersion":1,"agentPool":"pool1","images":[{"reference":"mcr.microsoft.com/a:v1"}]}`), nil
		},
		networkingImageRetryWait: func(context.Context) error {
			waits.Add(1)
			return nil
		},
	}
	config, err := app.fetchNetworkingImageCacheConfig(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, networkingLPSFetchAttempts, attempts.Load())
	assert.EqualValues(t, networkingLPSFetchAttempts-1, waits.Load())
	require.Len(t, config.Images, 1)
}

func TestFetchNetworkingImageCacheConfigStopsRetryOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		networkingImageCacheFetcher: func(context.Context) ([]byte, error) {
			return nil, errors.New("not ready")
		},
		networkingImageRetryWait: func(context.Context) error {
			cancel()
			return ctx.Err()
		},
	}
	_, err := app.fetchNetworkingImageCacheConfig(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestFetchNetworkingImageCacheConfigSelectsDeterministicDuplicateMetadata(t *testing.T) {
	app := &App{
		networkingImageCacheFetcher: func(context.Context) ([]byte, error) {
			return []byte(`{
				"schemaVersion":1,
				"agentPool":"pool1",
				"images":[
					{"reference":"mcr.microsoft.com/a:v1","workload":"z-workload","container":"a"},
					{"reference":"mcr.microsoft.com/a:v1","workload":"a-workload","container":"z"},
					{"reference":"mcr.microsoft.com/a:v1","workload":"a-workload","container":"a"}
				]
			}`), nil
		},
	}
	config, err := app.fetchNetworkingImageCacheConfig(context.Background())
	require.NoError(t, err)
	require.Len(t, config.Images, 1)
	assert.Equal(t, "a-workload", config.Images[0].Workload)
	assert.Equal(t, "a", config.Images[0].Container)
}

func TestPullNetworkingImagesInvokesPullImage(t *testing.T) {
	var mu sync.Mutex
	var references []string
	app := &App{
		networkingImagePuller: func(_ context.Context, reference string) (networkingImagePullOutput, error) {
			mu.Lock()
			defer mu.Unlock()
			references = append(references, reference)
			return networkingImagePullOutput{
				Reference:   reference,
				Digest:      "sha256:test",
				Snapshotter: "overlayfs",
				Outcome:     "pulled",
			}, nil
		},
	}
	images := []networkingImageEntry{
		{Reference: "mcr.microsoft.com/a:v1"},
		{Reference: "mcr.microsoft.com/b:v2"},
	}
	results, err := app.pullNetworkingImages(context.Background(), images)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.ElementsMatch(t, []string{"mcr.microsoft.com/a:v1", "mcr.microsoft.com/b:v2"}, references)
	assert.Equal(t, "sha256:test", results[0].Digest)
}

func TestPullNetworkingImageValidatesImageFetcherOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image-fetcher")
	require.NoError(t, os.WriteFile(path, []byte(`#!/bin/sh
printf '%s' '{"reference":"mcr.microsoft.com/wrong:v1","digest":"sha256:test","snapshotter":"overlayfs","outcome":"pulled"}'
`), 0o700))

	app := &App{}
	_, err := app.pullNetworkingImage(context.Background(), path, "mcr.microsoft.com/a:v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `returned reference "mcr.microsoft.com/wrong:v1"`)
}

func TestRunPrepullNetworkingImagesCommandIsFailOpenAndWritesStatus(t *testing.T) {
	dir := t.TempDir()
	nodeConfigPath := filepath.Join(dir, "node-config.json")
	require.NoError(t, os.WriteFile(nodeConfigPath, []byte(`{
		"version":"v1",
		"kubeletConfig":{"kubeletNodeLabels":{"kubernetes.azure.com/agentpool":"pool1"}}
	}`), 0o600))
	statusPath := filepath.Join(dir, "status.json")

	app := &App{
		networkingImagePuller: func(context.Context, string) (networkingImagePullOutput, error) {
			return networkingImagePullOutput{}, assert.AnError
		},
		nodeConfigPath:                 nodeConfigPath,
		networkingImageCacheStatusPath: statusPath,
		networkingImageCacheFetcher: func(context.Context) ([]byte, error) {
			return []byte(`{"schemaVersion":1,"agentPool":"pool1","images":[{"reference":"mcr.microsoft.com/a:v1"}]}`), nil
		},
	}
	require.NoError(t, app.runPrepullNetworkingImagesCommand(context.Background()))

	raw, err := os.ReadFile(statusPath)
	require.NoError(t, err)
	var status networkingImageCacheStatus
	require.NoError(t, json.Unmarshal(raw, &status))
	assert.Equal(t, "failed", status.Outcome)
	assert.NotEmpty(t, status.ComponentFingerprint)
	require.Len(t, status.Images, 1)
	assert.Equal(t, "failed", status.Images[0].Outcome)
}

func TestRunPrepullNetworkingImagesCommandReportsPartialSuccess(t *testing.T) {
	dir := t.TempDir()
	nodeConfigPath := filepath.Join(dir, "node-config.json")
	require.NoError(t, os.WriteFile(nodeConfigPath, []byte(`{
		"version":"v1",
		"kubeletConfig":{"kubeletNodeLabels":{"kubernetes.azure.com/agentpool":"pool1"}}
	}`), 0o600))
	statusPath := filepath.Join(dir, "status.json")

	app := &App{
		networkingImagePuller: func(_ context.Context, reference string) (networkingImagePullOutput, error) {
			if reference == "mcr.microsoft.com/b:v1" {
				return networkingImagePullOutput{}, assert.AnError
			}
			return networkingImagePullOutput{
				Reference:   reference,
				Digest:      "sha256:test",
				Snapshotter: "overlayfs",
				Outcome:     "pulled",
			}, nil
		},
		nodeConfigPath:                 nodeConfigPath,
		networkingImageCacheStatusPath: statusPath,
		networkingImageCacheFetcher: func(context.Context) ([]byte, error) {
			return []byte(`{
				"schemaVersion":1,
				"agentPool":"pool1",
				"images":[
					{"reference":"mcr.microsoft.com/a:v1"},
					{"reference":"mcr.microsoft.com/b:v1"}
				]
			}`), nil
		},
	}
	require.NoError(t, app.runPrepullNetworkingImagesCommand(context.Background()))

	raw, err := os.ReadFile(statusPath)
	require.NoError(t, err)
	var status networkingImageCacheStatus
	require.NoError(t, json.Unmarshal(raw, &status))
	assert.Equal(t, "partial", status.Outcome)
	require.Len(t, status.Images, 2)
	assert.Equal(t, "pulled", status.Images[0].Outcome)
	assert.Equal(t, "failed", status.Images[1].Outcome)
}
