package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

// NOTE: this works only if adding a VMSS to an existing cluster created with:
// az aks create -g <rg> -n <name> --network-plugin=azure --network-plugin-mode=overlay
func cnioverlay() *Scenario {
	return &Scenario{
		Name:        "cnioverlay",
		Description: "Test an Ubuntu 22.04 node configured with Azure CNI Overlay",
		Config: Config{
			ClusterSelector: NetworkPluginAzureSelector,
			ClusterMutator:  NetworkPluginAzureMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				addNetworkingLabels(nbc.AgentPoolProfile.CustomNodeLabels)
				for _, app := range nbc.ContainerService.Properties.AgentPoolProfiles {
					addNetworkingLabels(app.CustomNodeLabels)
				}

				// CNI overlay tricks AgentBaker into thinking network plugin = "none" since the CNI is installed via daemonset.
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = "none"
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
					ID: to.Ptr(DefaultImageVersionIDs["ubuntu2204"]),
				}
				vmss.SKU.Name = to.Ptr("Standard_D4s_v3")
			},
		},
	}
}

// This is a massive hack. DNC-RC needs these labels to create the NNC for the node.
// I'm just hard-coding the values from my test cluster here to get it working.
func addNetworkingLabels(nodeLabels map[string]string) {
	nodeLabels["kubernetes.azure.com/azure-cni-overlay"] = "true"
	nodeLabels["kubernetes.azure.com/network-name"] = "aks-vnet-39046617"
	nodeLabels["kubernetes.azure.com/network-resourcegroup"] = "widalytest"
	nodeLabels["kubernetes.azure.com/network-subnet"] = "aks-subnet"
	nodeLabels["kubernetes.azure.com/network-subscription"] = "18153b17-4e27-4b58-863e-f8105b8892a2"
	nodeLabels["kubernetes.azure.com/nodenetwork-vnetguid"] = "e9e144f8-2d89-4373-b572-7491a61a5d43"
	nodeLabels["kubernetes.azure.com/podnetwork-type"] = "overlay"
}
