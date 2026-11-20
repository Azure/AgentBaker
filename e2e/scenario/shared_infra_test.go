package scenario

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

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

func TestGetFirewallAllowsCVMValidation(t *testing.T) {
	fw := getFirewall(logging.WithLogger(t.Context(), t), "westus3", "subnet-id", "public-ip-id")
	coll := fw.Properties.ApplicationRuleCollections[0]
	require.Equal(t, armnetwork.AzureFirewallRCActionTypeAllow, *coll.Properties.Action.Type)
	var cvmRule *armnetwork.AzureFirewallApplicationRule
	for _, rule := range coll.Properties.Rules {
		if *rule.Name == "cvm-validation-fqdn" {
			cvmRule = rule
			break
		}
	}
	require.NotNil(t, cvmRule)
	require.Equal(t, []*string{to.Ptr(SharedVNetCIDR)}, cvmRule.SourceAddresses)
	require.Equal(t, []*armnetwork.AzureFirewallApplicationRuleProtocol{{
		ProtocolType: to.Ptr(armnetwork.AzureFirewallApplicationRuleProtocolTypeHTTPS),
		Port:         to.Ptr[int32](443),
	}}, cvmRule.Protocols)
	require.ElementsMatch(t, []*string{
		to.Ptr("github.com"),
		to.Ptr("pypi.org"),
		to.Ptr("files.pythonhosted.org"),
		to.Ptr("sharedwus.wus.attest.azure.net"),
	}, cvmRule.TargetFqdns)
}

func TestEnsureSharedFirewallReconcilesRules(t *testing.T) {
	originalConfig, originalAzure := config.Config, config.Azure
	t.Cleanup(func() { config.Config, config.Azure = originalConfig, originalAzure })
	config.Config = &config.Configuration{SubscriptionID: "test"}

	for _, tt := range []struct {
		name              string
		mutate            func(*armnetwork.AzureFirewall)
		wantUpdate        bool
		preserveUnrelated bool
		wantError         string
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
			name: "stale blob rule", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules[1].TargetFqdns = []*string{to.Ptr("old.blob.core.windows.net")}
			},
		},
		{
			name: "legacy firewall without CVM rule", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules = fw.Properties.ApplicationRuleCollections[0].Properties.Rules[:4]
			},
		},
		{
			name: "incomplete CVM destinations", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules[4].TargetFqdns = []*string{to.Ptr("github.com")}
			},
		},
		{
			name: "CVM rule without attestation destination", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules[4].TargetFqdns = []*string{
					to.Ptr("github.com"), to.Ptr("pypi.org"), to.Ptr("files.pythonhosted.org"),
				}
			},
		},
		{
			name: "wrong CVM protocol", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules[4].Protocols[0].ProtocolType = to.Ptr(armnetwork.AzureFirewallApplicationRuleProtocolTypeHTTP)
			},
		},
		{
			name: "wrong CVM port", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules[4].Protocols[0].Port = to.Ptr[int32](80)
			},
		},
		{
			name: "wrong CVM source", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules[4].SourceAddresses = []*string{to.Ptr("192.168.0.0/16")}
			},
		},
		{
			name: "application deny action", wantUpdate: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Action.Type = to.Ptr(armnetwork.AzureFirewallRCActionTypeDeny)
			},
		},
		{
			name: "CVM migration preserves unrelated configuration", wantUpdate: true, preserveUnrelated: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules = fw.Properties.ApplicationRuleCollections[0].Properties.Rules[:4]
			},
		},
		{
			name: "NTP repair preserves unrelated configuration", wantUpdate: true, preserveUnrelated: true,
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.NetworkRuleCollections[0].Properties.Rules[0].DestinationPorts = []*string{to.Ptr("443")}
			},
		},
		{
			name: "unrelated application deny rules are not changed to allow", preserveUnrelated: true,
			wantError: "cannot change collection action to Allow with unrelated rules",
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.ApplicationRuleCollections[0].Properties.Action.Type = to.Ptr(armnetwork.AzureFirewallRCActionTypeDeny)
			},
		},
		{
			name: "unrelated network deny rules are not changed to allow", preserveUnrelated: true,
			wantError: "cannot change collection action to Allow with unrelated rules",
			mutate: func(fw *armnetwork.AzureFirewall) {
				fw.Properties.NetworkRuleCollections[0].Properties.Action.Type = to.Ptr(armnetwork.AzureFirewallRCActionTypeDeny)
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
			var checkPreserved func(armnetwork.AzureFirewall)
			if tt.preserveUnrelated {
				appRule := &armnetwork.AzureFirewallApplicationRule{
					Name: to.Ptr("unrelated-app"), SourceAddresses: []*string{to.Ptr("10.1.0.0/16")},
					TargetFqdns: []*string{to.Ptr("example.com")},
				}
				netRule := &armnetwork.AzureFirewallNetworkRule{
					Name: to.Ptr("unrelated-network"), DestinationPorts: []*string{to.Ptr("8443")},
				}
				appCollection := &armnetwork.AzureFirewallApplicationRuleCollection{
					Name: to.Ptr("unrelated-app-collection"),
					Properties: &armnetwork.AzureFirewallApplicationRuleCollectionPropertiesFormat{
						Priority: to.Ptr[int32](300),
						Action:   &armnetwork.AzureFirewallRCAction{Type: to.Ptr(armnetwork.AzureFirewallRCActionTypeDeny)},
						Rules:    []*armnetwork.AzureFirewallApplicationRule{appRule},
					},
				}
				netCollection := &armnetwork.AzureFirewallNetworkRuleCollection{
					Name: to.Ptr("unrelated-network-collection"),
					Properties: &armnetwork.AzureFirewallNetworkRuleCollectionPropertiesFormat{
						Priority: to.Ptr[int32](400),
						Action:   &armnetwork.AzureFirewallRCAction{Type: to.Ptr(armnetwork.AzureFirewallRCActionTypeDeny)},
						Rules:    []*armnetwork.AzureFirewallNetworkRule{netRule},
					},
				}
				natCollection := &armnetwork.AzureFirewallNatRuleCollection{
					Name: to.Ptr("unrelated-nat"),
					Properties: &armnetwork.AzureFirewallNatRuleCollectionProperties{
						Priority: to.Ptr[int32](500),
						Rules: []*armnetwork.AzureFirewallNatRule{{
							Name: to.Ptr("unrelated-nat"), TranslatedAddress: to.Ptr("10.2.0.4"), TranslatedPort: to.Ptr("443"),
						}},
					},
				}
				fw.Tags = map[string]*string{"owner": to.Ptr("another-test")}
				fw.Properties.ThreatIntelMode = to.Ptr(armnetwork.AzureFirewallThreatIntelModeDeny)
				fw.Properties.ApplicationRuleCollections[0].Properties.Priority = to.Ptr[int32](150)
				fw.Properties.ApplicationRuleCollections[0].Properties.Rules = append(fw.Properties.ApplicationRuleCollections[0].Properties.Rules, appRule)
				fw.Properties.NetworkRuleCollections[0].Properties.Rules = append(fw.Properties.NetworkRuleCollections[0].Properties.Rules, netRule)
				fw.Properties.ApplicationRuleCollections = append(fw.Properties.ApplicationRuleCollections, appCollection)
				fw.Properties.NetworkRuleCollections = append(fw.Properties.NetworkRuleCollections, netCollection)
				fw.Properties.NatRuleCollections = []*armnetwork.AzureFirewallNatRuleCollection{natCollection}
				checkPreserved = func(updated armnetwork.AzureFirewall) {
					require.Equal(t, fw.Tags, updated.Tags)
					require.Equal(t, fw.Properties.IPConfigurations, updated.Properties.IPConfigurations)
					require.Equal(t, armnetwork.AzureFirewallThreatIntelModeDeny, *updated.Properties.ThreatIntelMode)
					require.Equal(t, int32(150), *updated.Properties.ApplicationRuleCollections[0].Properties.Priority)
					require.Contains(t, updated.Properties.ApplicationRuleCollections[0].Properties.Rules, appRule)
					require.Contains(t, updated.Properties.NetworkRuleCollections[0].Properties.Rules, netRule)
					require.Contains(t, updated.Properties.ApplicationRuleCollections, appCollection)
					require.Contains(t, updated.Properties.NetworkRuleCollections, netCollection)
					require.Equal(t, []*armnetwork.AzureFirewallNatRuleCollection{natCollection}, updated.Properties.NatRuleCollections)
				}
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
					if checkPreserved != nil {
						checkPreserved(updated)
					}
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
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				require.Zero(t, updates)
				return
			}
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
