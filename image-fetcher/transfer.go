package main

import (
	"context"
	"fmt"
	containerd "github.com/containerd/containerd/v2/client"
	transferimage "github.com/containerd/containerd/v2/core/transfer/image"
	"github.com/containerd/containerd/v2/core/transfer/registry"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const serverTransferEnv = "IMAGE_FETCHER_SERVER_TRANSFER"

func transferImage(ctx context.Context, client *containerd.Client, ref string, platform ocispec.Platform) (containerd.Image, error) {
	source, err := registry.NewOCIRegistry(ctx, ref, registry.WithHostDir("/etc/containerd/certs.d"))
	if err != nil {
		return nil, fmt.Errorf("create registry source: %w", err)
	}
	destination := transferimage.NewStore(ref, transferimage.WithPlatforms(platform),
		transferimage.WithImageLabels(map[string]string{"io.cri-containerd.image": "managed"}))
	if err := client.Transfer(ctx, source, destination); err != nil {
		return nil, fmt.Errorf("transfer image: %w", err)
	}
	imageMeta, err := client.ImageService().Get(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("load transferred image: %w", err)
	}
	return containerd.NewImageWithPlatform(client, imageMeta, platforms.OnlyStrict(platform)), nil
}
