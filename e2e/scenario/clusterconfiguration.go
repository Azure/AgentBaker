package scenario

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

func NewNetworkPluginKubenetConfigurator() ClusterConfigurator {
	return ClusterConfigurator{
		DesiredName: "agentbaker-e2e-test-cluster-kubenet",
		Selector: func(cluster *armcontainerservice.ManagedCluster) bool {
			if cluster != nil && cluster.Properties != nil && cluster.Properties.NetworkProfile != nil {
				return *cluster.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginKubenet
			}
			return false
		},
		Mutator: func(cluster *armcontainerservice.ManagedCluster) {
			if cluster != nil {
				if cluster.Properties == nil {
					cluster.Properties = &armcontainerservice.ManagedClusterProperties{}
				}
				if cluster.Properties.NetworkProfile == nil {
					cluster.Properties.NetworkProfile = &armcontainerservice.NetworkProfile{}
				}
				cluster.Properties.NetworkProfile.NetworkPlugin = to.Ptr(armcontainerservice.NetworkPluginKubenet)
			}
		},
	}
}
