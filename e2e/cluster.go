package e2e_test

import (
	"context"
	"fmt"
	mrand "math/rand"
	"strings"
	"testing"

	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

const (
	managedClusterResourceType = "Microsoft.ContainerService/managedClusters"
)

type paramCache map[string]map[string]string

func isExistingResourceGroup(ctx context.Context, cloud *azureClient, resourceGroupName string) (bool, error) {
	rgExistence, err := cloud.resourceGroupClient.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get RG %q: %q", resourceGroupName, err)
	}

	return rgExistence.Success, nil
}

func ensureResourceGroup(ctx context.Context, t *testing.T, cloud *azureClient, resourceGroupName string) error {
	t.Logf("ensuring resource group %q...", resourceGroupName)

	rgExists, err := isExistingResourceGroup(ctx, cloud, resourceGroupName)
	if err != nil {
		return err
	}

	if !rgExists {
		_, err = cloud.resourceGroupClient.CreateOrUpdate(
			ctx,
			resourceGroupName,
			armresources.ResourceGroup{
				Location: to.Ptr(e2eTestLocation),
				Name:     to.Ptr(resourceGroupName),
			},
			nil)

		if err != nil {
			return fmt.Errorf("failed to create RG %q: %q", resourceGroupName, err)
		}
	}

	return nil
}

func validateExistingClusterState(
	ctx context.Context,
	t *testing.T,
	cloud *azureClient,
	resourceGroupName string,
	clusterModel *armcontainerservice.ManagedCluster) (bool, error) {
	var needRecreate bool
	clusterName := *clusterModel.Name

	cluster, err := cloud.aksClient.Get(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		if isResourceNotFoundError(err) {
			t.Logf("received ResourceNotFound error when trying to GET test cluster %q", clusterName)
			needRecreate = true
		} else {
			return false, fmt.Errorf("failed to get aks cluster %q: %q", clusterName, err)
		}
	} else {
		// We only need to check the MC resource group + cluster properties if the cluster resource itself exists
		rgExists, err := isExistingResourceGroup(ctx, cloud, *clusterModel.Properties.NodeResourceGroup)
		if err != nil {
			return false, err
		}

		if !rgExists || cluster.Properties == nil || cluster.Properties.ProvisioningState == nil || *cluster.Properties.ProvisioningState == "Failed" {
			t.Logf("deleting test cluster in bad state: %q", clusterName)

			needRecreate = true
			if err := deleteExistingCluster(ctx, cloud, resourceGroupName, clusterName); err != nil {
				return false, fmt.Errorf("failed to delete cluster in bad state: %s", err)
			}
		}
	}

	return needRecreate, nil
}

func createNewCluster(
	ctx context.Context,
	cloud *azureClient,
	resourceGroupName string,
	clusterModel *armcontainerservice.ManagedCluster) (*armcontainerservice.ManagedCluster, error) {
	pollerResp, err := cloud.aksClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		*clusterModel.Name,
		*clusterModel,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to begin aks cluster creation: %q", err)
	}

	clusterResp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for aks cluster creation %q", err)
	}

	return &clusterResp.ManagedCluster, nil
}

func deleteExistingCluster(ctx context.Context, cloud *azureClient, resourceGroupName, clusterName string) error {
	poller, err := cloud.aksClient.BeginDelete(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to start aks cluster %q deletion: %q", clusterName, err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for aks cluster %q deletion: %q", clusterName, err)
	}

	return nil
}

func getClusterSubnetID(ctx context.Context, cloud *azureClient, location, mcResourceGroupName, clusterName string) (string, error) {
	pager := cloud.vnetClient.NewListPager(mcResourceGroupName, nil)

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to advance page: %q", err)
		}
		for _, v := range nextResult.Value {
			if v == nil {
				return "", fmt.Errorf("aks vnet id was empty")
			}
			return fmt.Sprintf("%s/subnets/%s", *v.ID, "aks-subnet"), nil
		}
	}

	return "", fmt.Errorf("failed to find aks vnet")
}

func listClusters(ctx context.Context, t *testing.T, cloud *azureClient, resourceGroupName string) ([]*armcontainerservice.ManagedCluster, error) {
	clusters := []*armcontainerservice.ManagedCluster{}
	pager := cloud.resourceClient.NewListByResourceGroupPager(resourceGroupName, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to advance page: %q", err)
		}
		for _, resource := range page.Value {
			if strings.EqualFold(*resource.Type, managedClusterResourceType) {
				cluster, err := cloud.aksClient.Get(ctx, resourceGroupName, *resource.Name, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to get aks cluster: %q", err)
				}
				if cluster.Properties == nil {
					return nil, fmt.Errorf("aks cluster properties were nil")
				}

				t.Logf("found agentbaker e2e cluster: %q", *cluster.Name)
				clusters = append(clusters, &cluster.ManagedCluster)
			}
		}
	}

	return clusters, nil
}

func getViableClusters(scenario *scenario.Scenario, clusters []*armcontainerservice.ManagedCluster) []*armcontainerservice.ManagedCluster {
	viableClusters := []*armcontainerservice.ManagedCluster{}
	for _, cluster := range clusters {
		if scenario.ScenarioConfig.ClusterSelector(cluster) {
			viableClusters = append(viableClusters, cluster)
		}
	}
	return viableClusters
}

func mustChooseCluster(
	ctx context.Context,
	t *testing.T,
	r *mrand.Rand,
	cloud *azureClient,
	suiteConfig *suiteConfig,
	scenario *scenario.Scenario,
	clusters *[]*armcontainerservice.ManagedCluster,
	paramCache paramCache) (*kubeclient, *armcontainerservice.ManagedCluster, map[string]string, string) {
	var (
		chosenKubeClient    *kubeclient
		chosenCluster       *armcontainerservice.ManagedCluster
		chosenClusterParams map[string]string
		chosenSubnetID      string
	)

	viableClusters := getViableClusters(scenario, *clusters)

	if len(viableClusters) == 0 {
		t.Logf("unable to find viable test cluster for scenario %q, attempting to create a new one...", scenario.Name)
		clusterBaseModel := getBaseClusterModel(
			fmt.Sprintf(testClusterNameTemplate, randomLowercaseString(r, 5)),
			suiteConfig.location,
		)
		scenario.ScenarioConfig.ClusterMutator(&clusterBaseModel)

		cluster, err := createNewCluster(ctx, cloud, suiteConfig.resourceGroupName, &clusterBaseModel)
		if err != nil {
			t.Fatalf("unable to create new cluster: %s", err)
		}

		kube, subnetID, clusterParams, err := prepareClusterForTests(ctx, t, cloud, suiteConfig, cluster, paramCache)
		if err != nil {
			t.Fatalf("unable to prepare new cluster for test: %s", err)
		}

		*clusters = append(*clusters, cluster)

		chosenCluster = cluster
		chosenKubeClient = kube
		chosenSubnetID = subnetID
		chosenClusterParams = clusterParams
	} else {
		for _, viableCluster := range viableClusters {
			var cluster *armcontainerservice.ManagedCluster

			needRecreate, err := validateExistingClusterState(ctx, t, cloud, suiteConfig.resourceGroupName, viableCluster)
			if err != nil {
				t.Logf("unable to validate state of viable cluster %q: %s", *viableCluster.Name, err)
				continue
			}

			if needRecreate {
				t.Logf("viable cluster %q is in a bad state, attempting to recreate...", *viableCluster.Name)

				newCluster, err := createNewCluster(ctx, cloud, suiteConfig.resourceGroupName, viableCluster)
				if err != nil {
					t.Logf("unable to recreate viable cluster %q: %s", *viableCluster.Name, err)
					continue
				}
				cluster = newCluster
			} else {
				cluster = viableCluster
			}

			kube, subnetID, clusterParams, err := prepareClusterForTests(ctx, t, cloud, suiteConfig, cluster, paramCache)
			if err != nil {
				t.Logf("unable to prepare viable cluster for testing: %s", err)
				continue
			}

			chosenCluster = cluster
			chosenKubeClient = kube
			chosenSubnetID = subnetID
			chosenClusterParams = clusterParams
			break
		}
	}

	if chosenCluster == nil {
		t.Fatalf("unable to successfully choose a cluster for scenario %q", scenario.Name)
	}

	return chosenKubeClient, chosenCluster, chosenClusterParams, chosenSubnetID
}

func prepareClusterForTests(
	ctx context.Context,
	t *testing.T,
	cloud *azureClient,
	suiteConfig *suiteConfig,
	cluster *armcontainerservice.ManagedCluster,
	paramCache paramCache) (*kubeclient, string, map[string]string, error) {
	clusterName := *cluster.Name

	subnetID, err := getClusterSubnetID(ctx, cloud, suiteConfig.location, *cluster.Properties.NodeResourceGroup, clusterName)
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable get subnet ID of cluster %q: %s", clusterName, err)
	}

	kube, err := getClusterKubeClient(ctx, cloud, suiteConfig.resourceGroupName, clusterName)
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable get kube client using cluster %q: %s", clusterName, err)
	}

	if err := ensureDebugDaemonset(ctx, kube); err != nil {
		return nil, "", nil, fmt.Errorf("unable to ensure debug damonset of viable cluster %q: %s", clusterName, err)
	}

	clusterParams, err := getClusterParametersWithCache(ctx, t, kube, clusterName, paramCache)
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable to get cluster paramters: %s", err)
	}

	return kube, subnetID, clusterParams, nil
}

func getClusterParametersWithCache(ctx context.Context, t *testing.T, kube *kubeclient, clusterName string, paramCache paramCache) (map[string]string, error) {
	cachedParams, ok := paramCache[clusterName]
	if !ok {
		params, err := pollExtractClusterParameters(ctx, t, kube)
		if err != nil {
			return nil, fmt.Errorf("unable to extract cluster parameters from %q: %s", clusterName, err)
		}
		paramCache[clusterName] = params
		return params, nil
	} else {
		return cachedParams, nil
	}
}

func getBaseClusterModel(clusterName, location string) armcontainerservice.ManagedCluster {
	return armcontainerservice.ManagedCluster{
		Name:     to.Ptr(clusterName),
		Location: to.Ptr(location),
		Properties: &armcontainerservice.ManagedClusterProperties{
			DNSPrefix: to.Ptr(clusterName),
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Name:         to.Ptr("nodepool1"),
					Count:        to.Ptr[int32](2),
					VMSize:       to.Ptr("Standard_DS2_v2"),
					MaxPods:      to.Ptr[int32](110),
					OSType:       to.Ptr(armcontainerservice.OSTypeLinux),
					Type:         to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
					Mode:         to.Ptr(armcontainerservice.AgentPoolModeSystem),
					OSDiskSizeGB: to.Ptr[int32](512),
				},
			},
			NetworkProfile: &armcontainerservice.NetworkProfile{
				NetworkPlugin: to.Ptr(armcontainerservice.NetworkPluginKubenet),
			},
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: to.Ptr(armcontainerservice.ResourceIdentityTypeSystemAssigned),
		},
	}
}
