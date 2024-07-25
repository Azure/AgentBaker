package e2e

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

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
	Model                          *armcontainerservice.ManagedCluster
	Kube                           *Kubeclient
	SubnetID                       string
	NodeBootstrappingConfiguration *datamodel.NodeBootstrappingConfiguration
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
func ClusterKubenet(ctx context.Context, t *testing.T) (*Cluster, error) {
	clusterKubenetOnce.Do(func() {
		// WARNING: if you modify cluster configuration, please change the version below
		// this will avoid potential conflicts with tests running on other branches
		clusterKubenet, clusterKubenetError = createCluster(ctx, t, getKubenetClusterModel("abe2e-kubenet-v1-"+config.Location))
	})
	return clusterKubenet, clusterKubenetError
}

func ClusterKubenetAirgap(ctx context.Context, t *testing.T) (*Cluster, error) {
	clusterKubenetAirgapOnce.Do(func() {
		// WARNING: if you modify cluster configuration, please change the version below
		// this will avoid potential conflicts with tests running on other branches
		clusterKubenetAirgap, clusterKubenetAirgapError = createCluster(ctx, t, getKubenetClusterModel("abe2e-kubenet-airgap-v1"+config.Location))
		if clusterKubenetAirgapError == nil {
			clusterKubenetAirgapError = addAirgapNetworkSettings(ctx, t, clusterKubenetAirgap)
		}
	})
	return clusterKubenetAirgap, clusterKubenetAirgapError
}

func ClusterAzureNetwork(ctx context.Context, t *testing.T) (*Cluster, error) {
	clusterAzureNetworkOnce.Do(func() {
		// WARNING: if you modify cluster configuration, please change the version below
		// this will avoid potential conflicts with tests running on other branches
		clusterAzureNetwork, clusterAzureNetworkError = createCluster(ctx, t, getAzureNetworkClusterModel("abe2e-azure-network-v1"+config.Location))
	})
	return clusterAzureNetwork, clusterAzureNetworkError
}

func nodeBootsrappingConfig(ctx context.Context, t *testing.T, kube *Kubeclient) (*datamodel.NodeBootstrappingConfiguration, error) {
	clusterParams, err := pollExtractClusterParameters(ctx, t, kube)
	if err != nil {
		return nil, fmt.Errorf("extract cluster parameters: %w", err)
	}

	baseNodeBootstrappingConfig, err := getBaseNodeBootstrappingConfiguration(clusterParams)
	if err != nil {
		return nil, fmt.Errorf("get base node bootstrapping configuration: %w", err)
	}

	return baseNodeBootstrappingConfig, nil
}

func createCluster(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*Cluster, error) {
	createdCluster, err := createNewAKSClusterWithRetry(ctx, t, cluster)
	if err != nil {
		return nil, err
	}

	// sometimes tests can be interrupted and vmss are left behind
	// don't waste resource and delete them
	if err := collectGarbageVMSS(ctx, t, createdCluster); err != nil {
		return nil, fmt.Errorf("collect garbage vmss: %w", err)
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

	nbc, err := nodeBootsrappingConfig(ctx, t, kube)
	if err != nil {
		return nil, fmt.Errorf("get node bootstrapping configuration: %w", err)
	}

	return &Cluster{Model: createdCluster, Kube: kube, SubnetID: subnetID, NodeBootstrappingConfiguration: nbc}, nil

}

func createNewAKSCluster(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	t.Logf("Creating or updating cluster %s in rg %s\n", *cluster.Name, *cluster.Location)
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
func createNewAKSClusterWithRetry(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	maxRetries := 10
	retryInterval := 30 * time.Second
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		t.Logf("Attempt %d: creating or updating cluster %s in rg %s\n", attempt+1, *cluster.Name, *cluster.Location)

		createdCluster, err := createNewAKSCluster(ctx, t, cluster)
		if err == nil {
			return createdCluster, nil
		}

		// Check if the error is a 409 Conflict
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			lastErr = err
			t.Logf("Attempt %d failed with 409 Conflict: %v. Retrying in %v...\n", attempt+1, err, retryInterval)

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

func collectGarbageVMSS(ctx context.Context, t *testing.T, cluster *armcontainerservice.ManagedCluster) error {
	rg := *cluster.Properties.NodeResourceGroup
	pager := config.Azure.VMSS.NewListPager(rg, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get next page of VMSS: %w", err)
		}
		for _, vmss := range page.Value {
			if _, ok := vmss.Tags["KEEP_VMSS"]; ok {
				continue
			}
			// don't delete managed pools
			if _, ok := vmss.Tags["aks-managed-poolName"]; ok {
				continue
			}

			// don't delete VMSS created in the last hour. They might be currently used in tests
			// extra 10 minutes is a buffer for test cleanup, clock drift and timeout adjustments
			if config.TestTimeout == 0 || time.Since(*vmss.Properties.TimeCreated) < config.TestTimeout+10*time.Minute {
				continue
			}

			_, err := config.Azure.VMSS.BeginDelete(ctx, rg, *vmss.Name, &armcompute.VirtualMachineScaleSetsClientBeginDeleteOptions{
				ForceDeletion: to.Ptr(true),
			})
			if err != nil {
				t.Logf("failed to delete vmss %q: %s", *vmss.Name, err)
				continue
			}
			t.Logf("deleted garbage vmss %q", *vmss.ID)
		}
	}

	return nil
}

func isExistingResourceGroup(ctx context.Context, resourceGroupName string) (bool, error) {
	rgExistence, err := config.Azure.ResourceGroup.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get RG %q: %w", resourceGroupName, err)
	}

	return rgExistence.Success, nil
}

func ensureResourceGroup(ctx context.Context) error {
	rgExists, err := isExistingResourceGroup(ctx, config.ResourceGroupName)
	if err != nil {
		return err
	}

	if !rgExists {
		_, err = config.Azure.ResourceGroup.CreateOrUpdate(
			ctx,
			config.ResourceGroupName,
			armresources.ResourceGroup{
				Location: to.Ptr(config.Location),
				Name:     to.Ptr(config.ResourceGroupName),
			},
			nil)

		if err != nil {
			return fmt.Errorf("failed to create RG %q: %w", config.ResourceGroupName, err)
		}
	}

	return nil
}
