package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	containerd "github.com/containerd/containerd/v2/client"
	transferimage "github.com/containerd/containerd/v2/core/transfer/image"
	"github.com/containerd/containerd/v2/core/transfer/registry"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultSocket          = "/run/containerd/containerd.sock"
	defaultNS              = "k8s.io"
	defaultHostsDir        = "/etc/containerd/certs.d"
	transferSnapshotterEnv = "IMAGE_FETCHER_SNAPSHOTTER"
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

	failed := 0
	for _, ref := range os.Args[1:] {
		if err := fetchImage(ctx, client, ref); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL  %s: %v\n", ref, err)
			failed++
		}
	}

	if failed > 0 {
		os.Exit(1)
	}
}

// fetchImage preserves the existing 150 MiB unpack policy. ACL overlayfs
// caching uses a fetch-only server transfer first so signed referrers are
// retained once, then unpacks small images locally from the shared content
// store without a second registry traversal.
func fetchImage(ctx context.Context, client *containerd.Client, ref string) error {
	fetchOnly := os.Getenv("IMAGE_FETCH_ONLY") == "true"
	transferSnapshotter := os.Getenv(transferSnapshotterEnv)
	switch transferSnapshotter {
	case "", "overlayfs":
	default:
		return fmt.Errorf("unsupported %s %q", transferSnapshotterEnv, transferSnapshotter)
	}

	fmt.Printf("Fetching %s ...\n", ref)

	platform := fmt.Sprintf("linux/%s", runtime.GOARCH)
	p, err := platforms.Parse(platform)
	if err != nil {
		return fmt.Errorf("parse platform %s: %w", platform, err)
	}
	platformMatcher := platforms.OnlyStrict(p)

	if transferSnapshotter == "overlayfs" {
		if err := requireDmverityReferrerCapability(ctx, client); err != nil {
			return fmt.Errorf("dm-verity referrer caching unavailable: %w", err)
		}

		image, err := transferImage(ctx, client, ref, p)
		if err != nil {
			return fmt.Errorf("transfer failed: %w", err)
		}
		if err := validateImagePlatform(ctx, image, p); err != nil {
			return err
		}

		if fetchOnly {
			fmt.Printf("OK    %s -> %s (transferred)\n", image.Name(), image.Target().Digest)
			return nil
		}

		size, err := image.Size(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN  %s: could not determine image size, skipping unpack: %v\n", ref, err)
			fmt.Printf("OK    %s -> %s (transferred)\n", image.Name(), image.Target().Digest)
			return nil
		}

		if size < pullSizeThreshold {
			if err := image.Unpack(ctx, transferSnapshotter); err != nil {
				return fmt.Errorf("local %s unpack failed: %w", transferSnapshotter, err)
			}
			fmt.Printf("OK    %s -> %s (transferred and unpacked, %s)\n", image.Name(), image.Target().Digest, formatSize(size))
			return nil
		}

		fmt.Printf("OK    %s -> %s (transferred, %s)\n", image.Name(), image.Target().Digest, formatSize(size))
		return nil
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

func transferImage(ctx context.Context, client *containerd.Client, ref string, platform ocispec.Platform) (containerd.Image, error) {
	source, err := registry.NewOCIRegistry(ctx, ref, registry.WithHostDir(defaultHostsDir))
	if err != nil {
		return nil, fmt.Errorf("create registry source: %w", err)
	}

	if err := client.Transfer(ctx, source, transferimage.NewStore(ref, transferimage.WithPlatforms(platform))); err != nil {
		return nil, err
	}

	imageMeta, err := client.ImageService().Get(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("load transferred image: %w", err)
	}
	return containerd.NewImageWithPlatform(client, imageMeta, platforms.OnlyStrict(platform)), nil
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
