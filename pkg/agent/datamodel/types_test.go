// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
)

const (
	// scaleSetPriorityRegular is the default ScaleSet Priority
	ScaleSetPriorityRegular = "Regular"
	// ScaleSetPriorityLow means the ScaleSet will use Low-priority VMs
	ScaleSetPriorityLow = "Low"
	// StorageAccount means that the nodes use raw storage accounts for their os and attached volumes
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
	expectedRet := `{"Name":"AzureStackCloud","mcrURL":"mcr.microsoft.fakecustomcloud","repoDepotEndpoint":"https://repodepot.azure.microsoft.fakecustomcloud/ubuntu","managementPortalURL":"https://portal.azure.microsoft.fakecustomcloud/","serviceManagementEndpoint":"https://management.core.microsoft.fakecustomcloud/","resourceManagerEndpoint":"https://management.azure.microsoft.fakecustomcloud/","activeDirectoryEndpoint":"https://login.microsoftonline.microsoft.fakecustomcloud/","keyVaultEndpoint":"https://vault.cloudapi.microsoft.fakecustomcloud/","graphEndpoint":"https://graph.cloudapi.microsoft.fakecustomcloud/","storageEndpointSuffix":"core.microsoft.fakecustomcloud","sqlDatabaseDNSSuffix":"database.cloudapi.microsoft.fakecustomcloud","keyVaultDNSSuffix":"vault.cloudapi.microsoft.fakecustomcloud","resourceManagerVMDNSSuffix":"cloudapp.azure.microsoft.fakecustomcloud/","containerRegistryDNSSuffix":".azurecr.microsoft.fakecustomcloud","cosmosDBDNSSuffix":"documents.core.microsoft.fakecustomcloud/","tokenAudience":"https://management.core.microsoft.fakecustomcloud/","resourceIdentifiers":{}}`
	actual, err := properities.GetCustomEnvironmentJSON(false)
	fmt.Println(actual)
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
				OSType: Linux,
			},
			{
				OSType: Linux,
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

	if !p.AgentPoolProfiles[0].IsLinux() {
		t.Fatalf("expected IsLinux() to return true but instead returned false")
	}

	p.AgentPoolProfiles[0].OSType = Windows

	if !p.HasWindows() {
		t.Fatalf("expected HasWindows() to return true but instead returned false")
	}

	if !p.AgentPoolProfiles[0].IsWindows() {
		t.Fatalf("expected IsWindows() to return true but instead returned false")
	}

	if p.AgentPoolProfiles[0].IsLinux() {
		t.Fatalf("expected IsLinux() to return false but instead returned true")
	}
}

func TestTotalNodes(t *testing.T) {
	cases := []struct {
		name     string
		p        Properties
		expected int
	}{
		{
			name: "7 total nodes between 2 pools",
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count: 3,
					},
					{
						Count: 4,
					},
				},
			},
			expected: 7,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.p.TotalNodes() != c.expected {
				t.Fatalf("expected TotalNodes() to return %d but instead returned %d", c.expected, c.p.TotalNodes())
			}
		})
	}
}

func TestHasAvailabilityZones(t *testing.T) {
	cases := []struct {
		p                Properties
		expectedAgent    bool
		expectedAllZones bool
	}{
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:             1,
						AvailabilityZones: []string{"1", "2"},
					},
					{
						Count:             1,
						AvailabilityZones: []string{"1", "2"},
					},
				},
			},
			expectedAgent:    true,
			expectedAllZones: true,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count: 1,
					},
					{
						Count:             1,
						AvailabilityZones: []string{"1", "2"},
					},
				},
			},
			expectedAgent:    false,
			expectedAllZones: false,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:             1,
						AvailabilityZones: []string{},
					},
					{
						Count:             1,
						AvailabilityZones: []string{"1", "2"},
					},
				},
			},
			expectedAgent:    false,
			expectedAllZones: false,
		},
	}

	for _, c := range cases {
		if c.p.AgentPoolProfiles[0].HasAvailabilityZones() != c.expectedAgent {
			t.Fatalf("expected HasAvailabilityZones() to return %t but instead returned %t", c.expectedAgent, c.p.AgentPoolProfiles[0].HasAvailabilityZones())
		}
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
			t.Fatalf("expected Properties.IsIPMasqAgentEnabled() to return %t but instead returned %t", c.expectedPropertiesIsIPMasqAgentEnabled, c.p.IsIPMasqAgentEnabled())
		}
		if c.p.OrchestratorProfile.KubernetesConfig.IsIPMasqAgentEnabled() != c.expectedKubernetesConfigIsIPMasqAgentEnabled {
			t.Fatalf("expected KubernetesConfig.IsIPMasqAgentEnabled() to return %t but instead returned %t", c.expectedKubernetesConfigIsIPMasqAgentEnabled, c.p.OrchestratorProfile.KubernetesConfig.IsIPMasqAgentEnabled())
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

func TestAnyAgentIsLinux(t *testing.T) {
	tests := []struct {
		name     string
		p        *Properties
		expected bool
	}{
		{
			name: "one agent pool w/ Linux",
			p: &Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  2,
						OSType: Linux,
					},
				},
			},
			expected: true,
		},
		{
			name: "two agent pools, one w/ Linux",
			p: &Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  2,
						OSType: Windows,
					},
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						OSType: Linux,
					},
				},
			},
			expected: true,
		},
		{
			name: "two agent pools",
			p: &Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  2,
					},
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  100,
					},
				},
			},
			expected: false,
		},
		{
			name: "two agent pools, one w/ Windows",
			p: &Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  2,
					},
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  100,
						OSType: Windows,
					},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ret := test.p.AnyAgentIsLinux()
			if test.expected != ret {
				t.Errorf("expected %t, instead got : %t", test.expected, ret)
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
					Count:  1,
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
						Count:  1,
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
						Count:  1,
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
						ScaleSetPriority:    ScaleSetPrioritySpot,
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
						ScaleSetPriority:    ScaleSetPriorityLow,
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
						ScaleSetPriority:    ScaleSetPriorityRegular,
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
			t.Fatalf("expected IsVirtualMachineScaleSets() to return %t but instead returned %t", c.expectedISVMSS, c.p.AgentPoolProfiles[0].IsVirtualMachineScaleSets())
		}
		if c.p.AgentPoolProfiles[0].IsAvailabilitySets() != c.expectedIsAS {
			t.Fatalf("expected IsAvailabilitySets() to return %t but instead returned %t", c.expectedIsAS, c.p.AgentPoolProfiles[0].IsAvailabilitySets())
		}
		if c.p.AgentPoolProfiles[0].IsSpotScaleSet() != c.expectedSpot {
			t.Fatalf("expected IsSpotScaleSet() to return %t but instead returned %t", c.expectedSpot, c.p.AgentPoolProfiles[0].IsSpotScaleSet())
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
						Count:               1,
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
						Count:               1,
						AvailabilityProfile: VirtualMachineScaleSets,
						VnetSubnetID:        "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/BazAgentSubnet",
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
				Count:               1,
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
						Count:               1,
						AvailabilityProfile: VirtualMachineScaleSets,
						VnetSubnetID:        "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/BazAgentSubnet",
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
						Count:               1,
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
				Count:               1,
				AvailabilityProfile: VirtualMachineScaleSets,
				VnetSubnetID:        "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/BazAgentSubnet",
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
				Count:               1,
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
			Count:               1,
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
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count: 1,
					},
					{
						Count: 1,
					},
				},
			},
			expectedAgent: false,
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
		expected      string
	}{
		{
			name:          "vanilla pool profile",
			ap:            AgentPoolProfile{},
			rg:            "my-resource-group",
			deprecated:    true,
			nvidiaEnabled: false,
			expected:      "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,kubernetes.azure.com/cluster=my-resource-group",
		},
		{
			name:          "vanilla pool profile, no deprecated labels",
			ap:            AgentPoolProfile{},
			rg:            "my-resource-group",
			deprecated:    false,
			nvidiaEnabled: false,
			expected:      "kubernetes.azure.com/role=agent,agentpool=,kubernetes.azure.com/cluster=my-resource-group",
		},
		{
			name: "with managed disk",
			ap: AgentPoolProfile{
				StorageProfile: ManagedDisks,
			},
			rg:            "my-resource-group",
			deprecated:    true,
			nvidiaEnabled: false,
			expected:      "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,storageprofile=managed,storagetier=,kubernetes.azure.com/cluster=my-resource-group",
		},
		{
			name: "N series",
			ap: AgentPoolProfile{
				VMSize: "Standard_NC6",
			},
			rg:            "my-resource-group",
			deprecated:    true,
			nvidiaEnabled: true,
			expected:      "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,accelerator=nvidia,kubernetes.azure.com/cluster=my-resource-group",
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
			expected:      "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,kubernetes.azure.com/cluster=my-resource-group,mycustomlabel1=foo,mycustomlabel2=bar",
		},
		{
			name: "with custom labels, no deprecated labels",
			ap: AgentPoolProfile{
				CustomNodeLabels: map[string]string{
					"mycustomlabel1": "foo",
					"mycustomlabel2": "bar",
				},
			},
			rg:            "my-resource-group",
			deprecated:    false,
			nvidiaEnabled: false,
			expected:      "kubernetes.azure.com/role=agent,agentpool=,kubernetes.azure.com/cluster=my-resource-group,mycustomlabel1=foo,mycustomlabel2=bar",
		},
		{
			name: "N series and managed disk with custom labels",
			ap: AgentPoolProfile{
				StorageProfile: ManagedDisks,
				VMSize:         "Standard_NC6",
				CustomNodeLabels: map[string]string{
					"mycustomlabel1": "foo",
					"mycustomlabel2": "bar",
				},
			},
			rg:            "my-resource-group",
			deprecated:    true,
			nvidiaEnabled: true,
			expected:      "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,storageprofile=managed,storagetier=Standard_LRS,accelerator=nvidia,kubernetes.azure.com/cluster=my-resource-group,mycustomlabel1=foo,mycustomlabel2=bar",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.ap.GetKubernetesLabels(c.rg, c.deprecated, c.nvidiaEnabled) {
				t.Fatalf("Got unexpected AgentPoolProfile.GetKubernetesLabels(%s, %t) result. Expected: %s. Got: %s.",
					c.rg, c.deprecated, c.expected, c.ap.GetKubernetesLabels(c.rg, c.deprecated, c.nvidiaEnabled))
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
						DiskSizesGB:    []int{5},
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
						StorageProfile:      ManagedDisks,
						DiskEncryptionSetID: "DiskEncryptionSetID",
					},
					{
						StorageProfile:      ManagedDisks,
						DiskEncryptionSetID: "DiskEncryptionSetID",
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
						StorageProfile:   ManagedDisks,
						EncryptionAtHost: to.BoolPtr(true),
					},
					{
						StorageProfile:   ManagedDisks,
						EncryptionAtHost: to.BoolPtr(true),
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
				t.Fatalf("expected PrivateJumpboxProvision() to return %t but instead returned %t", c.expectedPrivateJB, c.p.OrchestratorProfile.KubernetesConfig.PrivateJumpboxProvision())
			}
			if c.p.AgentPoolProfiles[0].HasDisks() != c.expectedHasDisks {
				t.Fatalf("expected HasDisks() to return %t but instead returned %t", c.expectedHasDisks, c.p.AgentPoolProfiles[0].HasDisks())
			}
			if c.p.AgentPoolProfiles[0].DiskEncryptionSetID != c.expectedDesID {
				t.Fatalf("expected DiskEncryptionSetID to return %s but instead returned %s", c.expectedDesID, c.p.AgentPoolProfiles[0].DiskEncryptionSetID)
			}
			if to.Bool(c.p.AgentPoolProfiles[0].EncryptionAtHost) != c.expectedEncryptionAtHost {
				t.Fatalf("expected EncryptionAtHost to return %v but instead returned %v", c.expectedEncryptionAtHost, to.Bool(c.p.AgentPoolProfiles[0].EncryptionAtHost))
			}
		})
	}
}

func TestLinuxProfile(t *testing.T) {
	l := LinuxProfile{}

	if l.HasSecrets() || l.HasSearchDomain() || l.HasCustomNodesDNS() {
		t.Fatalf("Expected HasSecrets(), HasSearchDomain() and HasCustomNodesDNS() to return false when LinuxProfile is empty")
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
		CustomNodesDNS: &CustomNodesDNS{
			DNSServer: "testDNSServer",
		},
		CustomSearchDomain: &CustomSearchDomain{
			Name:          "testName",
			RealmPassword: "testRealmPassword",
			RealmUser:     "testRealmUser",
		},
	}

	if !(l.HasSecrets() && l.HasSearchDomain() && l.HasCustomNodesDNS()) {
		t.Fatalf("Expected HasSecrets(), HasSearchDomain() and HasCustomNodesDNS() to return true")
	}
}

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

func TestIsPrivateCluster(t *testing.T) {
	cases := []struct {
		p        Properties
		expected bool
	}{
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
				},
			},
			expected: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						PrivateCluster: &PrivateCluster{
							Enabled: to.BoolPtr(true),
						},
					},
				},
			},
			expected: true,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						PrivateCluster: &PrivateCluster{
							Enabled: to.BoolPtr(false),
						},
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: Kubernetes,
					KubernetesConfig: &KubernetesConfig{
						PrivateCluster: &PrivateCluster{},
					},
				},
			},
			expected: false,
		},
	}

	for _, c := range cases {
		if c.p.OrchestratorProfile.IsPrivateCluster() != c.expected {
			t.Fatalf("expected IsPrivateCluster() to return %t but instead got %t", c.expected, c.p.OrchestratorProfile.IsPrivateCluster())
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
			name:     "telemetry",
			feature:  "EnableTelemetry",
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
				FeatureFlags: &FeatureFlags{
					EnableTelemetry: true,
				},
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
			t.Fatalf("expected KubernetesConfig.IsAddonEnabled(%s) to return %t but instead returned %t", c.addonName, c.expected, c.k.IsAddonEnabled(c.addonName))
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
				t.Fatalf("expected KubernetesConfig.IsIPMasqAgentDisabled() to return %t but instead returned %t", c.expectedDisabled, c.k.IsIPMasqAgentDisabled())
			}
		})
	}
}

func TestGetAddonByName(t *testing.T) {
	ContainerMonitoringAddonName := "container-monitoring"

	// Addon present and enabled with logAnalyticsWorkspaceResourceId in config
	b := true
	c := KubernetesConfig{
		Addons: []KubernetesAddon{
			{
				Name:    ContainerMonitoringAddonName,
				Enabled: &b,
				Config: map[string]string{
					"logAnalyticsWorkspaceResourceId": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test-workspace-rg/providers/Microsoft.OperationalInsights/workspaces/test-workspace",
				},
			},
		},
	}

	addon := c.GetAddonByName(ContainerMonitoringAddonName)
	if addon.Config == nil || len(addon.Config) == 0 {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config instead returned null or empty")
	}

	if addon.Config["logAnalyticsWorkspaceResourceId"] == "" {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config with logAnalyticsWorkspaceResourceId, instead returned null or empty")
	}

	workspaceResourceID := addon.Config["logAnalyticsWorkspaceResourceId"]
	if workspaceResourceID == "" {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config with non empty azure logAnalyticsWorkspaceResourceId")
	}

	resourceParts := strings.Split(workspaceResourceID, "/")
	if len(resourceParts) != 9 {
		t.Fatalf("KubernetesConfig.IsContainerMonitoringAddonEnabled() should have addon config with valid Azure logAnalyticsWorkspaceResourceId, instead returned %s", workspaceResourceID)
	}

	// Addon present and enabled with legacy config
	b = true
	c = KubernetesConfig{
		Addons: []KubernetesAddon{
			{
				Name:    ContainerMonitoringAddonName,
				Enabled: &b,
				Config: map[string]string{
					"workspaceGuid": "MDAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAw",
					"workspaceKey":  "NEQrdnlkNS9qU2NCbXNBd1pPRi8wR09CUTVrdUZRYzlKVmFXK0hsbko1OGN5ZVBKY3dUcGtzK3JWbXZnY1hHbW15dWpMRE5FVlBpVDhwQjI3NGE5WWc9PQ==",
				},
			},
		},
	}

	addon = c.GetAddonByName(ContainerMonitoringAddonName)
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
			t.Fatalf("expected KubernetesConfig.IsAddonDisabled(%s) to return %t but instead returned %t", c.addonName, c.expected, c.k.IsAddonDisabled(c.addonName))
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
	alphabetizedString := "--address=0.0.0.0 --allow-privileged=true --anonymous-auth=false --authorization-mode=Webhook --cgroups-per-qos=true --client-ca-file=/etc/kubernetes/certs/ca.crt --keep-terminated-pod-volumes=false --kubeconfig=/var/lib/kubelet/kubeconfig --pod-manifest-path=/etc/kubernetes/manifests "
	alphabetizedStringForPowershell := `"--address=0.0.0.0", "--allow-privileged=true", "--anonymous-auth=false", "--authorization-mode=Webhook", "--cgroups-per-qos=true", "--client-ca-file=/etc/kubernetes/certs/ca.crt", "--keep-terminated-pod-volumes=false", "--kubeconfig=/var/lib/kubelet/kubeconfig", "--pod-manifest-path=/etc/kubernetes/manifests"`
	cases := []struct {
		name                  string
		kc                    KubernetesConfig
		expected              string
		expectedForPowershell string
	}{
		{
			name:                  "zero value kubernetesConfig",
			kc:                    KubernetesConfig{},
			expected:              "",
			expectedForPowershell: "",
		},
		// Some values
		{
			name: "expected values",
			kc: KubernetesConfig{
				KubeletConfig: map[string]string{
					"--address":                     "0.0.0.0",
					"--allow-privileged":            "true",
					"--anonymous-auth":              "false",
					"--authorization-mode":          "Webhook",
					"--client-ca-file":              "/etc/kubernetes/certs/ca.crt",
					"--pod-manifest-path":           "/etc/kubernetes/manifests",
					"--cgroups-per-qos":             "true",
					"--kubeconfig":                  "/var/lib/kubelet/kubeconfig",
					"--keep-terminated-pod-volumes": "false",
				},
			},
			expected:              alphabetizedString,
			expectedForPowershell: alphabetizedStringForPowershell,
		},
		// Switch the "order" in the map, validate the same return string
		{
			name: "expected values re-ordered",
			kc: KubernetesConfig{
				KubeletConfig: map[string]string{
					"--address":                     "0.0.0.0",
					"--allow-privileged":            "true",
					"--kubeconfig":                  "/var/lib/kubelet/kubeconfig",
					"--client-ca-file":              "/etc/kubernetes/certs/ca.crt",
					"--authorization-mode":          "Webhook",
					"--pod-manifest-path":           "/etc/kubernetes/manifests",
					"--cgroups-per-qos":             "true",
					"--keep-terminated-pod-volumes": "false",
					"--anonymous-auth":              "false",
				},
			},
			expected:              alphabetizedString,
			expectedForPowershell: alphabetizedStringForPowershell,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expectedForPowershell != c.kc.GetOrderedKubeletConfigStringForPowershell() {
				t.Fatalf("Got unexpected AgentPoolProfile.GetOrderedKubeletConfigStringForPowershell() result. Expected: %s. Got: %s.", c.expectedForPowershell, c.kc.GetOrderedKubeletConfigStringForPowershell())
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
