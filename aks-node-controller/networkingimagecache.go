package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
)

const (
	networkingImageCacheComponentName = "networkingImageCache"
	networkingImageCacheSchemaVersion = 1
	defaultImageFetcherPath           = "/opt/azure/containers/image-fetcher"
	defaultNetworkingStatusPath       = "/opt/azure/containers/networking-image-cache-status.json"
	agentPoolLabelKey                 = "kubernetes.azure.com/agentpool"
	maxNetworkingImages               = 128
	maxImageReferenceLength           = 512
	networkingLPSFetchTimeout         = 10 * time.Second
	networkingPullTimeout             = 2 * time.Minute
	networkingOverallTimeout          = 5 * time.Minute
	networkingPullConcurrency         = 4
	networkingLPSFetchAttempts        = 3
	networkingLPSRetryDelay           = 2 * time.Second
)

type networkingImageCacheConfig struct {
	SchemaVersion int                    `json:"schemaVersion"`
	AgentPool     string                 `json:"agentPool,omitempty"`
	Images        []networkingImageEntry `json:"images"`
	Fingerprint   string                 `json:"-"`
}

type networkingImageEntry struct {
	Reference string `json:"reference"`
	Workload  string `json:"workload,omitempty"`
	Container string `json:"container,omitempty"`
}

type networkingImageResult struct {
	Reference   string        `json:"reference"`
	Digest      string        `json:"digest,omitempty"`
	Snapshotter string        `json:"snapshotter,omitempty"`
	Outcome     string        `json:"outcome"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
}

type networkingImagePullOutput struct {
	Reference   string `json:"reference"`
	Digest      string `json:"digest"`
	Snapshotter string `json:"snapshotter"`
	Outcome     string `json:"outcome"`
}

type networkingImageCacheStatus struct {
	SchemaVersion        int                     `json:"schemaVersion"`
	ComponentFingerprint string                  `json:"componentFingerprint,omitempty"`
	AgentPool            string                  `json:"agentPool,omitempty"`
	Outcome              string                  `json:"outcome"`
	StartedAt            time.Time               `json:"startedAt"`
	FinishedAt           time.Time               `json:"finishedAt"`
	Images               []networkingImageResult `json:"images,omitempty"`
	Error                string                  `json:"error,omitempty"`
}

func (a *App) runPrepullNetworkingImagesCommand(ctx context.Context) (err error) {
	started := time.Now().UTC()
	status := networkingImageCacheStatus{StartedAt: started, Outcome: "failed"}
	defer func() {
		status.FinishedAt = time.Now().UTC()
		if err != nil {
			status.Error = err.Error()
			slog.Warn("networking image pre-pull completed with error (fail-open)", "error", err)
		} else {
			slog.Info("networking image pre-pull completed", "outcome", status.Outcome, "images", len(status.Images))
		}
		if writeErr := a.writeNetworkingImageCacheStatus(status); writeErr != nil {
			slog.Warn("failed to write networking image cache status", "error", writeErr)
		}
		if a.eventLogger != nil {
			level := helpers.EventLevelInformational
			if err != nil {
				level = helpers.EventLevelError
			}
			message := fmt.Sprintf("prepull-networking-images outcome=%s images=%d", status.Outcome, len(status.Images))
			if err != nil {
				message += " error=" + err.Error()
			}
			a.eventLogger.LogEvent("PrepullNetworkingImages", message, level, started, status.FinishedAt)
		}
		err = nil
	}()

	ctx, cancel := context.WithTimeout(ctx, networkingOverallTimeout)
	defer cancel()

	config, fetchErr := a.fetchNetworkingImageCacheConfig(ctx)
	if fetchErr != nil {
		status.Outcome = "skipped"
		return fetchErr
	}
	status.SchemaVersion = config.SchemaVersion
	status.ComponentFingerprint = config.Fingerprint
	status.AgentPool = config.AgentPool

	localPool, poolErr := a.agentPoolFromNodeConfig()
	if poolErr != nil {
		return poolErr
	}
	if localPool == "" {
		return errors.New("node config has no kubernetes.azure.com/agentpool label")
	}
	if !strings.EqualFold(config.AgentPool, localPool) {
		return fmt.Errorf("LPS agent pool %q does not match node config pool %q", config.AgentPool, localPool)
	}

	results, pullErr := a.pullNetworkingImages(ctx, config.Images)
	status.Images = results
	status.Outcome = "complete"
	if pullErr != nil {
		status.Outcome = "failed"
		for _, result := range results {
			if result.Outcome != "failed" {
				status.Outcome = "partial"
				break
			}
		}
		return pullErr
	}
	return nil
}

func (a *App) fetchNetworkingImageCacheConfig(ctx context.Context) (*networkingImageCacheConfig, error) {
	var (
		data []byte
		err  error
	)
	for attempt := 1; attempt <= networkingLPSFetchAttempts; attempt++ {
		if a.networkingImageCacheFetcher != nil {
			data, err = a.networkingImageCacheFetcher(ctx)
		} else {
			data, err = a.fetchLPSComponentOverGRPC(ctx, networkingImageCacheComponentName, networkingLPSFetchTimeout)
		}
		if err == nil {
			break
		}
		if attempt == networkingLPSFetchAttempts {
			return nil, fmt.Errorf("fetching %s from LPS after %d attempts: %w", networkingImageCacheComponentName, attempt, err)
		}
		if err := a.waitForNetworkingImageRetry(ctx); err != nil {
			return nil, err
		}
	}

	var config networkingImageCacheConfig
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("parsing networking image cache config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("parsing networking image cache config: trailing JSON data")
	}
	if config.SchemaVersion != networkingImageCacheSchemaVersion {
		return nil, fmt.Errorf("unsupported networking image cache schema version %d", config.SchemaVersion)
	}
	config.AgentPool = strings.TrimSpace(config.AgentPool)
	if config.AgentPool == "" {
		return nil, errors.New("networking image cache config has no agentPool")
	}
	if len(config.Images) == 0 {
		return nil, errors.New("networking image cache config has no images")
	}
	if len(config.Images) > maxNetworkingImages {
		return nil, fmt.Errorf("networking image count %d exceeds limit %d", len(config.Images), maxNetworkingImages)
	}

	deduped := make(map[string]networkingImageEntry, len(config.Images))
	for _, image := range config.Images {
		image.Reference = strings.TrimSpace(image.Reference)
		if err := validateNetworkingImageReference(image.Reference); err != nil {
			return nil, err
		}
		existing, exists := deduped[image.Reference]
		if !exists || image.Workload < existing.Workload ||
			(image.Workload == existing.Workload && image.Container < existing.Container) {
			deduped[image.Reference] = image
		}
	}
	config.Images = config.Images[:0]
	for _, image := range deduped {
		config.Images = append(config.Images, image)
	}
	sort.Slice(config.Images, func(i, j int) bool {
		return config.Images[i].Reference < config.Images[j].Reference
	})
	config.Fingerprint = fmt.Sprintf("%x", sha256.Sum256(data))
	return &config, nil
}

func (a *App) waitForNetworkingImageRetry(ctx context.Context) error {
	if a.networkingImageRetryWait != nil {
		return a.networkingImageRetryWait(ctx)
	}
	timer := time.NewTimer(networkingLPSRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validateNetworkingImageReference(reference string) error {
	if reference == "" {
		return errors.New("networking image reference must not be empty")
	}
	if len(reference) > maxImageReferenceLength {
		return fmt.Errorf("networking image reference exceeds %d bytes", maxImageReferenceLength)
	}
	if strings.IndexFunc(reference, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }) >= 0 {
		return fmt.Errorf("networking image reference %q contains whitespace", reference)
	}
	lastSlash := strings.LastIndex(reference, "/")
	if lastSlash <= 0 || (!strings.Contains(reference[lastSlash+1:], ":") && !strings.Contains(reference, "@")) {
		return fmt.Errorf("networking image reference %q must include registry, repository, and tag or digest", reference)
	}
	return nil
}

func (a *App) pullNetworkingImages(ctx context.Context, images []networkingImageEntry) ([]networkingImageResult, error) {
	if len(images) == 0 {
		return nil, nil
	}
	path := a.imageFetcherPath
	if path == "" {
		path = defaultImageFetcherPath
	}

	results := make([]networkingImageResult, len(images))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := networkingPullConcurrency
	if len(images) < workers {
		workers = len(images)
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				image := images[index]
				started := time.Now()
				result := networkingImageResult{Reference: image.Reference, Outcome: "failed"}
				imageCtx, cancel := context.WithTimeout(ctx, networkingPullTimeout)
				output, err := a.pullNetworkingImage(imageCtx, path, image.Reference)
				if err != nil {
					result.Error = err.Error()
				} else {
					result.Digest = output.Digest
					result.Snapshotter = output.Snapshotter
					result.Outcome = output.Outcome
				}
				cancel()
				result.Duration = time.Since(started)
				results[index] = result
			}
		}()
	}
	for index := range images {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return results, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()

	var errs []error
	for _, result := range results {
		if result.Error != "" {
			errs = append(errs, fmt.Errorf("%s: %s", result.Reference, result.Error))
		}
	}
	return results, errors.Join(errs...)
}

func (a *App) pullNetworkingImage(ctx context.Context, path, reference string) (networkingImagePullOutput, error) {
	if a.networkingImagePuller != nil {
		return a.networkingImagePuller(ctx, reference)
	}
	cmd := exec.CommandContext(ctx, path, "pull-image", "--image", reference)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return networkingImagePullOutput{}, fmt.Errorf("image-fetcher failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var result networkingImagePullOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return networkingImagePullOutput{}, fmt.Errorf("parsing image-fetcher result: %w", err)
	}
	if result.Reference != reference {
		return networkingImagePullOutput{}, fmt.Errorf("image-fetcher returned reference %q for %q", result.Reference, reference)
	}
	if result.Digest == "" || result.Snapshotter == "" {
		return networkingImagePullOutput{}, errors.New("image-fetcher result is missing digest or snapshotter")
	}
	switch result.Outcome {
	case "cache-hit", "unpacked", "pulled":
		return result, nil
	default:
		return networkingImagePullOutput{}, fmt.Errorf("image-fetcher returned unsupported outcome %q", result.Outcome)
	}
}

func (a *App) agentPoolFromNodeConfig() (string, error) {
	raw, err := os.ReadFile(a.getNodeConfigPath())
	if err != nil {
		return "", fmt.Errorf("reading node config for agent pool: %w", err)
	}
	config, parseErr := nodeconfigutils.UnmarshalConfigurationV1(raw)
	if config == nil {
		return "", fmt.Errorf("parsing node config for agent pool: %w", parseErr)
	}
	return strings.TrimSpace(config.GetKubeletConfig().GetKubeletNodeLabels()[agentPoolLabelKey]), nil
}

func (a *App) writeNetworkingImageCacheStatus(status networkingImageCacheStatus) error {
	path := a.networkingImageCacheStatusPath
	if path == "" {
		path = defaultNetworkingStatusPath
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".networking-image-cache-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
