package image

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
)

const (
	containerdSocketPath = "/run/containerd/containerd.sock"

	k8sNamespace = "k8s.io"
)

func ListImages(sku, version string) (*List, error) {
	client, err := containerd.New(containerdSocketPath)
	if err != nil {
		return nil, fmt.Errorf("create containerd client over socket %s: %w", containerdSocketPath, err)
	}

	ctx := namespaces.WithNamespace(context.Background(), k8sNamespace)
	imageService := client.ImageService()

	images, err := imageService.List(ctx)
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
		if err := img.SetByteSize(image.Target.Size); err != nil {
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
