// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"testing"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/go-autorest/autorest/to"
)

func TestGetClusterAutoscalerNodesConfig(t *testing.T) {
	specConfig := api.AzureCloudSpecEnvMap["AzurePublicCloud"].KubernetesSpecConfig
	cases := []struct {
		name                string
		addon               api.KubernetesAddon
		cs                  *ContainerService
		expectedNodesConfig string
	}{
		{
			name: "1 pool",
			addon: api.KubernetesAddon{
				Name:    common.ClusterAutoscalerAddonName,
				Enabled: to.BoolPtr(true),
				Mode:    api.AddonModeEnsureExists,
				Config: map[string]string{
					"scan-interval": "1m",
					"v":             "3",
				},
				Containers: []api.KubernetesContainerSpec{
					{
						Name:           common.ClusterAutoscalerAddonName,
						CPURequests:    "100m",
						MemoryRequests: "300Mi",
						CPULimits:      "100m",
						MemoryLimits:   "300Mi",
						Image:          specConfig.KubernetesImageBase + api.K8sComponentsByVersionMap["1.15.4"][common.ClusterAutoscalerAddonName],
					},
				},
				Pools: []api.AddonNodePoolsConfig{
					{
						Name: "pool1",
						Config: map[string]string{
							"min-nodes": "1",
							"max-nodes": "10",
						},
					},
				},
			},
			cs: &ContainerService{
				Properties: &api.Properties{
					OrchestratorProfile: &api.OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.15.4",
						KubernetesConfig: &api.KubernetesConfig{
							NetworkPlugin: api.NetworkPluginAzure,
							Addons: []api.KubernetesAddon{
								{
									Name:    common.ClusterAutoscalerAddonName,
									Enabled: to.BoolPtr(true),
								},
							},
							UseManagedIdentity: true,
						},
					},
					AgentPoolProfiles: []*api.AgentPoolProfile{
						{
							Name:                "pool1",
							Count:               1,
							AvailabilityProfile: api.VirtualMachineScaleSets,
						},
					},
				},
			},
			expectedNodesConfig: "        - --nodes=1:10:k8s-pool1-49584119-vmss",
		},
		{
			name: "multiple pools",
			addon: api.KubernetesAddon{
				Name:    common.ClusterAutoscalerAddonName,
				Enabled: to.BoolPtr(true),
				Mode:    api.AddonModeEnsureExists,
				Config: map[string]string{
					"scan-interval": "1m",
					"v":             "3",
				},
				Containers: []api.KubernetesContainerSpec{
					{
						Name:           common.ClusterAutoscalerAddonName,
						CPURequests:    "100m",
						MemoryRequests: "300Mi",
						CPULimits:      "100m",
						MemoryLimits:   "300Mi",
						Image:          specConfig.KubernetesImageBase + api.K8sComponentsByVersionMap["1.15.4"][common.ClusterAutoscalerAddonName],
					},
				},
				Pools: []api.AddonNodePoolsConfig{
					{
						Name: "pool1",
						Config: map[string]string{
							"min-nodes": "1",
							"max-nodes": "10",
						},
					},
					{
						Name: "pool2",
						Config: map[string]string{
							"min-nodes": "1",
							"max-nodes": "10",
						},
					},
				},
			},
			cs: &ContainerService{
				Properties: &api.Properties{
					OrchestratorProfile: &api.OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.15.4",
						KubernetesConfig: &api.KubernetesConfig{
							NetworkPlugin: api.NetworkPluginAzure,
							Addons: []api.KubernetesAddon{
								{
									Name:    common.ClusterAutoscalerAddonName,
									Enabled: to.BoolPtr(true),
								},
							},
							UseManagedIdentity: true,
						},
					},
					AgentPoolProfiles: []*api.AgentPoolProfile{
						{
							Name:                "pool1",
							Count:               1,
							AvailabilityProfile: api.VirtualMachineScaleSets,
						},
						{
							Name:                "pool2",
							Count:               1,
							AvailabilityProfile: api.VirtualMachineScaleSets,
						},
					},
				},
			},
			expectedNodesConfig: "        - --nodes=1:10:k8s-pool1-49584119-vmss\n        - --nodes=1:10:k8s-pool2-49584119-vmss",
		},
		{
			name: "no pools",
			addon: api.KubernetesAddon{
				Name:    common.ClusterAutoscalerAddonName,
				Enabled: to.BoolPtr(true),
				Mode:    api.AddonModeEnsureExists,
				Config: map[string]string{
					"scan-interval": "1m",
					"v":             "3",
				},
				Containers: []api.KubernetesContainerSpec{
					{
						Name:           common.ClusterAutoscalerAddonName,
						CPURequests:    "100m",
						MemoryRequests: "300Mi",
						CPULimits:      "100m",
						MemoryLimits:   "300Mi",
						Image:          specConfig.KubernetesImageBase + api.K8sComponentsByVersionMap["1.15.4"][common.ClusterAutoscalerAddonName],
					},
				},
			},
			cs: &ContainerService{
				Properties: &api.Properties{
					OrchestratorProfile: &api.OrchestratorProfile{
						OrchestratorType:    api.Kubernetes,
						OrchestratorVersion: "1.15.4",
						KubernetesConfig: &api.KubernetesConfig{
							NetworkPlugin: api.NetworkPluginAzure,
							Addons: []api.KubernetesAddon{
								{
									Name:    common.ClusterAutoscalerAddonName,
									Enabled: to.BoolPtr(true),
								},
							},
							UseManagedIdentity: true,
						},
					},
					AgentPoolProfiles: []*api.AgentPoolProfile{
						{
							Name:                "pool1",
							Count:               1,
							AvailabilityProfile: api.VirtualMachineScaleSets,
						},
						{
							Name:                "pool2",
							Count:               1,
							AvailabilityProfile: api.VirtualMachineScaleSets,
						},
					},
				},
			},
			expectedNodesConfig: "",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			result := GetClusterAutoscalerNodesConfig(c.addon, c.cs)
			if c.expectedNodesConfig != result {
				t.Errorf("expected GetClusterAutoscalerNodesConfig to return %s, instead got %s", c.expectedNodesConfig, result)
			}
		})
	}
}

func concatenateDefaultAddons(addons []api.KubernetesAddon, version string) []api.KubernetesAddon {
	defaults := getDefaultAddons(version)
	defaults = append(defaults, addons...)
	return defaults
}

func overwriteDefaultAddons(addons []api.KubernetesAddon, version string) []api.KubernetesAddon {
	overrideAddons := make(map[string]api.KubernetesAddon)
	for _, addonOverride := range addons {
		overrideAddons[addonOverride.Name] = addonOverride
	}

	var ret []api.KubernetesAddon
	defaults := getDefaultAddons(version)

	for _, addon := range defaults {
		if _, exists := overrideAddons[addon.Name]; exists {
			ret = append(ret, overrideAddons[addon.Name])
			continue
		}
		ret = append(ret, addon)
	}

	return ret
}

func omitFromAddons(addons []string, completeSet []api.KubernetesAddon) []api.KubernetesAddon {
	var ret []api.KubernetesAddon
	for _, addon := range completeSet {
		if !isInStrSlice(addon.Name, addons) {
			ret = append(ret, addon)
		}
	}
	return ret
}

func isInStrSlice(name string, names []string) bool {
	for _, n := range names {
		if name == n {
			return true
		}
	}
	return false
}

func getDefaultAddons(version string) []api.KubernetesAddon {
	specConfig := api.AzureCloudSpecEnvMap["AzurePublicCloud"].KubernetesSpecConfig
	addons := []api.KubernetesAddon{
		{
			Name:    common.BlobfuseFlexVolumeAddonName,
			Enabled: to.BoolPtr(true),
			Containers: []api.KubernetesContainerSpec{
				{
					Name:           common.BlobfuseFlexVolumeAddonName,
					CPURequests:    "50m",
					MemoryRequests: "100Mi",
					CPULimits:      "50m",
					MemoryLimits:   "100Mi",
					Image:          api.K8sComponentsByVersionMap[version][common.BlobfuseFlexVolumeAddonName],
				},
			},
		},
		{
			Name:    common.KeyVaultFlexVolumeAddonName,
			Enabled: to.BoolPtr(true),
			Containers: []api.KubernetesContainerSpec{
				{
					Name:           common.KeyVaultFlexVolumeAddonName,
					CPURequests:    "50m",
					MemoryRequests: "100Mi",
					CPULimits:      "50m",
					MemoryLimits:   "100Mi",
					Image:          api.K8sComponentsByVersionMap[version][common.KeyVaultFlexVolumeAddonName],
				},
			},
		},
		{
			Name:    common.DashboardAddonName,
			Enabled: to.BoolPtr(true),
			Containers: []api.KubernetesContainerSpec{
				{
					Name:           common.DashboardAddonName,
					CPURequests:    "300m",
					MemoryRequests: "150Mi",
					CPULimits:      "300m",
					MemoryLimits:   "150Mi",
					Image:          specConfig.KubernetesImageBase + api.K8sComponentsByVersionMap[version][common.DashboardAddonName],
				},
			},
		},
		{
			Name:    common.MetricsServerAddonName,
			Enabled: to.BoolPtr(true),
			Containers: []api.KubernetesContainerSpec{
				{
					Name:  common.MetricsServerAddonName,
					Image: specConfig.KubernetesImageBase + api.K8sComponentsByVersionMap[version][common.MetricsServerAddonName],
				},
			},
		},
		{
			Name:    common.IPMASQAgentAddonName,
			Enabled: to.BoolPtr(true),
			Containers: []api.KubernetesContainerSpec{
				{
					Name:           common.IPMASQAgentAddonName,
					CPURequests:    "50m",
					MemoryRequests: "50Mi",
					CPULimits:      "50m",
					MemoryLimits:   "250Mi",
					Image:          specConfig.KubernetesImageBase + api.K8sComponentsByVersionMap[version][common.IPMASQAgentAddonName],
				},
			},
			Config: map[string]string{
				"non-masquerade-cidr": api.DefaultVNETCIDR,
				"non-masq-cni-cidr":   api.DefaultCNICIDR,
				"enable-ipv6":         "false",
			},
		},
		{
			Name:    common.AzureCNINetworkMonitorAddonName,
			Enabled: to.BoolPtr(true),
			Containers: []api.KubernetesContainerSpec{
				{
					Name:  common.AzureCNINetworkMonitorAddonName,
					Image: specConfig.AzureCNIImageBase + api.K8sComponentsByVersionMap[version][common.AzureCNINetworkMonitorAddonName],
				},
			},
		},
		{
			Name:    common.AuditPolicyAddonName,
			Enabled: to.BoolPtr(true),
		},
		{
			Name:    common.AzureCloudProviderAddonName,
			Enabled: to.BoolPtr(true),
		},
		{
			Name:    common.CoreDNSAddonName,
			Enabled: to.BoolPtr(api.DefaultCoreDNSAddonEnabled),
			Config: map[string]string{
				"domain":    "cluster.local",
				"clusterIP": api.DefaultKubernetesDNSServiceIP,
			},
			Containers: []api.KubernetesContainerSpec{
				{
					Name:  common.CoreDNSAddonName,
					Image: specConfig.KubernetesImageBase + api.K8sComponentsByVersionMap[version][common.CoreDNSAddonName],
				},
			},
		},
		{
			Name:    common.KubeProxyAddonName,
			Enabled: to.BoolPtr(api.DefaultKubeProxyAddonEnabled),
			Config: map[string]string{
				"cluster-cidr": api.DefaultKubernetesSubnet,
				"proxy-mode":   string(api.KubeProxyModeIPTables),
				"featureGates": "{}",
			},
			Containers: []api.KubernetesContainerSpec{
				{
					Name:  common.KubeProxyAddonName,
					Image: specConfig.KubernetesImageBase + api.K8sComponentsByVersionMap[version][common.KubeProxyAddonName],
				},
			},
		},
	}

	if common.IsKubernetesVersionGe(version, "1.15.0") {
		addons = append(addons, api.KubernetesAddon{
			Name:    common.PodSecurityPolicyAddonName,
			Enabled: to.BoolPtr(true),
		})
	}

	return addons
}
