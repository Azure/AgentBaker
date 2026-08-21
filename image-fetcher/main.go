package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"

	introspectionapi "github.com/containerd/containerd/api/services/introspection/v1"
	containerd "github.com/containerd/containerd/v2/client"
	transferimage "github.com/containerd/containerd/v2/core/transfer/image"
	transferregistry "github.com/containerd/containerd/v2/core/transfer/registry"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultSocket              = "/run/containerd/containerd.sock"
	defaultNS                  = "k8s.io"
	defaultRegistryHostsDir    = "/etc/containerd/certs.d"
	dmverityCacheEnv           = "IMAGE_FETCHER_DMVERITY_CACHE"
	dmverityDiffer             = "erofs"
	dmverityReferrerCapability = "dmverity-referrers"
	dmveritySnapshotter        = "erofs"
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

// In the standard mode, fetchImage uses client.Fetch() to download the selected
// image without unpacking it, then unpacks images below pullSizeThreshold unless
// IMAGE_FETCH_ONLY=true.
//
// In the dm-verity mode, registry resolution and unpack run in containerd's
// transfer service so its referrer handler is used. Every cached image is
// explicitly unpacked into EROFS; a content-only image record could otherwise
// be recovered by CRI as locally available without signed dm-verity metadata.
func fetchImage(ctx context.Context, client *containerd.Client, ref string) error {
	fetchOnly := os.Getenv("IMAGE_FETCH_ONLY") == "true"
	dmverityCache := os.Getenv(dmverityCacheEnv) == "true"

	fmt.Printf("Fetching %s ...\n", ref)

	platform := fmt.Sprintf("linux/%s", runtime.GOARCH)
	p, err := platforms.Parse(platform)
	if err != nil {
		return fmt.Errorf("parse platform %s: %w", platform, err)
	}
	platformMatcher := platforms.OnlyStrict(p)

	if dmverityCache {
		if fetchOnly {
			return fmt.Errorf("%s is incompatible with IMAGE_FETCH_ONLY=true", dmverityCacheEnv)
		}
		if err := requireDmverityReferrerCapability(ctx, client); err != nil {
			return err
		}
		return transferDmverityImage(ctx, client, ref, p, platformMatcher)
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

func requireDmverityReferrerCapability(ctx context.Context, client *containerd.Client) error {
	response, err := client.IntrospectionService().Plugins(
		ctx,
		fmt.Sprintf("type==%s, id==%s", plugins.DiffPlugin, dmverityDiffer),
	)
	if err != nil {
		return fmt.Errorf("inspect containerd diff plugins: %w", err)
	}
	return validateDmverityReferrerCapability(response.Plugins)
}

func validateDmverityReferrerCapability(plugins []*introspectionapi.Plugin) error {
	for _, plugin := range plugins {
		if plugin.ID == dmverityDiffer &&
			plugin.InitErr == nil &&
			slices.Contains(plugin.Capabilities, dmverityReferrerCapability) {
			return nil
		}
	}
	return fmt.Errorf(
		"containerd differ %q is not active with capability %q",
		dmverityDiffer,
		dmverityReferrerCapability,
	)
}

func transferDmverityImage(
	ctx context.Context,
	client *containerd.Client,
	ref string,
	platform ocispec.Platform,
	platformMatcher platforms.MatchComparer,
) error {
	registry, err := transferregistry.NewOCIRegistry(
		ctx,
		ref,
		transferregistry.WithHostDir(defaultRegistryHostsDir),
	)
	if err != nil {
		return fmt.Errorf("configure registry transfer: %w", err)
	}

	// containerd-acl-erofs.toml defines the matching transfer unpack
	// combination and explicitly binds snapshotter erofs to differ erofs.
	store := transferimage.NewStore(
		ref,
		transferimage.WithPlatforms(platform),
		transferimage.WithUnpack(platform, dmveritySnapshotter),
	)
	if err := client.Transfer(ctx, registry, store); err != nil {
		return fmt.Errorf("transfer pull failed: %w", err)
	}

	storedImage, err := client.GetImage(ctx, ref)
	if err != nil {
		return fmt.Errorf("get transferred image: %w", err)
	}
	image := containerd.NewImageWithPlatform(client, storedImage.Metadata(), platformMatcher)
	if err := validateImagePlatform(ctx, image, platform); err != nil {
		return err
	}

	unpacked, err := image.IsUnpacked(ctx, dmveritySnapshotter)
	if err != nil {
		return fmt.Errorf("check transferred image unpack state: %w", err)
	}
	if !unpacked {
		return fmt.Errorf(
			"transfer completed without unpacking image in snapshotter %q",
			dmveritySnapshotter,
		)
	}

	size, err := image.Size(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN  %s: could not determine transferred image size: %v\n", ref, err)
		fmt.Printf("OK    %s -> %s (transferred and unpacked)\n", image.Name(), image.Target().Digest)
		return nil
	}

	fmt.Printf("OK    %s -> %s (transferred and unpacked, %s)\n", image.Name(), image.Target().Digest, formatSize(size))
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
