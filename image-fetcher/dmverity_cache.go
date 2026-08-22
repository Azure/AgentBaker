package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	introspectionapi "github.com/containerd/containerd/api/services/introspection/v1"
	containerd "github.com/containerd/containerd/v2/client"
	transferimage "github.com/containerd/containerd/v2/core/transfer/image"
	transferregistry "github.com/containerd/containerd/v2/core/transfer/registry"
	"github.com/containerd/containerd/v2/plugins"
)

const (
	defaultRegistryHostsDir    = "/etc/containerd/certs.d"
	dmverityCacheEnv           = "IMAGE_FETCHER_DMVERITY_CACHE"
	dmverityReferrerCapability = "dmverity-referrers"
	erofsDifferPluginID        = "erofs"
	erofsSnapshotterName       = "erofs"
)

func requireDmverityReferrerCapability(ctx context.Context, client *containerd.Client) error {
	response, err := client.IntrospectionService().Plugins(
		ctx,
		fmt.Sprintf("type==%s, id==%s", plugins.DiffPlugin, erofsDifferPluginID),
	)
	if err != nil {
		return fmt.Errorf("inspect containerd diff plugins: %w", err)
	}
	return validateDmverityReferrerCapability(response.Plugins)
}

func validateDmverityReferrerCapability(plugins []*introspectionapi.Plugin) error {
	for _, plugin := range plugins {
		if plugin.ID == erofsDifferPluginID &&
			plugin.InitErr == nil &&
			slices.Contains(plugin.Capabilities, dmverityReferrerCapability) {
			return nil
		}
	}
	return fmt.Errorf(
		"containerd differ %q is not active with capability %q",
		erofsDifferPluginID,
		dmverityReferrerCapability,
	)
}

// cacheDmverityImage delegates registry resolution and EROFS unpacking to
// containerd's transfer service so its dm-verity referrer handler is used. A
// content-only image record could otherwise be recovered by CRI as locally
// available without signed dm-verity metadata.
func cacheDmverityImage(ctx context.Context, client *containerd.Client, ref string) error {
	platform, platformMatcher, err := currentPlatform()
	if err != nil {
		return err
	}

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
		transferimage.WithUnpack(platform, erofsSnapshotterName),
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

	unpacked, err := image.IsUnpacked(ctx, erofsSnapshotterName)
	if err != nil {
		return fmt.Errorf("check transferred image unpack state: %w", err)
	}
	if !unpacked {
		return fmt.Errorf(
			"transfer completed without unpacking image in snapshotter %q",
			erofsSnapshotterName,
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
