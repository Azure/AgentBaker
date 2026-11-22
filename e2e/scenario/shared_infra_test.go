package scenario

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v10"
	"github.com/stretchr/testify/require"
)

func TestGetFirewallAllowsNTP(t *testing.T) {
	fw := getFirewall(logging.WithLogger(t.Context(), t), "westus3", "subnet-id", "public-ip-id")
	require.Len(t, fw.Properties.NetworkRuleCollections, 1)
	coll := fw.Properties.NetworkRuleCollections[0]
	require.Equal(t, armnetwork.AzureFirewallRCActionTypeAllow, *coll.Properties.Action.Type)
	require.Len(t, coll.Properties.Rules, 1)
	rule := coll.Properties.Rules[0]
	require.Equal(t, []*string{to.Ptr(SharedVNetCIDR)}, rule.SourceAddresses)
	require.Equal(t, []*string{to.Ptr("*")}, rule.DestinationAddresses)
	require.Equal(t, []*string{to.Ptr("123")}, rule.DestinationPorts)
	require.Equal(t, []*armnetwork.AzureFirewallNetworkRuleProtocol{to.Ptr(armnetwork.AzureFirewallNetworkRuleProtocolUDP)}, rule.Protocols)
}

func TestEnsureSharedFirewallReconcilesRules(t *testing.T) {
	originalConfig, originalAzure := config.Config, config.Azure
	t.Cleanup(func() { config.Config, config.Azure = originalConfig, originalAzure })
	config.Config = &config.Configuration{SubscriptionID: "test"}

	for _, tt := range []struct {
		name       string
		mutate     func(*armnetwork.AzureFirewall)
		wantUpdate bool
	}{
		{name: "current rules"},
		{
			name: "legacy firewall without network rules", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) { fw.Properties.NetworkRuleCollections = nil },
		},
		{
			name: "wrong protocol", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.NetworkRuleCollections[0].Properties.Rules[0].Protocols = []*armnetwork.AzureFirewallNetworkRuleProtocol{to.Ptr(armnetwork.AzureFirewallNetworkRuleProtocolTCP)}
			},
		},
		{
			name: "wrong port", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.NetworkRuleCollections[0].Properties.Rules[0].DestinationPorts = []*string{to.Ptr("443")}
			},
		},
		{
			name: "wrong source", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.NetworkRuleCollections[0].Properties.Rules[0].SourceAddresses = []*string{to.Ptr("192.168.0.0/16")}
			},
		},
		{
			name: "restricted destination", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.NetworkRuleCollections[0].Properties.Rules[0].DestinationAddresses = []*string{to.Ptr("192.0.2.1")}
			},
		},
		{
			name: "deny action", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.NetworkRuleCollections[0].Properties.Action.Type = to.Ptr(armnetwork.AzureFirewallRCActionTypeDeny)
			},
		},
		{
			name: "failed with current rules", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ProvisioningState = to.Ptr(armnetwork.ProvisioningStateFailed)
			},
		},
		{
			name: "stale blob rule", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules[1].TargetFqdns = []*string{to.Ptr("old.blob.core.windows.net")}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := logging.WithLogger(t.Context(), t)
			publicIPID := "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Network/publicIPAddresses/existing-pip"
			fw := getFirewall(ctx, "westus3", "subnet-id", publicIPID)
			fw.ID = to.Ptr("/subscriptions/test/resourceGroups/rg/providers/Microsoft.Network/azureFirewalls/" + SharedFirewallName)
			fw.Properties.IPConfigurations[0].Properties.PrivateIPAddress = to.Ptr("10.0.1.4")
			if tt.mutate != nil {
				tt.mutate(fw)
			}

			updates := 0
			responder := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				require.True(t, strings.HasSuffix(req.URL.Path, "/azureFirewalls/"+SharedFirewallName))
				switch req.Method {
				case http.MethodGet:
				case http.MethodPut:
					updates++
					var updated armnetwork.AzureFirewall
					require.NoError(t, json.NewDecoder(req.Body).Decode(&updated))
					require.True(t, firewallAppRulesUpToDate(updated))
					require.True(t, firewallNetworkRulesUpToDate(updated))
					require.Equal(t, publicIPID, *updated.Properties.IPConfigurations[0].Properties.PublicIPAddress.ID)
					updated.ID = fw.ID
					updated.Properties.ProvisioningState = to.Ptr(armnetwork.ProvisioningStateSucceeded)
					updated.Properties.IPConfigurations[0].Properties.PrivateIPAddress = to.Ptr("10.0.1.4")
					fw = &updated
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				body, err := json.Marshal(fw)
				require.NoError(t, err)
				return gcResponse(req, http.StatusOK, string(body))
			})
			opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{responder}}}
			client, err := armnetwork.NewAzureFirewallsClient("test", nil, opts)
			require.NoError(t, err)
			config.Azure = &config.AzureClient{AzureFirewall: client}

			ip, err := ensureSharedFirewall(ctx, "rg", "westus3")
			require.NoError(t, err)
			require.Equal(t, "10.0.1.4", ip)
			if tt.wantUpdate {
				require.Equal(t, 1, updates)
			} else {
				require.Zero(t, updates)
			}
			previousUpdates := updates
			ip, err = ensureSharedFirewall(ctx, "rg", "westus3")
			require.NoError(t, err)
			require.Equal(t, "10.0.1.4", ip)
			require.Equal(t, previousUpdates, updates, "current rules should not trigger another update")
		})
	}
}

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

func TestEnsureSharedBastionRecreatesFailedBastion(t *testing.T) {
	originalConfig, originalAzure := config.Config, config.Azure
	t.Cleanup(func() { config.Config, config.Azure = originalConfig, originalAzure })
	config.Config = &config.Configuration{SubscriptionID: "test", DefaultPollInterval: time.Millisecond}

	for _, tt := range []struct {
		name      string
		state     string
		dnsName   string
		wantCalls []string
		wantErr   string
	}{
		{name: "reuse succeeded", state: "Succeeded", dnsName: "bastion.example", wantCalls: []string{"GET bastion"}},
		{
			name: "recreate failed", state: "Failed",
			wantCalls: []string{"GET bastion", "DELETE bastion", "PUT pip", "PUT bastion"},
		},
		{
			name: "in-progress bastion reports its state", state: "Updating",
			wantCalls: []string{"GET bastion"}, wantErr: "exists in state Updating but has no DNS name",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			deleted := false
			responder := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				resource := "bastion"
				if strings.HasSuffix(req.URL.Path, "/publicIPAddresses/"+SharedBastionPIPName) {
					resource = "pip"
				} else {
					require.True(t, strings.HasSuffix(req.URL.Path, "/bastionHosts/"+SharedBastionName), req.URL.Path)
				}
				calls = append(calls, req.Method+" "+resource)
				switch {
				case req.Method == http.MethodGet:
					body := fmt.Sprintf(`{"properties":{"provisioningState":%q,"enableIpConnect":true}}`, tt.state)
					if tt.dnsName != "" {
						body = fmt.Sprintf(`{"properties":{"provisioningState":%q,"enableIpConnect":true,"dnsName":%q}}`, tt.state, tt.dnsName)
					}
					return gcResponse(req, http.StatusOK, body)
				case req.Method == http.MethodDelete:
					deleted = true
					return gcResponse(req, http.StatusOK, "")
				case req.Method == http.MethodPut && resource == "pip":
					return gcResponse(req, http.StatusOK, `{"id":"/subscriptions/test/resourceGroups/rg/providers/Microsoft.Network/publicIPAddresses/pip","properties":{"provisioningState":"Succeeded"}}`)
				case req.Method == http.MethodPut:
					require.True(t, deleted, "the failed bastion must be deleted before it is recreated")
					return gcResponse(req, http.StatusOK, `{"properties":{"provisioningState":"Succeeded","dnsName":"new.bastion.example"}}`)
				}
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil
			})
			opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{responder}}}
			network, err := armnetwork.NewClientFactory("test", nil, opts)
			require.NoError(t, err)
			config.Azure = &config.AzureClient{
				BastionHosts: network.NewBastionHostsClient(), PublicIPAddresses: network.NewPublicIPAddressesClient(),
			}

			dnsName, err := ensureSharedBastion(logging.WithLogger(t.Context(), t), "rg", "westus3")
			require.Equal(t, tt.wantCalls, calls)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.state == "Failed" {
				require.Equal(t, "new.bastion.example", dnsName)
			} else {
				require.Equal(t, tt.dnsName, dnsName)
			}
		})
	}
}

func TestEnsureSubnetReappliesFailedSubnet(t *testing.T) {
	originalConfig, originalAzure := config.Config, config.Azure
	t.Cleanup(func() { config.Config, config.Azure = originalConfig, originalAzure })
	config.Config = &config.Configuration{SubscriptionID: "test", DefaultPollInterval: time.Millisecond}

	for _, state := range []string{"Succeeded", "Failed"} {
		t.Run(state, func(t *testing.T) {
			const natID = "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Network/natGateways/nat"
			puts := 0
			responder := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				require.True(t, strings.HasSuffix(req.URL.Path, "/subnets/AzureBastionSubnet"), req.URL.Path)
				switch req.Method {
				case http.MethodGet:
					return gcResponse(req, http.StatusOK, fmt.Sprintf(
						`{"name":"AzureBastionSubnet","properties":{"provisioningState":%q,"addressPrefix":"10.0.0.0/26","natGateway":{"id":%q}}}`, state, natID))
				case http.MethodPut:
					puts++
					var subnet armnetwork.Subnet
					require.NoError(t, json.NewDecoder(req.Body).Decode(&subnet))
					require.Equal(t, "10.0.0.0/26", *subnet.Properties.AddressPrefix)
					require.Equal(t, natID, *subnet.Properties.NatGateway.ID, "reapplying must keep the subnet's attachments")
					return gcResponse(req, http.StatusOK, `{"name":"AzureBastionSubnet","properties":{"provisioningState":"Succeeded"}}`)
				}
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil
			})
			opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{responder}}}
			client, err := armnetwork.NewSubnetsClient("test", nil, opts)
			require.NoError(t, err)
			config.Azure = &config.AzureClient{Subnet: client}

			require.NoError(t, ensureSubnet(logging.WithLogger(t.Context(), t), "rg", SharedVNetName, "AzureBastionSubnet", "10.0.0.0/26"))
			if state == "Failed" {
				require.Equal(t, 1, puts)
			} else {
				require.Zero(t, puts)
			}
		})
	}
}

func TestReapplyFailedSubnetRereadsAfterConflict(t *testing.T) {
	originalConfig, originalAzure := config.Config, config.Azure
	t.Cleanup(func() { config.Config, config.Azure = originalConfig, originalAzure })
	config.Config = &config.Configuration{SubscriptionID: "test", DefaultPollInterval: time.Millisecond}

	gets, puts := 0, 0
	responder := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
		switch req.Method {
		case http.MethodGet:
			gets++
			if gets == 1 {
				return gcResponse(req, http.StatusOK, `{"name":"s","properties":{"provisioningState":"Failed","addressPrefix":"10.0.0.0/26"}}`)
			}
			// Another run repaired the subnet and attached a route table during the backoff.
			return gcResponse(req, http.StatusOK, `{"name":"s","properties":{"provisioningState":"Succeeded","addressPrefix":"10.0.0.0/26","routeTable":{"id":"rt"}}}`)
		case http.MethodPut:
			puts++
			return gcResponse(req, http.StatusConflict, `{"error":{"code":"AnotherOperationInProgress","message":"conflict"}}`)
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil
	})
	opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{responder}}}
	client, err := armnetwork.NewSubnetsClient("test", nil, opts)
	require.NoError(t, err)
	config.Azure = &config.AzureClient{Subnet: client}

	failed := armnetwork.Subnet{Properties: &armnetwork.SubnetPropertiesFormat{ProvisioningState: to.Ptr(armnetwork.ProvisioningStateFailed)}}
	require.NoError(t, reapplyFailedSubnet(logging.WithLogger(t.Context(), t), "rg", SharedVNetName, "s", failed))
	require.Equal(t, 1, puts, "a retry must not PUT a stale body over the repaired subnet")
	require.Equal(t, 2, gets)
}
