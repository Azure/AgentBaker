package scenario

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMSizeSKUReusesRegionalCatalogue(t *testing.T) {
	// Each invocation needs a fresh region because the cache lives for the process.
	location := "SkuCacheTest" + uuid.NewString()
	calls := 0
	client, err := armcompute.NewResourceSKUsClient("test", nil, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
			vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				calls++
				assert.Equal(t, http.MethodGet, req.Method)
				assert.Equal(t, strings.ToLower(req.URL.Query().Get("$filter")), req.URL.Query().Get("$filter"))
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{"value":[
						{"name":"size-a","resourceType":"virtualMachines"},
						{"name":"size-b","resourceType":"virtualMachines"}
					]}`)),
					Request: req,
				}
			}),
		}},
	})
	require.NoError(t, err)
	previousAzure := config.Azure
	config.Azure = &config.AzureClient{ResourceSKUs: client}
	t.Cleanup(func() { config.Azure = previousAzure })

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = getVMSizeSKU(ctx, location, "size-a")
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, calls)

	for _, test := range []struct{ location, size string }{
		{location, "size-a"},
		{strings.ToLower(location), "SIZE-B"},
	} {
		sku, err := getVMSizeSKU(t.Context(), test.location, test.size)
		require.NoError(t, err)
		assert.Equal(t, strings.ToLower(test.size), *sku.Name)
	}
	assert.Equal(t, 1, calls)
	_, err = getVMSizeSKU(t.Context(), location, "missing-size")
	require.ErrorContains(t, err, `VM size "missing-size" not found in location`)
	assert.Equal(t, 1, calls)
	_, err = getVMSizeSKU(t.Context(), location+"-other", "size-a")
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "different regions need separate catalogues")
}
