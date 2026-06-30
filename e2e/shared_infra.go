package e2e

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/google/uuid"
)

const (
	SharedVNetName          = "abe2e-shared-vnet"
	SharedVNetCIDR          = "10.0.0.0/8"
	SharedVNetIPv6CIDR      = "fd00::/48"
	SharedBastionName       = "abe2e-shared-bastion"
	SharedBastionPIPName    = "abe2e-shared-bastion-pip"
	SharedClusterIdentity   = "abe2e-cluster-identity"
	BastionSubnetCIDR       = "10.0.0.0/26"
	FirewallSubnetCIDR      = "10.0.1.0/24"
	PESubnetName            = "abe2e-pe-subnet"
	PESubnetCIDR            = "10.0.2.0/24"
	networkContributorRolID = "/providers/Microsoft.Authorization/roleDefinitions/4d97b98b-1d4f-4787-a291-c67834d212e7"
)

type SharedInfra struct {
	VNetName       string
	ResourceGroup  string
	BastionDNSName string
	FirewallIP     string
	IdentityID     string // resource ID of the user-assigned managed identity
	TenantID       string // tenant ID of the user-assigned managed identity
}

var CachedEnsureSharedInfra = cachedFunc(ensureSharedInfra)

func ensureSharedInfra(ctx context.Context, location string) (*SharedInfra, error) {
	defer toolkit.LogStepCtx(ctx, "ensuring shared infrastructure")()
	rg := config.ResourceGroupName(location)

	if err := ensureSharedVNet(ctx, rg, location); err != nil {
		return nil, fmt.Errorf("ensuring shared VNet: %w", err)
	}

	if err := ensurePESubnet(ctx, rg); err != nil {
		return nil, fmt.Errorf("ensuring PE subnet: %w", err)
	}

	bastionDNS, err := ensureSharedBastion(ctx, rg, location)
	if err != nil {
		return nil, fmt.Errorf("ensuring shared bastion: %w", err)
	}

	firewallIP, err := ensureSharedFirewall(ctx, rg, location)
	if err != nil {
		return nil, fmt.Errorf("ensuring shared firewall: %w", err)
	}

	identityID, tenantID, err := ensureClusterIdentity(ctx, rg, location)
	if err != nil {
		return nil, fmt.Errorf("ensuring cluster identity: %w", err)
	}

	// Best-effort cleanup of orphaned cluster subnets
	cleanupOrphanedSubnets(ctx, rg)

	return &SharedInfra{
		VNetName:       SharedVNetName,
		ResourceGroup:  rg,
		BastionDNSName: bastionDNS,
		FirewallIP:     firewallIP,
		IdentityID:     identityID,
		TenantID:       tenantID,
	}, nil
}

func ensureSharedVNet(ctx context.Context, rg, location string) error {
	existing, err := config.Azure.VNet.Get(ctx, rg, SharedVNetName, nil)
	if err == nil {
		// VNet exists — ensure it has the IPv6 address space for dual-stack clusters.
		hasIPv6 := false
		if existing.Properties != nil && existing.Properties.AddressSpace != nil {
			for _, prefix := range existing.Properties.AddressSpace.AddressPrefixes {
				if prefix != nil && *prefix == SharedVNetIPv6CIDR {
					hasIPv6 = true
					break
				}
			}
		}
		if !hasIPv6 {
			toolkit.Logf(ctx, "adding IPv6 address space %s to shared VNet %s", SharedVNetIPv6CIDR, SharedVNetName)
			if existing.Properties == nil {
				existing.Properties = &armnetwork.VirtualNetworkPropertiesFormat{}
			}
			if existing.Properties.AddressSpace == nil {
				existing.Properties.AddressSpace = &armnetwork.AddressSpace{}
			}
			prefixes := existing.Properties.AddressSpace.AddressPrefixes
			prefixes = append(prefixes, to.Ptr(SharedVNetIPv6CIDR))
			// Preserve existing subnets in the PUT body — Azure VNet CreateOrUpdate
			// is a full PUT, so omitting Subnets would delete them.
			existing.Properties.AddressSpace.AddressPrefixes = prefixes
			poller, updateErr := config.Azure.VNet.BeginCreateOrUpdate(ctx, rg, SharedVNetName, armnetwork.VirtualNetwork{
				Location:   existing.Location,
				Tags:       existing.Tags,
				Properties: existing.Properties,
			}, nil)
			if updateErr != nil {
				return fmt.Errorf("adding IPv6 to shared VNet: %w", updateErr)
			}
			if _, updateErr = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions); updateErr != nil {
				return fmt.Errorf("waiting for IPv6 VNet update: %w", updateErr)
			}
		}
		return nil
	}
	if !isNotFoundError(err) {
		return fmt.Errorf("checking shared VNet: %w", err)
	}

	toolkit.Logf(ctx, "creating shared VNet %s in %s", SharedVNetName, rg)
	poller, err := config.Azure.VNet.BeginCreateOrUpdate(ctx, rg, SharedVNetName, armnetwork.VirtualNetwork{
		Location: to.Ptr(location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr(SharedVNetCIDR), to.Ptr(SharedVNetIPv6CIDR)},
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("creating shared VNet: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("waiting for shared VNet creation: %w", err)
	}
	return nil
}

// ensurePESubnet creates the dedicated subnet for shared private endpoints.
func ensurePESubnet(ctx context.Context, rg string) error {
	_, err := config.Azure.Subnet.Get(ctx, rg, SharedVNetName, PESubnetName, nil)
	if err == nil {
		return nil
	}
	if !isNotFoundError(err) {
		return fmt.Errorf("checking PE subnet: %w", err)
	}
	toolkit.Logf(ctx, "creating PE subnet %s", PESubnetName)
	poller, err := config.Azure.Subnet.BeginCreateOrUpdate(ctx, rg, SharedVNetName, PESubnetName, armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr(PESubnetCIDR),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("creating PE subnet: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("waiting for PE subnet creation: %w", err)
	}
	return nil
}

// cleanupOrphanedSubnets removes cluster subnets whose corresponding AKS cluster
// no longer exists and that have no active Azure resources attached.
// Only considers subnets that are not recently provisioned to avoid racing with
// cluster creation.
func cleanupOrphanedSubnets(ctx context.Context, rg string) {
	pager := config.Azure.Subnet.NewListPager(rg, SharedVNetName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			toolkit.Logf(ctx, "warning: failed to list subnets for cleanup: %v", err)
			return
		}
		for _, subnet := range page.Value {
			name := *subnet.Name
			if !strings.HasPrefix(name, "aks-subnet-") {
				continue
			}
			if subnetHasActiveResources(subnet) {
				continue
			}
			clusterName := strings.TrimPrefix(name, "aks-subnet-")
			_, err := config.Azure.AKS.Get(ctx, rg, clusterName, nil)
			if err == nil {
				continue
			}
			if !isNotFoundError(err) {
				toolkit.Logf(ctx, "warning: transient error checking cluster %s, skipping subnet cleanup: %v", clusterName, err)
				continue
			}
			toolkit.Logf(ctx, "deleting orphaned subnet %s", name)
			poller, err := config.Azure.Subnet.BeginDelete(ctx, rg, SharedVNetName, name, nil)
			if err != nil {
				toolkit.Logf(ctx, "warning: failed to start deleting subnet %s: %v", name, err)
				continue
			}
			if _, err := poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions); err != nil {
				toolkit.Logf(ctx, "warning: failed to delete subnet %s: %v", name, err)
			}
		}
	}
}

func subnetHasActiveResources(subnet *armnetwork.Subnet) bool {
	if subnet.Properties == nil {
		return false
	}
	return len(subnet.Properties.IPConfigurations) > 0 ||
		len(subnet.Properties.ServiceAssociationLinks) > 0 ||
		len(subnet.Properties.ResourceNavigationLinks) > 0 ||
		subnet.Properties.NetworkSecurityGroup != nil ||
		subnet.Properties.RouteTable != nil
}

func ensureSubnet(ctx context.Context, rg, vnetName, subnetName, cidr string) error {
	_, err := config.Azure.Subnet.Get(ctx, rg, vnetName, subnetName, nil)
	if err == nil {
		return nil
	}
	if !isNotFoundError(err) {
		return fmt.Errorf("checking subnet %s: %w", subnetName, err)
	}

	return retryOn409(ctx, fmt.Sprintf("creating subnet %s", subnetName), func() error {
		toolkit.Logf(ctx, "creating subnet %s (%s) in VNet %s", subnetName, cidr, vnetName)
		poller, err := config.Azure.Subnet.BeginCreateOrUpdate(ctx, rg, vnetName, subnetName, armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix: to.Ptr(cidr),
			},
		}, nil)
		if err != nil {
			return fmt.Errorf("creating subnet %s: %w", subnetName, err)
		}
		_, err = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
		if err != nil {
			return fmt.Errorf("waiting for subnet %s: %w", subnetName, err)
		}
		return nil
	})
}

// ensureDualStackSubnet creates a subnet with both IPv4 and IPv6 address prefixes
// for dual-stack clusters.
func ensureDualStackSubnet(ctx context.Context, rg, vnetName, subnetName, ipv4CIDR, ipv6CIDR string) error {
	_, err := config.Azure.Subnet.Get(ctx, rg, vnetName, subnetName, nil)
	if err == nil {
		return nil
	}
	if !isNotFoundError(err) {
		return fmt.Errorf("checking subnet %s: %w", subnetName, err)
	}

	return retryOn409(ctx, fmt.Sprintf("creating dual-stack subnet %s", subnetName), func() error {
		toolkit.Logf(ctx, "creating dual-stack subnet %s (%s, %s) in VNet %s", subnetName, ipv4CIDR, ipv6CIDR, vnetName)
		poller, err := config.Azure.Subnet.BeginCreateOrUpdate(ctx, rg, vnetName, subnetName, armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefixes: []*string{to.Ptr(ipv4CIDR), to.Ptr(ipv6CIDR)},
			},
		}, nil)
		if err != nil {
			return fmt.Errorf("creating dual-stack subnet %s: %w", subnetName, err)
		}
		_, err = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
		if err != nil {
			return fmt.Errorf("waiting for dual-stack subnet %s: %w", subnetName, err)
		}
		return nil
	})
}

func ensureSharedBastion(ctx context.Context, rg, location string) (string, error) {
	existing, err := config.Azure.BastionHosts.Get(ctx, rg, SharedBastionName, nil)
	if err == nil {
		if existing.Properties == nil || existing.Properties.DNSName == nil {
			return "", fmt.Errorf("shared bastion %s exists but has no DNS name", SharedBastionName)
		}
		return *existing.Properties.DNSName, nil
	}
	if !isNotFoundError(err) {
		return "", fmt.Errorf("checking shared bastion: %w", err)
	}

	if err := ensureSubnet(ctx, rg, SharedVNetName, "AzureBastionSubnet", BastionSubnetCIDR); err != nil {
		return "", fmt.Errorf("ensuring bastion subnet: %w", err)
	}

	toolkit.Logf(ctx, "creating shared bastion public IP %s", SharedBastionPIPName)
	pipPoller, err := config.Azure.PublicIPAddresses.BeginCreateOrUpdate(ctx, rg, SharedBastionPIPName, armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("creating bastion public IP: %w", err)
	}
	pipResp, err := pipPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return "", fmt.Errorf("waiting for bastion public IP: %w", err)
	}

	bastionSubnetID := fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/AzureBastionSubnet",
		config.Config.SubscriptionID, rg, SharedVNetName,
	)

	toolkit.Logf(ctx, "creating shared bastion %s (Standard SKU, tunneling enabled)", SharedBastionName)
	bastionPoller, err := config.Azure.BastionHosts.BeginCreateOrUpdate(ctx, rg, SharedBastionName, armnetwork.BastionHost{
		Location: to.Ptr(location),
		SKU: &armnetwork.SKU{
			Name: to.Ptr(armnetwork.BastionHostSKUNameStandard),
		},
		Properties: &armnetwork.BastionHostPropertiesFormat{
			EnableTunneling: to.Ptr(true),
			IPConfigurations: []*armnetwork.BastionHostIPConfiguration{
				{
					Name: to.Ptr("bastion-ipcfg"),
					Properties: &armnetwork.BastionHostIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.SubResource{
							ID: to.Ptr(bastionSubnetID),
						},
						PublicIPAddress: &armnetwork.SubResource{
							ID: pipResp.ID,
						},
					},
				},
			},
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("creating shared bastion: %w", err)
	}
	resp, err := bastionPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return "", fmt.Errorf("waiting for shared bastion: %w", err)
	}
	return *resp.Properties.DNSName, nil
}

const (
	SharedFirewallName    = "abe2e-fw"
	SharedFirewallPIPName = "abe2e-fw-pip"
)

// ensureSharedFirewall creates or retrieves the shared Azure Firewall and returns its private IP.
// The firewall is shared across all clusters in the RG, so its application rules persist beyond
// the lifetime of any single cluster. We reconcile rules on every call so that changes to allowed
// FQDNs (notably the per-sub blob storage account FQDN, which now embeds a sub-suffix) are picked
// up. Without this, a firewall created against an older BlobStorageAccount() name silently blocks
// CSE zip downloads from the new per-sub storage account, causing Windows CSE to fail with
// NO_CSE_RESULT_LOG and Linux CSE to hang on artifact downloads.
func ensureSharedFirewall(ctx context.Context, rg, location string) (string, error) {
	existing, err := config.Azure.AzureFirewall.Get(ctx, rg, SharedFirewallName, nil)
	if err == nil {
		// Fast path: rules already match current config.
		if firewallAppRulesUpToDate(existing.AzureFirewall) {
			return getFirewallPrivateIP(existing.AzureFirewall)
		}
		toolkit.Logf(ctx, "shared firewall %s exists but app rules are stale; updating", SharedFirewallName)
		firewallSubnetID := fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/AzureFirewallSubnet",
			config.Config.SubscriptionID, rg, SharedVNetName,
		)
		// Reuse the existing PIP — recreating it would reassign the firewall's public IP and
		// break any external dependency pinned to it.
		var publicIPID string
		if existing.Properties != nil && len(existing.Properties.IPConfigurations) > 0 &&
			existing.Properties.IPConfigurations[0].Properties != nil &&
			existing.Properties.IPConfigurations[0].Properties.PublicIPAddress != nil &&
			existing.Properties.IPConfigurations[0].Properties.PublicIPAddress.ID != nil {
			publicIPID = *existing.Properties.IPConfigurations[0].Properties.PublicIPAddress.ID
		} else {
			return "", fmt.Errorf("existing firewall %s has no public IP configuration", SharedFirewallName)
		}
		updated := getFirewall(ctx, location, firewallSubnetID, publicIPID)
		fwPoller, err := config.Azure.AzureFirewall.BeginCreateOrUpdate(ctx, rg, SharedFirewallName, *updated, nil)
		if err != nil {
			return "", fmt.Errorf("updating shared firewall app rules: %w", err)
		}
		fwResp, err := fwPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
		if err != nil {
			return "", fmt.Errorf("waiting for shared firewall update: %w", err)
		}
		return getFirewallPrivateIP(fwResp.AzureFirewall)
	}
	if !isNotFoundError(err) {
		return "", fmt.Errorf("checking shared firewall: %w", err)
	}

	// Ensure firewall subnet exists
	if err := ensureSubnet(ctx, rg, SharedVNetName, "AzureFirewallSubnet", FirewallSubnetCIDR); err != nil {
		return "", fmt.Errorf("ensuring firewall subnet: %w", err)
	}

	firewallSubnetID := fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/AzureFirewallSubnet",
		config.Config.SubscriptionID, rg, SharedVNetName,
	)

	// Create public IP for firewall
	toolkit.Logf(ctx, "creating shared firewall public IP %s", SharedFirewallPIPName)
	pipPoller, err := config.Azure.PublicIPAddresses.BeginCreateOrUpdate(ctx, rg, SharedFirewallPIPName, armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("creating firewall public IP: %w", err)
	}
	pipResp, err := pipPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return "", fmt.Errorf("waiting for firewall public IP: %w", err)
	}

	// Create firewall
	toolkit.Logf(ctx, "creating shared firewall %s", SharedFirewallName)
	firewall := getFirewall(ctx, location, firewallSubnetID, *pipResp.ID)
	fwPoller, err := config.Azure.AzureFirewall.BeginCreateOrUpdate(ctx, rg, SharedFirewallName, *firewall, nil)
	if err != nil {
		return "", fmt.Errorf("creating shared firewall: %w", err)
	}
	fwResp, err := fwPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return "", fmt.Errorf("waiting for shared firewall: %w", err)
	}

	return getFirewallPrivateIP(fwResp.AzureFirewall)
}

func getFirewallPrivateIP(fw armnetwork.AzureFirewall) (string, error) {
	if fw.Properties != nil && fw.Properties.IPConfigurations != nil && len(fw.Properties.IPConfigurations) > 0 {
		if fw.Properties.IPConfigurations[0].Properties != nil && fw.Properties.IPConfigurations[0].Properties.PrivateIPAddress != nil {
			return *fw.Properties.IPConfigurations[0].Properties.PrivateIPAddress, nil
		}
	}
	return "", fmt.Errorf("firewall has no private IP address")
}

// firewallAppRulesUpToDate returns true when the existing firewall already allows the current
// per-sub blob storage FQDN. We only check the blob-storage-fqdn rule because that's the only
// rule whose target depends on dynamic config — the rest (aks-fqdn, mooncake-mar, dmc) use
// static values. If the storage FQDN changed (e.g. sub-suffix added), the rule is stale.
func firewallAppRulesUpToDate(fw armnetwork.AzureFirewall) bool {
	if fw.Properties == nil {
		return false
	}
	expectedFqdn := config.Config.BlobStorageAccount() + ".blob.core.windows.net"
	for _, coll := range fw.Properties.ApplicationRuleCollections {
		if coll == nil || coll.Properties == nil {
			continue
		}
		for _, rule := range coll.Properties.Rules {
			if rule == nil || rule.Name == nil || *rule.Name != "blob-storage-fqdn" {
				continue
			}
			// Found the dynamic rule; match means firewall is current.
			for _, fqdn := range rule.TargetFqdns {
				if fqdn != nil && *fqdn == expectedFqdn {
					return true
				}
			}
			return false
		}
	}
	// Rule not found at all → firewall predates this rule, treat as stale.
	return false
}

// ensureClusterIdentity creates a user-assigned managed identity for AKS clusters
// and grants it Network Contributor on the subscription so it can manage route tables
// in both the shared VNet and the MC_ resource groups.
func ensureClusterIdentity(ctx context.Context, rg, location string) (string, string, error) {
	existing, err := config.Azure.UserAssignedIdentities.Get(ctx, rg, SharedClusterIdentity, nil)
	if err == nil {
		return *existing.ID, *existing.Properties.TenantID, nil
	}
	if !isNotFoundError(err) {
		return "", "", fmt.Errorf("checking cluster identity: %w", err)
	}

	toolkit.Logf(ctx, "creating shared cluster identity %s", SharedClusterIdentity)
	resp, err := config.Azure.UserAssignedIdentities.CreateOrUpdate(ctx, rg, SharedClusterIdentity, armmsi.Identity{
		Location: to.Ptr(location),
	}, nil)
	if err != nil {
		return "", "", fmt.Errorf("creating cluster identity: %w", err)
	}

	// Grant Network Contributor on the shared VNet so the identity can manage
	// subnets and route table associations. AKS automatically grants the identity
	// Contributor on the MC_ resource group during cluster creation.
	vnetScope := fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s",
		config.Config.SubscriptionID, rg, SharedVNetName,
	)
	toolkit.Logf(ctx, "assigning Network Contributor to %s on shared VNet", SharedClusterIdentity)
	_, err = config.Azure.RoleAssignments.Create(ctx, vnetScope, uuid.New().String(), armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID:      resp.Properties.PrincipalID,
			RoleDefinitionID: to.Ptr(networkContributorRolID),
			PrincipalType:    to.Ptr(armauthorization.PrincipalTypeServicePrincipal),
		},
	}, nil)
	if err != nil {
		var azErr *azcore.ResponseError
		if errors.As(err, &azErr) && azErr.StatusCode == http.StatusConflict {
			// role assignment already exists
		} else {
			return "", "", fmt.Errorf("assigning Network Contributor: %w", err)
		}
	}

	return *resp.ID, *resp.Properties.TenantID, nil
}

type ClusterSubnetRequest struct {
	Location    string
	ClusterName string
	DualStack   bool
}

var CachedEnsureClusterSubnet = cachedFunc(ensureClusterSubnet)

func ensureClusterSubnet(ctx context.Context, req ClusterSubnetRequest) (string, error) {
	rg := config.ResourceGroupName(req.Location)
	subnetName := clusterSubnetName(req.ClusterName)

	// Check if this subnet already exists (idempotent)
	existing, err := config.Azure.Subnet.Get(ctx, rg, SharedVNetName, subnetName, nil)
	if err == nil {
		return *existing.ID, nil
	}
	if !isNotFoundError(err) {
		return "", fmt.Errorf("checking subnet %s: %w", subnetName, err)
	}

	// Collect CIDRs already in use
	usedCIDRs := map[string]bool{}
	pager := config.Azure.Subnet.NewListPager(rg, SharedVNetName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("listing subnets: %w", err)
		}
		for _, s := range page.Value {
			if s.Properties != nil {
				if s.Properties.AddressPrefix != nil {
					usedCIDRs[*s.Properties.AddressPrefix] = true
				}
				for _, p := range s.Properties.AddressPrefixes {
					if p != nil {
						usedCIDRs[*p] = true
					}
				}
			}
		}
	}

	// Find a free CIDR starting from the hash-based slot
	cidr := allocateSubnetCIDR(subnetName, usedCIDRs)
	if cidr == "" {
		return "", fmt.Errorf("no free /20 CIDR available in shared VNet")
	}

	if req.DualStack {
		ipv6CIDR := allocateSubnetIPv6CIDR(subnetName, usedCIDRs)
		if ipv6CIDR == "" {
			return "", fmt.Errorf("no free IPv6 /64 CIDR available in shared VNet")
		}
		if err := ensureDualStackSubnet(ctx, rg, SharedVNetName, subnetName, cidr, ipv6CIDR); err != nil {
			return "", err
		}
	} else {
		if err := ensureSubnet(ctx, rg, SharedVNetName, subnetName, cidr); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
		config.Config.SubscriptionID, rg, SharedVNetName, subnetName,
	), nil
}

func clusterSubnetName(clusterName string) string {
	return "aks-subnet-" + clusterName
}

func detachNodeResourceGroupReferencesFromClusterSubnet(ctx context.Context, location, clusterName, nodeResourceGroup string) error {
	rg := config.ResourceGroupName(location)
	subnetName := clusterSubnetName(clusterName)

	return retryOn409(ctx, fmt.Sprintf("detaching node resource group references from subnet %s", subnetName), func() error {
		subnetResp, err := config.Azure.Subnet.Get(ctx, rg, SharedVNetName, subnetName, nil)
		if err != nil {
			if isNotFoundError(err) {
				return nil
			}
			return fmt.Errorf("getting subnet %s: %w", subnetName, err)
		}

		shouldDetachRouteTable, err := subnetResourceIDInResourceGroup(routeTableID(subnetResp.Properties.RouteTable), nodeResourceGroup)
		if err != nil {
			return err
		}
		shouldDetachNSG, err := subnetResourceIDInResourceGroup(networkSecurityGroupID(subnetResp.Properties.NetworkSecurityGroup), nodeResourceGroup)
		if err != nil {
			return err
		}

		if shouldDetachRouteTable {
			routeTableID := *subnetResp.Properties.RouteTable.ID
			toolkit.Logf(ctx, "detaching route table %q from shared subnet %q because resource group %q is deleting", routeTableID, subnetName, nodeResourceGroup)
			subnetResp.Subnet.Properties.RouteTable = nil
		}
		if shouldDetachNSG {
			nsgID := *subnetResp.Properties.NetworkSecurityGroup.ID
			toolkit.Logf(ctx, "detaching network security group %q from shared subnet %q because resource group %q is deleting", nsgID, subnetName, nodeResourceGroup)
			subnetResp.Subnet.Properties.NetworkSecurityGroup = nil
		}
		if !shouldDetachRouteTable && !shouldDetachNSG {
			return nil
		}

		poller, err := config.Azure.Subnet.BeginCreateOrUpdate(ctx, rg, SharedVNetName, subnetName, subnetResp.Subnet, nil)
		if err != nil {
			return fmt.Errorf("detaching node resource group references from subnet %s: %w", subnetName, err)
		}
		_, err = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
		return err
	})
}

func routeTableID(routeTable *armnetwork.RouteTable) *string {
	if routeTable == nil {
		return nil
	}
	return routeTable.ID
}

func networkSecurityGroupID(nsg *armnetwork.SecurityGroup) *string {
	if nsg == nil {
		return nil
	}
	return nsg.ID
}

func subnetResourceIDInResourceGroup(resourceID *string, resourceGroup string) (bool, error) {
	if resourceID == nil {
		return false, nil
	}
	parsedResource, err := arm.ParseResourceID(*resourceID)
	if err != nil {
		return false, fmt.Errorf("parsing subnet resource ID %q: %w", *resourceID, err)
	}
	return strings.EqualFold(parsedResource.ResourceGroupName, resourceGroup), nil
}

const totalSubnetSlots = 4080

// reservedSecondOctets lists second-octet values whose /16 range is used by
// Kubernetes defaults and must not be allocated as cluster subnets.
//   - 244: default pod CIDR (10.244.0.0/16) for kubenet/overlay
var reservedSecondOctets = map[int]bool{
	244: true,
}

// allocateSubnetCIDR finds a free /20 CIDR by starting at a hash-derived slot
// and probing linearly until a free one is found.
func allocateSubnetCIDR(name string, usedCIDRs map[string]bool) string {
	h := sha256.Sum256([]byte(name))
	startIdx := (int(h[0])<<8 | int(h[1])) % totalSubnetSlots
	for i := 0; i < totalSubnetSlots; i++ {
		idx := (startIdx + i) % totalSubnetSlots
		cidr := cidrFromIndex(idx)
		if cidr == "" || usedCIDRs[cidr] {
			continue
		}
		return cidr
	}
	return ""
}

func cidrFromIndex(idx int) string {
	secondOctet := (idx / 16) + 1 // 1-255
	if reservedSecondOctets[secondOctet] {
		return ""
	}
	thirdOctet := (idx % 16) * 16 // 0, 16, 32, ..., 240
	return fmt.Sprintf("10.%d.%d.0/20", secondOctet, thirdOctet)
}

// allocateSubnetIPv6CIDR finds a free /64 within the VNet's fd00::/48 space.
// Uses the same hash-based approach as IPv4 allocation. The fd00::/48 space
// gives us 65536 /64 subnets (fd00:0:0:0000::/64 through fd00:0:0:ffff::/64).
// The variable part must be in the 4th group (bits 49-64) to stay inside the /48.
func allocateSubnetIPv6CIDR(name string, usedCIDRs map[string]bool) string {
	const totalIPv6Slots = 65536
	h := sha256.Sum256([]byte(name + "-ipv6"))
	startIdx := (int(h[0])<<8 | int(h[1])) % totalIPv6Slots
	for i := 0; i < totalIPv6Slots; i++ {
		idx := (startIdx + i) % totalIPv6Slots
		cidr := fmt.Sprintf("fd00:0:0:%04x::/64", idx)
		if usedCIDRs[cidr] {
			continue
		}
		return cidr
	}
	return ""
}

func isNotFoundError(err error) bool {
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) && azErr.StatusCode == http.StatusNotFound {
		return true
	}
	return false
}

// vnetFromSubnetID parses VNet info from a subnet resource ID and fetches the VNet for metadata.
func vnetFromSubnetID(ctx context.Context, subnetID string) (VNet, error) {
	parts := strings.Split(subnetID, "/")
	var rg, vnetName, subnetName string
	for i, p := range parts {
		if i+1 >= len(parts) {
			continue
		}
		switch p {
		case "resourceGroups":
			rg = parts[i+1]
		case "virtualNetworks":
			vnetName = parts[i+1]
		case "subnets":
			subnetName = parts[i+1]
		}
	}
	if rg == "" || vnetName == "" || subnetName == "" {
		return VNet{}, fmt.Errorf("failed to parse VNet info from subnet ID: %s", subnetID)
	}

	vnetResp, err := config.Azure.VNet.Get(ctx, rg, vnetName, nil)
	if err != nil {
		return VNet{}, fmt.Errorf("getting VNet %s in RG %s: %w", vnetName, rg, err)
	}

	var addressPrefix string
	if vnetResp.Properties.AddressSpace != nil && len(vnetResp.Properties.AddressSpace.AddressPrefixes) > 0 {
		addressPrefix = *vnetResp.Properties.AddressSpace.AddressPrefixes[0]
	}

	var resourceGUID string
	if vnetResp.Properties != nil && vnetResp.Properties.ResourceGUID != nil {
		resourceGUID = *vnetResp.Properties.ResourceGUID
	}

	return VNet{
		name:          vnetName,
		resourceGroup: rg,
		subnetName:    subnetName,
		subnetId:      subnetID,
		resourceGUID:  resourceGUID,
		addressPrefix: addressPrefix,
	}, nil
}

// retryOn409 retries an Azure operation that fails with 409 Conflict due to
// concurrent writes on the same resource (e.g., VNet subnet creates).
func retryOn409(ctx context.Context, operation string, fn func() error) error {
	maxRetries := 10
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		var azErr *azcore.ResponseError
		if !errors.As(err, &azErr) || azErr.StatusCode != http.StatusConflict {
			return err
		}
		if attempt == maxRetries-1 {
			return err
		}
		// jittered backoff: 2-8s
		backoff := time.Duration(2+rand.Intn(6)) * time.Second
		toolkit.Logf(ctx, "%s: 409 conflict (attempt %d/%d), retrying in %s...", operation, attempt+1, maxRetries, backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("%s: exhausted retries", operation)
}

// configureSharedVNet sets up the cluster model to use the shared VNet and
// user-assigned identity. The actual subnet is created later in prepareCluster
// after the cluster name hash is computed, with an auto-allocated CIDR.
func configureSharedVNet(ctx context.Context, model *armcontainerservice.ManagedCluster, location string) (*SharedInfra, error) {
	infra, err := CachedEnsureSharedInfra(ctx, location)
	if err != nil {
		return nil, fmt.Errorf("ensuring shared infra: %w", err)
	}

	// Mark the model so prepareCluster knows to create a subnet
	if model.Tags == nil {
		model.Tags = map[string]*string{}
	}

	// Use the shared user-assigned identity
	model.Identity = &armcontainerservice.ManagedClusterIdentity{
		Type: to.Ptr(armcontainerservice.ResourceIdentityTypeUserAssigned),
		UserAssignedIdentities: map[string]*armcontainerservice.ManagedServiceIdentityUserAssignedIdentitiesValue{
			infra.IdentityID: {},
		},
	}

	return infra, nil
}
