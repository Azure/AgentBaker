package e2e_test

import (
	"context"
	"fmt"
	"net"

	"github.com/Azure/agentbakere2e/suite"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

var (
	nsgName           string = "abe2e-airgap-securityGroup"
	defaultSubnetName string = "aks-subnet"
)

func cloudGapSecurityGroup(location, kubernetesEndpont string) armnetwork.SecurityGroup {
	securityRules := []*armnetwork.SecurityRule{}

	securityRules = append(securityRules, getSecurityRule("allow-mcr-microsoft-com", "204.79.197.219", 100))
	securityRules = append(securityRules, getSecurityRule("allow-acs-mirror.azureedge.net", "72.21.81.200", 101))
	securityRules = append(securityRules, getSecurityRule("allow-management.azure.com", "4.150.240.10", 102))
	securityRules = append(securityRules, getSecurityRule("allow-kubernetes-endpoint", kubernetesEndpont, 102))

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
			Priority:                 to.Ptr[int32](2001), // lower priroity than allowing mcr
		},
	}

	securityRules = append(securityRules, &allowVnet)
	securityRules = append(securityRules, &blockOutbound)

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

func addAirgapNetworkSettings(ctx context.Context, cloud *azureClient, suiteConfig *suite.Config, clusterConfig clusterConfig) error {
	fmt.Printf("Adding network settings for airgap cluster %s in rg %s\n", *clusterConfig.cluster.Name, *clusterConfig.cluster.Properties.NodeResourceGroup)

	if clusterConfig.subnetId == "" {
		subnetId, err := getClusterSubnetID(ctx, cloud, suiteConfig.Location, *clusterConfig.cluster.Properties.NodeResourceGroup, *clusterConfig.cluster.Name)
		if err != nil {
			return err
		}
		clusterConfig.subnetId = subnetId
	}

	ipAddresses, err := net.LookupIP(*clusterConfig.cluster.Properties.Fqdn)
	if err != nil {
		return err
	}
	nsgParams := cloudGapSecurityGroup(suiteConfig.Location, ipAddresses[0].String())

	nsg, err := createAirgapSecurityGroup(ctx, cloud, suiteConfig, clusterConfig, nsgParams, nil)
	if err != nil {
		return err
	}

	vnetName, err := getClusterVnetName(ctx, cloud, *clusterConfig.cluster.Properties.NodeResourceGroup)
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
	err = updateSubnet(ctx, cloud, clusterConfig, subnetParameters, vnetName)
	if err != nil {
		return err
	}

	fmt.Printf("Updated the subnet to airgap %s\n", *clusterConfig.cluster.Name)
	return nil
}

func createAirgapSecurityGroup(ctx context.Context, cloud *azureClient, suiteConfig *suite.Config, clusterConfig clusterConfig, nsgParams armnetwork.SecurityGroup, options *armnetwork.SecurityGroupsClientBeginCreateOrUpdateOptions) (*armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error) {
	poller, err := cloud.securityGroupClient.BeginCreateOrUpdate(ctx, *clusterConfig.cluster.Properties.NodeResourceGroup, nsgName, nsgParams, nil)
	if err != nil {
		return nil, err
	}
	nsg, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &nsg, nil
}

func updateSubnet(ctx context.Context, cloud *azureClient, clusterConfig clusterConfig, subnetParameters armnetwork.Subnet, vnetName string) error {
	poller, err := cloud.subnetClient.BeginCreateOrUpdate(ctx, *clusterConfig.cluster.Properties.NodeResourceGroup, vnetName, defaultSubnetName, subnetParameters, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func isNetworkSecurityGroupAirgap(cloud *azureClient, resourceGroupName string) (bool, error) {
	_, err := cloud.securityGroupClient.Get(context.Background(), resourceGroupName, nsgName, nil)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get network security group: %w", err)
	}
	return true, nil
}
