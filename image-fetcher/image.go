package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultSocket      = "/run/containerd/containerd.sock"
	defaultNS          = "k8s.io"
	defaultSnapshotter = "overlayfs"
	containerdConfig   = "/etc/containerd/config.toml"
	// images with compressed content size below this threshold are
	// unpacked after fetch, effectively turning the operation into a
	// full pull (~200 MiB compressed ≈ ~400 MiB unpacked).
	pullSizeThreshold = 200 * 1024 * 1024 // 200 MiB
)

type pullImageResult struct {
	Reference   string `json:"reference"`
	Digest      string `json:"digest"`
	Snapshotter string `json:"snapshotter"`
	Outcome     string `json:"outcome"`
}

func pullAndUnpackImage(ctx context.Context, client *containerd.Client, ref string) (pullImageResult, error) {
	platform := fmt.Sprintf("linux/%s", runtime.GOARCH)
	p, err := platforms.Parse(platform)
	if err != nil {
		return pullImageResult{}, fmt.Errorf("parse platform %s: %w", platform, err)
	}
	platformMatcher := platforms.OnlyStrict(p)
	snapshotter := detectSnapshotter(containerdConfig)

	if imageMeta, err := client.GetImage(ctx, ref); err == nil {
		image := containerd.NewImageWithPlatform(client, imageMeta.Metadata(), platformMatcher)
		if err := validateImageContentAndPlatform(ctx, image, p); err == nil {
			unpacked, unpackErr := image.IsUnpacked(ctx, snapshotter)
			if unpackErr == nil && unpacked {
				return pullImageResult{
					Reference:   imageMeta.Name(),
					Digest:      imageMeta.Target().Digest.String(),
					Snapshotter: snapshotter,
					Outcome:     "cache-hit",
				}, nil
			}
			if unpackErr == nil {
				if err := image.Unpack(ctx, snapshotter); err == nil {
					return pullImageResult{
						Reference:   imageMeta.Name(),
						Digest:      imageMeta.Target().Digest.String(),
						Snapshotter: snapshotter,
						Outcome:     "unpacked",
					}, nil
				}
			}
		}
	}

	image, err := client.Pull(ctx, ref,
		containerd.WithPlatformMatcher(platformMatcher),
		containerd.WithPullSnapshotter(snapshotter),
		containerd.WithPullUnpack,
	)
	if err != nil {
		return pullImageResult{}, fmt.Errorf("pull and unpack failed: %w", err)
	}
	if err := validateImageContentAndPlatform(ctx, image, p); err != nil {
		return pullImageResult{}, err
	}
	unpacked, err := image.IsUnpacked(ctx, snapshotter)
	if err != nil {
		return pullImageResult{}, fmt.Errorf("checking unpacked image: %w", err)
	}
	if !unpacked {
		return pullImageResult{}, fmt.Errorf("image is not unpacked in snapshotter %s", snapshotter)
	}
	return pullImageResult{
		Reference:   image.Name(),
		Digest:      image.Target().Digest.String(),
		Snapshotter: snapshotter,
		Outcome:     "pulled",
	}, nil
}

func validateImageContentAndPlatform(ctx context.Context, image containerd.Image, expected ocispec.Platform) error {
	if err := validateImagePlatform(ctx, image, expected); err != nil {
		return err
	}
	if _, err := image.Size(ctx); err != nil {
		return fmt.Errorf("validating image content: %w", err)
	}
	return nil
}

func detectSnapshotter(path string) string {
	if snapshotter := strings.TrimSpace(os.Getenv("CONTAINERD_SNAPSHOTTER")); snapshotter != "" {
		return snapshotter
	}
	file, err := os.Open(path)
	if err != nil {
		return defaultSnapshotter
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line
			continue
		}
		isCRISection := strings.Contains(section, "io.containerd.cri.v1.images") ||
			(strings.Contains(section, "io.containerd.grpc.v1.cri") && strings.Contains(section, ".containerd"))
		if !isCRISection || !strings.HasPrefix(line, "snapshotter") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		if unquoted, err := strconv.Unquote(value); err == nil && unquoted != "" {
			return unquoted
		}
		if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			return value[1 : len(value)-1]
		}
	}
	return defaultSnapshotter
}

// fetchImage uses client.Fetch() which:
//   - Downloads all blobs (manifest, config, layers) into the content store
//   - Creates an image record in the metadata database
//   - Does NOT unpack layers into the snapshotter
//
// If the total image content size is below pullSizeThreshold (150 MiB),
// client.Pull() is called to additionally unpack the layers. Pull reuses
// already-fetched content from the store and handles snapshotter resolution
// internally (namespace label → platform default).
func fetchImage(ctx context.Context, client *containerd.Client, ref string) error {
	fetchOnly := os.Getenv("IMAGE_FETCH_ONLY") == "true"

	fmt.Printf("Fetching %s ...\n", ref)

	platform := fmt.Sprintf("linux/%s", runtime.GOARCH)
	p, err := platforms.Parse(platform)
	if err != nil {
		return fmt.Errorf("parse platform %s: %w", platform, err)
	}
	platformMatcher := platforms.OnlyStrict(p)

	imageMeta, err := client.Fetch(ctx, ref,
		containerd.WithPlatformMatcher(platformMatcher),
	)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	image := containerd.NewImageWithPlatform(client, imageMeta, platformMatcher)
	if err := validateImagePlatform(ctx, image, p); err != nil {
		return err
	}

	if fetchOnly {
		fmt.Printf("OK    %s -> %s (fetched)\n", imageMeta.Name, imageMeta.Target.Digest)
		return nil
	}

	size, err := image.Size(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN  %s: could not determine image size, skipping unpack: %v\n", ref, err)
		fmt.Printf("OK    %s -> %s (fetched)\n", imageMeta.Name, imageMeta.Target.Digest)
		return nil
	}

	if size < pullSizeThreshold {
		// We use pull here instead of use unpack because some runtimes (e.g. containerd-shim-runsc-v1),
		// require pull to trigger unpacking into the correct snapshotter based on the image's platform.
		if _, err := client.Pull(ctx, ref,
			containerd.WithPlatformMatcher(platformMatcher),
			containerd.WithPullUnpack,
		); err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}
		fmt.Printf("OK    %s -> %s (pulled, %s)\n", imageMeta.Name, imageMeta.Target.Digest, formatSize(size))
	} else {
		fmt.Printf("OK    %s -> %s (fetched, %s)\n", imageMeta.Name, imageMeta.Target.Digest, formatSize(size))
	}

	return nil
}

func validateImagePlatform(ctx context.Context, image containerd.Image, expected ocispec.Platform) error {
	spec, err := image.Spec(ctx)
	if err != nil {
		return fmt.Errorf("read image config: %w", err)
	}

	actual := spec.Platform
	if !platforms.OnlyStrict(expected).Match(actual) {
		return fmt.Errorf("image platform mismatch: selected manifest for %s, but image config is %s", platforms.Format(expected), platforms.Format(actual))
	}

	return nil
}

func formatSize(bytes int64) string {
	const (
		mib = 1024 * 1024
		gib = 1024 * 1024 * 1024
	)
	switch {
	case bytes >= gib:
		return fmt.Sprintf("%.2f GiB", float64(bytes)/float64(gib))
	case bytes >= mib:
		return fmt.Sprintf("%.2f MiB", float64(bytes)/float64(mib))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
