package e2e

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

// cachedFunc creates a memoized version of a function
func cachedFunc[K comparable, V any](fn func(context.Context, K) (V, error)) func(context.Context, K) (V, error) {
	type entry struct {
		once  sync.Once
		value V
		err   error
	}

	var cache sync.Map

	return func(ctx context.Context, key K) (V, error) {
		actual, _ := cache.LoadOrStore(key, &entry{})
		e := actual.(*entry)

		e.once.Do(func() {
			e.value, e.err = fn(ctx, key)
		})

		return e.value, e.err
	}
}

// Request structs are used as a cache key.
// The cache key must uniquely identify the request
// The cache key should not container pointers, maps or slices to avoid issues with comparing the keys.
type CreateGalleryRequest struct {
	Location      string
	ResourceGroup string
}
type CreateGalleryImageRequest struct {
	ResourceGroup string
	GalleryName   string
	Location      string
	Arch          string
	Windows       bool
}

var CachedCreateGallery = cachedFunc(createGallery)

func createGallery(ctx context.Context, request CreateGalleryRequest) (armcompute.Gallery, error) {
	// gallery name should be unique within the subscription
	// minus isn't allowed
	galleryName := config.Config.TestGalleryNamePrefix + request.Location

	gallery, err := config.Azure.Galleries.Get(ctx, request.ResourceGroup, galleryName, nil)
	if err == nil {
		return gallery.Gallery, nil
	}
	if !isNotFoundErr(err) {
		return armcompute.Gallery{}, fmt.Errorf("failed to get gallery: %w", err)
	}
	// If the gallery does not exist, create it.
	poller, err := config.Azure.Galleries.BeginCreateOrUpdate(ctx, request.ResourceGroup, galleryName, armcompute.Gallery{
		Location: to.Ptr(request.Location),
		Properties: &armcompute.GalleryProperties{
			Description: to.Ptr("E2E test gallery for two-stage kubelet configuration"),
		},
	}, nil)
	if err != nil {
		return armcompute.Gallery{}, fmt.Errorf("failed to create gallery: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return armcompute.Gallery{}, fmt.Errorf("failed to poll gallery creation: %w", err)
	}
	return resp.Gallery, nil
}

var CachedCreateGalleryImage = cachedFunc(createGalleryImage)

func createGalleryImage(ctx context.Context, request CreateGalleryImageRequest) (armcompute.GalleryImage, error) {
	imageName := fmt.Sprintf("%s-%s-%s", config.Config.TestGalleryImagePrefix, request.Location, request.Arch)
	if request.Windows {
		imageName += "-windows"
	} else {
		imageName += "-linux"
	}
	image, err := config.Azure.GalleryImages.Get(ctx, request.ResourceGroup, request.GalleryName, imageName, nil)
	if err == nil {
		return image.GalleryImage, nil
	}
	if !isNotFoundErr(err) {
		return armcompute.GalleryImage{}, fmt.Errorf("failed to get gallery image: %w", err)
	}
	poller, err := config.Azure.GalleryImages.BeginCreateOrUpdate(ctx, request.ResourceGroup, request.GalleryName, imageName, armcompute.GalleryImage{
		Location: to.Ptr(request.Location),
		Properties: &armcompute.GalleryImageProperties{
			Architecture: func() *armcompute.Architecture {
				if request.Arch == "arm64" {
					return to.Ptr(armcompute.ArchitectureArm64)
				}
				return to.Ptr(armcompute.ArchitectureX64)
			}(),
			OSType: func() *armcompute.OperatingSystemTypes {
				if request.Windows {
					return to.Ptr(armcompute.OperatingSystemTypesWindows)
				}
				return to.Ptr(armcompute.OperatingSystemTypesLinux)
			}(),
			OSState: to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
			Identifier: &armcompute.GalleryImageIdentifier{
				// Combination of these 3 fields must be unique for each image
				Publisher: to.Ptr("akse2e"),
				Offer:     to.Ptr("akse2e"),
				SKU:       to.Ptr(imageName),
			},
			HyperVGeneration: to.Ptr(armcompute.HyperVGenerationV2),
		},
	}, nil)
	if err != nil {
		return armcompute.GalleryImage{}, fmt.Errorf("failed to create gallery image: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return armcompute.GalleryImage{}, fmt.Errorf("failed to poll gallery image creation: %w", err)
	}
	return resp.GalleryImage, nil
}

var ClusterLatestKubernetesVersion = cachedFunc(clusterLatestKubernetesVersion)

func clusterLatestKubernetesVersion(ctx context.Context, location string) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-latest-kubernetes-version", location)
	if err != nil {
		return nil, fmt.Errorf("getting latest kubernetes version cluster model: %w", err)
	}
	return prepareCluster(ctx, model, false, false, true)
}

var ClusterKubenet = cachedFunc(clusterKubenet)

func clusterKubenet(ctx context.Context, location string) (*Cluster, error) {
	return prepareCluster(ctx, getKubenetClusterModel("abe2e-kubenet", location), false, false, true)
}

var ClusterKubenetNoNvidiaDevicePlugin = cachedFunc(clusterKubenetNoNvidiaDevicePlugin)

func clusterKubenetNoNvidiaDevicePlugin(ctx context.Context, location string) (*Cluster, error) {
	// This is purposefully named in short form to avoid going over the 80
	// char limit of the resource groups.
	return prepareCluster(ctx, getKubenetClusterModel("abe2e-kubenet-no-nvidia-dev", location), false, false, false)
}

var ClusterKubenetAirgap = cachedFunc(clusterKubenetAirgap)

func clusterKubenetAirgap(ctx context.Context, location string) (*Cluster, error) {
	return prepareCluster(ctx, getKubenetClusterModel("abe2e-kubenet-airgap", location), true, false, true)
}

var ClusterKubenetAirgapNonAnon = cachedFunc(clusterKubenetAirgapNonAnon)

func clusterKubenetAirgapNonAnon(ctx context.Context, location string) (*Cluster, error) {
	return prepareCluster(ctx, getKubenetClusterModel("abe2e-kubenet-nonanonpull-airgap", location), true, true, true)
}

var ClusterAzureNetwork = cachedFunc(clusterAzureNetwork)

func clusterAzureNetwork(ctx context.Context, location string) (*Cluster, error) {
	return prepareCluster(ctx, getAzureNetworkClusterModel("abe2e-azure-network", location), false, false, true)
}

var ClusterAzureOverlayNetwork = cachedFunc(clusterAzureOverlayNetwork)

func clusterAzureOverlayNetwork(ctx context.Context, location string) (*Cluster, error) {
	return prepareCluster(ctx, getAzureOverlayNetworkClusterModel("abe2e-azure-overlay-network", location), false, false, true)
}

var ClusterAzureOverlayNetworkDualStack = cachedFunc(clusterAzureOverlayNetworkDualStack)

func clusterAzureOverlayNetworkDualStack(ctx context.Context, location string) (*Cluster, error) {
	return prepareCluster(ctx, getAzureOverlayNetworkDualStackClusterModel("abe2e-azure-overlay-dualstack", location), false, false, true)
}

var ClusterCiliumNetwork = cachedFunc(clusterCiliumNetwork)

func clusterCiliumNetwork(ctx context.Context, location string) (*Cluster, error) {
	return prepareCluster(ctx, getCiliumNetworkClusterModel("abe2e-cilium-network", location), false, false, true)
}

func isNotFoundErr(err error) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return false
}
