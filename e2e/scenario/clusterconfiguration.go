package scenario

import (
	"context"

	"github.com/Azure/agentbakere2e/cluster"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

const (
	// Default agentpool value of maxPods for Azure CNI
	azureCNIDefaultMaxPodsPerNode = 30
)

type Cluster struct {
	Creator func(ctx context.Context) (*armcontainerservice.ManagedCluster, error)
}

var ClusterNetworkKubenet = &Cluster{
	Creator: cluster.ClusterKubenet,
}

var ClusterNetworkAzure = &Cluster{
	Creator: cluster.ClusterAzureNetwork,
}

var ClusterNetworkKubenetAirgap = &Cluster{
	Creator: cluster.ClusterKubenetAirgap,
}
