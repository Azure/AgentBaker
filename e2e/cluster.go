package e2e

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type clusterConfig struct {
	cluster *armcontainerservice.ManagedCluster
	kube    *kubeclient
}

type VNet struct {
	name     string
	subnetId string
}

// Returns true if the cluster is configured with Azure CNI
func (c clusterConfig) isAzureCNI() (bool, error) {
	if c.cluster.Properties.NetworkProfile != nil {
		return *c.cluster.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginAzure, nil
	}
	return false, fmt.Errorf("cluster network profile was nil: %+v", c.cluster)
}

// Returns the maximum number of pods per node of the cluster's agentpool
func (c clusterConfig) maxPodsPerNode() (int, error) {
	if len(c.cluster.Properties.AgentPoolProfiles) > 0 {
		return int(*c.cluster.Properties.AgentPoolProfiles[0].MaxPods), nil
	}
	return 0, fmt.Errorf("cluster agentpool profiles were nil or empty: %+v", c.cluster)
}

// This map is used during cluster creation to check what VM size should
// be used to create the single default agentpool used for running cluster essentials
// and the jumpbox pods/resources. This is mainly need for regions where we can't use
// the default Standard_DS2_v2 VM size due to quota/capacity.
var locationToDefaultClusterAgentPoolVMSize = map[string]string{
	// TODO: add mapping for southcentralus to perform h100 testing
}

func isExistingResourceGroup(ctx context.Context, resourceGroupName string) (bool, error) {
	rgExistence, err := config.Azure.ResourceGroup.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get RG %q: %w", resourceGroupName, err)
	}

	return rgExistence.Success, nil
}

func ensureResourceGroup(ctx context.Context) error {
	log.Printf("ensuring resource group %q...", config.ResourceGroupName)

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

func validateAndPrepareCluster(ctx context.Context, clusterConfig *clusterConfig) error {
	kube, err := prepareClusterForTests(ctx, clusterConfig.cluster)
	if err != nil {
		return err
	}
	clusterConfig.kube = kube
	return nil
}

func prepareClusterForTests(
	ctx context.Context,
	cluster *armcontainerservice.ManagedCluster) (*kubeclient, error) {
	clusterName := *cluster.Name

	kube, err := getClusterKubeClient(ctx, config.ResourceGroupName, clusterName)
	if err != nil {
		return nil, fmt.Errorf("unable get kube client using cluster %q: %w", clusterName, err)
	}

	if err := ensureDebugDaemonset(ctx, kube); err != nil {
		return nil, fmt.Errorf("unable to ensure debug damonset of viable cluster %q: %w", clusterName, err)
	}
	return kube, nil
}
