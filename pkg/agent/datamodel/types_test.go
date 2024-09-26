// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
)

const (
	// scaleSetPriorityRegular is the default ScaleSet Priority.
	ScaleSetPriorityRegular = "Regular"
	// ScaleSetPriorityLow means the ScaleSet will use Low-priority VMs.
	ScaleSetPriorityLow = "Low"
	// StorageAccount means that the nodes use raw storage accounts for their os and attached volumes.
	StorageAccount = "StorageAccount"
	// Ephemeral means that the node's os disk is ephemeral. This is not compatible with attached volumes.
	Ephemeral = "Ephemeral"
)

func TestHasAadProfile(t *testing.T) {
	p := Properties{}

	if p.HasAadProfile() {
		t.Fatalf("Expected HasAadProfile() to return false")
	}

	p.AADProfile = &AADProfile{
		ClientAppID: "test",
		ServerAppID: "test",
	}

	if !p.HasAadProfile() {
		t.Fatalf("Expected HasAadProfile() to return true")
	}
}

func TestGetCustomEnvironmentJSON(t *testing.T) {
	properities := getMockProperitesWithCustomClouEnv()
	expectedRet := `{"name":"AzureStackCloud","Name":"AzureStackCloud","mcrURL":"mcr.microsoft.fakecustomcloud","repoDepotEndpoint":"https://repodepot.azure.microsoft.fakecustomcloud/ubuntu","managementPortalURL":"https://portal.azure.microsoft.fakecustomcloud/","serviceManagementEndpoint":"https://management.core.microsoft.fakecustomcloud/","resourceManagerEndpoint":"https://management.azure.microsoft.fakecustomcloud/","activeDirectoryEndpoint":"https://login.microsoftonline.microsoft.fakecustomcloud/","keyVaultEndpoint":"https://vault.cloudapi.microsoft.fakecustomcloud/","graphEndpoint":"https://graph.cloudapi.microsoft.fakecustomcloud/","storageEndpointSuffix":"core.microsoft.fakecustomcloud","sqlDatabaseDNSSuffix":"database.cloudapi.microsoft.fakecustomcloud","keyVaultDNSSuffix":"vault.cloudapi.microsoft.fakecustomcloud","resourceManagerVMDNSSuffix":"cloudapp.azure.microsoft.fakecustomcloud/","containerRegistryDNSSuffix":".azurecr.microsoft.fakecustomcloud","cosmosDBDNSSuffix":"documents.core.microsoft.fakecustomcloud/","tokenAudience":"https://management.core.microsoft.fakecustomcloud/","resourceIdentifiers":{}}` //nolint: lll
	actual, err := properities.GetCustomEnvironmentJSON(false)
	if err != nil {
		t.Error(err)
	}
	if expectedRet != actual {
		t.Errorf("Expected GetCustomEnvironmentJSON() to return %s, but got %s . ", expectedRet, actual)
	}
}

func TestPropertiesIsIPMasqAgentDisabled(t *testing.T) {
	cases := []struct {
		name             string
		p                *Properties
		expectedDisabled bool
	}{
		{
			name:             "default",
			p:                &Properties{},
			expectedDisabled: false,
		},
		{
			name: "hostedMasterProfile disabled",
			p: &Properties{
				HostedMasterProfile: &HostedMasterProfile{
					IPMasqAgent: false,
				},
			},
			expectedDisabled: true,
		},
		{
			name: "hostedMasterProfile enabled",
			p: &Properties{
				HostedMasterProfile: &HostedMasterProfile{
					IPMasqAgent: true,
				},
			},
			expectedDisabled: false,
		},
		{
			name: "nil KubernetesConfig",
			p: &Properties{
				OrchestratorProfile: &OrchestratorProfile{},
			},
			expectedDisabled: false,
		},
		{
			name: "default KubernetesConfig",
			p: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					KubernetesConfig: &KubernetesConfig{},
				},
			},
			expectedDisabled: false,
		},
		{
			name: "addons configured but no ip-masq-agent configuration",
			p: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							{
								Name:    "coredns",
								Enabled: to.BoolPtr(true),
							},
						},
					},
				},
			},
			expectedDisabled: false,
		},
		{
			name: "ip-masq-agent explicitly disabled",
			p: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							{
								Name:    IPMASQAgentAddonName,
								Enabled: to.BoolPtr(false),
							},
						},
					},
				},
			},
			expectedDisabled: true,
		},
		{
			name: "ip-masq-agent present but no configuration",
			p: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							{
								Name: IPMASQAgentAddonName,
							},
						},
					},
				},
			},
			expectedDisabled: false,
		},
		{
			name: "ip-masq-agent explicitly enabled",
			p: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							{
								Name:    IPMASQAgentAddonName,
								Enabled: to.BoolPtr(true),
							},
						},
					},
				},
			},
			expectedDisabled: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.p.IsIPMasqAgentDisabled() != c.expectedDisabled {
				t.Fatalf("expected Properties.IsIPMasqAgentDisabled() to return %t but instead returned %t", c.expectedDisabled, c.p.IsIPMasqAgentDisabled())
			}
		})
	}
}

func TestOSType(t *testing.T) {
	p := Properties{
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				OSType: "Linux",
			},
			{
				OSType: "Linux",
				Distro: AKSUbuntu1604,
			},
		},
	}

	if p.HasWindows() {
		t.Fatalf("expected HasWindows() to return false but instead returned true")
	}
	if p.AgentPoolProfiles[0].IsWindows() {
		t.Fatalf("expected IsWindows() to return false but instead returned true")
	}

	p.AgentPoolProfiles[0].OSType = Windows

	if !p.HasWindows() {
		t.Fatalf("expected HasWindows() to return true but instead returned false")
	}

	if !p.AgentPoolProfiles[0].IsWindows() {
		t.Fatalf("expected IsWindows() to return true but instead returned false")
	}
}

func TestIsIPMasqAgentEnabled(t *testing.T) {
	cases := []struct {
		p                                            Properties
		expectedPropertiesIsIPMasqAgentEnabled       bool
		expectedKubernetesConfigIsIPMasqAgentEnabled bool
	}{
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							getMockAddon(IPMASQAgentAddonName),
						},
					},
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       false,
			expectedKubernetesConfigIsIPMasqAgentEnabled: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{},
					},
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       false,
			expectedKubernetesConfigIsIPMasqAgentEnabled: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							{
								Name: IPMASQAgentAddonName,
								Containers: []KubernetesContainerSpec{
									{
										Name: IPMASQAgentAddonName,
									},
								},
							},
						},
					},
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       false,
			expectedKubernetesConfigIsIPMasqAgentEnabled: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							{
								Name:    IPMASQAgentAddonName,
								Enabled: to.BoolPtr(false),
								Containers: []KubernetesContainerSpec{
									{
										Name: IPMASQAgentAddonName,
									},
								},
							},
						},
					},
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       false,
			expectedKubernetesConfigIsIPMasqAgentEnabled: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							{
								Name:    IPMASQAgentAddonName,
								Enabled: to.BoolPtr(false),
								Containers: []KubernetesContainerSpec{
									{
										Name: IPMASQAgentAddonName,
									},
								},
							},
						},
					},
				},
				HostedMasterProfile: &HostedMasterProfile{
					IPMasqAgent: true,
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       true,
			expectedKubernetesConfigIsIPMasqAgentEnabled: false, // unsure of the validity of this case, but because it's possible we unit test it
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							{
								Name:    IPMASQAgentAddonName,
								Enabled: to.BoolPtr(true),
								Containers: []KubernetesContainerSpec{
									{
										Name: IPMASQAgentAddonName,
									},
								},
							},
						},
					},
				},
				HostedMasterProfile: &HostedMasterProfile{
					IPMasqAgent: true,
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       true,
			expectedKubernetesConfigIsIPMasqAgentEnabled: true,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						Addons: []KubernetesAddon{
							{
								Name:    IPMASQAgentAddonName,
								Enabled: to.BoolPtr(true),
								Containers: []KubernetesContainerSpec{
									{
										Name: IPMASQAgentAddonName,
									},
								},
							},
						},
					},
				},
				HostedMasterProfile: &HostedMasterProfile{
					IPMasqAgent: false,
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       false,
			expectedKubernetesConfigIsIPMasqAgentEnabled: true, // unsure of the validity of this case, but because it's possible we unit test it
		},
	}

	for _, c := range cases {
		if c.p.IsIPMasqAgentEnabled() != c.expectedPropertiesIsIPMasqAgentEnabled {
			t.Fatalf("expected Properties.IsIPMasqAgentEnabled() to return %t but instead returned %t",
				c.expectedPropertiesIsIPMasqAgentEnabled, c.p.IsIPMasqAgentEnabled())
		}
		if c.p.OrchestratorProfile.KubernetesConfig.IsIPMasqAgentEnabled() != c.expectedKubernetesConfigIsIPMasqAgentEnabled {
			t.Fatalf("expected KubernetesConfig.IsIPMasqAgentEnabled() to return %t but instead returned %t",
				c.expectedKubernetesConfigIsIPMasqAgentEnabled, c.p.OrchestratorProfile.KubernetesConfig.IsIPMasqAgentEnabled())
		}
	}
}

func TestGenerateClusterID(t *testing.T) {
	tests := []struct {
		name              string
		properties        *Properties
		expectedClusterID string
	}{
		{
			name: "From Hosted Master Profile",
			properties: &Properties{
				HostedMasterProfile: &HostedMasterProfile{
					DNSPrefix: "foo_hosted_master",
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name: "foo_agent1",
					},
				},
			},
			expectedClusterID: "42761241",
		},
		{
			name: "No Master Profile",
			properties: &Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name: "foo_agent2",
					},
				},
			},
			expectedClusterID: "11729301",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual := test.properties.GetClusterID()

			if actual != test.expectedClusterID {
				t.Errorf("expected cluster ID %s, but got %s", test.expectedClusterID, actual)
			}
		})
	}
}
func TestAreAgentProfilesCustomVNET(t *testing.T) {
	p := Properties{}
	p.AgentPoolProfiles = []*AgentPoolProfile{
		{
			VnetSubnetID: "subnetlink1",
		},
		{
			VnetSubnetID: "subnetlink2",
		},
	}

	if !p.AreAgentProfilesCustomVNET() {
		t.Fatalf("Expected isCustomVNET to be true when subnet exists for all agent pool profile")
	}

	p.AgentPoolProfiles = []*AgentPoolProfile{
		{
			VnetSubnetID: "subnetlink1",
		},
		{
			VnetSubnetID: "",
		},
	}

	if p.AreAgentProfilesCustomVNET() {
		t.Fatalf("Expected isCustomVNET to be false when subnet exists for some agent pool profile")
	}

	p.AgentPoolProfiles = nil

	if p.AreAgentProfilesCustomVNET() {
		t.Fatalf("Expected isCustomVNET to be false when agent pool profiles is nil")
	}
}

func TestPropertiesHasDCSeriesSKU(t *testing.T) {
	cases := GetDCSeriesVMCasesForTesting()

	for _, c := range cases {
		p := Properties{
			AgentPoolProfiles: []*AgentPoolProfile{
				{
					Name:   "agentpool",
					VMSize: c.VMSKU,
				},
			},
			OrchestratorProfile: &OrchestratorProfile{
				OrchestratorType:    Kubernetes,
				OrchestratorVersion: "1.16.0",
			},
		}
		ret := p.HasDCSeriesSKU()
		if ret != c.Expected {
			t.Fatalf("expected HasDCSeriesSKU(%s) to return %t, but instead got %t", c.VMSKU, c.Expected, ret)
		}
	}
}

func TestIsVHDDistroForAllNodes(t *testing.T) {
	cases := []struct {
		p        Properties
		expected bool
	}{
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Distro: AKSUbuntu1604,
					},
				},
			},
			expected: true,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						OSType: Windows,
					},
				},
			},
			expected: false,
		},
	}

	for _, c := range cases {
		if c.p.IsVHDDistroForAllNodes() != c.expected {
			t.Fatalf("expected IsVHDDistroForAllNodes() to return %t but instead returned %t", c.expected, c.p.IsVHDDistroForAllNodes())
		}
	}
}

func TestAvailabilityProfile(t *testing.T) {
	cases := []struct {
		p               Properties
		expectedHasVMSS bool
		expectedISVMSS  bool
		expectedIsAS    bool
		expectedLowPri  bool
		expectedSpot    bool
		expectedVMType  string
	}{
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						AvailabilityProfile: VirtualMachineScaleSets,
					},
				},
			},
			expectedHasVMSS: true,
			expectedISVMSS:  true,
			expectedIsAS:    false,
			expectedLowPri:  false,
			expectedSpot:    true,
			expectedVMType:  VMSSVMType,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						AvailabilityProfile: VirtualMachineScaleSets,
					},
				},
			},
			expectedHasVMSS: true,
			expectedISVMSS:  true,
			expectedIsAS:    false,
			expectedLowPri:  true,
			expectedSpot:    false,
			expectedVMType:  VMSSVMType,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						AvailabilityProfile: VirtualMachineScaleSets,
					},
					{
						AvailabilityProfile: AvailabilitySet,
					},
				},
			},
			expectedHasVMSS: true,
			expectedISVMSS:  true,
			expectedIsAS:    false,
			expectedLowPri:  false,
			expectedSpot:    false,
			expectedVMType:  VMSSVMType,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						AvailabilityProfile: AvailabilitySet,
					},
				},
			},
			expectedHasVMSS: false,
			expectedISVMSS:  false,
			expectedIsAS:    true,
			expectedLowPri:  false,
			expectedSpot:    false,
			expectedVMType:  StandardVMType,
		},
	}

	for _, c := range cases {
		if c.p.HasVMSSAgentPool() != c.expectedHasVMSS {
			t.Fatalf("expected HasVMSSAgentPool() to return %t but instead returned %t", c.expectedHasVMSS, c.p.HasVMSSAgentPool())
		}
		if c.p.AgentPoolProfiles[0].IsVirtualMachineScaleSets() != c.expectedISVMSS {
			t.Fatalf("expected IsVirtualMachineScaleSets() to return %t but instead returned %t", c.expectedISVMSS,
				c.p.AgentPoolProfiles[0].IsVirtualMachineScaleSets())
		}
		if c.p.AgentPoolProfiles[0].IsAvailabilitySets() != c.expectedIsAS {
			t.Fatalf("expected IsAvailabilitySets() to return %t but instead returned %t", c.expectedIsAS, c.p.AgentPoolProfiles[0].IsAvailabilitySets())
		}
		if c.p.GetVMType() != c.expectedVMType {
			t.Fatalf("expected GetVMType() to return %s but instead returned %s", c.expectedVMType, c.p.GetVMType())
		}
	}
}

func TestGetSubnetName(t *testing.T) {
	tests := []struct {
		name               string
		properties         *Properties
		expectedSubnetName string
	}{
		{
			name: "Cluster with HostedMasterProfile",
			properties: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
				},
				HostedMasterProfile: &HostedMasterProfile{
					FQDN:      "fqdn",
					DNSPrefix: "foo",
					Subnet:    "mastersubnet",
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:                "agentpool",
						VMSize:              "Standard_D2_v2",
						AvailabilityProfile: VirtualMachineScaleSets,
					},
				},
			},
			expectedSubnetName: "aks-subnet",
		},
		{
			name: "Cluster with HostedMasterProfile and custom VNET",
			properties: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
				},
				HostedMasterProfile: &HostedMasterProfile{
					FQDN:      "fqdn",
					DNSPrefix: "foo",
					Subnet:    "mastersubnet",
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:                "agentpool",
						VMSize:              "Standard_D2_v2",
						AvailabilityProfile: VirtualMachineScaleSets,
						VnetSubnetID:        "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/BazAgentSubnet", //nolint: lll
					},
				},
			},
			expectedSubnetName: "BazAgentSubnet",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual := test.properties.GetSubnetName()

			if actual != test.expectedSubnetName {
				t.Errorf("expected subnet name %s, but got %s", test.expectedSubnetName, actual)
			}
		})
	}
}

func TestGetRouteTableName(t *testing.T) {
	p := &Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType: Kubernetes,
		},
		HostedMasterProfile: &HostedMasterProfile{
			FQDN:      "fqdn",
			DNSPrefix: "foo",
			Subnet:    "mastersubnet",
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				AvailabilityProfile: VirtualMachineScaleSets,
			},
		},
	}

	actualRTName := p.GetRouteTableName()
	expectedRTName := "aks-agentpool-28513887-routetable"

	actualNSGName := p.GetNSGName()
	expectedNSGName := "aks-agentpool-28513887-nsg"

	if actualRTName != expectedRTName {
		t.Errorf("expected route table name %s, but got %s", expectedRTName, actualRTName)
	}

	if actualNSGName != expectedNSGName {
		t.Errorf("expected route table name %s, but got %s", expectedNSGName, actualNSGName)
	}
}

func TestProperties_GetVirtualNetworkName(t *testing.T) {
	tests := []struct {
		name                       string
		properties                 *Properties
		expectedVirtualNetworkName string
	}{
		{
			name: "Cluster with HostedMasterProfile and Custom VNET AgentProfiles",
			properties: &Properties{
				HostedMasterProfile: &HostedMasterProfile{
					FQDN:      "fqdn",
					DNSPrefix: "foo",
					Subnet:    "mastersubnet",
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:                "agentpool",
						VMSize:              "Standard_D2_v2",
						AvailabilityProfile: VirtualMachineScaleSets,
						VnetSubnetID:        "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/BazAgentSubnet", //nolint: lll
					},
				},
			},
			expectedVirtualNetworkName: "ExampleCustomVNET",
		},
		{
			name: "Cluster with HostedMasterProfile and AgentProfiles",
			properties: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
				},
				HostedMasterProfile: &HostedMasterProfile{
					FQDN:      "fqdn",
					DNSPrefix: "foo",
					Subnet:    "mastersubnet",
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:                "agentpool",
						VMSize:              "Standard_D2_v2",
						AvailabilityProfile: VirtualMachineScaleSets,
					},
				},
			},
			expectedVirtualNetworkName: "aks-vnet-28513887",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual := test.properties.GetVirtualNetworkName()

			if actual != test.expectedVirtualNetworkName {
				t.Errorf("expected virtual network name %s, but got %s", test.expectedVirtualNetworkName, actual)
			}
		})
	}
}

func TestProperties_GetVNetResourceGroupName(t *testing.T) {
	p := &Properties{
		HostedMasterProfile: &HostedMasterProfile{
			FQDN:      "fqdn",
			DNSPrefix: "foo",
			Subnet:    "mastersubnet",
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				AvailabilityProfile: VirtualMachineScaleSets,
				VnetSubnetID:        "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/BazAgentSubnet", //nolint: lll
			},
		},
	}
	expectedVNETResourceGroupName := "RESOURCE_GROUP_NAME"

	actual := p.GetVNetResourceGroupName()

	if expectedVNETResourceGroupName != actual {
		t.Errorf("expected vnet resource group name name %s, but got %s", expectedVNETResourceGroupName, actual)
	}
}

func TestGetPrimaryAvailabilitySetName(t *testing.T) {
	p := &Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType: Kubernetes,
		},
		HostedMasterProfile: &HostedMasterProfile{
			IPMasqAgent: false,
			DNSPrefix:   "foo",
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				AvailabilityProfile: AvailabilitySet,
			},
		},
	}

	expected := "agentpool-availabilitySet-28513887"
	got := p.GetPrimaryAvailabilitySetName()
	if got != expected {
		t.Errorf("expected primary availability set name %s, but got %s", expected, got)
	}

	p.AgentPoolProfiles = []*AgentPoolProfile{
		{
			Name:                "agentpool",
			VMSize:              "Standard_D2_v2",
			AvailabilityProfile: VirtualMachineScaleSets,
		},
	}
	expected = ""
	got = p.GetPrimaryAvailabilitySetName()
	if got != expected {
		t.Errorf("expected primary availability set name %s, but got %s", expected, got)
	}

	p.AgentPoolProfiles = nil
	expected = ""
	got = p.GetPrimaryAvailabilitySetName()
	if got != expected {
		t.Errorf("expected primary availability set name %s, but got %s", expected, got)
	}
}

func TestAgentPoolProfileIsVHDDistro(t *testing.T) {
	cases := []struct {
		name     string
		ap       AgentPoolProfile
		expected bool
	}{
		{
			name: "16.04 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntu1604,
			},
			expected: true,
		},
		{
			name: "18.04 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntu1804,
			},
			expected: true,
		},
		{
			name: "ubuntu distro",
			ap: AgentPoolProfile{
				Distro: Ubuntu,
			},
			expected: false,
		},
		{
			name: "ubuntu 18.04 non-VHD distro",
			ap: AgentPoolProfile{
				Distro: Ubuntu1804,
			},
			expected: false,
		},
		{
			name: "ubuntu 18.04 gen2 non-VHD distro",
			ap: AgentPoolProfile{
				Distro: Ubuntu1804Gen2,
			},
			expected: true,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.ap.IsVHDDistro() {
				t.Fatalf("Got unexpected AgentPoolProfile.IsVHDDistro() result. Expected: %t. Got: %t.", c.expected, c.ap.IsVHDDistro())
			}
		})
	}
}

func TestAgentPoolProfileIs2204VHDDistro(t *testing.T) {
	cases := []struct {
		name     string
		ap       AgentPoolProfile
		expected bool
	}{
		{
			name: "22.04 Gen1 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuContainerd2204,
			},
			expected: true,
		},
		{
			name: "22.04 Gen2 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuContainerd2204Gen2,
			},
			expected: true,
		},
		{
			name: "22.04 ARM64 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuArm64Containerd2204Gen2,
			},
			expected: true,
		},
		{
			name: "22.04 Gen2 TrustedLaunch VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuContainerd2204TLGen2,
			},
			expected: true,
		},
		{
			name: "ubuntu 18.04 non-VHD distro",
			ap: AgentPoolProfile{
				Distro: Ubuntu1804,
			},
			expected: false,
		},
		{
			name: "ubuntu 18.04 gen2 non-VHD distro",
			ap: AgentPoolProfile{
				Distro: Ubuntu1804Gen2,
			},
			expected: false,
		},
		{
			name: "18.04 Ubuntu VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuContainerd1804,
			},
			expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.ap.Is2204VHDDistro() {
				t.Fatalf("Got unexpected AgentPoolProfile.Is2204VHDDistro() result. Expected: %t. Got: %t.", c.expected, c.ap.Is2204VHDDistro())
			}
		})
	}
}

func TestAgentPoolProfileIs2404VHDDistro(t *testing.T) {
	cases := []struct {
		name     string
		ap       AgentPoolProfile
		expected bool
	}{
		{
			name: "24.04 Gen1 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuContainerd2404,
			},
			expected: true,
		},
		{
			name: "24.04 Gen2 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuContainerd2404Gen2,
			},
			expected: true,
		},
		{
			name: "24.04 ARM64 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuArm64Containerd2404Gen2,
			},
			expected: true,
		},
		{
			name: "ubuntu 18.04 non-VHD distro",
			ap: AgentPoolProfile{
				Distro: Ubuntu1804,
			},
			expected: false,
		},
		{
			name: "ubuntu 18.04 gen2 non-VHD distro",
			ap: AgentPoolProfile{
				Distro: Ubuntu1804Gen2,
			},
			expected: false,
		},
		{
			name: "18.04 Ubuntu VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuContainerd1804,
			},
			expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.ap.Is2404VHDDistro() {
				t.Fatalf("Got unexpected AgentPoolProfile.Is2204VHDDistro() result. Expected: %t. Got: %t.", c.expected, c.ap.Is2204VHDDistro())
			}
		})
	}
}

func TestAgentPoolProfileIsAzureLinuxCgroupV2VHDDistro(t *testing.T) {
	cases := []struct {
		name     string
		ap       AgentPoolProfile
		expected bool
	}{
		{
			name: "Azure Linux V2 Gen1 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV2,
			},
			expected: true,
		},
		{
			name: "Azure Linux V3 Gen1 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV3,
			},
			expected: true,
		},
		{
			name: "Azure Linux V2 Gen2 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV2Gen2,
			},
			expected: true,
		},
		{
			name: "Azure Linux V3 Gen2 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV3Gen2,
			},
			expected: true,
		},
		{
			name: "Azure Linux V2 Gen2 ARM64 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV2Arm64Gen2,
			},
			expected: true,
		},
		{
			name: "Azure Linux V3 Gen2 ARM64 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV3Arm64Gen2,
			},
			expected: true,
		},
		{
			name: "Azure Linux V2 Gen1 FIPS VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV2FIPS,
			},
			expected: true,
		},
		{
			name: "Azure Linux V3 Gen1 FIPS VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV3FIPS,
			},
			expected: true,
		},
		{
			name: "Azure Linux V2 Gen2 FIPS VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV2Gen2FIPS,
			},
			expected: true,
		},
		{
			name: "Azure Linux V3 Gen2 FIPS VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV3Gen2FIPS,
			},
			expected: true,
		},
		{
			name: "Azure Linux V2 Gen2 TrustedLaunch VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV2Gen2TL,
			},
			expected: true,
		},
		{
			name: "Azure Linux V2 Gen2 Kata VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSAzureLinuxV2Gen2Kata,
			},
			expected: true,
		},
		{
			name: "CBLMariner V2 Gen2 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSCBLMarinerV2Gen2,
			},
			expected: false,
		},
		{
			name: "CBLMariner V2 Gen1 FIPS VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSCBLMarinerV2FIPS,
			},
			expected: false,
		},
		{
			name: "CBLMariner V2 Gen2 ARM64 VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSCBLMarinerV2Arm64Gen2,
			},
			expected: false,
		},
		{
			name: "CBLMariner V2 Gen2 TrustedLaunch VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSCBLMarinerV2Gen2TL,
			},
			expected: false,
		},
		{
			name: "18.04 Ubuntu VHD distro",
			ap: AgentPoolProfile{
				Distro: AKSUbuntuContainerd1804,
			},
			expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.ap.IsAzureLinuxCgroupV2VHDDistro() {
				t.Fatalf("Got unexpected AgentPoolProfile.IsAzureLinuxCgroupV2VHDDistro() result. Expected: %t. Got: %t.",
					c.expected, c.ap.IsAzureLinuxCgroupV2VHDDistro())
			}
		})
	}
}

func TestIsCustomVNET(t *testing.T) {
	cases := []struct {
		p             Properties
		expectedAgent bool
	}{
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						VnetSubnetID: "testSubnet",
					},
				},
			},
			expectedAgent: true,
		},
	}

	for _, c := range cases {
		if c.p.AgentPoolProfiles[0].IsCustomVNET() != c.expectedAgent {
			t.Fatalf("expected IsCustomVnet() to return %t but instead returned %t", c.expectedAgent, c.p.AgentPoolProfiles[0].IsCustomVNET())
		}
	}
}

func TestAgentPoolProfileGetKubernetesLabels(t *testing.T) {
	cases := []struct {
		name          string
		ap            AgentPoolProfile
		rg            string
		deprecated    bool
		nvidiaEnabled bool
		fipsEnabled   bool
		osSku         string
		expected      string
	}{
		{
			name:          "vanilla pool profile",
			ap:            AgentPoolProfile{},
			rg:            "my-resource-group",
			deprecated:    true,
			nvidiaEnabled: false,
			fipsEnabled:   false,
			expected:      "agentpool=,kubernetes.azure.com/agentpool=",
		},
		{
			name: "with custom labels",
			ap: AgentPoolProfile{
				CustomNodeLabels: map[string]string{
					"mycustomlabel1": "foo",
					"mycustomlabel2": "bar",
				},
			},
			rg:            "my-resource-group",
			deprecated:    true,
			nvidiaEnabled: false,
			fipsEnabled:   false,
			expected:      "agentpool=,kubernetes.azure.com/agentpool=,mycustomlabel1=foo,mycustomlabel2=bar",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.ap.GetKubernetesLabels() {
				t.Fatalf("Got unexpected AgentPoolProfile.GetKubernetesLabels(%s, %t) result. Expected: %s. Got: %s.",
					c.rg, c.deprecated, c.expected, c.ap.GetKubernetesLabels())
			}
		})
	}
}

func TestHasStorageProfile(t *testing.T) {
	cases := []struct {
		name                     string
		p                        Properties
		expectedHasMD            bool
		expectedHasSA            bool
		expectedMasterMD         bool
		expectedAgent0E          bool
		expectedAgent0MD         bool
		expectedPrivateJB        bool
		expectedHasDisks         bool
		expectedDesID            string
		expectedEncryptionAtHost bool
	}{
		{
			name: "Storage Account",
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: StorageAccount,
					},
					{
						StorageProfile: StorageAccount,
					},
				},
			},
			expectedHasMD:    false,
			expectedHasSA:    true,
			expectedMasterMD: false,
			expectedAgent0MD: false,
			expectedAgent0E:  false,
			expectedHasDisks: true,
		},
		{
			name: "Managed Disk",
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: StorageAccount,
					},
					{
						StorageProfile: StorageAccount,
					},
				},
			},
			expectedHasMD:    true,
			expectedHasSA:    true,
			expectedMasterMD: true,
			expectedAgent0MD: false,
			expectedAgent0E:  false,
		},
		{
			name: "both",
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: ManagedDisks,
					},
					{
						StorageProfile: StorageAccount,
					},
				},
			},
			expectedHasMD:    true,
			expectedHasSA:    true,
			expectedMasterMD: false,
			expectedAgent0MD: true,
			expectedAgent0E:  false,
		},
		{
			name: "Managed Disk everywhere",
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: ManagedDisks,
					},
					{
						StorageProfile: ManagedDisks,
					},
				},
			},
			expectedHasMD:     true,
			expectedHasSA:     false,
			expectedMasterMD:  true,
			expectedAgent0MD:  true,
			expectedAgent0E:   false,
			expectedPrivateJB: false,
		},
		{
			name: "Managed disk master with ephemeral agent",
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: Ephemeral,
					},
				},
			},
			expectedHasMD:     true,
			expectedHasSA:     false,
			expectedMasterMD:  true,
			expectedAgent0MD:  false,
			expectedAgent0E:   true,
			expectedPrivateJB: false,
		},
		{
			name: "Mixed with jumpbox",
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						PrivateCluster: &PrivateCluster{
							Enabled: to.BoolPtr(true),
							JumpboxProfile: &PrivateJumpboxProfile{
								StorageProfile: ManagedDisks,
							},
						},
					},
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: StorageAccount,
					},
				},
			},
			expectedHasMD:     true,
			expectedHasSA:     true,
			expectedMasterMD:  false,
			expectedAgent0MD:  false,
			expectedAgent0E:   false,
			expectedPrivateJB: true,
		},
		{
			name: "Mixed with jumpbox alternate",
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						PrivateCluster: &PrivateCluster{
							Enabled: to.BoolPtr(true),
							JumpboxProfile: &PrivateJumpboxProfile{
								StorageProfile: StorageAccount,
							},
						},
					},
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: ManagedDisks,
					},
				},
			},
			expectedHasMD:     true,
			expectedHasSA:     true,
			expectedMasterMD:  true,
			expectedAgent0MD:  true,
			expectedAgent0E:   false,
			expectedPrivateJB: true,
		},
		{
			name: "Managed Disk with DiskEncryptionSetID setting",
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: ManagedDisks,
					},
					{
						StorageProfile: ManagedDisks,
					},
				},
			},
			expectedHasMD:     true,
			expectedHasSA:     false,
			expectedMasterMD:  true,
			expectedAgent0MD:  true,
			expectedAgent0E:   false,
			expectedPrivateJB: false,
			expectedDesID:     "DiskEncryptionSetID",
		},
		{
			name: "EncryptionAtHost setting",
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: ManagedDisks,
					},
					{
						StorageProfile: ManagedDisks,
					},
				},
			},
			expectedHasMD:            true,
			expectedHasSA:            false,
			expectedMasterMD:         true,
			expectedAgent0MD:         true,
			expectedAgent0E:          false,
			expectedPrivateJB:        false,
			expectedEncryptionAtHost: true,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.p.OrchestratorProfile != nil && c.p.OrchestratorProfile.KubernetesConfig.PrivateJumpboxProvision() != c.expectedPrivateJB {
				t.Fatalf("expected PrivateJumpboxProvision() to return %t but instead returned %t", c.expectedPrivateJB,
					c.p.OrchestratorProfile.KubernetesConfig.PrivateJumpboxProvision())
			}
		})
	}
}

func TestLinuxProfile(t *testing.T) {
	l := LinuxProfile{}

	if l.HasSecrets() || l.HasSearchDomain() {
		t.Fatalf("Expected HasSecrets() and HasSearchDomain() to return false when LinuxProfile is empty")
	}

	l = LinuxProfile{
		Secrets: []KeyVaultSecrets{
			{
				SourceVault: &KeyVaultID{"testVault"},
				VaultCertificates: []KeyVaultCertificate{
					{
						CertificateURL:   "testURL",
						CertificateStore: "testStore",
					},
				},
			},
		},
		CustomSearchDomain: &CustomSearchDomain{
			Name:          "testName",
			RealmPassword: "testRealmPassword",
			RealmUser:     "testRealmUser",
		},
	}

	if !(l.HasSecrets() && l.HasSearchDomain()) {
		t.Fatalf("Expected HasSecrets() and HasSearchDomain() to return true")
	}
}

//nolint:gocognit
func TestWindowsProfile(t *testing.T) {
	trueVar := true
	w := WindowsProfile{}

	if w.HasSecrets() || w.HasCustomImage() {
		t.Fatalf("Expected HasSecrets() and HasCustomImage() to return false when WindowsProfile is empty")
	}

	dv := w.GetWindowsDockerVersion()
	if dv != KubernetesWindowsDockerVersion {
		t.Fatalf("Expected GetWindowsDockerVersion() to equal default KubernetesWindowsDockerVersion, got %s", dv)
	}

	dh := w.GetDefaultContainerdWindowsSandboxIsolation()
	if dh != KubernetesDefaultContainerdWindowsSandboxIsolation {
		t.Fatalf("Expected GetWindowsDefaultRuntimeHandler() to equal default KubernetesDefaultContainerdWindowsSandboxIsolation, got %s", dh)
	}

	rth := w.GetContainerdWindowsRuntimeHandlers()
	if rth != "" {
		t.Fatalf("Expected GetContainerdWindowsRuntimeHandlers() to equal default empty, got %s", rth)
	}

	windowsSku := w.GetWindowsSku()
	if windowsSku != KubernetesDefaultWindowsSku {
		t.Fatalf("Expected GetWindowsSku() to equal default KubernetesDefaultWindowsSku, got %s", windowsSku)
	}

	isCSIProxyEnabled := w.IsCSIProxyEnabled()
	if isCSIProxyEnabled != DefaultEnableCSIProxyWindows {
		t.Fatalf("Expected IsCSIProxyEnabled() to equal default DefaultEnableCSIProxyWindows, got %t", isCSIProxyEnabled)
	}

	isAlwaysPullWindowsPauseImage := w.IsAlwaysPullWindowsPauseImage()
	if isAlwaysPullWindowsPauseImage {
		t.Fatalf("Expected IsAlwaysPullWindowsPauseImage() to equal default false, got %t", isAlwaysPullWindowsPauseImage)
	}
	w = WindowsProfile{
		Secrets: []KeyVaultSecrets{
			{
				SourceVault: &KeyVaultID{"testVault"},
				VaultCertificates: []KeyVaultCertificate{
					{
						CertificateURL:   "testURL",
						CertificateStore: "testStore",
					},
				},
			},
		},
		WindowsImageSourceURL:       "testCustomImage",
		IsCredentialAutoGenerated:   to.BoolPtr(true),
		EnableAHUB:                  to.BoolPtr(true),
		EnableCSIProxy:              to.BoolPtr(true),
		AlwaysPullWindowsPauseImage: to.BoolPtr(true),
	}

	if !(w.HasSecrets() && w.HasCustomImage()) {
		t.Fatalf("Expected HasSecrets() and HasCustomImage() to return true")
	}

	isCSIProxyEnabled = w.IsCSIProxyEnabled()
	if !isCSIProxyEnabled {
		t.Fatalf("Expected IsCSIProxyEnabled() to equal true, got %t", isCSIProxyEnabled)
	}

	isAlwaysPullWindowsPauseImage = w.IsAlwaysPullWindowsPauseImage()
	if !isAlwaysPullWindowsPauseImage {
		t.Fatalf("Expected IsAlwaysPullWindowsPauseImage() to equal true, got %t", isAlwaysPullWindowsPauseImage)
	}

	w = WindowsProfile{
		WindowsDockerVersion:      "18.03.1-ee-3",
		WindowsSku:                "Datacenter-Core-1809-with-Containers-smalldisk",
		SSHEnabled:                &trueVar,
		IsCredentialAutoGenerated: to.BoolPtr(false),
		EnableAHUB:                to.BoolPtr(false),
		ContainerdWindowsRuntimes: &ContainerdWindowsRuntimes{
			DefaultSandboxIsolation: "hyperv",
			RuntimeHandlers: []RuntimeHandlers{
				{BuildNumber: "17763"},
				{BuildNumber: "18362"},
			},
		},
		WindowsGmsaPackageUrl:   "windows-gmsa-package-url",
		WindowsSecureTlsEnabled: to.BoolPtr(false),
	}

	dv = w.GetWindowsDockerVersion()
	if dv != "18.03.1-ee-3" {
		t.Fatalf("Expected GetWindowsDockerVersion() to equal 18.03.1-ee-3, got %s", dv)
	}

	windowsSku = w.GetWindowsSku()
	if windowsSku != "Datacenter-Core-1809-with-Containers-smalldisk" {
		t.Fatalf("Expected GetWindowsSku() to equal Datacenter-Core-1809-with-Containers-smalldisk, got %s", windowsSku)
	}

	dv = w.GetWindowsDockerVersion()
	if dv != "18.03.1-ee-3" {
		t.Fatalf("Expected GetWindowsDockerVersion() to equal 18.03.1-ee-3, got %s", dv)
	}

	windowsSku = w.GetWindowsSku()
	if windowsSku != "Datacenter-Core-1809-with-Containers-smalldisk" {
		t.Fatalf("Expected GetWindowsSku() to equal Datacenter-Core-1809-with-Containers-smalldisk, got %s", windowsSku)
	}

	se := w.GetSSHEnabled()
	if !se {
		t.Fatalf("Expected SSHEnabled to return true, got %v", se)
	}

	jsonBytes, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("Expected JSON marshal to not return an error, but returned: %s", err)
	}

	unmarshalled := WindowsProfile{}
	if err = json.Unmarshal(jsonBytes, &unmarshalled); err != nil {
		t.Fatalf("Expected JSON unmarshal to not return an error, but returned: %s", err)
	}

	dv = unmarshalled.GetWindowsDockerVersion()
	if dv != "18.03.1-ee-3" {
		t.Fatalf("Expected unmarshalled GetWindowsDockerVersion() to equal 18.03.1-ee-3, got %s", dv)
	}

	windowsSku = unmarshalled.GetWindowsSku()
	if windowsSku != "Datacenter-Core-1809-with-Containers-smalldisk" {
		t.Fatalf("Expected unmarshalled GetWindowsSku() to equal Datacenter-Core-1809-with-Containers-smalldisk, got %s", windowsSku)
	}

	dv = unmarshalled.GetWindowsDockerVersion()
	if dv != "18.03.1-ee-3" {
		t.Fatalf("Expected unmarshalled GetWindowsDockerVersion() to equal 18.03.1-ee-3, got %s", dv)
	}

	windowsSku = unmarshalled.GetWindowsSku()
	if windowsSku != "Datacenter-Core-1809-with-Containers-smalldisk" {
		t.Fatalf("Expected unmarshalled GetWindowsSku() to equal Datacenter-Core-1809-with-Containers-smalldisk, got %s", windowsSku)
	}

	se = unmarshalled.GetSSHEnabled()
	if !se {
		t.Fatalf("Expected unmarshalled SSHEnabled to return true, got %v", se)
	}

	if unmarshalled.WindowsGmsaPackageUrl != "windows-gmsa-package-url" {
		t.Fatalf("Expected unmarshalled WindowsGMSAPackageURL to equal windows-gmsa-package-url, got %s", unmarshalled.WindowsGmsaPackageUrl)
	}

	if unmarshalled.WindowsSecureTlsEnabled == nil {
		t.Fatalf("Execpted unmarshalled WindowsSecureTLSEnabled to not be nil")
	}

	if *unmarshalled.WindowsSecureTlsEnabled != false {
		t.Fatalf("Expected unmarshalled WindowsSecureTLSEnabled to equal false, got %v", *unmarshalled.WindowsSecureTlsEnabled)
	}
}

func TestWindowsProfileCustomOS(t *testing.T) {
	cases := []struct {
		name            string
		w               WindowsProfile
		expectedRef     bool
		expectedGallery bool
		expectedURL     bool
	}{
		{
			name: "valid shared gallery image",
			w: WindowsProfile{
				ImageRef: &ImageReference{
					Name:           "test",
					ResourceGroup:  "testRG",
					SubscriptionID: "testSub",
					Gallery:        "testGallery",
					Version:        "0.1.0",
				},
			},
			expectedRef:     true,
			expectedGallery: true,
			expectedURL:     false,
		},
		{
			name: "valid non-shared image",
			w: WindowsProfile{
				ImageRef: &ImageReference{
					Name:          "test",
					ResourceGroup: "testRG",
				},
			},
			expectedRef:     true,
			expectedGallery: false,
			expectedURL:     false,
		},
		{
			name: "valid image URL",
			w: WindowsProfile{
				WindowsImageSourceURL: "https://some/image.vhd",
			},
			expectedRef:     false,
			expectedGallery: false,
			expectedURL:     true,
		},
		{
			name:            "valid no custom image",
			w:               WindowsProfile{},
			expectedRef:     false,
			expectedGallery: false,
			expectedURL:     false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.w.HasCustomImage() != c.expectedURL {
				t.Errorf("expected HasCustomImage() to return %t but instead returned %t", c.expectedURL, c.w.HasCustomImage())
			}
			if c.w.HasImageRef() != c.expectedRef {
				t.Errorf("expected HasImageRef() to return %t but instead returned %t", c.expectedRef, c.w.HasImageRef())
			}
		})
	}
}

func TestIsAzureCNI(t *testing.T) {
	k := &KubernetesConfig{
		NetworkPlugin: NetworkPluginAzure,
	}

	o := &OrchestratorProfile{
		KubernetesConfig: k,
	}
	if !o.IsAzureCNI() {
		t.Fatalf("unable to detect orchestrator profile is using Azure CNI from NetworkPlugin=%s", o.KubernetesConfig.NetworkPlugin)
	}

	k = &KubernetesConfig{
		NetworkPlugin: "none",
	}

	o = &OrchestratorProfile{
		KubernetesConfig: k,
	}
	if o.IsAzureCNI() {
		t.Fatalf("unable to detect orchestrator profile is not using Azure CNI from NetworkPlugin=%s", o.KubernetesConfig.NetworkPlugin)
	}

	o = &OrchestratorProfile{}
	if o.IsAzureCNI() {
		t.Fatalf("unable to detect orchestrator profile is not using Azure CNI from nil KubernetesConfig")
	}
}

func TestIsNoneCNI(t *testing.T) {
	k := &KubernetesConfig{
		NetworkPlugin: NetworkPluginNone,
	}

	o := &OrchestratorProfile{
		KubernetesConfig: k,
	}
	if !o.IsNoneCNI() {
		t.Fatalf("unable to detect orchestrator profile is using None CNI from NetworkPlugin=%s", o.KubernetesConfig.NetworkPlugin)
	}

	k = &KubernetesConfig{
		NetworkPlugin: "some",
	}

	o = &OrchestratorProfile{
		KubernetesConfig: k,
	}
	if o.IsNoneCNI() {
		t.Fatalf("unable to detect orchestrator profile is not using None CNI from NetworkPlugin=%s", o.KubernetesConfig.NetworkPlugin)
	}

	o = &OrchestratorProfile{}
	if o.IsNoneCNI() {
		t.Fatalf("unable to detect orchestrator profile is not using None CNI from nil KubernetesConfig")
	}
}

func TestOrchestrator(t *testing.T) {
	cases := []struct {
		p                    Properties
		expectedIsKubernetes bool
	}{
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: "NotKubernetes",
				},
			},
			expectedIsKubernetes: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
				},
			},
			expectedIsKubernetes: true,
		},
	}

	for _, c := range cases {
		if c.expectedIsKubernetes != c.p.OrchestratorProfile.IsKubernetes() {
			t.Fatalf("Expected IsKubernetes() to be %t with OrchestratorType=%s", c.expectedIsKubernetes, c.p.OrchestratorProfile.OrchestratorType)
		}
	}
}

func TestIsFeatureEnabled(t *testing.T) {
	tests := []struct {
		name     string
		feature  string
		flags    *FeatureFlags
		expected bool
	}{
		{
			name:     "nil flags",
			feature:  "BlockOutboundInternet",
			flags:    nil,
			expected: false,
		},
		{
			name:     "empty flags",
			feature:  "BlockOutboundInternet",
			flags:    &FeatureFlags{},
			expected: false,
		},
		{
			name:    "Enabled feature",
			feature: "CSERunInBackground",
			flags: &FeatureFlags{
				EnableCSERunInBackground: true,
				BlockOutboundInternet:    false,
			},
			expected: true,
		},
		{
			name:    "Disabled feature",
			feature: "CSERunInBackground",
			flags: &FeatureFlags{
				EnableCSERunInBackground: false,
				BlockOutboundInternet:    true,
			},
			expected: false,
		},
		{
			name:    "Non-existent feature",
			feature: "Foo",
			flags: &FeatureFlags{
				EnableCSERunInBackground: true,
				BlockOutboundInternet:    true,
			},
			expected: false,
		},
		{
			name:    "Windows DSR",
			feature: "EnableWinDSR",
			flags: &FeatureFlags{
				EnableWinDSR: true,
			},
			expected: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual := test.flags.IsFeatureEnabled(test.feature)
			if actual != test.expected {
				t.Errorf("expected feature %s to be enabled:%v, but got %v", test.feature, test.expected, actual)
			}
		})
	}
}

func TestGetKubeProxyFeatureGatesWindowsArguments(t *testing.T) {
	tests := []struct {
		name                 string
		properties           *Properties
		expectedFeatureGates string
	}{
		{
			name: "default",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{},
			},
			expectedFeatureGates: "",
		},
		{
			name: "Non kubeproxy feature",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{},
			},
			expectedFeatureGates: "",
		},
		{
			name: "IPV6 enabled",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{
					EnableIPv6DualStack: true,
				},
			},
			expectedFeatureGates: "\"IPv6DualStack=true\"",
		},
		{
			name: "IPV6 enabled but version does not have feature gate (too old)",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{
					EnableIPv6DualStack: true,
				},
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorVersion: "1.11.0",
				},
			},
			expectedFeatureGates: "",
		},
		{
			name: "IPV6 enabled but version does not have feature gate",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{
					EnableIPv6DualStack: true,
				},
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorVersion: "1.25.0",
				},
			},
			expectedFeatureGates: "",
		},
		{
			name: "IPV6 enabled and version has feature gate (>= 1.15 < 1.25)",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{
					EnableIPv6DualStack: true,
				},
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorVersion: "1.24.12",
				},
			},
			expectedFeatureGates: "\"IPv6DualStack=true\"",
		},
		{
			name: "IPV6 enabled but version does not have feature gate",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{
					EnableIPv6DualStack: true,
					EnableWinDSR:        true,
				},
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorVersion: "1.25.0",
				},
			},
			expectedFeatureGates: "\"WinDSR=true\", \"WinOverlay=false\"",
		},
		{
			name: "IPv6 enabled but version does not have feature gate",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{
					EnableIPv6DualStack: true,
					EnableWinDSR:        true,
				},
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorVersion: "1.26.2",
				},
			},
			expectedFeatureGates: "\"WinDSR=true\", \"WinOverlay=false\"",
		},
		{
			name: "WinDSR enabled",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{
					EnableWinDSR: true,
				},
			},
			expectedFeatureGates: "\"WinDSR=true\", \"WinOverlay=false\"",
		},
		{
			name: "both IPV6 and WinDSR enabled",
			properties: &Properties{
				FeatureFlags: &FeatureFlags{
					EnableIPv6DualStack: true,
					EnableWinDSR:        true,
				},
			},
			expectedFeatureGates: "\"IPv6DualStack=true\", \"WinDSR=true\", \"WinOverlay=false\"",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual := test.properties.GetKubeProxyFeatureGatesWindowsArguments()
			if actual != test.expectedFeatureGates {
				t.Errorf("expected featureGates %s, but got %s", test.expectedFeatureGates, actual)
			}
		})
	}
}

func TestKubernetesConfigIsAddonEnabled(t *testing.T) {
	cases := []struct {
		k         *KubernetesConfig
		addonName string
		expected  bool
	}{
		{
			k:         &KubernetesConfig{},
			addonName: "foo",
			expected:  false,
		},
		{
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name: "foo",
					},
				},
			},
			addonName: "foo",
			expected:  false,
		},
		{
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name:    "foo",
						Enabled: to.BoolPtr(false),
					},
				},
			},
			addonName: "foo",
			expected:  false,
		},
		{
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name:    "foo",
						Enabled: to.BoolPtr(true),
					},
				},
			},
			addonName: "foo",
			expected:  true,
		},
		{
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name:    "bar",
						Enabled: to.BoolPtr(true),
					},
				},
			},
			addonName: "foo",
			expected:  false,
		},
	}

	for _, c := range cases {
		if c.k.IsAddonEnabled(c.addonName) != c.expected {
			t.Fatalf("expected KubernetesConfig.IsAddonEnabled(%s) to return %t but instead returned %t", c.addonName, c.expected,
				c.k.IsAddonEnabled(c.addonName))
		}
	}
}

func TestKubernetesConfigIsIPMasqAgentDisabled(t *testing.T) {
	cases := []struct {
		name             string
		k                *KubernetesConfig
		expectedDisabled bool
	}{
		{
			name:             "default",
			k:                &KubernetesConfig{},
			expectedDisabled: false,
		},
		{
			name: "ip-masq-agent present but no configuration",
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name: IPMASQAgentAddonName,
					},
				},
			},
			expectedDisabled: false,
		},
		{
			name: "ip-masq-agent explicitly disabled",
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name:    IPMASQAgentAddonName,
						Enabled: to.BoolPtr(false),
					},
				},
			},
			expectedDisabled: true,
		},
		{
			name: "ip-masq-agent explicitly enabled",
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name:    IPMASQAgentAddonName,
						Enabled: to.BoolPtr(true),
					},
				},
			},
			expectedDisabled: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.k.IsIPMasqAgentDisabled() != c.expectedDisabled {
				t.Fatalf("expected KubernetesConfig.IsIPMasqAgentDisabled() to return %t but instead returned %t", c.expectedDisabled,
					c.k.IsIPMasqAgentDisabled())
			}
		})
	}
}

func TestGetAddonByName(t *testing.T) {
	containerMonitoringAddonName := "container-monitoring"

	// Addon present and enabled with logAnalyticsWorkspaceResourceId in config
	b := true
	c := KubernetesConfig{
		Addons: []KubernetesAddon{
			{
				Name:    containerMonitoringAddonName,
				Enabled: &b,
				Config: map[string]string{
					"logAnalyticsWorkspaceResourceId": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test-workspace-rg/providers/Microsoft.OperationalInsights/workspaces/test-workspace", //nolint:lll
				},
			},
		},
	}

	addon := c.GetAddonByName(containerMonitoringAddonName)
	if addon.Config == nil || len(addon.Config) == 0 {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config instead returned null or empty")
	}

	if addon.Config["logAnalyticsWorkspaceResourceId"] == "" {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config with logAnalyticsWorkspaceResourceId," +
			" instead returned null or empty")
	}

	workspaceResourceID := addon.Config["logAnalyticsWorkspaceResourceId"]
	if workspaceResourceID == "" {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config with non empty azure logAnalyticsWorkspaceResourceId")
	}

	resourceParts := strings.Split(workspaceResourceID, "/")
	if len(resourceParts) != 9 {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config with valid Azure logAnalyticsWorkspaceResourceId,"+
			" instead returned %s", workspaceResourceID)
	}

	// Addon present and enabled with legacy config
	b = true
	c = KubernetesConfig{
		Addons: []KubernetesAddon{
			{
				Name:    containerMonitoringAddonName,
				Enabled: &b,
				Config: map[string]string{
					"workspaceGuid": "MDAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAw",
					"workspaceKey":  "NEQrdnlkNS9qU2NCbXNBd1pPRi8wR09CUTVrdUZRYzlKVmFXK0hsbko1OGN5ZVBKY3dUcGtzK3JWbXZnY1hHbW15dWpMRE5FVlBpVDhwQjI3NGE5WWc9PQ==",
				},
			},
		},
	}

	addon = c.GetAddonByName(containerMonitoringAddonName)
	if addon.Config == nil || len(addon.Config) == 0 {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config instead returned null or empty")
	}

	if addon.Config["workspaceGuid"] == "" {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config with non empty workspaceGuid")
	}

	if addon.Config["workspaceKey"] == "" {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config with non empty workspaceKey")
	}
}

func TestKubernetesConfigIsAddonDisabled(t *testing.T) {
	cases := []struct {
		k         *KubernetesConfig
		addonName string
		expected  bool
	}{
		{
			k:         &KubernetesConfig{},
			addonName: "foo",
			expected:  false,
		},
		{
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name: "foo",
					},
				},
			},
			addonName: "foo",
			expected:  false,
		},
		{
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name:    "foo",
						Enabled: to.BoolPtr(false),
					},
				},
			},
			addonName: "foo",
			expected:  true,
		},
		{
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name:    "foo",
						Enabled: to.BoolPtr(true),
					},
				},
			},
			addonName: "foo",
			expected:  false,
		},
		{
			k: &KubernetesConfig{
				Addons: []KubernetesAddon{
					{
						Name:    "bar",
						Enabled: to.BoolPtr(true),
					},
				},
			},
			addonName: "foo",
			expected:  false,
		},
	}

	for _, c := range cases {
		if c.k.IsAddonDisabled(c.addonName) != c.expected {
			t.Fatalf("expected KubernetesConfig.IsAddonDisabled(%s) to return %t but instead returned %t", c.addonName, c.expected,
				c.k.IsAddonDisabled(c.addonName))
		}
	}
}

func TestHasContainerd(t *testing.T) {
	tests := []struct {
		name     string
		k        *KubernetesConfig
		expected bool
	}{
		{
			name: "docker",
			k: &KubernetesConfig{
				ContainerRuntime: Docker,
			},
			expected: false,
		},
		{
			name: "empty string",
			k: &KubernetesConfig{
				ContainerRuntime: "",
			},
			expected: false,
		},
		{
			name: "unexpected string",
			k: &KubernetesConfig{
				ContainerRuntime: "foo",
			},
			expected: false,
		},
		{
			name: "containerd",
			k: &KubernetesConfig{
				ContainerRuntime: Containerd,
			},
			expected: true,
		},
		{
			name: "kata",
			k: &KubernetesConfig{
				ContainerRuntime: KataContainers,
			},
			expected: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ret := test.k.NeedsContainerd()
			if test.expected != ret {
				t.Errorf("expected %t, instead got : %t", test.expected, ret)
			}
		})
	}
}

func TestKubernetesConfig_RequiresDocker(t *testing.T) {
	// k8sConfig with empty runtime string
	k := &KubernetesConfig{
		ContainerRuntime: "",
	}

	if !k.RequiresDocker() {
		t.Error("expected RequiresDocker to return true for empty runtime string")
	}

	// k8sConfig with empty runtime string
	k = &KubernetesConfig{
		ContainerRuntime: Docker,
	}

	if !k.RequiresDocker() {
		t.Error("expected RequiresDocker to return true for docker runtime")
	}
}

func TestKubernetesConfigGetOrderedKubeletConfigString(t *testing.T) {
	alphabetizedStringForPowershell := `"--address=0.0.0.0", "--allow-privileged=true", "--anonymous-auth=false", "--authorization-mode=Webhook", "--cgroups-per-qos=true", "--client-ca-file=/etc/kubernetes/certs/ca.crt", "--container-log-max-files=20", "--container-log-max-size=1024Mi", "--image-gc-high-threshold=80", "--image-gc-low-threshold=60", "--keep-terminated-pod-volumes=false", "--kubeconfig=/var/lib/kubelet/kubeconfig", "--pod-manifest-path=/etc/kubernetes/manifests"` //nolint:lll
	cases := []struct {
		name                  string
		config                *NodeBootstrappingConfiguration
		CustomKubeletConfig   *CustomKubeletConfig
		expectedForPowershell string
	}{
		{
			name:                  "zero value kubernetesConfig",
			config:                &NodeBootstrappingConfiguration{},
			CustomKubeletConfig:   nil,
			expectedForPowershell: "",
		},
		// Some values
		{
			name: "expected values",
			config: &NodeBootstrappingConfiguration{
				KubeletConfig: map[string]string{
					"--address":                      "0.0.0.0",
					"--allow-privileged":             "true",
					"--anonymous-auth":               "false",
					"--authorization-mode":           "Webhook",
					"--client-ca-file":               "/etc/kubernetes/certs/ca.crt",
					"--pod-manifest-path":            "/etc/kubernetes/manifests",
					"--node-status-report-frequency": "5m0s",
					"--cgroups-per-qos":              "true",
					"--image-gc-high-threshold":      "80",
					"--image-gc-low-threshold":       "60",
					"--kubeconfig":                   "/var/lib/kubelet/kubeconfig",
					"--keep-terminated-pod-volumes":  "false",
				},
			},
			CustomKubeletConfig: &CustomKubeletConfig{
				ImageGcHighThreshold:  to.Int32Ptr(80),
				ImageGcLowThreshold:   to.Int32Ptr(60),
				ContainerLogMaxSizeMB: to.Int32Ptr(1024),
				ContainerLogMaxFiles:  to.Int32Ptr(20),
			},
			expectedForPowershell: alphabetizedStringForPowershell,
		},
		// Switch the "order" in the map, validate the same return string
		{
			name: "expected values re-ordered",
			config: &NodeBootstrappingConfiguration{
				KubeletConfig: map[string]string{
					"--address":                      "0.0.0.0",
					"--allow-privileged":             "true",
					"--anonymous-auth":               "false",
					"--authorization-mode":           "Webhook",
					"--client-ca-file":               "/etc/kubernetes/certs/ca.crt",
					"--container-log-max-files":      "20",
					"--container-log-max-size":       "1024Mi",
					"--pod-manifest-path":            "/etc/kubernetes/manifests",
					"--node-status-report-frequency": "1m0s",
					"--cgroups-per-qos":              "true",
					"--kubeconfig":                   "/var/lib/kubelet/kubeconfig",
					"--keep-terminated-pod-volumes":  "false",
				},
			},
			CustomKubeletConfig: &CustomKubeletConfig{
				ImageGcHighThreshold:  to.Int32Ptr(80),
				ImageGcLowThreshold:   to.Int32Ptr(60),
				ContainerLogMaxSizeMB: to.Int32Ptr(1024),
				ContainerLogMaxFiles:  to.Int32Ptr(20),
			},
			expectedForPowershell: alphabetizedStringForPowershell,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expectedForPowershell != c.config.GetOrderedKubeletConfigStringForPowershell(c.CustomKubeletConfig) {
				t.Fatalf("Got unexpected AgentPoolProfile.GetOrderedKubeletConfigStringForPowershell() result. Expected: %s. Got: %s.",
					c.expectedForPowershell, c.config.GetOrderedKubeletConfigStringForPowershell(c.CustomKubeletConfig))
			}
		})
	}
}

func TestKubernetesAddonIsEnabled(t *testing.T) {
	cases := []struct {
		a        *KubernetesAddon
		expected bool
	}{
		{
			a:        &KubernetesAddon{},
			expected: false,
		},
		{
			a: &KubernetesAddon{
				Enabled: to.BoolPtr(false),
			},
			expected: false,
		},
		{
			a: &KubernetesAddon{
				Enabled: to.BoolPtr(true),
			},
			expected: true,
		},
	}

	for _, c := range cases {
		if c.a.IsEnabled() != c.expected {
			t.Fatalf("expected IsEnabled() to return %t but instead returned %t", c.expected, c.a.IsEnabled())
		}
	}
}

func TestKubernetesAddonIsDisabled(t *testing.T) {
	cases := []struct {
		a        *KubernetesAddon
		expected bool
	}{
		{
			a:        &KubernetesAddon{},
			expected: false,
		},
		{
			a: &KubernetesAddon{
				Enabled: to.BoolPtr(false),
			},
			expected: true,
		},
		{
			a: &KubernetesAddon{
				Enabled: to.BoolPtr(true),
			},
			expected: false,
		},
	}

	for _, c := range cases {
		if c.a.IsDisabled() != c.expected {
			t.Fatalf("expected IsDisabled() to return %t but instead returned %t", c.expected, c.a.IsDisabled())
		}
	}
}

func TestGetAddonContainersIndexByName(t *testing.T) {
	addonName := "testaddon"
	addon := getMockAddon(addonName)
	i := addon.GetAddonContainersIndexByName(addonName)
	if i != 0 {
		t.Fatalf("getAddonContainersIndexByName() did not return the expected index value 0, instead returned: %d", i)
	}
	i = addon.GetAddonContainersIndexByName("nonExistentContainerName")
	if i != -1 {
		t.Fatalf("getAddonContainersIndexByName() did not return the expected index value -1, instead returned: %d", i)
	}
}

func TestKubernetesConfig_UserAssignedIDEnabled(t *testing.T) {
	k := KubernetesConfig{
		UseManagedIdentity: true,
		UserAssignedID:     "fooID",
	}
	if !k.UserAssignedIDEnabled() {
		t.Errorf("expected userAssignedIDEnabled to be true when UseManagedIdentity is true and UserAssignedID is non-empty")
	}

	k = KubernetesConfig{
		UseManagedIdentity: false,
		UserAssignedID:     "fooID",
	}

	if k.UserAssignedIDEnabled() {
		t.Errorf("expected userAssignedIDEnabled to be false when useManagedIdentity is set to false")
	}
}

func TestGetOrderedKubeproxyConfigStringForPowershell(t *testing.T) {
	cases := []struct {
		name     string
		config   *NodeBootstrappingConfiguration
		expected string
	}{
		{
			name: "KubeproxyConfig is empty",
			config: &NodeBootstrappingConfiguration{
				ContainerService: &ContainerService{
					Properties: &Properties{},
				},
			},
			expected: `"--metrics-bind-address=0.0.0.0:10249"`,
		},
		{
			name: "default metrics bind address is used",
			config: &NodeBootstrappingConfiguration{
				ContainerService: &ContainerService{
					Properties: &Properties{},
				},
				KubeproxyConfig: map[string]string{
					"--metrics-bind-address": "0.0.0.0:10250",
				},
			},
			expected: `"--metrics-bind-address=0.0.0.0:10250"`,
		},
		{
			name: "KubeproxyConfig is not empty",
			config: &NodeBootstrappingConfiguration{
				ContainerService: &ContainerService{
					Properties: &Properties{},
				},
				KubeproxyConfig: map[string]string{
					"--hostname-override": "fakehost",
				},
			},
			expected: `"--hostname-override=fakehost", "--metrics-bind-address=0.0.0.0:10249"`,
		},
		{
			name: "custom configuration overrides default kubeproxyconfig",
			config: &NodeBootstrappingConfiguration{
				ContainerService: &ContainerService{
					Properties: &Properties{
						CustomConfiguration: &CustomConfiguration{
							WindowsKubernetesConfigurations: map[string]*ComponentConfiguration{
								string(ComponentkubeProxy): {
									Config: map[string]string{"--hostname-override": "override"},
								},
							},
						},
					},
				},
				KubeproxyConfig: map[string]string{
					"--hostname-override": "fakehost",
				},
			},
			expected: `"--hostname-override=override", "--metrics-bind-address=0.0.0.0:10249"`,
		},
		{
			name: "custom configuration does not override default kubeproxyconfig",
			config: &NodeBootstrappingConfiguration{
				ContainerService: &ContainerService{
					Properties: &Properties{
						CustomConfiguration: &CustomConfiguration{
							WindowsKubernetesConfigurations: map[string]*ComponentConfiguration{
								string(ComponentkubeProxy): {
									Config: map[string]string{"--kube-api-qps": "10"},
								},
							},
						},
					},
				},
				KubeproxyConfig: map[string]string{
					"--hostname-override": "fakehost",
				},
			},
			expected: `"--hostname-override=fakehost", "--kube-api-qps=10", "--metrics-bind-address=0.0.0.0:10249"`,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			actual := c.config.GetOrderedKubeproxyConfigStringForPowershell()
			if c.expected != actual {
				t.Fatalf("test case: %s, expected: %s. Got: %s.", c.name, c.expected, actual)
			}
		})
	}
}

func TestGetOrderedKubeletConfigStringForPowershell(t *testing.T) {
	cases := []struct {
		name                string
		config              *NodeBootstrappingConfiguration
		CustomKubeletConfig *CustomKubeletConfig
		expected            string
	}{
		{
			name: "KubeletConfig is empty",
			config: &NodeBootstrappingConfiguration{
				ContainerService: &ContainerService{
					Properties: &Properties{},
				},
			},
			CustomKubeletConfig: nil,
			expected:            "",
		},
		{
			name: "KubeletConfig is not empty",
			config: &NodeBootstrappingConfiguration{
				ContainerService: &ContainerService{
					Properties: &Properties{},
				},
				KubeletConfig: map[string]string{
					"--address":                      "0.0.0.0",
					"--allow-privileged":             "true",
					"--cloud-config":                 "c:\\k\\azure.json",
					"--node-status-report-frequency": "5m0s",
				},
			},
			CustomKubeletConfig: &CustomKubeletConfig{
				ImageGcLowThreshold:  to.Int32Ptr(60),
				ImageGcHighThreshold: to.Int32Ptr(80),
			},
			expected: `"--address=0.0.0.0", "--allow-privileged=true", "--cloud-config=c:\k\azure.json", "--image-gc-high-threshold=80", "--image-gc-low-threshold=60"`,
		},
		{
			name: "custom configuration overrides default KubeletConfig",
			config: &NodeBootstrappingConfiguration{
				ContainerService: &ContainerService{
					Properties: &Properties{
						CustomConfiguration: &CustomConfiguration{
							WindowsKubernetesConfigurations: map[string]*ComponentConfiguration{
								string(Componentkubelet): {
									Config: map[string]string{"--address": "127.0.0.1"},
								},
							},
						},
					},
				},
				KubeletConfig: map[string]string{
					"--address":                      "0.0.0.0",
					"--allow-privileged":             "true",
					"--cloud-config":                 "c:\\k\\azure.json",
					"--node-status-report-frequency": "5m0s",
				},
			},
			CustomKubeletConfig: &CustomKubeletConfig{
				ContainerLogMaxSizeMB: to.Int32Ptr(1024),
				ContainerLogMaxFiles:  to.Int32Ptr(20),
			},
			expected: `"--address=127.0.0.1", "--allow-privileged=true", "--cloud-config=c:\k\azure.json", "--container-log-max-files=20", "--container-log-max-size=1024Mi"`,
		},
		{
			name: "custom configuration does not override default KubeletConfig",
			config: &NodeBootstrappingConfiguration{
				ContainerService: &ContainerService{
					Properties: &Properties{
						CustomConfiguration: &CustomConfiguration{
							WindowsKubernetesConfigurations: map[string]*ComponentConfiguration{
								string(Componentkubelet): {
									Config: map[string]string{"--event-qps": "100"},
								},
							},
						},
					},
				},
				KubeletConfig: map[string]string{
					"--address":                      "0.0.0.0",
					"--allow-privileged":             "true",
					"--cloud-config":                 "c:\\k\\azure.json",
					"--node-status-report-frequency": "5m0s",
					"--container-log-max-files":      "10",
					"--container-log-max-size":       "512Mi",
					"--image-gc-high-threshold":      "30",
					"--image-gc-low-threshold":       "10",
				},
			},
			CustomKubeletConfig: &CustomKubeletConfig{
				ImageGcHighThreshold:  to.Int32Ptr(80),
				ImageGcLowThreshold:   to.Int32Ptr(60),
				ContainerLogMaxSizeMB: to.Int32Ptr(1024),
				ContainerLogMaxFiles:  to.Int32Ptr(20),
			},
			expected: `"--address=0.0.0.0", "--allow-privileged=true", "--cloud-config=c:\k\azure.json", "--container-log-max-files=20", "--container-log-max-size=1024Mi", "--event-qps=100", "--image-gc-high-threshold=80", "--image-gc-low-threshold=60"`, //nolint:lll
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			actual := c.config.GetOrderedKubeletConfigStringForPowershell(c.CustomKubeletConfig)
			if c.expected != actual {
				t.Fatalf("test case: %s, expected: %s. Got: %s.", c.name, c.expected, actual)
			}
		})
	}
}

func TestSecurityProfileGetProxyAddress(t *testing.T) {
	testProxyAddress := "https://test-private-egress-proxy"
	cases := []struct {
		name            string
		securityProfile *SecurityProfile
		expected        string
	}{
		{
			name:            "SecurityProfile nil",
			securityProfile: nil,
			expected:        "",
		},
		{
			name:            "PrivateEgress nil",
			securityProfile: &SecurityProfile{},
			expected:        "",
		},
		{
			name:            "PrivateEgress disabled",
			securityProfile: &SecurityProfile{PrivateEgress: &PrivateEgress{Enabled: false}},
			expected:        "",
		},
		{
			name:            "PrivateEgress enabled",
			securityProfile: &SecurityProfile{PrivateEgress: &PrivateEgress{Enabled: true, ProxyAddress: testProxyAddress}},
			expected:        testProxyAddress,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			actual := c.securityProfile.GetProxyAddress()
			if c.expected != actual {
				t.Fatalf("test case: %s, expected: %s. Got: %s.", c.name, c.expected, actual)
			}
		})
	}
}

func TestSecurityProfileGetPrivateEgressContainerRegistryServer(t *testing.T) {
	testContainerRegistryServer := "https://testserver.azurecr.io"
	cases := []struct {
		name            string
		securityProfile *SecurityProfile
		expected        string
	}{
		{
			name:            "SecurityProfile nil",
			securityProfile: nil,
			expected:        "",
		},
		{
			name:            "PrivateEgress nil",
			securityProfile: &SecurityProfile{},
			expected:        "",
		},
		{
			name:            "PrivateEgress disabled",
			securityProfile: &SecurityProfile{PrivateEgress: &PrivateEgress{Enabled: false}},
			expected:        "",
		},
		{
			name:            "PrivateEgress enabled",
			securityProfile: &SecurityProfile{PrivateEgress: &PrivateEgress{Enabled: true, ContainerRegistryServer: testContainerRegistryServer}},
			expected:        testContainerRegistryServer,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			actual := c.securityProfile.GetPrivateEgressContainerRegistryServer()
			if c.expected != actual {
				t.Fatalf("test case: %s, expected: %s. Got: %s.", c.name, c.expected, actual)
			}
		})
	}
}
