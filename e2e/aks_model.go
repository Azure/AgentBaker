package e2e

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerregistry/armcontainerregistry/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
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
	networkProfile.ServiceCidr = to.Ptr("10.0.0.0/16")
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
	ctx context.Context, clusterModel *armcontainerservice.ManagedCluster,
	location string,
) error {
	defer toolkit.LogStepCtx(ctx, "adding firewall rules")()
	routeTableName := "abe2e-fw-rt"
	rtGetResp, err := config.Azure.RouteTables.Get(
		ctx,
		*clusterModel.Properties.NodeResourceGroup,
		routeTableName,
		nil,
	)
	if err == nil && len(rtGetResp.Properties.Subnets) != 0 {
		// already associated with aks subnet
		return nil
	}

	vnet, err := getClusterVNet(ctx, *clusterModel.Properties.NodeResourceGroup)
	if err != nil {
		return err
	}

	// Create AzureFirewallSubnet - this subnet name is required by Azure Firewall
	firewallSubnetName := "AzureFirewallSubnet"
	firewallSubnetParams := armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.225.0.0/24"), // Use a different CIDR that doesn't overlap with 10.224.0.0/16
		},
	}

	toolkit.Logf(ctx, "Creating subnet %s in VNet %s", firewallSubnetName, vnet.name)
	subnetPoller, err := config.Azure.Subnet.BeginCreateOrUpdate(
		ctx,
		*clusterModel.Properties.NodeResourceGroup,
		vnet.name,
		firewallSubnetName,
		firewallSubnetParams,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start creating firewall subnet: %w", err)
	}

	subnetResp, err := subnetPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("failed to create firewall subnet: %w", err)
	}

	firewallSubnetID := *subnetResp.ID
	toolkit.Logf(ctx, "Created firewall subnet with ID: %s", firewallSubnetID)

	// Create public IP for the firewall
	publicIPName := "abe2e-fw-pip"
	publicIPParams := armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}

	toolkit.Logf(ctx, "Creating public IP %s", publicIPName)
	pipPoller, err := config.Azure.PublicIPAddresses.BeginCreateOrUpdate(
		ctx,
		*clusterModel.Properties.NodeResourceGroup,
		publicIPName,
		publicIPParams,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start creating public IP: %w", err)
	}

	pipResp, err := pipPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("failed to create public IP: %w", err)
	}

	publicIPID := *pipResp.ID
	toolkit.Logf(ctx, "Created public IP with ID: %s", publicIPID)

	firewallName := "abe2e-fw"
	firewall := getFirewall(ctx, location, firewallSubnetID, publicIPID)
	fwPoller, err := config.Azure.AzureFirewall.BeginCreateOrUpdate(ctx, *clusterModel.Properties.NodeResourceGroup, firewallName, *firewall, nil)
	if err != nil {
		return fmt.Errorf("failed to start Firewall creation: %w", err)
	}
	fwResp, err := fwPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create Firewall: %w", err)
	}

	// Get the firewall's private IP address
	var firewallPrivateIP string
	if fwResp.Properties != nil && fwResp.Properties.IPConfigurations != nil && len(fwResp.Properties.IPConfigurations) > 0 {
		if fwResp.Properties.IPConfigurations[0].Properties != nil && fwResp.Properties.IPConfigurations[0].Properties.PrivateIPAddress != nil {
			firewallPrivateIP = *fwResp.Properties.IPConfigurations[0].Properties.PrivateIPAddress
			toolkit.Logf(ctx, "Firewall private IP: %s", firewallPrivateIP)
		}
	}

	if firewallPrivateIP == "" {
		return fmt.Errorf("failed to get firewall private IP address")
	}

	routeTableParams := armnetwork.RouteTable{
		Location: to.Ptr(location),
		Properties: &armnetwork.RouteTablePropertiesFormat{
			Routes: []*armnetwork.Route{
				// Allow internal VNet traffic to bypass the firewall
				{
					Name: to.Ptr("vnet-local"),
					Properties: &armnetwork.RoutePropertiesFormat{
						AddressPrefix: to.Ptr("10.224.0.0/16"), // AKS subnet CIDR
						NextHopType:   to.Ptr(armnetwork.RouteNextHopTypeVnetLocal),
					},
				},
				// Route all other traffic (internet-bound) through the firewall
				{
					Name: to.Ptr("default-route-to-firewall"),
					Properties: &armnetwork.RoutePropertiesFormat{
						AddressPrefix:    to.Ptr("0.0.0.0/0"),
						NextHopType:      to.Ptr(armnetwork.RouteNextHopTypeVirtualAppliance),
						NextHopIPAddress: to.Ptr(firewallPrivateIP),
					},
				},
			},
			DisableBgpRoutePropagation: to.Ptr(true),
		},
	}

	toolkit.Logf(ctx, "Creating route table %s", routeTableName)
	rtPoller, err := config.Azure.RouteTables.BeginCreateOrUpdate(
		ctx,
		*clusterModel.Properties.NodeResourceGroup,
		routeTableName,
		routeTableParams,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start creating route table: %w", err)
	}

	rtResp, err := rtPoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("failed to create route table: %w", err)
	}

	toolkit.Logf(ctx, "Created route table with ID: %s", *rtResp.ID)

	// Get the AKS subnet and associate it with the route table
	aksSubnetResp, err := config.Azure.Subnet.Get(ctx, *clusterModel.Properties.NodeResourceGroup, vnet.name, "aks-subnet", nil)
	if err != nil {
		return fmt.Errorf("failed to get AKS subnet: %w", err)
	}

	// Update subnet to associate with route table
	aksSubnet := aksSubnetResp.Subnet
	if aksSubnet.Properties == nil {
		aksSubnet.Properties = &armnetwork.SubnetPropertiesFormat{}
	}
	aksSubnet.Properties.RouteTable = &armnetwork.RouteTable{
		ID: rtResp.ID,
	}

	toolkit.Logf(ctx, "Associating route table with AKS subnet")
	subnetUpdatePoller, err := config.Azure.Subnet.BeginCreateOrUpdate(
		ctx,
		*clusterModel.Properties.NodeResourceGroup,
		vnet.name,
		"aks-subnet",
		aksSubnet,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start updating subnet with route table: %w", err)
	}

	_, err = subnetUpdatePoller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return fmt.Errorf("failed to associate route table with subnet: %w", err)
	}

	toolkit.Logf(ctx, "Successfully configured firewall and routing for AKS cluster")
	return nil
}

func addPrivateAzureContainerRegistry(ctx context.Context, cluster *armcontainerservice.ManagedCluster, kube *Kubeclient, resourceGroupName string, kubeletIdentity *armcontainerservice.UserAssignedIdentity, isNonAnonymousPull bool) error {
	if cluster == nil || kube == nil || kubeletIdentity == nil {
		return errors.New("cluster, kubeclient, and kubeletIdentity cannot be nil when adding Private Azure Container Registry")
	}
	if err := createPrivateAzureContainerRegistry(ctx, cluster, resourceGroupName, isNonAnonymousPull); err != nil {
		return fmt.Errorf("failed to create private acr: %w", err)
	}

	if err := createPrivateAzureContainerRegistryPullSecret(ctx, cluster, kube, resourceGroupName, isNonAnonymousPull); err != nil {
		return fmt.Errorf("create private acr pull secret: %w", err)
	}
	vnet, err := getClusterVNet(ctx, *cluster.Properties.NodeResourceGroup)
	if err != nil {
		return err
	}

	err = addPrivateEndpointForACR(ctx, *cluster.Properties.NodeResourceGroup, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), vnet, *cluster.Location)
	if err != nil {
		return err
	}

	if err := assignACRPullToIdentity(ctx, config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location), *kubeletIdentity.ObjectID, *cluster.Location); err != nil {
		return fmt.Errorf("assigning acr pull permissions to kubelet identity: %w", err)
	}

	return nil
}

func addNetworkIsolatedSettings(ctx context.Context, clusterModel *armcontainerservice.ManagedCluster, location string) error {
	defer toolkit.LogStepCtx(ctx, fmt.Sprintf("Adding network settings for network isolated cluster %s in rg %s", *clusterModel.Name, *clusterModel.Properties.NodeResourceGroup))

	vnet, err := getClusterVNet(ctx, *clusterModel.Properties.NodeResourceGroup)
	if err != nil {
		return err
	}
	subnetId := vnet.subnetId

	nsgParams, err := networkIsolatedSecurityGroup(location, *clusterModel.Properties.Fqdn)
	if err != nil {
		return err
	}

	nsg, err := createNetworkIsolatedSecurityGroup(ctx, clusterModel, nsgParams, nil)
	if err != nil {
		return err
	}

	subnetParameters := armnetwork.Subnet{
		ID: to.Ptr(subnetId),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.224.0.0/16"),
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: nsg.ID,
			},
		},
	}
	if err = updateSubnet(ctx, clusterModel, subnetParameters, vnet.name); err != nil {
		return err
	}

	toolkit.Logf(ctx, "updated cluster %s subnet with network isolated cluster settings", *clusterModel.Name)
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

func addPrivateEndpointForACR(ctx context.Context, nodeResourceGroup, privateACRName string, vnet VNet, location string) error {
	toolkit.Logf(ctx, "Checking if private endpoint for private container registry is in rg %s", nodeResourceGroup)
	var err error
	var privateEndpoint *armnetwork.PrivateEndpoint
	privateEndpointName := fmt.Sprintf("PE-for-%s", privateACRName)
	if privateEndpoint, err = createPrivateEndpoint(ctx, nodeResourceGroup, privateEndpointName, privateACRName, vnet, location); err != nil {
		return err
	}

	privateZoneName := "privatelink.azurecr.io"
	var privateZone *armprivatedns.PrivateZone
	if privateZone, err = createPrivateZone(ctx, nodeResourceGroup, privateZoneName); err != nil {
		return err
	}

	if err = createPrivateDNSLink(ctx, vnet, nodeResourceGroup, privateZoneName); err != nil {
		return err
	}

	if err = addRecordSetToPrivateDNSZone(ctx, privateEndpoint, nodeResourceGroup, privateZoneName); err != nil {
		return err
	}

	if err = addDNSZoneGroup(ctx, privateZone, nodeResourceGroup, privateZoneName, *privateEndpoint.Name); err != nil {
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
	poller, err := config.Azure.PrivateEndpointClient.BeginCreateOrUpdate(
		ctx,
		nodeResourceGroup,
		privateEndpointName,
		peParams,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create private endpoint in BeginCreateOrUpdate: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create private endpoint in polling: %w", err)
	}

	toolkit.Logf(ctx, "Private Endpoint created or updated with ID: %s", *resp.ID)
	return &resp.PrivateEndpoint, nil
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
		return nil, fmt.Errorf("failed to create private dns zone in BeginCreateOrUpdate: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create private dns zone in polling: %w", err)
	}

	toolkit.Logf(ctx, "Private DNS Zone created or updated with ID: %s", *resp.ID)
	return &resp.PrivateZone, nil
}

func createPrivateDNSLink(ctx context.Context, vnet VNet, nodeResourceGroup, privateZoneName string) error {
	networkLinkName := "link-ABE2ETests"
	_, err := config.Azure.VirutalNetworkLinksClient.Get(
		ctx,
		nodeResourceGroup,
		privateZoneName,
		networkLinkName,
		nil,
	)

	if err == nil {
		// private dns link already created
		return nil
	}

	vnetForId, err := config.Azure.VNet.Get(ctx, nodeResourceGroup, vnet.name, nil)
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
		nodeResourceGroup,
		privateZoneName,
		networkLinkName,
		linkParams,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create virtual network link in BeginCreateOrUpdate: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual network link in polling: %w", err)
	}

	toolkit.Logf(ctx, "Virtual Network Link created or updated with ID: %s", *resp.ID)
	return nil
}

func addRecordSetToPrivateDNSZone(ctx context.Context, privateEndpoint *armnetwork.PrivateEndpoint, nodeResourceGroup, privateZoneName string) error {
	for i, dnsConfigPtr := range privateEndpoint.Properties.CustomDNSConfigs {
		var ipAddresses []string
		if dnsConfigPtr == nil {
			return fmt.Errorf("CustomDNSConfigs[%d] is nil", i)
		}

		// get the ip addresses
		dnsConfig := *dnsConfigPtr
		if len(dnsConfig.IPAddresses) == 0 {
			return fmt.Errorf("CustomDNSConfigs[%d].IPAddresses is nil or empty", i)
		}
		for _, ipPtr := range dnsConfig.IPAddresses {
			ipAddresses = append(ipAddresses, *ipPtr)
		}
		if len(ipAddresses) == 0 {
			return fmt.Errorf("IPAddresses is empty")
		}

		aRecords := make([]*armprivatedns.ARecord, len(ipAddresses))
		for i, ip := range ipAddresses {
			aRecords[i] = &armprivatedns.ARecord{IPv4Address: &ip}
		}
		ttl := int64(10)
		aRecordSet := armprivatedns.RecordSet{
			Properties: &armprivatedns.RecordSetProperties{
				TTL:      &ttl,
				ARecords: aRecords,
			},
		}
		_, err := config.Azure.RecordSetClient.CreateOrUpdate(ctx, nodeResourceGroup, privateZoneName, armprivatedns.RecordTypeA, *dnsConfig.Fqdn, aRecordSet, nil)
		if err != nil {
			return fmt.Errorf("failed to create record set: %w", err)
		}
	}

	toolkit.Logf(ctx, "Record Set created or updated")
	return nil
}

func addDNSZoneGroup(ctx context.Context, privateZone *armprivatedns.PrivateZone, nodeResourceGroup, privateZoneName, endpointName string) error {
	groupName := strings.Replace(privateZoneName, ".", "-", -1) // replace . with -
	_, err := config.Azure.PrivateDNSZoneGroup.Get(ctx, nodeResourceGroup, endpointName, groupName, nil)
	if err == nil {
		return nil
	}
	dnsZonegroup := armnetwork.PrivateDNSZoneGroup{
		Name: to.Ptr(fmt.Sprintf("%s/default", privateZoneName)),
		Properties: &armnetwork.PrivateDNSZoneGroupPropertiesFormat{
			PrivateDNSZoneConfigs: []*armnetwork.PrivateDNSZoneConfig{{
				Name: to.Ptr(groupName),
				Properties: &armnetwork.PrivateDNSZonePropertiesFormat{
					PrivateDNSZoneID: privateZone.ID,
				},
			}},
		},
	}
	dnsZoneResp, err := config.Azure.PrivateDNSZoneGroup.BeginCreateOrUpdate(ctx, nodeResourceGroup, endpointName, groupName, dnsZonegroup, nil)
	if err != nil {
		return fmt.Errorf("failed to create private dns zone group in BeginCreateOrUpdate: %w", err)
	}
	_, err = dnsZoneResp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create private dns zone group in polling: %w", err)
	}

	toolkit.Logf(ctx, "Private DNS Zone Group created or updated with ID")
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

func updateSubnet(ctx context.Context, cluster *armcontainerservice.ManagedCluster, subnetParameters armnetwork.Subnet, vnetName string) error {
	poller, err := config.Azure.Subnet.BeginCreateOrUpdate(ctx, *cluster.Properties.NodeResourceGroup, vnetName, config.Config.DefaultSubnetName, subnetParameters, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	if err != nil {
		return err
	}
	return nil
}
