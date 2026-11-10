package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/samber/lo"
)

const (
	gpuQuotaRecoverySubscription = "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8"
	maxGPUQuotaRecoveryAttempts  = 3
	gpuQuotaRetryDelay           = 15 * time.Second
	gpuQuotaDeletionTimeout      = 5 * time.Minute
)

// Serialize reclamation and its retry so concurrent scenarios don't evict resources
// based on quota errors that another scenario has already resolved.
var gpuQuotaRecoveryLock = make(chan struct{}, 1)

type gpuQuotaRecovery struct {
	attempts int
}

// Recovery failures must not be downgraded to quota/capacity skips by the PR gate.
type gpuQuotaRecoveryError struct{ error }

func (e *gpuQuotaRecoveryError) Unwrap() error { return e.error }

func (recovery *gpuQuotaRecovery) createVMSS(
	ctx context.Context,
	model armcompute.VirtualMachineScaleSet,
	create func() (*ScenarioVM, error),
) (*ScenarioVM, error) {
	vm, err := create()
	if err == nil || !isComputeCoreQuotaError(err) ||
		!strings.EqualFold(config.Config.SubscriptionID, gpuQuotaRecoverySubscription) ||
		model.SKU == nil || lo.FromPtr(model.SKU.Name) == "" || lo.FromPtr(model.Location) == "" {
		return vm, err
	}

	target := gpuQuotaTarget{size: *model.SKU.Name, location: *model.Location, priority: armcompute.VirtualMachinePriorityTypesRegular}
	if model.Properties != nil && model.Properties.VirtualMachineProfile != nil {
		target.priority = gpuQuotaPriority(model.Properties.VirtualMachineProfile.Priority)
	}
	gpu, lookupErr := target.isGPU(ctx)
	if lookupErr != nil {
		return vm, &gpuQuotaRecoveryError{errors.Join(err, lookupErr)}
	}
	if !gpu {
		return vm, err
	}

	waited := false
	select {
	case gpuQuotaRecoveryLock <- struct{}{}:
	default:
		waited = true
		select {
		case gpuQuotaRecoveryLock <- struct{}{}:
		case <-ctx.Done():
			return vm, &gpuQuotaRecoveryError{errors.Join(err, ctx.Err())}
		}
	}
	defer func() { <-gpuQuotaRecoveryLock }()

	for recovery.attempts < maxGPUQuotaRecoveryAttempts {
		if ctx.Err() != nil {
			return vm, &gpuQuotaRecoveryError{errors.Join(err, ctx.Err())}
		}
		recovery.attempts++
		logging.Logf(ctx, "GPU core quota exceeded for %s in %s; recovery attempt %d/%d: %v",
			target.size, target.location, recovery.attempts, maxGPUQuotaRecoveryAttempts, err)
		if waited {
			// A preceding recovery may have freed enough quota. Retry before evicting.
			waited = false
		} else if reclaimErr := target.reclaimOne(ctx); reclaimErr != nil {
			return vm, &gpuQuotaRecoveryError{errors.Join(err, fmt.Errorf("reclaim GPU quota: %w", reclaimErr))}
		}
		select {
		case <-ctx.Done():
			return vm, &gpuQuotaRecoveryError{errors.Join(err, ctx.Err())}
		case <-time.After(gpuQuotaRetryDelay):
		}
		if vm != nil {
			cleanupBastionTunnel(vm.SSHClient)
		}
		vm, err = create()
		if err == nil || !isComputeCoreQuotaError(err) {
			return vm, err
		}
	}
	return vm, fmt.Errorf("GPU quota recovery exhausted after %d attempts: %w", maxGPUQuotaRecoveryAttempts, err)
}

func isComputeCoreQuotaError(err error) bool {
	var responseErr *azcore.ResponseError
	if !errors.As(err, &responseErr) ||
		(responseErr.StatusCode != http.StatusConflict && responseErr.StatusCode != http.StatusOK) ||
		responseErr.ErrorCode != "OperationNotAllowed" || responseErr.RawResponse == nil {
		return false
	}
	payload, err := runtime.Payload(responseErr.RawResponse)
	if err != nil {
		return false
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(payload, &body) != nil || body.Error.Code != responseErr.ErrorCode {
		return false
	}
	message := strings.ToLower(strings.Join(strings.Fields(body.Error.Message), " "))
	return strings.Contains(message, "exceeding approved") &&
		(strings.Contains(message, " cores quota") || strings.Contains(message, " vcpu quota") ||
			strings.Contains(message, " vcpus quota"))
}

type gpuQuotaTarget struct {
	size     string
	location string
	priority armcompute.VirtualMachinePriorityTypes
}

func gpuQuotaPriority(priority *armcompute.VirtualMachinePriorityTypes) armcompute.VirtualMachinePriorityTypes {
	if priority == nil {
		return armcompute.VirtualMachinePriorityTypesRegular
	}
	if *priority == armcompute.VirtualMachinePriorityTypesLow {
		return armcompute.VirtualMachinePriorityTypesSpot
	}
	return *priority
}

func (t gpuQuotaTarget) isGPU(ctx context.Context) (bool, error) {
	catalogue, err := config.Azure.ListVMResourceSKUs(ctx, t.location)
	if err != nil {
		return false, fmt.Errorf("look up GPU SKU %s: %w", t.size, err)
	}
	sku := catalogue[strings.ToUpper(t.size)]
	if sku == nil {
		return false, fmt.Errorf("SKU %s not found in %s; refusing quota reclamation", t.size, t.location)
	}
	for _, capability := range sku.Capabilities {
		if capability == nil || !strings.EqualFold(lo.FromPtr(capability.Name), "GPUs") {
			continue
		}
		count, err := strconv.Atoi(lo.FromPtr(capability.Value))
		if err != nil {
			return false, fmt.Errorf("invalid GPU count for SKU %s: %w", t.size, err)
		}
		return count > 0, nil
	}
	return false, nil
}

type gpuQuotaResource struct {
	id       *arm.ResourceID
	scaleSet bool
}

func protectedGPUQuotaResource(name string, tags map[string]*string) bool {
	name = strings.ToLower(name)
	if strings.Contains(name, "abe2e") || strings.HasPrefix(name, "mc_") || strings.HasPrefix(name, "aks-") {
		return true
	}
	for key := range tags {
		if strings.EqualFold(key, buildIDTagKey) || strings.EqualFold(key, "KEEP_VMSS") ||
			strings.HasPrefix(strings.ToLower(key), "aks-managed-") {
			return true
		}
	}
	return false
}

func (t gpuQuotaTarget) candidate(id, name, location, size string, tags map[string]*string, scaleSet bool) (*gpuQuotaResource, error) {
	if !strings.EqualFold(size, t.size) || !strings.EqualFold(location, t.location) ||
		protectedGPUQuotaResource(name, tags) {
		return nil, nil
	}
	resourceID, err := arm.ParseResourceID(id)
	if err != nil {
		return nil, fmt.Errorf("invalid GPU resource ID %q: %w", id, err)
	}
	if name == "" || resourceID.ResourceGroupName == "" || !strings.EqualFold(resourceID.Name, name) {
		return nil, fmt.Errorf("inconsistent GPU resource name %q and ID %q; refusing deletion", name, id)
	}
	resourceType := "Microsoft.Compute/virtualMachines"
	if scaleSet {
		resourceType = "Microsoft.Compute/virtualMachineScaleSets"
	}
	if !strings.EqualFold(resourceID.SubscriptionID, gpuQuotaRecoverySubscription) ||
		!strings.EqualFold(resourceID.ResourceType.String(), resourceType) ||
		protectedGPUQuotaResource(resourceID.ResourceGroupName, nil) {
		return nil, nil
	}
	return &gpuQuotaResource{id: resourceID, scaleSet: scaleSet}, nil
}

func (t gpuQuotaTarget) vmCandidate(vm *armcompute.VirtualMachine) (*gpuQuotaResource, error) {
	if vm == nil || vm.Properties == nil || vm.Properties.HardwareProfile == nil ||
		vm.Properties.VirtualMachineScaleSet != nil || gpuQuotaPriority(vm.Properties.Priority) != t.priority {
		return nil, nil
	}
	return t.candidate(lo.FromPtr(vm.ID), lo.FromPtr(vm.Name), lo.FromPtr(vm.Location),
		string(lo.FromPtr(vm.Properties.HardwareProfile.VMSize)), vm.Tags, false)
}

func (t gpuQuotaTarget) vmssCandidate(vmss *armcompute.VirtualMachineScaleSet) (*gpuQuotaResource, error) {
	if vmss == nil || vmss.SKU == nil || lo.FromPtr(vmss.SKU.Capacity) <= 0 || vmss.Properties == nil ||
		(vmss.Properties.OrchestrationMode != nil && *vmss.Properties.OrchestrationMode != armcompute.OrchestrationModeUniform) {
		// Flexible scale sets can contain multiple sizes; their SKU alone isn't enough.
		return nil, nil
	}
	var priority *armcompute.VirtualMachinePriorityTypes
	if vmss.Properties.VirtualMachineProfile != nil {
		priority = vmss.Properties.VirtualMachineProfile.Priority
	}
	if gpuQuotaPriority(priority) != t.priority {
		return nil, nil
	}
	return t.candidate(lo.FromPtr(vmss.ID), lo.FromPtr(vmss.Name), lo.FromPtr(vmss.Location),
		lo.FromPtr(vmss.SKU.Name), vmss.Tags, true)
}

func (t gpuQuotaTarget) reclaimOne(ctx context.Context) error {
	var candidates []*gpuQuotaResource
	vms := config.Azure.VMs.NewListAllPager(nil)
	for vms.More() {
		page, err := vms.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list GPU quota VM candidates: %w", err)
		}
		for _, vm := range page.Value {
			candidate, err := t.vmCandidate(vm)
			if err != nil {
				return err
			}
			if candidate != nil {
				candidates = append(candidates, candidate)
			}
		}
	}
	scaleSets := config.Azure.VMSS.NewListAllPager(nil)
	for scaleSets.More() {
		page, err := scaleSets.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list GPU quota VMSS candidates: %w", err)
		}
		for _, vmss := range page.Value {
			candidate, err := t.vmssCandidate(vmss)
			if err != nil {
				return err
			}
			if candidate != nil {
				candidates = append(candidates, candidate)
			}
		}
	}

	for _, candidate := range candidates {
		retry, err := t.deleteCandidate(ctx, candidate)
		if err != nil {
			return err
		}
		if retry {
			return nil
		}
	}
	logging.Logf(ctx, "no reclaimable non-AgentBaker GPU resources with exact SKU %s in %s; leaving other resources untouched", t.size, t.location)
	return nil
}

func (t gpuQuotaTarget) deleteCandidate(ctx context.Context, candidate *gpuQuotaResource) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, gpuQuotaDeletionTimeout)
	defer cancel()
	id := candidate.id
	group, err := config.Azure.ResourceGroup.Get(ctx, id.ResourceGroupName, nil)
	if isNotFoundErr(err) {
		logging.Logf(ctx, "GPU candidate resource group %s is already absent; retrying creation before further reclamation", id.ResourceGroupName)
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("check GPU resource group %s: %w", id.ResourceGroupName, err)
	}
	if lo.FromPtr(group.ManagedBy) != "" || protectedGPUQuotaResource(id.ResourceGroupName, group.Tags) {
		return false, nil
	}

	// Re-read the resource immediately before deletion: its size or ownership tags
	// may have changed since the subscription-wide inventory was collected.
	var current *gpuQuotaResource
	if candidate.scaleSet {
		response, getErr := config.Azure.VMSS.Get(ctx, id.ResourceGroupName, id.Name, nil)
		err = getErr
		if err == nil {
			current, err = t.vmssCandidate(&response.VirtualMachineScaleSet)
		}
	} else {
		response, getErr := config.Azure.VMs.Get(ctx, id.ResourceGroupName, id.Name, nil)
		err = getErr
		if err == nil {
			current, err = t.vmCandidate(&response.VirtualMachine)
		}
	}
	if isNotFoundErr(err) {
		logging.Logf(ctx, "GPU candidate %s is already absent; retrying creation before further reclamation", id)
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("recheck GPU resource %s: %w", id, err)
	}
	if current == nil {
		return false, nil
	}
	if !strings.EqualFold(current.id.String(), id.String()) {
		return false, fmt.Errorf("GPU resource ID changed from %s to %s; refusing deletion", id, current.id)
	}

	logging.Logf(ctx, "reclaiming non-AgentBaker GPU resource %s (SKU %s, region %s) for E2E quota", id, t.size, t.location)
	deleteAccepted := false
	if candidate.scaleSet {
		poller, deleteErr := config.Azure.VMSS.BeginDelete(ctx, id.ResourceGroupName, id.Name, nil)
		err = deleteErr
		if err == nil {
			deleteAccepted = true
			_, err = poller.PollUntilDone(ctx, config.PollUntilDoneOptions())
		}
	} else {
		poller, deleteErr := config.Azure.VMs.BeginDelete(ctx, id.ResourceGroupName, id.Name, nil)
		err = deleteErr
		if err == nil {
			deleteAccepted = true
			_, err = poller.PollUntilDone(ctx, config.PollUntilDoneOptions())
		}
	}
	if !deleteAccepted && isNotFoundErr(err) {
		logging.Logf(ctx, "GPU candidate %s disappeared before deletion; retrying creation before further reclamation", id)
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("delete GPU resource %s: %w", id, err)
	}
	logging.Logf(ctx, "GPU resource deletion completed: %s", id)
	return true, nil
}
