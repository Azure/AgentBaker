package e2e

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

var (
	airgapNSGName     string = "abe2e-airgap-securityGroup"
	defaultSubnetName string = "aks-subnet"
)

func getRequiredSecurityRules(clusterFQDN string) ([]*armnetwork.SecurityRule, error) {
	// https://learn.microsoft.com/en-us/azure/aks/outbound-rules-control-egress#azure-global-required-fqdn--application-rules
	// note that we explicitly exclude packages.microsoft.com
	requiredDNSNames := []string{
		"mcr.microsoft.com",
		"management.azure.com",
		"acs-mirror.azureedge.net",
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
		Name:       &airgapNSGName,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{SecurityRules: rules},
	}, nil
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

func addAirgapNetworkSettings(ctx context.Context, cloud *azureClient, clusterConfig clusterConfig) error {
	log.Printf("Adding network settings for airgap cluster %s in rg %s\n", *clusterConfig.cluster.Name, *clusterConfig.cluster.Properties.NodeResourceGroup)

	vnet, err := getClusterVNet(ctx, cloud, *clusterConfig.cluster.Properties.NodeResourceGroup)
	if err != nil {
		return err
	}
	clusterConfig.subnetId = vnet.subnetId

	nsgParams, err := airGapSecurityGroup(config.Location, *clusterConfig.cluster.Properties.Fqdn)
	if err != nil {
		return err
	}

	nsg, err := createAirgapSecurityGroup(ctx, cloud, clusterConfig, nsgParams, nil)
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
	if err = updateSubnet(ctx, cloud, clusterConfig, subnetParameters, vnet.name); err != nil {
		return err
	}

	log.Printf("updated cluster %s subnet with airggap settings", *clusterConfig.cluster.Name)
	return nil
}

func createAirgapSecurityGroup(ctx context.Context, cloud *azureClient, clusterConfig clusterConfig, nsgParams armnetwork.SecurityGroup, options *armnetwork.SecurityGroupsClientBeginCreateOrUpdateOptions) (*armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error) {
	poller, err := cloud.securityGroupClient.BeginCreateOrUpdate(ctx, *clusterConfig.cluster.Properties.NodeResourceGroup, airgapNSGName, nsgParams, options)
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
	_, err := cloud.securityGroupClient.Get(context.Background(), resourceGroupName, airgapNSGName, nil)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get network security group: %w", err)
	}
	return true, nil
}
