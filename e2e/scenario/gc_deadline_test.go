package scenario

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	"github.com/stretchr/testify/require"
)

func gcTestAzure(t *testing.T, respond vmssCreationTestPolicy) *config.AzureClient {
	t.Helper()
	opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}}}
	client := &config.AzureClient{}
	var err error
	client.ResourceGroup, err = armresources.NewResourceGroupsClient("test", nil, opts)
	require.NoError(t, err)
	client.Tags, err = armresources.NewTagsClient("test", nil, opts)
	require.NoError(t, err)
	return client
}

func gcResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

func TestResourceGroupDeadline(t *testing.T) {
	for _, tt := range []struct {
		name, value, failure string
		write                bool
	}{
		{name: "missing", write: true},
		{name: "expired", value: `"2000-01-01T00:00:00Z"`, write: true},
		{name: "equal"},
		{name: "later"},
		{name: "null", value: "null", failure: "null deletion_due_time"},
		{name: "invalid", value: `"bad"`, failure: "parsing deletion_due_time"},
		{name: "read failure", failure: "reading GC deadline"},
		{name: "node RG not created yet", failure: "reading GC deadline"},
		{name: "write failure", failure: "renewing GC deadline", write: true},
		{name: "poll failure", failure: "waiting for GC deadline renewal", write: true},
		{name: "deleting", failure: "deleting RG"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				cfg := &config.Configuration{SuiteTimeout: 47 * time.Minute}
				writes := 0
				azure := gcTestAzure(t, func(req *http.Request) *http.Response {
					due := time.Now().Add(cfg.SuiteTimeout + CleanupTimeout)
					if req.URL.Path == "/operations/renew" {
						return gcResponse(req, 200, `{"status":"Failed","error":{"code":"Denied","message":"failed"}}`)
					}
					if req.Method == http.MethodGet {
						if tt.name == "read failure" {
							return gcResponse(req, 403, `{}`)
						}
						if tt.name == "node RG not created yet" {
							return gcResponse(req, 404, `{}`)
						}
						value := tt.value
						if tt.name == "equal" || tt.name == "later" {
							if tt.name == "later" {
								due = due.Add(time.Hour)
							}
							value = `"` + due.UTC().Format(time.RFC3339Nano) + `"`
						}
						tag := ""
						if value != "" {
							tag = `,"deletion_due_time":` + value
						}
						state := "Succeeded"
						if tt.name == "deleting" {
							state = "Deleting"
						}
						return gcResponse(req, 200, `{"name":"rg","id":"/subscriptions/test/resourceGroups/rg","properties":{"provisioningState":"`+state+`"},"tags":{"owner":"keep"`+tag+`}}`)
					}
					require.Equal(t, http.MethodPatch, req.Method)
					var patch armresources.TagsPatchResource
					require.NoError(t, json.NewDecoder(req.Body).Decode(&patch))
					require.Equal(t, armresources.TagsPatchOperationMerge, *patch.Operation)
					require.Equal(t, map[string]*string{deletionDueTimeTag: to.Ptr(due.UTC().Format(time.RFC3339Nano))}, patch.Properties.Tags)
					writes++
					if tt.name == "write failure" {
						return gcResponse(req, 403, `{}`)
					}
					response := gcResponse(req, 200, `{"properties":{"tags":{}}}`)
					if tt.name == "poll failure" {
						response.StatusCode = 202
						response.Header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/renew")
					}
					return response
				})
				logger := &executionLogger{}
				renewed := renewNodeResourceGroupDeadline(logging.WithLogger(t.Context(), logger), azure, cfg, &armcontainerservice.ManagedCluster{
					Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr("rg")},
				})
				require.Equal(t, tt.failure == "", renewed)
				if tt.write {
					require.Equal(t, 1, writes)
				} else {
					require.Zero(t, writes)
				}
				if tt.failure != "" {
					require.Contains(t, strings.Join(logger.logs, "\n"), "warning:")
					require.Contains(t, strings.Join(logger.logs, "\n"), tt.failure)
				} else {
					require.NotContains(t, strings.Join(logger.logs, "\n"), "warning:")
				}
				if tt.name == "deleting" {
					_, err := ensureResourceGroup(t.Context(), azure, "westus3")
					require.ErrorContains(t, err, "is deleting")
				}
			})
		})
	}
}

func TestCachedResourceGroupSetupRenewsOnlyNodeDeadline(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		cfg := &config.Configuration{SuiteTimeout: 47 * time.Minute}
		expired := "2000-01-01T00:00:00Z"
		tags := map[string]string{"abe2e-westus3": expired, "unused-node-rg": expired}
		reads := map[string]int{}
		writes := map[string]int{}
		azure := gcTestAzure(t, func(req *http.Request) *http.Response {
			rg := strings.Split(req.URL.Path, "/")[4]
			if req.Method == http.MethodGet {
				reads[rg]++
				tag := ""
				if tags[rg] != "" {
					tag = `"deletion_due_time":"` + tags[rg] + `"`
				}
				return gcResponse(req, 200, `{"name":"`+rg+`","id":"/subscriptions/test/resourceGroups/`+rg+`","tags":{`+tag+`}}`)
			}
			require.Equal(t, http.MethodPatch, req.Method, "existing parent RG must not be PUT")
			writes[rg]++
			var patch armresources.TagsPatchResource
			require.NoError(t, json.NewDecoder(req.Body).Decode(&patch))
			tags[rg] = *patch.Properties.Tags[deletionDueTimeTag]
			return gcResponse(req, 200, `{"properties":{"tags":{}}}`)
		})
		ctx := logging.WithLogger(t.Context(), t)
		ensure := cachedFunc(func(ctx context.Context, location string) (armresources.ResourceGroup, error) {
			return ensureResourceGroup(ctx, azure, location)
		})
		prepareCluster := cachedFunc(func(ctx context.Context, name string) (bool, error) {
			renewNodeResourceGroupDeadline(ctx, azure, cfg, &armcontainerservice.ManagedCluster{
				Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr(name)},
			})
			return true, nil
		})
		due := time.Now().Add(cfg.SuiteTimeout + CleanupTimeout).UTC().Format(time.RFC3339Nano)
		for range 2 {
			_, err := ensure(ctx, "westus3")
			require.NoError(t, err)
			_, err = prepareCluster(ctx, "actual-node-rg")
			require.NoError(t, err)
			require.Equal(t, map[string]int{"abe2e-westus3": 1, "actual-node-rg": 1}, reads)
			require.Equal(t, map[string]int{"actual-node-rg": 1}, writes)
			require.Equal(t, map[string]string{"abe2e-westus3": expired, "actual-node-rg": due, "unused-node-rg": expired}, tags)
			time.Sleep(time.Hour)
		}
	})
}

func TestEnsureResourceGroup(t *testing.T) {
	for _, tt := range []struct {
		name        string
		get, put    int
		tags        map[string]*string
		wantMethods []string
		failure     string
	}{
		{name: "create", get: 404, put: 200, wantMethods: []string{"GET", "PUT"}},
		{name: "existing without tags", get: 200, wantMethods: []string{"GET"}},
		{name: "existing with tags", get: 200, tags: map[string]*string{
			deletionDueTimeTag: to.Ptr("2000-01-01T00:00:00Z"), "owner": to.Ptr("keep"),
		}, wantMethods: []string{"GET"}},
		{name: "existing with invalid deadline", get: 200, tags: map[string]*string{
			deletionDueTimeTag: to.Ptr("bad"),
		}, wantMethods: []string{"GET"}},
		{name: "read failure", get: 403, wantMethods: []string{"GET"}, failure: "getting RG"},
		{name: "create failure", get: 404, put: 403, wantMethods: []string{"GET", "PUT"}, failure: "creating RG"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var methods []string
			azure := gcTestAzure(t, func(req *http.Request) *http.Response {
				methods = append(methods, req.Method)
				status := map[string]int{"GET": tt.get, "PUT": tt.put}[req.Method]
				require.NotZero(t, status, "unexpected request: %s", req.Method)
				if req.Method == http.MethodPut {
					var rg armresources.ResourceGroup
					require.NoError(t, json.NewDecoder(req.Body).Decode(&rg))
					require.Empty(t, rg.Tags)
				}
				body, err := json.Marshal(armresources.ResourceGroup{
					Name: to.Ptr("abe2e-westus3"), ID: to.Ptr("/subscriptions/test/resourceGroups/abe2e-westus3"), Tags: tt.tags,
				})
				require.NoError(t, err)
				return gcResponse(req, status, string(body))
			})
			logger := &executionLogger{}
			rg, err := ensureResourceGroup(logging.WithLogger(t.Context(), logger), azure, "westus3")
			if tt.failure != "" {
				require.ErrorContains(t, err, tt.failure)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.tags, rg.Tags)
			}
			require.Equal(t, tt.wantMethods, methods)
			require.NotContains(t, strings.Join(logger.logs, "\n"), "warning:")
		})
	}
}
