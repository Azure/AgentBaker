package main

import (
	"context"
	"fmt"
	"strings"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/platforms"
)

func ListImages(ctx context.Context, client *containerd.Client, sku, version string) (*List, error) {
	imageStore := client.ImageService()
	contentStore := client.ContentStore()

	images, err := imageStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing images with image service: %w", err)
	}

	digestToImage := map[string]*Image{}

	for _, image := range images {
		digest := image.Target.Digest.String()
		if _, ok := digestToImage[digest]; !ok {
			digestToImage[digest] = New()
		}
		img := digestToImage[digest]
		img.AddDigest(digest)
		if isID(image.Name) {
			if err := img.SetID(image.Name); err != nil {
				return nil, fmt.Errorf("setting ID for image digest %s: %w", digest, err)
			}
		} else {
			img.AddTag(image.Name)
		}
		size, err := image.Size(ctx, contentStore, platforms.Default())
		if err != nil {
			return nil, fmt.Errorf("calculating size for image digest %s: %w", digest, err)
		}
		if err := img.SetByteSize(size); err != nil {
			return nil, fmt.Errorf("setting size for image digest %s: %w", digest, err)
		}
	}

	var bom []*Image
	for digest := range digestToImage {
		bom = append(bom, digestToImage[digest])
	}

	return &List{
		SKU:     sku,
		Version: version,
		BOM:     bom,
	}, nil
}

func isID(imageName string) bool {
	return strings.Contains(imageName, "sha256")
}
