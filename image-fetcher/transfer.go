package main

import (
	"context"
	"fmt"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	transferimage "github.com/containerd/containerd/v2/core/transfer/image"
	"github.com/containerd/containerd/v2/core/transfer/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const fetcherTransferEnv = "IMAGE_FETCHER_USE_TRANSFER"

func transferImage(ctx context.Context, client *containerd.Client, ref string, platform ocispec.Platform) (images.Image, error) {
	source, err := registry.NewOCIRegistry(ctx, ref)
	if err != nil {
		return images.Image{}, fmt.Errorf("create registry source: %w", err)
	}
	destination := transferimage.NewStore(ref, transferimage.WithPlatforms(platform),
		transferimage.WithImageLabels(map[string]string{"io.cri-containerd.image": "managed"}))
	if err := client.Transfer(ctx, source, destination); err != nil {
		return images.Image{}, fmt.Errorf("transfer image: %w", err)
	}
	imageMeta, err := client.ImageService().Get(ctx, ref)
	if err != nil {
		return images.Image{}, fmt.Errorf("load transferred image: %w", err)
	}
	return imageMeta, nil
}
