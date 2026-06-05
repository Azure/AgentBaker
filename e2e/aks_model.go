package e2e

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerregistry/armcontainerregistry/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"k8s.io/apimachinery/pkg/util/wait"
)

// getLatestGAKubernetesVersion returns the highest GA Kubernetes version for the given location.
func getLatestGAKubernetesVersion(ctx context.Context, location string) (string, error) {
	versions, err := config.Azure.AKS.ListKubernetesVersions(context.Background(), location, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list Kubernetes versions: %w", err)
	}
	if len(versions.Values) == 0 {
		return "", fmt.Errorf("no Kubernetes versions available")
	}

	var latestPatchVersion string
	msg := fmt.Sprintf("Available Kubernetes versions for location %s:\n", location)
	defer func() { toolkit.Logf(ctx, "%s", msg) }()
	// Iterate through the available versions to find the latest GA version
	for _, k8sVersion := range versions.Values {
		if k8sVersion == nil {
			continue
		}
		msg += fmt.Sprintf("- %s\n", *k8sVersion.Version)

		// Skip preview versions
		if k8sVersion.IsPreview != nil && *k8sVersion.IsPreview {
			msg += " - - is in preview, skipping\n"
			continue
		}
		for patchVersion := range k8sVersion.PatchVersions {
			if patchVersion == "" {
				continue
			}
			msg += fmt.Sprintf(" - - %s\n", patchVersion)
			// Initialize latestVersion with first GA version found
			if latestPatchVersion == "" {
				latestPatchVersion = patchVersion
				msg += fmt.Sprintf(" - - first latest found, updating to: %s\n", latestPatchVersion)
				continue
			}
			// Compare versions
			if agent.IsKubernetesVersionGe(patchVersion, latestPatchVersion) {
				latestPatchVersion = patchVersion
				msg += fmt.Sprintf(" - - new latest found, updating to: %s\n", latestPatchVersion)
			}
		}
	}

	if latestPatchVersion == "" {
		return "", fmt.Errorf("no GA Kubernetes version found")
	}
	msg += fmt.Sprintf("Latest GA Kubernetes version for location %s: %s\n", location, latestPatchVersion)
	return latestPatchVersion, nil
}

// getLatestKubernetesVersionClusterModel returns a cluster model with the latest GA Kubernetes version.
func getLatestKubernetesVersionClusterModel(ctx context.Context, name, location, k8sSystemPoolSKU string) (*armcontainerservice.ManagedCluster, error) {
	version, err := getLatestGAKubernetesVersion(ctx, location)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest GA Kubernetes version: %w", err)
	}
	model := getBaseClusterModel(name, location, k8sSystemPoolSKU)
	model.Properties.KubernetesVersion = to.Ptr(version)
	return model, nil
}

func getKubenetClusterModel(name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	model := getBaseClusterModel(name, location, k8sSystemPoolSKU)
	model.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginKubenet)
	return model
}

func getAzureOverlayNetworkClusterModel(name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	model := getBaseClusterModel(name, location, k8sSystemPoolSKU)
	model.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginAzure)
	model.Properties.NetworkProfile.NetworkPluginMode = to.Ptr(armcontainerservice.NetworkPluginModeOverlay)
	return model
}

func getAzureOverlayNetworkDualStackClusterModel(name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	model := getAzureOverlayNetworkClusterModel(name, location, k8sSystemPoolSKU)

	model.Properties.NetworkProfile.IPFamilies = []*armcontainerservice.IPFamily{
		to.Ptr(armcontainerservice.IPFamilyIPv4),
		to.Ptr(armcontainerservice.IPFamilyIPv6),
	}

	networkProfile := model.Properties.NetworkProfile
	networkProfile.PodCidr = to.Ptr("10.244.0.0/16")
	networkProfile.PodCidrs = []*string{
		networkProfile.PodCidr,
		to.Ptr("fd12:3456:789a::/64 "),
	}
	networkProfile.ServiceCidr = to.Ptr("172.16.0.0/16")
	networkProfile.ServiceCidrs = []*string{
		networkProfile.ServiceCidr,
		to.Ptr("fd12:3456:789a:1::/108"),
	}

	networkProfile.PodCidr = nil
	networkProfile.PodCidrs = nil
	networkProfile.ServiceCidr = nil
	networkProfile.ServiceCidrs = nil

	return model
}

func getAzureNetworkClusterModel(name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	cluster := getBaseClusterModel(name, location, k8sSystemPoolSKU)
	cluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginAzure)
	if cluster.Properties.AgentPoolProfiles != nil {
		for _, app := range cluster.Properties.AgentPoolProfiles {
			app.MaxPods = to.Ptr[int32](30)
		}
	}
	return cluster
}
func getCiliumNetworkClusterModel(name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	cluster := getBaseClusterModel(name, location, k8sSystemPoolSKU)
	cluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginAzure)
	cluster.Properties.NetworkProfile.NetworkDataplane = to.Ptr(armcontainerservice.NetworkDataplaneCilium)
	cluster.Properties.NetworkProfile.NetworkPolicy = to.Ptr(armcontainerservice.NetworkPolicyCilium)
	if cluster.Properties.AgentPoolProfiles != nil {
		for _, app := range cluster.Properties.AgentPoolProfiles {
			app.MaxPods = to.Ptr[int32](30)
		}
	}
	return cluster
}

func getBaseClusterModel(clusterName, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	return &armcontainerservice.ManagedCluster{
		Name:     to.Ptr(clusterName),
		Location: to.Ptr(location),
		Properties: &armcontainerservice.ManagedClusterProperties{
			DNSPrefix: to.Ptr(clusterName),
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Name:         to.Ptr("nodepool1"),
					Count:        to.Ptr[int32](1),
					VMSize:       to.Ptr(k8sSystemPoolSKU),
					MaxPods:      to.Ptr[int32](110),
					OSType:       to.Ptr(armcontainerservice.OSTypeLinux),
					Type:         to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
					Mode:         to.Ptr(armcontainerservice.AgentPoolModeSystem),
					OSDiskSizeGB: to.Ptr[int32](512),
				},
			},
			AutoUpgradeProfile: &armcontainerservice.ManagedClusterAutoUpgradeProfile{
				NodeOSUpgradeChannel: to.Ptr(armcontainerservice.NodeOSUpgradeChannelNodeImage),
				UpgradeChannel:       to.Ptr(armcontainerservice.UpgradeChannelNone),
			},
			NetworkProfile: &armcontainerservice.NetworkProfile{
				NetworkPlugin: to.Ptr(armcontainerservice.NetworkPluginKubenet),
				ServiceCidr:   to.Ptr("172.16.0.0/16"),
				DNSServiceIP:  to.Ptr("172.16.0.10"),
			},
			AddonProfiles: map[string]*armcontainerservice.ManagedClusterAddonProfile{
				"omsagent": {
					Enabled: to.Ptr(false),
				},
			},
			LinuxProfile: &armcontainerservice.LinuxProfile{
				AdminUsername: to.Ptr("azureuser"),
				SSH: &armcontainerservice.SSHConfiguration{
					PublicKeys: []*armcontainerservice.SSHPublicKey{
						{
							KeyData: to.Ptr(string(config.SysSSHPublicKey)),
						},
					},
				},
			},
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: to.Ptr(armcontainerservice.ResourceIdentityTypeSystemAssigned),
		},
	}
}

func getFirewall(ctx context.Context, location, firewallSubnetID, publicIPID string) *armnetwork.AzureFirewall {
	var (
		natRuleCollections []*armnetwork.AzureFirewallNatRuleCollection
		netRuleCollections []*armnetwork.AzureFirewallNetworkRuleCollection
	)

	// Application rule for AKS FQDN tags
	aksAppRule := armnetwork.AzureFirewallApplicationRule{
		Name:            to.Ptr("aks-fqdn"),
		SourceAddresses: []*string{to.Ptr("*")},
		Protocols: []*armnetwork.AzureFirewallApplicationRuleProtocol{
			{
				ProtocolType: to.Ptr(armnetwork.AzureFirewallApplicationRuleProtocolTypeHTTP),
				Port:         to.Ptr[int32](80),
			},
			{
				ProtocolType: to.Ptr(armnetwork.AzureFirewallApplicationRuleProtocolTypeHTTPS),
				Port:         to.Ptr[int32](443),
			},
		},
		FqdnTags: []*string{to.Ptr("AzureKubernetesService")},
	}

	// needed for scriptless e2e hack
	blobStorageFqdn := config.Config.BlobStorageAccount() + ".blob.core.windows.net"
	blobStorageAppRule := armnetwork.AzureFirewallApplicationRule{
		Name:            to.Ptr("blob-storage-fqdn"),
		SourceAddresses: []*string{to.Ptr("*")},
		Protocols: []*armnetwork.AzureFirewallApplicationRuleProtocol{
			{
				ProtocolType: to.Ptr(armnetwork.AzureFirewallApplicationRuleProtocolTypeHTTPS),
				Port:         to.Ptr[int32](443),
			},
		},
		TargetFqdns: []*string{to.Ptr(blobStorageFqdn)},
	}

	// needed for Mock Azure China Cloud tests
	mooncakeMAR := "mcr.azure.cn"
	mooncakeMARData := "*.data.mcr.azure.cn"
	mooncakeMARRule := armnetwork.AzureFirewallApplicationRule{
		Name:            to.Ptr("mooncake-mar-fqdn"),
		SourceAddresses: []*string{to.Ptr("*")},
		Protocols: []*armnetwork.AzureFirewallApplicationRuleProtocol{
			{
				ProtocolType: to.Ptr(armnetwork.AzureFirewallApplicationRuleProtocolTypeHTTPS),
				Port:         to.Ptr[int32](443),
			},
		},
		TargetFqdns: []*string{to.Ptr(mooncakeMAR), to.Ptr(mooncakeMARData)},
	}

	// Needed for access to download.microsoft.com
	// This is currently only needed by the Supernova (MA35D) SKU GPU tests
	// Driver install code in setupAmdAma() depends on this
	dmcRule := armnetwork.AzureFirewallApplicationRule{
		Name:            to.Ptr("dmc-fqdn"),
		SourceAddresses: []*string{to.Ptr("*")},
		Protocols: []*armnetwork.AzureFirewallApplicationRuleProtocol{
			{
				ProtocolType: to.Ptr(armnetwork.AzureFirewallApplicationRuleProtocolTypeHTTPS),
				Port:         to.Ptr[int32](443),
			},
		},
		TargetFqdns: []*string{to.Ptr("download.microsoft.com")},
	}

	appRuleCollection := armnetwork.AzureFirewallApplicationRuleCollection{
		Name: to.Ptr("aksfwar"),
		Properties: &armnetwork.AzureFirewallApplicationRuleCollectionPropertiesFormat{
			Priority: to.Ptr[int32](100),
			Action: &armnetwork.AzureFirewallRCAction{
				Type: to.Ptr(armnetwork.AzureFirewallRCActionTypeAllow),
			},
			Rules: []*armnetwork.AzureFirewallApplicationRule{&aksAppRule, &blobStorageAppRule, &mooncakeMARRule, &dmcRule},
		},
	}

	ipConfigurations := []*armnetwork.AzureFirewallIPConfiguration{
		{
			Name: to.Ptr("firewall-ip-config"),
			Properties: &armnetwork.AzureFirewallIPConfigurationPropertiesFormat{
				Subnet: &armnetwork.SubResource{
					ID: to.Ptr(firewallSubnetID),
				},
				PublicIPAddress: &armnetwork.SubResource{
					ID: to.Ptr(publicIPID),
				},
			},
		},
	}

	toolkit.Logf(ctx, "Firewall rules configured successfully")
	return &armnetwork.AzureFirewall{
		Location: to.Ptr(location),
		Properties: &armnetwork.AzureFirewallPropertiesFormat{
			ApplicationRuleCollections: []*armnetwork.AzureFirewallApplicationRuleCollection{&appRuleCollection},
			NetworkRuleCollections:     netRuleCollections,
			NatRuleCollections:         natRuleCollections,
			IPConfigurations:           ipConfigurations,
		},
	}
}

func addFirewallRules(
	ctx context.Context, infra *SharedInfra, clusterModel *armcontainerservice.ManagedCluster,
) error {
	defer toolkit.LogStepCtx(ctx, "adding firewall rules")()

	nodeRG := *clusterModel.Properties.NodeResourceGroup
	vnet, err := getClusterVNet(ctx, clusterModel)
	if err != nil {
		return err
	}

	firewallPrivateIP := infra.FirewallIP

	// For kubenet, the AKS-managed route table must stay attached so that pod
	// routes (managed by cloud-provider-azure) and firewall routes coexist.
	// For Azure CNI variants, the subnet may not have any route table, so we
	// create and associate a dedicated one before adding the firewall routes.
	aksSubnetResp, err := config.Azure.Subnet.Get(ctx, vnet.resourceGroup, vnet.name, vnet.subnetName, nil)
	if err != nil {
		return fmt.Errorf("failed to get AKS subnet: %w", err)
	}
	aksRTName, err := ensureFirewallRouteTable(ctx, clusterModel, vnet, aksSubnetResp.Subnet)
	if err != nil {
		return err
	}

	// Add firewall routes to the existing AKS route table using individual
	// route operations. This avoids replacing the entire table (which would
	// race with cloud-provider-azure pod route updates) and preserves the
	// subnet association so pod CIDR routes remain active.
	firewallRoutes := []armnetwork.Route{
		{
			Name: to.Ptr("vnet-local"),
			Properties: &armnetwork.RoutePropertiesFormat{
				AddressPrefix: to.Ptr(vnet.addressPrefix),
				NextHopType:   to.Ptr(armnetwork.RouteNextHopTypeVnetLocal),
			},
		},
		{
			Name: to.Ptr("default-route-to-firewall"),
			Properties: &armnetwork.RoutePropertiesFormat{
				AddressPrefix:    to.Ptr("0.0.0.0/0"),
				NextHopType:      to.Ptr(armnetwork.RouteNextHopTypeVirtualAppliance),
				NextHopIPAddress: to.Ptr(firewallPrivateIP),
			},
		},
	}

	for _, route := range firewallRoutes {
		toolkit.Logf(ctx, "Adding route %q to AKS route table %q", *route.Name, aksRTName)
		poller, err := config.Azure.Routes.BeginCreateOrUpdate(ctx, nodeRG, aksRTName, *route.Name, route, nil)
		if err != nil {
			return fmt.Errorf("failed to start adding route %q: %w", *route.Name, err)
		}
		_, err = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
		if err != nil {
			return fmt.Errorf("failed to add route %q to AKS route table: %w", *route.Name, err)
		}
	}

	toolkit.Logf(ctx, "Successfully added firewall routes to AKS route table %q", aksRTName)
	return nil
}

func ensureFirewallRouteTable(
	ctx context.Context,
	clusterModel *armcontainerservice.ManagedCluster,
	vnet VNet,
	aksSubnet armnetwork.Subnet,
) (string, error) {
	if aksSubnet.Properties == nil {
		return "", fmt.Errorf("AKS subnet has no properties")
	}
	if aksSubnet.Properties.RouteTable != nil && aksSubnet.Properties.RouteTable.ID != nil {
		aksRTID := *aksSubnet.Properties.RouteTable.ID
		parsedRT, err := arm.ParseResourceID(aksRTID)
		if err != nil {
			return "", fmt.Errorf("failed to parse AKS route table resource ID %q: %w", aksRTID, err)
		}
		if parsedRT.Name == "" {
			return "", fmt.Errorf("parsed empty route table name from resource ID %q", aksRTID)
		}
		return parsedRT.Name, nil
	}

	if clusterModel.Properties == nil || clusterModel.Properties.NetworkProfile == nil || clusterModel.Properties.NetworkProfile.NetworkPlugin == nil {
		return "", fmt.Errorf("AKS subnet has no route table associated and cluster network plugin is unknown")
	}
	if *clusterModel.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginKubenet {
		return "", fmt.Errorf("AKS subnet has no route table associated for kubenet cluster")
	}

	rg := *clusterModel.Properties.NodeResourceGroup
	routeTableName := "abe2e-fw-rt"
	toolkit.Logf(ctx, "AKS subnet has no route table; creating dedicated firewall route table %q", routeTableName)

	var routeTableID *string
	err := retryOn409(ctx, fmt.Sprintf("creating route table %s", routeTableName), func() error {
		poller, err := config.Azure.RouteTables.BeginCreateOrUpdate(ctx, rg, routeTableName, armnetwork.RouteTable{
			Location: clusterModel.Location,
		}, nil)
		if err != nil {
			return fmt.Errorf("failed to start creating firewall route table %q: %w", routeTableName, err)
		}
		routeTableResp, err := poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
		if err != nil {
			return fmt.Errorf("failed to create firewall route table %q: %w", routeTableName, err)
		}
		routeTableID = routeTableResp.ID
		return nil
	})
	if err != nil {
		return "", err
	}

	aksSubnet.Properties.RouteTable = &armnetwork.RouteTable{
		ID: routeTableID,
	}
	if err := updateSubnet(ctx, clusterModel, aksSubnet, vnet); err != nil {
		return "", fmt.Errorf("failed to associate firewall route table %q with AKS subnet: %w", routeTableName, err)
	}

	return routeTableName, nil
}

func addPrivateAzureContainerRegistry(ctx context.Context, cluster *armcontainerservice.ManagedCluster, kube *Kubeclient, kubeletIdentity *armcontainerservice.UserAssignedIdentity, isNonAnonymousPull bool) error {
	if cluster == nil || kube == nil || kubeletIdentity == nil {
		return errors.New("cluster, kubeclient, and kubeletIdentity cannot be nil when adding Private Azure Container Registry")
	}
	resourceGroupName := config.ResourceGroupName(*cluster.Location)
	if err := createPrivateAzureContainerRegistry(ctx, cluster, resourceGroupName, isNonAnonymousPull); err != nil {
		return fmt.Errorf("failed to create private acr: %w", err)
	}

	if err := createPrivateAzureContainerRegistryPullSecret(ctx, cluster, kube, resourceGroupName, isNonAnonymousPull); err != nil {
		return fmt.Errorf("create private acr pull secret: %w", err)
	}
	vnet, err := getClusterVNet(ctx, cluster)
	if err != nil {
		return err
	}

	err = addPrivateEndpointForACR(ctx, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), vnet, *cluster.Location)
	if err != nil {
		return err
	}

	if err := assignACRPullToIdentity(ctx, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), *kubeletIdentity.ObjectID, *cluster.Location); err != nil {
		return fmt.Errorf("assigning acr pull permissions to kubelet identity: %w", err)
	}

	return nil
}

func addNetworkIsolatedSettings(ctx context.Context, clusterModel *armcontainerservice.ManagedCluster) error {
	location := *clusterModel.Location
	defer toolkit.LogStepCtx(ctx, fmt.Sprintf("Adding network settings for network isolated cluster %s in rg %s", *clusterModel.Name, *clusterModel.Properties.NodeResourceGroup))

	vnet, err := getClusterVNet(ctx, clusterModel)
	if err != nil {
		return err
	}

	// The subnet is long-lived and shared across test runs. Once the NSG is
	// associated we never need to touch it again. Private endpoints from
	// previous runs can leave lingering IP prefix allocations on the subnet
	// that make any PUT fail with InUsePrefixCannotBeDeleted, so we skip
	// the update entirely when the NSG is already in place.
	currentSubnet, err := config.Azure.Subnet.Get(ctx, vnet.resourceGroup, vnet.name, vnet.subnetName, nil)
	if err != nil {
		return fmt.Errorf("getting subnet %s: %w", vnet.subnetName, err)
	}
	if currentSubnet.Properties != nil && currentSubnet.Properties.NetworkSecurityGroup != nil {
		toolkit.Logf(ctx, "subnet %s already has NSG, skipping update", vnet.subnetName)
		return nil
	}

	nsgParams, err := networkIsolatedSecurityGroup(location, *clusterModel.Properties.Fqdn)
	if err != nil {
		return err
	}
	nsg, err := createNetworkIsolatedSecurityGroup(ctx, clusterModel, nsgParams, nil)
	if err != nil {
		return err
	}

	if err = updateSubnet(ctx, clusterModel, armnetwork.Subnet{
		ID: to.Ptr(vnet.subnetId),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: currentSubnet.Properties.AddressPrefix,
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: nsg.ID,
			},
		},
	}, vnet); err != nil {
		// After a cluster is GC'd, private endpoint IP prefix allocations can
		// linger on the subnet. The merge in updateSubnet preserves them, but
		// if Azure still rejects the PUT we log and continue — the NSG will be
		// associated on the next run once allocations clear.
		if strings.Contains(err.Error(), "InUsePrefixCannotBeDeleted") {
			toolkit.Logf(ctx, "warning: cannot update subnet %s (lingering IP allocations), will retry next run", vnet.subnetName)
			return nil
		}
		return err
	}

	toolkit.Logf(ctx, "updated cluster %s subnet with network isolated settings", *clusterModel.Name)
	return nil
}

func networkIsolatedSecurityGroup(location, clusterFQDN string) (armnetwork.SecurityGroup, error) {
	requiredRules, err := getRequiredSecurityRules(clusterFQDN)
	if err != nil {
		return armnetwork.SecurityGroup{}, fmt.Errorf("failed to get required security rules for network isolated resource group: %w", err)
	}

	allowVnet := &armnetwork.SecurityRule{
		Name: to.Ptr("AllowVnetOutBound"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
			SourceAddressPrefix:      to.Ptr("VirtualNetwork"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("VirtualNetwork"),
			DestinationPortRange:     to.Ptr("*"),
			Priority:                 to.Ptr[int32](2000),
		},
	}

	blockOutbound := &armnetwork.SecurityRule{
		Name: to.Ptr("block-all-outbound"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
			SourceAddressPrefix:      to.Ptr("*"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("*"),
			DestinationPortRange:     to.Ptr("*"),
			Priority:                 to.Ptr[int32](2001),
		},
	}

	rules := append([]*armnetwork.SecurityRule{allowVnet, blockOutbound}, requiredRules...)

	return armnetwork.SecurityGroup{
		Location:   &location,
		Name:       &config.Config.NetworkIsolatedNSGName,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{SecurityRules: rules},
	}, nil
}

// ensurePrivateDNSZone creates the private DNS zone and VNet link in the
// shared RG. This is a VNet-level resource (one per cluster, not per ACR)
// so it must run once before any private endpoint setup.
//
// Old clusters may have created their own privatelink.azurecr.io zones in
// MC_ RGs and linked our shared VNet to them. A VNet can only link to one
// zone per namespace, so we clean up stale links first.
func ensurePrivateDNSZone(ctx context.Context, vnet VNet) (*armprivatedns.PrivateZone, error) {
	sharedRG := vnet.resourceGroup
	privateZoneName := "privatelink.azurecr.io"

	if err := cleanupConflictingDNSZoneLinks(ctx, vnet, sharedRG, privateZoneName); err != nil {
		return nil, fmt.Errorf("cleaning up conflicting DNS zone links: %w", err)
	}

	zone, err := createPrivateZone(ctx, sharedRG, privateZoneName)
	if err != nil {
		return nil, fmt.Errorf("creating private DNS zone: %w", err)
	}

	if err = createPrivateDNSLink(ctx, vnet, sharedRG, privateZoneName); err != nil {
		return nil, fmt.Errorf("creating private DNS VNet link: %w", err)
	}

	return zone, nil
}

// cleanupConflictingDNSZoneLinks finds privatelink.azurecr.io zones in other
// resource groups that have a VNet link pointing to our shared VNet, and
// deletes those links. This prevents "overlapping namespaces" errors.
func cleanupConflictingDNSZoneLinks(ctx context.Context, vnet VNet, sharedRG, privateZoneName string) error {
	sharedVNetID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s",
		config.Config.SubscriptionID, vnet.resourceGroup, vnet.name)

	subPager := config.Azure.PrivateZonesClient.NewListPager(nil)
	for subPager.More() {
		page, err := subPager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing private DNS zones: %w", err)
		}
		for _, zone := range page.Value {
			if zone.Name == nil || *zone.Name != privateZoneName {
				continue
			}
			zoneRG := resourceGroupFromID(*zone.ID)
			if strings.EqualFold(zoneRG, sharedRG) {
				continue
			}
			if err := deleteVNetLinkIfPointsToSharedVNet(ctx, zoneRG, privateZoneName, sharedVNetID); err != nil {
				toolkit.Logf(ctx, "warning: failed to clean up DNS link in %s: %v", zoneRG, err)
			}
		}
	}
	return nil
}

// deleteVNetLinkIfPointsToSharedVNet checks all VNet links on a DNS zone and
// deletes any that point to our shared VNet.
func deleteVNetLinkIfPointsToSharedVNet(ctx context.Context, zoneRG, zoneName, sharedVNetID string) error {
	linkPager := config.Azure.VirutalNetworkLinksClient.NewListPager(zoneRG, zoneName, nil)
	for linkPager.More() {
		page, err := linkPager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing VNet links in %s/%s: %w", zoneRG, zoneName, err)
		}
		for _, link := range page.Value {
			if link.Properties == nil || link.Properties.VirtualNetwork == nil || link.Properties.VirtualNetwork.ID == nil {
				continue
			}
			if !strings.EqualFold(*link.Properties.VirtualNetwork.ID, sharedVNetID) {
				continue
			}
			toolkit.Logf(ctx, "deleting conflicting DNS zone link %s in %s/%s (points to shared VNet)", *link.Name, zoneRG, zoneName)
			poller, err := config.Azure.VirutalNetworkLinksClient.BeginDelete(ctx, zoneRG, zoneName, *link.Name, nil)
			if err != nil {
				return fmt.Errorf("deleting VNet link %s: %w", *link.Name, err)
			}
			if _, err = poller.PollUntilDone(ctx, nil); err != nil {
				return fmt.Errorf("waiting for VNet link %s deletion: %w", *link.Name, err)
			}
		}
	}
	return nil
}

// resourceGroupFromID extracts the resource group name from an Azure resource ID.
func resourceGroupFromID(id string) string {
	parts := strings.Split(id, "/")
	for i, part := range parts {
		if strings.EqualFold(part, "resourceGroups") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func addPrivateEndpointForACR(ctx context.Context, privateACRName string, vnet VNet, location string) error {
	sharedRG := vnet.resourceGroup
	privateZoneName := "privatelink.azurecr.io"

	// PEs live in the shared RG on the PE subnet so all clusters share one
	// PE per ACR with a single IP address → single DNS A record.
	peSubnetID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
		config.Config.SubscriptionID, sharedRG, vnet.name, PESubnetName)
	peVnet := VNet{
		resourceGroup: sharedRG,
		name:          vnet.name,
		subnetName:    PESubnetName,
		subnetId:      peSubnetID,
	}

	privateEndpointName := fmt.Sprintf("PE-for-%s", privateACRName)
	toolkit.Logf(ctx, "ensuring private endpoint %s in shared RG %s", privateEndpointName, sharedRG)
	privateEndpoint, err := createPrivateEndpoint(ctx, sharedRG, privateEndpointName, privateACRName, peVnet, location)
	if err != nil {
		return err
	}

	if err = addRecordSetToPrivateDNSZone(ctx, privateEndpoint, sharedRG, sharedRG, privateZoneName); err != nil {
		return err
	}
	return nil
}

func createPrivateAzureContainerRegistryPullSecret(ctx context.Context, cluster *armcontainerservice.ManagedCluster, kubeconfig *Kubeclient, resourceGroup string, isNonAnonymousPull bool) error {
	privateACRName := config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location)
	if isNonAnonymousPull {
		toolkit.Logf(ctx, "Creating the secret for non-anonymous pull ACR for the e2e debug pods")
		kubeconfigPath := os.Getenv("HOME") + "/.kube/config"
		if err := fetchAndSaveKubeconfig(ctx, resourceGroup, *cluster.Name, kubeconfigPath); err != nil {
			toolkit.Logf(ctx, "failed to fetch kubeconfig: %v", err)
			return err
		}
		username, password, err := getAzureContainerRegistryCredentials(ctx, resourceGroup, privateACRName)
		if err != nil {
			toolkit.Logf(ctx, "failed to get private ACR credentials: %v", err)
			return err
		}
		if err := kubeconfig.createKubernetesSecret(ctx, "default", config.Config.ACRSecretName, privateACRName, username, password); err != nil {
			toolkit.Logf(ctx, "failed to create Kubernetes secret: %v", err)
			return err
		}
	}
	return nil
}

func createPrivateAzureContainerRegistry(ctx context.Context, cluster *armcontainerservice.ManagedCluster, resourceGroup string, isNonAnonymousPull bool) error {
	privateACRName := config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location)
	toolkit.Logf(ctx, "Creating private Azure Container Registry %s in rg %s", privateACRName, resourceGroup)

	acr, err := config.Azure.RegistriesClient.Get(ctx, resourceGroup, privateACRName, nil)
	if err == nil {
		err, recreateACR := shouldRecreateACR(ctx, resourceGroup, privateACRName)
		if err != nil {
			return fmt.Errorf("failed to check cache rules: %w", err)
		}
		if !recreateACR {
			toolkit.Logf(ctx, "Private ACR already exists at id %s, skipping creation", *acr.ID)
			return nil
		}
		toolkit.Logf(ctx, "Private ACR exists with the wrong cache deleting...")
		if err := deletePrivateAzureContainerRegistry(ctx, resourceGroup, privateACRName); err != nil {
			return fmt.Errorf("failed to delete private acr: %w", err)
		}
		// if ACR gets recreated so should the cluster
		toolkit.Logf(ctx, "Private ACR deleted, deleting cluster %s", *cluster.Name)
		if err := deleteCluster(ctx, *cluster.Name, resourceGroup); err != nil {
			return fmt.Errorf("failed to delete cluster: %w", err)
		}
	} else {
		// check if error is anything but not found
		var azErr *azcore.ResponseError
		if errors.As(err, &azErr) && azErr.StatusCode != 404 {
			return fmt.Errorf("failed to get private ACR: %w", err)
		}
	}

	toolkit.Logf(ctx, "ACR does not exist, creating...")
	createParams := armcontainerregistry.Registry{
		Location: to.Ptr(*cluster.Location),
		SKU: &armcontainerregistry.SKU{
			Name: to.Ptr(armcontainerregistry.SKUNamePremium),
		},
		Properties: &armcontainerregistry.RegistryProperties{
			AdminUserEnabled:     to.Ptr(isNonAnonymousPull),  // if non-anonymous pull is enabled, admin user must be enabled to be able to set credentials for the debug pods
			AnonymousPullEnabled: to.Ptr(!isNonAnonymousPull), // required to pull images from the private ACR without authentication
		},
	}
	pollerResp, err := config.Azure.RegistriesClient.BeginCreate(
		ctx,
		resourceGroup,
		privateACRName,
		createParams,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create private ACR in BeginCreate: %w", err)
	}
	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create private ACR during polling: %w", err)
	}

	toolkit.Logf(ctx, "Private Azure Container Registry created")

	if err := addCacheRulesToPrivateAzureContainerRegistry(ctx, config.ResourceGroupName(*cluster.Location), privateACRName); err != nil {
		return fmt.Errorf("failed to add cache rules to private acr: %w", err)
	}

	return nil
}

func getAzureContainerRegistryCredentials(ctx context.Context, resourceGroup, privateACRName string) (string, string, error) {
	toolkit.Logf(ctx, "Getting credentials for private Azure Container Registry in rg %s", resourceGroup)
	acrCreds, err := config.Azure.RegistriesClient.ListCredentials(ctx, resourceGroup, privateACRName, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to get private ACR credentials: %w", err)
	}
	username := *acrCreds.Username
	password := *acrCreds.Passwords[0].Value
	toolkit.Logf(ctx, "Private Azure Container Registry credentials retrieved")
	return username, password, nil
}

func fetchAndSaveKubeconfig(ctx context.Context, resourceGroup, clusterName, kubeconfigPath string) error {
	adminCredentials, err := config.Azure.AKS.ListClusterAdminCredentials(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster admin credentials: %w", err)
	}
	if len(adminCredentials.Kubeconfigs) == 0 {
		return fmt.Errorf("no kubeconfig returned for cluster %s", clusterName)
	}

	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0700); err != nil {
		return fmt.Errorf("failed to create kubeconfig directory: %w", err)
	}
	if err := os.WriteFile(kubeconfigPath, adminCredentials.Kubeconfigs[0].Value, 0600); err != nil {
		return fmt.Errorf("failed to save kubeconfig to %s: %w", kubeconfigPath, err)
	}
	toolkit.Logf(ctx, "Kubeconfig successfully saved to %s", kubeconfigPath)
	return nil
}

func deletePrivateAzureContainerRegistry(ctx context.Context, resourceGroup, privateACRName string) error {
	toolkit.Logf(ctx, "Deleting private Azure Container Registry in rg %s", resourceGroup)

	pollerResp, err := config.Azure.RegistriesClient.BeginDelete(ctx, resourceGroup, privateACRName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete private ACR: %w", err)
	}
	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete private ACR during polling: %w", err)
	}
	toolkit.Logf(ctx, "Private Azure Container Registry deleted")
	return nil
}

// if the ACR needs to be recreated so does the network isolated k8s cluster
func shouldRecreateACR(ctx context.Context, resourceGroup, privateACRName string) (error, bool) {
	toolkit.Logf(ctx, "Checking if private Azure Container Registry cache rules are correct in rg %s", resourceGroup)

	cacheRules, err := config.Azure.CacheRulesClient.Get(ctx, resourceGroup, privateACRName, "aks-managed-rule", nil)
	if err != nil {
		var azErr *azcore.ResponseError
		if errors.As(err, &azErr) && azErr.StatusCode == 404 {
			toolkit.Logf(ctx, "Private ACR cache not found, need to recreate")
			return nil, true
		}
		return fmt.Errorf("failed to get cache rules: %w", err), false
	}
	if cacheRules.Properties != nil && cacheRules.Properties.TargetRepository != nil && *cacheRules.Properties.TargetRepository != config.Config.AzureContainerRegistrytargetRepository {
		toolkit.Logf(ctx, "Private ACR cache is not correct: %s", *cacheRules.Properties.TargetRepository)
		return nil, true
	}
	toolkit.Logf(ctx, "Private ACR cache is correct")
	return nil, false
}

func addCacheRulesToPrivateAzureContainerRegistry(ctx context.Context, resourceGroup, privateACRName string) error {
	toolkit.Logf(ctx, "Adding cache rules to private Azure Container Registry in rg %s", resourceGroup)

	cacheParams := armcontainerregistry.CacheRule{
		Properties: &armcontainerregistry.CacheRuleProperties{
			SourceRepository: to.Ptr("mcr.microsoft.com/*"),
			TargetRepository: to.Ptr(config.Config.AzureContainerRegistrytargetRepository),
		},
	}
	cacheCreateResp, err := config.Azure.CacheRulesClient.BeginCreate(
		ctx,
		resourceGroup,
		privateACRName,
		"aks-managed-rule",
		cacheParams,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create cache rule in BeginCreate: %w", err)
	}
	_, err = cacheCreateResp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create cache rule in polling: %w", err)
	}

	toolkit.Logf(ctx, "Cache rule created")
	return nil
}

func createPrivateEndpoint(ctx context.Context, nodeResourceGroup, privateEndpointName, privateACRName string, vnet VNet, location string) (*armnetwork.PrivateEndpoint, error) {
	existingPE, err := config.Azure.PrivateEndpointClient.Get(ctx, nodeResourceGroup, privateEndpointName, nil)
	if err == nil && existingPE.ID != nil {
		toolkit.Logf(ctx, "Private Endpoint already exists with ID: %s", *existingPE.ID)
		return &existingPE.PrivateEndpoint, nil
	}
	if err != nil && !strings.Contains(err.Error(), "ResourceNotFound") {
		return nil, fmt.Errorf("failed to get private endpoint: %w", err)
	}
	toolkit.Logf(ctx, "Creating Private Endpoint in rg %s", nodeResourceGroup)
	acrID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerRegistry/registries/%s", config.Config.SubscriptionID, config.ResourceGroupName(location), privateACRName)

	peParams := armnetwork.PrivateEndpoint{
		Location: to.Ptr(location),
		Properties: &armnetwork.PrivateEndpointProperties{
			Subnet: &armnetwork.Subnet{
				ID: to.Ptr(vnet.subnetId),
			},
			PrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{
				{
					Name: to.Ptr(privateEndpointName),
					Properties: &armnetwork.PrivateLinkServiceConnectionProperties{
						PrivateLinkServiceID: to.Ptr(acrID),
						GroupIDs:             []*string{to.Ptr("registry")},
					},
				},
			},
			CustomDNSConfigs: []*armnetwork.CustomDNSConfigPropertiesFormat{},
		},
	}

	var result armnetwork.PrivateEndpoint
	err = retryOn409(ctx, fmt.Sprintf("creating private endpoint %s", privateEndpointName), func() error {
		poller, err := config.Azure.PrivateEndpointClient.BeginCreateOrUpdate(
			ctx,
			nodeResourceGroup,
			privateEndpointName,
			peParams,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create private endpoint in BeginCreateOrUpdate: %w", err)
		}
		resp, err := poller.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to create private endpoint in polling: %w", err)
		}
		result = resp.PrivateEndpoint
		return nil
	})
	if err != nil {
		return nil, err
	}

	toolkit.Logf(ctx, "Private Endpoint created or updated with ID: %s", *result.ID)
	return &result, nil
}

func createPrivateZone(ctx context.Context, nodeResourceGroup, privateZoneName string) (*armprivatedns.PrivateZone, error) {
	pzResp, err := config.Azure.PrivateZonesClient.Get(
		ctx,
		nodeResourceGroup,
		privateZoneName,
		nil,
	)
	if err == nil {
		return &pzResp.PrivateZone, nil
	}
	dnsZoneParams := armprivatedns.PrivateZone{
		Location: to.Ptr("global"),
	}
	poller, err := config.Azure.PrivateZonesClient.BeginCreateOrUpdate(
		ctx,
		nodeResourceGroup,
		privateZoneName,
		dnsZoneParams,
		nil,
	)
	if err != nil {
		// 409 means another operation is in progress — wait and re-fetch
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			return waitForPrivateZone(ctx, nodeResourceGroup, privateZoneName)
		}
		return nil, fmt.Errorf("failed to create private dns zone in BeginCreateOrUpdate: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create private dns zone in polling: %w", err)
	}

	toolkit.Logf(ctx, "Private DNS Zone created or updated with ID: %s", *resp.ID)
	return &resp.PrivateZone, nil
}

func waitForPrivateZone(ctx context.Context, nodeResourceGroup, privateZoneName string) (*armprivatedns.PrivateZone, error) {
	defer toolkit.LogStepCtxf(ctx, "waiting for private DNS zone %s (409 conflict)", privateZoneName)()
	var zone *armprivatedns.PrivateZone
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		resp, err := config.Azure.PrivateZonesClient.Get(ctx, nodeResourceGroup, privateZoneName, nil)
		if err != nil {
			var respErr *azcore.ResponseError
			if errors.As(err, &respErr) && respErr.StatusCode == 404 {
				return false, nil // zone doesn't exist yet
			}
			return false, err
		}
		zone = &resp.PrivateZone
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("waiting for private dns zone %q: %w", privateZoneName, err)
	}
	return zone, nil
}

func createPrivateDNSLink(ctx context.Context, vnet VNet, resourceGroup, privateZoneName string) error {
	networkLinkName := "link-ABE2ETests"
	_, err := config.Azure.VirutalNetworkLinksClient.Get(
		ctx,
		resourceGroup,
		privateZoneName,
		networkLinkName,
		nil,
	)

	if err == nil {
		// private dns link already created
		return nil
	}

	vnetRG := vnet.resourceGroup
	if vnetRG == "" {
		vnetRG = resourceGroup
	}
	vnetForId, err := config.Azure.VNet.Get(ctx, vnetRG, vnet.name, nil)
	if err != nil {
		return fmt.Errorf("failed to get vnet: %w", err)
	}
	linkParams := armprivatedns.VirtualNetworkLink{
		Location: to.Ptr("global"),
		Properties: &armprivatedns.VirtualNetworkLinkProperties{
			VirtualNetwork: &armprivatedns.SubResource{
				ID: vnetForId.ID,
			},
			RegistrationEnabled: to.Ptr(false),
		},
	}
	poller, err := config.Azure.VirutalNetworkLinksClient.BeginCreateOrUpdate(
		ctx,
		resourceGroup,
		privateZoneName,
		networkLinkName,
		linkParams,
		nil,
	)
	if err != nil {
		// 409 means another operation is in progress — link is being created by another run
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			toolkit.Logf(ctx, "Virtual network link creation conflict (409), waiting for completion")
			return wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
				_, err := config.Azure.VirutalNetworkLinksClient.Get(ctx, resourceGroup, privateZoneName, networkLinkName, nil)
				if err != nil {
					var respErr *azcore.ResponseError
					if errors.As(err, &respErr) && respErr.StatusCode == 404 {
						return false, nil // link doesn't exist yet
					}
					return false, err
				}
				return true, nil
			})
		}
		return fmt.Errorf("failed to create virtual network link in BeginCreateOrUpdate: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual network link in polling: %w", err)
	}

	toolkit.Logf(ctx, "Virtual Network Link created or updated with ID: %s", *resp.ID)
	return nil
}

// addRecordSetToPrivateDNSZone creates A records in the private DNS zone for a
// private endpoint. It reads the PE's NIC to get the actual private IPs (since
// CustomDNSConfigs is unreliable when DNS zone groups have been used).
func addRecordSetToPrivateDNSZone(ctx context.Context, privateEndpoint *armnetwork.PrivateEndpoint, peResourceGroup, dnsZoneResourceGroup, privateZoneName string) error {
	if privateEndpoint.Properties == nil || len(privateEndpoint.Properties.NetworkInterfaces) == 0 {
		return fmt.Errorf("private endpoint has no network interfaces")
	}

	nicID := *privateEndpoint.Properties.NetworkInterfaces[0].ID
	nicName := nicID[strings.LastIndex(nicID, "/")+1:]
	nic, err := config.Azure.NetworkInterfaces.Get(ctx, peResourceGroup, nicName, nil)
	if err != nil {
		return fmt.Errorf("getting PE NIC %s: %w", nicName, err)
	}

	// Each NIC IP config has a private IP and an associated FQDN from the
	// PE's privateLinkServiceConnections. Create one A record per FQDN.
	for _, ipConfig := range nic.Properties.IPConfigurations {
		if ipConfig.Properties == nil || ipConfig.Properties.PrivateIPAddress == nil {
			continue
		}
		ip := *ipConfig.Properties.PrivateIPAddress

		// The PE's CustomDNSConfigs or PrivateLinkServiceConnections tell us the
		// FQDN, but they may be empty. Use the IP config's
		// PrivateLinkConnectionProperties for the FQDN list.
		if ipConfig.Properties.PrivateLinkConnectionProperties == nil || len(ipConfig.Properties.PrivateLinkConnectionProperties.Fqdns) == 0 {
			continue
		}
		for _, fqdn := range ipConfig.Properties.PrivateLinkConnectionProperties.Fqdns {
			if fqdn == nil {
				continue
			}
			// The NIC returns FQDNs like "myacr.azurecr.io" or
			// "myacr.westus3.data.azurecr.io". The private DNS zone is
			// "privatelink.azurecr.io", so the record name is everything
			// before ".azurecr.io":
			//   "myacr.azurecr.io"                → record "myacr"
			//   "myacr.westus3.data.azurecr.io"   → record "myacr.westus3.data"
			// Azure's CNAME chain maps X.azurecr.io → X.privatelink.azurecr.io
			recordName := strings.TrimSuffix(*fqdn, ".azurecr.io")
			aRecordSet := armprivatedns.RecordSet{
				Properties: &armprivatedns.RecordSetProperties{
					TTL:      to.Ptr[int64](10),
					ARecords: []*armprivatedns.ARecord{{IPv4Address: to.Ptr(ip)}},
				},
			}
			_, err := config.Azure.RecordSetClient.CreateOrUpdate(ctx, dnsZoneResourceGroup, privateZoneName, armprivatedns.RecordTypeA, recordName, aRecordSet, nil)
			if err != nil {
				return fmt.Errorf("failed to create A record %s → %s: %w", recordName, ip, err)
			}
			toolkit.Logf(ctx, "DNS A record: %s.%s → %s", recordName, privateZoneName, ip)
		}
	}
	return nil
}

func getRequiredSecurityRules(clusterFQDN string) ([]*armnetwork.SecurityRule, error) {
	// https://learn.microsoft.com/en-us/azure/aks/outbound-rules-control-egress#azure-global-required-fqdn--application-rules
	// note that we explicitly exclude packages.microsoft.com
	requiredDNSNames := []string{
		"management.azure.com",
		clusterFQDN,
		"packages.aks.azure.com",
	}
	var rules []*armnetwork.SecurityRule
	var priority int32 = 100

	for _, dnsName := range requiredDNSNames {
		ips, err := net.LookupIP(dnsName)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup IP for DNS name %s: %w", dnsName, err)
		}
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				rules = append(rules, getSecurityRule(fmt.Sprintf("%s-%d", strings.ReplaceAll(dnsName, ".", "-"), priority), ipv4.String(), priority))
				priority++
			}
		}
	}

	return rules, nil
}

func getSecurityRule(name, destinationAddressPrefix string, priority int32) *armnetwork.SecurityRule {
	return &armnetwork.SecurityRule{
		Name: to.Ptr(name),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
			SourceAddressPrefix:      to.Ptr("*"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr(destinationAddressPrefix),
			DestinationPortRange:     to.Ptr("*"),
			Priority:                 to.Ptr(priority),
		},
	}
}

func createNetworkIsolatedSecurityGroup(ctx context.Context, cluster *armcontainerservice.ManagedCluster, nsgParams armnetwork.SecurityGroup, options *armnetwork.SecurityGroupsClientBeginCreateOrUpdateOptions) (*armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error) {
	poller, err := config.Azure.SecurityGroup.BeginCreateOrUpdate(ctx, *cluster.Properties.NodeResourceGroup, config.Config.NetworkIsolatedNSGName, nsgParams, options)
	if err != nil {
		return nil, err
	}
	nsg, err := poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return nil, err
	}
	return &nsg, nil
}

// updateSubnet reads the current subnet, merges in the desired fields, and PUTs
// the result. This read-modify-write avoids stripping existing properties like
// private endpoint allocations, service endpoints, or delegations — a bare PUT
// with only the desired fields causes Azure to return InUsePrefixCannotBeDeleted.
func updateSubnet(ctx context.Context, cluster *armcontainerservice.ManagedCluster, desired armnetwork.Subnet, vnet VNet) error {
	return retryOn409(ctx, fmt.Sprintf("updating subnet %s", vnet.subnetName), func() error {
		existing, err := config.Azure.Subnet.Get(ctx, vnet.resourceGroup, vnet.name, vnet.subnetName, nil)
		if err != nil {
			return fmt.Errorf("getting subnet %s: %w", vnet.subnetName, err)
		}

		merged := existing.Subnet
		mergeSubnetProperties(merged.Properties, desired.Properties)

		poller, err := config.Azure.Subnet.BeginCreateOrUpdate(ctx, vnet.resourceGroup, vnet.name, vnet.subnetName, merged, nil)
		if err != nil {
			return err
		}
		_, err = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
		return err
	})
}

// mergeSubnetProperties overwrites only the fields that are set in desired.
func mergeSubnetProperties(dst, desired *armnetwork.SubnetPropertiesFormat) {
	if dst == nil || desired == nil {
		return
	}
	if desired.AddressPrefix != nil {
		dst.AddressPrefix = desired.AddressPrefix
	}
	if desired.NetworkSecurityGroup != nil {
		dst.NetworkSecurityGroup = desired.NetworkSecurityGroup
	}
	if desired.RouteTable != nil {
		dst.RouteTable = desired.RouteTable
	}
	if desired.ServiceEndpoints != nil {
		dst.ServiceEndpoints = desired.ServiceEndpoints
	}
}
