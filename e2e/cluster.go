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

func ensureCluster(ctx context.Context, t *testing.T, cloud *azureClient, location, resourceGroupName string, cluster *armcontainerservice.ManagedCluster, isNewCluster bool) error {
	var needCreate bool
	clusterName := *cluster.Name

	if !isNewCluster {
		aksCluster, err := cloud.aksClient.Get(ctx, resourceGroupName, clusterName, nil)
		if err != nil {
			if isResourceNotFoundError(err) {
				t.Logf("received ResourceNotFound error when trying to GET test cluster %q", clusterName)
				needCreate = true
			} else {
				return fmt.Errorf("failed to get aks cluster %q: %q", clusterName, err)
			}
		} else {
			// We only need to check the MC resource group + cluster properties if the cluster resource itself exists
			rgExists, err := isExistingResourceGroup(ctx, cloud, *cluster.Properties.NodeResourceGroup)
			if err != nil {
				return err
			}

			if !rgExists || aksCluster.Properties == nil || aksCluster.Properties.ProvisioningState == nil || *aksCluster.Properties.ProvisioningState == "Failed" {
				t.Logf("deleting test cluster in bad state: %q", clusterName)

				needCreate = true
				poller, err := cloud.aksClient.BeginDelete(ctx, resourceGroupName, clusterName, nil)
				if err != nil {
					return fmt.Errorf("failed to start aks cluster %q deletion: %q", clusterName, err)
				}

				_, err = poller.PollUntilDone(ctx, nil)
				if err != nil {
					return fmt.Errorf("failed to wait for aks cluster %q deletion: %q", clusterName, err)
				}
			}
		}
	} else {
		needCreate = true
	}

	// A new cluster is created if the test RG does not exist, the cluster itself does not exist, or if the cluster is in an unusable state
	if needCreate {
		t.Logf("recreating test cluster %q...", clusterName)

		pollerResp, err := cloud.aksClient.BeginCreateOrUpdate(
			ctx,
			resourceGroupName,
			clusterName,
			*cluster,
			nil,
		)

		if err != nil {
			return fmt.Errorf("failed to recreate aks cluster: %q", err)
		}

		_, err = pollerResp.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to finish aks cluster recreation %q", err)
		}
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
	clusters []*armcontainerservice.ManagedCluster) (*kubeclient, *armcontainerservice.ManagedCluster, map[string]string, string) {
	var (
		chosenKubeClient    *kubeclient
		chosenCluster       *armcontainerservice.ManagedCluster
		chosenClusterParams map[string]string
		chosenSubnetID      string
	)

	viableClusters := getViableClusters(scenario, clusters)
	if len(viableClusters) == 0 {
		t.Logf("unable to find viable test cluster for scenario %q, attempting to create a new one...", scenario.Name)
		newCluster := getBaseClusterModel(
			fmt.Sprintf(testClusterNameTemplate, randomLowercaseString(r, 5)),
			suiteConfig.location,
		)
		chosenCluster = &newCluster
		scenario.ScenarioConfig.ClusterMutator(chosenCluster)

		if err := ensureCluster(ctx, t, cloud, suiteConfig.location, suiteConfig.resourceGroupName, chosenCluster, true); err != nil {
			t.Fatal(err)
		}
	} else {
		for _, cluster := range viableClusters {
			if err := ensureCluster(ctx, t, cloud, suiteConfig.location, suiteConfig.resourceGroupName, cluster, false); err != nil {
				t.Logf("unable to ensure viable cluster %q: %s", *cluster.Name, err)
				continue
			}

			subnetID, err := getClusterSubnetID(ctx, cloud, suiteConfig.location, *cluster.Properties.NodeResourceGroup, *cluster.Name)
			if err != nil {
				t.Logf("unable get subnet ID of viable cluster %q: %s", *cluster.Name, err)
				continue
			}

			kube, err := getClusterKubeClient(ctx, cloud, suiteConfig.resourceGroupName, *cluster.Name)
			if err != nil {
				t.Logf("unable to get kube client using viable cluster %q: %s", *cluster.Name, err)
				continue
			}

			if err := ensureDebugDaemonset(ctx, kube); err != nil {
				t.Logf("unable to ensure debug damonset of viable cluster %q: %s", *cluster.Name, err)
				continue
			}

			clusterParams, err := pollExtractClusterParameters(ctx, t, kube)
			if err != nil {
				t.Logf("unable to extract cluster parameters from %q: %s", *cluster.Name, err)
				continue
			}

			chosenKubeClient = kube
			chosenCluster = cluster
			chosenSubnetID = subnetID
			chosenClusterParams = clusterParams
			break
		}
	}

	if chosenCluster == nil {
		t.Fatal("unable to choose test cluster from viable cluster set")
	}

	return chosenKubeClient, chosenCluster, chosenClusterParams, chosenSubnetID
}

func getBaseClusterModel(clusterName, location string) armcontainerservice.ManagedCluster {
	return armcontainerservice.ManagedCluster{
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
