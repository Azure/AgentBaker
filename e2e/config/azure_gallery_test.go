package config

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

type galleryTestPolicy func(*http.Request) string

func (f galleryTestPolicy) Do(req *policy.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(f(req.Raw()))),
		Request:    req.Raw(),
	}, nil
}

func galleryTestClient(respond galleryTestPolicy) *AzureClient {
	return &AzureClient{ArmOptions: &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}},
	}}
}

func galleryTestVersion(state, replicationState string) string {
	return `{
		"id": "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery/images/image/versions/1.0.0",
		"name": "1.0.0",
		"location": "westus",
		"properties": {
			"provisioningState": "` + state + `",
			"publishingProfile": {"targetRegions": [{"name": "eastus"}]},
			"replicationStatus": {"summary": [{"region": "eastus", "state": "` + replicationState + `"}]}
		}
	}`
}

func TestEnsureReplicationChecksRegionalReadiness(t *testing.T) {
	for _, tt := range []struct {
		name            string
		provisioning    string
		replication     string
		cancelAfterRead bool
		errorContains   string
	}{
		{name: "completed", provisioning: "Succeeded", replication: "Completed"},
		{name: "advertised but still replicating", provisioning: "Succeeded", replication: "InProgress", cancelAfterRead: true, errorContains: "context canceled"},
		{name: "unknown regional state", provisioning: "Succeeded", replication: "Unknown", cancelAfterRead: true, errorContains: "context canceled"},
		{name: "regional replication failed", provisioning: "Succeeded", replication: "Failed", errorContains: "replication failed in eastus"},
		{name: "aggregate and regional failure", provisioning: "Failed", replication: "Failed", errorContains: "replication failed in eastus"},
		{name: "version failed", provisioning: "Failed", replication: "Completed", errorContains: "operation failed with state: Failed"},
		{name: "another region updating", provisioning: "Updating", replication: "Completed", cancelAfterRead: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(logging.WithLogger(t.Context(), discardLogger{}))
			defer cancel()
			client := galleryTestClient(func(req *http.Request) string {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "ReplicationStatus", req.URL.Query().Get("$expand"))
				if tt.cancelAfterRead {
					cancel()
				}
				return galleryTestVersion(tt.provisioning, tt.replication)
			})
			image := &Image{Name: "image", Gallery: &Gallery{SubscriptionID: "test", ResourceGroupName: "rg", Name: "gallery"}}
			var snapshot armcompute.GalleryImageVersion
			require.NoError(t, json.Unmarshal([]byte(galleryTestVersion(tt.provisioning, "Unknown")), &snapshot))
			err := client.ensureReplication(ctx, image, &snapshot, "eastus")
			if tt.errorContains != "" {
				require.ErrorContains(t, err, tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEnsureReplicationPreservesLiveRegions(t *testing.T) {
	ctx := logging.WithLogger(t.Context(), discardLogger{})
	image := &Image{Name: "image", Gallery: &Gallery{SubscriptionID: "test", ResourceGroupName: "rg", Name: "gallery"}}
	var snapshot armcompute.GalleryImageVersion
	require.NoError(t, json.Unmarshal([]byte(strings.ReplaceAll(galleryTestVersion("Succeeded", "Completed"), "eastus", "westus")), &snapshot))
	current := strings.Replace(galleryTestVersion("Succeeded", "Completed"),
		`[{"name": "eastus"}]`, `[{"name": "westus"}, {"name": "northeurope"}]`, 1)
	updated := false
	readAfterUpdate := false
	client := galleryTestClient(func(req *http.Request) string {
		if req.Method == http.MethodPut {
			var update armcompute.GalleryImageVersion
			require.NoError(t, json.NewDecoder(req.Body).Decode(&update))
			var regions []string
			for _, region := range update.Properties.PublishingProfile.TargetRegions {
				regions = append(regions, *region.Name)
			}
			require.ElementsMatch(t, []string{"westus", "northeurope", "eastus"}, regions)
			require.NotNil(t, update.Properties.SafetyProfile)
			require.NotNil(t, update.Properties.SafetyProfile.AllowDeletionOfReplicatedLocations)
			require.False(t, *update.Properties.SafetyProfile.AllowDeletionOfReplicatedLocations)
			require.False(t, updated)
			updated = true
			current = strings.Replace(current, `{"name": "northeurope"}`, `{"name": "northeurope"}, {"name": "eastus"}`, 1)
		} else {
			require.Equal(t, http.MethodGet, req.Method)
			readAfterUpdate = updated
		}
		return current
	})
	require.NoError(t, client.ensureReplication(ctx, image, &snapshot, "eastus"))
	require.True(t, updated)
	require.True(t, readAfterUpdate)
}

func TestEnsureSIGImageVersionReportsRegionalFailure(t *testing.T) {
	ctx := logging.WithLogger(t.Context(), discardLogger{})
	client := galleryTestClient(func(req *http.Request) string {
		require.Equal(t, http.MethodGet, req.Method)
		return strings.Replace(galleryTestVersion("Failed", "Failed"),
			`"state": "Failed"`, `"state": "Failed", "details": "regional storage quota exceeded"`, 1)
	})
	image := &Image{Name: "image", Version: "1.0.0", Gallery: &Gallery{SubscriptionID: "test", ResourceGroupName: "rg", Name: "gallery"}}
	_, err := client.EnsureSIGImageVersion(ctx, image, "eastus")
	require.ErrorContains(t, err, "replication failed in eastus: regional storage quota exceeded")
}

func TestEnsureReplicationUsesCallerDeadlineAndPollInterval(t *testing.T) {
	for _, provisioning := range []string{"Succeeded", "Updating"} {
		for _, outcome := range []string{"deadline", "completed"} {
			t.Run(provisioning+"/"+outcome, func(t *testing.T) {
				previousInterval := Config.DefaultPollInterval
				Config.DefaultPollInterval = 37 * time.Second
				defer func() { Config.DefaultPollInterval = previousInterval }()
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(logging.WithLogger(t.Context(), discardLogger{}), 12*time.Minute)
					defer cancel()
					start := time.Now()
					var reads []time.Duration
					client := galleryTestClient(func(req *http.Request) string {
						require.Equal(t, http.MethodGet, req.Method)
						reads = append(reads, time.Since(start))
						regionalState := "InProgress"
						if outcome == "completed" && time.Since(start) >= 11*time.Minute {
							regionalState = "Completed"
						}
						return galleryTestVersion(provisioning, regionalState)
					})
					image := &Image{Name: "image", Gallery: &Gallery{SubscriptionID: "test", ResourceGroupName: "rg", Name: "gallery"}}
					var snapshot armcompute.GalleryImageVersion
					require.NoError(t, json.Unmarshal([]byte(galleryTestVersion(provisioning, "InProgress")), &snapshot))
					err := client.ensureReplication(ctx, image, &snapshot, "eastus")
					if outcome == "completed" {
						require.NoError(t, err)
						require.GreaterOrEqual(t, time.Since(start), 11*time.Minute)
					} else {
						require.ErrorIs(t, err, context.DeadlineExceeded)
						require.Equal(t, 12*time.Minute, time.Since(start))
					}
					require.Greater(t, len(reads), 2)
					for i := 1; i < len(reads); i++ {
						if reads[i] != reads[i-1] {
							require.Equal(t, Config.DefaultPollInterval, reads[i]-reads[i-1])
						}
					}
				})
			})
		}
	}
}
