package cluster

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

// WARNING: if you modify cluster configuration, please change the version below
// this will avoid potential conflicts with tests running on other branches
// there is no strict rules or a hidden meaning for the version
// testClusterNamePrefix is also used for versioning cluster configurations
const testClusterNamePrefix = "abe2e-v20240704-"

var (
	clusterKubenet       *Cluster
	clusterKubenetAirgap *Cluster
	clusterAzureNetwork  *Cluster

	clusterKubenetError       error
	clusterKubenetAirgapError error
	clusterAzureNetworkError  error

	clusterKubenetOnce       sync.Once
	clusterKubenetAirgapOnce sync.Once
	clusterAzureNetworkOnce  sync.Once
)

type Cluster struct {
	Model    *armcontainerservice.ManagedCluster
	Kube     *Kubeclient
	SubnetID string
}

// Returns true if the cluster is configured with Azure CNI
func (c *Cluster) IsAzureCNI() (bool, error) {
	if c.Model.Properties.NetworkProfile != nil {
		return *c.Model.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginAzure, nil
	}
	return false, fmt.Errorf("cluster network profile was nil: %+v", c.Model)
}

// Returns the maximum number of pods per node of the cluster's agentpool
func (c *Cluster) MaxPodsPerNode() (int, error) {
	if len(c.Model.Properties.AgentPoolProfiles) > 0 {
		return int(*c.Model.Properties.AgentPoolProfiles[0].MaxPods), nil
	}
	return 0, fmt.Errorf("cluster agentpool profiles were nil or empty: %+v", c.Model)
}

// Same cluster can be attempted to be created concurrently by different tests
// sync.Once is used to ensure that only one cluster for the set of tests is created
func ClusterKubenet(ctx context.Context) (*Cluster, error) {
	clusterKubenetOnce.Do(func() {
		clusterKubenet, clusterKubenetError = createCluster(ctx, getKubenetClusterModel(testClusterNamePrefix+"kubenet-v1"))
	})
	return clusterKubenet, clusterKubenetError
}

func ClusterKubenetAirgap(ctx context.Context) (*Cluster, error) {
	clusterKubenetAirgapOnce.Do(func() {
		cluster, err := createCluster(ctx, getKubenetClusterModel(testClusterNamePrefix+"kubenet-airgap"))
		if err == nil {
			err = addAirgapNetworkSettings(ctx, cluster)
		}
		clusterKubenetAirgap, clusterKubenetAirgapError = cluster, err
	})
	return clusterKubenetAirgap, clusterKubenetAirgapError
}

func ClusterAzureNetwork(ctx context.Context) (*Cluster, error) {
	clusterAzureNetworkOnce.Do(func() {
		clusterAzureNetwork, clusterAzureNetworkError = createCluster(ctx, getAzureNetworkClusterModel(testClusterNamePrefix+"azure-network"))
	})
	return clusterAzureNetwork, clusterAzureNetworkError
}

func createCluster(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*Cluster, error) {
	createdCluster, err := createNewAKSClusterWithRetry(ctx, cluster)
	if err != nil {
		return nil, err
	}

	kube, err := getClusterKubeClient(ctx, config.ResourceGroupName, *cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("get kube client using cluster %q: %w", *cluster.Name, err)
	}

	if err := ensureDebugDaemonset(ctx, kube); err != nil {
		return nil, fmt.Errorf("ensure debug damonset for %q: %w", *cluster.Name, err)
	}

	subnetID, err := getClusterSubnetID(ctx, *createdCluster.Properties.NodeResourceGroup)
	if err != nil {
		return nil, fmt.Errorf("get cluster subnet: %w", err)
	}

	return &Cluster{Model: createdCluster, Kube: kube, SubnetID: subnetID}, nil

}

func createNewAKSCluster(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	log.Printf("Creating new cluster %s in rg %s\n", *cluster.Name, *cluster.Location)
	pollerResp, err := config.Azure.AKS.BeginCreateOrUpdate(
		ctx,
		config.ResourceGroupName,
		*cluster.Name,
		*cluster,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to begin aks cluster creation: %w", err)
	}

	clusterResp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for aks cluster creation %w", err)
	}

	return &clusterResp.ManagedCluster, nil
}

// createNewAKSClusterWithRetry is a wrapper around createNewAKSCluster
// that retries creating a cluster if it fails with a 409 Conflict error
// clusters are reused, and sometimes a cluster can be in UPDATING or DELETING state
// simple retry should be sufficient to avoid such conflicts
func createNewAKSClusterWithRetry(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	maxRetries := 10
	retryInterval := 30 * time.Second
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		log.Printf("Attempt %d: Creating new cluster %s in rg %s\n", attempt+1, *cluster.Name, *cluster.Location)

		createdCluster, err := createNewAKSCluster(ctx, cluster)
		if err == nil {
			return createdCluster, nil
		}

		// Check if the error is a 409 Conflict
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			lastErr = err
			log.Printf("Attempt %d failed with 409 Conflict: %v. Retrying in %v...\n", attempt+1, err, retryInterval)

			select {
			case <-time.After(retryInterval):
				// Continue to next iteration
			case <-ctx.Done():
				return nil, fmt.Errorf("context canceled while retrying cluster creation: %w", ctx.Err())
			}
		} else {
			// If it's not a 409 error, return immediately
			return nil, fmt.Errorf("failed to create cluster: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to create cluster after %d attempts due to persistent 409 Conflict: %w", maxRetries, lastErr)
}

type VNet struct {
	name     string
	subnetId string
}

func getClusterVNet(ctx context.Context, mcResourceGroupName string) (VNet, error) {
	pager := config.Azure.VNet.NewListPager(mcResourceGroupName, nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return VNet{}, fmt.Errorf("failed to advance page: %w", err)
		}
		for _, v := range nextResult.Value {
			if v == nil {
				return VNet{}, fmt.Errorf("aks vnet was empty")
			}
			return VNet{name: *v.Name, subnetId: fmt.Sprintf("%s/subnets/%s", *v.ID, "aks-subnet")}, nil
		}
	}
	return VNet{}, fmt.Errorf("failed to find aks vnet")
}
