package e2e_test

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

func setupCluster(ctx context.Context, cloud *azureClient, location, resourceGroupName, clusterName string) error {
	var needNewCluster bool

	testRgExistence, err := cloud.resourceGroupClient.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return fmt.Errorf("failed to get AB E2E RG %q: %q", resourceGroupName, err)
	}

	if testRgExistence.Success {
		aksCluster, err := cloud.aksClient.Get(ctx, resourceGroupName, clusterName, nil)
		if err != nil {
			if !isResourceNotFoundError(err) {
				return fmt.Errorf("failed to get aks cluster %q: %q", clusterName, err)
			}
			needNewCluster = true
		}

		// We only need to check the MC resource group + cluster properties if the cluster resource itself exists
		if !needNewCluster {
			mcRgExistence, err := cloud.resourceGroupClient.CheckExistence(ctx, agentbakerTestClusterMCResourceGroupName, nil)
			if err != nil {
				return fmt.Errorf("failed to get test cluster MC RG %q: %q", agentbakerTestClusterMCResourceGroupName, err)
			}

			if !mcRgExistence.Success || aksCluster.Properties == nil || aksCluster.Properties.ProvisioningState == nil || *aksCluster.Properties.ProvisioningState == "Failed" {
				needNewCluster = true
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
		needNewCluster = true
		_, err := cloud.resourceGroupClient.CreateOrUpdate(
			ctx,
			resourceGroupName,
			armresources.ResourceGroup{
				Location: to.Ptr(e2eTestLocation),
				Name:     to.Ptr(resourceGroupName),
			},
			nil)

		if err != nil {
			return fmt.Errorf("failed to create AB E2E RG %q: %q", resourceGroupName, err)
		}
	}

	if needNewCluster {
		pollerResp, err := cloud.aksClient.BeginCreateOrUpdate(
			ctx,
			resourceGroupName,
			clusterName,
			armcontainerservice.ManagedCluster{
				Location: to.Ptr(location),
				Properties: &armcontainerservice.ManagedClusterProperties{
					DNSPrefix: to.Ptr(clusterName),
					AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
						{
							Name:         to.Ptr("nodepool1"),
							Count:        to.Ptr[int32](1),
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
					Type: to.Ptr(armcontainerservice.ResourceIdentityType("SystemAssigned")),
				},
			},
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

func getClusterSubnetID(ctx context.Context, cloud *azureClient, location, resourceGroupName, clusterName string) (string, error) {
	pager := cloud.vnetClient.NewListPager(agentbakerTestClusterMCResourceGroupName, nil)

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
