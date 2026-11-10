package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const gpuQuotaTestSize = "Standard_NC4as_T4_v3"

func gpuQuotaTestError(status int, code, message string) error {
	body := fmt.Sprintf(`{"error":{"code":%q,"message":%q}}`, code, message)
	return &azcore.ResponseError{
		StatusCode: status,
		ErrorCode:  code,
		RawResponse: &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		},
	}
}

func gpuQuotaTestCoreError() error {
	return gpuQuotaTestError(http.StatusConflict, "OperationNotAllowed",
		"Operation could not be completed as it results in exceeding approved standardNCSv3Family Cores quota.")
}

func gpuQuotaTestID(group, resourceType, name string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/%s/%s",
		gpuQuotaRecoverySubscription, group, resourceType, name)
}

func gpuQuotaTestVM(group, name string) *armcompute.VirtualMachine {
	return &armcompute.VirtualMachine{
		ID: to.Ptr(gpuQuotaTestID(group, "virtualMachines", name)), Name: to.Ptr(name),
		Location: to.Ptr("westus3"),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(gpuQuotaTestSize))},
		},
	}
}

func gpuQuotaTestVMSS(group, name string) *armcompute.VirtualMachineScaleSet {
	return &armcompute.VirtualMachineScaleSet{
		ID: to.Ptr(gpuQuotaTestID(group, "virtualMachineScaleSets", name)), Name: to.Ptr(name),
		Location: to.Ptr("westus3"),
		SKU:      &armcompute.SKU{Name: to.Ptr(gpuQuotaTestSize), Capacity: to.Ptr[int64](1)},
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			OrchestrationMode: to.Ptr(armcompute.OrchestrationModeUniform),
		},
	}
}

type gpuQuotaTestAzure struct {
	vms        []*armcompute.VirtualMachine
	scaleSets  []*armcompute.VirtualMachineScaleSet
	live       map[string]any
	groups     map[string]armresources.ResourceGroup
	deleted    []string
	gpuCount   string
	requests   int
	failPath   string
	failMethod string
	failStatus int
	paginate   bool
	async      bool
	polls      int
}

type gpuQuotaTestSSHConn struct {
	ssh.Conn
	closed bool
}

func (c *gpuQuotaTestSSHConn) Close() error {
	c.closed = true
	return nil
}

func (a *gpuQuotaTestAzure) install(t *testing.T) {
	t.Helper()
	oldAzure, oldConfig, oldLock := config.Azure, config.Config, gpuQuotaRecoveryLock
	config.Config = config.DefaultConfiguration()
	config.Config.SubscriptionID = gpuQuotaRecoverySubscription
	gpuQuotaRecoveryLock = make(chan struct{}, 1)
	t.Cleanup(func() { config.Azure, config.Config, gpuQuotaRecoveryLock = oldAzure, oldConfig, oldLock })
	opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{
		PerCallPolicies: []policy.Policy{vmssCreationTestPolicy(func(req *http.Request) *http.Response {
			return a.respond(t, req)
		})},
		Retry: policy.RetryOptions{MaxRetries: -1},
	}}
	client := &config.AzureClient{}
	var err error
	client.ResourceSKUs, err = armcompute.NewResourceSKUsClient(gpuQuotaRecoverySubscription, nil, opts)
	require.NoError(t, err)
	client.VMs, err = armcompute.NewVirtualMachinesClient(gpuQuotaRecoverySubscription, nil, opts)
	require.NoError(t, err)
	client.VMSS, err = armcompute.NewVirtualMachineScaleSetsClient(gpuQuotaRecoverySubscription, nil, opts)
	require.NoError(t, err)
	client.ResourceGroup, err = armresources.NewResourceGroupsClient(gpuQuotaRecoverySubscription, nil, opts)
	require.NoError(t, err)
	config.Azure = client
}

func (a *gpuQuotaTestAzure) respond(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	a.requests++
	status := http.StatusOK
	header := http.Header{"Content-Type": {"application/json"}}
	var body any
	subscription := "/subscriptions/" + gpuQuotaRecoverySubscription
	switch {
	case req.Method == a.failMethod && req.URL.Path == a.failPath:
		status = http.StatusForbidden
		body = map[string]any{"error": map[string]string{"code": "AuthorizationFailed", "message": "denied"}}
		if a.failStatus == http.StatusNotFound {
			status = http.StatusNotFound
			body = map[string]any{"error": map[string]string{"code": "ResourceNotFound", "message": "not found"}}
		}
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/operations/delete"):
		a.polls++
		state := "InProgress"
		if a.polls >= 2 {
			state = "Succeeded"
		}
		body = map[string]string{"status": state}
	case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/skus"):
		require.Equal(t, "location eq 'westus3'", req.URL.Query().Get("$filter"))
		body = map[string]any{"value": []any{map[string]any{
			"name": gpuQuotaTestSize, "resourceType": "virtualMachines",
			"capabilities": []any{map[string]string{"name": "GPUs", "value": a.gpuCount}},
		}}}
	case req.Method == http.MethodGet && req.URL.Path == subscription+"/providers/Microsoft.Compute/virtualMachines":
		vms := []*armcompute.VirtualMachine{}
		for _, vm := range a.vms {
			if !slices.Contains(a.deleted, *vm.ID) {
				vms = append(vms, vm)
			}
		}
		body = map[string]any{"value": vms}
		if a.paginate {
			body = map[string]any{"value": vms, "nextLink": "https://management.azure.com" + subscription + "/vms-page2"}
		}
	case req.Method == http.MethodGet && req.URL.Path == subscription+"/vms-page2":
		body = map[string]any{"value": []*armcompute.VirtualMachine{gpuQuotaTestVM("scratch", "page2")}}
	case req.Method == http.MethodGet && req.URL.Path == subscription+"/providers/Microsoft.Compute/virtualMachineScaleSets":
		sets := []*armcompute.VirtualMachineScaleSet{}
		for _, set := range a.scaleSets {
			if !slices.Contains(a.deleted, *set.ID) {
				sets = append(sets, set)
			}
		}
		body = map[string]any{"value": sets}
	case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/resourcegroups/") &&
		!strings.Contains(strings.ToLower(req.URL.Path), "/providers/"):
		group := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		body = a.groups[group]
	case req.Method == http.MethodGet:
		body = a.live[req.URL.Path]
		if body == nil {
			for _, vm := range a.vms {
				if *vm.ID == req.URL.Path {
					body = vm
				}
			}
			for _, set := range a.scaleSets {
				if *set.ID == req.URL.Path {
					body = set
				}
			}
			if req.URL.Path == gpuQuotaTestID("scratch", "virtualMachines", "page2") {
				body = gpuQuotaTestVM("scratch", "page2")
			}
		}
		require.NotNil(t, body, "unexpected Azure GET: %s", req.URL)
	case req.Method == http.MethodDelete:
		a.deleted = append(a.deleted, req.URL.Path)
		status = http.StatusNoContent
		if a.async {
			status = http.StatusAccepted
			header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/delete")
		}
	default:
		t.Fatalf("unexpected Azure request: %s %s", req.Method, req.URL)
	}
	data := []byte{}
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		require.NoError(t, err)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(string(data))), Request: req}
}

func TestGPUQuotaRecoveryReclaimsExactSKUAndWaitsForDeletion(t *testing.T) {
	for _, scaleSet := range []bool{false, true} {
		t.Run(fmt.Sprintf("scaleSet=%v", scaleSet), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				vm := gpuQuotaTestVM("scratch", "external")
				set := gpuQuotaTestVMSS("scratch", "external-set")
				a := &gpuQuotaTestAzure{gpuCount: "1", async: true}
				expectedID := *vm.ID
				if scaleSet {
					a.scaleSets = []*armcompute.VirtualMachineScaleSet{set}
					expectedID = *set.ID
				} else {
					a.vms = []*armcompute.VirtualMachine{vm}
				}
				a.install(t)
				calls := 0
				wantVM := &ScenarioVM{}
				conn := &gpuQuotaTestSSHConn{}
				failedVM := &ScenarioVM{SSHClient: &SSHClient{Client: &ssh.Client{Conn: conn}}}
				got, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
					calls++
					if calls == 1 {
						quotaErr := gpuQuotaTestCoreError()
						if scaleSet {
							quotaErr = gpuQuotaTestError(200, "OperationNotAllowed", "exceeding approved vCPU quota")
						}
						return failedVM, fmt.Errorf("create VMSS: %w", quotaErr)
					}
					require.Equal(t, []string{expectedID}, a.deleted)
					require.GreaterOrEqual(t, a.polls, 2, "retry must wait for deletion to finish")
					require.True(t, conn.closed, "close the preceding attempt's tunnel before replacing its VM")
					return wantVM, nil
				})
				require.NoError(t, err)
				require.Same(t, wantVM, got)
				require.Equal(t, 2, calls)
			})
		})
	}
}

func TestGPUQuotaRecoveryRejectsUnrelatedErrorsAndSubscriptions(t *testing.T) {
	for _, tc := range []struct {
		name         string
		subscription string
		err          error
	}{
		{name: "allocation", err: gpuQuotaTestError(200, "AllocationFailed", "exceeding approved cores quota")},
		{name: "SKU capacity", err: gpuQuotaTestError(409, "SkuNotAvailable", "SKU is unavailable")},
		{name: "VM count quota", err: gpuQuotaTestError(409, "OperationNotAllowed", "exceeding approved VM count quota")},
		{name: "unrelated operation", err: gpuQuotaTestError(409, "OperationNotAllowed", "another operation is pending")},
		{name: "server error", err: gpuQuotaTestError(500, "OperationNotAllowed", "exceeding approved cores quota")},
		{name: "other subscription", subscription: "other", err: gpuQuotaTestCoreError()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &gpuQuotaTestAzure{}
			a.install(t)
			if tc.subscription != "" {
				config.Config.SubscriptionID = tc.subscription
			}
			calls := 0
			_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
				calls++
				return nil, tc.err
			})
			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, 1, calls)
			assert.Zero(t, a.requests, "ineligible failures must not inventory or delete resources")
		})
	}
	t.Run("CPU size", func(t *testing.T) {
		a := &gpuQuotaTestAzure{gpuCount: "0", vms: []*armcompute.VirtualMachine{gpuQuotaTestVM("scratch", "external")}}
		a.install(t)
		quotaErr := gpuQuotaTestCoreError()
		_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
			return nil, quotaErr
		})
		require.ErrorIs(t, err, quotaErr)
		assert.Equal(t, 1, a.requests, "only the GPU capability lookup is allowed")
		assert.Empty(t, a.deleted)
	})
}

func TestGPUQuotaRecoveryProtectsOtherResources(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		otherSKU := gpuQuotaTestVM("scratch", "other-sku")
		otherSKU.Properties.HardwareProfile.VMSize = to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_NC8as_T4_v3"))
		otherRegion := gpuQuotaTestVM("scratch", "other-region")
		otherRegion.Location = to.Ptr("eastus")
		abName := gpuQuotaTestVM("scratch", "ABE2E-running")
		abGroup := gpuQuotaTestVM("abe2e-westus3", "running")
		abTagged := gpuQuotaTestVM("scratch", "tagged")
		abTagged.Tags = map[string]*string{"BUILDID": to.Ptr("123")}
		kept := gpuQuotaTestVM("scratch", "kept")
		kept.Tags = map[string]*string{"keep_vmss": to.Ptr("true")}
		managed := gpuQuotaTestVM("scratch", "managed")
		managed.Tags = map[string]*string{"aks-managed-poolName": to.Ptr("gpu")}
		instance := gpuQuotaTestVM("scratch", "flex-instance")
		instance.Properties.VirtualMachineScaleSet = &armcompute.SubResource{ID: to.Ptr("parent")}
		spot := gpuQuotaTestVM("scratch", "spot")
		spot.Properties.Priority = to.Ptr(armcompute.VirtualMachinePriorityTypesSpot)
		otherSub := gpuQuotaTestVM("scratch", "other-sub")
		otherSub.ID = to.Ptr(strings.Replace(*otherSub.ID, gpuQuotaRecoverySubscription, "other", 1))
		managedGroup := gpuQuotaTestVM("managed", "gpu")
		taggedGroup := gpuQuotaTestVM("ab-tagged", "gpu")
		renamedAKSGroup := gpuQuotaTestVM("MC_cluster", "gpu")
		changed := gpuQuotaTestVM("scratch", "changed")
		current := gpuQuotaTestVM("scratch", "changed")
		current.Properties.HardwareProfile.VMSize = otherSKU.Properties.HardwareProfile.VMSize
		flex := gpuQuotaTestVMSS("scratch", "flex")
		flex.Properties.OrchestrationMode = to.Ptr(armcompute.OrchestrationModeFlexible)
		empty := gpuQuotaTestVMSS("scratch", "empty")
		empty.SKU.Capacity = to.Ptr[int64](0)
		a := &gpuQuotaTestAzure{
			gpuCount: "1",
			vms: []*armcompute.VirtualMachine{otherSKU, otherRegion, abName, abGroup, abTagged, kept, managed,
				instance, spot, otherSub, managedGroup, taggedGroup, renamedAKSGroup, changed},
			scaleSets: []*armcompute.VirtualMachineScaleSet{flex, empty},
			live:      map[string]any{*changed.ID: current},
			groups: map[string]armresources.ResourceGroup{
				"managed":   {ManagedBy: to.Ptr("/providers/Microsoft.ContainerService/managedClusters/cluster")},
				"ab-tagged": {Tags: map[string]*string{"buildID": to.Ptr("123")}},
			},
		}
		a.install(t)
		calls := 0
		quotaErr := gpuQuotaTestCoreError()
		_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
			calls++
			return nil, quotaErr
		})
		require.ErrorIs(t, err, quotaErr)
		require.ErrorContains(t, err, "GPU quota recovery exhausted")
		assert.Equal(t, 4, calls)
		assert.Empty(t, a.deleted)
	})
}

func TestGPUQuotaRecoveryRetriesAtMostThreeTimes(t *testing.T) {
	for _, succeeds := range []bool{false, true} {
		t.Run(fmt.Sprintf("succeeds=%v", succeeds), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				a := &gpuQuotaTestAzure{gpuCount: "1", vms: []*armcompute.VirtualMachine{
					gpuQuotaTestVM("scratch", "one"), gpuQuotaTestVM("scratch", "two"),
					gpuQuotaTestVM("scratch", "three"), gpuQuotaTestVM("scratch", "four"),
				}}
				a.install(t)
				calls := 0
				quotaErr := gpuQuotaTestCoreError()
				_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
					calls++
					if succeeds && calls == 4 {
						return &ScenarioVM{}, nil
					}
					return nil, quotaErr
				})
				if succeeds {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, quotaErr)
					require.ErrorContains(t, err, "GPU quota recovery exhausted after 3 attempts")
				}
				assert.Equal(t, 4, calls)
				assert.Len(t, a.deleted, 3, "reclaim only one matching resource before each retry")
				assert.NotContains(t, a.deleted, gpuQuotaTestID("scratch", "virtualMachines", "four"))
			})
		})
	}
}

func TestGPUQuotaRecoveryBudgetSurvivesProvisioningRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := &gpuQuotaTestAzure{gpuCount: "1", vms: []*armcompute.VirtualMachine{
			gpuQuotaTestVM("scratch", "one"), gpuQuotaTestVM("scratch", "two"),
			gpuQuotaTestVM("scratch", "three"), gpuQuotaTestVM("scratch", "four"),
		}}
		a.install(t)
		scenario := &Scenario{gpuQuotaRecovery: new(gpuQuotaRecovery)}
		model := *gpuQuotaTestVMSS("abe2e-westus3", "test")
		galleryErr := gpuQuotaTestError(http.StatusNotFound, "GalleryImageNotFound", "image not yet propagated")
		calls := 0
		bake := freshScenario(scenario)
		_, err := bake.gpuQuotaRecovery.createVMSS(t.Context(), model, func() (*ScenarioVM, error) {
			calls++
			if calls == 3 {
				return nil, galleryErr
			}
			return nil, gpuQuotaTestCoreError()
		})
		require.ErrorIs(t, err, galleryErr)
		require.Len(t, a.deleted, 2)

		quotaErr := gpuQuotaTestCoreError()
		calls = 0
		provision := freshScenario(scenario)
		_, err = provision.gpuQuotaRecovery.createVMSS(t.Context(), model, func() (*ScenarioVM, error) {
			calls++
			return nil, quotaErr
		})
		require.ErrorIs(t, err, quotaErr)
		assert.ErrorContains(t, err, "GPU quota recovery exhausted after 3 attempts")
		assert.Equal(t, 2, calls, "an outer provisioning retry must not reset the quota budget")
		assert.Len(t, a.deleted, 3)
		assert.NotContains(t, a.deleted, gpuQuotaTestID("scratch", "virtualMachines", "four"))
	})
}

func TestGPUQuotaRecoveryPropagatesInventoryAndDeletionErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "later inventory page", method: http.MethodGet, path: "/subscriptions/" + gpuQuotaRecoverySubscription + "/vms-page2"},
		{name: "resource group", method: http.MethodGet, path: "/subscriptions/" + gpuQuotaRecoverySubscription + "/resourcegroups/scratch"},
		{name: "deletion", method: http.MethodDelete, path: gpuQuotaTestID("scratch", "virtualMachines", "external")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &gpuQuotaTestAzure{
				gpuCount: "1", paginate: true, failMethod: tc.method, failPath: tc.path,
				vms: []*armcompute.VirtualMachine{gpuQuotaTestVM("scratch", "external")},
			}
			a.install(t)
			quotaErr := gpuQuotaTestCoreError()
			calls := 0
			_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
				calls++
				return nil, quotaErr
			})
			require.ErrorIs(t, err, quotaErr)
			require.ErrorContains(t, err, "AuthorizationFailed")
			config.Config.SkipTestsWithSKUCapacityIssue = true
			require.NoError(t, skipIfSKUNotAvailableErr(err, false), "reclamation failures must fail even in the PR gate")
			assert.Equal(t, 1, calls)
			assert.Empty(t, a.deleted)
		})
	}
}

func TestGPUQuotaRecoveryRejectsInvalidMetadata(t *testing.T) {
	for _, tc := range []struct {
		name     string
		gpuCount string
		mutate   func(*armcompute.VirtualMachine)
		wantErr  string
	}{
		{name: "unknown GPU count", gpuCount: "unknown", wantErr: "invalid GPU count"},
		{name: "invalid ID", gpuCount: "1", mutate: func(vm *armcompute.VirtualMachine) { vm.ID = to.Ptr("invalid") }, wantErr: "invalid GPU resource ID"},
		{name: "name mismatch", gpuCount: "1", mutate: func(vm *armcompute.VirtualMachine) { vm.Name = to.Ptr("another") }, wantErr: "inconsistent GPU resource name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := gpuQuotaTestVM("scratch", "external")
			if tc.mutate != nil {
				tc.mutate(vm)
			}
			a := &gpuQuotaTestAzure{gpuCount: tc.gpuCount, vms: []*armcompute.VirtualMachine{vm}}
			a.install(t)
			quotaErr := gpuQuotaTestCoreError()
			_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
				return nil, quotaErr
			})
			require.ErrorIs(t, err, quotaErr)
			require.ErrorContains(t, err, tc.wantErr)
			assert.Empty(t, a.deleted)
		})
	}
}

func TestGPUQuotaRecoveryFindsLaterInventoryPage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := &gpuQuotaTestAzure{gpuCount: "1", paginate: true}
		a.install(t)
		calls := 0
		_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
			calls++
			if calls == 1 {
				return nil, gpuQuotaTestCoreError()
			}
			return &ScenarioVM{}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, []string{gpuQuotaTestID("scratch", "virtualMachines", "page2")}, a.deleted)
	})
}

func TestGPUQuotaRecoveryStopsOnDeletionPollingFailure(t *testing.T) {
	a := &gpuQuotaTestAzure{
		gpuCount: "1", async: true, failMethod: http.MethodGet, failPath: "/operations/delete",
		vms: []*armcompute.VirtualMachine{gpuQuotaTestVM("scratch", "external")},
	}
	a.install(t)
	quotaErr := gpuQuotaTestCoreError()
	calls := 0
	_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
		calls++
		return nil, quotaErr
	})
	require.ErrorIs(t, err, quotaErr)
	require.ErrorContains(t, err, "AuthorizationFailed")
	assert.Equal(t, 1, calls, "must not create another VM before deletion is confirmed")
	assert.Len(t, a.deleted, 1)
}

func TestGPUQuotaRecoveryDoesNotTreatPolling404AsDeletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := &gpuQuotaTestAzure{
			gpuCount: "1", async: true, failMethod: http.MethodGet,
			failPath: "/operations/delete", failStatus: http.StatusNotFound,
			vms: []*armcompute.VirtualMachine{
				gpuQuotaTestVM("scratch", "first"), gpuQuotaTestVM("scratch", "second"),
			},
		}
		a.install(t)
		calls := 0
		quotaErr := gpuQuotaTestCoreError()
		_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
			calls++
			return nil, quotaErr
		})
		require.ErrorIs(t, err, quotaErr)
		assert.ErrorContains(t, err, "ResourceNotFound")
		assert.Equal(t, 1, calls, "an unknown deletion result must stop recovery")
		assert.Equal(t, []string{gpuQuotaTestID("scratch", "virtualMachines", "first")}, a.deleted,
			"do not delete another resource when the first deletion is unconfirmed")
	})
}

func TestGPUQuotaRecoveryRetriesAfterCandidateDisappears(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "resource GET", method: http.MethodGet, path: gpuQuotaTestID("scratch", "virtualMachines", "first")},
		{name: "resource DELETE", method: http.MethodDelete, path: gpuQuotaTestID("scratch", "virtualMachines", "first")},
		{name: "resource group GET", method: http.MethodGet, path: "/subscriptions/" + gpuQuotaRecoverySubscription + "/resourcegroups/scratch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				a := &gpuQuotaTestAzure{
					gpuCount: "1", failMethod: tc.method,
					failPath: tc.path, failStatus: http.StatusNotFound,
					vms: []*armcompute.VirtualMachine{
						gpuQuotaTestVM("scratch", "first"), gpuQuotaTestVM("scratch", "second"),
					},
				}
				a.install(t)
				calls := 0
				_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
					calls++
					if calls == 1 {
						return nil, gpuQuotaTestCoreError()
					}
					return &ScenarioVM{}, nil
				})
				require.NoError(t, err)
				assert.Empty(t, a.deleted, "a concurrent deletion may already have freed quota; retry before another eviction")
			})
		})
	}
}

func TestGPUQuotaRecoveryIgnoresQuotaTermsInRequestURL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := &gpuQuotaTestAzure{gpuCount: "1"}
		a.install(t)
		err := gpuQuotaTestError(http.StatusConflict, "OperationNotAllowed", "exceeding approved VM count quota")
		var responseErr *azcore.ResponseError
		require.ErrorAs(t, err, &responseErr)
		requestURL, parseErr := url.Parse("https://management.azure.com" + gpuQuotaTestID("scratch", "virtualMachineScaleSets", "gpu-cores-test"))
		require.NoError(t, parseErr)
		responseErr.RawResponse.Request = &http.Request{Method: http.MethodPut, URL: requestURL}
		_, got := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
			return nil, err
		})
		require.ErrorIs(t, got, err)
		assert.Zero(t, a.requests, "quota classification must inspect the ARM error, not tokens in the request URL")
	})
}

func TestGPUQuotaRecoveryWaiterRetriesBeforeEvicting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := &gpuQuotaTestAzure{gpuCount: "1", vms: []*armcompute.VirtualMachine{gpuQuotaTestVM("scratch", "external")}}
		a.install(t)
		gpuQuotaRecoveryLock <- struct{}{}
		done := make(chan error, 1)
		calls := 0
		go func() {
			_, err := new(gpuQuotaRecovery).createVMSS(t.Context(), *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
				calls++
				if calls == 1 {
					return nil, gpuQuotaTestCoreError()
				}
				return &ScenarioVM{}, nil
			})
			done <- err
		}()
		synctest.Wait()
		assert.Equal(t, 1, calls)
		<-gpuQuotaRecoveryLock
		require.NoError(t, <-done)
		assert.Equal(t, 2, calls)
		assert.Empty(t, a.deleted)
	})
}

func TestGPUQuotaRecoveryCancellationDoesNotDelete(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := &gpuQuotaTestAzure{gpuCount: "1", vms: []*armcompute.VirtualMachine{gpuQuotaTestVM("scratch", "external")}}
		a.install(t)
		gpuQuotaRecoveryLock <- struct{}{}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() {
			_, err := new(gpuQuotaRecovery).createVMSS(ctx, *gpuQuotaTestVMSS("abe2e-westus3", "test"), func() (*ScenarioVM, error) {
				return nil, gpuQuotaTestCoreError()
			})
			done <- err
		}()
		synctest.Wait()
		cancel()
		require.ErrorIs(t, <-done, context.Canceled)
		<-gpuQuotaRecoveryLock
		assert.Empty(t, a.deleted)
	})
}
