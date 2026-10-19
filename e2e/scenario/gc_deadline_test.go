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
	"github.com/Azure/agentbaker/e2e/toolkit"
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
					_, err := ensureResourceGroup(t.Context(), azure, cfg, "westus3")
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
		ensure := toolkit.Memoize(func(ctx context.Context, location string) (armresources.ResourceGroup, error) {
			return ensureResourceGroup(ctx, azure, cfg, location)
		})
		prepareCluster := toolkit.Memoize(func(ctx context.Context, name string) (bool, error) {
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
			rg, err := ensureResourceGroup(logging.WithLogger(t.Context(), logger), azure, &config.Configuration{}, "westus3")
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

func TestSaturdayDeadline(t *testing.T) {
	for _, tt := range []struct {
		name, earliest, want string
	}{
		{"Sunday", "2026-10-18T12:00:00Z", "2026-10-24T08:00:00Z"},
		{"Monday", "2026-10-19T12:00:00Z", "2026-10-24T08:00:00Z"},
		{"Tuesday", "2026-10-20T12:00:00Z", "2026-10-24T08:00:00Z"},
		{"Wednesday", "2026-10-21T12:00:00Z", "2026-10-24T08:00:00Z"},
		{"Thursday", "2026-10-22T12:00:00Z", "2026-10-24T08:00:00Z"},
		{"Friday", "2026-10-23T23:59:59.999999999Z", "2026-10-24T08:00:00Z"},
		{"Saturday midnight", "2026-10-24T00:00:00Z", "2026-10-24T08:00:00Z"},
		{"before cutoff", "2026-10-24T07:59:59.999999999Z", "2026-10-24T08:00:00Z"},
		{"at cutoff", "2026-10-24T08:00:00Z", "2026-10-24T08:00:00Z"},
		{"after cutoff", "2026-10-24T08:00:00.000000001Z", "2026-10-31T08:00:00Z"},
		{"Saturday end", "2026-10-24T23:59:59.999999999Z", "2026-10-31T08:00:00Z"},
		{"local Saturday UTC Friday", "2026-10-24T01:00:00+13:00", "2026-10-24T08:00:00Z"},
		{"local Sunday UTC Saturday", "2026-10-25T01:00:00+13:00", "2026-10-31T08:00:00Z"},
		{"year boundary", "2026-12-31T12:00:00Z", "2027-01-02T08:00:00Z"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			earliest, err := time.Parse(time.RFC3339Nano, tt.earliest)
			require.NoError(t, err)
			due := saturdayDeadline(earliest)
			require.Equal(t, tt.want, due.Format(time.RFC3339Nano))
			require.Equal(t, time.UTC, due.Location())
			require.False(t, due.Before(earliest))
		})
	}
}

func TestADOResourceGroupDeadline(t *testing.T) {
	for _, shared := range []bool{false, true} {
		kind := "node"
		if shared {
			kind = "shared"
		}
		for _, tt := range []struct {
			name, existing, want string
			create               bool
			elapsed              time.Duration
		}{
			{name: "missing", want: "2000-01-08T08:00:00Z"},
			{name: "expired", existing: "1999-12-31T00:00:00Z", want: "2000-01-08T08:00:00Z"},
			{name: "later weekday", existing: "2000-01-09T12:00:00Z", want: "2000-01-15T08:00:00Z"},
			{name: "equal", existing: "2000-01-08T08:00:00Z", want: "2000-01-08T08:00:00Z"},
			{name: "Saturday before cutoff", existing: "2000-01-08T00:00:00Z", want: "2000-01-08T08:00:00Z"},
			{name: "Saturday after cutoff", existing: "2000-01-08T12:00:00Z", want: "2000-01-15T08:00:00Z"},
			{name: "later Saturday", existing: "2000-01-15T08:00:00Z", want: "2000-01-15T08:00:00Z"},
			{name: "new shared RG", create: true, want: "2000-01-08T08:00:00Z"},
			{name: "suite crosses into Saturday", elapsed: 6*24*time.Hour + 23*time.Hour, want: "2000-01-08T08:00:00Z"},
			{name: "suite ends at cutoff", elapsed: 7*24*time.Hour + 6*time.Hour, want: "2000-01-08T08:00:00Z"},
			{name: "suite crosses cutoff", elapsed: 7*24*time.Hour + 6*time.Hour + time.Nanosecond, want: "2000-01-15T08:00:00Z"},
			{name: "suite crosses into Sunday", elapsed: 7*24*time.Hour + 23*time.Hour, want: "2000-01-15T08:00:00Z"},
		} {
			if tt.create && !shared {
				continue
			}
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					// The synthetic clock starts on Saturday, 2000-01-01.
					elapsed := tt.elapsed
					if elapsed == 0 {
						elapsed = 48 * time.Hour
					}
					time.Sleep(elapsed)
					cfg := &config.Configuration{InADO: true, SuiteTimeout: 2*time.Hour - CleanupTimeout}
					writes := 0
					azure := gcTestAzure(t, func(req *http.Request) *http.Response {
						if req.Method == http.MethodGet && tt.create {
							return gcResponse(req, 404, `{}`)
						}
						if req.Method == http.MethodPatch {
							writes++
							var patch armresources.TagsPatchResource
							require.NoError(t, json.NewDecoder(req.Body).Decode(&patch))
							require.Equal(t, armresources.TagsPatchOperationMerge, *patch.Operation)
							require.Equal(t, map[string]*string{deletionDueTimeTag: to.Ptr(tt.want)}, patch.Properties.Tags)
							return gcResponse(req, 200, `{"properties":{"tags":{}}}`)
						}
						require.Contains(t, []string{http.MethodGet, http.MethodPut}, req.Method)
						if req.Method == http.MethodPut {
							require.True(t, tt.create)
						}
						tags := map[string]*string{"owner": to.Ptr("keep")}
						if tt.existing != "" {
							tags[deletionDueTimeTag] = to.Ptr(tt.existing)
						}
						body, err := json.Marshal(armresources.ResourceGroup{
							Name: to.Ptr("rg"), ID: to.Ptr("/subscriptions/test/resourceGroups/rg"), Tags: tags,
						})
						require.NoError(t, err)
						return gcResponse(req, 200, string(body))
					})
					ctx := logging.WithLogger(t.Context(), t)
					if shared {
						_, err := ensureResourceGroup(ctx, azure, cfg, "westus3")
						require.NoError(t, err)
					} else {
						require.True(t, renewNodeResourceGroupDeadline(ctx, azure, cfg, &armcontainerservice.ManagedCluster{
							Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr("rg")},
						}))
					}
					if tt.existing == tt.want {
						require.Zero(t, writes)
					} else {
						require.Equal(t, 1, writes)
					}
				})
			})
		}
	}
}
