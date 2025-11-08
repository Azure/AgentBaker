package config

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/toolkit"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerregistry/armcontainerregistry"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
)

type AzureClient struct {
	AKS                       *armcontainerservice.ManagedClustersClient
	AzureFirewall             *armnetwork.AzureFirewallsClient
	Blob                      *azblob.Client
	StorageContainers         *armstorage.BlobContainersClient
	CacheRulesClient          *armcontainerregistry.CacheRulesClient
	Core                      *azcore.Client
	Credential                *azidentity.DefaultAzureCredential
	Maintenance               *armcontainerservice.MaintenanceConfigurationsClient
	NetworkInterfaces         *armnetwork.InterfacesClient
	PrivateDNSZoneGroup       *armnetwork.PrivateDNSZoneGroupsClient
	PrivateEndpointClient     *armnetwork.PrivateEndpointsClient
	PrivateZonesClient        *armprivatedns.PrivateZonesClient
	RecordSetClient           *armprivatedns.RecordSetsClient
	RegistriesClient          *armcontainerregistry.RegistriesClient
	Resource                  *armresources.Client
	ResourceGroup             *armresources.ResourceGroupsClient
	RoleAssignments           *armauthorization.RoleAssignmentsClient
	SecurityGroup             *armnetwork.SecurityGroupsClient
	StorageAccounts           *armstorage.AccountsClient
	Subnet                    *armnetwork.SubnetsClient
	UserAssignedIdentities    *armmsi.UserAssignedIdentitiesClient
	VMSS                      *armcompute.VirtualMachineScaleSetsClient
	VMSSVM                    *armcompute.VirtualMachineScaleSetVMsClient
	VMs                       *armcompute.VirtualMachinesClient
	Images                    *armcompute.ImagesClient
	Snapshots                 *armcompute.SnapshotsClient
	Galleries                 *armcompute.GalleriesClient
	GalleryImages             *armcompute.GalleryImagesClient
	GalleryImageVersions      *armcompute.GalleryImageVersionsClient
	VNet                      *armnetwork.VirtualNetworksClient
	VirutalNetworkLinksClient *armprivatedns.VirtualNetworkLinksClient
	ArmOptions                *arm.ClientOptions
	VMSSVMRunCommands         *armcompute.VirtualMachineScaleSetVMRunCommandsClient
	VMExtensionImages         *armcompute.VirtualMachineExtensionImagesClient
}

func mustNewAzureClient() *AzureClient {
	client, err := NewAzureClient()
	if err != nil {
		panic(err)
	}
	return client

}

func NewHttpClient() *http.Client {
	httpClient := &http.Client{
		// use a bunch of connections for load balancing
		// ensure all timeouts are defined and reasonable
		// ensure TLS1.2+ and HTTP2
		//Transport: armbalancer.New(armbalancer.Options{
		//	PoolSize: 100,
		//}),
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}
	return httpClient
}

func NewAzureClient() (*AzureClient, error) {
	httpClient := NewHttpClient()
	logger := runtime.NewLogPolicy(&policy.LogOptions{
		IncludeBody: true,
	})

	opts := &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: httpClient,
			PerCallPolicies: []policy.Policy{
				logger,
			},
			Retry: DefaultRetryOpts(),
		},
	}

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	plOpts := runtime.PipelineOptions{}
	clOpts := &azcore.ClientOptions{
		Transport: httpClient,
		PerCallPolicies: []policy.Policy{
			runtime.NewBearerTokenPolicy(credential, []string{"https://management.azure.com/.default"}, nil),
			logger,
		},
		Retry: DefaultRetryOpts(),
	}

	cloud := &AzureClient{}

	// purely for telemetry, entirely unused today
	cloud.Core, err = azcore.NewClient("agentbakere2e.e2e_test", "v0.0.0", plOpts, clOpts)
	if err != nil {
		return nil, fmt.Errorf("create core client: %w", err)
	}

	cloud.RegistriesClient, err = armcontainerregistry.NewRegistriesClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	cloud.CacheRulesClient, err = armcontainerregistry.NewCacheRulesClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache rules client: %w", err)
	}

	cloud.PrivateEndpointClient, err = armnetwork.NewPrivateEndpointsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create private endpoint client: %w", err)
	}

	cloud.PrivateZonesClient, err = armprivatedns.NewPrivateZonesClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create private dns zones client: %w", err)
	}

	cloud.VirutalNetworkLinksClient, err = armprivatedns.NewVirtualNetworkLinksClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual network links client: %w", err)
	}

	cloud.RecordSetClient, err = armprivatedns.NewRecordSetsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create record set client: %w", err)
	}

	cloud.PrivateDNSZoneGroup, err = armnetwork.NewPrivateDNSZoneGroupsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create private dns zone group client: %w", err)
	}

	cloud.SecurityGroup, err = armnetwork.NewSecurityGroupsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create security group client: %w", err)
	}

	cloud.Subnet, err = armnetwork.NewSubnetsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create subnet client: %w", err)
	}

	cloud.AKS, err = armcontainerservice.NewManagedClustersClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create aks client: %w", err)
	}

	cloud.Maintenance, err = armcontainerservice.NewMaintenanceConfigurationsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create maintenance client: %w", err)
	}

	cloud.NetworkInterfaces, err = armnetwork.NewInterfacesClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create network interfaces client: %w", err)
	}

	cloud.VMSS, err = armcompute.NewVirtualMachineScaleSetsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vmss client: %w", err)
	}

	cloud.VMSSVM, err = armcompute.NewVirtualMachineScaleSetVMsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vmss vm client: %w", err)
	}

	cloud.VMs, err = armcompute.NewVirtualMachinesClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vms client: %w", err)
	}

	cloud.Images, err = armcompute.NewImagesClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create images client: %w", err)
	}

	cloud.Snapshots, err = armcompute.NewSnapshotsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create snapshots client: %w", err)
	}

	cloud.GalleryImages, err = armcompute.NewGalleryImagesClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create gallery images client: %w", err)
	}

	cloud.GalleryImageVersions, err = armcompute.NewGalleryImageVersionsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create gallery image versions client: %w", err)
	}

	cloud.Resource, err = armresources.NewClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create resource client: %w", err)
	}

	cloud.ResourceGroup, err = armresources.NewResourceGroupsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create resource group client: %w", err)
	}

	cloud.VNet, err = armnetwork.NewVirtualNetworksClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vnet client: %w", err)
	}

	cloud.AzureFirewall, err = armnetwork.NewAzureFirewallsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create firewall client: %w", err)
	}

	cloud.Blob, err = azblob.NewClient(Config.BlobStorageAccountURL(), credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create blob container client: %w", err)
	}

	cloud.StorageContainers, err = armstorage.NewBlobContainersClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create blob container client: %w", err)
	}

	cloud.RoleAssignments, err = armauthorization.NewRoleAssignmentsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create role assignment client: %w", err)
	}

	cloud.UserAssignedIdentities, err = armmsi.NewUserAssignedIdentitiesClient(Config.SubscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create user assigned identities client: %w", err)
	}

	cloud.StorageAccounts, err = armstorage.NewAccountsClient(Config.SubscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create storage accounts client: %w", err)
	}

	cloud.VMSSVMRunCommands, err = armcompute.NewVirtualMachineScaleSetVMRunCommandsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vmss vm run command client: %w", err)
	}

	cloud.VMExtensionImages, err = armcompute.NewVirtualMachineExtensionImagesClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vm extension images client: %w", err)
	}

	// Ensure the gallery exists
	cloud.Galleries, err = armcompute.NewGalleriesClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create galleries client: %w", err)
	}

	cloud.Credential = credential
	cloud.ArmOptions = opts

	return cloud, nil
}

// UploadAndGetLink uploads the data to the blob storage and returns the signed link to download the blob
// If the blob already exists, it will be overwritten
func (a *AzureClient) UploadAndGetLink(ctx context.Context, blobName string, file *os.File) (string, error) {
	_, err := a.Blob.UploadFile(ctx, Config.BlobContainer, blobName, file, nil)
	if err != nil {
		return "", fmt.Errorf("upload blob: %w", err)
	}

	// is there a better way?
	return fmt.Sprintf("%s/%s/%s", Config.BlobStorageAccountURL(), Config.BlobContainer, blobName), nil
}

// UploadAndGetSignedLink uploads the data to the blob storage and returns the signed link to download the blob
// If the blob already exists, it will be overwritten
func (a *AzureClient) UploadAndGetSignedLink(ctx context.Context, blobName string, file *os.File) (string, error) {
	_, err := a.Blob.UploadFile(ctx, Config.BlobContainer, blobName, file, nil)
	if err != nil {
		return "", fmt.Errorf("upload blob: %w", err)
	}

	udc, err := a.Blob.ServiceClient().GetUserDelegationCredential(ctx, service.KeyInfo{
		Expiry: to.Ptr(time.Now().Add(time.Hour).UTC().Format(sas.TimeFormat)),
		Start:  to.Ptr(time.Now().UTC().Format(sas.TimeFormat)),
	}, nil)
	if err != nil {
		return "", fmt.Errorf("get user delegation credential: %w", err)
	}

	sig, err := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		ExpiryTime:    time.Now().Add(time.Hour).UTC(),
		Permissions:   to.Ptr(sas.BlobPermissions{Read: true}).String(),
		ContainerName: Config.BlobContainer,
		BlobName:      blobName,
	}.SignWithUserDelegation(udc)
	if err != nil {
		return "", fmt.Errorf("sign blob: %w", err)
	}

	return fmt.Sprintf("%s/%s/%s?%s", Config.BlobStorageAccountURL(), Config.BlobContainer, blobName, sig.Encode()), nil
}

func (a *AzureClient) CreateVMManagedIdentity(ctx context.Context, identityLocation string) (string, error) {
	identity, err := a.UserAssignedIdentities.CreateOrUpdate(ctx, ResourceGroupName(identityLocation), VMIdentityName, armmsi.Identity{
		Location: to.Ptr(identityLocation),
	}, nil)
	if err != nil {
		return "", fmt.Errorf("create managed identity: %w", err)
	}

	// NOTE: We are not creating new storage account per location, we will use the one
	// that's already created in the default location.
	err = a.createBlobStorageAccount(ctx)
	if err != nil {
		return "", err
	}
	err = a.createBlobStorageContainer(ctx)
	if err != nil {
		return "", err
	}

	if err := a.assignRolesToVMIdentity(ctx, identity.Properties.PrincipalID); err != nil {
		return "", err
	}
	return *identity.Properties.ClientID, nil
}

func (a *AzureClient) createBlobStorageAccount(ctx context.Context) error {
	poller, err := a.StorageAccounts.BeginCreate(ctx, ResourceGroupName(Config.DefaultLocation), Config.BlobStorageAccount(), armstorage.AccountCreateParameters{
		Kind:     to.Ptr(armstorage.KindStorageV2),
		Location: to.Ptr(Config.DefaultLocation),
		SKU: &armstorage.SKU{
			Name: to.Ptr(armstorage.SKUNameStandardLRS),
		},
		Properties: &armstorage.AccountPropertiesCreateParameters{
			AllowBlobPublicAccess: to.Ptr(false),
			AllowSharedKeyAccess:  to.Ptr(false),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("create storage account: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("create storage account: %w", err)
	}
	return nil
}

func (a *AzureClient) createBlobStorageContainer(ctx context.Context) error {
	_, err := a.StorageContainers.Create(ctx, ResourceGroupName(Config.DefaultLocation), Config.BlobStorageAccount(), Config.BlobContainer, armstorage.BlobContainer{}, nil)
	if err != nil {
		return fmt.Errorf("create blob container: %w", err)
	}
	return nil
}

func (a *AzureClient) assignRolesToVMIdentity(ctx context.Context, principalID *string) error {
	scope := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Storage/storageAccounts/%s", Config.SubscriptionID, ResourceGroupName(Config.DefaultLocation), Config.BlobStorageAccount())
	// Role assignment requires uid to be provided
	uid := uuid.New().String()
	_, err := a.RoleAssignments.Create(ctx, scope, uid, armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID: principalID,
			// built-in "Storage Blob Data Contributor" role
			// https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles
			RoleDefinitionID: to.Ptr("/providers/Microsoft.Authorization/roleDefinitions/ba92f5b4-2d11-453d-a403-e96b0029c9fe"),
		},
	}, nil)
	var respError *azcore.ResponseError
	if err != nil {
		// if the role assignment already exists, ignore the error
		if errors.As(err, &respError) && respError.StatusCode == http.StatusConflict {
			return nil
		}
		return fmt.Errorf("assign Storage Blob Data Contributor role: %w", err)
	}
	return nil
}

func (a *AzureClient) LatestSIGImageVersionByTag(ctx context.Context, image *Image, tagName, tagValue, location string) (VHDResourceID, error) {
	logf(ctx, "Looking up images in %s", image.azurePortalImageUrl())

	imagesClient, imagesClientErr := armcompute.NewGalleryImagesClient(image.Gallery.SubscriptionID, a.Credential, a.ArmOptions)
	if imagesClientErr != nil {
		return "", fmt.Errorf("failed to create a new images client: %v", imagesClientErr)
	}

	_, getImageError := imagesClient.Get(ctx, image.Gallery.ResourceGroupName, image.Gallery.Name, image.Name, &armcompute.GalleryImagesClientGetOptions{})
	if getImageError != nil {
		return "", fmt.Errorf("image does not exist in galery: %v", getImageError)
	}

	imageVersionsClient, imageVersionsClientErr := armcompute.NewGalleryImageVersionsClient(image.Gallery.SubscriptionID, a.Credential, a.ArmOptions)
	if imageVersionsClientErr != nil {
		return "", fmt.Errorf("failed to create a new image versions client: %v", imageVersionsClientErr)
	}

	pager := imageVersionsClient.NewListByGalleryImagePager(image.Gallery.ResourceGroupName, image.Gallery.Name, image.Name, nil)
	// this is ugly. The pager doesn't have any error capability so we can't tell if the image gallery exists or not. This case should be caught by the code above, but who knows.
	var hasAny bool = false
	var latestVersion *armcompute.GalleryImageVersion
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get next page: %w", err)
		}
		hasAny = true
		versions := page.Value
		for _, version := range versions {
			// skip images tagged with the no-selection tag, indicating they
			// shouldn't be selected dynmically for running abe2e scenarios
			if _, ok := version.Tags[noSelectionTagName]; ok {
				logf(ctx, "Skipping version %s as it has no selection tag %s", *version.ID, noSelectionTagName)
				continue
			}

			if tagName != "" {
				var tag *string
				var ok bool
				// Case-insensitive tag lookup
				for k, v := range version.Tags {
					if strings.EqualFold(k, tagName) {
						tag = v
						ok = true
						break
					}
				}
				if !ok || tag == nil || *tag != tagValue {
					continue
				}
			}

			if *version.Properties.ProvisioningState != armcompute.GalleryProvisioningStateSucceeded && *version.Properties.ProvisioningState != armcompute.GalleryProvisioningStateUpdating {
				logf(ctx, "Skipping version %s with tag %s=%s due to %s", *version.ID, tagName, tagValue, err)
				continue
			}

			if latestVersion == nil || version.Properties.PublishingProfile.PublishedDate.After(*latestVersion.Properties.PublishingProfile.PublishedDate) {
				latestVersion = version
			}
		}
	}
	if !hasAny {
		return "", fmt.Errorf("no versions found in gallery - likely image or gallery don't exist: %s", image.azurePortalImageUrl())
	}

	if latestVersion == nil {
		logf(ctx, "Could not find VHD with tag %s=%s in %s",
			tagName,
			tagValue,
			image.azurePortalImageUrl())
		return "", ErrNotFound
	}

	if err := a.ensureReplication(ctx, image, latestVersion, location); err != nil {
		return "", fmt.Errorf("failed ensuring image replication: %w", err)
	}

	return VHDResourceID(*latestVersion.ID), nil
}

func (a *AzureClient) ensureReplication(ctx context.Context, image *Image, version *armcompute.GalleryImageVersion, location string) error {
	// Wait for any ongoing update operations to complete first
	if err := a.waitForVersionOperationCompletion(ctx, image, version); err != nil {
		return fmt.Errorf("waiting for version operation completion: %w", err)
	}

	if replicatedToCurrentRegion(version, location) {
		logf(ctx, "Image version %s is already in region %s", *version.ID, location)
		return nil
	}
	regions := make([]string, 0, len(version.Properties.PublishingProfile.TargetRegions))
	for _, targetRegion := range version.Properties.PublishingProfile.TargetRegions {
		regions = append(regions, *targetRegion.Name)
	}
	logf(ctx, "##vso[task.logissue type=warning;]Replicating to region %s, available regions: %s, image version %s", location, strings.Join(regions, ", "), *version.ID)

	start := time.Now() // Record the start time
	err := a.replicateImageVersionToCurrentRegion(ctx, image, version, location)
	elapsed := time.Since(start) // Calculate the elapsed time

	toolkit.LogDuration(ctx, elapsed, 3*time.Minute, fmt.Sprintf("Replication took: %s (%s)", toolkit.FormatDuration(elapsed), *version.ID))

	return err
}

func (a *AzureClient) waitForVersionOperationCompletion(ctx context.Context, image *Image, version *armcompute.GalleryImageVersion) error {
	// If not in updating state, no need to wait
	if *version.Properties.ProvisioningState != armcompute.GalleryProvisioningStateUpdating {
		return nil
	}

	logf(ctx, "Image version %s is in 'Updating' state, waiting for operation to complete", *version.ID)

	imgVersionClient, err := armcompute.NewGalleryImageVersionsClient(image.Gallery.SubscriptionID, a.Credential, a.ArmOptions)
	if err != nil {
		return fmt.Errorf("create a new image version client: %v", err)
	}

	// Use the standard wait.PollUntilContextTimeout helper used throughout the codebase
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		// Get the latest version state using the existing client
		resp, err := imgVersionClient.Get(ctx, image.Gallery.ResourceGroupName, image.Gallery.Name, image.Name, *version.Name, nil)
		if err != nil {
			// Return error to stop polling on permanent errors
			return false, fmt.Errorf("get image version during wait: %w", err)
		}

		currentState := *resp.Properties.ProvisioningState
		logf(ctx, "Image version %s current state: %s", *version.ID, currentState)

		// Check if operation completed
		if currentState != armcompute.GalleryProvisioningStateUpdating {
			if currentState == armcompute.GalleryProvisioningStateSucceeded {
				logf(ctx, "Image version %s operation completed successfully", *version.ID)
				// Update the version object with the latest state
				*version = resp.GalleryImageVersion
				return true, nil // Done successfully
			} else {
				// Operation failed
				return false, fmt.Errorf("image version %s operation failed with state: %s", *version.ID, currentState)
			}
		}

		// Still updating, continue polling
		return false, nil
	})

	if err != nil {
		return fmt.Errorf("waiting for image version operation completion: %w", err)
	}

	return nil
}

func (a *AzureClient) replicateImageVersionToCurrentRegion(ctx context.Context, image *Image, version *armcompute.GalleryImageVersion, location string) error {
	galleryImageVersion, err := armcompute.NewGalleryImageVersionsClient(image.Gallery.SubscriptionID, a.Credential, a.ArmOptions)
	if err != nil {
		return fmt.Errorf("create a new images client: %v", err)
	}
	version.Properties.PublishingProfile.TargetRegions = append(version.Properties.PublishingProfile.TargetRegions, &armcompute.TargetRegion{
		Name:                 &location,
		RegionalReplicaCount: to.Ptr[int32](1),
		StorageAccountType:   to.Ptr(armcompute.StorageAccountTypeStandardLRS),
	})

	resp, err := galleryImageVersion.BeginCreateOrUpdate(ctx, image.Gallery.ResourceGroupName, image.Gallery.Name, image.Name, *version.Name, *version, nil)
	if err != nil {
		return fmt.Errorf("begin updating image version target regions: %w", err)
	}
	if _, err := resp.PollUntilDone(ctx, DefaultPollUntilDoneOptions); err != nil {
		return fmt.Errorf("updating image version target regions: %w", err)
	}

	return nil
}

func (a *AzureClient) EnsureSIGImageVersion(ctx context.Context, image *Image, location string) (VHDResourceID, error) {
	galleryImageVersion, err := armcompute.NewGalleryImageVersionsClient(image.Gallery.SubscriptionID, a.Credential, a.ArmOptions)
	if err != nil {
		return "", fmt.Errorf("create a new images client: %v", err)
	}
	logf(ctx, "Looking up images for gallery subscription %s resource group %s gallery name %s image name %s version %s ",
		image.Gallery.SubscriptionID,
		image.Gallery.ResourceGroupName,
		image.Gallery.Name,
		image.Name,
		image.Version)

	resp, err := galleryImageVersion.Get(ctx, image.Gallery.ResourceGroupName, image.Gallery.Name, image.Name, image.Version, nil)
	if err != nil {
		return "", fmt.Errorf("getting live image version info: %w", err)
	}

	liveVersion := &resp.GalleryImageVersion
	if *liveVersion.Properties.ProvisioningState != armcompute.GalleryProvisioningStateSucceeded && *liveVersion.Properties.ProvisioningState != armcompute.GalleryProvisioningStateUpdating {
		return "", fmt.Errorf("unexpected provisioning state: %q", *liveVersion.Properties.ProvisioningState)
	}

	if err := a.ensureReplication(ctx, image, liveVersion, location); err != nil {
		return "", fmt.Errorf("Failed ensuring image replication: %w", err)
	}

	return VHDResourceID(*resp.ID), nil
}

func DefaultRetryOpts() policy.RetryOptions {
	return policy.RetryOptions{
		MaxRetries: 3,
		RetryDelay: time.Second * 5,
		StatusCodes: []int{
			http.StatusRequestTimeout,      // 408
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
			http.StatusNotFound,            // 404
		},
	}
}

func replicatedToCurrentRegion(version *armcompute.GalleryImageVersion, location string) bool {
	for _, targetRegion := range version.Properties.PublishingProfile.TargetRegions {
		if strings.EqualFold(strings.ReplaceAll(*targetRegion.Name, " ", ""), location) {
			return true
		}
	}
	return false
}

// DeleteSIGImageVersion deletes a SIG image version
func (a *AzureClient) DeleteSIGImageVersion(ctx context.Context, galleryResourceGroup, galleryName, imageName, version string) {
	// Ignore errors, don't need to wait for the deletion to complete
	_, _ = a.GalleryImageVersions.BeginDelete(ctx, galleryResourceGroup, galleryName, imageName, version, nil)
}

// DeleteDisk deletes a managed disk
func (a *AzureClient) DeleteDisk(ctx context.Context, resourceGroupName, diskName string) error {
	disks, err := armcompute.NewDisksClient(Config.SubscriptionID, a.Credential, a.ArmOptions)
	if err != nil {
		return fmt.Errorf("failed to create disks client: %w", err)
	}

	deleteOp, err := disks.BeginDelete(ctx, resourceGroupName, diskName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete disk: %w", err)
	}

	_, err = deleteOp.PollUntilDone(ctx, DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("failed to complete disk deletion: %w", err)
	}

	return nil
}

// DeleteSnapshot deletes a disk snapshot
func (a *AzureClient) DeleteSnapshot(ctx context.Context, resourceGroupName, snapshotName string) error {
	deleteOp, err := a.Snapshots.BeginDelete(ctx, resourceGroupName, snapshotName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	_, err = deleteOp.PollUntilDone(ctx, DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("failed to complete snapshot deletion: %w", err)
	}

	return nil
}

// GetLatestVMExtensionImageVersion lists VM extension images for a given extension name and returns the latest version.
// This is equivalent to: az vm extension image list -n Compute.AKS.Linux.AKSNode --latest
func (a *AzureClient) GetLatestVMExtensionImageVersion(ctx context.Context, location, extType, extPublisher string) (string, error) {
	// List extension versions
	resp, err := a.VMExtensionImages.ListVersions(ctx, location, extPublisher, extType, &armcompute.VirtualMachineExtensionImagesClientListVersionsOptions{})
	if err != nil {
		return "", fmt.Errorf("listing extension versions: %w", err)
	}

	if len(resp.VirtualMachineExtensionImageArray) == 0 {
		return "", fmt.Errorf("no extension versions found")
	}

	version := make([]VMExtenstionVersion, len(resp.VirtualMachineExtensionImageArray))
	for i, ext := range resp.VirtualMachineExtensionImageArray {
		version[i] = parseVersion(ext)
	}

	sort.Slice(version, func(i, j int) bool {
		return version[i].Less(version[j])
	})

	return *version[len(version)-1].Original.Name, nil
}

// VMExtenstionVersion represents a parsed version of a VM extension image.
type VMExtenstionVersion struct {
	Original *armcompute.VirtualMachineExtensionImage
	Major    int
	Minor    int
	Patch    int
}

// parseVersion parses the version from a VM extension image name, which can be in the format 1.151, 1.0.1, etc.
// You can find all the versions of a specific VM extension by running:
// az vm extension image list -n Compute.AKS.Linux.AKSNode
func parseVersion(v *armcompute.VirtualMachineExtensionImage) VMExtenstionVersion {
	// Split by dots
	parts := strings.Split(*v.Name, ".")

	version := VMExtenstionVersion{Original: v}

	if len(parts) >= 1 {
		if major, err := strconv.Atoi(parts[0]); err == nil {
			version.Major = major
		}
	}
	if len(parts) >= 2 {
		if minor, err := strconv.Atoi(parts[1]); err == nil {
			version.Minor = minor
		}
	}
	if len(parts) >= 3 {
		if patch, err := strconv.Atoi(parts[2]); err == nil {
			version.Patch = patch
		}
	}

	return version
}

func (v VMExtenstionVersion) Less(other VMExtenstionVersion) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}
