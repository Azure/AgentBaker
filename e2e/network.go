package e2e_test

import (
	"context"
	"fmt"

	"github.com/Azure/agentbakere2e/suite"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

var nsgName = "abe2e-airgap-securityGroup2"

func cloudGapSecurityGroup(location string) armnetwork.SecurityGroup {
	return armnetwork.SecurityGroup{
		Location: &location,
		Name:     &nsgName,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
				{
					Name: to.Ptr("allow-outbound-to-mcr-microsoft-com"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
						SourceAddressPrefix:      to.Ptr("*"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("204.79.197.219"),
						DestinationPortRange:     to.Ptr("*"),
					},
				},
			},
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
	nsgParams := cloudGapSecurityGroup(suiteConfig.Location)

	poller, err := cloud.securityGroupClient.BeginCreateOrUpdate(ctx, suiteConfig.ResourceGroupName, nsgName, nsgParams, nil)
	if err != nil {
		fmt.Printf("failed in cloud.securityGroupClient.BeginCreateOrUpdate\n")
		return err
	}
	nsg, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	fmt.Printf("created nsg %s, nsgID %s\n", nsgName, *nsg.ID)

	vnetName, err := getClusterVnetName(ctx, cloud, *clusterConfig.cluster.Properties.NodeResourceGroup)
	if err != nil {
		return err
	}
	fmt.Printf("got vnet name %s\n", vnetName)
	fmt.Printf("subnetID %s, nsg.ID %s\n", clusterConfig.subnetId, *nsg.ID)
	subnetParameters := armnetwork.Subnet{
		ID:   to.Ptr(clusterConfig.subnetId),
		Name: to.Ptr("aks-subnet-airgap"),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.224.0.0/16"),
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: nsg.ID,
			},
		},
	}
	poller2, err := cloud.subnetClient.BeginCreateOrUpdate(ctx, *clusterConfig.cluster.Properties.NodeResourceGroup, vnetName, "aks-subnet", subnetParameters, nil)
	if err != nil {
		fmt.Printf("failed in subnetsClient.CreateOrUpdate\n")
		return err
	}
	_, err = poller2.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	fmt.Printf("Updated the subnet %s\n", *clusterConfig.cluster.Name)
	return nil
}
