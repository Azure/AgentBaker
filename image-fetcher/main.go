package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
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
		fmt.Fprintf(os.Stderr, "Example: %s mcr.microsoft.com/oss/kubernetes/pause:3.9\n", os.Args[0])
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

// fetchImage uses client.Fetch() which:
//   - Downloads all blobs (manifest, config, layers) into the content store
//   - Creates an image record in the metadata database
//   - Does NOT unpack layers into the snapshotter
//
// If the total image content size is below pullSizeThreshold (300 MiB),
// client.Pull() is called to additionally unpack the layers. Pull reuses
// already-fetched content from the store and handles snapshotter resolution
// internally (namespace label → platform default).
func fetchImage(ctx context.Context, client *containerd.Client, ref string) error {
	fmt.Printf("Fetching %s ...\n", ref)

	platform := fmt.Sprintf("linux/%s", runtime.GOARCH)

	imageMeta, err := client.Fetch(ctx, ref,
		containerd.WithPlatform(platform),
	)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	image := containerd.NewImage(client, imageMeta)

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
			containerd.WithPlatform(platform),
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
