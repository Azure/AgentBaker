// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/aks-engine/pkg/helpers"
	"github.com/Azure/go-autorest/autorest/to"
)

func TestCertsAlreadyPresent(t *testing.T) {
	var cert *CertificateProfile

	result := certsAlreadyPresent(nil, 1)
	expected := map[string]bool{
		"ca":         false,
		"apiserver":  false,
		"client":     false,
		"kubeconfig": false,
		"etcd":       false,
	}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("certsAlreadyPresent() did not return false for all certs for a non-existent CertificateProfile")
	}
	cert = &CertificateProfile{}
	result = certsAlreadyPresent(cert, 1)
	expected = map[string]bool{
		"ca":         false,
		"apiserver":  false,
		"client":     false,
		"kubeconfig": false,
		"etcd":       false,
	}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("certsAlreadyPresent() did not return false for all certs for empty CertificateProfile")
	}
	cert = &CertificateProfile{
		APIServerCertificate: "a",
	}
	result = certsAlreadyPresent(cert, 1)
	expected = map[string]bool{
		"ca":         false,
		"apiserver":  false,
		"client":     false,
		"kubeconfig": false,
		"etcd":       false,
	}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("certsAlreadyPresent() did not return false for all certs for 1 cert in CertificateProfile")
	}

	cert = &CertificateProfile{
		APIServerCertificate:  "a",
		CaCertificate:         "c",
		CaPrivateKey:          "d",
		ClientCertificate:     "e",
		ClientPrivateKey:      "f",
		KubeConfigCertificate: "g",
		KubeConfigPrivateKey:  "h",
		EtcdClientCertificate: "i",
		EtcdClientPrivateKey:  "j",
		EtcdServerCertificate: "k",
		EtcdServerPrivateKey:  "l",
	}
	result = certsAlreadyPresent(cert, 3)
	expected = map[string]bool{
		"ca":         true,
		"apiserver":  false,
		"client":     true,
		"kubeconfig": true,
		"etcd":       false,
	}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("certsAlreadyPresent() did not return expected result for some certs in CertificateProfile")
	}
	cert = &CertificateProfile{
		APIServerCertificate:  "a",
		APIServerPrivateKey:   "b",
		CaCertificate:         "c",
		CaPrivateKey:          "d",
		ClientCertificate:     "e",
		ClientPrivateKey:      "f",
		KubeConfigCertificate: "g",
		KubeConfigPrivateKey:  "h",
		EtcdClientCertificate: "i",
		EtcdClientPrivateKey:  "j",
		EtcdServerCertificate: "k",
		EtcdServerPrivateKey:  "l",
		EtcdPeerCertificates:  []string{"0", "1", "2"},
		EtcdPeerPrivateKeys:   []string{"0", "1", "2"},
	}
	result = certsAlreadyPresent(cert, 3)
	expected = map[string]bool{
		"ca":         true,
		"apiserver":  true,
		"client":     true,
		"kubeconfig": true,
		"etcd":       true,
	}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("certsAlreadyPresent() did not return expected result for all certs in CertificateProfile")
	}
}

func TestSetMissingKubeletValues(t *testing.T) {
	config := &KubernetesConfig{}
	defaultKubeletConfig := map[string]string{
		"--network-plugin":               "1",
		"--pod-infra-container-image":    "2",
		"--max-pods":                     "3",
		"--eviction-hard":                "4",
		"--node-status-update-frequency": "5",
		"--image-gc-high-threshold":      "6",
		"--image-gc-low-threshold":       "7",
		"--non-masquerade-cidr":          "8",
		"--cloud-provider":               "9",
		"--pod-max-pids":                 "10",
	}
	setMissingKubeletValues(config, defaultKubeletConfig)
	for key, val := range defaultKubeletConfig {
		if config.KubeletConfig[key] != val {
			t.Fatalf("setMissingKubeletValue() did not return the expected value %s for key %s, instead returned: %s", val, key, config.KubeletConfig[key])
		}
	}

	config = &KubernetesConfig{
		KubeletConfig: map[string]string{
			"--network-plugin":            "a",
			"--pod-infra-container-image": "b",
			"--cloud-provider":            "c",
		},
	}
	expectedResult := map[string]string{
		"--network-plugin":               "a",
		"--pod-infra-container-image":    "b",
		"--max-pods":                     "3",
		"--eviction-hard":                "4",
		"--node-status-update-frequency": "5",
		"--image-gc-high-threshold":      "6",
		"--image-gc-low-threshold":       "7",
		"--non-masquerade-cidr":          "8",
		"--cloud-provider":               "c",
		"--pod-max-pids":                 "10",
	}
	setMissingKubeletValues(config, defaultKubeletConfig)
	for key, val := range expectedResult {
		if config.KubeletConfig[key] != val {
			t.Fatalf("setMissingKubeletValue() did not return the expected value %s for key %s, instead returned: %s", val, key, config.KubeletConfig[key])
		}
	}
	config = &KubernetesConfig{
		KubeletConfig: map[string]string{},
	}
	setMissingKubeletValues(config, defaultKubeletConfig)
	for key, val := range defaultKubeletConfig {
		if config.KubeletConfig[key] != val {
			t.Fatalf("setMissingKubeletValue() did not return the expected value %s for key %s, instead returned: %s", val, key, config.KubeletConfig[key])
		}
	}
}

func getFakeAddons(defaultAddonMap map[string]string, customImage string) []api.KubernetesAddon {
	var fakeCustomAddons []api.KubernetesAddon
	for addonName := range defaultAddonMap {
		containerName := addonName
		if addonName == common.ContainerMonitoringAddonName {
			containerName = "omsagent"
		}
		if addonName == common.CalicoAddonName {
			containerName = "calico-typha"
		}
		if addonName == common.AADPodIdentityAddonName {
			containerName = "nmi"
		}
		if addonName == common.KubeDNSAddonName {
			containerName = "kubedns"
		}
		if addonName == common.AntreaAddonName {
			containerName = common.AntreaControllerContainerName
		}
		if addonName == common.FlannelAddonName {
			containerName = common.KubeFlannelContainerName
		}
		customAddon := api.KubernetesAddon{
			Name:    addonName,
			Enabled: to.BoolPtr(true),
			Containers: []api.KubernetesContainerSpec{
				{
					Name:           containerName,
					CPURequests:    "50m",
					MemoryRequests: "150Mi",
					CPULimits:      "50m",
					MemoryLimits:   "150Mi",
				},
			},
		}
		if customImage != "" {
			customAddon.Containers[0].Image = customImage
		}
		fakeCustomAddons = append(fakeCustomAddons, customAddon)
	}
	return fakeCustomAddons
}

func TestAcceleratedNetworking(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled = nil
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows = nil

	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  true,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	// In upgrade scenario, nil AcceleratedNetworkingEnabled should always render as false (i.e., we never turn on this feature on an existing vm that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled) {
		t.Errorf("expected nil acceleratedNetworkingEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled))
	}
	// In upgrade scenario, nil AcceleratedNetworkingEnabledWindows should always render as false (i.e., we never turn on this feature on an existing vm that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows) {
		t.Errorf("expected nil acceleratedNetworkingEnabledWindows to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled = nil
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows = nil

	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    true,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	// In scale scenario, nil AcceleratedNetworkingEnabled should always render as false (i.e., we never turn on this feature on an existing agent pool / VMSS that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled) {
		t.Errorf("expected nil acceleratedNetworkingEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled))
	}
	// In scale scenario, nil AcceleratedNetworkingEnabledWindows should always render as false (i.e., we never turn on this feature on an existing VM that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows) {
		t.Errorf("expected nil acceleratedNetworkingEnabledWindows to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled = nil
	mockCS.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2_v2"
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows = nil
	mockCS.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2_v2"

	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	// In create scenario, nil AcceleratedNetworkingEnabled should be the defaults
	acceleratedNetworkingEnabled := api.DefaultAcceleratedNetworking
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled) != acceleratedNetworkingEnabled {
		t.Errorf("expected default acceleratedNetworkingEnabled to be %t, instead got %t", acceleratedNetworkingEnabled, to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled))
	}
	// In create scenario, nil AcceleratedNetworkingEnabledWindows should be the defaults
	acceleratedNetworkingEnabled = api.DefaultAcceleratedNetworkingWindowsEnabled
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows) != acceleratedNetworkingEnabled {
		t.Errorf("expected default acceleratedNetworkingEnabledWindows to be %t, instead got %t", acceleratedNetworkingEnabled, to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled = nil
	mockCS.Properties.AgentPoolProfiles[0].VMSize = "Standard_D666_v2"
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows = nil
	mockCS.Properties.AgentPoolProfiles[0].VMSize = "Standard_D666_v2"

	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	// In non-supported VM SKU scenario, acceleratedNetworkingEnabled should always be false
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled) {
		t.Errorf("expected acceleratedNetworkingEnabled to be %t for an unsupported VM SKU, instead got %t", false, to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled))
	}
	// In non-supported VM SKU scenario, acceleratedNetworkingEnabledWindows should always be false
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows) {
		t.Errorf("expected acceleratedNetworkingEnabledWindows to be %t for an unsupported VM SKU, instead got %t", false, to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows))
	}
}

func TestVMSSOverProvisioning(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = api.VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = nil
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  true,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	// In upgrade scenario, nil AcceleratedNetworkingEnabled should always render as false (i.e., we never turn on this feature on an existing vm that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) {
		t.Errorf("expected nil VMSSOverProvisioningEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = api.VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = nil
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    true,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	// In scale scenario, nil VMSSOverProvisioningEnabled should always render as false (i.e., we never turn on this feature on an existing agent pool / VMSS that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) {
		t.Errorf("expected nil VMSSOverProvisioningEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = api.VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = nil
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	// In create scenario, nil VMSSOverProvisioningEnabled should be the defaults
	vmssOverProvisioningEnabled := api.DefaultVMSSOverProvisioningEnabled
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) != vmssOverProvisioningEnabled {
		t.Errorf("expected default VMSSOverProvisioningEnabled to be %t, instead got %t", vmssOverProvisioningEnabled, to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = api.VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = to.BoolPtr(true)
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	// In create scenario with explicit true, VMSSOverProvisioningEnabled should be true
	if !to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) {
		t.Errorf("expected VMSSOverProvisioningEnabled to be true, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = api.VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = to.BoolPtr(false)
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	// In create scenario with explicit false, VMSSOverProvisioningEnabled should be false
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) {
		t.Errorf("expected VMSSOverProvisioningEnabled to be false, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}
}

func TestAuditDEnabled(t *testing.T) {
	mockCS := getMockBaseContainerService("1.12.7")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	isUpgrade := true
	mockCS.setAgentProfileDefaults(isUpgrade, false)

	// In upgrade scenario, nil AuditDEnabled should always render as false (i.e., we never turn on this feature on an existing vm that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled) {
		t.Errorf("expected nil AuditDEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled))
	}

	mockCS = getMockBaseContainerService("1.12.7")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	isScale := true
	mockCS.setAgentProfileDefaults(false, isScale)

	// In scale scenario, nil AuditDEnabled should always render as false (i.e., we never turn on this feature on an existing agent pool / vms that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled) {
		t.Errorf("expected nil AuditDEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled))
	}

	mockCS = getMockBaseContainerService("1.12.7")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.setAgentProfileDefaults(false, false)

	// In create scenario, nil AuditDEnabled should be the defaults
	auditDEnabledEnabled := api.DefaultAuditDEnabled
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled) != auditDEnabledEnabled {
		t.Errorf("expected default AuditDEnabled to be %t, instead got %t", auditDEnabledEnabled, to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled = to.BoolPtr(true)
	mockCS.setAgentProfileDefaults(false, false)

	// In create scenario with explicit true, AuditDEnabled should be true
	if !to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled) {
		t.Errorf("expected AuditDEnabled to be true, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled = to.BoolPtr(false)
	mockCS.setAgentProfileDefaults(false, false)

	// In create scenario with explicit false, AuditDEnabled should be false
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled) {
		t.Errorf("expected AuditDEnabled to be false, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled))
	}
}

func TestKubeletFeatureGatesEnsureFeatureGatesOnAgentsFor1_6_0(t *testing.T) {
	mockCS := getMockBaseContainerService("1.6.0")
	properties := mockCS.Properties

	// No KubernetesConfig.KubeletConfig set for MasterProfile or AgentProfile
	// so they will inherit the top-level config
	properties.OrchestratorProfile.KubernetesConfig = getKubernetesConfigWithFeatureGates("TopLevel=true")

	mockCS.setKubeletConfig(false)

	agentFeatureGates := properties.AgentPoolProfiles[0].KubernetesConfig.KubeletConfig["--feature-gates"]
	if agentFeatureGates != "TopLevel=true" {
		t.Fatalf("setKubeletConfig did not add 'TopLevel=true' for agent profile: expected 'TopLevel=true' got '%s'", agentFeatureGates)
	}

	// Verify that the TopLevel feature gate override has only been applied to the agents
	masterFeatureFates := properties.MasterProfile.KubernetesConfig.KubeletConfig["--feature-gates"]
	if masterFeatureFates != "TopLevel=true" {
		t.Fatalf("setKubeletConfig modified feature gates for master profile: expected 'TopLevel=true' got '%s'", agentFeatureGates)
	}
}

func TestKubeletFeatureGatesEnsureMasterAndAgentConfigUsedFor1_6_0(t *testing.T) {
	mockCS := getMockBaseContainerService("1.6.0")
	properties := mockCS.Properties

	// Set MasterProfile and AgentProfiles KubernetesConfig.KubeletConfig values
	// Verify that they are used instead of the top-level config
	properties.OrchestratorProfile.KubernetesConfig = getKubernetesConfigWithFeatureGates("TopLevel=true")
	properties.MasterProfile = &MasterProfile{KubernetesConfig: getKubernetesConfigWithFeatureGates("MasterLevel=true")}
	properties.AgentPoolProfiles[0].KubernetesConfig = getKubernetesConfigWithFeatureGates("AgentLevel=true")

	mockCS.setKubeletConfig(false)

	agentFeatureGates := properties.AgentPoolProfiles[0].KubernetesConfig.KubeletConfig["--feature-gates"]
	if agentFeatureGates != "AgentLevel=true" {
		t.Fatalf("setKubeletConfig agent profile: expected 'AgentLevel=true' got '%s'", agentFeatureGates)
	}

	// Verify that the TopLevel feature gate override has only been applied to the agents
	masterFeatureFates := properties.MasterProfile.KubernetesConfig.KubeletConfig["--feature-gates"]
	if masterFeatureFates != "MasterLevel=true" {
		t.Fatalf("setKubeletConfig master profile: expected 'MasterLevel=true' got '%s'", agentFeatureGates)
	}
}

func TestEtcdDiskSize(t *testing.T) {
	mockCS := getMockBaseContainerService("1.8.10")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 1
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB != api.DefaultEtcdDiskSize {
		t.Fatalf("EtcdDiskSizeGB did not have the expected size, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB, api.DefaultEtcdDiskSize)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 5
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB != api.DefaultEtcdDiskSizeGT3Nodes {
		t.Fatalf("EtcdDiskSizeGB did not have the expected size, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB, api.DefaultEtcdDiskSizeGT3Nodes)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 5
	properties.AgentPoolProfiles[0].Count = 6
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB != api.DefaultEtcdDiskSizeGT10Nodes {
		t.Fatalf("EtcdDiskSizeGB did not have the expected size, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB, api.DefaultEtcdDiskSizeGT10Nodes)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 5
	properties.AgentPoolProfiles[0].Count = 16
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB != api.DefaultEtcdDiskSizeGT20Nodes {
		t.Fatalf("EtcdDiskSizeGB did not have the expected size, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB, api.DefaultEtcdDiskSizeGT20Nodes)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 5
	properties.AgentPoolProfiles[0].Count = 50
	customEtcdDiskSize := "512"
	properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB = customEtcdDiskSize
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB != customEtcdDiskSize {
		t.Fatalf("EtcdDiskSizeGB did not have the expected size, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB, customEtcdDiskSize)
	}
}

func TestGenerateEtcdEncryptionKey(t *testing.T) {
	key1 := generateEtcdEncryptionKey()
	key2 := generateEtcdEncryptionKey()
	if key1 == key2 {
		t.Fatalf("generateEtcdEncryptionKey should return a unique key each time, instead returned identical %s and %s", key1, key2)
	}
	for _, val := range []string{key1, key2} {
		_, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			t.Fatalf("generateEtcdEncryptionKey should return a base64 encoded key, instead returned %s", val)
		}
	}
}

func TestNetworkPolicyDefaults(t *testing.T) {
	mockCS := getMockBaseContainerService("1.8.10")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = api.NetworkPolicyCalico
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != api.NetworkPluginKubenet {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, api.NetworkPluginKubenet)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = api.NetworkPolicyCilium
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != api.NetworkPluginCilium {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, api.NetworkPluginCilium)
	}

	mockCS = getMockBaseContainerService("1.15.7")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = api.NetworkPolicyAntrea
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != api.NetworkPluginAntrea {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, api.NetworkPluginAntrea)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = api.NetworkPolicyAzure
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != api.NetworkPluginAzure {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, api.NetworkPluginAzure)
	}
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy != "" {
		t.Fatalf("NetworkPolicy did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy, "")
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = api.NetworkPolicyNone
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != api.NetworkPluginKubenet {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, api.NetworkPluginKubenet)
	}
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy != "" {
		t.Fatalf("NetworkPolicy did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy, "")
	}
}

func TestNetworkPluginDefaults(t *testing.T) {
	mockCS := getMockBaseContainerService("1.15.7")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != api.DefaultNetworkPlugin {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, api.DefaultNetworkPlugin)
	}

	mockCS = getMockBaseContainerService("1.15.7")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.Addons = []api.KubernetesAddon{
		{
			Name:    common.FlannelAddonName,
			Enabled: to.BoolPtr(true),
		},
	}
	mockCS.setOrchestratorDefaults(true, true)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != api.NetworkPluginFlannel {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, api.NetworkPluginFlannel)
	}
}

func TestContainerRuntime(t *testing.T) {

	for _, mobyVersion := range []string{"3.0.1", "3.0.3", "3.0.4", "3.0.5", "3.0.6", "3.0.7", "3.0.8", "3.0.10"} {
		mockCS := getMockBaseContainerService("1.10.13")
		properties := mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.MobyVersion = mobyVersion
		mockCS.setOrchestratorDefaults(true, true)
		if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != api.Docker {
			t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, api.Docker)
		}
		if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != api.DefaultMobyVersion {
			t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.MobyVersion, api.DefaultMobyVersion)
		}
		if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != "" {
			t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, "")
		}

		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.MobyVersion = mobyVersion
		mockCS.setOrchestratorDefaults(false, false)
		if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != api.Docker {
			t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, api.Docker)
		}
		if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != mobyVersion {
			t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.MobyVersion, mobyVersion)
		}
		if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != "" {
			t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, "")
		}
	}

	mockCS := getMockBaseContainerService("1.10.13")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.setOrchestratorDefaults(false, false)
	if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != api.Docker {
		t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, api.Docker)
	}
	if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != api.DefaultMobyVersion {
		t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.MobyVersion, api.DefaultMobyVersion)
	}
	if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != "" {
		t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, "")
	}

	mockCS = getMockBaseContainerService("1.10.13")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime = api.Containerd
	mockCS.setOrchestratorDefaults(false, false)
	if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != api.Containerd {
		t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, api.Containerd)
	}
	if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != "" {
		t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.MobyVersion, "")
	}
	if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != api.DefaultContainerdVersion {
		t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, api.DefaultContainerdVersion)
	}

	mockCS = getMockBaseContainerService("1.10.13")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime = api.KataContainers
	mockCS.setOrchestratorDefaults(false, false)
	if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != api.KataContainers {
		t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, api.KataContainers)
	}
	if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != "" {
		t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.MobyVersion, "")
	}
	if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != api.DefaultContainerdVersion {
		t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, api.DefaultContainerdVersion)
	}

	for _, containerdVersion := range []string{"1.1.2", "1.1.4", "1.1.5"} {

		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime = api.Containerd
		properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion = containerdVersion
		mockCS.setOrchestratorDefaults(true, true)
		if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != api.Containerd {
			t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, api.Containerd)
		}
		if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != "" {
			t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.MobyVersion, "")
		}
		if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != api.DefaultContainerdVersion {
			t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, api.DefaultContainerdVersion)
		}

		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime = api.Containerd
		properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion = containerdVersion
		mockCS.setOrchestratorDefaults(false, false)
		if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != api.Containerd {
			t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, api.Containerd)
		}
		if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != "" {
			t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.MobyVersion, "")
		}
		if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != containerdVersion {
			t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, containerdVersion)
		}
	}
}

func TestEtcdVersion(t *testing.T) {
	// Default (no value) scenario
	for _, etcdVersion := range []string{""} {
		// Upgrade scenario should always upgrade to newer, default etcd version
		// This sort of artificial (upgrade scenario should always have value), but strictly speaking this is what we want to do
		mockCS := getMockBaseContainerService("1.10.13")
		properties := mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(true, false)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != api.DefaultEtcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, api.DefaultEtcdVersion)
		}

		// Create scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, false)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != api.DefaultEtcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, api.DefaultEtcdVersion)
		}

		// Scale scenario should always accept the provided value
		// This sort of artificial (upgrade scenario should always have value), but strictly speaking this is what we want to do
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, true)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != api.DefaultEtcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, api.DefaultEtcdVersion)
		}
	}

	// These versions are all less than or equal to default
	for _, etcdVersion := range []string{"2.2.5", "3.2.24", api.DefaultEtcdVersion} {
		// Upgrade scenario should always upgrade to newer, default etcd version
		mockCS := getMockBaseContainerService("1.10.13")
		properties := mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(true, false)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != api.DefaultEtcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, api.DefaultEtcdVersion)
		}

		// Create scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, false)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != etcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, etcdVersion)
		}

		// Scale scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, true)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != etcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, etcdVersion)
		}
	}

	// These versions are all greater than default
	for _, etcdVersion := range []string{"3.4.0", "99.99"} {
		// Upgrade scenario should always keep the user-configured etcd version if it is greater than default
		mockCS := getMockBaseContainerService("1.10.13")
		properties := mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(true, false)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != etcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, etcdVersion)
		}

		// Create scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, false)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != etcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, etcdVersion)
		}

		// Scale scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, true)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != etcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, etcdVersion)
		}
	}
}

func TestStorageProfile(t *testing.T) {
	// Test ManagedDisks default configuration
	mockCS := getMockBaseContainerService("1.13.12")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 1
	properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &api.PrivateCluster{
		Enabled:        to.BoolPtr(true),
		JumpboxProfile: &api.PrivateJumpboxProfile{},
	}
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.MasterProfile.StorageProfile != api.ManagedDisks {
		t.Fatalf("MasterProfile.StorageProfile did not have the expected configuration, got %s, expected %s",
			properties.MasterProfile.StorageProfile, api.ManagedDisks)
	}
	if properties.AgentPoolProfiles[0].StorageProfile != api.ManagedDisks {
		t.Fatalf("AgentPoolProfile.StorageProfile did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].StorageProfile, api.ManagedDisks)
	}
	if properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.JumpboxProfile.StorageProfile != api.ManagedDisks {
		t.Fatalf("MasterProfile.StorageProfile did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.JumpboxProfile.StorageProfile, api.ManagedDisks)
	}
	if !properties.AgentPoolProfiles[0].IsVirtualMachineScaleSets() {
		t.Fatalf("AgentPoolProfile[0].AvailabilityProfile did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].AvailabilityProfile, api.AvailabilitySet)
	}

	mockCS = getMockBaseContainerService("1.13.12")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if !properties.AgentPoolProfiles[0].IsVirtualMachineScaleSets() {
		t.Fatalf("AgentPoolProfile[0].AvailabilityProfile did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].AvailabilityProfile, api.VirtualMachineScaleSets)
	}

}

// TestMasterProfileDefaults covers tests for setMasterProfileDefaults
func TestMasterProfileDefaults(t *testing.T) {
	// this validates default masterProfile configuration
	mockCS := getMockBaseContainerService("1.13.12")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = ""
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = api.NetworkPluginAzure
	properties.MasterProfile.AvailabilityProfile = ""
	properties.MasterProfile.Count = 3
	mockCS.Properties = properties
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.MasterProfile.IsVirtualMachineScaleSets() {
		t.Fatalf("Master VMAS, AzureCNI: MasterProfile AvailabilityProfile did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.AvailabilityProfile, api.AvailabilitySet)
	}
	if properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet != api.DefaultKubernetesSubnet {
		t.Fatalf("Master VMAS, AzureCNI: MasterProfile ClusterSubnet did not have the expected default configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet, api.DefaultKubernetesSubnet)
	}
	if properties.MasterProfile.Subnet != properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet {
		t.Fatalf("Master VMAS, AzureCNI: MasterProfile Subnet did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.Subnet, properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet)
	}
	if properties.AgentPoolProfiles[0].Subnet != properties.MasterProfile.Subnet {
		t.Fatalf("Master VMAS, AzureCNI: AgentPoolProfiles Subnet did not have the expected default configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].Subnet, properties.MasterProfile.Subnet)
	}
	if properties.MasterProfile.FirstConsecutiveStaticIP != "10.255.255.5" {
		t.Fatalf("Master VMAS, AzureCNI: MasterProfile FirstConsecutiveStaticIP did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.FirstConsecutiveStaticIP, "10.255.255.5")
	}

	// this validates default VMSS masterProfile configuration
	mockCS = getMockBaseContainerService("1.13.12")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = api.NetworkPluginAzure
	properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = ""
	properties.MasterProfile.AvailabilityProfile = api.VirtualMachineScaleSets
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  true,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if !properties.MasterProfile.IsVirtualMachineScaleSets() {
		t.Fatalf("Master VMSS, AzureCNI: MasterProfile AvailabilityProfile did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.AvailabilityProfile, api.VirtualMachineScaleSets)
	}
	if properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet != api.DefaultKubernetesSubnet {
		t.Fatalf("Master VMSS, AzureCNI: MasterProfile ClusterSubnet did not have the expected default configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet, api.DefaultKubernetesSubnet)
	}
	if properties.MasterProfile.FirstConsecutiveStaticIP != api.DefaultFirstConsecutiveKubernetesStaticIPVMSS {
		t.Fatalf("Master VMSS, AzureCNI: MasterProfile FirstConsecutiveStaticIP did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.FirstConsecutiveStaticIP, api.DefaultFirstConsecutiveKubernetesStaticIPVMSS)
	}
	if properties.MasterProfile.Subnet != api.DefaultKubernetesMasterSubnet {
		t.Fatalf("Master VMSS, AzureCNI: MasterProfile Subnet did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.Subnet, api.DefaultKubernetesMasterSubnet)
	}
	if properties.MasterProfile.AgentSubnet != api.DefaultKubernetesAgentSubnetVMSS {
		t.Fatalf("Master VMSS, AzureCNI: MasterProfile AgentSubnet did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.AgentSubnet, api.DefaultKubernetesAgentSubnetVMSS)
	}

	// this validates default masterProfile configuration and kubenet
	mockCS = getMockBaseContainerService("1.13.12")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = ""
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = api.NetworkPluginKubenet
	properties.MasterProfile.AvailabilityProfile = api.VirtualMachineScaleSets
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    true,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet != api.DefaultKubernetesClusterSubnet {
		t.Fatalf("Master VMSS, kubenet: MasterProfile ClusterSubnet did not have the expected default configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet, api.DefaultKubernetesClusterSubnet)
	}
	if properties.MasterProfile.Subnet != api.DefaultKubernetesMasterSubnet {
		t.Fatalf("Master VMSS, kubenet: MasterProfile Subnet did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.Subnet, api.DefaultKubernetesMasterSubnet)
	}
	if properties.MasterProfile.FirstConsecutiveStaticIP != api.DefaultFirstConsecutiveKubernetesStaticIPVMSS {
		t.Fatalf("Master VMSS, kubenet: MasterProfile FirstConsecutiveStaticIP did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.FirstConsecutiveStaticIP, api.DefaultFirstConsecutiveKubernetesStaticIPVMSS)
	}
	if properties.MasterProfile.AgentSubnet != api.DefaultKubernetesAgentSubnetVMSS {
		t.Fatalf("Master VMSS, kubenet: MasterProfile AgentSubnet did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.AgentSubnet, api.DefaultKubernetesAgentSubnetVMSS)
	}
	properties.MasterProfile.AvailabilityProfile = api.AvailabilitySet
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    true,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.MasterProfile.FirstConsecutiveStaticIP != api.DefaultFirstConsecutiveKubernetesStaticIP {
		t.Fatalf("Master VMAS, kubenet: MasterProfile FirstConsecutiveStaticIP did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.FirstConsecutiveStaticIP, api.DefaultFirstConsecutiveKubernetesStaticIP)
	}

	// this validates default vmas masterProfile configuration, AzureCNI, and custom vnet
	mockCS = getMockBaseContainerService("1.10.3")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.VnetSubnetID = "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/ExampleMasterSubnet"
	properties.MasterProfile.VnetCidr = "10.239.0.0/16"
	properties.MasterProfile.FirstConsecutiveStaticIP = "10.239.255.239"
	properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = ""
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = api.NetworkPluginAzure
	properties.MasterProfile.AvailabilityProfile = api.AvailabilitySet
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    true,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})

	if properties.MasterProfile.FirstConsecutiveStaticIP != "10.239.255.239" {
		t.Fatalf("Master VMAS, AzureCNI, customvnet: MasterProfile FirstConsecutiveStaticIP did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.FirstConsecutiveStaticIP, "10.239.255.239")
	}

	// this validates default VMSS masterProfile configuration, AzureCNI, and custom VNET
	mockCS = getMockBaseContainerService("1.10.3")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.VnetSubnetID = "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/ExampleMasterSubnet"
	properties.MasterProfile.VnetCidr = "10.239.0.0/16"
	properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = ""
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = api.NetworkPluginAzure
	properties.MasterProfile.AvailabilityProfile = api.VirtualMachineScaleSets
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    true,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.MasterProfile.FirstConsecutiveStaticIP != "10.239.0.4" {
		t.Fatalf("Master VMSS, AzureCNI, customvnet: MasterProfile FirstConsecutiveStaticIP did not have the expected default configuration, got %s, expected %s",
			properties.MasterProfile.FirstConsecutiveStaticIP, "10.239.0.4")
	}

	// this validates default configurations for LoadBalancerSku and ExcludeMasterFromStandardLB
	mockCS = getMockBaseContainerService("1.13.12")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku = api.StandardLoadBalancerSku
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	excludeMaster := api.DefaultExcludeMasterFromStandardLB
	if *properties.OrchestratorProfile.KubernetesConfig.ExcludeMasterFromStandardLB != excludeMaster {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ExcludeMasterFromStandardLB did not have the expected configuration, got %t, expected %t",
			*properties.OrchestratorProfile.KubernetesConfig.ExcludeMasterFromStandardLB, excludeMaster)
	}

	// this validates default configurations for MaximumLoadBalancerRuleCount.
	mockCS = getMockBaseContainerService("1.13.12")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.OrchestratorProfile.KubernetesConfig.MaximumLoadBalancerRuleCount != api.DefaultMaximumLoadBalancerRuleCount {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.MaximumLoadBalancerRuleCount did not have the expected configuration, got %d, expected %d",
			properties.OrchestratorProfile.KubernetesConfig.MaximumLoadBalancerRuleCount, api.DefaultMaximumLoadBalancerRuleCount)
	}

	// this validates cluster subnet default configuration for dual stack feature with 1.16
	mockCS = getMockBaseContainerService("1.16.0")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.FeatureFlags = &FeatureFlags{EnableIPv6DualStack: true}
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	expectedClusterSubnet := strings.Join([]string{api.DefaultKubernetesClusterSubnet, "fc00::/8"}, ",")
	if properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet != expectedClusterSubnet {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ClusterSubnet did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet, expectedClusterSubnet)
	}

	// this validates cluster subnet default configuration for dual stack feature in 1.16 when only ipv4 subnet provided
	mockCS = getMockBaseContainerService("1.16.0")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.FeatureFlags = &FeatureFlags{EnableIPv6DualStack: true}
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	expectedClusterSubnet = strings.Join([]string{api.DefaultKubernetesClusterSubnet, "fc00::/8"}, ",")
	if properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet != expectedClusterSubnet {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ClusterSubnet did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet, expectedClusterSubnet)
	}

	// this validates cluster subnet default configuration for dual stack feature.
	mockCS = getMockBaseContainerService("1.17.0")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.FeatureFlags = &FeatureFlags{EnableIPv6DualStack: true}
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	expectedClusterSubnet = strings.Join([]string{api.DefaultKubernetesClusterSubnet, api.DefaultKubernetesClusterSubnetIPv6}, ",")
	if properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet != expectedClusterSubnet {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ClusterSubnet did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet, expectedClusterSubnet)
	}

	// this validates cluster subnet default configuration for dual stack feature when only ipv4 subnet provided
	mockCS = getMockBaseContainerService("1.17.0")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = "10.244.0.0/16"
	properties.FeatureFlags = &FeatureFlags{EnableIPv6DualStack: true}
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	expectedClusterSubnet = strings.Join([]string{"10.244.0.0/16", api.DefaultKubernetesClusterSubnetIPv6}, ",")
	if properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet != expectedClusterSubnet {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ClusterSubnet did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet, expectedClusterSubnet)
	}

	// this validates cluster subnet default configuration for dual stack feature when only ipv6 subnet provided
	mockCS = getMockBaseContainerService("1.17.0")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = "ace:cab:deca::/8"
	properties.FeatureFlags = &FeatureFlags{EnableIPv6DualStack: true}
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	expectedClusterSubnet = strings.Join([]string{api.DefaultKubernetesClusterSubnet, "ace:cab:deca::/8"}, ",")
	if properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet != expectedClusterSubnet {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ClusterSubnet did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet, expectedClusterSubnet)
	}

	// this validates default configurations for OutboundRuleIdleTimeoutInMinutes.
	mockCS = getMockBaseContainerService("1.14.4")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku = api.StandardLoadBalancerSku
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.OrchestratorProfile.KubernetesConfig.OutboundRuleIdleTimeoutInMinutes != api.DefaultOutboundRuleIdleTimeoutInMinutes {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.OutboundRuleIdleTimeoutInMinutes did not have the expected configuration, got %d, expected %d",
			properties.OrchestratorProfile.KubernetesConfig.OutboundRuleIdleTimeoutInMinutes, api.DefaultOutboundRuleIdleTimeoutInMinutes)
	}

	// this validates cluster subnet default configuration for single stack IPv6 only cluster
	mockCS = getMockBaseContainerService("1.18.0")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.FeatureFlags = &FeatureFlags{EnableIPv6Only: true}
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.OrchestratorProfile.KubernetesConfig.DNSServiceIP != api.DefaultKubernetesDNSServiceIPv6 {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.DNSServiceIP did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.DNSServiceIP, api.DefaultKubernetesDNSServiceIPv6)
	}
	if properties.OrchestratorProfile.KubernetesConfig.ServiceCIDR != api.DefaultKubernetesServiceCIDRIPv6 {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ServiceCIDR did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ServiceCIDR, api.DefaultKubernetesServiceCIDRIPv6)
	}
	if properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet != api.DefaultKubernetesClusterSubnetIPv6 {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ClusterSubnet did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet, api.DefaultKubernetesClusterSubnetIPv6)
	}
}

func TestAgentPoolProfile(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 1
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.AgentPoolProfiles[0].ScaleSetPriority != "" {
		t.Fatalf("AgentPoolProfiles[0].ScaleSetPriority did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].ScaleSetPriority, "")
	}
	if properties.AgentPoolProfiles[0].ScaleSetEvictionPolicy != "" {
		t.Fatalf("AgentPoolProfiles[0].ScaleSetEvictionPolicy did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].ScaleSetEvictionPolicy, "")
	}
	properties.AgentPoolProfiles[0].ScaleSetPriority = api.ScaleSetPriorityLow
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.AgentPoolProfiles[0].ScaleSetEvictionPolicy != api.ScaleSetEvictionPolicyDelete {
		t.Fatalf("AgentPoolProfile[0].ScaleSetEvictionPolicy did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].ScaleSetEvictionPolicy, api.ScaleSetEvictionPolicyDelete)
	}
	properties.AgentPoolProfiles[0].ScaleSetPriority = api.ScaleSetPrioritySpot
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.AgentPoolProfiles[0].ScaleSetEvictionPolicy != api.ScaleSetEvictionPolicyDelete {
		t.Fatalf("AgentPoolProfile[0].ScaleSetEvictionPolicy did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].ScaleSetEvictionPolicy, api.ScaleSetEvictionPolicyDelete)
	}
	if *properties.AgentPoolProfiles[0].SpotMaxPrice != float64(-1) {
		t.Fatalf("AgentPoolProfile[0].SpotMaxPrice did not have the expected value, got %g, expected %g",
			*properties.AgentPoolProfiles[0].SpotMaxPrice, float64(-1))
	}

	properties.AgentPoolProfiles[0].SpotMaxPrice = to.Float64Ptr(float64(88))
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if *properties.AgentPoolProfiles[0].SpotMaxPrice != float64(88) {
		t.Fatalf("AgentPoolProfile[0].SpotMaxPrice did not have the expected value, got %g, expected %g",
			*properties.AgentPoolProfiles[0].SpotMaxPrice, float64(88))
	}
}

// TestDistroDefaults covers tests for setMasterProfileDefaults and setAgentProfileDefaults
func TestDistroDefaults(t *testing.T) {

	var tests = []struct {
		name                   string              // test case name
		orchestratorProfile    OrchestratorProfile // orchestrator to be tested
		masterProfileDistro    Distro
		agentPoolProfileDistro Distro
		expectedAgentDistro    Distro // expected agent result default disto to be used
		expectedMasterDistro   Distro // expected master result default disto to be used
		isUpgrade              bool
		isScale                bool
		cloudName              string
	}{
		{
			"default_kubernetes",
			OrchestratorProfile{
				OrchestratorType: api.Kubernetes,
				KubernetesConfig: &KubernetesConfig{},
			},
			"",
			"",
			AKSUbuntu1604,
			AKSUbuntu1604,
			false,
			false,
			api.AzurePublicCloud,
		},
		{
			"default_kubernetes_usgov",
			OrchestratorProfile{
				OrchestratorType: api.Kubernetes,
				KubernetesConfig: &KubernetesConfig{},
			},
			"",
			"",
			AKSUbuntu1604,
			AKSUbuntu1604,
			false,
			false,
			api.AzureUSGovernmentCloud,
		},
		{
			"1804_upgrade_kubernetes",
			OrchestratorProfile{
				OrchestratorType: api.Kubernetes,
				KubernetesConfig: &KubernetesConfig{},
			},
			AKSUbuntu1804,
			AKSUbuntu1804,
			AKSUbuntu1804,
			AKSUbuntu1804,
			true,
			false,
			api.AzurePublicCloud,
		},
		{
			"default_kubernetes_germancloud",
			OrchestratorProfile{
				OrchestratorType: api.Kubernetes,
				KubernetesConfig: &KubernetesConfig{},
			},
			AKS1604Deprecated,
			AKS1604Deprecated,
			Ubuntu,
			Ubuntu,
			true,
			false,
			api.AzureGermanCloud,
		},
		{
			"deprecated_distro_kubernetes",
			OrchestratorProfile{
				OrchestratorType: api.Kubernetes,
				KubernetesConfig: &KubernetesConfig{},
			},
			AKS1604Deprecated,
			AKS1604Deprecated,
			AKSUbuntu1604,
			AKSUbuntu1604,
			true,
			false,
			api.AzureChinaCloud,
		},
		{
			"docker_engine_kubernetes",
			OrchestratorProfile{
				OrchestratorType: api.Kubernetes,
				KubernetesConfig: &KubernetesConfig{},
			},
			AKS1604Deprecated,
			AKSDockerEngine,
			AKSUbuntu1604,
			AKSUbuntu1604,
			false,
			true,
			api.AzurePublicCloud,
		},
		{
			"default_swarm",
			OrchestratorProfile{
				OrchestratorType: api.Swarm,
			},
			"",
			"",
			Ubuntu,
			Ubuntu,
			false,
			false,
			api.AzurePublicCloud,
		},
		{
			"default_swarmmode",
			OrchestratorProfile{
				OrchestratorType: api.SwarmMode,
			},
			"",
			"",
			Ubuntu,
			Ubuntu,
			false,
			false,
			api.AzurePublicCloud,
		},
		{
			"default_dcos",
			OrchestratorProfile{
				OrchestratorType: api.DCOS,
			},
			"",
			"",
			Ubuntu,
			Ubuntu,
			false,
			false,
			api.AzurePublicCloud,
		},
	}

	for _, test := range tests {
		mockAPI := getMockAPIProperties("1.0.0")
		mockAPI.OrchestratorProfile = &test.orchestratorProfile
		mockAPI.MasterProfile.Distro = test.masterProfileDistro
		for _, agent := range mockAPI.AgentPoolProfiles {
			agent.Distro = test.agentPoolProfileDistro
		}
		cs := &ContainerService{
			Properties: &mockAPI,
		}
		switch test.cloudName {
		case api.AzurePublicCloud:
			cs.Location = "westus2"
		case api.AzureChinaCloud:
			cs.Location = "chinaeast"
		case api.AzureGermanCloud:
			cs.Location = "germanynortheast"
		case api.AzureUSGovernmentCloud:
			cs.Location = "usgovnorth"
		default:
			cs.Location = "westus2"
		}
		cs.Properties.OrchestratorProfile.KubernetesConfig = &KubernetesConfig{
			LoadBalancerSku: api.StandardLoadBalancerSku,
		}
		cs.SetPropertiesDefaults(api.PropertiesDefaultsParams{
			IsScale:    test.isScale,
			IsUpgrade:  test.isUpgrade,
			PkiKeySize: helpers.DefaultPkiKeySize,
		})
		if cs.Properties.MasterProfile.Distro != test.expectedMasterDistro {
			t.Fatalf("SetPropertiesDefaults() test case %v did not return right masterProfile Distro configurations %v != %v", test.name, cs.Properties.MasterProfile.Distro, test.expectedMasterDistro)
		}
		for _, agent := range cs.Properties.AgentPoolProfiles {
			if agent.Distro != test.expectedAgentDistro {
				t.Fatalf("SetPropertiesDefaults() test case %v did not return right pool Distro configurations %v != %v", test.name, agent.Distro, test.expectedAgentDistro)
			}
			if to.Bool(agent.SinglePlacementGroup) != false {
				t.Fatalf("SetPropertiesDefaults() test case %v did not return right singlePlacementGroup configurations %v != %v", test.name, agent.SinglePlacementGroup, false)
			}
		}
	}
}

func TestWindowsProfileDefaults(t *testing.T) {
	trueVar := true

	var tests = []struct {
		name                   string // test case name
		windowsProfile         WindowsProfile
		expectedWindowsProfile WindowsProfile
		isUpgrade              bool
		isScale                bool
	}{
		{
			"defaults in creating",
			WindowsProfile{},
			WindowsProfile{
				WindowsPublisher:      api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            api.AKSWindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          api.AKSWindowsServer2019OSImageConfig.ImageVersion,
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"aks vhd current version in creating",
			WindowsProfile{
				WindowsPublisher: api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       api.AKSWindowsServer2019OSImageConfig.ImageSku,
			},
			WindowsProfile{
				WindowsPublisher:      api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            api.AKSWindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          api.AKSWindowsServer2019OSImageConfig.ImageVersion,
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"aks vhd override sku in creating",
			WindowsProfile{
				WindowsPublisher: api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
			},
			WindowsProfile{
				WindowsPublisher:      api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            "override",
				ImageVersion:          "latest",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"aks vhd override version in creating",
			WindowsProfile{
				WindowsPublisher: api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       api.AKSWindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            api.AKSWindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          "override",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"vanilla vhd current version in creating",
			WindowsProfile{
				WindowsPublisher: api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       api.WindowsServer2019OSImageConfig.ImageSku,
			},
			WindowsProfile{
				WindowsPublisher:      api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            api.WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          api.WindowsServer2019OSImageConfig.ImageVersion,
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"vanilla vhd override sku in creating",
			WindowsProfile{
				WindowsPublisher: api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
			},
			WindowsProfile{
				WindowsPublisher:      api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            "override",
				ImageVersion:          "latest",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"vanilla vhd override version in creating",
			WindowsProfile{
				WindowsPublisher: api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.WindowsServer2019OSImageConfig.ImageOffer,
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            api.WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          "override",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"vanilla vhd spepcific version in creating",
			WindowsProfile{
				WindowsPublisher: api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       api.WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            api.WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          "override",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"user overrides latest version in creating",
			WindowsProfile{
				WindowsPublisher: "override",
				WindowsOffer:     "override",
				WindowsSku:       "override",
			},
			WindowsProfile{
				WindowsPublisher:      "override",
				WindowsOffer:          "override",
				WindowsSku:            "override",
				ImageVersion:          "latest",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"user overrides specific version in creating",
			WindowsProfile{
				WindowsPublisher: "override",
				WindowsOffer:     "override",
				WindowsSku:       "override",
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      "override",
				WindowsOffer:          "override",
				WindowsSku:            "override",
				ImageVersion:          "override",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            &trueVar,
			},
			false,
			false,
		},
		{
			"aks-engine does not set default ProvisioningScriptsPackageURL when it is not empty in upgrading",
			WindowsProfile{
				WindowsPublisher: api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       api.WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:     api.WindowsServer2019OSImageConfig.ImageVersion,
			},
			WindowsProfile{
				WindowsPublisher:      api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            api.WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          api.WindowsServer2019OSImageConfig.ImageVersion,
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            nil,
			},
			true,
			false,
		},
		{
			"aks-engine sets default WindowsSku and ImageVersion when they are empty in upgrading",
			WindowsProfile{
				WindowsPublisher: api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "",
				ImageVersion:     "",
			},
			WindowsProfile{
				WindowsPublisher:      api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            api.AKSWindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          api.AKSWindowsServer2019OSImageConfig.ImageVersion,
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            nil,
			},
			true,
			false,
		},
		{
			"aks-engine does not set default WindowsSku and ImageVersion when they are not empty in upgrading",
			WindowsProfile{
				WindowsPublisher: api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            "override",
				ImageVersion:          "override",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            nil,
			},
			true,
			false,
		},
		{
			"aks-engine sets default vanilla WindowsSku and ImageVersion when they are empty in upgrading",
			WindowsProfile{
				WindowsPublisher: api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "",
				ImageVersion:     "",
			},
			WindowsProfile{
				WindowsPublisher:      api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            api.WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          api.WindowsServer2019OSImageConfig.ImageVersion,
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            nil,
			},
			true,
			false,
		},
		{
			"aks-engine does not set vanilla default WindowsSku and ImageVersion when they are not empty in upgrading",
			WindowsProfile{
				WindowsPublisher: api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            "override",
				ImageVersion:          "override",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            nil,
			},
			true,
			false,
		},
		{
			"aks-engine does not override version when WindowsPublisher does not match in upgrading",
			WindowsProfile{
				WindowsPublisher: api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
				ImageVersion:     "",
			},
			WindowsProfile{
				WindowsPublisher:      api.WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            "override",
				ImageVersion:          "",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            nil,
			},
			true,
			false,
		},
		{
			"aks-engine does not override version when WindowsOffer does not match in upgrading",
			WindowsProfile{
				WindowsPublisher: api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "",
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            "",
				ImageVersion:          "override",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            nil,
			},
			true,
			false,
		},
		{
			"aks-engine does not change any value in scaling",
			WindowsProfile{
				WindowsPublisher: api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "",
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      api.AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          api.AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            "",
				ImageVersion:          "override",
				AdminUsername:         "",
				AdminPassword:         "",
				WindowsImageSourceURL: "",
				WindowsDockerVersion:  "",
				SSHEnabled:            nil,
			},
			false,
			true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mockAPI := getMockAPIProperties("1.16.0")
			mockAPI.WindowsProfile = &test.windowsProfile
			cs := ContainerService{
				Properties: &mockAPI,
			}
			cs.setWindowsProfileDefaults(test.isUpgrade, test.isScale)

			actual := mockAPI.WindowsProfile
			expected := &test.expectedWindowsProfile

			equal := reflect.DeepEqual(actual, expected)
			if !equal {
				t.Errorf("unexpected diff while comparing WindowsProfile. expected: %v, actual: %v", *expected, *actual)
			}
		})
	}
}

// TestSetVMSSDefaultsAndZones covers tests for setVMSSDefaultsForAgents and masters
func TestSetVMSSDefaultsAndZones(t *testing.T) {
	// masters with VMSS and no zones
	mockCS := getMockBaseContainerService("1.12.0")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.AvailabilityProfile = api.VirtualMachineScaleSets
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.MasterProfile.HasAvailabilityZones() {
		t.Fatalf("MasterProfile.HasAvailabilityZones did not have the expected return, got %t, expected %t",
			properties.MasterProfile.HasAvailabilityZones(), false)
	}
	if properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku != api.DefaultLoadBalancerSku {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.LoadBalancerSku did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku, api.DefaultLoadBalancerSku)
	}
	// masters with VMSS and zones
	mockCS = getMockBaseContainerService("1.12.0")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.AvailabilityProfile = api.VirtualMachineScaleSets
	properties.MasterProfile.AvailabilityZones = []string{"1", "2"}
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	singlePlacementGroup := api.DefaultSinglePlacementGroup
	if *properties.MasterProfile.SinglePlacementGroup != singlePlacementGroup {
		t.Fatalf("MasterProfile.SinglePlacementGroup default did not have the expected configuration, got %t, expected %t",
			*properties.MasterProfile.SinglePlacementGroup, singlePlacementGroup)
	}
	if !properties.MasterProfile.HasAvailabilityZones() {
		t.Fatalf("MasterProfile.HasAvailabilityZones did not have the expected return, got %t, expected %t",
			properties.MasterProfile.HasAvailabilityZones(), true)
	}
	if properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku != api.StandardLoadBalancerSku {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.LoadBalancerSku did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku, api.StandardLoadBalancerSku)
	}
	excludeMaster := api.DefaultExcludeMasterFromStandardLB
	if *properties.OrchestratorProfile.KubernetesConfig.ExcludeMasterFromStandardLB != excludeMaster {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ExcludeMasterFromStandardLB did not have the expected configuration, got %t, expected %t",
			*properties.OrchestratorProfile.KubernetesConfig.ExcludeMasterFromStandardLB, excludeMaster)
	}
	// agents with VMSS and no zones
	mockCS = getMockBaseContainerService("1.12.0")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.AgentPoolProfiles[0].Count = 4
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if properties.AgentPoolProfiles[0].HasAvailabilityZones() {
		t.Fatalf("AgentPoolProfiles[0].HasAvailabilityZones did not have the expected return, got %t, expected %t",
			properties.AgentPoolProfiles[0].HasAvailabilityZones(), false)
	}
	if properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku != api.DefaultLoadBalancerSku {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.LoadBalancerSku did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku, api.DefaultLoadBalancerSku)
	}
	// agents with VMSS and zones
	mockCS = getMockBaseContainerService("1.13.12")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.AgentPoolProfiles[0].Count = 4
	properties.AgentPoolProfiles[0].SinglePlacementGroup = nil
	properties.AgentPoolProfiles[0].AvailabilityZones = []string{"1", "2"}
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if !properties.AgentPoolProfiles[0].IsVirtualMachineScaleSets() {
		t.Fatalf("AgentPoolProfile[0].AvailabilityProfile did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].AvailabilityProfile, api.VirtualMachineScaleSets)
	}
	if !properties.AgentPoolProfiles[0].HasAvailabilityZones() {
		t.Fatalf("AgentPoolProfiles[0].HasAvailabilityZones did not have the expected return, got %t, expected %t",
			properties.AgentPoolProfiles[0].HasAvailabilityZones(), true)
	}
	singlePlacementGroup = false
	if *properties.AgentPoolProfiles[0].SinglePlacementGroup != singlePlacementGroup {
		t.Fatalf("AgentPoolProfile[0].SinglePlacementGroup default did not have the expected configuration, got %t, expected %t",
			*properties.AgentPoolProfiles[0].SinglePlacementGroup, singlePlacementGroup)
	}
	if properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku != api.StandardLoadBalancerSku {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.LoadBalancerSku did not have the expected configuration, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku, api.StandardLoadBalancerSku)
	}
	excludeMaster = api.DefaultExcludeMasterFromStandardLB
	if *properties.OrchestratorProfile.KubernetesConfig.ExcludeMasterFromStandardLB != excludeMaster {
		t.Fatalf("OrchestratorProfile.KubernetesConfig.ExcludeMasterFromStandardLB did not have the expected configuration, got %t, expected %t",
			*properties.OrchestratorProfile.KubernetesConfig.ExcludeMasterFromStandardLB, excludeMaster)
	}

	properties.AgentPoolProfiles[0].SinglePlacementGroup = nil
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if to.Bool(properties.AgentPoolProfiles[0].SinglePlacementGroup) {
		t.Fatalf("AgentPoolProfile[0].SinglePlacementGroup did not have the expected configuration, got %t, expected %t",
			*properties.AgentPoolProfiles[0].SinglePlacementGroup, false)
	}

	if !*properties.AgentPoolProfiles[0].SinglePlacementGroup && properties.AgentPoolProfiles[0].StorageProfile != api.ManagedDisks {
		t.Fatalf("AgentPoolProfile[0].StorageProfile did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].StorageProfile, api.ManagedDisks)
	}

}

func TestAzureCNIVersionString(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10.3")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 1
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = api.NetworkPluginAzure
	mockCS.setOrchestratorDefaults(true, true)

	if properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion != api.AzureCniPluginVerLinux {
		t.Fatalf("Azure CNI Version string not the expected value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion, api.AzureCniPluginVerLinux)
	}

	mockCS = getMockBaseContainerService("1.10.3")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 1
	properties.AgentPoolProfiles[0].OSType = Windows
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = api.NetworkPluginAzure
	mockCS.setOrchestratorDefaults(true, true)

	if properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion != api.AzureCniPluginVerWindows {
		t.Fatalf("Azure CNI Version string not the expected value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion, api.AzureCniPluginVerWindows)
	}

	mockCS = getMockBaseContainerService("1.10.3")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 1
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = api.NetworkPluginKubenet
	mockCS.setOrchestratorDefaults(true, true)

	if properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion != "" {
		t.Fatalf("Azure CNI Version string not the expected value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion, "")
	}
}

func TestEnableAggregatedAPIs(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10.3")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.EnableRbac = to.BoolPtr(false)
	mockCS.setOrchestratorDefaults(true, true)

	if properties.OrchestratorProfile.KubernetesConfig.EnableAggregatedAPIs {
		t.Fatalf("got unexpected EnableAggregatedAPIs config value for EnableRbac=false: %t",
			properties.OrchestratorProfile.KubernetesConfig.EnableAggregatedAPIs)
	}

	mockCS = getMockBaseContainerService("1.10.3")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.EnableRbac = to.BoolPtr(true)
	mockCS.setOrchestratorDefaults(true, true)

	if !properties.OrchestratorProfile.KubernetesConfig.EnableAggregatedAPIs {
		t.Fatalf("got unexpected EnableAggregatedAPIs config value for EnableRbac=true: %t",
			properties.OrchestratorProfile.KubernetesConfig.EnableAggregatedAPIs)
	}
}

func TestCloudControllerManagerEnabled(t *testing.T) {
	// test that 1.16 defaults to false
	cs := CreateMockContainerService("testcluster", "1.16.1", 3, 2, false)
	cs.setOrchestratorDefaults(false, false)
	if cs.Properties.OrchestratorProfile.KubernetesConfig.UseCloudControllerManager == to.BoolPtr(true) {
		t.Fatal("expected UseCloudControllerManager to default to false")
	}

	// test that 1.17 defaults to false
	cs = CreateMockContainerService("testcluster", "1.17.0", 3, 2, false)
	cs.setOrchestratorDefaults(false, false)
	if cs.Properties.OrchestratorProfile.KubernetesConfig.UseCloudControllerManager == to.BoolPtr(true) {
		t.Fatal("expected UseCloudControllerManager to default to false")
	}
}

func TestDefaultCloudProvider(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10.3")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	mockCS.setOrchestratorDefaults(true, true)

	if to.Bool(properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff) {
		t.Fatalf("got unexpected CloudProviderBackoff expected false, got %t",
			to.Bool(properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff))
	}

	if !to.Bool(properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimit) {
		t.Fatalf("got unexpected CloudProviderBackoff expected true, got %t",
			to.Bool(properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff))
	}

	mockCS = getMockBaseContainerService("1.10.3")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff = to.BoolPtr(false)
	properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimit = to.BoolPtr(false)
	mockCS.setOrchestratorDefaults(true, true)

	if to.Bool(properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff) {
		t.Fatalf("got unexpected CloudProviderBackoff expected true, got %t",
			to.Bool(properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff))
	}

	if to.Bool(properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimit) {
		t.Fatalf("got unexpected CloudProviderBackoff expected true, got %t",
			to.Bool(properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff))
	}
}

func TestCloudProviderBackoff(t *testing.T) {
	cases := []struct {
		name      string
		cs        ContainerService
		isUpgrade bool
		isScale   bool
		expected  KubernetesConfig
	}{
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType: api.Kubernetes,
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: KubernetesConfig{
				CloudProviderBackoffMode:          "v1",
				CloudProviderBackoff:              to.BoolPtr(false),
				CloudProviderBackoffRetries:       api.DefaultKubernetesCloudProviderBackoffRetries,
				CloudProviderBackoffJitter:        api.DefaultKubernetesCloudProviderBackoffJitter,
				CloudProviderBackoffDuration:      api.DefaultKubernetesCloudProviderBackoffDuration,
				CloudProviderBackoffExponent:      api.DefaultKubernetesCloudProviderBackoffExponent,
				CloudProviderRateLimit:            to.BoolPtr(api.DefaultKubernetesCloudProviderRateLimit),
				CloudProviderRateLimitQPS:         api.DefaultKubernetesCloudProviderRateLimitQPS,
				CloudProviderRateLimitQPSWrite:    api.DefaultKubernetesCloudProviderRateLimitQPSWrite,
				CloudProviderRateLimitBucket:      api.DefaultKubernetesCloudProviderRateLimitBucket,
				CloudProviderRateLimitBucketWrite: api.DefaultKubernetesCloudProviderRateLimitBucketWrite,
			},
		},
		{
			name: "Kubernetes 1.14.0",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: KubernetesConfig{
				CloudProviderBackoffMode:          "v2",
				CloudProviderBackoff:              to.BoolPtr(true),
				CloudProviderBackoffRetries:       api.DefaultKubernetesCloudProviderBackoffRetries,
				CloudProviderBackoffJitter:        0,
				CloudProviderBackoffDuration:      api.DefaultKubernetesCloudProviderBackoffDuration,
				CloudProviderBackoffExponent:      0,
				CloudProviderRateLimit:            to.BoolPtr(api.DefaultKubernetesCloudProviderRateLimit),
				CloudProviderRateLimitQPS:         api.DefaultKubernetesCloudProviderRateLimitQPS,
				CloudProviderRateLimitQPSWrite:    api.DefaultKubernetesCloudProviderRateLimitQPSWrite,
				CloudProviderRateLimitBucket:      api.DefaultKubernetesCloudProviderRateLimitBucket,
				CloudProviderRateLimitBucketWrite: api.DefaultKubernetesCloudProviderRateLimitBucketWrite,
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.cs.setOrchestratorDefaults(c.isUpgrade, c.isScale)
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffMode != c.expected.CloudProviderBackoffMode {
				t.Errorf("expected %s, but got %s", c.expected.CloudProviderBackoffMode, c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffMode)
			}
			if to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff) != to.Bool(c.expected.CloudProviderBackoff) {
				t.Errorf("expected %t, but got %t", to.Bool(c.expected.CloudProviderBackoff), to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff))
			}
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffRetries != c.expected.CloudProviderBackoffRetries {
				t.Errorf("expected %d, but got %d", c.expected.CloudProviderBackoffRetries, c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffRetries)
			}
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffJitter != c.expected.CloudProviderBackoffJitter {
				t.Errorf("expected %f, but got %f", c.expected.CloudProviderBackoffJitter, c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffJitter)
			}
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffDuration != c.expected.CloudProviderBackoffDuration {
				t.Errorf("expected %d, but got %d", c.expected.CloudProviderBackoffDuration, c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffDuration)
			}
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffExponent != c.expected.CloudProviderBackoffExponent {
				t.Errorf("expected %f, but got %f", c.expected.CloudProviderBackoffExponent, c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffExponent)
			}
			if to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimit) != to.Bool(c.expected.CloudProviderRateLimit) {
				t.Errorf("expected %t, but got %t", to.Bool(c.expected.CloudProviderRateLimit), to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimit))
			}
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPS != c.expected.CloudProviderRateLimitQPS {
				t.Errorf("expected %f, but got %f", c.expected.CloudProviderRateLimitQPS, c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPS)
			}
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPSWrite != c.expected.CloudProviderRateLimitQPSWrite {
				t.Errorf("expected %f, but got %f", c.expected.CloudProviderRateLimitQPSWrite, c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPSWrite)
			}
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucket != c.expected.CloudProviderRateLimitBucket {
				t.Errorf("expected %d, but got %d", c.expected.CloudProviderRateLimitBucket, c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucket)
			}
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucketWrite != c.expected.CloudProviderRateLimitBucketWrite {
				t.Errorf("expected %d, but got %d", c.expected.CloudProviderRateLimitBucketWrite, c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucketWrite)
			}
		})
	}
}

func TestSetOrchestratorDefaultsVMAS(t *testing.T) {
	cs := &ContainerService{
		Properties: &Properties{
			ServicePrincipalProfile: &ServicePrincipalProfile{
				ClientID: "barClientID",
				Secret:   "bazSecret",
			},
			MasterProfile: &MasterProfile{
				Count:               3,
				DNSPrefix:           "myprefix1",
				VMSize:              "Standard_DS2_v2",
				AvailabilityProfile: api.AvailabilitySet,
			},
			OrchestratorProfile: &OrchestratorProfile{
				OrchestratorType:    api.Kubernetes,
				OrchestratorVersion: "1.12.8",
				KubernetesConfig: &KubernetesConfig{
					NetworkPlugin: api.NetworkPluginAzure,
				},
			},
		},
	}

	cs.setOrchestratorDefaults(false, false)
	if cs.Properties.OrchestratorProfile.OrchestratorVersion != "1.12.8" {
		t.Error("setOrchestratorDefaults should not adjust given OrchestratorVersion")
	}

	cs.Properties.OrchestratorProfile.OrchestratorVersion = ""
	cs.setOrchestratorDefaults(false, false)
	if cs.Properties.OrchestratorProfile.OrchestratorVersion == "" {
		t.Error("setOrchestratorDefaults should provide a version if it is not given.")
	}
}

func TestProxyModeDefaults(t *testing.T) {
	// Test that default is what we expect
	mockCS := getMockBaseContainerService("1.10.12")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.MasterProfile.Count = 1
	mockCS.setOrchestratorDefaults(true, true)

	if properties.OrchestratorProfile.KubernetesConfig.ProxyMode != api.DefaultKubeProxyMode {
		t.Fatalf("ProxyMode string not the expected default value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.ProxyMode, api.DefaultKubeProxyMode)
	}

	// Test that default assignment flow doesn't overwrite a user-provided config
	mockCS = getMockBaseContainerService("1.10.12")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ProxyMode = api.KubeProxyModeIPVS
	properties.MasterProfile.Count = 1
	mockCS.setOrchestratorDefaults(true, true)

	if properties.OrchestratorProfile.KubernetesConfig.ProxyMode != api.KubeProxyModeIPVS {
		t.Fatalf("ProxyMode string not the expected default value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.ProxyMode, api.KubeProxyModeIPVS)
	}
}

func TestPreserveNodesProperties(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10.8")
	mockCS.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	if !to.Bool(mockCS.Properties.AgentPoolProfiles[0].PreserveNodesProperties) {
		t.Errorf("expected preserveNodesProperties to be %t instead got %t", true, to.Bool(mockCS.Properties.AgentPoolProfiles[0].PreserveNodesProperties))
	}
}

func TestUbuntu1804Flags(t *testing.T) {
	// Validate --resolv-conf is missing with 16.04 distro and present with 18.04
	cs := CreateMockContainerService("testcluster", "1.10.13", 3, 2, true)
	cs.Properties.MasterProfile.Distro = AKSUbuntu1604
	cs.Properties.AgentPoolProfiles[0].Distro = AKSUbuntu1804
	cs.Properties.AgentPoolProfiles[0].OSType = Linux
	cs.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	km := cs.Properties.MasterProfile.KubernetesConfig.KubeletConfig
	if _, ok := km["--resolv-conf"]; ok {
		t.Fatalf("got unexpected '--resolv-conf' kubelet config value '%s' with Ubuntu 16.04 ",
			km["--resolv-conf"])
	}
	ka := cs.Properties.AgentPoolProfiles[0].KubernetesConfig.KubeletConfig
	if ka["--resolv-conf"] != "/run/systemd/resolve/resolv.conf" {
		t.Fatalf("got unexpected '--resolv-conf' kubelet config value %s with Ubuntu 18.04, the expected value is %s",
			ka["--resolv-conf"], "/run/systemd/resolve/resolv.conf")
	}

	cs = CreateMockContainerService("testcluster", "1.10.13", 3, 2, true)
	cs.Properties.MasterProfile.Distro = Ubuntu1804
	cs.Properties.AgentPoolProfiles[0].Distro = Ubuntu
	cs.Properties.AgentPoolProfiles[0].OSType = Linux
	cs.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	km = cs.Properties.MasterProfile.KubernetesConfig.KubeletConfig
	if km["--resolv-conf"] != "/run/systemd/resolve/resolv.conf" {
		t.Fatalf("got unexpected '--resolv-conf' kubelet config value %s with Ubuntu 18.04, the expected value is %s",
			km["--resolv-conf"], "/run/systemd/resolve/resolv.conf")
	}
	ka = cs.Properties.AgentPoolProfiles[0].KubernetesConfig.KubeletConfig
	if _, ok := ka["--resolv-conf"]; ok {
		t.Fatalf("got unexpected '--resolv-conf' kubelet config value '%s' with Ubuntu 16.04 ",
			ka["--resolv-conf"])
	}

	cs = CreateMockContainerService("testcluster", "1.10.13", 3, 2, true)
	cs.Properties.MasterProfile.Distro = Ubuntu
	cs.Properties.AgentPoolProfiles[0].Distro = ""
	cs.Properties.AgentPoolProfiles[0].OSType = Windows
	cs.SetPropertiesDefaults(api.PropertiesDefaultsParams{
		IsScale:    false,
		IsUpgrade:  false,
		PkiKeySize: helpers.DefaultPkiKeySize,
	})
	km = cs.Properties.MasterProfile.KubernetesConfig.KubeletConfig
	if _, ok := km["--resolv-conf"]; ok {
		t.Fatalf("got unexpected '--resolv-conf' kubelet config value '%s' with Ubuntu 16.04 ",
			km["--resolv-conf"])
	}
	ka = cs.Properties.AgentPoolProfiles[0].KubernetesConfig.KubeletConfig
	if ka["--resolv-conf"] != "\"\"\"\"" {
		t.Fatalf("got unexpected '--resolv-conf' kubelet config value %s with Windows, the expected value is %s",
			ka["--resolv-conf"], "\"\"\"\"")
	}
}

func getMockBaseContainerService(orchestratorVersion string) ContainerService {
	mockAPIProperties := getMockAPIProperties(orchestratorVersion)
	return ContainerService{
		Properties: &mockAPIProperties,
	}
}

// getMockCertificateProfile generates fake certificates.
//
// Adds some fake certficates would bypass the "SetDefaultCerts" part of setting default
// values, which accelerates test case run dramatically. This is useful for test
// cases that are not testing the certificate generation part of the code.
func getMockCertificateProfile() *CertificateProfile {
	return &CertificateProfile{
		CaCertificate:         "FakeCert",
		CaPrivateKey:          "FakePrivateKey",
		ClientCertificate:     "FakeClientCertificate",
		ClientPrivateKey:      "FakeClientPrivateKey",
		APIServerCertificate:  "FakeAPIServerCert",
		APIServerPrivateKey:   "FakeAPIServerPrivateKey",
		EtcdClientCertificate: "FakeEtcdClientCertificate",
		EtcdClientPrivateKey:  "FakeEtcdClientPrivateKey",
		EtcdServerCertificate: "FakeEtcdServerCertificate",
		EtcdServerPrivateKey:  "FakeEtcdServerPrivateKey",
		KubeConfigCertificate: "FakeKubeConfigCertificate",
		KubeConfigPrivateKey:  "FakeKubeConfigPrivateKey",
	}
}

func getMockAPIProperties(orchestratorVersion string) Properties {
	return Properties{
		ProvisioningState: "",
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorVersion: orchestratorVersion,
			KubernetesConfig:    &KubernetesConfig{},
		},
		MasterProfile:      &MasterProfile{},
		CertificateProfile: getMockCertificateProfile(),
		AgentPoolProfiles: []*AgentPoolProfile{
			{},
			{},
			{},
			{},
		}}
}

func getKubernetesConfigWithFeatureGates(featureGates string) *KubernetesConfig {
	return &KubernetesConfig{
		KubeletConfig: map[string]string{"--feature-gates": featureGates},
	}
}

func TestDefaultEnablePodSecurityPolicy(t *testing.T) {
	cases := []struct {
		name     string
		cs       ContainerService
		expected bool
	}{
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.15.0-alpha.1",
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.15.0-beta.1",
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.15.0",
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.cs.setOrchestratorDefaults(false, false)
			if to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.EnablePodSecurityPolicy) != c.expected {
				t.Errorf("expected  %t, but got %t", c.expected, to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.EnablePodSecurityPolicy))
			}
		})
	}
}

func TestDefaultLoadBalancerSKU(t *testing.T) {
	cases := []struct {
		name     string
		cs       ContainerService
		expected string
	}{
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: api.BasicLoadBalancerSku,
		},
		{
			name: "basic",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: "basic",
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: api.BasicLoadBalancerSku,
		},
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: api.BasicLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: api.BasicLoadBalancerSku,
		},
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: "standard",
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: api.StandardLoadBalancerSku,
		},
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: api.StandardLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: api.StandardLoadBalancerSku,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.cs.setLoadBalancerSkuDefaults()
			if c.cs.Properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku != c.expected {
				t.Errorf("expected %s, but got %s", c.expected, c.cs.Properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku)
			}
		})
	}
}

func TestEnableRBAC(t *testing.T) {
	cases := []struct {
		name      string
		cs        ContainerService
		isUpgrade bool
		isScale   bool
		expected  bool
	}{
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType: api.Kubernetes,
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: true,
		},
		{
			name: "1.14 disabled",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: common.GetLatestPatchVersion("1.14", common.GetAllSupportedKubernetesVersions(false, false)),
						KubernetesConfig: &KubernetesConfig{
							EnableRbac: to.BoolPtr(false),
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
		{
			name: "1.14 disabled upgrade",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: common.GetLatestPatchVersion("1.14", common.GetAllSupportedKubernetesVersions(false, false)),
						KubernetesConfig: &KubernetesConfig{
							EnableRbac: to.BoolPtr(false),
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			isUpgrade: true,
			expected:  false,
		},
		{
			name: "1.15",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: common.GetLatestPatchVersion("1.15", common.GetAllSupportedKubernetesVersions(false, false)),
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: true,
		},
		{
			name: "1.15 upgrade",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: common.GetLatestPatchVersion("1.15", common.GetAllSupportedKubernetesVersions(false, false)),
					},
					MasterProfile: &MasterProfile{},
				},
			},
			isUpgrade: true,
			expected:  true,
		},
		{
			name: "1.15 upgrade false--> true override",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: common.GetLatestPatchVersion("1.15", common.GetAllSupportedKubernetesVersions(false, false)),
						KubernetesConfig: &KubernetesConfig{
							EnableRbac: to.BoolPtr(false),
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			isUpgrade: true,
			expected:  true,
		},
		{
			name: "1.16 upgrade false--> true override",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: common.GetLatestPatchVersion("1.16", common.GetAllSupportedKubernetesVersions(false, false)),
						KubernetesConfig: &KubernetesConfig{
							EnableRbac: to.BoolPtr(false),
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			isUpgrade: true,
			expected:  true,
		},
		{
			name: "1.15 upgrade no false--> true override in AKS scenario",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: common.GetLatestPatchVersion("1.15", common.GetAllSupportedKubernetesVersions(false, false)),
						KubernetesConfig: &KubernetesConfig{
							EnableRbac: to.BoolPtr(false),
						},
					},
					HostedMasterProfile: &HostedMasterProfile{
						FQDN: "foo",
					},
				},
			},
			isUpgrade: true,
			expected:  false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.cs.setOrchestratorDefaults(c.isUpgrade, c.isScale)
			if to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.EnableRbac) != c.expected {
				t.Errorf("expected %t, but got %t", c.expected, to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.EnableRbac))
			}
		})
	}
}

func TestDefaultCloudProviderDisableOutboundSNAT(t *testing.T) {
	cases := []struct {
		name     string
		cs       ContainerService
		expected bool
	}{
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
		{
			name: "basic LB",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: api.BasicLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
		{
			name: "basic LB w/ true",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku:                  api.BasicLoadBalancerSku,
							CloudProviderDisableOutboundSNAT: to.BoolPtr(true),
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
		{
			name: "basic LB w/ false",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku:                  api.BasicLoadBalancerSku,
							CloudProviderDisableOutboundSNAT: to.BoolPtr(false),
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
		{
			name: "standard LB w/ true",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku:                  api.StandardLoadBalancerSku,
							CloudProviderDisableOutboundSNAT: to.BoolPtr(true),
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: true,
		},
		{
			name: "standard LB w/ false",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku:                  api.StandardLoadBalancerSku,
							CloudProviderDisableOutboundSNAT: to.BoolPtr(false),
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.cs.setOrchestratorDefaults(false, false)
			if to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderDisableOutboundSNAT) != c.expected {
				t.Errorf("expected %t, but got %t", c.expected, to.Bool(c.cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderDisableOutboundSNAT))
			}
		})
	}
}

func TestSetTelemetryProfileDefaults(t *testing.T) {
	cases := []struct {
		name             string
		telemetryProfile *TelemetryProfile
		expected         *TelemetryProfile
	}{
		{
			name:             "default",
			telemetryProfile: nil,
			expected: &TelemetryProfile{
				ApplicationInsightsKey: api.DefaultApplicationInsightsKey,
			},
		},
		{
			name:             "key not set",
			telemetryProfile: &TelemetryProfile{},
			expected: &TelemetryProfile{
				ApplicationInsightsKey: api.DefaultApplicationInsightsKey,
			},
		},
		{
			name: "key set",
			telemetryProfile: &TelemetryProfile{
				ApplicationInsightsKey: "app-insights-key",
			},
			expected: &TelemetryProfile{
				ApplicationInsightsKey: "app-insights-key",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			props := Properties{
				TelemetryProfile: c.telemetryProfile,
			}

			cs := ContainerService{
				Properties: &props,
			}
			cs.setTelemetryProfileDefaults()

			actual := props.TelemetryProfile
			expected := c.expected

			equal := reflect.DeepEqual(actual, expected)

			if !equal {
				t.Errorf("unexpected diff while conparing Properties.TelemetryProfile: expected: %s, actual: %s", expected, actual)
			}
		})
	}
}

func TestSetPropertiesDefaults(t *testing.T) {
	cases := []struct {
		name   string
		params api.PropertiesDefaultsParams
	}{
		{
			name: "default",
			params: api.PropertiesDefaultsParams{
				IsUpgrade:  false,
				IsScale:    false,
				PkiKeySize: helpers.DefaultPkiKeySize,
			},
		},
		{
			name: "upgrade",
			params: api.PropertiesDefaultsParams{
				IsUpgrade:  true,
				IsScale:    false,
				PkiKeySize: helpers.DefaultPkiKeySize,
			},
		},
		{
			name: "scale",
			params: api.PropertiesDefaultsParams{
				IsUpgrade:  false,
				IsScale:    true,
				PkiKeySize: helpers.DefaultPkiKeySize,
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			cs := getMockBaseContainerService("1.16")

			err := cs.SetPropertiesDefaults(c.params)

			if err != nil {
				t.Errorf("ContainerService.SetPropertiesDefaults returned error: %s", err)
			}

			// verify TelemetryProfile is set
			if cs.Properties.TelemetryProfile == nil {
				t.Errorf("ContainerService.Properties.TelemetryProfile should be set")
			}
		})
	}
}

func TestImageReference(t *testing.T) {
	cases := []struct {
		name                      string
		cs                        ContainerService
		isUpgrade                 bool
		isScale                   bool
		expectedMasterProfile     MasterProfile
		expectedAgentPoolProfiles []AgentPoolProfile
	}{
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType: api.Kubernetes,
					},
					MasterProfile:      &MasterProfile{},
					CertificateProfile: getMockCertificateProfile(),
					AgentPoolProfiles: []*AgentPoolProfile{
						{},
					},
				},
			},
			expectedMasterProfile: MasterProfile{
				Distro:   AKSUbuntu1604,
				ImageRef: nil,
			},
			expectedAgentPoolProfiles: []AgentPoolProfile{
				{
					Distro:   AKSUbuntu1604,
					ImageRef: nil,
				},
			},
		},
		{
			name: "image references",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType: api.Kubernetes,
					},
					MasterProfile: &MasterProfile{
						ImageRef: &ImageReference{
							Name:           "name",
							ResourceGroup:  "resource-group",
							SubscriptionID: "sub-id",
							Gallery:        "gallery",
							Version:        "version",
						},
					},
					CertificateProfile: getMockCertificateProfile(),
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							ImageRef: &ImageReference{
								Name:           "name",
								ResourceGroup:  "resource-group",
								SubscriptionID: "sub-id",
								Gallery:        "gallery",
								Version:        "version",
							},
						},
					},
				},
			},
			expectedMasterProfile: MasterProfile{
				Distro: "",
				ImageRef: &ImageReference{
					Name:           "name",
					ResourceGroup:  "resource-group",
					SubscriptionID: "sub-id",
					Gallery:        "gallery",
					Version:        "version",
				},
			},
			expectedAgentPoolProfiles: []AgentPoolProfile{
				{
					Distro: "",
					ImageRef: &ImageReference{
						Name:           "name",
						ResourceGroup:  "resource-group",
						SubscriptionID: "sub-id",
						Gallery:        "gallery",
						Version:        "version",
					},
				},
			},
		},
		{
			name: "mixed",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType: api.Kubernetes,
					},
					MasterProfile:      &MasterProfile{},
					CertificateProfile: getMockCertificateProfile(),
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							ImageRef: &ImageReference{
								Name:           "name",
								ResourceGroup:  "resource-group",
								SubscriptionID: "sub-id",
								Gallery:        "gallery",
								Version:        "version",
							},
						},
						{},
					},
				},
			},
			expectedMasterProfile: MasterProfile{
				Distro:   AKSUbuntu1604,
				ImageRef: nil,
			},
			expectedAgentPoolProfiles: []AgentPoolProfile{
				{
					Distro: "",
					ImageRef: &ImageReference{
						Name:           "name",
						ResourceGroup:  "resource-group",
						SubscriptionID: "sub-id",
						Gallery:        "gallery",
						Version:        "version",
					},
				},
				{
					Distro:   AKSUbuntu1604,
					ImageRef: nil,
				},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.cs.Properties.OrchestratorProfile.KubernetesConfig = &KubernetesConfig{
				LoadBalancerSku: api.BasicLoadBalancerSku,
			}

			c.cs.SetPropertiesDefaults(api.PropertiesDefaultsParams{
				IsUpgrade:  c.isUpgrade,
				IsScale:    c.isScale,
				PkiKeySize: helpers.DefaultPkiKeySize,
			})
			if c.cs.Properties.MasterProfile.Distro != c.expectedMasterProfile.Distro {
				t.Errorf("expected %s, but got %s", c.expectedMasterProfile.Distro, c.cs.Properties.MasterProfile.Distro)
			}
			if c.expectedMasterProfile.ImageRef == nil {
				if c.cs.Properties.MasterProfile.ImageRef != nil {
					t.Errorf("expected nil, but got an ImageRef")
				}
			} else {
				if c.cs.Properties.MasterProfile.ImageRef == nil {
					t.Errorf("got unexpected nil MasterProfile.ImageRef")
				}
				if c.cs.Properties.MasterProfile.ImageRef.Name != c.expectedMasterProfile.ImageRef.Name {
					t.Errorf("expected %s, but got %s", c.expectedMasterProfile.ImageRef.Name, c.cs.Properties.MasterProfile.ImageRef.Name)
				}
				if c.cs.Properties.MasterProfile.ImageRef.ResourceGroup != c.expectedMasterProfile.ImageRef.ResourceGroup {
					t.Errorf("expected %s, but got %s", c.expectedMasterProfile.ImageRef.ResourceGroup, c.cs.Properties.MasterProfile.ImageRef.ResourceGroup)
				}
				if c.cs.Properties.MasterProfile.ImageRef.SubscriptionID != c.expectedMasterProfile.ImageRef.SubscriptionID {
					t.Errorf("expected %s, but got %s", c.expectedMasterProfile.ImageRef.SubscriptionID, c.cs.Properties.MasterProfile.ImageRef.SubscriptionID)
				}
				if c.cs.Properties.MasterProfile.ImageRef.Gallery != c.expectedMasterProfile.ImageRef.Gallery {
					t.Errorf("expected %s, but got %s", c.expectedMasterProfile.ImageRef.Gallery, c.cs.Properties.MasterProfile.ImageRef.Gallery)
				}
				if c.cs.Properties.MasterProfile.ImageRef.Version != c.expectedMasterProfile.ImageRef.Version {
					t.Errorf("expected %s, but got %s", c.expectedMasterProfile.ImageRef.Version, c.cs.Properties.MasterProfile.ImageRef.Version)
				}
			}
			for i, profile := range c.cs.Properties.AgentPoolProfiles {
				if profile.Distro != c.expectedAgentPoolProfiles[i].Distro {
					t.Errorf("expected %s, but got %s", c.expectedAgentPoolProfiles[i].Distro, profile.Distro)
				}
				if c.expectedAgentPoolProfiles[i].ImageRef == nil {
					if profile.ImageRef != nil {
						t.Errorf("expected nil, but got an ImageRef")
					}
				} else {
					if profile.ImageRef == nil {
						t.Errorf("got unexpected nil MasterProfile.ImageRef")
					}
					if profile.ImageRef.Name != c.expectedAgentPoolProfiles[i].ImageRef.Name {
						t.Errorf("expected %s, but got %s", c.expectedAgentPoolProfiles[i].ImageRef.Name, profile.ImageRef.Name)
					}
					if profile.ImageRef.ResourceGroup != c.expectedAgentPoolProfiles[i].ImageRef.ResourceGroup {
						t.Errorf("expected %s, but got %s", c.expectedAgentPoolProfiles[i].ImageRef.ResourceGroup, profile.ImageRef.ResourceGroup)
					}
					if profile.ImageRef.SubscriptionID != c.expectedAgentPoolProfiles[i].ImageRef.SubscriptionID {
						t.Errorf("expected %s, but got %s", c.expectedAgentPoolProfiles[i].ImageRef.SubscriptionID, profile.ImageRef.SubscriptionID)
					}
					if profile.ImageRef.Gallery != c.expectedAgentPoolProfiles[i].ImageRef.Gallery {
						t.Errorf("expected %s, but got %s", c.expectedAgentPoolProfiles[i].ImageRef.Gallery, profile.ImageRef.Gallery)
					}
					if profile.ImageRef.Version != c.expectedAgentPoolProfiles[i].ImageRef.Version {
						t.Errorf("expected %s, but got %s", c.expectedAgentPoolProfiles[i].ImageRef.Version, profile.ImageRef.Version)
					}
				}
				if to.Bool(profile.SinglePlacementGroup) != true {
					t.Errorf("expected %v, but got %v", true, to.Bool(profile.SinglePlacementGroup))
				}
			}
		})
	}
}

func TestCustomHyperkubeDistro(t *testing.T) {
	cases := []struct {
		name                      string
		cs                        ContainerService
		isUpgrade                 bool
		isScale                   bool
		expectedMasterProfile     MasterProfile
		expectedAgentPoolProfiles []AgentPoolProfile
	}{
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType: api.Kubernetes,
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: api.BasicLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{},
					AgentPoolProfiles: []*AgentPoolProfile{
						{},
					},
				},
			},
			expectedMasterProfile: MasterProfile{
				Distro:   AKSUbuntu1604,
				ImageRef: nil,
			},
			expectedAgentPoolProfiles: []AgentPoolProfile{
				{
					Distro:   AKSUbuntu1604,
					ImageRef: nil,
				},
			},
		},
		{
			name: "custom hyperkube",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType: api.Kubernetes,
						KubernetesConfig: &KubernetesConfig{
							CustomHyperkubeImage: "myimage",
							LoadBalancerSku:      api.BasicLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{},
					AgentPoolProfiles: []*AgentPoolProfile{
						{},
					},
				},
			},
			expectedMasterProfile: MasterProfile{
				Distro: Ubuntu,
			},
			expectedAgentPoolProfiles: []AgentPoolProfile{
				{
					Distro: Ubuntu,
				},
			},
		},
		{
			name: "custom hyperkube w/ distro",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType: api.Kubernetes,
						KubernetesConfig: &KubernetesConfig{
							CustomHyperkubeImage: "myimage",
							LoadBalancerSku:      api.BasicLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{
						Distro: Ubuntu1804,
					},
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							Distro: Ubuntu1804,
						},
					},
				},
			},
			expectedMasterProfile: MasterProfile{
				Distro: Ubuntu1804,
			},
			expectedAgentPoolProfiles: []AgentPoolProfile{
				{
					Distro: Ubuntu1804,
				},
			},
		},
		{
			name: "custom hyperkube w/ mixed distro config",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType: api.Kubernetes,
						KubernetesConfig: &KubernetesConfig{
							CustomHyperkubeImage: "myimage",
							LoadBalancerSku:      api.BasicLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{},
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							Name:   "pool1",
							Distro: Ubuntu1804,
						},
						{
							Name: "pool2",
						},
					},
				},
			},
			expectedMasterProfile: MasterProfile{
				Distro: Ubuntu,
			},
			expectedAgentPoolProfiles: []AgentPoolProfile{
				{
					Distro: Ubuntu1804,
				},
				{
					Distro: Ubuntu,
				},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.cs.SetPropertiesDefaults(api.PropertiesDefaultsParams{
				IsUpgrade:  c.isUpgrade,
				IsScale:    c.isScale,
				PkiKeySize: helpers.DefaultPkiKeySize,
			})
			if c.cs.Properties.MasterProfile.Distro != c.expectedMasterProfile.Distro {
				t.Errorf("expected %s, but got %s", c.expectedMasterProfile.Distro, c.cs.Properties.MasterProfile.Distro)
			}
			for i, profile := range c.cs.Properties.AgentPoolProfiles {
				if profile.Distro != c.expectedAgentPoolProfiles[i].Distro {
					t.Errorf("expected %s, but got %s", c.expectedAgentPoolProfiles[i].Distro, profile.Distro)
				}
				if to.Bool(profile.SinglePlacementGroup) != true {
					t.Errorf("expected %v, but got %v", true, to.Bool(profile.SinglePlacementGroup))
				}
			}
		})
	}
}

func TestDefaultIPAddressCount(t *testing.T) {
	cases := []struct {
		name           string
		cs             ContainerService
		expectedMaster int
		expectedPool0  int
		expectedPool1  int
	}{
		{
			name: "kubenet",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							NetworkPlugin:   api.NetworkPluginKubenet,
							LoadBalancerSku: api.StandardLoadBalancerSku,
						},
					},
					MasterProfile:      &MasterProfile{},
					CertificateProfile: getMockCertificateProfile(),
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							Name: "pool1",
						},
						{
							Name: "pool2",
						},
					},
				},
			},
			expectedMaster: 1,
			expectedPool0:  1,
			expectedPool1:  1,
		},
		{
			name: "Azure CNI",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							NetworkPlugin:   api.NetworkPluginAzure,
							LoadBalancerSku: api.StandardLoadBalancerSku,
						},
					},
					MasterProfile:      &MasterProfile{},
					CertificateProfile: getMockCertificateProfile(),
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							Name: "pool1",
						},
						{
							Name: "pool2",
						},
					},
				},
			},
			expectedMaster: api.DefaultKubernetesMaxPodsVNETIntegrated + 1,
			expectedPool0:  api.DefaultKubernetesMaxPodsVNETIntegrated + 1,
			expectedPool1:  api.DefaultKubernetesMaxPodsVNETIntegrated + 1,
		},
		{
			name: "Azure CNI + custom IPAddressCount",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							NetworkPlugin:   api.NetworkPluginAzure,
							LoadBalancerSku: api.StandardLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{
						IPAddressCount: 24,
					},
					CertificateProfile: getMockCertificateProfile(),
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							Name:           "pool1",
							IPAddressCount: 24,
						},
						{
							Name:           "pool2",
							IPAddressCount: 24,
						},
					},
				},
			},
			expectedMaster: 24,
			expectedPool0:  24,
			expectedPool1:  24,
		},
		{
			name: "kubenet + custom IPAddressCount",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							NetworkPlugin:   api.NetworkPluginKubenet,
							LoadBalancerSku: api.StandardLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{
						IPAddressCount: 24,
					},
					CertificateProfile: getMockCertificateProfile(),
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							Name:           "pool1",
							IPAddressCount: 24,
						},
						{
							Name:           "pool2",
							IPAddressCount: 24,
						},
					},
				},
			},
			expectedMaster: 24,
			expectedPool0:  24,
			expectedPool1:  24,
		},
		{
			name: "Azure CNI + mixed config",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							NetworkPlugin:   api.NetworkPluginAzure,
							LoadBalancerSku: api.StandardLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{
						KubernetesConfig: &KubernetesConfig{
							KubeletConfig: map[string]string{
								"--max-pods": "24",
							},
						},
					},
					CertificateProfile: getMockCertificateProfile(),
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							Name: "pool1",
							KubernetesConfig: &KubernetesConfig{
								KubeletConfig: map[string]string{
									"--max-pods": "128",
								},
							},
						},
						{
							Name: "pool2",
						},
					},
				},
			},
			expectedMaster: 25,
			expectedPool0:  129,
			expectedPool1:  api.DefaultKubernetesMaxPodsVNETIntegrated + 1,
		},
		{
			name: "kubenet + mixed config",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							NetworkPlugin:   api.NetworkPluginKubenet,
							LoadBalancerSku: api.StandardLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{
						KubernetesConfig: &KubernetesConfig{
							KubeletConfig: map[string]string{
								"--max-pods": "24",
							},
						},
					},
					CertificateProfile: getMockCertificateProfile(),
					AgentPoolProfiles: []*AgentPoolProfile{
						{
							Name: "pool1",
							KubernetesConfig: &KubernetesConfig{
								KubeletConfig: map[string]string{
									"--max-pods": "128",
								},
							},
						},
						{
							Name: "pool2",
						},
					},
				},
			},
			expectedMaster: 1,
			expectedPool0:  1,
			expectedPool1:  1,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.cs.SetPropertiesDefaults(api.PropertiesDefaultsParams{
				IsScale:    false,
				IsUpgrade:  true,
				PkiKeySize: helpers.DefaultPkiKeySize,
			})
			if c.cs.Properties.MasterProfile.IPAddressCount != c.expectedMaster {
				t.Errorf("expected %d, but got %d", c.expectedMaster, c.cs.Properties.MasterProfile.IPAddressCount)
			}
			if c.cs.Properties.AgentPoolProfiles[0].IPAddressCount != c.expectedPool0 {
				t.Errorf("expected %d, but got %d", c.expectedPool0, c.cs.Properties.AgentPoolProfiles[0].IPAddressCount)
			}
			if c.cs.Properties.AgentPoolProfiles[1].IPAddressCount != c.expectedPool1 {
				t.Errorf("expected %d, but got %d", c.expectedPool1, c.cs.Properties.AgentPoolProfiles[1].IPAddressCount)
			}
			if to.Bool(c.cs.Properties.AgentPoolProfiles[1].SinglePlacementGroup) != false {
				t.Errorf("expected %v, but got %v", false, to.Bool(c.cs.Properties.AgentPoolProfiles[1].SinglePlacementGroup))
			}
		})
	}
}
