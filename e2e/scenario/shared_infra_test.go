// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package scenario

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v10"
	"github.com/stretchr/testify/require"
)

func TestEnsureSharedInfraDoesNotDeleteAnotherRunnersSubnet(t *testing.T) {
	for _, state := range []string{"Updating", "Succeeded"} {
		t.Run(state, func(t *testing.T) {
			oldAzure := config.Azure
			t.Cleanup(func() { config.Azure = oldAzure })
			var deletes int
			options := &arm.ClientOptions{ClientOptions: policy.ClientOptions{
				Retry: policy.RetryOptions{MaxRetries: -1},
				Transport: galleryCleanupTransport(func(req *http.Request) (*http.Response, error) {
					status, body := http.StatusOK, ""
					switch {
					case req.Method == http.MethodDelete:
						deletes++
						status = http.StatusNoContent
					case strings.HasSuffix(req.URL.Path, "/virtualNetworks/"+SharedVNetName):
						body = `{"properties":{"addressSpace":{"addressPrefixes":["10.0.0.0/8","fd00::/48"]}}}`
					case strings.HasSuffix(req.URL.Path, "/subnets/"+PESubnetName):
						body = `{"name":"abe2e-pe-subnet"}`
					case strings.HasSuffix(req.URL.Path, "/bastionHosts/"+SharedBastionName):
						body = `{"properties":{"dnsName":"fixture.bastion","enableIpConnect":true}}`
					case strings.HasSuffix(req.URL.Path, "/azureFirewalls/"+SharedFirewallName):
						body = fmt.Sprintf(`{"properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.1.4"}}],"applicationRuleCollections":[{"properties":{"rules":[{"name":"blob-storage-fqdn","targetFqdns":[%q]}]}}]}}`,
							config.Config.BlobStorageAccount()+".blob.core.windows.net")
					case strings.HasSuffix(req.URL.Path, "/userAssignedIdentities/"+SharedClusterIdentity):
						body = `{"id":"fixture-identity","properties":{"tenantId":"fixture-tenant"}}`
					case strings.HasSuffix(req.URL.Path, "/subnets"):
						body = fmt.Sprintf(`{"value":[{"name":"aks-subnet-another-run-in-flight","properties":{"addressPrefix":"10.83.48.0/20","provisioningState":%q}}]}`, state)
					case strings.HasSuffix(req.URL.Path, "/managedClusters/another-run-in-flight"):
						status = http.StatusNotFound
						body = `{"error":{"code":"ResourceNotFound","message":"cluster creation has not started yet"}}`
					default:
						t.Fatalf("unexpected shared infrastructure request: %s %s", req.Method, req.URL.Path)
					}
					return &http.Response{
						StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
						Body: io.NopCloser(strings.NewReader(body)), Request: req,
					}, nil
				}),
			}}
			credential := &fake.TokenCredential{}
			client := &config.AzureClient{}
			var err error
			client.VNet, err = armnetwork.NewVirtualNetworksClient("subscription", credential, options)
			require.NoError(t, err)
			client.Subnet, err = armnetwork.NewSubnetsClient("subscription", credential, options)
			require.NoError(t, err)
			client.BastionHosts, err = armnetwork.NewBastionHostsClient("subscription", credential, options)
			require.NoError(t, err)
			client.AzureFirewall, err = armnetwork.NewAzureFirewallsClient("subscription", credential, options)
			require.NoError(t, err)
			client.UserAssignedIdentities, err = armmsi.NewUserAssignedIdentitiesClient("subscription", credential, options)
			require.NoError(t, err)
			client.AKS, err = armcontainerservice.NewManagedClustersClient("subscription", credential, options)
			require.NoError(t, err)
			config.Azure = client

			infra, err := ensureSharedInfra(t.Context(), "westus3")
			require.NoError(t, err)
			require.Equal(t, SharedVNetName, infra.VNetName)
			require.Equal(t, 0, deletes, "another runner may have created the subnet before its cluster resource exists")
		})
	}
}
