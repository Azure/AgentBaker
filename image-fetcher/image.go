package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultSocket = "/run/containerd/containerd.sock"
	defaultNS     = "k8s.io"
	// images with compressed content size below this threshold are
	// unpacked after fetch, effectively turning the operation into a
	// full pull (~200 MiB compressed ≈ ~400 MiB unpacked).
	pullSizeThreshold = 200 * 1024 * 1024 // 200 MiB
)

// fetchImage can delegate registry fetching to the host Transfer service.
// Otherwise, it uses client.Fetch(), which:
//   - Downloads all blobs (manifest, config, layers) into the content store
//   - Creates an image record in the metadata database
//   - Does NOT unpack layers into the snapshotter
//
// If the total image content size is below pullSizeThreshold (200 MiB),
// ACL unpacks locally into overlayfs. Other images use client.Pull() for
// platform-aware snapshotter resolution.
func fetchImage(ctx context.Context, client *containerd.Client, ref string) error {
	fetchOnly := os.Getenv("IMAGE_FETCH_ONLY") == "true"

	fmt.Printf("Fetching %s ...\n", ref)

	platform := fmt.Sprintf("linux/%s", runtime.GOARCH)
	p, err := platforms.Parse(platform)
	if err != nil {
		return fmt.Errorf("parse platform %s: %w", platform, err)
	}
	platformMatcher := platforms.OnlyStrict(p)

	serverTransfer := os.Getenv(serverTransferEnv) == "true"
	var image containerd.Image
	if serverTransfer {
		image, err = transferImage(ctx, client, ref, p)
	} else {
		var imageMeta images.Image
		imageMeta, err = client.Fetch(ctx, ref, containerd.WithPlatformMatcher(platformMatcher))
		image = containerd.NewImageWithPlatform(client, imageMeta, platformMatcher)
	}
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	if err := validateImagePlatform(ctx, image, p); err != nil {
		return err
	}

	if fetchOnly {
		fmt.Printf("OK    %s -> %s (fetched)\n", image.Name(), image.Target().Digest)
		return nil
	}

	size, err := image.Size(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN  %s: could not determine image size, skipping unpack: %v\n", ref, err)
		fmt.Printf("OK    %s -> %s (fetched)\n", image.Name(), image.Target().Digest)
		return nil
	}

	if size < pullSizeThreshold {
		if serverTransfer {
			err = image.Unpack(ctx, "overlayfs")
		} else {
			// Other runtimes require Pull for platform-aware snapshotter resolution.
			_, err = client.Pull(ctx, ref,
				containerd.WithPlatformMatcher(platformMatcher),
				containerd.WithPullUnpack,
			)
		}
		if err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}
		fmt.Printf("OK    %s -> %s (pulled, %s)\n", image.Name(), image.Target().Digest, formatSize(size))
	} else {
		fmt.Printf("OK    %s -> %s (fetched, %s)\n", image.Name(), image.Target().Digest, formatSize(size))
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
