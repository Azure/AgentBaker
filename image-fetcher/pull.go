package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pelletier/go-toml/v2"
)

type pullImageResult struct {
	Reference   string `json:"reference"`
	Digest      string `json:"digest"`
	Snapshotter string `json:"snapshotter"`
	Outcome     string `json:"outcome"`
}

type imagePullClient interface {
	Fetch(context.Context, string, ...containerd.RemoteOpt) (images.Image, error)
	Pull(context.Context, string, ...containerd.RemoteOpt) (containerd.Image, error)
}

func pullAndUnpackImage(ctx context.Context, client *containerd.Client, ref string) (pullImageResult, error) {
	data, err := effectiveContainerdConfig(ctx)
	if err != nil {
		return pullImageResult{}, err
	}
	snapshotter, err := snapshotterFromConfig(data)
	if err != nil {
		return pullImageResult{}, err
	}
	platform := platforms.Normalize(platforms.DefaultSpec())
	platform.OS, platform.Architecture = "linux", runtime.GOARCH
	matcher := platforms.OnlyStrict(platform)
	image, getErr := client.GetImage(ctx, ref)
	outcome := "cache-hit"
	if getErr == nil {
		image = containerd.NewImageWithPlatform(client, image.Metadata(), matcher)
		if err = validateImagePlatform(ctx, image, platform); err == nil {
			err = checkImageContent(ctx, client.ContentStore(), image.Target(), matcher)
		}
	}
	if getErr != nil || err != nil {
		resolver, resolverErr := registryResolverFromConfig(ctx, data)
		if resolverErr != nil {
			return pullImageResult{}, resolverErr
		}
		image, err = fetchAndPullImage(ctx, client, ref, matcher, snapshotter, resolver)
		if err != nil {
			return pullImageResult{}, err
		}
		outcome = "pulled"
	}
	unpacked, err := image.IsUnpacked(ctx, snapshotter)
	if err != nil {
		return pullImageResult{}, fmt.Errorf("checking CRI snapshots: %w", err)
	}
	if !unpacked {
		if err = image.Unpack(ctx, snapshotter); err != nil {
			return pullImageResult{}, fmt.Errorf("unpacking cached image: %w", err)
		}
		outcome = "unpacked"
	}
	if err = validateImagePlatform(ctx, image, platform); err != nil {
		return pullImageResult{}, err
	}
	if err = checkImageContent(ctx, client.ContentStore(), image.Target(), matcher); err != nil {
		return pullImageResult{}, fmt.Errorf("validating complete image content: %w", err)
	}
	unpacked, err = image.IsUnpacked(ctx, snapshotter)
	if err != nil {
		return pullImageResult{}, fmt.Errorf("checking final CRI snapshotter %q: %w", snapshotter, err)
	}
	if !unpacked {
		return pullImageResult{}, fmt.Errorf("image is not ready in CRI snapshotter %q", snapshotter)
	}
	return pullImageResult{Reference: ref, Digest: image.Target().Digest.String(), Snapshotter: snapshotter, Outcome: outcome}, nil
}

func fetchAndPullImage(
	ctx context.Context, client imagePullClient, ref string, matcher platforms.MatchComparer, snapshotter string, resolver remotes.Resolver,
) (containerd.Image, error) {
	// Pull's unpacker can skip compressed blobs when their snapshots already exist.
	if _, err := client.Fetch(ctx, ref, containerd.WithPlatformMatcher(matcher), containerd.WithResolver(resolver)); err != nil {
		return nil, fmt.Errorf("fetching complete image content: %w", err)
	}
	image, err := client.Pull(ctx, ref,
		containerd.WithResolver(resolver),
		containerd.WithPlatformMatcher(matcher),
		containerd.WithPullSnapshotter(snapshotter),
		containerd.WithPullUnpack,
	)
	if err != nil {
		return nil, fmt.Errorf("pulling and unpacking image: %w", err)
	}
	return image, nil
}

func checkImageContent(ctx context.Context, provider content.Provider, target ocispec.Descriptor, matcher platforms.MatchComparer) error {
	available, _, _, missing, err := images.Check(ctx, provider, target, matcher)
	if err != nil {
		return err
	}
	if !available || len(missing) != 0 {
		return fmt.Errorf("image content is incomplete: available=%t missing=%d", available, len(missing))
	}
	return nil
}

func effectiveContainerdConfig(ctx context.Context) ([]byte, error) {
	// Dump resolves imports and defaults; reading only config.toml can choose
	// the wrong snapshotter when CRI configuration lives in an imported file.
	// #nosec G702 -- All command arguments are constants; context carries cancellation only.
	data, err := exec.CommandContext(ctx, "containerd", "config", "dump").Output()
	if err != nil {
		return nil, fmt.Errorf("reading effective containerd configuration: %w", err)
	}
	return data, nil
}

func snapshotterFromConfig(data []byte) (string, error) {
	type runtimeConfig struct {
		Snapshotter string `toml:"snapshotter"`
	}
	type pluginConfig struct {
		Snapshotter      string                   `toml:"snapshotter"`
		RuntimePlatforms map[string]runtimeConfig `toml:"runtime_platforms"`
		Containerd       struct {
			Snapshotter        string                   `toml:"snapshotter"`
			DefaultRuntimeName string                   `toml:"default_runtime_name"`
			Runtimes           map[string]runtimeConfig `toml:"runtimes"`
		} `toml:"containerd"`
	}
	var config struct {
		Plugins map[string]pluginConfig `toml:"plugins"`
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("parsing effective containerd configuration: %w", err)
	}
	var name string
	if imagePlugin, ok := config.Plugins["io.containerd.cri.v1.images"]; ok {
		name = imagePlugin.Snapshotter
		runtimeName := config.Plugins["io.containerd.cri.v1.runtime"].Containerd.DefaultRuntimeName
		if runtime := imagePlugin.RuntimePlatforms[runtimeName]; runtime.Snapshotter != "" {
			name = runtime.Snapshotter
		}
	} else if plugin, ok := config.Plugins["io.containerd.grpc.v1.cri"]; ok {
		name = plugin.Containerd.Snapshotter
		if runtime := plugin.Containerd.Runtimes[plugin.Containerd.DefaultRuntimeName]; runtime.Snapshotter != "" {
			name = runtime.Snapshotter
		}
	}
	if name == "" || strings.TrimSpace(name) != name {
		return "", fmt.Errorf("effective CRI snapshotter is missing or invalid")
	}
	return name, nil
}
