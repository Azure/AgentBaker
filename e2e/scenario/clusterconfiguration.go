package scenario

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
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
	if cluster != nil && cluster.Properties != nil && cluster.Properties.NetworkProfile != nil {
		cluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginAzure)
	}
}
