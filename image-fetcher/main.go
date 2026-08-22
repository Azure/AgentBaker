package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultSocket = "/run/containerd/containerd.sock"
	defaultNS     = "k8s.io"
	// images with compressed content size below this threshold are
	// unpacked after fetch, effectively turning the operation into a
	// full pull (~150 MiB compressed ≈ ~300 MiB unpacked).
	pullSizeThreshold = 150 * 1024 * 1024 // 150 MiB
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <image-ref> [image-ref...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2\n", os.Args[0])
		os.Exit(1)
	}

	socket := os.Getenv("CONTAINERD_SOCKET")
	if socket == "" {
		socket = defaultSocket
	}
	ns := os.Getenv("CONTAINERD_NAMESPACE")
	if ns == "" {
		ns = defaultNS
	}

	client, err := containerd.New(socket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to containerd at %s: %v\n", socket, err)
		os.Exit(1)
	}
	defer client.Close()

	ctx := namespaces.WithNamespace(context.Background(), ns)

	fetch := fetchImage
	if os.Getenv(dmverityCacheEnv) == "true" {
		if os.Getenv("IMAGE_FETCH_ONLY") == "true" {
			fmt.Fprintf(os.Stderr, "%s is incompatible with IMAGE_FETCH_ONLY=true\n", dmverityCacheEnv)
			os.Exit(1)
		}
		if err := requireDmverityReferrerCapability(ctx, client); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to configure dm-verity cache mode: %v\n", err)
			os.Exit(1)
		}
		fetch = cacheDmverityImage
	}

	failed := 0
	for _, ref := range os.Args[1:] {
		fmt.Printf("Fetching %s ...\n", ref)
		if err := fetch(ctx, client, ref); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL  %s: %v\n", ref, err)
			failed++
		}
	}

	if failed > 0 {
		os.Exit(1)
	}
}

// fetchImage downloads the selected image without unpacking it, then unpacks
// images below pullSizeThreshold unless IMAGE_FETCH_ONLY=true.
func fetchImage(ctx context.Context, client *containerd.Client, ref string) error {
	fetchOnly := os.Getenv("IMAGE_FETCH_ONLY") == "true"

	p, platformMatcher, err := currentPlatform()
	if err != nil {
		return err
	}

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

func currentPlatform() (ocispec.Platform, platforms.MatchComparer, error) {
	platform := fmt.Sprintf("linux/%s", runtime.GOARCH)
	p, err := platforms.Parse(platform)
	if err != nil {
		return ocispec.Platform{}, nil, fmt.Errorf("parse platform %s: %w", platform, err)
	}
	return p, platforms.OnlyStrict(p), nil
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
