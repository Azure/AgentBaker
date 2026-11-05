package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	lpsv1 "github.com/Azure/agentbaker/aks-live-patching/pkg/gen/akslivepatching/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func imageCacheFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/image-cache-networking.json")
	require.NoError(t, err)
	return data
}

func imageCacheTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("KUBELET_NODE_LABELS", agentPoolLabelKey+"=pool")
	return &App{
		nodeConfigPath:       filepath.Join(t.TempDir(), "absent-config.json"),
		imageCacheStatusPath: filepath.Join(t.TempDir(), "image-cache-status.json"),
		imageCacheRetryWait:  func(context.Context) error { return nil },
	}
}

func readImageCacheStatus(t *testing.T, app *App) imageCacheStatus {
	t.Helper()
	data, err := os.ReadFile(app.imageCacheStatusPath)
	require.NoError(t, err)
	var result imageCacheStatus
	require.NoError(t, json.Unmarshal(data, &result))
	return result
}

func TestParseImageCacheProducerFixture(t *testing.T) {
	config, err := parseImageCacheConfig(imageCacheFixture(t), "networking")
	require.NoError(t, err)
	require.Equal(t, "registry.example/network:v1", config.Images[0].Reference)
}

func TestParseImageCacheRejectsInvalidContract(t *testing.T) {
	original := string(imageCacheFixture(t))
	for name, payload := range map[string]string{
		"old contract":      `{"schemaVersion":1,"agentPool":"pool","images":[]}`,
		"unknown field":     strings.Replace(original, `"schemaVersion": 1`, `"schemaVersion": 1, "unexpected": true`, 1),
		"wrong schema":      strings.Replace(original, `"schemaVersion": 1`, `"schemaVersion": 2`, 1),
		"wrong source":      strings.Replace(original, `"source": "networking"`, `"source": "other"`, 1),
		"missing images":    `{"schemaVersion":1,"source":"networking","agentPool":"pool"}`,
		"null images":       strings.Replace(original, `"images": [`, `"images": null, "oldImages": [`, 1),
		"trailing JSON":     original + `{}`,
		"whitespace":        strings.Replace(original, "registry.example/network:v1", " registry.example/network:v1", 1),
		"revision mismatch": strings.Replace(original, "registry.example/network:v1", "registry.example/network:v2", 1),
		"bad source hash":   strings.Replace(original, `"sourceConfigHash": "ec35`, `"sourceConfigHash": "xyz`, 1),
		"wrong kind":        strings.Replace(original, "DaemonSet", "Deployment", 1),
		"empty container":   strings.Replace(original, `"container": "agent"`, `"container": ""`, 1),
		"oversized":         strings.Repeat(" ", imageCacheMaxBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseImageCacheConfig([]byte(payload), "networking")
			require.Error(t, err)
		})
	}
}

func TestImageCacheRetryClassification(t *testing.T) {
	fixture := imageCacheFixture(t)
	for _, code := range []codes.Code{
		codes.Unavailable, codes.DeadlineExceeded, codes.NotFound, codes.Unauthenticated,
		codes.PermissionDenied, codes.InvalidArgument, codes.FailedPrecondition, codes.ResourceExhausted, codes.Internal,
	} {
		t.Run(code.String(), func(t *testing.T) {
			app := imageCacheTestApp(t)
			attempts := 0
			app.imageCacheFetcher = func(context.Context, string) ([]byte, error) {
				attempts++
				if attempts == 1 {
					return nil, status.Error(code, "test failure")
				}
				return fixture, nil
			}
			_, err := app.fetchImageCacheConfig(context.Background(), "networking")
			if code == codes.Unavailable || code == codes.DeadlineExceeded {
				require.NoError(t, err)
				require.Equal(t, 2, attempts)
			} else {
				require.Error(t, err)
				require.Equal(t, 1, attempts)
			}
		})
	}
	app := imageCacheTestApp(t)
	attempts := 0
	app.imageCacheFetcher = func(context.Context, string) ([]byte, error) {
		attempts++
		return nil, status.Error(codes.Unavailable, "not ready")
	}
	_, err := app.fetchImageCacheConfig(context.Background(), "networking")
	require.Error(t, err)
	require.Equal(t, imageCacheFetchAttempts, attempts)
}

func TestImageCacheGRPCContractAndAuthentication(t *testing.T) {
	fixture := imageCacheFixture(t)
	server := &mockLPSServer{resp: &lpsv1.GetComponentConfigResponse{
		ComponentName: "imageCache/networking", Config: string(fixture),
	}}
	app := newGRPCTestApp(t, server).App
	_, err := app.fetchImageCacheConfig(context.Background(), "networking")
	require.NoError(t, err)
	require.Equal(t, "imageCache/networking", server.gotComponent)
	require.Equal(t, []string{"attested-doc-token"}, server.gotToken)
	require.Equal(t, []string{testVMResourceID}, server.gotResourceID)
}

func TestImageCacheGRPCRejectsWrongComponent(t *testing.T) {
	server := &mockLPSServer{resp: &lpsv1.GetComponentConfigResponse{
		ComponentName: ancComponentName, Config: string(imageCacheFixture(t)),
	}}
	app := newGRPCTestApp(t, server).App
	_, err := app.fetchImageCacheConfig(context.Background(), "networking")
	require.ErrorContains(t, err, "returned component")
}

func TestImageCacheFailOpenStatus(t *testing.T) {
	fixture := imageCacheFixture(t)
	for _, test := range []struct {
		name    string
		payload []byte
		code    codes.Code
		pool    string
		outcome string
	}{
		{name: "ready", payload: fixture, pool: "pool", outcome: "complete"},
		{name: "wrong pool", payload: fixture, pool: "other", outcome: "failed"},
		{name: "removed", code: codes.NotFound, pool: "pool", outcome: "disabled"},
		{name: "denied", code: codes.PermissionDenied, pool: "pool", outcome: "failed"},
		{name: "invalid", payload: []byte("{}"), pool: "pool", outcome: "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := imageCacheTestApp(t)
			t.Setenv("KUBELET_NODE_LABELS", agentPoolLabelKey+"="+test.pool)
			app.imageCacheFetcher = func(context.Context, string) ([]byte, error) {
				if test.code != codes.OK {
					return nil, status.Error(test.code, "test error")
				}
				return test.payload, nil
			}
			app.imageCachePuller = func(_ context.Context, ref string) (imageCachePullOutput, error) {
				return imageCachePullOutput{Reference: ref, Digest: "sha256:example", Snapshotter: "overlayfs", Outcome: "cache-hit"}, nil
			}
			require.NoError(t, app.runPrepullImagesCommand(context.Background(), "networking"))
			result := readImageCacheStatus(t, app)
			require.Equal(t, test.outcome, result.Outcome)
			require.Equal(t, 2, result.SchemaVersion)
			require.False(t, result.FinishedAt.Before(result.StartedAt))
			matches, err := filepath.Glob(filepath.Join(filepath.Dir(app.imageCacheStatusPath), ".image-cache-*.tmp"))
			require.NoError(t, err)
			require.Empty(t, matches)
		})
	}
}

func TestImageCacheEmptyInventoryIsNoMatch(t *testing.T) {
	var config imageCacheConfig
	require.NoError(t, json.Unmarshal(imageCacheFixture(t), &config))
	config.Images = []imageCacheEntry{}
	config.Revision = fmt.Sprintf("%x", sha256.Sum256([]byte("[]")))
	payload, err := json.Marshal(config)
	require.NoError(t, err)
	app := imageCacheTestApp(t)
	app.imageCacheFetcher = func(context.Context, string) ([]byte, error) { return payload, nil }
	app.imageCachePuller = func(context.Context, string) (imageCachePullOutput, error) {
		t.Error("empty inventory must not pull")
		return imageCachePullOutput{}, nil
	}
	require.NoError(t, app.runPrepullImagesCommand(context.Background(), "networking"))
	require.Equal(t, "no-match", readImageCacheStatus(t, app).Outcome)
}

func TestImageCacheCanceledPullsAreNotSuccesses(t *testing.T) {
	app := imageCacheTestApp(t)
	app.imageCachePuller = func(context.Context, string) (imageCachePullOutput, error) {
		t.Error("canceled work must not start")
		return imageCachePullOutput{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err := app.pullImageCacheImages(ctx, []imageCacheEntry{{Reference: "registry.example/network:v1"}})
	require.Error(t, err)
	require.Len(t, results, 1)
	require.NotEmpty(t, results[0].Error)
}

func TestImageCachePullConcurrencyAndPartialFailure(t *testing.T) {
	app := imageCacheTestApp(t)
	var active, maximum atomic.Int32
	app.imageCachePuller = func(_ context.Context, ref string) (imageCachePullOutput, error) {
		count := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); count > old && !maximum.CompareAndSwap(old, count); old = maximum.Load() {
		}
		if strings.HasSuffix(ref, ":1") {
			return imageCachePullOutput{}, errors.New("test pull error")
		}
		return imageCachePullOutput{Reference: ref, Digest: "sha256:example", Snapshotter: "overlayfs", Outcome: "pulled"}, nil
	}
	images := make([]imageCacheEntry, 12)
	for i := range images {
		images[i].Reference = fmt.Sprintf("registry.example/network:%d", i)
	}
	results, err := app.pullImageCacheImages(context.Background(), images)
	require.ErrorContains(t, err, "test pull error")
	require.LessOrEqual(t, maximum.Load(), int32(imageCacheConcurrency))
	require.Len(t, results, len(images))
	require.NotEmpty(t, results[1].Error)
	for index, result := range results {
		if index != 1 {
			require.Empty(t, result.Error)
			require.Equal(t, images[index].Reference, result.Reference)
		}
	}
}

func TestImageCacheRejectsUnprovenPullSuccess(t *testing.T) {
	app := imageCacheTestApp(t)
	app.imageCachePuller = func(context.Context, string) (imageCachePullOutput, error) {
		return imageCachePullOutput{Reference: "wrong-image", Outcome: "cache-hit"}, nil
	}
	_, err := app.pullImageCacheImage(context.Background(), "registry.example/network:v1")
	require.Error(t, err)
}
