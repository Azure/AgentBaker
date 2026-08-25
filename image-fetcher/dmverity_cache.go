package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	containerd "github.com/containerd/containerd/v2/client"
	transferimage "github.com/containerd/containerd/v2/core/transfer/image"
	"github.com/containerd/containerd/v2/core/transfer/registry"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	dmverityReferrersEnv       = "IMAGE_FETCHER_DMVERITY_REFERRERS"
	defaultHostsDir            = "/etc/containerd/certs.d"
	dmverityReferrerCapability = "dmverity-referrers"
	localTransferPluginID      = "local"
)

func containerdRetainsDmverityReferrers(ctx context.Context, client *containerd.Client) bool {
	if os.Getenv(dmverityReferrersEnv) != "true" {
		return false
	}

	response, err := client.IntrospectionService().Plugins(
		ctx,
		fmt.Sprintf("type==%s,id==%s", plugins.TransferPlugin, localTransferPluginID),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN  could not inspect containerd transfer plugin, using legacy fetch path: %v\n", err)
		return false
	}
	for _, plugin := range response.Plugins {
		if plugin.Type != string(plugins.TransferPlugin) || plugin.ID != localTransferPluginID {
			continue
		}
		if plugin.InitErr != nil {
			fmt.Fprintf(
				os.Stderr,
				"WARN  containerd transfer plugin %q failed initialization, using legacy fetch path: %s\n",
				plugin.ID,
				plugin.InitErr.Message,
			)
			return false
		}
		return slices.Contains(plugin.Capabilities, dmverityReferrerCapability)
	}
	return false
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
