package config

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
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
	"github.com/Azure/go-armbalancer"
	"github.com/google/uuid"
)

type AzureClient struct {
	AKS                       *armcontainerservice.ManagedClustersClient
	Blob                      *azblob.Client
	StorageContainers         *armstorage.BlobContainersClient
	CacheRulesClient          *armcontainerregistry.CacheRulesClient
	Core                      *azcore.Client
	Credential                *azidentity.DefaultAzureCredential
	GalleryImageVersion       *armcompute.GalleryImageVersionsClient
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
}

func mustNewAzureClient(subscription string) *AzureClient {
	client, err := NewAzureClient(subscription)
	if err != nil {
		panic(err)
	}
	return client

}

func NewAzureClient(subscription string) (*AzureClient, error) {
	httpClient := &http.Client{
		// use a bunch of connections for load balancing
		// ensure all timeouts are defined and reasonable
		// ensure TLS1.2+ and HTTP2
		Transport: armbalancer.New(armbalancer.Options{
			PoolSize: 100,
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
		}),
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

	cloud.RegistriesClient, err = armcontainerregistry.NewRegistriesClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	cloud.CacheRulesClient, err = armcontainerregistry.NewCacheRulesClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache rules client: %w", err)
	}

	cloud.PrivateEndpointClient, err = armnetwork.NewPrivateEndpointsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create private endpoint client: %w", err)
	}

	cloud.PrivateZonesClient, err = armprivatedns.NewPrivateZonesClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create private dns zones client: %w", err)
	}

	cloud.VirutalNetworkLinksClient, err = armprivatedns.NewVirtualNetworkLinksClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual network links client: %w", err)
	}

	cloud.RecordSetClient, err = armprivatedns.NewRecordSetsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create record set client: %w", err)
	}

	cloud.PrivateDNSZoneGroup, err = armnetwork.NewPrivateDNSZoneGroupsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create private dns zone group client: %w", err)
	}

	cloud.SecurityGroup, err = armnetwork.NewSecurityGroupsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create security group client: %w", err)
	}

	cloud.Subnet, err = armnetwork.NewSubnetsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create subnet client: %w", err)
	}

	cloud.AKS, err = armcontainerservice.NewManagedClustersClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create aks client: %w", err)
	}

	cloud.Maintenance, err = armcontainerservice.NewMaintenanceConfigurationsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create maintenance client: %w", err)
	}

	cloud.VMSS, err = armcompute.NewVirtualMachineScaleSetsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vmss client: %w", err)
	}

	cloud.VMSSVM, err = armcompute.NewVirtualMachineScaleSetVMsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vmss vm client: %w", err)
	}

	cloud.Resource, err = armresources.NewClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create resource client: %w", err)
	}

	cloud.ResourceGroup, err = armresources.NewResourceGroupsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create resource group client: %w", err)
	}

	cloud.VNet, err = armnetwork.NewVirtualNetworksClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create vnet client: %w", err)
	}

	cloud.GalleryImageVersion, err = armcompute.NewGalleryImageVersionsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create a new images client: %v", err)
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

	cloud.Credential = credential

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

	if err := a.assignReaderRoleToBlobStorage(ctx, identity.Properties.PrincipalID); err != nil {
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

func (a *AzureClient) assignReaderRoleToBlobStorage(ctx context.Context, principalID *string) error {
	scope := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Storage/storageAccounts/%s", Config.SubscriptionID, ResourceGroupName, Config.BlobStorageAccount())
	// Role assignment requires uid to be provided
	uid := uuid.New().String()
	_, err := a.RoleAssignments.Create(ctx, scope, uid, armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID: principalID,
			// built-in "Storage Blob Data Reader" role
			RoleDefinitionID: to.Ptr("/providers/Microsoft.Authorization/roleDefinitions/2a2b9908-6ea1-4ae2-8e65-a410df84e7d1"),
		},
	}, nil)
	var respError *azcore.ResponseError
	if err != nil {
		// if the role assignment already exists, ignore the error
		if errors.As(err, &respError) && respError.StatusCode == http.StatusConflict {
			return nil
		}
		return fmt.Errorf("assign reader role: %w", err)
	}
	return nil

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
