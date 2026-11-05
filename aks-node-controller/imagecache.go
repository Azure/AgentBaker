package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	imageCacheMaxBytes       = 1 << 20
	imageCacheMaxImages      = 128
	imageCacheFetchTimeout   = 10 * time.Second
	imageCachePullTimeout    = 2 * time.Minute
	imageCacheOverallTimeout = 5 * time.Minute
	imageCacheFetchAttempts  = 3
	imageCacheConcurrency    = 4
	defaultImageFetcherPath  = "/opt/azure/containers/image-fetcher"
	defaultImageCacheStatus  = "/opt/azure/containers/image-cache-status.json"
	agentPoolLabelKey        = "kubernetes.azure.com/agentpool"
)

type imageCacheWorkload struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type imageCacheEntry struct {
	Reference string             `json:"reference"`
	Workload  imageCacheWorkload `json:"workload"`
	Container string             `json:"container"`
}

type imageCacheConfig struct {
	SchemaVersion    int               `json:"schemaVersion"`
	Source           string            `json:"source"`
	AgentPool        string            `json:"agentPool"`
	Revision         string            `json:"revision"`
	SourceConfigHash string            `json:"sourceConfigHash"`
	Images           []imageCacheEntry `json:"images"`
}

type imageCachePullOutput struct {
	Reference   string `json:"reference"`
	Digest      string `json:"digest"`
	Snapshotter string `json:"snapshotter"`
	Outcome     string `json:"outcome"`
}

type imageCacheImageResult struct {
	imageCachePullOutput
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

type imageCacheStatus struct {
	SchemaVersion    int                     `json:"schemaVersion"`
	Source           string                  `json:"source"`
	AgentPool        string                  `json:"agentPool,omitempty"`
	Revision         string                  `json:"revision,omitempty"`
	SourceConfigHash string                  `json:"sourceConfigHash,omitempty"`
	Outcome          string                  `json:"outcome"`
	StartedAt        time.Time               `json:"startedAt"`
	FinishedAt       time.Time               `json:"finishedAt"`
	Images           []imageCacheImageResult `json:"images"`
	Error            string                  `json:"error,omitempty"`
}

func (a *App) runPrepullImagesCommand(ctx context.Context, source string) (err error) {
	result := imageCacheStatus{
		SchemaVersion: 2, Source: source, Outcome: "failed",
		StartedAt: time.Now().UTC(), Images: []imageCacheImageResult{},
	}
	defer func() {
		result.FinishedAt = time.Now().UTC()
		if err != nil {
			result.Error = err.Error()
			slog.Warn("image pre-pull failed open", "source", source, "error", err)
		}
		if writeErr := a.writeImageCacheStatus(result); writeErr != nil {
			slog.Warn("writing image-cache status failed", "error", writeErr)
		}
		if a.eventLogger != nil {
			level := helpers.EventLevelInformational
			if err != nil {
				level = helpers.EventLevelError
			}
			a.eventLogger.LogEvent("PrepullImages",
				fmt.Sprintf("source=%s outcome=%s images=%d", source, result.Outcome, len(result.Images)),
				level, result.StartedAt, result.FinishedAt)
		}
		err = nil
	}()
	if !validImageCacheLabel(source) {
		return errors.New("invalid image-cache source name")
	}
	ctx, cancel := context.WithTimeout(ctx, imageCacheOverallTimeout)
	defer cancel()
	config, err := a.fetchImageCacheConfig(ctx, source)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			result.Outcome = "disabled"
			return nil
		}
		return err
	}
	result.AgentPool, result.Revision, result.SourceConfigHash = config.AgentPool, config.Revision, config.SourceConfigHash
	pool, err := a.imageCacheAgentPool()
	if err != nil {
		return err
	}
	if pool == "" || !strings.EqualFold(config.AgentPool, pool) {
		return fmt.Errorf("LPS agent pool %q does not match local pool %q", config.AgentPool, pool)
	}
	if len(config.Images) == 0 {
		result.Outcome = "no-match"
		return nil
	}
	result.Images, err = a.pullImageCacheImages(ctx, config.Images)
	result.Outcome = "complete"
	if err != nil {
		result.Outcome = "failed"
		for _, image := range result.Images {
			if image.Error == "" {
				result.Outcome = "partial"
				break
			}
		}
	}
	return err
}

func (a *App) fetchImageCacheConfig(ctx context.Context, source string) (*imageCacheConfig, error) {
	var data []byte
	var err error
	for attempt := 0; attempt < imageCacheFetchAttempts; attempt++ {
		if a.imageCacheFetcher != nil {
			data, err = a.imageCacheFetcher(ctx, source)
		} else {
			data, err = a.fetchLPSComponentOverGRPC(ctx, "imageCache/"+source,
				imageCacheFetchTimeout, grpc.MaxCallRecvMsgSize(imageCacheMaxBytes))
		}
		if err == nil {
			break
		}
		if ctx.Err() != nil || !retryImageCacheFetch(err) || attempt == imageCacheFetchAttempts-1 {
			return nil, err
		}
		err = nil
		if a.imageCacheRetryWait != nil {
			err = a.imageCacheRetryWait(ctx)
		} else {
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				err = ctx.Err()
			case <-timer.C:
			}
			timer.Stop()
		}
		if err != nil {
			return nil, err
		}
	}
	return parseImageCacheConfig(data, source)
}

func retryImageCacheFetch(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) {
		return true
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return true
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

func parseImageCacheConfig(data []byte, source string) (*imageCacheConfig, error) {
	if len(data) > imageCacheMaxBytes {
		return nil, errors.New("image-cache payload exceeds size limit")
	}
	var config imageCacheConfig
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("parsing image-cache config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("image-cache config has trailing JSON")
	}
	if config.SchemaVersion != 1 || config.Source != source || !validImageCacheLabel(source) ||
		!validImageCacheLabel(config.AgentPool) || config.Images == nil {
		return nil, errors.New("invalid image-cache schema, source, agent pool or image array")
	}
	if hash, err := hex.DecodeString(config.SourceConfigHash); err != nil || len(hash) != sha256.Size {
		return nil, errors.New("invalid image-cache source-config hash")
	}
	if len(config.Images) > imageCacheMaxImages {
		return nil, errors.New("image-cache image count exceeds limit")
	}
	references := make([]string, 0, len(config.Images))
	seen := map[string]bool{}
	for _, image := range config.Images {
		if image.Reference == "" || len(image.Reference) > 512 ||
			strings.IndexFunc(image.Reference, unicode.IsSpace) >= 0 {
			return nil, errors.New("invalid exact image reference")
		}
		lastSlash := strings.LastIndex(image.Reference, "/")
		if lastSlash <= 0 || (!strings.Contains(image.Reference[lastSlash+1:], ":") &&
			!strings.Contains(image.Reference, "@")) {
			return nil, errors.New("image reference requires registry, repository and tag or digest")
		}
		if seen[image.Reference] {
			return nil, errors.New("image-cache contains duplicate exact references")
		}
		seen[image.Reference] = true
		if image.Workload.Kind != "DaemonSet" ||
			!validImageCacheLabel(image.Workload.Namespace) ||
			!validImageCacheName(image.Workload.Name, 253) ||
			!validImageCacheLabel(image.Container) {
			return nil, errors.New("invalid image-cache workload metadata")
		}
		references = append(references, image.Reference)
	}
	sort.Strings(references)
	canonical, err := json.Marshal(references)
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(canonical)) != config.Revision {
		return nil, errors.New("image-cache revision does not match exact references")
	}
	return &config, nil
}

func validImageCacheName(value string, limit int) bool {
	if value == "" || len(value) > limit {
		return false
	}
	for i, r := range value {
		alphanumeric := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if !alphanumeric && (r != '-' && r != '.' || i == 0 || i == len(value)-1) {
			return false
		}
	}
	for _, part := range strings.Split(value, ".") {
		if part == "" || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
	}
	return true
}

func validImageCacheLabel(value string) bool {
	return !strings.Contains(value, ".") && validImageCacheName(value, 63)
}

func (a *App) pullImageCacheImages(ctx context.Context, images []imageCacheEntry) ([]imageCacheImageResult, error) {
	results := make([]imageCacheImageResult, len(images))
	for i, image := range images {
		results[i] = imageCacheImageResult{
			imageCachePullOutput: imageCachePullOutput{Reference: image.Reference, Outcome: "failed"},
			Error:                "not attempted",
		}
	}
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(imageCacheConcurrency, len(images)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				start := time.Now()
				pullCtx, cancel := context.WithTimeout(ctx, imageCachePullTimeout)
				output, err := a.pullImageCacheImage(pullCtx, images[index].Reference)
				cancel()
				if err != nil {
					results[index].Error = err.Error()
				} else {
					results[index].imageCachePullOutput = output
					results[index].Error = ""
				}
				results[index].Duration = time.Since(start)
			}
		}()
	}
dispatch:
	for index := range images {
		if ctx.Err() != nil {
			break dispatch
		}
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- index:
		}
	}
	close(jobs)
	workers.Wait()
	var failures []error
	for _, result := range results {
		if result.Error != "" {
			failures = append(failures, fmt.Errorf("%s: %s", result.Reference, result.Error))
		}
	}
	return results, errors.Join(failures...)
}

func (a *App) pullImageCacheImage(ctx context.Context, reference string) (imageCachePullOutput, error) {
	var result imageCachePullOutput
	var err error
	if a.imageCachePuller != nil {
		result, err = a.imageCachePuller(ctx, reference)
	} else {
		path := a.imageFetcherPath
		if path == "" {
			path = defaultImageFetcherPath
		}
		data, runErr := exec.CommandContext(ctx, path, "pull-image", "--image", reference).Output()
		if runErr != nil {
			var exitError *exec.ExitError
			if errors.As(runErr, &exitError) {
				return result, fmt.Errorf("image-fetcher failed: %w: %s", runErr, strings.TrimSpace(string(exitError.Stderr)))
			}
			return result, fmt.Errorf("image-fetcher failed: %w", runErr)
		}
		err = json.Unmarshal(data, &result)
	}
	if err != nil {
		return result, err
	}
	if result.Reference != reference || result.Digest == "" || result.Snapshotter == "" {
		return result, errors.New("image-fetcher did not prove the requested image ready")
	}
	switch result.Outcome {
	case "cache-hit", "unpacked", "pulled":
		return result, nil
	default:
		return result, fmt.Errorf("unsupported image-fetcher outcome %q", result.Outcome)
	}
}

func (a *App) imageCacheAgentPool() (string, error) {
	data, err := os.ReadFile(a.getNodeConfigPath())
	if os.IsNotExist(err) {
		for _, label := range strings.Split(os.Getenv("KUBELET_NODE_LABELS"), ",") {
			if value, ok := strings.CutPrefix(label, agentPoolLabelKey+"="); ok {
				return value, nil
			}
		}
	}
	if err != nil {
		return "", err
	}
	config, err := nodeconfigutils.UnmarshalConfigurationV1(data)
	if config == nil {
		return "", fmt.Errorf("parsing node config: %w", err)
	}
	return config.GetKubeletConfig().GetKubeletNodeLabels()[agentPoolLabelKey], nil
}

func (a *App) writeImageCacheStatus(result imageCacheStatus) error {
	path := a.imageCacheStatusPath
	if path == "" {
		path = defaultImageCacheStatus
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".image-cache-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
