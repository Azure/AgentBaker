// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/go-autorest/autorest/to"
)

func TestGetAzureCNIURLFuncs(t *testing.T) {
	// Default case
	cs := CreateMockContainerService("testcluster", defaultTestClusterVer, 1, 3, false)
	cs.Location = "eastus"
	cloudSpecConfig := cs.GetCloudSpecConfig()

	o := OrchestratorProfile{
		OrchestratorType: "Kubernetes",
		KubernetesConfig: &api.KubernetesConfig{},
	}
	linuxURL := o.KubernetesConfig.GetAzureCNIURLLinux(cloudSpecConfig)
	windowsURL := o.KubernetesConfig.GetAzureCNIURLWindows(cloudSpecConfig)
	if linuxURL != cloudSpecConfig.KubernetesSpecConfig.VnetCNILinuxPluginsDownloadURL {
		t.Fatalf("GetAzureCNIURLLinux() should return default %s, instead returned %s", cloudSpecConfig.KubernetesSpecConfig.VnetCNILinuxPluginsDownloadURL, linuxURL)
	}
	if windowsURL != cloudSpecConfig.KubernetesSpecConfig.VnetCNIWindowsPluginsDownloadURL {
		t.Fatalf("GetAzureCNIURLWindows() should return default %s, instead returned %s", cloudSpecConfig.KubernetesSpecConfig.VnetCNIWindowsPluginsDownloadURL, windowsURL)
	}

	// User-configurable case
	cs = CreateMockContainerService("testcluster", defaultTestClusterVer, 1, 3, false)
	cs.Location = "eastus"
	cloudSpecConfig = cs.GetCloudSpecConfig()

	customLinuxURL := "https://custom-url/azure-cni-linux.0.0.1.tgz"
	customWindowsURL := "https://custom-url/azure-cni-windows.0.0.1.tgz"
	o = OrchestratorProfile{
		OrchestratorType: "Kubernetes",
		KubernetesConfig: &api.KubernetesConfig{
			AzureCNIURLLinux:   customLinuxURL,
			AzureCNIURLWindows: customWindowsURL,
		},
	}

	linuxURL = o.KubernetesConfig.GetAzureCNIURLLinux(cloudSpecConfig)
	windowsURL = o.KubernetesConfig.GetAzureCNIURLWindows(cloudSpecConfig)
	if linuxURL != customLinuxURL {
		t.Fatalf("GetAzureCNIURLLinux() should return custom URL %s, instead returned %s", customLinuxURL, linuxURL)
	}
	if windowsURL != customWindowsURL {
		t.Fatalf("GetAzureCNIURLWindows() should return custom URL %s, instead returned %s", customWindowsURL, windowsURL)
	}
}

func TestGetLocations(t *testing.T) {

	// Test locations for Azure
	mockCSDefault := getMockBaseContainerService("1.11.6")
	mockCSDefault.Location = "eastus"

	expected := []string{
		"australiacentral",
		"australiacentral2",
		"australiaeast",
		"australiasoutheast",
		"brazilsouth",
		"canadacentral",
		"canadaeast",
		"centralindia",
		"centralus",
		"centraluseuap",
		"chinaeast",
		"chinaeast2",
		"chinanorth",
		"chinanorth2",
		"eastasia",
		"eastus",
		"eastus2",
		"eastus2euap",
		"francecentral",
		"francesouth",
		"germanynorth",
		"germanywestcentral",
		"japaneast",
		"japanwest",
		"koreacentral",
		"koreasouth",
		"northcentralus",
		"northeurope",
		"norwayeast",
		"norwaywest",
		"southafricanorth",
		"southafricawest",
		"southcentralus",
		"southeastasia",
		"southindia",
		"switzerlandnorth",
		"switzerlandwest",
		"uaecentral",
		"uaenorth",
		"uksouth",
		"ukwest",
		"usdodcentral",
		"usdodeast",
		"westcentralus",
		"westeurope",
		"westindia",
		"westus",
		"westus2",
		"chinaeast",
		"chinanorth",
		"chinanorth2",
		"chinaeast2",
		"germanycentral",
		"germanynortheast",
		"usgovvirginia",
		"usgoviowa",
		"usgovarizona",
		"usgovtexas",
		"francecentral",
	}
	actual := mockCSDefault.GetLocations()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Test TestGetLocations() : expected to return %s, but got %s . ", expected, actual)
	}
}

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
					KubernetesConfig: &api.KubernetesConfig{},
				},
			},
			expectedDisabled: false,
		},
		{
			name: "addons configured but no ip-masq-agent configuration",
			p: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							{
								Name:    common.CoreDNSAddonName,
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
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							{
								Name:    common.IPMASQAgentAddonName,
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
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							{
								Name: common.IPMASQAgentAddonName,
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
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							{
								Name:    common.IPMASQAgentAddonName,
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

func TestPropertiesIsHostedMasterProfile(t *testing.T) {
	cases := []struct {
		name     string
		p        Properties
		expected bool
	}{
		{
			name: "valid master 1 node",
			p: Properties{
				MasterProfile: &MasterProfile{
					Count: 1,
				},
			},
			expected: false,
		},
		{
			name: "valid master 3 nodes",
			p: Properties{
				MasterProfile: &MasterProfile{
					Count: 3,
				},
			},
			expected: false,
		},
		{
			name: "valid master 5 nodes",
			p: Properties{
				MasterProfile: &MasterProfile{
					Count: 5,
				},
			},
			expected: false,
		},
		{
			name: "zero value hosted master",
			p: Properties{
				HostedMasterProfile: &HostedMasterProfile{},
			},
			expected: true,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.p.IsHostedMasterProfile() != c.expected {
				t.Fatalf("expected IsHostedMasterProfile() to return %t but instead returned %t", c.expected, c.p.IsHostedMasterProfile())
			}
		})
	}
}

func TestOSType(t *testing.T) {
	p := Properties{
		MasterProfile: &MasterProfile{
			Distro: AKSUbuntu1604,
		},
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
	if p.HasCoreOS() {
		t.Fatalf("expected HasCoreOS() to return false but instead returned true")
	}
	if p.AgentPoolProfiles[0].IsWindows() {
		t.Fatalf("expected IsWindows() to return false but instead returned true")
	}

	if !p.AgentPoolProfiles[0].IsLinux() {
		t.Fatalf("expected IsLinux() to return true but instead returned false")
	}

	if p.AgentPoolProfiles[0].IsCoreOS() {
		t.Fatalf("expected IsCoreOS() to return false but instead returned true")
	}

	if p.AgentPoolProfiles[1].IsCoreOS() {
		t.Fatalf("expected IsCoreOS() to return false but instead returned true")
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

	if p.AgentPoolProfiles[0].IsCoreOS() {
		t.Fatalf("expected IsCoreOS() to return false but instead returned true")
	}
}

func TestCloudProviderDefaults(t *testing.T) {
	// Test cloudprovider defaults when no user-provided values
	v := "1.8.0"
	p := Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
	}
	o := p.OrchestratorProfile
	o.KubernetesConfig.SetCloudProviderBackoffDefaults()
	p.SetCloudProviderRateLimitDefaults()

	intCases := []struct {
		defaultVal  int
		computedVal int
	}{
		{
			defaultVal:  api.DefaultKubernetesCloudProviderBackoffRetries,
			computedVal: o.KubernetesConfig.CloudProviderBackoffRetries,
		},
		{
			defaultVal:  api.DefaultKubernetesCloudProviderBackoffDuration,
			computedVal: o.KubernetesConfig.CloudProviderBackoffDuration,
		},
		{
			defaultVal:  api.DefaultKubernetesCloudProviderRateLimitBucket,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucket,
		},
		{
			defaultVal:  api.DefaultKubernetesCloudProviderRateLimitBucketWrite,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucketWrite,
		},
	}

	for _, c := range intCases {
		if c.computedVal != c.defaultVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %d, got %d", c.defaultVal, c.computedVal)
		}
	}

	floatCases := []struct {
		defaultVal  float64
		computedVal float64
	}{
		{
			defaultVal:  api.DefaultKubernetesCloudProviderBackoffJitter,
			computedVal: o.KubernetesConfig.CloudProviderBackoffJitter,
		},
		{
			defaultVal:  api.DefaultKubernetesCloudProviderBackoffExponent,
			computedVal: o.KubernetesConfig.CloudProviderBackoffExponent,
		},
		{
			defaultVal:  api.DefaultKubernetesCloudProviderRateLimitQPS,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPS,
		},
		{
			defaultVal:  api.DefaultKubernetesCloudProviderRateLimitQPSWrite,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPSWrite,
		},
	}

	for _, c := range floatCases {
		if c.computedVal != c.defaultVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %f, got %f", c.defaultVal, c.computedVal)
		}
	}

	customCloudProviderBackoffDuration := 99
	customCloudProviderBackoffExponent := 10.0
	customCloudProviderBackoffJitter := 11.9
	customCloudProviderBackoffRetries := 9
	customCloudProviderRateLimitBucket := 37
	customCloudProviderRateLimitQPS := 9.9
	customCloudProviderRateLimitQPSWrite := 100.1
	customCloudProviderRateLimitBucketWrite := 42

	// Test cloudprovider defaults when user provides configuration
	v = "1.8.0"
	p = Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig: &api.KubernetesConfig{
				CloudProviderBackoffDuration:      customCloudProviderBackoffDuration,
				CloudProviderBackoffExponent:      customCloudProviderBackoffExponent,
				CloudProviderBackoffJitter:        customCloudProviderBackoffJitter,
				CloudProviderBackoffRetries:       customCloudProviderBackoffRetries,
				CloudProviderRateLimitBucket:      customCloudProviderRateLimitBucket,
				CloudProviderRateLimitQPS:         customCloudProviderRateLimitQPS,
				CloudProviderRateLimitQPSWrite:    customCloudProviderRateLimitQPSWrite,
				CloudProviderRateLimitBucketWrite: customCloudProviderRateLimitBucketWrite,
			},
		},
	}
	o = p.OrchestratorProfile
	o.KubernetesConfig.SetCloudProviderBackoffDefaults()
	p.SetCloudProviderRateLimitDefaults()

	intCasesCustom := []struct {
		customVal   int
		computedVal int
	}{
		{
			customVal:   customCloudProviderBackoffRetries,
			computedVal: o.KubernetesConfig.CloudProviderBackoffRetries,
		},
		{
			customVal:   customCloudProviderBackoffDuration,
			computedVal: o.KubernetesConfig.CloudProviderBackoffDuration,
		},
		{
			customVal:   customCloudProviderRateLimitBucket,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucket,
		},
		{
			customVal:   customCloudProviderRateLimitBucketWrite,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucketWrite,
		},
	}

	for _, c := range intCasesCustom {
		if c.computedVal != c.customVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %d, got %d", c.customVal, c.computedVal)
		}
	}

	floatCasesCustom := []struct {
		customVal   float64
		computedVal float64
	}{
		{
			customVal:   customCloudProviderBackoffJitter,
			computedVal: o.KubernetesConfig.CloudProviderBackoffJitter,
		},
		{
			customVal:   customCloudProviderBackoffExponent,
			computedVal: o.KubernetesConfig.CloudProviderBackoffExponent,
		},
		{
			customVal:   customCloudProviderRateLimitQPS,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPS,
		},
		{
			customVal:   customCloudProviderRateLimitQPSWrite,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPSWrite,
		},
	}

	for _, c := range floatCasesCustom {
		if c.computedVal != c.customVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %f, got %f", c.customVal, c.computedVal)
		}
	}

	// Test cloudprovider defaults when user provides *some* config values
	v = "1.8.0"
	p = Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig: &api.KubernetesConfig{
				CloudProviderBackoffDuration: customCloudProviderBackoffDuration,
				CloudProviderRateLimitBucket: customCloudProviderRateLimitBucket,
				CloudProviderRateLimitQPS:    customCloudProviderRateLimitQPS,
			},
		},
	}
	o = p.OrchestratorProfile
	o.KubernetesConfig.SetCloudProviderBackoffDefaults()
	p.SetCloudProviderRateLimitDefaults()

	intCasesMixed := []struct {
		expectedVal int
		computedVal int
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffRetries,
			computedVal: o.KubernetesConfig.CloudProviderBackoffRetries,
		},
		{
			expectedVal: customCloudProviderBackoffDuration,
			computedVal: o.KubernetesConfig.CloudProviderBackoffDuration,
		},
		{
			expectedVal: customCloudProviderRateLimitBucket,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucket,
		},
	}

	for _, c := range intCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %d, got %d", c.expectedVal, c.computedVal)
		}
	}

	floatCasesMixed := []struct {
		expectedVal float64
		computedVal float64
	}{
		{
			expectedVal: customCloudProviderRateLimitQPS,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPS,
		},
	}

	for _, c := range floatCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %f, got %f", c.expectedVal, c.computedVal)
		}
	}

	// Test cloudprovider defaults for VMSS scenario
	v = "1.14.0"
	p = Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				AvailabilityProfile: api.VirtualMachineScaleSets,
			},
		},
	}
	o = p.OrchestratorProfile
	o.KubernetesConfig.SetCloudProviderBackoffDefaults()
	p.SetCloudProviderRateLimitDefaults()

	intCasesMixed = []struct {
		expectedVal int
		computedVal int
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffRetries,
			computedVal: o.KubernetesConfig.CloudProviderBackoffRetries,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffDuration,
			computedVal: o.KubernetesConfig.CloudProviderBackoffDuration,
		},
		{
			expectedVal: common.MaxAgentCount,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucket,
		},
	}

	for _, c := range intCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %d, got %d", c.expectedVal, c.computedVal)
		}
	}

	floatCasesMixed = []struct {
		expectedVal float64
		computedVal float64
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffJitter,
			computedVal: o.KubernetesConfig.CloudProviderBackoffJitter,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffExponent,
			computedVal: o.KubernetesConfig.CloudProviderBackoffExponent,
		},
		{
			expectedVal: float64(common.MaxAgentCount) * common.MinCloudProviderQPSToBucketFactor,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPS,
		},
	}

	for _, c := range floatCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %f, got %f", c.expectedVal, c.computedVal)
		}
	}

	// Test cloudprovider defaults for VMSS scenario with 3 pools
	v = "1.14.0"
	p = Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				AvailabilityProfile: api.VirtualMachineScaleSets,
			},
			{
				AvailabilityProfile: api.VirtualMachineScaleSets,
			},
			{
				AvailabilityProfile: api.VirtualMachineScaleSets,
			},
		},
	}
	o = p.OrchestratorProfile
	o.KubernetesConfig.SetCloudProviderBackoffDefaults()
	p.SetCloudProviderRateLimitDefaults()

	intCasesMixed = []struct {
		expectedVal int
		computedVal int
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffRetries,
			computedVal: o.KubernetesConfig.CloudProviderBackoffRetries,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffDuration,
			computedVal: o.KubernetesConfig.CloudProviderBackoffDuration,
		},
		{
			expectedVal: common.MaxAgentCount * 3,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucket,
		},
	}

	for _, c := range intCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %d, got %d", c.expectedVal, c.computedVal)
		}
	}

	floatCasesMixed = []struct {
		expectedVal float64
		computedVal float64
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffJitter,
			computedVal: o.KubernetesConfig.CloudProviderBackoffJitter,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffExponent,
			computedVal: o.KubernetesConfig.CloudProviderBackoffExponent,
		},
		{
			expectedVal: float64(common.MaxAgentCount*3) * common.MinCloudProviderQPSToBucketFactor,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPS,
		},
	}

	for _, c := range floatCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %f, got %f", c.expectedVal, c.computedVal)
		}
	}

	// Test cloudprovider defaults for VMSS scenario + AKS
	v = "1.14.0"
	p = Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				AvailabilityProfile: api.VirtualMachineScaleSets,
			},
		},
		HostedMasterProfile: &HostedMasterProfile{
			FQDN: "my-cluster",
		},
	}
	o = p.OrchestratorProfile
	o.KubernetesConfig.SetCloudProviderBackoffDefaults()
	p.SetCloudProviderRateLimitDefaults()

	intCasesMixed = []struct {
		expectedVal int
		computedVal int
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffRetries,
			computedVal: o.KubernetesConfig.CloudProviderBackoffRetries,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffDuration,
			computedVal: o.KubernetesConfig.CloudProviderBackoffDuration,
		},
		{
			expectedVal: common.MaxAgentCount,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucket,
		},
	}

	for _, c := range intCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %d, got %d", c.expectedVal, c.computedVal)
		}
	}

	floatCasesMixed = []struct {
		expectedVal float64
		computedVal float64
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffJitter,
			computedVal: o.KubernetesConfig.CloudProviderBackoffJitter,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffExponent,
			computedVal: o.KubernetesConfig.CloudProviderBackoffExponent,
		},
		{
			expectedVal: float64(common.MaxAgentCount) * common.MinCloudProviderQPSToBucketFactor,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPS,
		},
	}

	for _, c := range floatCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %f, got %f", c.expectedVal, c.computedVal)
		}
	}

	// Test cloudprovider defaults for VMAS scenario
	v = "1.14.0"
	p = Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				AvailabilityProfile: api.AvailabilitySet,
			},
		},
	}
	o = p.OrchestratorProfile
	o.KubernetesConfig.SetCloudProviderBackoffDefaults()
	p.SetCloudProviderRateLimitDefaults()

	intCasesMixed = []struct {
		expectedVal int
		computedVal int
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffRetries,
			computedVal: o.KubernetesConfig.CloudProviderBackoffRetries,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffDuration,
			computedVal: o.KubernetesConfig.CloudProviderBackoffDuration,
		},
		{
			expectedVal: common.MaxAgentCount,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucket,
		},
	}

	for _, c := range intCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %d, got %d", c.expectedVal, c.computedVal)
		}
	}

	floatCasesMixed = []struct {
		expectedVal float64
		computedVal float64
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffJitter,
			computedVal: o.KubernetesConfig.CloudProviderBackoffJitter,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffExponent,
			computedVal: o.KubernetesConfig.CloudProviderBackoffExponent,
		},
		{
			expectedVal: float64(common.MaxAgentCount) * common.MinCloudProviderQPSToBucketFactor,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPS,
		},
	}

	for _, c := range floatCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %f, got %f", c.expectedVal, c.computedVal)
		}
	}

	// Test cloudprovider defaults for VMAS + VMSS scenario
	v = "1.14.0"
	p = Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				AvailabilityProfile: api.AvailabilitySet,
			},
			{
				AvailabilityProfile: api.VirtualMachineScaleSets,
			},
		},
	}
	o = p.OrchestratorProfile
	o.KubernetesConfig.SetCloudProviderBackoffDefaults()
	p.SetCloudProviderRateLimitDefaults()

	intCasesMixed = []struct {
		expectedVal int
		computedVal int
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffRetries,
			computedVal: o.KubernetesConfig.CloudProviderBackoffRetries,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffDuration,
			computedVal: o.KubernetesConfig.CloudProviderBackoffDuration,
		},
		{
			expectedVal: 2 * common.MaxAgentCount,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitBucket,
		},
	}

	for _, c := range intCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %d, got %d", c.expectedVal, c.computedVal)
		}
	}

	floatCasesMixed = []struct {
		expectedVal float64
		computedVal float64
	}{
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffJitter,
			computedVal: o.KubernetesConfig.CloudProviderBackoffJitter,
		},
		{
			expectedVal: api.DefaultKubernetesCloudProviderBackoffExponent,
			computedVal: o.KubernetesConfig.CloudProviderBackoffExponent,
		},
		{
			expectedVal: float64(common.MaxAgentCount*2) * common.MinCloudProviderQPSToBucketFactor,
			computedVal: o.KubernetesConfig.CloudProviderRateLimitQPS,
		},
	}

	for _, c := range floatCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig empty cloudprovider configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %f, got %f", c.expectedVal, c.computedVal)
		}
	}

	// Test cloudprovider defaults for backoff mode v2
	v = "1.14.0"
	p = Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig: &api.KubernetesConfig{
				CloudProviderBackoffMode: api.CloudProviderBackoffModeV2,
			},
		},
	}
	o = p.OrchestratorProfile
	o.KubernetesConfig.SetCloudProviderBackoffDefaults()

	floatCasesMixed = []struct {
		expectedVal float64
		computedVal float64
	}{
		{
			expectedVal: 0,
			computedVal: o.KubernetesConfig.CloudProviderBackoffJitter,
		},
		{
			expectedVal: 0,
			computedVal: o.KubernetesConfig.CloudProviderBackoffExponent,
		},
	}

	for _, c := range floatCasesMixed {
		if c.computedVal != c.expectedVal {
			t.Fatalf("KubernetesConfig cloudprovider backoff v2 configs should reflect default values after SetCloudProviderBackoffDefaults(), expected %f, got %f", c.expectedVal, c.computedVal)
		}
	}
}

func TestTotalNodes(t *testing.T) {
	cases := []struct {
		name     string
		p        Properties
		expected int
	}{
		{
			name: "2 total nodes between master and pool",
			p: Properties{
				MasterProfile: &MasterProfile{
					Count: 1,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count: 1,
					},
				},
			},
			expected: 2,
		},
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
		{
			name: "11 total nodes between master and pool",
			p: Properties{
				MasterProfile: &MasterProfile{
					Count: 5,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count: 6,
					},
				},
			},
			expected: 11,
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
		expectedMaster   bool
		expectedAgent    bool
		expectedAllZones bool
	}{
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:             1,
					AvailabilityZones: []string{"1", "2"},
				},
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
			expectedMaster:   true,
			expectedAgent:    true,
			expectedAllZones: true,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count: 1,
				},
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
			expectedMaster:   false,
			expectedAgent:    false,
			expectedAllZones: false,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count: 1,
				},
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
			expectedMaster:   false,
			expectedAgent:    false,
			expectedAllZones: false,
		},
	}

	for _, c := range cases {
		if c.p.MasterProfile.HasAvailabilityZones() != c.expectedMaster {
			t.Fatalf("expected HasAvailabilityZones() to return %t but instead returned %t", c.expectedMaster, c.p.MasterProfile.HasAvailabilityZones())
		}
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							getMockAddon(common.IPMASQAgentAddonName),
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{},
					},
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       false,
			expectedKubernetesConfigIsIPMasqAgentEnabled: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							{
								Name: common.IPMASQAgentAddonName,
								Containers: []api.KubernetesContainerSpec{
									{
										Name: common.IPMASQAgentAddonName,
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							{
								Name:    common.IPMASQAgentAddonName,
								Enabled: to.BoolPtr(false),
								Containers: []api.KubernetesContainerSpec{
									{
										Name: common.IPMASQAgentAddonName,
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							{
								Name:    common.IPMASQAgentAddonName,
								Enabled: to.BoolPtr(false),
								Containers: []api.KubernetesContainerSpec{
									{
										Name: common.IPMASQAgentAddonName,
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							{
								Name:    common.IPMASQAgentAddonName,
								Enabled: to.BoolPtr(true),
								Containers: []api.KubernetesContainerSpec{
									{
										Name: common.IPMASQAgentAddonName,
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						Addons: []api.KubernetesAddon{
							{
								Name:    common.IPMASQAgentAddonName,
								Enabled: to.BoolPtr(true),
								Containers: []api.KubernetesContainerSpec{
									{
										Name: common.IPMASQAgentAddonName,
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
			name: "From Master Profile",
			properties: &Properties{
				MasterProfile: &MasterProfile{
					DNSPrefix: "foo_master",
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name: "foo_agent0",
					},
				},
			},
			expectedClusterID: "24569115",
		},
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
	cases := common.GetDCSeriesVMCasesForTesting()

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
				OrchestratorType:    api.Kubernetes,
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
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: AKSUbuntu1604,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:  1,
						Distro: Ubuntu,
					},
					{
						Count:  1,
						Distro: AKSUbuntu1604,
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: AKSUbuntu1804,
				},
			},
			expected: true,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: Ubuntu1804,
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: AKSUbuntu1804,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:  1,
						Distro: AKSUbuntu1804,
					},
					{
						Count:  1,
						Distro: AKSUbuntu1804,
					},
				},
			},
			expected: true,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: Ubuntu1804,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:  1,
						Distro: Ubuntu,
					},
					{
						Count:  1,
						Distro: Ubuntu1804Gen2,
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: Ubuntu1804,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:  1,
						Distro: Ubuntu1804,
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: AKSUbuntu1604,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:  1,
						OSType: Windows,
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: AKSUbuntu1804,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:  1,
						OSType: Windows,
					},
				},
			},
			expected: false,
		},
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
						AvailabilityProfile: api.VirtualMachineScaleSets,
						ScaleSetPriority:    api.ScaleSetPrioritySpot,
					},
				},
			},
			expectedHasVMSS: true,
			expectedISVMSS:  true,
			expectedIsAS:    false,
			expectedLowPri:  false,
			expectedSpot:    true,
			expectedVMType:  api.VMSSVMType,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						AvailabilityProfile: api.VirtualMachineScaleSets,
						ScaleSetPriority:    api.ScaleSetPriorityLow,
					},
				},
			},
			expectedHasVMSS: true,
			expectedISVMSS:  true,
			expectedIsAS:    false,
			expectedLowPri:  true,
			expectedSpot:    false,
			expectedVMType:  api.VMSSVMType,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						AvailabilityProfile: api.VirtualMachineScaleSets,
						ScaleSetPriority:    api.ScaleSetPriorityRegular,
					},
					{
						AvailabilityProfile: api.AvailabilitySet,
					},
				},
			},
			expectedHasVMSS: true,
			expectedISVMSS:  true,
			expectedIsAS:    false,
			expectedLowPri:  false,
			expectedSpot:    false,
			expectedVMType:  api.VMSSVMType,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						AvailabilityProfile: api.AvailabilitySet,
					},
				},
			},
			expectedHasVMSS: false,
			expectedISVMSS:  false,
			expectedIsAS:    true,
			expectedLowPri:  false,
			expectedSpot:    false,
			expectedVMType:  api.StandardVMType,
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
			name: "Cluster with HosterMasterProfile",
			properties: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
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
						AvailabilityProfile: api.VirtualMachineScaleSets,
					},
				},
			},
			expectedSubnetName: "aks-subnet",
		},
		{
			name: "Cluster with HosterMasterProfile and custom VNET",
			properties: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
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
						AvailabilityProfile: api.VirtualMachineScaleSets,
						VnetSubnetID:        "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/BazAgentSubnet",
					},
				},
			},
			expectedSubnetName: "BazAgentSubnet",
		},
		{
			name: "Cluster with MasterProfile",
			properties: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &MasterProfile{
					Count:     1,
					DNSPrefix: "foo",
					VMSize:    "Standard_DS2_v2",
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:                "agentpool",
						VMSize:              "Standard_D2_v2",
						Count:               1,
						AvailabilityProfile: api.VirtualMachineScaleSets,
					},
				},
			},
			expectedSubnetName: "k8s-subnet",
		},
		{
			name: "Cluster with MasterProfile and custom VNET",
			properties: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &MasterProfile{
					Count:        1,
					DNSPrefix:    "foo",
					VMSize:       "Standard_DS2_v2",
					VnetSubnetID: "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/BazAgentSubnet",
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:                "agentpool",
						VMSize:              "Standard_D2_v2",
						Count:               1,
						AvailabilityProfile: api.VirtualMachineScaleSets,
					},
				},
			},
			expectedSubnetName: "BazAgentSubnet",
		},
		{
			name: "Cluster with VMSS MasterProfile",
			properties: &Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &MasterProfile{
					Count:               1,
					DNSPrefix:           "foo",
					VMSize:              "Standard_DS2_v2",
					AvailabilityProfile: api.VirtualMachineScaleSets,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Name:                "agentpool",
						VMSize:              "Standard_D2_v2",
						Count:               1,
						AvailabilityProfile: api.VirtualMachineScaleSets,
					},
				},
			},
			expectedSubnetName: "subnetmaster",
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
			OrchestratorType: api.Kubernetes,
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
				AvailabilityProfile: api.VirtualMachineScaleSets,
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

	p = &Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType: api.Kubernetes,
		},
		MasterProfile: &MasterProfile{
			Count:     1,
			DNSPrefix: "foo",
			VMSize:    "Standard_DS2_v2",
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				Count:               1,
				AvailabilityProfile: api.VirtualMachineScaleSets,
			},
		},
	}

	actualRTName = p.GetRouteTableName()
	expectedRTName = "k8s-master-28513887-routetable"

	actualNSGName = p.GetNSGName()
	expectedNSGName = "k8s-master-28513887-nsg"

	if actualRTName != expectedRTName {
		t.Errorf("expected route table name %s, but got %s", actualRTName, expectedRTName)
	}

	if actualNSGName != expectedNSGName {
		t.Errorf("expected route table name %s, but got %s", actualNSGName, expectedNSGName)
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
						AvailabilityProfile: api.VirtualMachineScaleSets,
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
					OrchestratorType: api.Kubernetes,
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
						AvailabilityProfile: api.VirtualMachineScaleSets,
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
				AvailabilityProfile: api.VirtualMachineScaleSets,
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
			OrchestratorType: api.Kubernetes,
		},
		MasterProfile: &MasterProfile{
			Count:     1,
			DNSPrefix: "foo",
			VMSize:    "Standard_DS2_v2",
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				Count:               1,
				AvailabilityProfile: api.AvailabilitySet,
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
			AvailabilityProfile: api.VirtualMachineScaleSets,
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

func TestUbuntuVersion(t *testing.T) {
	cases := []struct {
		p                  Properties
		expectedMaster1604 bool
		expectedAgent1604  bool
		expectedMaster1804 bool
		expectedAgent1804  bool
	}{
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: AKSUbuntu1604,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:  1,
						Distro: AKSUbuntu1604,
						OSType: Linux,
					},
				},
			},
			expectedMaster1604: true,
			expectedAgent1604:  true,
			expectedMaster1804: false,
			expectedAgent1804:  false,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: AKSUbuntu1804,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:  1,
						Distro: ACC1604,
					},
				},
			},
			expectedMaster1604: false,
			expectedAgent1604:  true,
			expectedMaster1804: true,
			expectedAgent1804:  false,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count:  1,
					Distro: Ubuntu,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count:  1,
						Distro: "",
						OSType: Windows,
					},
				},
			},
			expectedMaster1604: true,
			expectedAgent1604:  false,
			expectedMaster1804: false,
			expectedAgent1804:  false,
		},
	}

	for _, c := range cases {
		if c.p.MasterProfile.IsUbuntu1804() != c.expectedMaster1804 {
			t.Fatalf("expected IsUbuntu1804() for master to return %t but instead returned %t", c.expectedMaster1804, c.p.MasterProfile.IsUbuntu1804())
		}
		if c.p.AgentPoolProfiles[0].IsUbuntu1804() != c.expectedAgent1804 {
			t.Fatalf("expected IsUbuntu1804() for agent to return %t but instead returned %t", c.expectedAgent1804, c.p.AgentPoolProfiles[0].IsUbuntu1804())
		}
	}
}

func TestIsCustomVNET(t *testing.T) {
	cases := []struct {
		p              Properties
		expectedMaster bool
		expectedAgent  bool
	}{
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					VnetSubnetID: "testSubnet",
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						VnetSubnetID: "testSubnet",
					},
				},
			},
			expectedMaster: true,
			expectedAgent:  true,
		},
		{
			p: Properties{
				MasterProfile: &MasterProfile{
					Count: 1,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						Count: 1,
					},
					{
						Count: 1,
					},
				},
			},
			expectedMaster: false,
			expectedAgent:  false,
		},
	}

	for _, c := range cases {
		if c.p.MasterProfile.IsCustomVNET() != c.expectedMaster {
			t.Fatalf("expected IsCustomVnet() to return %t but instead returned %t", c.expectedMaster, c.p.MasterProfile.IsCustomVNET())
		}
		if c.p.AgentPoolProfiles[0].IsCustomVNET() != c.expectedAgent {
			t.Fatalf("expected IsCustomVnet() to return %t but instead returned %t", c.expectedAgent, c.p.AgentPoolProfiles[0].IsCustomVNET())
		}
	}

}

func TestAgentPoolProfileGetKubernetesLabels(t *testing.T) {
	cases := []struct {
		name       string
		ap         AgentPoolProfile
		rg         string
		deprecated bool
		expected   string
	}{
		{
			name:       "vanilla pool profile",
			ap:         AgentPoolProfile{},
			rg:         "my-resource-group",
			deprecated: true,
			expected:   "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,kubernetes.azure.com/cluster=my-resource-group",
		},
		{
			name:       "vanilla pool profile, no deprecated labels",
			ap:         AgentPoolProfile{},
			rg:         "my-resource-group",
			deprecated: false,
			expected:   "kubernetes.azure.com/role=agent,agentpool=,kubernetes.azure.com/cluster=my-resource-group",
		},
		{
			name: "with managed disk",
			ap: AgentPoolProfile{
				StorageProfile: api.ManagedDisks,
			},
			rg:         "my-resource-group",
			deprecated: true,
			expected:   "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,storageprofile=managed,storagetier=,kubernetes.azure.com/cluster=my-resource-group",
		},
		{
			name: "N series",
			ap: AgentPoolProfile{
				VMSize: "Standard_NC6",
			},
			rg:         "my-resource-group",
			deprecated: true,
			expected:   "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,accelerator=nvidia,kubernetes.azure.com/cluster=my-resource-group",
		},
		{
			name: "with custom labels",
			ap: AgentPoolProfile{
				CustomNodeLabels: map[string]string{
					"mycustomlabel1": "foo",
					"mycustomlabel2": "bar",
				},
			},
			rg:         "my-resource-group",
			deprecated: true,
			expected:   "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,kubernetes.azure.com/cluster=my-resource-group,mycustomlabel1=foo,mycustomlabel2=bar",
		},
		{
			name: "with custom labels, no deprecated labels",
			ap: AgentPoolProfile{
				CustomNodeLabels: map[string]string{
					"mycustomlabel1": "foo",
					"mycustomlabel2": "bar",
				},
			},
			rg:         "my-resource-group",
			deprecated: false,
			expected:   "kubernetes.azure.com/role=agent,agentpool=,kubernetes.azure.com/cluster=my-resource-group,mycustomlabel1=foo,mycustomlabel2=bar",
		},
		{
			name: "N series and managed disk with custom labels",
			ap: AgentPoolProfile{
				StorageProfile: api.ManagedDisks,
				VMSize:         "Standard_NC6",
				CustomNodeLabels: map[string]string{
					"mycustomlabel1": "foo",
					"mycustomlabel2": "bar",
				},
			},
			rg:         "my-resource-group",
			deprecated: true,
			expected:   "kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=,storageprofile=managed,storagetier=Standard_LRS,accelerator=nvidia,kubernetes.azure.com/cluster=my-resource-group,mycustomlabel1=foo,mycustomlabel2=bar",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.ap.GetKubernetesLabels(c.rg, c.deprecated) {
				t.Fatalf("Got unexpected AgentPoolProfile.GetKubernetesLabels(%s, %t) result. Expected: %s. Got: %s.",
					c.rg, c.deprecated, c.expected, c.ap.GetKubernetesLabels(c.rg, c.deprecated))
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
				MasterProfile: &MasterProfile{
					StorageProfile: api.StorageAccount,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: api.StorageAccount,
						DiskSizesGB:    []int{5},
					},
					{
						StorageProfile: api.StorageAccount,
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
				MasterProfile: &MasterProfile{
					StorageProfile: api.ManagedDisks,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: api.StorageAccount,
					},
					{
						StorageProfile: api.StorageAccount,
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
				MasterProfile: &MasterProfile{
					StorageProfile: api.StorageAccount,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: api.ManagedDisks,
					},
					{
						StorageProfile: api.StorageAccount,
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
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &MasterProfile{
					StorageProfile: api.ManagedDisks,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: api.ManagedDisks,
					},
					{
						StorageProfile: api.ManagedDisks,
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
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &MasterProfile{
					StorageProfile: api.ManagedDisks,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: api.Ephemeral,
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						PrivateCluster: &api.PrivateCluster{
							Enabled: to.BoolPtr(true),
							JumpboxProfile: &api.PrivateJumpboxProfile{
								StorageProfile: api.ManagedDisks,
							},
						},
					},
				},
				MasterProfile: &MasterProfile{
					StorageProfile: api.StorageAccount,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: api.StorageAccount,
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						PrivateCluster: &api.PrivateCluster{
							Enabled: to.BoolPtr(true),
							JumpboxProfile: &api.PrivateJumpboxProfile{
								StorageProfile: api.StorageAccount,
							},
						},
					},
				},
				MasterProfile: &MasterProfile{
					StorageProfile: api.ManagedDisks,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile: api.ManagedDisks,
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
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &MasterProfile{
					StorageProfile: api.ManagedDisks,
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile:      api.ManagedDisks,
						DiskEncryptionSetID: "DiskEncryptionSetID",
					},
					{
						StorageProfile:      api.ManagedDisks,
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
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &MasterProfile{
					StorageProfile:   api.ManagedDisks,
					EncryptionAtHost: to.BoolPtr(true),
				},
				AgentPoolProfiles: []*AgentPoolProfile{
					{
						StorageProfile:   api.ManagedDisks,
						EncryptionAtHost: to.BoolPtr(true),
					},
					{
						StorageProfile:   api.ManagedDisks,
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
			if to.Bool(c.p.MasterProfile.EncryptionAtHost) != c.expectedEncryptionAtHost {
				t.Fatalf("expected EncryptionAtHost to return %v but instead returned %v", c.expectedEncryptionAtHost, to.Bool(c.p.MasterProfile.EncryptionAtHost))
			}
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

func TestAgentPoolProfileIsAuditDEnabled(t *testing.T) {
	cases := []struct {
		name     string
		ap       AgentPoolProfile
		expected bool
	}{
		{
			name:     "default",
			ap:       AgentPoolProfile{},
			expected: false,
		},
		{
			name: "true",
			ap: AgentPoolProfile{
				AuditDEnabled: to.BoolPtr(true),
			},
			expected: true,
		},
		{
			name: "false",
			ap: AgentPoolProfile{
				AuditDEnabled: to.BoolPtr(false),
			},
			expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.ap.IsAuditDEnabled() {
				t.Fatalf("Got unexpected AgentPoolProfile.IsAuditDEnabled() result. Expected: %t. Got: %t.", c.expected, c.ap.IsAuditDEnabled())
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
	if dv != api.KubernetesWindowsDockerVersion {
		t.Fatalf("Expected GetWindowsDockerVersion() to equal default KubernetesWindowsDockerVersion, got %s", dv)
	}

	windowsSku := w.GetWindowsSku()
	if windowsSku != api.KubernetesDefaultWindowsSku {
		t.Fatalf("Expected GetWindowsSku() to equal default KubernetesDefaultWindowsSku, got %s", windowsSku)
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
		WindowsImageSourceURL:     "testCustomImage",
		IsCredentialAutoGenerated: to.BoolPtr(true),
		EnableAHUB:                to.BoolPtr(true),
	}

	if !(w.HasSecrets() && w.HasCustomImage()) {
		t.Fatalf("Expected HasSecrets() and HasCustomImage() to return true")
	}

	w = WindowsProfile{
		WindowsDockerVersion:      "18.03.1-ee-3",
		WindowsSku:                "Datacenter-Core-1809-with-Containers-smalldisk",
		SSHEnabled:                &trueVar,
		IsCredentialAutoGenerated: to.BoolPtr(false),
		EnableAHUB:                to.BoolPtr(false),
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

func TestGetAPIServerEtcdAPIVersion(t *testing.T) {
	o := OrchestratorProfile{}

	if o.GetAPIServerEtcdAPIVersion() != "" {
		t.Fatalf("Expected GetAPIServerEtcdAPIVersion() to return \"\" but instead got %s", o.GetAPIServerEtcdAPIVersion())
	}

	o.KubernetesConfig = &api.KubernetesConfig{
		EtcdVersion: "3.2.1",
	}

	if o.GetAPIServerEtcdAPIVersion() != "etcd3" {
		t.Fatalf("Expected GetAPIServerEtcdAPIVersion() to return \"etcd3\" but instead got %s", o.GetAPIServerEtcdAPIVersion())
	}

	// invalid version string
	o.KubernetesConfig.EtcdVersion = "2.3.8"
	if o.GetAPIServerEtcdAPIVersion() != "etcd2" {
		t.Fatalf("Expected GetAPIServerEtcdAPIVersion() to return \"etcd2\" but instead got %s", o.GetAPIServerEtcdAPIVersion())
	}
}

func TestIsAzureCNI(t *testing.T) {
	k := &api.KubernetesConfig{
		NetworkPlugin: api.NetworkPluginAzure,
	}

	o := &api.OrchestratorProfile{
		KubernetesConfig: k,
	}
	if !o.IsAzureCNI() {
		t.Fatalf("unable to detect orchestrator profile is using Azure CNI from NetworkPlugin=%s", o.KubernetesConfig.NetworkPlugin)
	}

	k = &api.KubernetesConfig{
		NetworkPlugin: "none",
	}

	o = &api.OrchestratorProfile{
		KubernetesConfig: k,
	}
	if o.IsAzureCNI() {
		t.Fatalf("unable to detect orchestrator profile is not using Azure CNI from NetworkPlugin=%s", o.KubernetesConfig.NetworkPlugin)
	}

	o = &api.OrchestratorProfile{}
	if o.IsAzureCNI() {
		t.Fatalf("unable to detect orchestrator profile is not using Azure CNI from nil KubernetesConfig")
	}
}

func TestOrchestrator(t *testing.T) {
	cases := []struct {
		p                    Properties
		expectedIsDCOS       bool
		expectedIsKubernetes bool
		expectedIsSwarmMode  bool
	}{
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.DCOS,
				},
			},
			expectedIsDCOS:       true,
			expectedIsKubernetes: false,
			expectedIsSwarmMode:  false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
			},
			expectedIsDCOS:       false,
			expectedIsKubernetes: true,
			expectedIsSwarmMode:  false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.SwarmMode,
				},
			},
			expectedIsDCOS:       false,
			expectedIsKubernetes: false,
			expectedIsSwarmMode:  true,
		},
	}

	for _, c := range cases {
		if c.expectedIsKubernetes != c.p.OrchestratorProfile.IsKubernetes() {
			t.Fatalf("Expected IsKubernetes() to be %t with OrchestratorType=%s", c.expectedIsKubernetes, c.p.OrchestratorProfile.OrchestratorType)
		}
		if c.expectedIsSwarmMode != c.p.OrchestratorProfile.IsSwarmMode() {
			t.Fatalf("Expected IsSwarmMode() to be %t with OrchestratorType=%s", c.expectedIsSwarmMode, c.p.OrchestratorProfile.OrchestratorType)
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
					OrchestratorType: api.DCOS,
				},
			},
			expected: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
			},
			expected: false,
		},
		{
			p: Properties{
				OrchestratorProfile: &OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						PrivateCluster: &api.PrivateCluster{
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						PrivateCluster: &api.PrivateCluster{
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
					OrchestratorType: api.Kubernetes,
					KubernetesConfig: &api.KubernetesConfig{
						PrivateCluster: &api.PrivateCluster{},
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

func TestMasterProfileHasCosmosEtcd(t *testing.T) {
	cases := []struct {
		name     string
		m        MasterProfile
		expected bool
	}{
		{
			name: "enabled",
			m: MasterProfile{
				CosmosEtcd: to.BoolPtr(true),
			},
			expected: true,
		},
		{
			name: "disabled",
			m: MasterProfile{
				CosmosEtcd: to.BoolPtr(false),
			},
			expected: false,
		},
		{
			name:     "zero value master profile",
			m:        MasterProfile{},
			expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.m.HasCosmosEtcd() {
				t.Fatalf("Got unexpected MasterProfile.HasCosmosEtcd() result. Expected: %t. Got: %t.", c.expected, c.m.HasCosmosEtcd())
			}
		})
	}
}

func TestMasterProfileGetCosmosEndPointURI(t *testing.T) {
	dnsPrefix := "my-prefix"
	etcdEndpointURIFmt := "%sk8s.etcd.cosmosdb.azure.com"
	cases := []struct {
		name     string
		m        MasterProfile
		expected string
	}{
		{
			name: "valid DNS prefix",
			m: MasterProfile{
				CosmosEtcd: to.BoolPtr(true),
				DNSPrefix:  dnsPrefix,
			},
			expected: fmt.Sprintf(etcdEndpointURIFmt, dnsPrefix),
		},
		{
			name: "no DNS prefix",
			m: MasterProfile{
				CosmosEtcd: to.BoolPtr(true),
			},
			expected: fmt.Sprintf(etcdEndpointURIFmt, ""),
		},
		{
			name: "cosmos etcd disabled",
			m: MasterProfile{
				CosmosEtcd: to.BoolPtr(false),
			},
			expected: "",
		},
		{
			name:     "zero value master profile",
			m:        MasterProfile{},
			expected: "",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.m.GetCosmosEndPointURI() {
				t.Fatalf("Got unexpected MasterProfile.GetCosmosEndPointURI() result. Expected: %s. Got: %s.", c.expected, c.m.GetCosmosEndPointURI())
			}
		})
	}
}

func TestMasterAvailabilityProfile(t *testing.T) {
	cases := []struct {
		name           string
		p              Properties
		expectedISVMSS bool
		expectedIsVMAS bool
	}{
		{
			name: "zero value master profile",
			p: Properties{
				MasterProfile: &MasterProfile{},
			},
			expectedISVMSS: false,
			expectedIsVMAS: false,
		},
		{
			name: "master profile w/ AS",
			p: Properties{
				MasterProfile: &MasterProfile{
					AvailabilityProfile: api.AvailabilitySet,
				},
			},
			expectedISVMSS: false,
			expectedIsVMAS: true,
		},
		{
			name: "master profile w/ VMSS",
			p: Properties{
				MasterProfile: &MasterProfile{
					AvailabilityProfile: api.VirtualMachineScaleSets,
				},
			},
			expectedISVMSS: true,
			expectedIsVMAS: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.p.MasterProfile.IsVirtualMachineScaleSets() != c.expectedISVMSS {
				t.Fatalf("expected MasterProfile.IsVirtualMachineScaleSets() to return %t but instead returned %t", c.expectedISVMSS, c.p.MasterProfile.IsVirtualMachineScaleSets())
			}
		})
	}
}

func TestMasterProfileHasMultipleNodes(t *testing.T) {
	cases := []struct {
		name     string
		m        MasterProfile
		expected bool
	}{
		{
			name: "1",
			m: MasterProfile{
				Count: 1,
			},
			expected: false,
		},
		{
			name: "2",
			m: MasterProfile{
				Count: 2,
			},
			expected: true,
		},
		{
			name: "3",
			m: MasterProfile{
				Count: 3,
			},
			expected: true,
		},
		{
			name: "0",
			m: MasterProfile{
				Count: 0,
			},
			expected: false,
		},
		{
			name: "-1",
			m: MasterProfile{
				Count: -1,
			},
			expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.expected != c.m.HasMultipleNodes() {
				t.Fatalf("Got unexpected MasterProfile.HasMultipleNodes() result. Expected: %t. Got: %t.", c.expected, c.m.HasMultipleNodes())
			}
		})
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
