package e2e

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

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
