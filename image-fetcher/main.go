//nolint:forbidigo // used for vhd building only
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
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

func execute() int {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <image-ref> [image-ref...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s --gc\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s mcr.microsoft.com/oss/kubernetes/pause:3.9\n", os.Args[0])
		return 1
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
		return 1
	}
	defer client.Close()

	ctx := namespaces.WithNamespace(context.Background(), ns)

	if len(os.Args) == 2 && os.Args[1] == "--gc" {
		if err := triggerGarbageCollection(ctx, client); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to trigger containerd GC: %v\n", err)
			return 1
		}
		fmt.Println("Triggered containerd GC")
		return 0
	}

	failed := 0
	for _, ref := range os.Args[1:] {
		if err := fetchImage(ctx, client, ref); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL  %s: %v\n", ref, err)
			failed++
		}
	}

	if failed > 0 {
		return 1
	}
	return 0
}

func main() {
	os.Exit(execute())
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

	fmt.Printf("Working on %s ...\n", ref)

	platform := fmt.Sprintf("linux/%s", runtime.GOARCH)
	p, pErr := platforms.Parse(platform)
	if pErr != nil {
		return fmt.Errorf("parse platform %s: %w", platform, pErr)
	}
	platformMatcher := platforms.OnlyStrict(p)

	var imageSize int64
	if !fetchOnly {
		size, err := getRemoteImageSize(ctx, ref, platformMatcher)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR  %s: could not determine remote image size: %v\n", ref, err)
			return err
		}
		imageSize = size
	}

	if fetchOnly || (imageSize > pullSizeThreshold) {
		imageMeta, err := client.Fetch(ctx, ref,
			containerd.WithPlatformMatcher(platformMatcher),
		)
		if err != nil {
			return fmt.Errorf("fetch failed: %w", err)
		}
		fmt.Printf("OK    %s -> %s (fetched)\n", imageMeta.Name, imageMeta.Target.Digest)
		return nil
	}

	// We use pull here instead of use unpack because some runtimes (e.g. containerd-shim-runsc-v1),
	// require pull to trigger unpacking into the correct snapshotter based on the image's platform.
	imageMeta, err := client.Pull(ctx, ref, containerd.WithPlatformMatcher(platformMatcher),
		containerd.WithPullUnpack,
		containerd.WithChildLabelMap(images.ChildGCLabelsFilterLayers),
	)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	fmt.Printf("OK    %s -> %s (pulled, %s)\n", imageMeta.Name(), imageMeta.Target().Digest.String(), formatSize(imageSize))

	return nil
}

func getRemoteImageSize(ctx context.Context, ref string, platformMatcher platforms.MatchComparer) (int64, error) {
	resolver := docker.NewResolver(docker.ResolverOptions{})

	name, desc, err := resolver.Resolve(ctx, ref)
	if err != nil {
		return 0, fmt.Errorf("resolve reference: %w", err)
	}

	fetcher, err := resolver.Fetcher(ctx, name)
	if err != nil {
		return 0, fmt.Errorf("get fetcher: %w", err)
	}

	size, err := remoteDescriptorSize(ctx, fetcher, desc, platformMatcher)
	if err != nil {
		return 0, fmt.Errorf("calculate remote size: %w", err)
	}

	return size, nil
}

func remoteDescriptorSize(ctx context.Context, fetcher remotes.Fetcher, desc ocispec.Descriptor, platformMatcher platforms.MatchComparer) (int64, error) {
	size := sanitizeDescriptorSize(desc.Size)

	switch {
	case images.IsIndexType(desc.MediaType):
		var index ocispec.Index
		if err := fetchDescriptorJSON(ctx, fetcher, desc, &index); err != nil {
			return 0, fmt.Errorf("fetch index %s: %w", desc.Digest, err)
		}

		manifestDesc, err := selectManifestDescriptor(index.Manifests, platformMatcher)
		if err != nil {
			return 0, err
		}

		manifestSize, err := remoteDescriptorSize(ctx, fetcher, manifestDesc, platformMatcher)
		if err != nil {
			return 0, err
		}

		return size + manifestSize, nil
	case images.IsManifestType(desc.MediaType):
		var manifest ocispec.Manifest
		if err := fetchDescriptorJSON(ctx, fetcher, desc, &manifest); err != nil {
			return 0, fmt.Errorf("fetch manifest %s: %w", desc.Digest, err)
		}

		size += sanitizeDescriptorSize(manifest.Config.Size)
		for _, layer := range manifest.Layers {
			size += sanitizeDescriptorSize(layer.Size)
		}

		return size, nil
	default:
		return 0, fmt.Errorf("unsupported media type %q", desc.MediaType)
	}
}

func fetchDescriptorJSON(ctx context.Context, fetcher remotes.Fetcher, desc ocispec.Descriptor, target any) error {
	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := json.NewDecoder(rc).Decode(target); err != nil {
		if err == io.EOF {
			return fmt.Errorf("empty descriptor payload")
		}
		return err
	}

	return nil
}

func selectManifestDescriptor(manifests []ocispec.Descriptor, platformMatcher platforms.MatchComparer) (ocispec.Descriptor, error) {
	if len(manifests) == 0 {
		return ocispec.Descriptor{}, fmt.Errorf("image index has no manifests")
	}

	candidates := manifests
	if platformMatcher != nil {
		candidates = nil
		for _, manifest := range manifests {
			if manifest.Platform == nil || platformMatcher.Match(*manifest.Platform) {
				candidates = append(candidates, manifest)
			}
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Platform == nil {
				return false
			}
			if candidates[j].Platform == nil {
				return true
			}
			return platformMatcher.Less(*candidates[i].Platform, *candidates[j].Platform)
		})
	}

	if len(candidates) == 0 {
		return ocispec.Descriptor{}, fmt.Errorf("no manifest matched requested platform")
	}

	return candidates[0], nil
}

func sanitizeDescriptorSize(size int64) int64 {
	if size < 0 {
		return 0
	}
	return size
}

func triggerGarbageCollection(ctx context.Context, client *containerd.Client) error {
	ls := client.LeasesService()
	l, err := ls.Create(ctx, leases.WithRandomID(), leases.WithExpiration(time.Hour))
	if err != nil {
		return err
	}
	return ls.Delete(ctx, l, leases.SynchronousDelete)
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
