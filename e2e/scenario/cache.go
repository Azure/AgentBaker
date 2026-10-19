package scenario

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
)

var CachedCreateGallery = toolkit.Memoize(createGallery)

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
			Description: to.Ptr("E2E test gallery for VHD caching"),
		},
	}, nil)
	if err != nil {
		return armcompute.Gallery{}, fmt.Errorf("failed to create gallery: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, config.PollUntilDoneOptions())
	if err != nil {
		return armcompute.Gallery{}, fmt.Errorf("failed to poll gallery creation: %w", err)
	}
	return resp.Gallery, nil
}

var CachedCreateGalleryImage = toolkit.Memoize(createGalleryImage)

type CreateGalleryImageRequest struct {
	ResourceGroup    string
	GalleryName      string
	Location         string
	Arch             string
	Windows          bool
	HyperVGeneration *armcompute.HyperVGeneration
}

// createGalleryImage creates or retrieves an Azure Compute Gallery Image for e2e testing
func createGalleryImage(ctx context.Context, request CreateGalleryImageRequest) (armcompute.GalleryImage, error) {
	imageName := fmt.Sprintf("%s-%s-%s-gen%s", config.Config.TestGalleryImagePrefix, request.Location, request.Arch, *request.HyperVGeneration)
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
			HyperVGeneration: request.HyperVGeneration, // IMPORTANT, INCORRECT VALUE CAUSES VM PROVISIONING TO FAIL WITHOUT CLEAR ERROR MESSAGE
		},
	}, nil)
	if err != nil {
		return armcompute.GalleryImage{}, fmt.Errorf("failed to create gallery image: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, config.PollUntilDoneOptions())
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

var ClusterLatestKubernetesVersion = toolkit.Memoize(clusterLatestKubernetesVersion)

// clusterLatestKubernetesVersion creates a cluster with the latest available Kubernetes version
func clusterLatestKubernetesVersion(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-latest-k8s-v2", request.Location, request.K8sSystemPoolSKU)
	if err != nil {
		return nil, fmt.Errorf("getting latest kubernetes version cluster model: %w", err)
	}
	return prepareCluster(ctx, model, false, false)
}

var ClusterLatestKubernetesVersionKubenet = toolkit.Memoize(clusterLatestKubernetesVersionKubenet)

func clusterLatestKubernetesVersionKubenet(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-latest-k8s-kubenet-v1", request.Location, request.K8sSystemPoolSKU)
	if err != nil {
		return nil, fmt.Errorf("getting latest kubernetes version cluster model: %w", err)
	}
	model = kubenetClusterModelMutator(model)
	return prepareCluster(ctx, model, false, false)
}

var ClusterLatestKubernetesVersionAzureNetwork = toolkit.Memoize(clusterLatestKubernetesVersionAzureNetwork)

func clusterLatestKubernetesVersionAzureNetwork(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-latest-k8s-azure-network-v1", request.Location, request.K8sSystemPoolSKU)
	if err != nil {
		return nil, fmt.Errorf("getting latest kubernetes version cluster model: %w", err)
	}
	model = azureNetworkClusterModelMutator(model)
	return prepareCluster(ctx, model, false, false)
}

var ClusterLatestKubernetesVersionAzureOverlayNetworkDualStack = toolkit.Memoize(clusterLatestKubernetesVersionAzureOverlayNetworkDualStack)

func clusterLatestKubernetesVersionAzureOverlayNetworkDualStack(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-latest-k8s-azure-overlay-dualstack-v1", request.Location, request.K8sSystemPoolSKU)
	if err != nil {
		return nil, fmt.Errorf("getting latest kubernetes version cluster model: %w", err)
	}
	model = azureOverlayNetworkDualStackClusterModelMutator(model)
	return prepareCluster(ctx, model, false, false)
}

var ClusterLatestKubernetesVersionAzureBootstrapProfileCache = toolkit.Memoize(clusterLatestKubernetesVersionAzureBootstrapProfileCache)

// clusterAzureBootstrapProfileCache creates a cluster with bootstrap profile cache but without network isolation
func clusterLatestKubernetesVersionAzureBootstrapProfileCache(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-latest-k8s-azure-bootstrapprofile-cache-v1", request.Location, request.K8sSystemPoolSKU)
	if err != nil {
		return nil, fmt.Errorf("getting latest kubernetes version cluster model: %w", err)
	}
	model = azureNetworkClusterModelMutator(model)
	return prepareCluster(ctx, model, false, true)
}

var ClusterKubenet = toolkit.Memoize(clusterKubenet)

// clusterKubenet creates a basic cluster using kubenet networking with shared VNet
func clusterKubenet(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	clusterName := "abe2e-kubenet-v5"
	model := getKubenetClusterModel(clusterName, request.Location, request.K8sSystemPoolSKU)
	return prepareCluster(ctx, model, false, false)
}

var ClusterAzureNetwork = toolkit.Memoize(clusterAzureNetwork)

// clusterAzureNetwork creates a cluster with Azure CNI networking
func clusterAzureNetwork(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model := getAzureNetworkClusterModel("abe2e-azure-network-v4", request.Location, request.K8sSystemPoolSKU)
	return prepareCluster(ctx, model, false, false)
}

var ClusterAzureBootstrapProfileCache = toolkit.Memoize(clusterAzureBootstrapProfileCache)

// clusterAzureBootstrapProfileCache creates a cluster with bootstrap profile cache but without network isolation
func clusterAzureBootstrapProfileCache(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model := getAzureNetworkClusterModel("abe2e-azure-bootstrapprofile-cache-v2", request.Location, request.K8sSystemPoolSKU)
	return prepareCluster(ctx, model, false, true)
}

var ClusterAzureNetworkIsolated = toolkit.Memoize(clusterAzureNetworkIsolated)

// clusterAzureNetworkIsolated creates a networkisolated Azure network cluster (no internet access)
func clusterAzureNetworkIsolated(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model := getAzureNetworkClusterModel("abe2e-azure-networkisolated-v3", request.Location, request.K8sSystemPoolSKU)
	return prepareCluster(ctx, model, true, false)
}

var ClusterAzureOverlayNetwork = toolkit.Memoize(clusterAzureOverlayNetwork)

// clusterAzureOverlayNetwork creates a cluster with Azure CNI Overlay networking
func clusterAzureOverlayNetwork(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model := getAzureOverlayNetworkClusterModel("abe2e-azure-overlay-network-v4", request.Location, request.K8sSystemPoolSKU)
	return prepareCluster(ctx, model, false, false)
}

var ClusterAzureOverlayNetworkDualStack = toolkit.Memoize(clusterAzureOverlayNetworkDualStack)

func clusterAzureOverlayNetworkDualStack(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model := getAzureOverlayNetworkDualStackClusterModel("abe2e-azure-overlay-dualstack-v6", request.Location, request.K8sSystemPoolSKU)
	return prepareCluster(ctx, model, false, false)
}

var ClusterCiliumNetwork = toolkit.Memoize(clusterCiliumNetwork)

// clusterCiliumNetwork creates a cluster with Cilium CNI networking
func clusterCiliumNetwork(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model := getCiliumNetworkClusterModel("abe2e-cilium-network-v4", request.Location, request.K8sSystemPoolSKU)
	return prepareCluster(ctx, model, false, false)
}

// isNotFoundErr checks if an error represents a "not found" response from Azure API
func isNotFoundErr(err error) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return false
}

var CachedPrepareVHD = toolkit.Memoize(prepareVHD)

type GetVHDRequest struct {
	Location string
	Image    config.Image
}

// prepareVHD retrieves the Azure resource ID for a VHD image. A gallery is scanned for the correct version
// and replicated to the location specified in the request if it does not already exist.
func prepareVHD(ctx context.Context, request GetVHDRequest) (config.VHDResourceID, error) {
	return config.GetVHDResourceID(ctx, request.Image, request.Location)
}

var CachedEnsureResourceGroup = toolkit.Memoize(func(ctx context.Context, location string) (armresources.ResourceGroup, error) {
	return ensureResourceGroup(ctx, config.Azure, config.Config, config.ResourceGroupName(location), location)
})
var CachedCreateVMManagedIdentity = toolkit.Memoize(func(ctx context.Context, location string) (string, error) {
	// Shared storage uses DefaultLocation even when the scenario runs elsewhere.
	if _, err := CachedEnsureResourceGroup(ctx, config.Config.DefaultLocation); err != nil {
		return "", fmt.Errorf("ensure storage resource group: %w", err)
	}
	return config.Azure.CreateVMManagedIdentity(ctx, location)
})
var CachedCompileAndUploadAKSNodeController = toolkit.Memoize(compileAndUploadAKSNodeController)

var cachedVMResourceSKUs = toolkit.Memoize(func(ctx context.Context, location string) (map[string]*armcompute.ResourceSKU, error) {
	return config.Azure.ListVMResourceSKUs(ctx, location)
})

func getVMSizeSKU(ctx context.Context, location, vmSize string) (*armcompute.ResourceSKU, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	catalogue, err := cachedVMResourceSKUs(ctx, strings.ToLower(location))
	if err != nil {
		return nil, err
	}
	if sku := catalogue[strings.ToUpper(vmSize)]; sku != nil {
		return sku, nil
	}
	return nil, fmt.Errorf("VM size %q not found in location %q", vmSize, location)
}

// GetLatestExtensionVersionRequest is the cache key for VM extension version lookups.
type GetLatestExtensionVersionRequest struct {
	Location  string
	ExtType   string
	Publisher string
}

// CachedGetLatestVMExtensionImageVersion caches the result of querying the Azure API
// for the latest VM extension image version.
var CachedGetLatestVMExtensionImageVersion = toolkit.Memoize(
	func(ctx context.Context, req GetLatestExtensionVersionRequest) (string, error) {
		return config.Azure.GetLatestVMExtensionImageVersion(ctx, req.Location, req.ExtType, req.Publisher)
	},
)
