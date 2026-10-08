// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/require"
)

func TestNetworkIsolatedGRIDScenario(t *testing.T) {
	for _, vhd := range []*config.Image{config.VHDUbuntu2204Gen2Containerd, config.VHDUbuntu2404Gen2Containerd} {
		t.Run(vhd.Name, func(t *testing.T) {
			s := networkIsolatedGRIDScenario("network-isolated-grid", vhd)
			require.True(t, s.Tags.GPU)
			require.True(t, s.Tags.NetworkIsolated)
			require.True(t, s.Tags.NonAnonymousACR)
			require.True(t, s.DisableScriptless)
			require.Same(t, vhd, s.VHD)
			require.NotNil(t, s.Validator)

			properties := datamodel.GetK8sDefaultProperties(false)
			properties.OrchestratorProfile.OrchestratorVersion = "1.34.11"
			properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{}
			nbc := &datamodel.NodeBootstrappingConfiguration{
				ContainerService:             &datamodel.ContainerService{Properties: properties},
				AgentPoolProfile:             properties.AgentPoolProfiles[0],
				K8sComponents:                &datamodel.K8sComponents{},
				KubeletConfig:                map[string]string{},
				UserAssignedIdentityClientID: "existing-kubelet-identity",
				EnableScriptlessCSECmd:       true,
				EnableScriptlessNBCCSECmd:    true,
			}

			cluster := &Cluster{Model: &armcontainerservice.ManagedCluster{Location: to.Ptr("eastus")}}
			s.BootstrapConfigMutator(cluster, nbc)

			require.Equal(t, "Standard_NV6ads_A10_v5", nbc.AgentPoolProfile.VMSize)
			require.True(t, nbc.ConfigGPUDriverIfNeeded)
			require.True(t, nbc.EnableNvidia)
			require.False(t, nbc.EnableGPUDevicePluginIfNeeded)
			require.False(t, nbc.EnableManagedGPU)
			require.True(t, nbc.EnableScriptlessCSECmd)
			require.True(t, nbc.EnableScriptlessNBCCSECmd)
			require.Equal(t, datamodel.OutboundTypeBlock, nbc.OutboundType)
			require.True(t, properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity)
			require.True(t, nbc.AgentPoolProfile.KubernetesConfig.UseManagedIdentity)
			require.Equal(t, "existing-kubelet-identity", nbc.UserAssignedIdentityClientID)
			require.True(t, properties.SecurityProfile.PrivateEgress.Enabled)
			require.Equal(t, config.GetPrivateACRName(true, "eastus")+".azurecr.io/aks-managed-repository",
				properties.SecurityProfile.PrivateEgress.ContainerRegistryServer)
			require.Equal(t, "/var/lib/kubelet/credential-provider-config.yaml", nbc.KubeletConfig["--image-credential-provider-config"])
			require.Equal(t, "/var/lib/kubelet/credential-provider", nbc.KubeletConfig["--image-credential-provider-bin-dir"])
			require.Equal(t, "https://packages.aks.azure.com/cloud-provider-azure/v1.34.11/binaries/azure-acr-credential-provider-linux-amd64-v1.34.11.tar.gz",
				nbc.K8sComponents.LinuxCredentialProviderURL)

			vmss := &armcompute.VirtualMachineScaleSet{SKU: &armcompute.SKU{}}
			s.VMConfigMutator(vmss)
			require.Equal(t, "Standard_NV6ads_A10_v5", *vmss.SKU.Name)
		})
	}
}

func TestScriptlessDeliveryRespectsRunnerAndScenario(t *testing.T) {
	old := config.Config.DisableScriptless
	t.Cleanup(func() { config.Config.DisableScriptless = old })
	for _, tc := range []struct {
		name     string
		runner   bool
		scenario bool
		want     bool
	}{
		{name: "runner and scenario use baked scripts"},
		{name: "runner requires generated scripts", runner: true, want: true},
		{name: "scenario requires current generated source", scenario: true, want: true},
		{name: "both require generated scripts", runner: true, scenario: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config.Config.DisableScriptless = tc.runner
			s := &Scenario{Config: Config{DisableScriptless: tc.scenario}}
			require.Equal(t, tc.want, scriptlessDisabled(s))
		})
	}
}
