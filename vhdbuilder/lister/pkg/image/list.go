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
			digestToImage[digest] = &Image{}
		}
		img := digestToImage[digest]
		if isID(image.Name) {
			if img.ID != "" && img.ID != image.Name {
				return nil, fmt.Errorf("found multiple IDs for digest %s: %s and %s", digest, img.ID, image.Name)
			}
			if img.ID == "" {
				img.ID = image.Name
			}
		} else {
			img.RepoTags = append(img.RepoTags, image.Name)
		}
		if img.Bytes == 0 {
			img.Bytes = image.Target.Size
		} else {
			if img.Bytes != image.Target.Size {
				return nil, fmt.Errorf("found different byte sizes (%d, %d) for digest %s", img.Bytes, image.Target.Size, digest)
			}
		}
	}

	var bom []Image
	for digest := range digestToImage {
		img := digestToImage[digest]
		bom = append(bom, *img)
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
