package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

func (t *Template) ubuntu2204ARM64() *Scenario {
	return &Scenario{
		Name:        "ubuntu2204-arm64",
		Description: "Tests that an Ubuntu 2204 Node using ARM64 architecture can be properly bootstrapped",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu2204Gen2ARM64Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
				// This needs to be set based on current CSE implementation...
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
				nbc.IsARM64 = true

			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	}
}
