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
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerregistry/armcontainerregistry"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
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
	defer func() { logf(ctx, "%s", msg) }()
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
	model := getBaseClusterModel(ctx, name, location, k8sSystemPoolSKU)
	model.Properties.KubernetesVersion = to.Ptr(version)
	return model, nil
}

func getKubenetClusterModel(ctx context.Context, name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	model := getBaseClusterModel(ctx, name, location, k8sSystemPoolSKU)
	model.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginKubenet)
	return model
}

func getAzureOverlayNetworkClusterModel(ctx context.Context, name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	model := getBaseClusterModel(ctx, name, location, k8sSystemPoolSKU)
	model.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginAzure)
	model.Properties.NetworkProfile.NetworkPluginMode = to.Ptr(armcontainerservice.NetworkPluginModeOverlay)
	return model
}

func getAzureOverlayNetworkDualStackClusterModel(ctx context.Context, name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	model := getAzureOverlayNetworkClusterModel(ctx, name, location, k8sSystemPoolSKU)

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

func getAzureNetworkClusterModel(ctx context.Context, name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	cluster := getBaseClusterModel(ctx, name, location, k8sSystemPoolSKU)
	cluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginAzure)
	if cluster.Properties.AgentPoolProfiles != nil {
		for _, app := range cluster.Properties.AgentPoolProfiles {
			app.MaxPods = to.Ptr[int32](30)
		}
	}
	return cluster
}
func getCiliumNetworkClusterModel(ctx context.Context, name, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	cluster := getBaseClusterModel(ctx, name, location, k8sSystemPoolSKU)
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

func getBaseClusterModel(ctx context.Context, clusterName, location, k8sSystemPoolSKU string) *armcontainerservice.ManagedCluster {
	_, aksSubnet, _, _, _ := createAKSNetworkWithFirewall(
		ctx,
		config.ResourceGroupName(location),
		location,
		clusterName+"-vnet",
		clusterName+"-subnet",
		clusterName+"-fw",
		clusterName+"-pip",
		clusterName+"-fwcfg",
	)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create network and firewall: %w", err)
	// }

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
					VnetSubnetID: aksSubnet.ID,
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
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: to.Ptr(armcontainerservice.ResourceIdentityTypeSystemAssigned),
		},
	}
}

func addFirewallRules(ctx context.Context, resourceGroupName, location, firewallName, publicIPId, firewallSubnetID, ipConfigName string) *armnetwork.AzureFirewall {
	var (
		natRuleCollections []*armnetwork.AzureFirewallNatRuleCollection
		netRuleCollections []*armnetwork.AzureFirewallNetworkRuleCollection
	)

	appRule := armnetwork.AzureFirewallApplicationRule{
		Name:            to.Ptr("fqdn"),
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

	appRuleCollection := armnetwork.AzureFirewallApplicationRuleCollection{
		Name: to.Ptr("aksfwar"),
		Properties: &armnetwork.AzureFirewallApplicationRuleCollectionPropertiesFormat{
			Priority: to.Ptr[int32](100),
			Action: &armnetwork.AzureFirewallRCAction{
				Type: to.Ptr(armnetwork.AzureFirewallRCActionTypeAllow),
			},
			Rules: []*armnetwork.AzureFirewallApplicationRule{&appRule},
		},
	}

	ipConfigurations := []*armnetwork.AzureFirewallIPConfiguration{
		{
			Name: to.Ptr(ipConfigName),
			Properties: &armnetwork.AzureFirewallIPConfigurationPropertiesFormat{
				Subnet: &armnetwork.SubResource{
					ID: to.Ptr(firewallSubnetID),
				},
				PublicIPAddress: &armnetwork.SubResource{
					ID: to.Ptr(publicIPId),
				},
			},
		},
	}

	logf(ctx, "Firewall rules added successfully")
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

func createAKSNetworkWithFirewall(
	ctx context.Context,
	resourceGroupName, location, vnetName, subnetName, firewallName, publicIPName, ipConfigName string,
) (*armnetwork.VirtualNetwork, *armnetwork.Subnet, *armnetwork.PublicIPAddress, *armnetwork.AzureFirewall, error) {
	vnetParams := armnetwork.VirtualNetwork{
		Location: to.Ptr(location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr("10.240.0.0/16")},
			},
			Subnets: []*armnetwork.Subnet{
				{
					Name: to.Ptr(subnetName),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr("10.240.0.0/24"),
					},
				},
				{
					Name: to.Ptr("AzureFirewallSubnet"),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr("10.240.1.0/24"),
					},
				},
			},
		},
	}
	vnetPoller, err := config.Azure.VNet.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, vnetParams, nil)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to start VNet creation: %w", err)
	}
	vnetResp, err := vnetPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create VNet: %w", err)
	}
	vnet := &vnetResp.VirtualNetwork

	var aksSubnet, fwSubnet *armnetwork.Subnet
	for _, s := range vnet.Properties.Subnets {
		if s.Name != nil && *s.Name == subnetName {
			aksSubnet = s
		}
		if s.Name != nil && *s.Name == "AzureFirewallSubnet" {
			fwSubnet = s
		}
	}
	if aksSubnet == nil || fwSubnet == nil {
		return nil, nil, nil, nil, fmt.Errorf("subnets not found after VNet creation")
	}

	pipParams := armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}
	pipPoller, err := config.Azure.PublicIPAddresses.BeginCreateOrUpdate(ctx, resourceGroupName, publicIPName, pipParams, nil)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to start Public IP creation: %w", err)
	}
	pipResp, err := pipPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create Public IP: %w", err)
	}
	publicIP := &pipResp.PublicIPAddress

	firewall := addFirewallRules(ctx, resourceGroupName, location, firewallName, *publicIP.ID, *fwSubnet.ID, ipConfigName)
	fwPoller, err := config.Azure.AzureFirewall.BeginCreateOrUpdate(ctx, resourceGroupName, firewallName, *firewall, nil)
	if err != nil {
		return vnet, aksSubnet, publicIP, nil, fmt.Errorf("failed to start Firewall creation: %w", err)
	}
	fwResp, err := fwPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return vnet, aksSubnet, publicIP, nil, fmt.Errorf("failed to create Firewall: %w", err)
	}
	firewallOut := &fwResp.AzureFirewall

	// TODO: Associate route table with AKS subnet to force egress through firewall
	// (create a route table and set next hop to firewall private IP)

	return vnet, aksSubnet, publicIP, firewallOut, nil
}

func addAirgapNetworkSettings(ctx context.Context, clusterModel *armcontainerservice.ManagedCluster, privateACRName, location string) error {
	logf(ctx, "Adding network settings for airgap cluster %s in rg %s", *clusterModel.Name, *clusterModel.Properties.NodeResourceGroup)

	vnet, err := getClusterVNet(ctx, *clusterModel.Properties.NodeResourceGroup)
	if err != nil {
		return err
	}
	subnetId := vnet.subnetId

	nsgParams, err := airGapSecurityGroup(location, *clusterModel.Properties.Fqdn)
	if err != nil {
		return err
	}

	nsg, err := createAirgapSecurityGroup(ctx, clusterModel, nsgParams, nil)
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

	err = addPrivateEndpointForACR(ctx, *clusterModel.Properties.NodeResourceGroup, privateACRName, vnet, location)
	if err != nil {
		return err
	}

	logf(ctx, "updated cluster %s subnet with airgap settings", *clusterModel.Name)
	return nil
}

func airGapSecurityGroup(location, clusterFQDN string) (armnetwork.SecurityGroup, error) {
	requiredRules, err := getRequiredSecurityRules(clusterFQDN)
	if err != nil {
		return armnetwork.SecurityGroup{}, fmt.Errorf("failed to get required security rules for airgap resource group: %w", err)
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
		Name:       &config.Config.AirgapNSGName,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{SecurityRules: rules},
	}, nil
}

func addPrivateEndpointForACR(ctx context.Context, nodeResourceGroup, privateACRName string, vnet VNet, location string) error {
	logf(ctx, "Checking if private endpoint for private container registry is in rg %s", nodeResourceGroup)

	var err error
	var exists bool
	privateEndpointName := "PE-for-ABE2ETests"
	if exists, err = privateEndpointExists(ctx, nodeResourceGroup, privateEndpointName); err != nil {
		return err
	}
	if exists {
		logf(ctx, "Private Endpoint already exists, skipping creation")
		return nil
	}

	var peResp armnetwork.PrivateEndpointsClientCreateOrUpdateResponse
	if peResp, err = createPrivateEndpoint(ctx, nodeResourceGroup, privateEndpointName, privateACRName, vnet, location); err != nil {
		return err
	}

	privateZoneName := "privatelink.azurecr.io"
	var pzResp armprivatedns.PrivateZonesClientCreateOrUpdateResponse
	if pzResp, err = createPrivateZone(ctx, nodeResourceGroup, privateZoneName); err != nil {
		return err
	}

	if err = createPrivateDNSLink(ctx, vnet, nodeResourceGroup, privateZoneName); err != nil {
		return err
	}

	if err = addRecordSetToPrivateDNSZone(ctx, peResp, nodeResourceGroup, privateZoneName); err != nil {
		return err
	}

	if err = addDNSZoneGroup(ctx, pzResp, nodeResourceGroup, privateZoneName, *peResp.Name); err != nil {
		return err
	}
	return nil
}

func privateEndpointExists(ctx context.Context, nodeResourceGroup, privateEndpointName string) (bool, error) {
	existingPE, err := config.Azure.PrivateEndpointClient.Get(ctx, nodeResourceGroup, privateEndpointName, nil)
	if err == nil && existingPE.ID != nil {
		logf(ctx, "Private Endpoint already exists with ID: %s", *existingPE.ID)
		return true, nil
	}
	if err != nil && !strings.Contains(err.Error(), "ResourceNotFound") {
		return false, fmt.Errorf("failed to get private endpoint: %w", err)
	}
	return false, nil
}

func createPrivateAzureContainerRegistryPullSecret(ctx context.Context, cluster *armcontainerservice.ManagedCluster, kubeconfig *Kubeclient, resourceGroup string, isNonAnonymousPull bool) error {
	privateACRName := config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location)
	if isNonAnonymousPull {
		logf(ctx, "Creating the secret for non-anonymous pull ACR for the e2e debug pods")
		kubeconfigPath := os.Getenv("HOME") + "/.kube/config"
		if err := fetchAndSaveKubeconfig(ctx, resourceGroup, *cluster.Name, kubeconfigPath); err != nil {
			logf(ctx, "failed to fetch kubeconfig: %v", err)
			return err
		}
		username, password, err := getAzureContainerRegistryCredentials(ctx, resourceGroup, privateACRName)
		if err != nil {
			logf(ctx, "failed to get private ACR credentials: %v", err)
			return err
		}
		if err := kubeconfig.createKubernetesSecret(ctx, "default", config.Config.ACRSecretName, privateACRName, username, password); err != nil {
			logf(ctx, "failed to create Kubernetes secret: %v", err)
			return err
		}
	}
	return nil
}

func createPrivateAzureContainerRegistry(ctx context.Context, cluster *armcontainerservice.ManagedCluster, resourceGroup string, isNonAnonymousPull bool) error {
	privateACRName := config.GetPrivateACRName(isNonAnonymousPull, *cluster.Location)
	logf(ctx, "Creating private Azure Container Registry %s in rg %s", privateACRName, resourceGroup)

	acr, err := config.Azure.RegistriesClient.Get(ctx, resourceGroup, privateACRName, nil)
	if err == nil {
		err, recreateACR := shouldRecreateACR(ctx, resourceGroup, privateACRName)
		if err != nil {
			return fmt.Errorf("failed to check cache rules: %w", err)
		}
		if !recreateACR {
			logf(ctx, "Private ACR already exists at id %s, skipping creation", *acr.ID)
			return nil
		}
		logf(ctx, "Private ACR exists with the wrong cache deleting...")
		if err := deletePrivateAzureContainerRegistry(ctx, resourceGroup, privateACRName); err != nil {
			return fmt.Errorf("failed to delete private acr: %w", err)
		}
		// if ACR gets recreated so should the cluster
		logf(ctx, "Private ACR deleted, deleting cluster %s", *cluster.Name)
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

	logf(ctx, "ACR does not exist, creating...")
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

	logf(ctx, "Private Azure Container Registry created")

	if err := addCacheRulesToPrivateAzureContainerRegistry(ctx, config.ResourceGroupName(*cluster.Location), privateACRName); err != nil {
		return fmt.Errorf("failed to add cache rules to private acr: %w", err)
	}

	return nil
}

func getAzureContainerRegistryCredentials(ctx context.Context, resourceGroup, privateACRName string) (string, string, error) {
	logf(ctx, "Getting credentials for private Azure Container Registry in rg %s", resourceGroup)
	acrCreds, err := config.Azure.RegistriesClient.ListCredentials(ctx, resourceGroup, privateACRName, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to get private ACR credentials: %w", err)
	}
	username := *acrCreds.Username
	password := *acrCreds.Passwords[0].Value
	logf(ctx, "Private Azure Container Registry credentials retrieved")
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
	logf(ctx, "Kubeconfig successfully saved to %s", kubeconfigPath)
	return nil
}

func deletePrivateAzureContainerRegistry(ctx context.Context, resourceGroup, privateACRName string) error {
	logf(ctx, "Deleting private Azure Container Registry in rg %s", resourceGroup)

	pollerResp, err := config.Azure.RegistriesClient.BeginDelete(ctx, resourceGroup, privateACRName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete private ACR: %w", err)
	}
	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete private ACR during polling: %w", err)
	}
	logf(ctx, "Private Azure Container Registry deleted")
	return nil
}

// if the ACR needs to be recreated so does the airgap k8s cluster
func shouldRecreateACR(ctx context.Context, resourceGroup, privateACRName string) (error, bool) {
	logf(ctx, "Checking if private Azure Container Registry cache rules are correct in rg %s", resourceGroup)

	cacheRules, err := config.Azure.CacheRulesClient.Get(ctx, resourceGroup, privateACRName, "aks-managed-rule", nil)
	if err != nil {
		var azErr *azcore.ResponseError
		if errors.As(err, &azErr) && azErr.StatusCode == 404 {
			logf(ctx, "Private ACR cache not found, need to recreate")
			return nil, true
		}
		return fmt.Errorf("failed to get cache rules: %w", err), false
	}
	if cacheRules.Properties != nil && cacheRules.Properties.TargetRepository != nil && *cacheRules.Properties.TargetRepository != config.Config.AzureContainerRegistrytargetRepository {
		logf(ctx, "Private ACR cache is not correct: %s", *cacheRules.Properties.TargetRepository)
		return nil, true
	}
	logf(ctx, "Private ACR cache is correct")
	return nil, false
}

func addCacheRulesToPrivateAzureContainerRegistry(ctx context.Context, resourceGroup, privateACRName string) error {
	logf(ctx, "Adding cache rules to private Azure Container Registry in rg %s", resourceGroup)

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

	logf(ctx, "Cache rule created")
	return nil
}

func createPrivateEndpoint(ctx context.Context, nodeResourceGroup, privateEndpointName, privateACRName string, vnet VNet, location string) (armnetwork.PrivateEndpointsClientCreateOrUpdateResponse, error) {
	logf(ctx, "Creating Private Endpoint in rg %s", nodeResourceGroup)
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
		return armnetwork.PrivateEndpointsClientCreateOrUpdateResponse{}, fmt.Errorf("failed to create private endpoint in BeginCreateOrUpdate: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return armnetwork.PrivateEndpointsClientCreateOrUpdateResponse{}, fmt.Errorf("failed to create private endpoint in polling: %w", err)
	}

	logf(ctx, "Private Endpoint created or updated with ID: %s", *resp.ID)
	return resp, nil
}

func createPrivateZone(ctx context.Context, nodeResourceGroup, privateZoneName string) (armprivatedns.PrivateZonesClientCreateOrUpdateResponse, error) {
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
		return armprivatedns.PrivateZonesClientCreateOrUpdateResponse{}, fmt.Errorf("failed to create private dns zone in BeginCreateOrUpdate: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return armprivatedns.PrivateZonesClientCreateOrUpdateResponse{}, fmt.Errorf("failed to create private dns zone in polling: %w", err)
	}

	logf(ctx, "Private DNS Zone created or updated with ID: %s", *resp.ID)
	return resp, nil
}

func createPrivateDNSLink(ctx context.Context, vnet VNet, nodeResourceGroup, privateZoneName string) error {
	vnetForId, err := config.Azure.VNet.Get(ctx, nodeResourceGroup, vnet.name, nil)
	if err != nil {
		return fmt.Errorf("failed to get vnet: %w", err)
	}
	networkLinkName := "link-ABE2ETests"
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

	logf(ctx, "Virtual Network Link created or updated with ID: %s", *resp.ID)
	return nil
}

func addRecordSetToPrivateDNSZone(ctx context.Context, peResp armnetwork.PrivateEndpointsClientCreateOrUpdateResponse, nodeResourceGroup, privateZoneName string) error {
	for i, dnsConfigPtr := range peResp.Properties.CustomDNSConfigs {
		var ipAddresses []string
		if dnsConfigPtr == nil {
			return fmt.Errorf("CustomDNSConfigs[%d] is nil", i)
		}

		// get the ip addresses
		dnsConfig := *dnsConfigPtr
		if dnsConfig.IPAddresses == nil || len(dnsConfig.IPAddresses) == 0 {
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

	logf(ctx, "Record Set created or updated")
	return nil
}

func addDNSZoneGroup(ctx context.Context, pzResp armprivatedns.PrivateZonesClientCreateOrUpdateResponse, nodeResourceGroup, privateZoneName, endpointName string) error {
	groupName := strings.Replace(privateZoneName, ".", "-", -1) // replace . with -
	dnsZonegroup := armnetwork.PrivateDNSZoneGroup{
		Name: to.Ptr(fmt.Sprintf("%s/default", privateZoneName)),
		Properties: &armnetwork.PrivateDNSZoneGroupPropertiesFormat{
			PrivateDNSZoneConfigs: []*armnetwork.PrivateDNSZoneConfig{{
				Name: to.Ptr(groupName),
				Properties: &armnetwork.PrivateDNSZonePropertiesFormat{
					PrivateDNSZoneID: pzResp.ID,
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

	logf(ctx, "Private DNS Zone Group created or updated with ID")
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

func createAirgapSecurityGroup(ctx context.Context, cluster *armcontainerservice.ManagedCluster, nsgParams armnetwork.SecurityGroup, options *armnetwork.SecurityGroupsClientBeginCreateOrUpdateOptions) (*armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error) {
	poller, err := config.Azure.SecurityGroup.BeginCreateOrUpdate(ctx, *cluster.Properties.NodeResourceGroup, config.Config.AirgapNSGName, nsgParams, options)
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
