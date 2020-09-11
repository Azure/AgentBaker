// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
)

var (
	azurePublicCloudSpec = &AzureEnvironmentSpecConfig{
		CloudName: AzurePublicCloud,
		//DockerSpecConfig specify the docker engine download repo
		DockerSpecConfig: DefaultDockerSpecConfig,
		//KubernetesSpecConfig is the default kubernetes container image url.
		KubernetesSpecConfig: DefaultKubernetesSpecConfig,

		EndpointConfig: AzureEndpointConfig{
			ResourceManagerVMDNSSuffix: "cloudapp.azure.com",
		},
	}
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

func getFakeAddons(defaultAddonMap map[string]string, customImage string) []KubernetesAddon {
	var fakeCustomAddons []KubernetesAddon
	for addonName := range defaultAddonMap {
		containerName := addonName
		if addonName == ContainerMonitoringAddonName {
			containerName = "omsagent"
		}
		if addonName == CalicoAddonName {
			containerName = "calico-typha"
		}
		if addonName == AADPodIdentityAddonName {
			containerName = "nmi"
		}
		if addonName == KubeDNSAddonName {
			containerName = "kubedns"
		}
		if addonName == AntreaAddonName {
			containerName = AntreaControllerContainerName
		}
		if addonName == FlannelAddonName {
			containerName = KubeFlannelContainerName
		}
		customAddon := KubernetesAddon{
			Name:    addonName,
			Enabled: to.BoolPtr(true),
			Containers: []KubernetesContainerSpec{
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
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled = nil
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows = nil

	// In upgrade scenario, nil AcceleratedNetworkingEnabled should always render as false (i.e., we never turn on this feature on an existing vm that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled) {
		t.Errorf("expected nil acceleratedNetworkingEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled))
	}
	// In upgrade scenario, nil AcceleratedNetworkingEnabledWindows should always render as false (i.e., we never turn on this feature on an existing vm that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows) {
		t.Errorf("expected nil acceleratedNetworkingEnabledWindows to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled = nil
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows = nil

	// In scale scenario, nil AcceleratedNetworkingEnabled should always render as false (i.e., we never turn on this feature on an existing agent pool / VMSS that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled) {
		t.Errorf("expected nil acceleratedNetworkingEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled))
	}
	// In scale scenario, nil AcceleratedNetworkingEnabledWindows should always render as false (i.e., we never turn on this feature on an existing VM that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows) {
		t.Errorf("expected nil acceleratedNetworkingEnabledWindows to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabled = nil
	mockCS.Properties.AgentPoolProfiles[0].VMSize = "Standard_D666_v2"
	mockCS.Properties.AgentPoolProfiles[0].AcceleratedNetworkingEnabledWindows = nil
	mockCS.Properties.AgentPoolProfiles[0].VMSize = "Standard_D666_v2"

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
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = nil

	// In upgrade scenario, nil AcceleratedNetworkingEnabled should always render as false (i.e., we never turn on this feature on an existing vm that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) {
		t.Errorf("expected nil VMSSOverProvisioningEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = nil

	// In scale scenario, nil VMSSOverProvisioningEnabled should always render as false (i.e., we never turn on this feature on an existing agent pool / VMSS that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) {
		t.Errorf("expected nil VMSSOverProvisioningEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = nil

	// In create scenario, nil VMSSOverProvisioningEnabled should be the defaults
	vmssOverProvisioningEnabled := DefaultVMSSOverProvisioningEnabled
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) != vmssOverProvisioningEnabled {
		t.Errorf("expected default VMSSOverProvisioningEnabled to be %t, instead got %t", vmssOverProvisioningEnabled, to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = to.BoolPtr(true)

	// In create scenario with explicit true, VMSSOverProvisioningEnabled should be true
	if !to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) {
		t.Errorf("expected VMSSOverProvisioningEnabled to be true, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AvailabilityProfile = VirtualMachineScaleSets
	mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled = to.BoolPtr(false)

	// In create scenario with explicit false, VMSSOverProvisioningEnabled should be false
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled) {
		t.Errorf("expected VMSSOverProvisioningEnabled to be false, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].VMSSOverProvisioningEnabled))
	}
}

func TestAuditDEnabled(t *testing.T) {
	mockCS := getMockBaseContainerService("1.12.7")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	isUpgrade := true
	mockCS.setAgentProfileDefaults(isUpgrade, false)

	// In upgrade scenario, nil AuditDEnabled should always render as false (i.e., we never turn on this feature on an existing vm that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled) {
		t.Errorf("expected nil AuditDEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled))
	}

	mockCS = getMockBaseContainerService("1.12.7")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	isScale := true
	mockCS.setAgentProfileDefaults(false, isScale)

	// In scale scenario, nil AuditDEnabled should always render as false (i.e., we never turn on this feature on an existing agent pool / vms that didn't have it before)
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled) {
		t.Errorf("expected nil AuditDEnabled to be false after upgrade, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled))
	}

	mockCS = getMockBaseContainerService("1.12.7")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.setAgentProfileDefaults(false, false)

	// In create scenario, nil AuditDEnabled should be the defaults
	auditDEnabledEnabled := DefaultAuditDEnabled
	if to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled) != auditDEnabledEnabled {
		t.Errorf("expected default AuditDEnabled to be %t, instead got %t", auditDEnabledEnabled, to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled = to.BoolPtr(true)
	mockCS.setAgentProfileDefaults(false, false)

	// In create scenario with explicit true, AuditDEnabled should be true
	if !to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled) {
		t.Errorf("expected AuditDEnabled to be true, instead got %t", to.Bool(mockCS.Properties.AgentPoolProfiles[0].AuditDEnabled))
	}

	mockCS = getMockBaseContainerService("1.10.8")
	mockCS.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
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
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 1
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB != DefaultEtcdDiskSize {
		t.Fatalf("EtcdDiskSizeGB did not have the expected size, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB, DefaultEtcdDiskSize)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 5
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB != DefaultEtcdDiskSizeGT3Nodes {
		t.Fatalf("EtcdDiskSizeGB did not have the expected size, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB, DefaultEtcdDiskSizeGT3Nodes)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 5
	properties.AgentPoolProfiles[0].Count = 6
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB != DefaultEtcdDiskSizeGT10Nodes {
		t.Fatalf("EtcdDiskSizeGB did not have the expected size, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB, DefaultEtcdDiskSizeGT10Nodes)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 5
	properties.AgentPoolProfiles[0].Count = 16
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB != DefaultEtcdDiskSizeGT20Nodes {
		t.Fatalf("EtcdDiskSizeGB did not have the expected size, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB, DefaultEtcdDiskSizeGT20Nodes)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 5
	properties.AgentPoolProfiles[0].Count = 50
	customEtcdDiskSize := "512"
	properties.OrchestratorProfile.KubernetesConfig.EtcdDiskSizeGB = customEtcdDiskSize
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
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
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyCalico
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != NetworkPluginKubenet {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, NetworkPluginKubenet)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyCilium
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != NetworkPluginCilium {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, NetworkPluginCilium)
	}

	mockCS = getMockBaseContainerService("1.15.7")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyAntrea
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != NetworkPluginAntrea {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, NetworkPluginAntrea)
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyAzure
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != NetworkPluginAzure {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, NetworkPluginAzure)
	}
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy != "" {
		t.Fatalf("NetworkPolicy did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy, "")
	}

	mockCS = getMockBaseContainerService("1.8.10")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyNone
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != NetworkPluginKubenet {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, NetworkPluginKubenet)
	}
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy != "" {
		t.Fatalf("NetworkPolicy did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy, "")
	}
}

func TestNetworkPluginDefaults(t *testing.T) {
	mockCS := getMockBaseContainerService("1.15.7")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != DefaultNetworkPlugin {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, DefaultNetworkPlugin)
	}

	mockCS = getMockBaseContainerService("1.15.7")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.Addons = []KubernetesAddon{
		{
			Name:    FlannelAddonName,
			Enabled: to.BoolPtr(true),
		},
	}
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin != NetworkPluginFlannel {
		t.Fatalf("NetworkPlugin did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin, NetworkPluginFlannel)
	}
}

func TestContainerRuntime(t *testing.T) {

	for _, mobyVersion := range []string{"3.0.1", "3.0.3", "3.0.4", "3.0.5", "3.0.6", "3.0.7", "3.0.8", "3.0.10"} {
		mockCS := getMockBaseContainerService("1.10.13")
		properties := mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.MobyVersion = mobyVersion
		mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != Docker {
			t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, Docker)
		}
		if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != DefaultMobyVersion {
			t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.MobyVersion, DefaultMobyVersion)
		}
		if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != "" {
			t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, "")
		}

		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.MobyVersion = mobyVersion
		mockCS.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != Docker {
			t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, Docker)
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
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != Docker {
		t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, Docker)
	}
	if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != DefaultMobyVersion {
		t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.MobyVersion, DefaultMobyVersion)
	}
	if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != "" {
		t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, "")
	}

	mockCS = getMockBaseContainerService("1.10.13")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime = Containerd
	mockCS.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != Containerd {
		t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, Containerd)
	}
	if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != "" {
		t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.MobyVersion, "")
	}
	if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != DefaultContainerdVersion {
		t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, DefaultContainerdVersion)
	}

	mockCS = getMockBaseContainerService("1.10.13")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime = KataContainers
	mockCS.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
	if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != KataContainers {
		t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, KataContainers)
	}
	if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != "" {
		t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.MobyVersion, "")
	}
	if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != DefaultContainerdVersion {
		t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
			properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, DefaultContainerdVersion)
	}

	for _, containerdVersion := range []string{"1.1.2", "1.1.4", "1.1.5"} {

		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime = Containerd
		properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion = containerdVersion
		mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != Containerd {
			t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, Containerd)
		}
		if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != "" {
			t.Fatalf("MobyVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.MobyVersion, "")
		}
		if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != DefaultContainerdVersion {
			t.Fatalf("Containerd did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion, DefaultContainerdVersion)
		}

		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime = Containerd
		properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion = containerdVersion
		mockCS.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime != Containerd {
			t.Fatalf("ContainerRuntime did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.ContainerRuntime, Containerd)
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
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(true, false, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != DefaultEtcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, DefaultEtcdVersion)
		}

		// Create scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != DefaultEtcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, DefaultEtcdVersion)
		}

		// Scale scenario should always accept the provided value
		// This sort of artificial (upgrade scenario should always have value), but strictly speaking this is what we want to do
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, true, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != DefaultEtcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, DefaultEtcdVersion)
		}
	}

	// These versions are all less than or equal to default
	for _, etcdVersion := range []string{"2.2.5", "3.2.24", DefaultEtcdVersion} {
		// Upgrade scenario should always upgrade to newer, default etcd version
		mockCS := getMockBaseContainerService("1.10.13")
		properties := mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(true, false, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != DefaultEtcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, DefaultEtcdVersion)
		}

		// Create scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != etcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, etcdVersion)
		}

		// Scale scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, true, azurePublicCloudSpec)
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
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(true, false, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != etcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, etcdVersion)
		}

		// Create scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != etcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, etcdVersion)
		}

		// Scale scenario should always accept the provided value
		mockCS = getMockBaseContainerService("1.10.13")
		properties = mockCS.Properties
		properties.OrchestratorProfile.OrchestratorType = Kubernetes
		properties.OrchestratorProfile.KubernetesConfig.EtcdVersion = etcdVersion
		mockCS.setOrchestratorDefaults(false, true, azurePublicCloudSpec)
		if properties.OrchestratorProfile.KubernetesConfig.EtcdVersion != etcdVersion {
			t.Fatalf("EtcdVersion did not have the expected value, got %s, expected %s",
				properties.OrchestratorProfile.KubernetesConfig.EtcdVersion, etcdVersion)
		}
	}
}

func TestAgentPoolProfile(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 1
	if properties.AgentPoolProfiles[0].ScaleSetPriority != "" {
		t.Fatalf("AgentPoolProfiles[0].ScaleSetPriority did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].ScaleSetPriority, "")
	}
	if properties.AgentPoolProfiles[0].ScaleSetEvictionPolicy != "" {
		t.Fatalf("AgentPoolProfiles[0].ScaleSetEvictionPolicy did not have the expected configuration, got %s, expected %s",
			properties.AgentPoolProfiles[0].ScaleSetEvictionPolicy, "")
	}
	properties.AgentPoolProfiles[0].ScaleSetPriority = ScaleSetPrioritySpot
	properties.AgentPoolProfiles[0].SpotMaxPrice = to.Float64Ptr(float64(88))
	if *properties.AgentPoolProfiles[0].SpotMaxPrice != float64(88) {
		t.Fatalf("AgentPoolProfile[0].SpotMaxPrice did not have the expected value, got %g, expected %g",
			*properties.AgentPoolProfiles[0].SpotMaxPrice, float64(88))
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
				WindowsPublisher:      AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            AKSWindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          AKSWindowsServer2019OSImageConfig.ImageVersion,
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
				WindowsPublisher: AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       AKSWindowsServer2019OSImageConfig.ImageSku,
			},
			WindowsProfile{
				WindowsPublisher:      AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            AKSWindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          AKSWindowsServer2019OSImageConfig.ImageVersion,
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
				WindowsPublisher: AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
			},
			WindowsProfile{
				WindowsPublisher:      AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          AKSWindowsServer2019OSImageConfig.ImageOffer,
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
				WindowsPublisher: AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       AKSWindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            AKSWindowsServer2019OSImageConfig.ImageSku,
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
				WindowsPublisher: WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       WindowsServer2019OSImageConfig.ImageSku,
			},
			WindowsProfile{
				WindowsPublisher:      WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          WindowsServer2019OSImageConfig.ImageVersion,
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
				WindowsPublisher: WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
			},
			WindowsProfile{
				WindowsPublisher:      WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          WindowsServer2019OSImageConfig.ImageOffer,
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
				WindowsPublisher: WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     WindowsServer2019OSImageConfig.ImageOffer,
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            WindowsServer2019OSImageConfig.ImageSku,
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
				WindowsPublisher: WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            WindowsServer2019OSImageConfig.ImageSku,
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
				WindowsPublisher: WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:     WindowsServer2019OSImageConfig.ImageVersion,
			},
			WindowsProfile{
				WindowsPublisher:      WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          WindowsServer2019OSImageConfig.ImageVersion,
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
				WindowsPublisher: AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "",
				ImageVersion:     "",
			},
			WindowsProfile{
				WindowsPublisher:      AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            AKSWindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          AKSWindowsServer2019OSImageConfig.ImageVersion,
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
				WindowsPublisher: AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          AKSWindowsServer2019OSImageConfig.ImageOffer,
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
				WindowsPublisher: WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "",
				ImageVersion:     "",
			},
			WindowsProfile{
				WindowsPublisher:      WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:            WindowsServer2019OSImageConfig.ImageSku,
				ImageVersion:          WindowsServer2019OSImageConfig.ImageVersion,
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
				WindowsPublisher: WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          WindowsServer2019OSImageConfig.ImageOffer,
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
				WindowsPublisher: WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "override",
				ImageVersion:     "",
			},
			WindowsProfile{
				WindowsPublisher:      WindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          AKSWindowsServer2019OSImageConfig.ImageOffer,
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
				WindowsPublisher: AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     WindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "",
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          WindowsServer2019OSImageConfig.ImageOffer,
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
				WindowsPublisher: AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:     AKSWindowsServer2019OSImageConfig.ImageOffer,
				WindowsSku:       "",
				ImageVersion:     "override",
			},
			WindowsProfile{
				WindowsPublisher:      AKSWindowsServer2019OSImageConfig.ImagePublisher,
				WindowsOffer:          AKSWindowsServer2019OSImageConfig.ImageOffer,
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

func TestAzureCNIVersionString(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10.3")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 1
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginAzure
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)

	if properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion != AzureCniPluginVerLinux {
		t.Fatalf("Azure CNI Version string not the expected value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion, AzureCniPluginVerLinux)
	}

	mockCS = getMockBaseContainerService("1.10.3")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 1
	properties.AgentPoolProfiles[0].OSType = Windows
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginAzure
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)

	if properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion != AzureCniPluginVerWindows {
		t.Fatalf("Azure CNI Version string not the expected value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion, AzureCniPluginVerWindows)
	}

	mockCS = getMockBaseContainerService("1.10.3")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 1
	properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)

	if properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion != "" {
		t.Fatalf("Azure CNI Version string not the expected value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.AzureCNIVersion, "")
	}
}

func TestEnableAggregatedAPIs(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10.3")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.EnableRbac = to.BoolPtr(false)
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)

	if properties.OrchestratorProfile.KubernetesConfig.EnableAggregatedAPIs {
		t.Fatalf("got unexpected EnableAggregatedAPIs config value for EnableRbac=false: %t",
			properties.OrchestratorProfile.KubernetesConfig.EnableAggregatedAPIs)
	}

	mockCS = getMockBaseContainerService("1.10.3")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.EnableRbac = to.BoolPtr(true)
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)

	if !properties.OrchestratorProfile.KubernetesConfig.EnableAggregatedAPIs {
		t.Fatalf("got unexpected EnableAggregatedAPIs config value for EnableRbac=true: %t",
			properties.OrchestratorProfile.KubernetesConfig.EnableAggregatedAPIs)
	}
}

func TestCloudControllerManagerEnabled(t *testing.T) {
	// test that 1.16 defaults to false
	cs := CreateMockContainerService("testcluster", "1.16.1", 3, 2, false)
	cs.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
	if cs.Properties.OrchestratorProfile.KubernetesConfig.UseCloudControllerManager == to.BoolPtr(true) {
		t.Fatal("expected UseCloudControllerManager to default to false")
	}

	// test that 1.17 defaults to false
	cs = CreateMockContainerService("testcluster", "1.17.0", 3, 2, false)
	cs.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
	if cs.Properties.OrchestratorProfile.KubernetesConfig.UseCloudControllerManager == to.BoolPtr(true) {
		t.Fatal("expected UseCloudControllerManager to default to false")
	}
}

func TestDefaultCloudProvider(t *testing.T) {
	mockCS := getMockBaseContainerService("1.10.3")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)

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
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff = to.BoolPtr(false)
	properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimit = to.BoolPtr(false)
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)

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
						OrchestratorType: Kubernetes,
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: KubernetesConfig{
				CloudProviderBackoffMode:          "v1",
				CloudProviderBackoff:              to.BoolPtr(false),
				CloudProviderBackoffRetries:       DefaultKubernetesCloudProviderBackoffRetries,
				CloudProviderBackoffJitter:        DefaultKubernetesCloudProviderBackoffJitter,
				CloudProviderBackoffDuration:      DefaultKubernetesCloudProviderBackoffDuration,
				CloudProviderBackoffExponent:      DefaultKubernetesCloudProviderBackoffExponent,
				CloudProviderRateLimit:            to.BoolPtr(DefaultKubernetesCloudProviderRateLimit),
				CloudProviderRateLimitQPS:         DefaultKubernetesCloudProviderRateLimitQPS,
				CloudProviderRateLimitQPSWrite:    DefaultKubernetesCloudProviderRateLimitQPSWrite,
				CloudProviderRateLimitBucket:      DefaultKubernetesCloudProviderRateLimitBucket,
				CloudProviderRateLimitBucketWrite: DefaultKubernetesCloudProviderRateLimitBucketWrite,
			},
		},
		{
			name: "Kubernetes 1.14.0",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: KubernetesConfig{
				CloudProviderBackoffMode:          "v2",
				CloudProviderBackoff:              to.BoolPtr(true),
				CloudProviderBackoffRetries:       DefaultKubernetesCloudProviderBackoffRetries,
				CloudProviderBackoffJitter:        0,
				CloudProviderBackoffDuration:      DefaultKubernetesCloudProviderBackoffDuration,
				CloudProviderBackoffExponent:      0,
				CloudProviderRateLimit:            to.BoolPtr(DefaultKubernetesCloudProviderRateLimit),
				CloudProviderRateLimitQPS:         DefaultKubernetesCloudProviderRateLimitQPS,
				CloudProviderRateLimitQPSWrite:    DefaultKubernetesCloudProviderRateLimitQPSWrite,
				CloudProviderRateLimitBucket:      DefaultKubernetesCloudProviderRateLimitBucket,
				CloudProviderRateLimitBucketWrite: DefaultKubernetesCloudProviderRateLimitBucketWrite,
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.cs.setOrchestratorDefaults(c.isUpgrade, c.isScale, azurePublicCloudSpec)
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
				AvailabilityProfile: AvailabilitySet,
			},
			OrchestratorProfile: &OrchestratorProfile{
				OrchestratorType:    Kubernetes,
				OrchestratorVersion: "1.12.8",
				KubernetesConfig: &KubernetesConfig{
					NetworkPlugin: NetworkPluginAzure,
				},
			},
		},
	}

	cs.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
	if cs.Properties.OrchestratorProfile.OrchestratorVersion != "1.12.8" {
		t.Error("setOrchestratorDefaults should not adjust given OrchestratorVersion")
	}

	cs.Properties.OrchestratorProfile.OrchestratorVersion = ""
	cs.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
	if cs.Properties.OrchestratorProfile.OrchestratorVersion == "" {
		t.Error("setOrchestratorDefaults should provide a version if it is not given.")
	}
}

func TestProxyModeDefaults(t *testing.T) {
	// Test that default is what we expect
	mockCS := getMockBaseContainerService("1.10.12")
	properties := mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.MasterProfile.Count = 1
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)

	if properties.OrchestratorProfile.KubernetesConfig.ProxyMode != DefaultKubeProxyMode {
		t.Fatalf("ProxyMode string not the expected default value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.ProxyMode, DefaultKubeProxyMode)
	}

	// Test that default assignment flow doesn't overwrite a user-provided config
	mockCS = getMockBaseContainerService("1.10.12")
	properties = mockCS.Properties
	properties.OrchestratorProfile.OrchestratorType = Kubernetes
	properties.OrchestratorProfile.KubernetesConfig.ProxyMode = KubeProxyModeIPVS
	properties.MasterProfile.Count = 1
	mockCS.setOrchestratorDefaults(true, true, azurePublicCloudSpec)

	if properties.OrchestratorProfile.KubernetesConfig.ProxyMode != KubeProxyModeIPVS {
		t.Fatalf("ProxyMode string not the expected default value, got %s, expected %s", properties.OrchestratorProfile.KubernetesConfig.ProxyMode, KubeProxyModeIPVS)
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
						OrchestratorType:    Kubernetes,
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
						OrchestratorType:    Kubernetes,
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
						OrchestratorType:    Kubernetes,
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
						OrchestratorType:    Kubernetes,
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
			c.cs.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: BasicLoadBalancerSku,
		},
		{
			name: "basic",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: "basic",
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: BasicLoadBalancerSku,
		},
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: BasicLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: BasicLoadBalancerSku,
		},
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: "standard",
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: StandardLoadBalancerSku,
		},
		{
			name: "default",
			cs: ContainerService{
				Properties: &Properties{
					OrchestratorProfile: &OrchestratorProfile{
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: StandardLoadBalancerSku,
						},
					},
					MasterProfile: &MasterProfile{},
				},
			},
			expected: StandardLoadBalancerSku,
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
						OrchestratorType: Kubernetes,
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: GetLatestPatchVersion("1.14", GetAllSupportedKubernetesVersions(false, false)),
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: GetLatestPatchVersion("1.14", GetAllSupportedKubernetesVersions(false, false)),
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: GetLatestPatchVersion("1.15", GetAllSupportedKubernetesVersions(false, false)),
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: GetLatestPatchVersion("1.15", GetAllSupportedKubernetesVersions(false, false)),
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: GetLatestPatchVersion("1.15", GetAllSupportedKubernetesVersions(false, false)),
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: GetLatestPatchVersion("1.16", GetAllSupportedKubernetesVersions(false, false)),
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: GetLatestPatchVersion("1.15", GetAllSupportedKubernetesVersions(false, false)),
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
			c.cs.setOrchestratorDefaults(c.isUpgrade, c.isScale, azurePublicCloudSpec)
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
						OrchestratorType:    Kubernetes,
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku: BasicLoadBalancerSku,
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku:                  BasicLoadBalancerSku,
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku:                  BasicLoadBalancerSku,
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku:                  StandardLoadBalancerSku,
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
						OrchestratorType:    Kubernetes,
						OrchestratorVersion: "1.14.0",
						KubernetesConfig: &KubernetesConfig{
							LoadBalancerSku:                  StandardLoadBalancerSku,
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
			c.cs.setOrchestratorDefaults(false, false, azurePublicCloudSpec)
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
				ApplicationInsightsKey: DefaultApplicationInsightsKey,
			},
		},
		{
			name:             "key not set",
			telemetryProfile: &TelemetryProfile{},
			expected: &TelemetryProfile{
				ApplicationInsightsKey: DefaultApplicationInsightsKey,
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
