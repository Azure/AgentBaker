package e2e

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerregistry/armcontainerregistry"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
)

// global var used here and in scenario_test.go, kube.go, template.go
var PrivateACRName string = "privateacre2e"

func getKubenetClusterModel(name string) *armcontainerservice.ManagedCluster {
	model := getBaseClusterModel(name)
	model.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginKubenet)
	return model
}

func getAzureNetworkClusterModel(name string) *armcontainerservice.ManagedCluster {
	cluster := getBaseClusterModel(name)
	cluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginAzure)
	if cluster.Properties.AgentPoolProfiles != nil {
		for _, app := range cluster.Properties.AgentPoolProfiles {
			app.MaxPods = to.Ptr[int32](30)
		}
	}
	return cluster
}

func getBaseClusterModel(clusterName string) *armcontainerservice.ManagedCluster {
	return &armcontainerservice.ManagedCluster{
		Name:     to.Ptr(clusterName),
		Location: to.Ptr(config.Config.Location),
		Properties: &armcontainerservice.ManagedClusterProperties{
			DNSPrefix: to.Ptr(clusterName),
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Name:         to.Ptr("nodepool1"),
					Count:        to.Ptr[int32](1),
					VMSize:       to.Ptr("standard_d2ds_v5"),
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
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: to.Ptr(armcontainerservice.ResourceIdentityTypeSystemAssigned),
		},
	}
}

func addAirgapNetworkSettings(ctx context.Context, t *testing.T, cluster *Cluster) error {
	t.Logf("Adding network settings for airgap cluster %s in rg %s\n", *cluster.Model.Name, *cluster.Model.Properties.NodeResourceGroup)

	vnet, err := getClusterVNet(ctx, *cluster.Model.Properties.NodeResourceGroup)
	if err != nil {
		return err
	}
	subnetId := vnet.subnetId

	nsgParams, err := airGapSecurityGroup(config.Config.Location, *cluster.Model.Properties.Fqdn)
	if err != nil {
		return err
	}

	nsg, err := createAirgapSecurityGroup(ctx, cluster.Model, nsgParams, nil)
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
	if err = updateSubnet(ctx, cluster.Model, subnetParameters, vnet.name); err != nil {
		return err
	}

	err = addPrivateEndpointForACR(ctx, t, *cluster.Model.Properties.NodeResourceGroup, vnet)
	if err != nil {
		return err
	}

	t.Logf("updated cluster %s subnet with airgap settings", *cluster.Model.Name)
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

func addPrivateEndpointForACR(ctx context.Context, t *testing.T, nodeResourceGroup string, vnet VNet) error {
	t.Logf("Checking if private endpoint for private container registry is in rg %s\n", nodeResourceGroup)

	var err error
	var exists bool
	privateEndpointName := "PE-for-ABE2ETests"
	if exists, err = privateEndpointExists(ctx, t, nodeResourceGroup, privateEndpointName); err != nil {
		return err
	}
	if exists {
		t.Logf("Private Endpoint already exists, skipping creation")
		return nil
	}

	if err := createPrivateAzureContainerRegistry(ctx, t, nodeResourceGroup, PrivateACRName); err != nil {
		return err
	}

	if err := addCacheRuelsToPrivateAzureContainerRegistry(ctx, t, nodeResourceGroup, PrivateACRName); err != nil {
		return err
	}

	var peResp armnetwork.PrivateEndpointsClientCreateOrUpdateResponse
	if peResp, err = createPrivateEndpoint(ctx, t, nodeResourceGroup, privateEndpointName, PrivateACRName, vnet); err != nil {
		return err
	}

	privateZoneName := "privatelink.azurecr.io"
	var pzResp armprivatedns.PrivateZonesClientCreateOrUpdateResponse
	if pzResp, err = createPrivateZone(ctx, t, nodeResourceGroup, privateZoneName); err != nil {
		return err
	}

	if err = createPrivateDNSLink(ctx, t, vnet, nodeResourceGroup, privateZoneName); err != nil {
		return err
	}

	if err = addRecordSetToPrivateDNSZone(ctx, t, peResp, nodeResourceGroup, privateZoneName); err != nil {
		return err
	}

	if err = addDNSZoneGroup(ctx, t, pzResp, nodeResourceGroup, privateZoneName, *peResp.Name); err != nil {
		return err
	}
	return nil
}

func privateEndpointExists(ctx context.Context, t *testing.T, nodeResourceGroup, privateEndpointName string) (bool, error) {
	existingPE, err := config.Azure.PrivateEndpointClient.Get(ctx, nodeResourceGroup, privateEndpointName, nil)
	if err == nil && existingPE.ID != nil {
		t.Logf("Private Endpoint already exists with ID: %s\n", *existingPE.ID)
		return true, nil
	}
	if err != nil && !strings.Contains(err.Error(), "ResourceNotFound") {
		return false, fmt.Errorf("failed to get private endpoint: %w", err)
	}
	return false, nil
}

func createPrivateAzureContainerRegistry(ctx context.Context, t *testing.T, nodeResourceGroup, privateACRName string) error {
	t.Logf("Creating private Azure Container Registry in rg %s\n", nodeResourceGroup)

	createParams := armcontainerregistry.Registry{
		Location: to.Ptr(config.Config.Location), // Replace with your desired location
		SKU: &armcontainerregistry.SKU{
			Name: to.Ptr(armcontainerregistry.SKUNamePremium),
		},
		Properties: &armcontainerregistry.RegistryProperties{
			AdminUserEnabled:     to.Ptr(false),
			AnonymousPullEnabled: to.Ptr(true), // required to pull images from the private ACR without authentication
		},
	}

	_, err := config.Azure.RegistriesClient.Get(ctx, nodeResourceGroup, privateACRName, nil)
	if err != nil && strings.Contains(err.Error(), "ResourceNotFound") {
		// Create the private ACR if it doesn't exist
		pollerResp, err := config.Azure.RegistriesClient.BeginCreate(
			ctx,
			nodeResourceGroup,
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

		t.Logf("Private Azure Container Registry created\n")
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get private ACR: %w", err)
	}

	updateParams := &armcontainerregistry.RegistryUpdateParameters{
		Properties: &armcontainerregistry.RegistryPropertiesUpdateParameters{
			AdminUserEnabled:     to.Ptr(false),
			AnonymousPullEnabled: to.Ptr(true), // required to pull images from the private ACR without authentication
		},
	}
	pollerResp, err := config.Azure.RegistriesClient.BeginUpdate(
		ctx,
		nodeResourceGroup,
		privateACRName,
		*updateParams,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create private ACR in BeginUpdate: %w", err)
	}

	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create private ACR in polling: %w", err)
	}

	t.Logf("Private Azure Container Registry updated\n")
	return nil
}

func addCacheRuelsToPrivateAzureContainerRegistry(ctx context.Context, t *testing.T, nodeResourceGroup, privateACRName string) error {
	cacheParams := armcontainerregistry.CacheRule{
		Properties: &armcontainerregistry.CacheRuleProperties{
			SourceRepository: to.Ptr("mcr.microsoft.com/*"),
			TargetRepository: to.Ptr("aks/*"),
		},
	}
	cacheCreateResp, err := config.Azure.CacheRulesClient.BeginCreate(
		ctx,
		nodeResourceGroup,
		privateACRName,
		"aks-managed-rul",
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

	t.Logf("Cache rule created\n")
	return nil
}

func createPrivateEndpoint(ctx context.Context, t *testing.T, nodeResourceGroup, privateEndpointName, acrName string, vnet VNet) (armnetwork.PrivateEndpointsClientCreateOrUpdateResponse, error) {
	t.Logf("Creating private Azure Container Registry in rg %s\n", nodeResourceGroup)
	acrID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerRegistry/registries/%s", config.Config.SubscriptionID, nodeResourceGroup, acrName)

	peParams := armnetwork.PrivateEndpoint{
		Location: to.Ptr(config.Config.Location),
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

	t.Logf("Private Endpoint created or updated with ID: %s\n", *resp.ID)
	return resp, nil
}

func createPrivateZone(ctx context.Context, t *testing.T, nodeResourceGroup, privateZoneName string) (armprivatedns.PrivateZonesClientCreateOrUpdateResponse, error) {
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

	t.Logf("Private DNS Zone created or updated with ID: %s\n", *resp.ID)
	return resp, nil
}

func createPrivateDNSLink(ctx context.Context, t *testing.T, vnet VNet, nodeResourceGroup, privateZoneName string) error {
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

	t.Logf("Virtual Network Link created or updated with ID: %s\n", *resp.ID)
	return nil
}

func addRecordSetToPrivateDNSZone(ctx context.Context, t *testing.T, peResp armnetwork.PrivateEndpointsClientCreateOrUpdateResponse, nodeResourceGroup, privateZoneName string) error {
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

	t.Logf("Record Set created or updated")
	return nil
}

func addDNSZoneGroup(ctx context.Context, t *testing.T, pzResp armprivatedns.PrivateZonesClientCreateOrUpdateResponse, nodeResourceGroup, privateZoneName, endpointName string) error {
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

	t.Logf("Private DNS Zone Group created or updated with ID")
	return nil
}

func getRequiredSecurityRules(clusterFQDN string) ([]*armnetwork.SecurityRule, error) {
	// https://learn.microsoft.com/en-us/azure/aks/outbound-rules-control-egress#azure-global-required-fqdn--application-rules
	// note that we explicitly exclude packages.microsoft.com
	requiredDNSNames := []string{
		"management.azure.com",
		clusterFQDN,
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
			Priority:                 to.Ptr[int32](priority),
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
