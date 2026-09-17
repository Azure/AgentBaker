package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gcTestAzure(t *testing.T, respond vmssCreationTestPolicy) {
	t.Helper()
	previousAzure, previousCache := config.Azure, cachedEnsureResourceGroup
	t.Cleanup(func() {
		config.Azure, cachedEnsureResourceGroup = previousAzure, previousCache
	})
	cachedEnsureResourceGroup = cachedFunc(ensureResourceGroup)
	opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}}}
	client := &config.AzureClient{}
	var err error
	client.ResourceGroup, err = armresources.NewResourceGroupsClient("test", nil, opts)
	require.NoError(t, err)
	client.Tags, err = armresources.NewTagsClient("test", nil, opts)
	require.NoError(t, err)
	client.AKS, err = armcontainerservice.NewManagedClustersClient("test", nil, opts)
	require.NoError(t, err)
	client.VMSS, err = armcompute.NewVirtualMachineScaleSetsClient("test", nil, opts)
	require.NoError(t, err)
	config.Azure = client
}

func gcTestResponse(req *http.Request, status int, body any) *http.Response {
	data, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(data))),
		Request:    req,
	}
}

func gcTestContext(t *testing.T) (context.Context, time.Time) {
	t.Helper()
	previousDeadline := config.Config.SuiteDeadline
	t.Cleanup(func() { config.Config.SuiteDeadline = previousDeadline })
	config.Config.SuiteDeadline = time.Now().Add(config.Config.SuiteTimeout)
	return logging.WithLogger(t.Context(), t), config.Config.SuiteDeadline.Add(CleanupTimeout)
}

func TestGCDeadlineRequiresConfiguredDeadline(t *testing.T) {
	previousDeadline := config.Config.SuiteDeadline
	t.Cleanup(func() { config.Config.SuiteDeadline = previousDeadline })
	config.Config.SuiteDeadline = time.Time{}
	gcTestAzure(t, func(req *http.Request) *http.Response {
		t.Errorf("unexpected ARM call: %s %s", req.Method, req.URL)
		return gcTestResponse(req, http.StatusInternalServerError, nil)
	})
	err := extendResourceGroupDeadline(context.Background(), "rg")
	require.ErrorContains(t, err, "suite deadline is required")
}

func TestExtendResourceGroupDeadline(t *testing.T) {
	for _, tt := range []struct {
		name      string
		tag       *string
		present   bool
		offset    time.Duration
		useOffset bool
		readCode  int
		writeCode int
		deleting  bool
		wantWrite bool
		wantError string
	}{
		{name: "missing", wantWrite: true},
		{name: "expired", present: true, tag: to.Ptr("2000-01-01T00:00:00Z"), wantWrite: true},
		{name: "earlier", present: true, useOffset: true, offset: -time.Hour, wantWrite: true},
		{name: "equal", present: true, useOffset: true},
		{name: "later", present: true, useOffset: true, offset: time.Hour},
		{name: "null", present: true, wantError: "null deletion_due_time"},
		{name: "empty", present: true, tag: to.Ptr(""), wantError: "parsing deletion_due_time"},
		{name: "invalid", present: true, tag: to.Ptr("not-a-time"), wantError: "parsing deletion_due_time"},
		{name: "read forbidden", readCode: http.StatusForbidden, wantError: "reading GC deadline"},
		{name: "RG missing", readCode: http.StatusNotFound, wantError: "reading GC deadline"},
		{name: "write forbidden", writeCode: http.StatusForbidden, wantWrite: true, wantError: "renewing GC deadline"},
		{name: "deleting", deleting: true, wantError: "deleting RG"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, due := gcTestContext(t)
			tags := map[string]*string{"owner": to.Ptr("keep"), "gc_lease": to.Ptr("1h")}
			if tt.present {
				tags[deletionDueTimeTag] = tt.tag
				if tt.useOffset {
					tags[deletionDueTimeTag] = to.Ptr(due.Add(tt.offset).Format(time.RFC3339Nano))
				}
			}
			rg := armresources.ResourceGroup{
				ID: to.Ptr("/subscriptions/test/resourceGroups/rg"), Tags: tags,
				Properties: &armresources.ResourceGroupProperties{ProvisioningState: to.Ptr("Succeeded")},
			}
			if tt.deleting {
				rg.Properties.ProvisioningState = to.Ptr("Deleting")
			}
			writes := 0
			gcTestAzure(t, func(req *http.Request) *http.Response {
				if req.Method == http.MethodGet {
					if tt.readCode != 0 {
						return gcTestResponse(req, tt.readCode, nil)
					}
					return gcTestResponse(req, http.StatusOK, rg)
				}
				require.Equal(t, http.MethodPatch, req.Method)
				require.Equal(t, "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Resources/tags/default", req.URL.Path)
				var patch armresources.TagsPatchResource
				require.NoError(t, json.NewDecoder(req.Body).Decode(&patch))
				require.Equal(t, armresources.TagsPatchOperationMerge, *patch.Operation)
				require.Equal(t, map[string]*string{deletionDueTimeTag: to.Ptr(due.UTC().Format(time.RFC3339Nano))}, patch.Properties.Tags)
				writes++
				if tt.writeCode != 0 {
					return gcTestResponse(req, tt.writeCode, nil)
				}
				// An independent writer adds a tag after our GET. Merge retains it.
				tags["concurrent"] = to.Ptr("keep-too")
				tags[deletionDueTimeTag] = patch.Properties.Tags[deletionDueTimeTag]
				return gcTestResponse(req, http.StatusOK, armresources.TagsResource{Properties: &armresources.Tags{Tags: tags}})
			})
			err := extendResourceGroupDeadline(ctx, "rg")
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantWrite, writes == 1)
			assert.Equal(t, "keep", *tags["owner"])
			assert.Equal(t, "1h", *tags["gc_lease"])
			if tt.wantWrite && tt.writeCode == 0 {
				assert.Equal(t, "keep-too", *tags["concurrent"])
			}
		})
	}
}

func TestGCDeadlineRenewalFailureAllowsResourceGroupUse(t *testing.T) {
	ctx, _ := gcTestContext(t)
	logger := &executionLogger{}
	ctx = logging.WithLogger(ctx, logger)
	writes := 0
	gcTestAzure(t, func(req *http.Request) *http.Response {
		if req.Method == http.MethodPatch {
			writes++
			return gcTestResponse(req, http.StatusForbidden, nil)
		}
		require.Equal(t, http.MethodGet, req.Method)
		return gcTestResponse(req, http.StatusOK, armresources.ResourceGroup{
			ID: to.Ptr("/subscriptions/test/resourceGroups/rg"), Name: to.Ptr("rg"),
		})
	})
	for range 2 {
		rg, err := CachedEnsureResourceGroup(ctx, "westus3")
		require.NoError(t, err)
		require.Equal(t, "rg", *rg.Name)
	}
	assert.Equal(t, 2, writes, "failed renewal must not be cached")
	require.Contains(t, strings.Join(logger.logs, "\n"), "warning: failed to renew resource group")
}

func TestGCDeadlineRenewalFailureAllowsClusterUse(t *testing.T) {
	for _, state := range []string{"Succeeded", "Starting", "Updating"} {
		t.Run(state, func(t *testing.T) {
			ctx, _ := gcTestContext(t)
			logger := &executionLogger{}
			ctx = logging.WithLogger(ctx, logger)
			clusterReads := 0
			gcTestAzure(t, func(req *http.Request) *http.Response {
				switch {
				case strings.Contains(req.URL.Path, "/managedClusters/"):
					clusterReads++
					currentState := state
					if clusterReads > 1 {
						currentState = "Succeeded"
					}
					return gcTestResponse(req, http.StatusOK, armcontainerservice.ManagedCluster{
						Properties: &armcontainerservice.ManagedClusterProperties{
							ProvisioningState: to.Ptr(currentState), NodeResourceGroup: to.Ptr("actual-node-rg"),
						},
					})
				case strings.Contains(req.URL.Path, "/virtualMachineScaleSets"):
					return gcTestResponse(req, http.StatusOK, map[string]any{"value": []any{map[string]any{"name": "system"}}})
				case strings.HasSuffix(strings.ToLower(req.URL.Path), "/resourcegroups/actual-node-rg"):
					return gcTestResponse(req, http.StatusOK, armresources.ResourceGroup{ID: to.Ptr("/subscriptions/test/resourceGroups/actual-node-rg")})
				case req.Method == http.MethodPatch:
					return gcTestResponse(req, http.StatusForbidden, nil)
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL)
					return gcTestResponse(req, http.StatusForbidden, nil)
				}
			})
			cluster, err := getExistingCluster(ctx, "westus3", "cluster")
			require.NoError(t, err)
			require.NotNil(t, cluster)
			require.Contains(t, strings.Join(logger.logs, "\n"), `warning: failed to renew resource group "actual-node-rg" GC deadline`)
		})
	}
}

func TestGCDeadlinePropagatesAsyncUpdateFailure(t *testing.T) {
	ctx, _ := gcTestContext(t)
	gcTestAzure(t, func(req *http.Request) *http.Response {
		if req.URL.Path == "/operations/renew" {
			return gcTestResponse(req, http.StatusOK, map[string]any{
				"status": "Failed", "error": map[string]string{"code": "RenewalFailed", "message": "tag update failed"},
			})
		}
		if req.Method == http.MethodGet {
			return gcTestResponse(req, http.StatusOK, armresources.ResourceGroup{ID: to.Ptr("/subscriptions/test/resourceGroups/rg")})
		}
		response := gcTestResponse(req, http.StatusAccepted, nil)
		response.Header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/renew")
		return response
	})
	require.ErrorContains(t, extendResourceGroupDeadline(ctx, "rg"), "waiting for GC deadline renewal")
}

func TestGCDeadlineRechecksRecreatedNodeResourceGroup(t *testing.T) {
	ctx, due := gcTestContext(t)
	var tag *string
	writes := 0
	gcTestAzure(t, func(req *http.Request) *http.Response {
		if req.Method == http.MethodGet {
			tags := map[string]*string{}
			if tag != nil {
				tags[deletionDueTimeTag] = tag
			}
			return gcTestResponse(req, http.StatusOK, armresources.ResourceGroup{ID: to.Ptr("/subscriptions/test/resourceGroups/rg"), Tags: tags})
		}
		writes++
		tag = to.Ptr(due.UTC().Format(time.RFC3339Nano))
		return gcTestResponse(req, http.StatusOK, armresources.TagsResource{})
	})
	renewResourceGroupDeadline(ctx, "rg")
	tag = nil // Cluster recreation can reuse the RG name after its deletion.
	renewNodeResourceGroupDeadline(ctx, &armcontainerservice.ManagedCluster{
		Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr("rg")},
	})
	assert.Equal(t, 2, writes)
	assert.Equal(t, due.UTC().Format(time.RFC3339Nano), *tag)
}

func TestGCDeadlineProtectsBothResourceGroups(t *testing.T) {
	for _, newCluster := range []bool{false, true} {
		t.Run(fmt.Sprintf("new=%t", newCluster), func(t *testing.T) {
			ctx, due := gcTestContext(t)
			parentName := config.ResourceGroupName("westus3")
			nodeName := "actual-node-rg"
			model := armcontainerservice.ManagedCluster{
				Name: to.Ptr("cluster"), Location: to.Ptr("westus3"),
				Properties: &armcontainerservice.ManagedClusterProperties{
					ProvisioningState: to.Ptr("Succeeded"), NodeResourceGroup: to.Ptr(nodeName),
				},
			}
			tags := map[string]map[string]*string{
				parentName: {"owner": to.Ptr("parent")},
				nodeName:   {"owner": to.Ptr("node")},
			}
			var mu sync.Mutex
			writes := map[string]int{}
			createdRG, createdCluster := false, false
			gcTestAzure(t, func(req *http.Request) *http.Response {
				mu.Lock()
				defer mu.Unlock()
				parts := strings.Split(req.URL.Path, "/")
				rgName := parts[4]
				switch {
				case strings.Contains(req.URL.Path, "/managedClusters/"):
					if req.Method == http.MethodPut {
						require.True(t, newCluster)
						createdCluster = true
						return gcTestResponse(req, http.StatusCreated, model)
					}
					if newCluster && !createdCluster {
						return gcTestResponse(req, http.StatusNotFound, nil)
					}
					return gcTestResponse(req, http.StatusOK, model)
				case strings.Contains(req.URL.Path, "/virtualMachineScaleSets"):
					require.Contains(t, tags[nodeName], deletionDueTimeTag, "node RG must be renewed before VMSS access")
					return gcTestResponse(req, http.StatusOK, map[string]any{"value": []any{map[string]any{"name": "system"}}})
				case strings.HasSuffix(req.URL.Path, "/tags/default"):
					require.Equal(t, http.MethodPatch, req.Method)
					var patch armresources.TagsPatchResource
					require.NoError(t, json.NewDecoder(req.Body).Decode(&patch))
					require.Equal(t, armresources.TagsPatchOperationMerge, *patch.Operation)
					writes[rgName]++
					tags[rgName][deletionDueTimeTag] = patch.Properties.Tags[deletionDueTimeTag]
					return gcTestResponse(req, http.StatusOK, armresources.TagsResource{Properties: &armresources.Tags{Tags: tags[rgName]}})
				default:
					if req.Method == http.MethodPut {
						require.True(t, newCluster, "existing parent RG must not be PUT")
						var rg armresources.ResourceGroup
						require.NoError(t, json.NewDecoder(req.Body).Decode(&rg))
						assert.Empty(t, rg.Tags)
						createdRG = true
					} else {
						require.Equal(t, http.MethodGet, req.Method)
						if newCluster && !createdRG {
							return gcTestResponse(req, http.StatusNotFound, nil)
						}
					}
					return gcTestResponse(req, http.StatusOK, armresources.ResourceGroup{
						ID: to.Ptr("/subscriptions/test/resourceGroups/" + rgName), Tags: tags[rgName],
					})
				}
			})
			getCluster := cachedFunc(func(ctx context.Context, _ string) (*armcontainerservice.ManagedCluster, error) {
				return getOrCreateCluster(ctx, &model)
			})
			// Later scenarios and native retries reuse the cluster but check renewal.
			for range 2 {
				var group sync.WaitGroup
				for range 4 {
					group.Go(func() {
						_, err := CachedEnsureResourceGroup(ctx, "westus3")
						if !assert.NoError(t, err) {
							return
						}
						cluster, err := getCluster(ctx, "cluster")
						if assert.NoError(t, err) {
							renewResourceGroupDeadline(ctx, *cluster.Properties.NodeResourceGroup)
						}
					})
				}
				group.Wait()
			}
			require.Equal(t, due.UTC().Format(time.RFC3339Nano), *tags[parentName][deletionDueTimeTag])
			require.Equal(t, due.UTC().Format(time.RFC3339Nano), *tags[nodeName][deletionDueTimeTag])
			assert.Positive(t, writes[nodeName])
			assert.Equal(t, "node", *tags[nodeName]["owner"])
			if !newCluster {
				assert.Positive(t, writes[parentName])
				assert.Equal(t, "parent", *tags[parentName]["owner"])
			}
			// A later suite must renew even though infrastructure creation is cached.
			nodeWrites := writes[nodeName]
			config.Config.SuiteDeadline = due.Add(time.Hour)
			laterCtx := logging.WithLogger(t.Context(), t)
			_, err := CachedEnsureResourceGroup(laterCtx, "westus3")
			require.NoError(t, err)
			cluster, err := getCluster(laterCtx, "cluster")
			require.NoError(t, err)
			renewResourceGroupDeadline(laterCtx, *cluster.Properties.NodeResourceGroup)
			require.Equal(t, due.Add(time.Hour+CleanupTimeout).UTC().Format(time.RFC3339Nano), *tags[parentName][deletionDueTimeTag])
			require.Equal(t, due.Add(time.Hour+CleanupTimeout).UTC().Format(time.RFC3339Nano), *tags[nodeName][deletionDueTimeTag])
			assert.Equal(t, nodeWrites+1, writes[nodeName])
		})
	}
}
