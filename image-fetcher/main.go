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
	defaultSocket    = "/run/containerd/containerd.sock"
	defaultNamespace = "k8s.io"
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
		ns = defaultNamespace
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
// This is the Go API equivalent of "content fetch + images create" that
// ctr CLI no longer exposes as a single operation in containerd 2.x.
func fetchImage(ctx context.Context, client *containerd.Client, ref string) error {
	fmt.Printf("Fetching %s ...\n", ref)

	platform := fmt.Sprintf("linux/%s", runtime.GOARCH)

	image, err := client.Fetch(ctx, ref,
		containerd.WithPlatform(platform),
	)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	fmt.Printf("OK    %s -> %s\n", image.Name, image.Target.Digest)
	return nil
}
