package e2e

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

var (
	nsgName           string = "abe2e-airgap-securityGroup"
	defaultSubnetName string = "aks-subnet"
)

func airGapSecurityGroup(location, kubernetesEndpont string) armnetwork.SecurityGroup {
	allowVnet := armnetwork.SecurityRule{
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

	blockOutbound := armnetwork.SecurityRule{
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

	securityRules := []*armnetwork.SecurityRule{
		getSecurityRule("allow-mcr-microsoft-com", "204.79.197.219", 100),
		getSecurityRule("allow-acs-mirror.azureedge.net", "72.21.81.200", 101),
		getSecurityRule("allow-management.azure.com", "4.150.240.10", 102),
		getSecurityRule("allow-kubernetes-endpoint", kubernetesEndpont, 103),
		&allowVnet,
		&blockOutbound,
	}

	return armnetwork.SecurityGroup{
		Location:   &location,
		Name:       &nsgName,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{SecurityRules: securityRules},
	}
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

func addAirgapNetworkSettings(ctx context.Context, clusterConfig clusterConfig) error {
	log.Printf("Adding network settings for airgap cluster %s in rg %s\n", *clusterConfig.cluster.Name, *clusterConfig.cluster.Properties.NodeResourceGroup)

	vnet, err := getClusterVNet(ctx, *clusterConfig.cluster.Properties.NodeResourceGroup)
	if err != nil {
		return err
	}
	clusterConfig.subnetId = vnet.subnetId

	ipAddresses, err := net.LookupIP(*clusterConfig.cluster.Properties.Fqdn)
	if err != nil {
		return err
	}
	nsgParams := airGapSecurityGroup(config.Location, ipAddresses[0].String())

	nsg, err := config.Azure.CreateOrUpdateSecurityGroup(ctx, *clusterConfig.cluster.Properties.NodeResourceGroup, nsgName, nsgParams)
	if err != nil {
		return err
	}

	subnetParameters := armnetwork.Subnet{
		ID: to.Ptr(clusterConfig.subnetId),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.224.0.0/16"),
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: nsg.ID,
			},
		},
	}
	_, err = config.Azure.CreateOrUpdateSubnet(ctx, *clusterConfig.cluster.Properties.NodeResourceGroup, vnet.name, defaultSubnetName, subnetParameters)
	if err != nil {
		return err
	}

	fmt.Printf("Updated the subnet to airgap %s\n", *clusterConfig.cluster.Name)
	return nil
}

func isNetworkSecurityGroupAirgap(resourceGroupName string) (bool, error) {
	_, err := config.Azure.SecurityGroup.Get(context.Background(), resourceGroupName, nsgName, nil)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get network security group: %w", err)
	}
	return true, nil
}
