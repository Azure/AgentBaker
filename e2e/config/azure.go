package config

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

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
)

type AzureClient struct {
	AKS                       *armcontainerservice.ManagedClustersClient
	Blob                      *azblob.Client
	StorageContainers         *armstorage.BlobContainersClient
	CacheRulesClient          *armcontainerregistry.CacheRulesClient
	Core                      *azcore.Client
	Credential                *azidentity.DefaultAzureCredential
	Maintenance               *armcontainerservice.MaintenanceConfigurationsClient
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
	VNet                      *armnetwork.VirtualNetworksClient
	VirutalNetworkLinksClient *armprivatedns.VirtualNetworkLinksClient
	ArmOptions                *arm.ClientOptions
	VMSSVMRunCommands         *armcompute.VirtualMachineScaleSetVMRunCommandsClient
}

func mustNewAzureClient() *AzureClient {
	client, err := NewAzureClient()
	if err != nil {
		panic(err)
	}
	return client

}

func NewAzureClient() (*AzureClient, error) {
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

	cloud.VMSS, err = armcompute.NewVirtualMachineScaleSetsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vmss client: %w", err)
	}

	cloud.VMSSVM, err = armcompute.NewVirtualMachineScaleSetVMsClient(Config.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vmss vm client: %w", err)
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
		ExpiryTime:    time.Now().Add(time.Hour),
		Permissions:   to.Ptr(sas.BlobPermissions{Read: true}).String(),
		ContainerName: Config.BlobContainer,
		BlobName:      blobName,
	}.SignWithUserDelegation(udc)
	if err != nil {
		return "", fmt.Errorf("sign blob: %w", err)
	}

	return fmt.Sprintf("%s/%s/%s?%s", Config.BlobStorageAccountURL(), Config.BlobContainer, blobName, sig.Encode()), nil
}

func (a *AzureClient) CreateVMManagedIdentity(ctx context.Context) (string, error) {
	identity, err := a.UserAssignedIdentities.CreateOrUpdate(ctx, ResourceGroupName, VMIdentityName, armmsi.Identity{
		Location: to.Ptr(Config.Location),
	}, nil)
	if err != nil {
		return "", fmt.Errorf("create managed identity: %w", err)
	}
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
	poller, err := a.StorageAccounts.BeginCreate(ctx, ResourceGroupName, Config.BlobStorageAccount(), armstorage.AccountCreateParameters{
		Kind:     to.Ptr(armstorage.KindStorageV2),
		Location: &Config.Location,
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
	_, err := a.StorageContainers.Create(ctx, ResourceGroupName, Config.BlobStorageAccount(), Config.BlobContainer, armstorage.BlobContainer{}, nil)
	if err != nil {
		return fmt.Errorf("create blob container: %w", err)
	}
	return nil
}

func (a *AzureClient) assignRolesToVMIdentity(ctx context.Context, principalID *string) error {
	scope := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Storage/storageAccounts/%s", Config.SubscriptionID, ResourceGroupName, Config.BlobStorageAccount())
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

func (a *AzureClient) LatestSIGImageVersionByTag(ctx context.Context, image *Image, tagName, tagValue string) (VHDResourceID, error) {
	galleryImageVersion, err := armcompute.NewGalleryImageVersionsClient(image.Gallery.SubscriptionID, a.Credential, a.ArmOptions)
	if err != nil {
		return "", fmt.Errorf("create a new images client: %v", err)
	}
	pager := galleryImageVersion.NewListByGalleryImagePager(image.Gallery.ResourceGroupName, image.Gallery.Name, image.Name, nil)
	var latestVersion *armcompute.GalleryImageVersion
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get next page: %w", err)
		}
		versions := page.Value
		for _, version := range versions {
			// skip images tagged with the no-selection tag, indicating they
			// shouldn't be selected dynmically for running abe2e scenarios
			if _, ok := version.Tags[noSelectionTagName]; ok {
				continue
			}
			if tagName != "" {
				tag, ok := version.Tags[tagName]
				if !ok || tag == nil || *tag != tagValue {
					continue
				}
			}

			if err := ensureProvisioningState(version); err != nil {
				continue
			}
			if latestVersion == nil || version.Properties.PublishingProfile.PublishedDate.After(*latestVersion.Properties.PublishingProfile.PublishedDate) {
				latestVersion = version
			}
		}
	}
	if latestVersion == nil {
		return "", ErrNotFound
	}

	if err := a.ensureReplication(ctx, image, latestVersion); err != nil {
		return "", fmt.Errorf("ensuring image replication: %w", err)
	}

	return VHDResourceID(*latestVersion.ID), nil
}

func (a *AzureClient) ensureReplication(ctx context.Context, image *Image, version *armcompute.GalleryImageVersion) error {
	if replicatedToCurrentRegion(version) {
		return nil
	}
	return a.replicateImageVersionToCurrentRegion(ctx, image, version)
}

func (a *AzureClient) replicateImageVersionToCurrentRegion(ctx context.Context, image *Image, version *armcompute.GalleryImageVersion) error {
	galleryImageVersion, err := armcompute.NewGalleryImageVersionsClient(image.Gallery.SubscriptionID, a.Credential, a.ArmOptions)
	if err != nil {
		return fmt.Errorf("create a new images client: %v", err)
	}
	version.Properties.PublishingProfile.TargetRegions = append(version.Properties.PublishingProfile.TargetRegions, &armcompute.TargetRegion{
		Name:                 &Config.Location,
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

func (a *AzureClient) EnsureSIGImageVersion(ctx context.Context, image *Image) (VHDResourceID, error) {
	galleryImageVersion, err := armcompute.NewGalleryImageVersionsClient(image.Gallery.SubscriptionID, a.Credential, a.ArmOptions)
	if err != nil {
		return "", fmt.Errorf("create a new images client: %v", err)
	}
	resp, err := galleryImageVersion.Get(ctx, image.Gallery.ResourceGroupName, image.Gallery.Name, image.Name, image.Version, nil)
	if err != nil {
		return "", fmt.Errorf("getting live image version info: %w", err)
	}

	liveVersion := &resp.GalleryImageVersion
	if err := ensureProvisioningState(liveVersion); err != nil {
		return "", fmt.Errorf("ensuring image version provisioning state: %w", err)
	}

	if err := a.ensureReplication(ctx, image, liveVersion); err != nil {
		return "", fmt.Errorf("ensuring image replication: %w", err)
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

func replicatedToCurrentRegion(version *armcompute.GalleryImageVersion) bool {
	for _, targetRegion := range version.Properties.PublishingProfile.TargetRegions {
		if strings.EqualFold(strings.ReplaceAll(*targetRegion.Name, " ", ""), Config.Location) {
			return true
		}
	}
	return false
}

func ensureProvisioningState(version *armcompute.GalleryImageVersion) error {
	if *version.Properties.ProvisioningState != armcompute.GalleryProvisioningStateSucceeded {
		return fmt.Errorf("unexpected provisioning state: %q", *version.Properties.ProvisioningState)
	}
	return nil
}

func (a *AzureClient) CreateVMSSWithRetry(ctx context.Context, t *testing.T, resourceGroupName string, vmssName string, parameters armcompute.VirtualMachineScaleSet) (*armcompute.VirtualMachineScaleSet, error) {
	t.Logf("creating VMSS %s in resource group %s", vmssName, resourceGroupName)
	delay := 5 * time.Second
	retryOn := func(err error) bool {
		var respErr *azcore.ResponseError
		// AllocationFailed sometimes happens for GPU SKUs with limited availability, sometimes retrying helps
		return errors.As(err, &respErr) && respErr.StatusCode == 200 && respErr.ErrorCode == "AllocationFailed"
	}
	attempt := 0
	for {
		attempt++
		vmss, err := a.createVMSS(ctx, resourceGroupName, vmssName, parameters)
		if err == nil {
			t.Logf("created VMSS %s in resource group %s", vmssName, resourceGroupName)
			return vmss, nil
		}

		// not a retryable error
		if !retryOn(err) {
			return nil, err
		}

		if attempt >= 10 {
			return nil, fmt.Errorf("failed to create VMSS after 10 retries: %w", err)
		}

		t.Logf("failed to create VMSS: %v, attempt: %v, retrying in %v", err, attempt, delay)
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(delay):
		}
	}

}

func (a *AzureClient) createVMSS(ctx context.Context, resourceGroupName string, vmssName string, parameters armcompute.VirtualMachineScaleSet) (*armcompute.VirtualMachineScaleSet, error) {
	operation, err := a.VMSS.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		vmssName,
		parameters,
		nil,
	)
	if err != nil {
		return nil, err
	}
	vmssResp, err := operation.PollUntilDone(ctx, DefaultPollUntilDoneOptions)
	if err != nil {
		return nil, err
	}
	return &vmssResp.VirtualMachineScaleSet, nil

}
