package config

import (
	"context"
	"crypto/tls"
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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/Azure/go-armbalancer"
)

type AzureClient struct {
	AKS                       *armcontainerservice.ManagedClustersClient
	Blob                      *azblob.Client
	Core                      *azcore.Client
	Credential                *azidentity.DefaultAzureCredential
	GalleryImageVersion       *armcompute.GalleryImageVersionsClient
	Maintenance               *armcontainerservice.MaintenanceConfigurationsClient
	PrivateDNSZoneGroup       *armnetwork.PrivateDNSZoneGroupsClient
	PrivateEndpointClient     *armnetwork.PrivateEndpointsClient
	PrivateZonesClient        *armprivatedns.PrivateZonesClient
	RecordSetClient           *armprivatedns.RecordSetsClient
	Resource                  *armresources.Client
	ResourceGroup             *armresources.ResourceGroupsClient
	SecurityGroup             *armnetwork.SecurityGroupsClient
	Subnet                    *armnetwork.SubnetsClient
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

	cloud.Blob, err = azblob.NewClient(Config.BlobStorageAccount, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("create blob container client: %w", err)
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

	return fmt.Sprintf("%s/%s/%s?%s", Config.BlobStorageAccount, Config.BlobContainer, blobName, sig.Encode()), nil
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
