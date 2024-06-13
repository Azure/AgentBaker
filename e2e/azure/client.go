package azure

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/go-armbalancer"
)

type Client struct {
	Core          *azcore.Client
	VMSS          *armcompute.VirtualMachineScaleSetsClient
	VMSSVM        *armcompute.VirtualMachineScaleSetVMsClient
	VNet          *armnetwork.VirtualNetworksClient
	Resource      *armresources.Client
	ResourceGroup *armresources.ResourceGroupsClient
	AKS           *armcontainerservice.ManagedClustersClient
	SecurityGroup *armnetwork.SecurityGroupsClient
	Subnet        *armnetwork.SubnetsClient
}

func MustNewAzureClient(subscription string) *Client {
	client, err := NewAzureClient(subscription)
	if err != nil {
		panic(err)
	}
	return client

}

func NewAzureClient(subscription string) (*Client, error) {
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
		},
	}
	opts.Retry = DefaultRetryOpts()

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
	}
	clOpts.Retry = DefaultRetryOpts()

	// purely for telemetry, entirely unused today
	coreClient, err := azcore.NewClient("agentbakere2e.e2e_test", "v0.0.0", plOpts, clOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create core client: %w", err)
	}

	securityGroupClient, err := armnetwork.NewSecurityGroupsClient(subscription, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create security group client: %w", err)
	}

	subnetClient, err := armnetwork.NewSubnetsClient(subscription, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subnet client: %w", err)
	}

	aksClient, err := armcontainerservice.NewManagedClustersClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create aks client: %w", err)
	}

	vmssClient, err := armcompute.NewVirtualMachineScaleSetsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create vmss client: %w", err)
	}

	vmssVMClient, err := armcompute.NewVirtualMachineScaleSetVMsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create vmss vm client: %w", err)
	}

	resourceClient, err := armresources.NewClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource client: %w", err)
	}

	resourceGroupClient, err := armresources.NewResourceGroupsClient(subscription, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource group client: %w", err)
	}

	vnetClient, err := armnetwork.NewVirtualNetworksClient(subscription, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create vnet client: %w", err)
	}

	var cloud = &Client{
		Core:          coreClient,
		AKS:           aksClient,
		Resource:      resourceClient,
		ResourceGroup: resourceGroupClient,
		VMSS:          vmssClient,
		VMSSVM:        vmssVMClient,
		VNet:          vnetClient,
		SecurityGroup: securityGroupClient,
		Subnet:        subnetClient,
	}

	return cloud, nil
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

func (c *Client) CreateCluster(
	ctx context.Context,
	resourceGroupName string,
	config *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	pollerResp, err := c.AKS.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		*config.Name,
		*config,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to begin aks cluster creation: %w", err)
	}

	clusterResp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for aks cluster creation %w", err)
	}

	return &clusterResp.ManagedCluster, nil
}

func (c *Client) DeleteCluster(ctx context.Context, resourceGroupName, clusterName string) error {
	poller, err := c.AKS.BeginDelete(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to start aks cluster %q deletion: %w", clusterName, err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for aks cluster %q deletion: %w", clusterName, err)
	}

	return nil
}

func (c *Client) EnsureResourceGroup(ctx context.Context, resourceGroupName string, location string) error {
	log.Printf("Ensuring resource group %q exists", resourceGroupName)

	rgExists, err := c.IsExistingResourceGroup(ctx, resourceGroupName)
	if err != nil {
		return err
	}

	if !rgExists {
		_, err = c.ResourceGroup.CreateOrUpdate(
			ctx,
			resourceGroupName,
			armresources.ResourceGroup{
				Location: to.Ptr(location),
				Name:     to.Ptr(resourceGroupName),
			},
			nil)

		if err != nil {
			return fmt.Errorf("failed to create RG %q: %w", resourceGroupName, err)
		}
	}
	return nil
}

func (c *Client) IsExistingResourceGroup(ctx context.Context, resourceGroupName string) (bool, error) {
	rgExistence, err := c.ResourceGroup.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get RG %q: %w", resourceGroupName, err)
	}
	return rgExistence.Success, nil
}

func (c *Client) CreateOrUpdateSecurityGroup(ctx context.Context, resourceGroupName, securityGroupName string, securityGroup armnetwork.SecurityGroup) (armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error) {
	poller, err := c.SecurityGroup.BeginCreateOrUpdate(ctx, resourceGroupName, securityGroupName, securityGroup, nil)
	if err != nil {
		return armnetwork.SecurityGroupsClientCreateOrUpdateResponse{}, err
	}
	return poller.PollUntilDone(ctx, nil)

}

func (c *Client) CreateOrUpdateSubnet(ctx context.Context, resourceGroupName, vnetName, subnetName string, subnetParameters armnetwork.Subnet) (armnetwork.SubnetsClientCreateOrUpdateResponse, error) {
	poller, err := c.Subnet.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, subnetName, subnetParameters, nil)
	if err != nil {
		return armnetwork.SubnetsClientCreateOrUpdateResponse{}, err
	}
	return poller.PollUntilDone(ctx, nil)
}
