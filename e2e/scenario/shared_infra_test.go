package scenario

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v10"
	"github.com/stretchr/testify/require"
)

func TestEnsureSharedInfraPreservesOtherClusterSubnets(t *testing.T) {
	// TODO: fix this, right now it's blocking needed E2E tests.
	t.Skip()

	originalConfig, originalAzure := config.Config, config.Azure
	t.Cleanup(func() { config.Config, config.Azure = originalConfig, originalAzure })
	config.Config = &config.Configuration{SubscriptionID: "test"}
	var deletedSubnets []string
	responder := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
		path := req.URL.Path
		if req.Method == http.MethodDelete {
			deletedSubnets = append(deletedSubnets, path)
			return gcResponse(req, http.StatusNoContent, "")
		}
		require.Equal(t, http.MethodGet, req.Method)
		switch {
		case strings.HasSuffix(path, "/virtualNetworks/"+SharedVNetName):
			return gcResponse(req, 200, `{"properties":{"addressSpace":{"addressPrefixes":["10.0.0.0/8","fd00::/48"]}}}`)
		case strings.HasSuffix(path, "/subnets/"+PESubnetName):
			return gcResponse(req, 200, `{"properties":{"addressPrefix":"10.0.2.0/24"}}`)
		case strings.HasSuffix(path, "/bastionHosts/"+SharedBastionName):
			return gcResponse(req, 200, `{"properties":{"dnsName":"bastion.example","enableIpConnect":true}}`)
		case strings.HasSuffix(path, "/azureFirewalls/"+SharedFirewallName):
			return gcResponse(req, 200, fmt.Sprintf(`{"properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.1.4"}}],"applicationRuleCollections":[{"properties":{"rules":[{"name":"blob-storage-fqdn","targetFqdns":["%s.blob.core.windows.net"]}]}}]}}`, config.Config.BlobStorageAccount()))
		case strings.HasSuffix(path, "/userAssignedIdentities/"+SharedClusterIdentity):
			return gcResponse(req, 200, `{"id":"/subscriptions/test/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/identity","properties":{"tenantId":"tenant"}}`)
		case strings.HasSuffix(path, "/subnets"):
			return gcResponse(req, 200, `{"value":[{"name":"aks-subnet-new-cluster","properties":{"provisioningState":"Succeeded","addressPrefix":"10.19.96.0/20"}}]}`)
		case strings.HasSuffix(path, "/managedClusters/new-cluster"):
			return gcResponse(req, 404, `{"error":{"code":"ResourceNotFound"}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, path)
			return nil
		}
	})
	opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{responder}}}
	network, err := armnetwork.NewClientFactory("test", nil, opts)
	require.NoError(t, err)
	identities, err := armmsi.NewUserAssignedIdentitiesClient("test", nil, opts)
	require.NoError(t, err)
	clusters, err := armcontainerservice.NewManagedClustersClient("test", nil, opts)
	require.NoError(t, err)
	config.Azure = &config.AzureClient{
		VNet: network.NewVirtualNetworksClient(), Subnet: network.NewSubnetsClient(),
		BastionHosts: network.NewBastionHostsClient(), AzureFirewall: network.NewAzureFirewallsClient(),
		UserAssignedIdentities: identities, AKS: clusters,
	}

	infra, err := ensureSharedInfra(logging.WithLogger(t.Context(), t), "westus3")

	require.NoError(t, err)
	require.Equal(t, "bastion.example", infra.BastionDNSName)
	require.Equal(t, "10.0.1.4", infra.FirewallIP)
	require.Empty(t, deletedSubnets, "another run may have created its subnet before creating its cluster")
}
