package scenario

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

// Selectors

func NetworkPluginKubenetSelector(cluster *armcontainerservice.ManagedCluster) bool {
	return *cluster.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginKubenet
}

// Mutators

func NetworkPluginKubenetMutator(cluster *armcontainerservice.ManagedCluster) {
	cluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginKubenet)
}
