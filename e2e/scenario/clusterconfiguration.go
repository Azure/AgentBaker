package scenario

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

const (
	azureCNIDefaultMaxPodsPerNode = 30
)

// Selectors

func NetworkPluginKubenetSelector(cluster *armcontainerservice.ManagedCluster) bool {
	if cluster != nil && cluster.Properties != nil && cluster.Properties.NetworkProfile != nil {
		return *cluster.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginKubenet
	}
	return false
}

func NetworkPluginAzureSelector(cluster *armcontainerservice.ManagedCluster) bool {
	if cluster != nil && cluster.Properties != nil && cluster.Properties.NetworkProfile != nil {
		return *cluster.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginAzure
	}
	return false
}

// Mutators

func NetworkPluginKubenetMutator(cluster *armcontainerservice.ManagedCluster) {
	if cluster != nil && cluster.Properties != nil && cluster.Properties.NetworkProfile != nil {
		cluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginKubenet)
	}
}

func NetworkPluginAzureMutator(cluster *armcontainerservice.ManagedCluster) {
	if cluster != nil && cluster.Properties != nil {
		if cluster.Properties.NetworkProfile != nil {
			cluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginAzure)
		}
		if cluster.Properties.AgentPoolProfiles != nil {
			for _, app := range cluster.Properties.AgentPoolProfiles {
				app.MaxPods = to.Ptr[int32](azureCNIDefaultMaxPodsPerNode)
			}
		}
	}
}
