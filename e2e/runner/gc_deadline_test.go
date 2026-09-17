package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runnerGCPolicy func(*http.Request) *http.Response

func (f runnerGCPolicy) Do(req *policy.Request) (*http.Response, error) {
	return f(req.Raw()), nil
}

func TestExecutorPreservesSuiteGCDeadlineAcrossRetries(t *testing.T) {
	restoreRunnerConfig(t)
	config.Config.SuiteTimeout = 47 * time.Minute
	config.Config.TestTimeout = time.Minute
	previousAzure := config.Azure
	t.Cleanup(func() { config.Azure = previousAzure })

	ctx, cancel := context.WithTimeout(t.Context(), config.Config.SuiteTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	writes := 0
	dueTag := ""
	respond := runnerGCPolicy(func(req *http.Request) *http.Response {
		body := `{"id":"/subscriptions/test/resourceGroups/runner-gc","tags":{"owner":"keep"}}`
		if dueTag != "" {
			body = `{"id":"/subscriptions/test/resourceGroups/runner-gc","tags":{"owner":"keep","deletion_due_time":"` + dueTag + `"}}`
		}
		if req.Method == http.MethodPatch {
			var patch armresources.TagsPatchResource
			require.NoError(t, json.NewDecoder(req.Body).Decode(&patch))
			require.Equal(t, armresources.TagsPatchOperationMerge, *patch.Operation)
			require.Equal(t, deadline.Add(scenario.CleanupTimeout).UTC().Format(time.RFC3339Nano), *patch.Properties.Tags["deletion_due_time"])
			writes++
			dueTag = *patch.Properties.Tags["deletion_due_time"]
			body = `{"properties":{"tags":{}}}`
		} else {
			require.Equal(t, http.MethodGet, req.Method, "must not replace existing RG tags")
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(body)), Request: req,
		}
	})
	opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}}}
	client := &config.AzureClient{}
	var err error
	client.ResourceGroup, err = armresources.NewResourceGroupsClient("test", nil, opts)
	require.NoError(t, err)
	client.Tags, err = armresources.NewTagsClient("test", nil, opts)
	require.NoError(t, err)
	config.Azure = client

	exec := newExecutor(ctx, &bytes.Buffer{}, runOptions{
		parallel: 1, retries: 1, logDir: t.TempDir(), outputMode: "grouped",
	}, 1)
	attempts := 0
	exec.runScenario = func(ctx context.Context, _, _ string, _ *scenario.Scenario) scenario.Outcome {
		attempts++
		attemptDeadline, _ := ctx.Deadline()
		assert.True(t, attemptDeadline.Before(deadline))
		if _, err := scenario.CachedEnsureResourceGroup(ctx, "runner-gc-test"); err != nil {
			return scenario.Outcome{Error: err}
		}
		if attempts == 1 {
			return scenario.Outcome{Error: errors.New("retry")}
		}
		return scenario.Outcome{}
	}
	exec.execute("GC", &scenario.Scenario{})
	require.Equal(t, 2, attempts)
	require.Equal(t, 1, exec.summary().Flaky)
	require.Equal(t, 1, writes, "native retry must not write the same deadline again")
}
