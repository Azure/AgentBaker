package azure

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Azure/agentbaker/vhdbuilder/gc/internal/env"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/go-armbalancer"
)

type Client interface {
	ResourceGroups(ctx context.Context) ([]*armresources.ResourceGroup, error)
	BeginDeleteResourceGroup(ctx context.Context, resourceGroupName string) error
}

type client struct {
	resourceGroup *armresources.ResourceGroupsClient
	resources     *armresources.Client
}

var _ Client = (*client)(nil)

func NewClient() (Client, error) {
	httpClient := &http.Client{
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
			Retry: defaultRetryOpts(),
		},
	}

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	cloud := &client{}

	cloud.resourceGroup, err = armresources.NewResourceGroupsClient(env.Variables.SubscriptionID, credential, opts)
	if err != nil {
		return nil, fmt.Errorf("create resource group client: %w", err)
	}

	return cloud, nil
}

func (c *client) ResourceGroups(ctx context.Context) ([]*armresources.ResourceGroup, error) {
	var groups []*armresources.ResourceGroup
	pager := c.resourceGroup.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting next page of resource groups: %w", err)
		}
		groups = append(groups, page.Value...)
	}
	return groups, nil
}

func (c *client) BeginDeleteResourceByID(ctx context.Context, resourceID string) error {
	_, err := c.resources.BeginDeleteByID(ctx, resourceID, "", nil)
	return err
}

func defaultRetryOpts() policy.RetryOptions {
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
