package e2e

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type VNet struct {
	name     string
	subnetId string
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
