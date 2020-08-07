// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"reflect"
	"strings"
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

	o := api.OrchestratorProfile{
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
	o = api.OrchestratorProfile{
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

	// Test location for Azure Stack Cloud
	mockCSDefaultSpec := getMockBaseContainerService("1.11.6")
	mockCSPDefaultSpec := GetMockPropertiesWithCustomCloudProfile("azurestackcloud", true, true, false)
	mockCSDefaultSpec.Properties.CustomCloudProfile = mockCSPDefaultSpec.CustomCloudProfile
	mockCSDefaultSpec.Location = "randomlocation"

	expectedResult := []string{"randomlocation"}
	actualResult := mockCSDefaultSpec.GetLocations()
	if !reflect.DeepEqual(expectedResult, actualResult) {
		t.Errorf("Test TestGetLocations() : expected to return %s, but got %s . ", expectedResult, actualResult)
	}

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

	p.AADProfile = &api.AADProfile{
		ClientAppID: "test",
		ServerAppID: "test",
	}

	if !p.HasAadProfile() {
		t.Fatalf("Expected HasAadProfile() to return true")
	}

}

func TestGetCustomCloudName(t *testing.T) {
	testcases := []struct {
		name       string
		properties Properties
		expected   string
	}{
		{
			"lower case cloud name",
			GetMockPropertiesWithCustomCloudProfile("azurestackcloud", true, true, true),
			"azurestackcloud",
		},
		{
			"cammel case cloud name",
			GetMockPropertiesWithCustomCloudProfile("AzureStackCloud", true, true, true),
			"AzureStackCloud",
		},
	}
	for _, testcase := range testcases {
		actual := testcase.properties.GetCustomCloudName()
		if testcase.expected != actual {
			t.Errorf("Test \"%s\": expected GetCustomCloudName() to return %s, but got %s . ", testcase.name, testcase.expected, actual)
		}
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
				HostedMasterProfile: &api.HostedMasterProfile{
					IPMasqAgent: false,
				},
			},
			expectedDisabled: true,
		},
		{
			name: "hostedMasterProfile enabled",
			p: &Properties{
				HostedMasterProfile: &api.HostedMasterProfile{
					IPMasqAgent: true,
				},
			},
			expectedDisabled: false,
		},
		{
			name: "nil KubernetesConfig",
			p: &Properties{
				OrchestratorProfile: &api.OrchestratorProfile{},
			},
			expectedDisabled: false,
		},
		{
			name: "default KubernetesConfig",
			p: &Properties{
				OrchestratorProfile: &api.OrchestratorProfile{
					KubernetesConfig: &api.KubernetesConfig{},
				},
			},
			expectedDisabled: false,
		},
		{
			name: "addons configured but no ip-masq-agent configuration",
			p: &Properties{
				OrchestratorProfile: &api.OrchestratorProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
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
				MasterProfile: &api.MasterProfile{
					Count: 1,
				},
			},
			expected: false,
		},
		{
			name: "valid master 3 nodes",
			p: Properties{
				MasterProfile: &api.MasterProfile{
					Count: 3,
				},
			},
			expected: false,
		},
		{
			name: "valid master 5 nodes",
			p: Properties{
				MasterProfile: &api.MasterProfile{
					Count: 5,
				},
			},
			expected: false,
		},
		{
			name: "zero value hosted master",
			p: Properties{
				HostedMasterProfile: &api.HostedMasterProfile{},
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
		MasterProfile: &api.MasterProfile{
			Distro: api.RHEL,
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
			{
				OSType: api.Linux,
			},
			{
				OSType: api.Linux,
				Distro: api.RHEL,
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

	if p.AgentPoolProfiles[0].IsRHEL() {
		t.Fatalf("expected IsRHEL() to return false but instead returned true")
	}

	if p.AgentPoolProfiles[0].IsCoreOS() {
		t.Fatalf("expected IsCoreOS() to return false but instead returned true")
	}

	if !p.AgentPoolProfiles[1].IsRHEL() {
		t.Fatalf("expected IsRHEL() to return true but instead returned false")
	}

	if p.AgentPoolProfiles[1].IsCoreOS() {
		t.Fatalf("expected IsCoreOS() to return false but instead returned true")
	}

	if !p.MasterProfile.IsRHEL() {
		t.Fatalf("expected IsRHEL() to return true but instead returned false")
	}

	if p.MasterProfile.IsCoreOS() {
		t.Fatalf("expected IsCoreOS() to return false but instead returned true")
	}

	p.MasterProfile.Distro = api.CoreOS
	p.AgentPoolProfiles[0].OSType = api.Windows
	p.AgentPoolProfiles[1].Distro = api.CoreOS

	if !p.HasWindows() {
		t.Fatalf("expected HasWindows() to return true but instead returned false")
	}

	if !p.HasCoreOS() {
		t.Fatalf("expected HasCoreOS() to return true but instead returned false")
	}

	if !p.AgentPoolProfiles[0].IsWindows() {
		t.Fatalf("expected IsWindows() to return true but instead returned false")
	}

	if p.AgentPoolProfiles[0].IsLinux() {
		t.Fatalf("expected IsLinux() to return false but instead returned true")
	}

	if p.AgentPoolProfiles[0].IsRHEL() {
		t.Fatalf("expected IsRHEL() to return false but instead returned true")
	}

	if p.AgentPoolProfiles[0].IsCoreOS() {
		t.Fatalf("expected IsCoreOS() to return false but instead returned true")
	}

	if p.AgentPoolProfiles[1].IsRHEL() {
		t.Fatalf("expected IsRHEL() to return false but instead returned true")
	}

	if !p.AgentPoolProfiles[1].IsCoreOS() {
		t.Fatalf("expected IsCoreOS() to return true but instead returned false")
	}

	if p.MasterProfile.IsRHEL() {
		t.Fatalf("expected IsRHEL() to return false but instead returned true")
	}

	if !p.MasterProfile.IsCoreOS() {
		t.Fatalf("expected IsCoreOS() to return true but instead returned false")
	}
}

func TestCloudProviderDefaults(t *testing.T) {
	// Test cloudprovider defaults when no user-provided values
	v := "1.8.0"
	p := Properties{
		OrchestratorProfile: &api.OrchestratorProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
			{
				AvailabilityProfile: api.VirtualMachineScaleSets,
			},
		},
		HostedMasterProfile: &api.HostedMasterProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
			OrchestratorType:    "Kubernetes",
			OrchestratorVersion: v,
			KubernetesConfig:    &api.KubernetesConfig{},
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
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
				MasterProfile: &api.MasterProfile{
					Count: 1,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				MasterProfile: &api.MasterProfile{
					Count: 5,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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

func TestIsAzureStackCloud(t *testing.T) {
	testcases := []struct {
		name       string
		properties Properties
		expected   bool
	}{
		{
			"Empty environment name",
			GetMockPropertiesWithCustomCloudProfile("", true, true, false),
			true,
		},
		{
			"Empty environment name with AzureEnvironmentSpecConfig",
			GetMockPropertiesWithCustomCloudProfile("", true, true, true),
			true,
		},
		{
			"lower case cloud name",
			GetMockPropertiesWithCustomCloudProfile("azurestackcloud", true, true, true),
			true,
		},
		{
			"cammel case cloud name",
			GetMockPropertiesWithCustomCloudProfile("AzureStackCloud", true, true, true),
			true,
		},
		{
			"incorrect cloud name",
			GetMockPropertiesWithCustomCloudProfile("NotAzureStackCloud", true, true, true),
			true,
		},
		{
			"empty cloud profile",
			GetMockPropertiesWithCustomCloudProfile("AzureStackCloud", false, false, false),
			false,
		},
		{
			"empty environment ",
			GetMockPropertiesWithCustomCloudProfile("AzureStackCloud", true, false, true),
			true,
		},
	}
	for _, testcase := range testcases {
		actual := testcase.properties.IsAzureStackCloud()
		if testcase.expected != actual {
			t.Errorf("Test \"%s\": expected IsAzureStackCloud() to return %t, but got %t . ", testcase.name, testcase.expected, actual)
		}
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
				MasterProfile: &api.MasterProfile{
					Count:             1,
					AvailabilityZones: []string{"1", "2"},
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				MasterProfile: &api.MasterProfile{
					Count: 1,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				MasterProfile: &api.MasterProfile{
					Count: 1,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
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
				HostedMasterProfile: &api.HostedMasterProfile{
					IPMasqAgent: true,
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       true,
			expectedKubernetesConfigIsIPMasqAgentEnabled: false, // unsure of the validity of this case, but because it's possible we unit test it
		},
		{
			p: Properties{
				OrchestratorProfile: &api.OrchestratorProfile{
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
				HostedMasterProfile: &api.HostedMasterProfile{
					IPMasqAgent: true,
				},
			},
			expectedPropertiesIsIPMasqAgentEnabled:       true,
			expectedKubernetesConfigIsIPMasqAgentEnabled: true,
		},
		{
			p: Properties{
				OrchestratorProfile: &api.OrchestratorProfile{
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
				HostedMasterProfile: &api.HostedMasterProfile{
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
				MasterProfile: &api.MasterProfile{
					DNSPrefix: "foo_master",
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				HostedMasterProfile: &api.HostedMasterProfile{
					DNSPrefix: "foo_hosted_master",
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  2,
						OSType: api.Linux,
					},
				},
			},
			expected: true,
		},
		{
			name: "two agent pools, one w/ Linux",
			p: &Properties{
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  2,
						OSType: api.Windows,
					},
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						OSType: api.Linux,
					},
				},
			},
			expected: true,
		},
		{
			name: "two agent pools",
			p: &Properties{
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  2,
					},
					{
						Name:   "agentpool1",
						VMSize: "Standard_D2_v2",
						Count:  100,
						OSType: api.Windows,
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
	p.AgentPoolProfiles = []*api.AgentPoolProfile{
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

	p.AgentPoolProfiles = []*api.AgentPoolProfile{
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

func TestGetCustomEnvironmentJSON(t *testing.T) {
	expectedResult := `{"name":"azurestackcloud","managementPortalURL":"https://management.local.azurestack.external/","publishSettingsURL":"https://management.local.azurestack.external/publishsettings/index","serviceManagementEndpoint":"https://management.azurestackci15.onmicrosoft.com/36f71706-54df-4305-9847-5b038a4cf189","resourceManagerEndpoint":"https://management.local.azurestack.external/","activeDirectoryEndpoint":"https://login.windows.net/","galleryEndpoint":"https://portal.local.azurestack.external=30015/","keyVaultEndpoint":"https://vault.azurestack.external/","graphEndpoint":"https://graph.windows.net/","serviceBusEndpoint":"https://servicebus.azurestack.external/","batchManagementEndpoint":"https://batch.azurestack.external/","storageEndpointSuffix":"core.azurestack.external","sqlDatabaseDNSSuffix":"database.azurestack.external","trafficManagerDNSSuffix":"trafficmanager.cn","keyVaultDNSSuffix":"vault.azurestack.external","serviceBusEndpointSuffix":"servicebus.azurestack.external","serviceManagementVMDNSSuffix":"chinacloudapp.cn","resourceManagerVMDNSSuffix":"cloudapp.azurestack.external","containerRegistryDNSSuffix":"azurecr.io","cosmosDBDNSSuffix":"","tokenAudience":"https://management.azurestack.external/","resourceIdentifiers":{"graph":"","keyVault":"","datalake":"","batch":"","operationalInsights":"","storage":""}}`
	testcases := []struct {
		name       string
		properties Properties
		escape     bool
		expected   string
	}{
		{
			"no escape",
			GetMockPropertiesWithCustomCloudProfile("azurestackcloud", true, true, true),
			true,
			strings.Replace(expectedResult, "\"", "\\\"", -1),
		},
		{
			"escape",
			GetMockPropertiesWithCustomCloudProfile("azurestackcloud", true, true, true),
			false,
			expectedResult,
		},
	}
	for _, testcase := range testcases {
		actual, err := testcase.properties.GetCustomEnvironmentJSON(testcase.escape)
		if err != nil {
			t.Error(err)
		}
		if testcase.expected != actual {
			t.Errorf("Test \"%s\": expected GetCustomEnvironmentJSON() to return %s, but got %s . ", testcase.name, testcase.expected, actual)
		}
	}
}

func TestPropertiesHasDCSeriesSKU(t *testing.T) {
	cases := common.GetDCSeriesVMCasesForTesting()

	for _, c := range cases {
		p := Properties{
			AgentPoolProfiles: []*api.AgentPoolProfile{
				{
					Name:   "agentpool",
					VMSize: c.VMSKU,
					Count:  1,
				},
			},
			OrchestratorProfile: &api.OrchestratorProfile{
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
				MasterProfile: &api.MasterProfile{
					Count:  1,
					Distro: api.AKSUbuntu1604,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Count:  1,
						Distro: api.Ubuntu,
					},
					{
						Count:  1,
						Distro: api.AKSUbuntu1604,
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &api.MasterProfile{
					Count:  1,
					Distro: api.AKSUbuntu1804,
				},
			},
			expected: true,
		},
		{
			p: Properties{
				MasterProfile: &api.MasterProfile{
					Count:  1,
					Distro: api.Ubuntu1804,
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &api.MasterProfile{
					Count:  1,
					Distro: api.AKSUbuntu1804,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Count:  1,
						Distro: api.AKSUbuntu1804,
					},
					{
						Count:  1,
						Distro: api.AKSUbuntu1804,
					},
				},
			},
			expected: true,
		},
		{
			p: Properties{
				MasterProfile: &api.MasterProfile{
					Count:  1,
					Distro: api.Ubuntu1804,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Count:  1,
						Distro: api.Ubuntu,
					},
					{
						Count:  1,
						Distro: api.Ubuntu1804Gen2,
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &api.MasterProfile{
					Count:  1,
					Distro: api.Ubuntu1804,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Count:  1,
						Distro: api.Ubuntu1804,
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &api.MasterProfile{
					Count:  1,
					Distro: api.AKSUbuntu1604,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Count:  1,
						OSType: api.Windows,
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				MasterProfile: &api.MasterProfile{
					Count:  1,
					Distro: api.AKSUbuntu1804,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Count:  1,
						OSType: api.Windows,
					},
				},
			},
			expected: false,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Count:  1,
						Distro: api.AKSUbuntu1604,
					},
				},
			},
			expected: true,
		},
		{
			p: Properties{
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Count:  1,
						OSType: api.Windows,
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
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
		if c.p.AgentPoolProfiles[0].IsLowPriorityScaleSet() != c.expectedLowPri {
			t.Fatalf("expected IsLowPriorityScaleSet() to return %t but instead returned %t", c.expectedLowPri, c.p.AgentPoolProfiles[0].IsLowPriorityScaleSet())
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
				OrchestratorProfile: &api.OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
				HostedMasterProfile: &api.HostedMasterProfile{
					FQDN:      "fqdn",
					DNSPrefix: "foo",
					Subnet:    "mastersubnet",
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
				HostedMasterProfile: &api.HostedMasterProfile{
					FQDN:      "fqdn",
					DNSPrefix: "foo",
					Subnet:    "mastersubnet",
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &api.MasterProfile{
					Count:     1,
					DNSPrefix: "foo",
					VMSize:    "Standard_DS2_v2",
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &api.MasterProfile{
					Count:        1,
					DNSPrefix:    "foo",
					VMSize:       "Standard_DS2_v2",
					VnetSubnetID: "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/BazAgentSubnet",
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
				MasterProfile: &api.MasterProfile{
					Count:               1,
					DNSPrefix:           "foo",
					VMSize:              "Standard_DS2_v2",
					AvailabilityProfile: api.VirtualMachineScaleSets,
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
			OrchestratorType: api.Kubernetes,
		},
		HostedMasterProfile: &api.HostedMasterProfile{
			FQDN:      "fqdn",
			DNSPrefix: "foo",
			Subnet:    "mastersubnet",
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
			OrchestratorType: api.Kubernetes,
		},
		MasterProfile: &api.MasterProfile{
			Count:     1,
			DNSPrefix: "foo",
			VMSize:    "Standard_DS2_v2",
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
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
				HostedMasterProfile: &api.HostedMasterProfile{
					FQDN:      "fqdn",
					DNSPrefix: "foo",
					Subnet:    "mastersubnet",
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
				OrchestratorProfile: &api.OrchestratorProfile{
					OrchestratorType: api.Kubernetes,
				},
				HostedMasterProfile: &api.HostedMasterProfile{
					FQDN:      "fqdn",
					DNSPrefix: "foo",
					Subnet:    "mastersubnet",
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
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
		HostedMasterProfile: &api.HostedMasterProfile{
			FQDN:      "fqdn",
			DNSPrefix: "foo",
			Subnet:    "mastersubnet",
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
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
		OrchestratorProfile: &api.OrchestratorProfile{
			OrchestratorType: api.Kubernetes,
		},
		MasterProfile: &api.MasterProfile{
			Count:     1,
			DNSPrefix: "foo",
			VMSize:    "Standard_DS2_v2",
		},
		AgentPoolProfiles: []*api.AgentPoolProfile{
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

	p.AgentPoolProfiles = []*api.AgentPoolProfile{
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
