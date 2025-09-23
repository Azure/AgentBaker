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

// cachedFunc creates a thread-safe memoized version of a function.
// Results are cached per unique Request key using sync.Once for single execution.
// Request type must be comparable (no slices/maps/pointers).
// Cache persists for program lifetime with no TTL or invalidation.
// WARNING: Incorrect keys can cause hard-to-debug cache collisions.
func cachedFunc[Request comparable, Response any](fn func(context.Context, Request) (Response, error)) func(context.Context, Request) (Response, error) {
	type entry struct {
		once  sync.Once
		value Response
		err   error
	}

	var cache sync.Map

	return func(ctx context.Context, key Request) (Response, error) {
		actual, _ := cache.LoadOrStore(key, &entry{})
		e := actual.(*entry)

		e.once.Do(func() {
			e.value, e.err = fn(ctx, key)
		})

		return e.value, e.err
	}
}

var CachedCreateGallery = cachedFunc(createGallery)

type CreateGalleryRequest struct {
	Location      string
	ResourceGroup string
}

// createGallery creates or retrieves an Azure Compute Gallery for e2e testing
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

type CreateGalleryImageRequest struct {
	ResourceGroup string
	GalleryName   string
	Location      string
	Arch          string
	Windows       bool
}

// createGalleryImage creates or retrieves an Azure Compute Gallery Image for e2e testing
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

// ClusterRequest represents the parameters needed to create a cluster
type ClusterRequest struct {
	Location         string
	K8sSystemPoolSKU string
}

var ClusterLatestKubernetesVersion = cachedFunc(clusterLatestKubernetesVersion)

// clusterLatestKubernetesVersion creates a cluster with the latest available Kubernetes version
func clusterLatestKubernetesVersion(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-latest-kubernetes-version", request.Location, request.K8sSystemPoolSKU)
	if err != nil {
		return nil, fmt.Errorf("getting latest kubernetes version cluster model: %w", err)
	}
	return prepareCluster(ctx, model, false, false)
}

var ClusterKubenet = cachedFunc(clusterKubenet)

// clusterKubenet creates a basic cluster using kubenet networking
func clusterKubenet(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	return prepareCluster(ctx, getKubenetClusterModel("abe2e-kubenet-v2", request.Location, request.K8sSystemPoolSKU), false, false)
}

var ClusterKubenetAirgap = cachedFunc(clusterKubenetAirgap)

// clusterKubenetAirgap creates an airgapped kubenet cluster (no internet access)
func clusterKubenetAirgap(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	return prepareCluster(ctx, getKubenetClusterModel("abe2e-kubenet-airgap", request.Location, request.K8sSystemPoolSKU), true, false)
}

var ClusterKubenetAirgapNonAnon = cachedFunc(clusterKubenetAirgapNonAnon)

// clusterKubenetAirgapNonAnon creates an airgapped kubenet cluster with non-anonymous image pulls
func clusterKubenetAirgapNonAnon(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	return prepareCluster(ctx, getKubenetClusterModel("abe2e-kubenet-nonanonpull-airgap", request.Location, request.K8sSystemPoolSKU), true, true)
}

var ClusterAzureNetwork = cachedFunc(clusterAzureNetwork)

// clusterAzureNetwork creates a cluster with Azure CNI networking
func clusterAzureNetwork(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	return prepareCluster(ctx, getAzureNetworkClusterModel("abe2e-azure-network", request.Location, request.K8sSystemPoolSKU), false, false)
}

var ClusterAzureOverlayNetwork = cachedFunc(clusterAzureOverlayNetwork)

// clusterAzureOverlayNetwork creates a cluster with Azure CNI Overlay networking
func clusterAzureOverlayNetwork(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	return prepareCluster(ctx, getAzureOverlayNetworkClusterModel("abe2e-azure-overlay-network", request.Location, request.K8sSystemPoolSKU), false, false)
}

var ClusterAzureOverlayNetworkDualStack = cachedFunc(clusterAzureOverlayNetworkDualStack)

// clusterAzureOverlayNetworkDualStack creates a dual-stack (IPv4+IPv6) Azure CNI Overlay cluster
func clusterAzureOverlayNetworkDualStack(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	return prepareCluster(ctx, getAzureOverlayNetworkDualStackClusterModel("abe2e-azure-overlay-dualstack", request.Location, request.K8sSystemPoolSKU), false, false)
}

var ClusterCiliumNetwork = cachedFunc(clusterCiliumNetwork)

// clusterCiliumNetwork creates a cluster with Cilium CNI networking
func clusterCiliumNetwork(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	return prepareCluster(ctx, getCiliumNetworkClusterModel("abe2e-cilium-network", request.Location, request.K8sSystemPoolSKU), false, false)
}

// isNotFoundErr checks if an error represents a "not found" response from Azure API
func isNotFoundErr(err error) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return false
}

var CachedPrepareVHD = cachedFunc(prepareVHD)

type GetVHDRequest struct {
	Location string
	Image    config.Image
}

// prepareVHD retrieves the Azure resource ID for a VHD image. A gallery is scanned for the correct version
// and replicated to the location specified in the request if it does not already exist.
func prepareVHD(ctx context.Context, request GetVHDRequest) (config.VHDResourceID, error) {
	return config.GetVHDResourceID(ctx, request.Image, request.Location)
}

var CachedEnsureResourceGroup = cachedFunc(ensureResourceGroup)
var CachedCreateVMManagedIdentity = cachedFunc(config.Azure.CreateVMManagedIdentity)
var CachedCompileAndUploadAKSNodeController = cachedFunc(compileAndUploadAKSNodeController)
