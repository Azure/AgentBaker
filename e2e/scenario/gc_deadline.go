package scenario

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
)

const deletionDueTimeTag = "deletion_due_time"

// DevInfra GC expires the cluster and node RGs independently, even during active
// tests. Extend both through the suite deadline plus cleanup, without permanently
// exempting unused infrastructure. Log renewal failures and continue so
// tag permissions or GC metadata do not block otherwise usable test infrastructure.
// Renewal cannot stop a DELETE already selected by GC; the initial shared-infra
// setup also precedes the AKS lookup needed to identify the node RG.
type suiteGCDeadlineKey struct{}

// WithSuiteDeadline captures the suite deadline before shorter attempt and setup
// timeouts are applied.
func WithSuiteDeadline(ctx context.Context) context.Context {
	deadline, ok := ctx.Deadline()
	if !ok {
		return ctx
	}
	return context.WithValue(ctx, suiteGCDeadlineKey{}, deadline.Add(CleanupTimeout))
}

func renewResourceGroupDeadline(ctx context.Context, resourceGroup string) {
	if err := extendResourceGroupDeadline(ctx, resourceGroup); err != nil {
		logging.Logf(ctx, "warning: failed to renew resource group %q GC deadline: %v", resourceGroup, err)
	}
}

func renewNodeResourceGroupDeadline(ctx context.Context, cluster *armcontainerservice.ManagedCluster) {
	if cluster == nil || cluster.Properties == nil || cluster.Properties.NodeResourceGroup == nil {
		logging.Log(ctx, "warning: cannot renew GC deadline: AKS response has no node resource group")
		return
	}
	renewResourceGroupDeadline(ctx, *cluster.Properties.NodeResourceGroup)
}

func extendResourceGroupDeadline(ctx context.Context, resourceGroup string) error {
	if resourceGroup == "" {
		return fmt.Errorf("cannot renew an empty resource group name")
	}
	due, ok := ctx.Value(suiteGCDeadlineKey{}).(time.Time)
	if !ok {
		return fmt.Errorf("suite deadline is required to protect shared resource groups")
	}
	rg, err := config.Azure.ResourceGroup.Get(ctx, resourceGroup, nil)
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
	poller, err := config.Azure.Tags.BeginUpdateAtScope(ctx, *rg.ID, armresources.TagsPatchResource{
		Operation: to.Ptr(armresources.TagsPatchOperationMerge),
		Properties: &armresources.Tags{Tags: map[string]*string{
			deletionDueTimeTag: to.Ptr(due.UTC().Format(time.RFC3339Nano)),
		}},
	}, nil)
	if err != nil {
		return fmt.Errorf("renewing GC deadline for RG %q: %w", resourceGroup, err)
	}
	if _, err := poller.PollUntilDone(ctx, config.PollUntilDoneOptions()); err != nil {
		return fmt.Errorf("waiting for GC deadline renewal for RG %q: %w", resourceGroup, err)
	}
	logging.Logf(ctx, "extended RG %q GC deadline to %s", resourceGroup, due.UTC().Format(time.RFC3339Nano))
	return nil
}
