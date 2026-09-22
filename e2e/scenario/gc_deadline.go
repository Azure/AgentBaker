package scenario

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
)

const deletionDueTimeTag = "deletion_due_time"

func ensureResourceGroup(ctx context.Context, azure *config.AzureClient, cfg *config.Configuration, location string) (armresources.ResourceGroup, error) {
	name := config.ResourceGroupName(location)
	response, err := azure.ResourceGroup.Get(ctx, name, nil)
	rg := response.ResourceGroup
	if isNotFoundErr(err) {
		created, createErr := azure.ResourceGroup.CreateOrUpdate(ctx, name, armresources.ResourceGroup{
			Location: to.Ptr(location), Name: to.Ptr(name),
		}, nil)
		if createErr != nil {
			return armresources.ResourceGroup{}, fmt.Errorf("creating RG %q: %w", name, createErr)
		}
		rg = created.ResourceGroup
	} else if err != nil {
		return armresources.ResourceGroup{}, fmt.Errorf("getting RG %q: %w", name, err)
	}
	if rg.Properties != nil && rg.Properties.ProvisioningState != nil && strings.EqualFold(*rg.Properties.ProvisioningState, "Deleting") {
		return armresources.ResourceGroup{}, fmt.Errorf("RG %q is deleting", name)
	}
	renewResourceGroupDeadline(ctx, azure, cfg, rg)
	return rg, nil
}

// GC expires parent and node RGs independently. Allow a suite plus cleanup from
// now, without a permanent exemption. Renewal is best-effort so tag failures do
// not block usable infrastructure. This cannot stop a DELETE already selected
// by GC or protect the node RG before the initial AKS lookup.
func renewResourceGroupDeadline(ctx context.Context, azure *config.AzureClient, cfg *config.Configuration, rg armresources.ResourceGroup) bool {
	if err := extendResourceGroupDeadline(ctx, azure, cfg, rg); err != nil {
		logging.Logf(ctx, "warning: failed to renew resource group GC deadline: %v", err)
		return false
	}
	return true
}

func renewNodeResourceGroupDeadline(ctx context.Context, azure *config.AzureClient, cfg *config.Configuration, cluster *armcontainerservice.ManagedCluster) bool {
	if cluster == nil || cluster.Properties == nil || cluster.Properties.NodeResourceGroup == nil || *cluster.Properties.NodeResourceGroup == "" {
		logging.Log(ctx, "warning: cannot renew GC deadline: AKS response has no node resource group")
		return false
	}
	name := *cluster.Properties.NodeResourceGroup
	rg, err := azure.ResourceGroup.Get(ctx, name, nil)
	if err != nil {
		logging.Logf(ctx, "warning: reading GC deadline for RG %q: %v", name, err)
		return false
	}
	return renewResourceGroupDeadline(ctx, azure, cfg, rg.ResourceGroup)
}

func extendResourceGroupDeadline(ctx context.Context, azure *config.AzureClient, cfg *config.Configuration, rg armresources.ResourceGroup) error {
	resourceGroup := ""
	if rg.Name != nil {
		resourceGroup = *rg.Name
	}
	due := time.Now().Add(cfg.SuiteTimeout + CleanupTimeout)
	if rg.Properties != nil && rg.Properties.ProvisioningState != nil && strings.EqualFold(*rg.Properties.ProvisioningState, "Deleting") {
		return fmt.Errorf("cannot renew GC deadline for deleting RG %q", resourceGroup)
	}
	if value, exists := rg.Tags[deletionDueTimeTag]; exists {
		if value == nil {
			return fmt.Errorf("RG %q has a null %s tag", resourceGroup, deletionDueTimeTag)
		}
		existing, err := time.Parse(time.RFC3339Nano, *value)
		if err != nil {
			return fmt.Errorf("parsing %s for RG %q: %w", deletionDueTimeTag, resourceGroup, err)
		}
		if !existing.Before(due) {
			return nil
		}
	}
	if rg.ID == nil || *rg.ID == "" {
		return fmt.Errorf("RG %q has no resource ID", resourceGroup)
	}

	// Merge only this tag: replacing the tag map could erase another writer's
	// unrelated tags. ARM does not provide an atomic max-deadline operation.
	poller, err := azure.Tags.BeginUpdateAtScope(ctx, *rg.ID, armresources.TagsPatchResource{
		Operation: to.Ptr(armresources.TagsPatchOperationMerge),
		Properties: &armresources.Tags{Tags: map[string]*string{
			deletionDueTimeTag: to.Ptr(due.UTC().Format(time.RFC3339Nano)),
		}},
	}, nil)
	if err != nil {
		return fmt.Errorf("renewing GC deadline for RG %q: %w", resourceGroup, err)
	}
	if _, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{Frequency: cfg.DefaultPollInterval}); err != nil {
		return fmt.Errorf("waiting for GC deadline renewal for RG %q: %w", resourceGroup, err)
	}
	logging.Logf(ctx, "extended RG %q GC deadline to %s", resourceGroup, due.UTC().Format(time.RFC3339Nano))
	return nil
}
