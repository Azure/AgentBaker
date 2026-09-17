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

// GC expires parent and node RGs independently. Allow a suite plus cleanup from
// now, without a permanent exemption. Renewal is best-effort so tag failures do
// not block usable infrastructure. This cannot stop a DELETE already selected
// by GC or protect the node RG before the initial AKS lookup.
func renewResourceGroupDeadline(ctx context.Context, azure *config.AzureClient, cfg *config.Configuration, resourceGroup string) {
	if err := extendResourceGroupDeadline(ctx, azure, cfg, resourceGroup); err != nil {
		logging.Logf(ctx, "warning: failed to renew resource group %q GC deadline: %v", resourceGroup, err)
	}
}

func renewNodeResourceGroupDeadline(ctx context.Context, azure *config.AzureClient, cfg *config.Configuration, cluster *armcontainerservice.ManagedCluster) {
	if cluster == nil || cluster.Properties == nil || cluster.Properties.NodeResourceGroup == nil {
		logging.Log(ctx, "warning: cannot renew GC deadline: AKS response has no node resource group")
		return
	}
	renewResourceGroupDeadline(ctx, azure, cfg, *cluster.Properties.NodeResourceGroup)
}

func extendResourceGroupDeadline(ctx context.Context, azure *config.AzureClient, cfg *config.Configuration, resourceGroup string) error {
	if resourceGroup == "" {
		return fmt.Errorf("cannot renew an empty resource group name")
	}
	due := time.Now().Add(cfg.SuiteTimeout + CleanupTimeout)
	rg, err := azure.ResourceGroup.Get(ctx, resourceGroup, nil)
	if err != nil {
		return fmt.Errorf("reading GC deadline for RG %q: %w", resourceGroup, err)
	}
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
