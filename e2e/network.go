package e2e_test

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

func cloudGapSecurityGroup(location string, subnetID string) armnetwork.SecurityGroup {
	subnet := armnetwork.Subnet{
		ID: &subnetID,
	}
	name := "abe2e-airgap-securityGroup"
	return armnetwork.SecurityGroup{
		Location: &location,
		Name:     &name,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
				{
					Name: to.Ptr("allow-mcr-microsoft-com"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
						SourceAddressPrefix:      to.Ptr("204.79.197.219"), // Replace with your desired source IP range mcr.microsoft.com
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("*"),
						DestinationPortRange:     to.Ptr("*"),
						Priority:                 to.Ptr[int32](100),
					},
				},
			},
			Subnets: []*armnetwork.Subnet{&subnet},
		},
	}
}
