package config

import (
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skuTestClient(t *testing.T, respond galleryTestPolicy) *AzureClient {
	t.Helper()
	client := galleryTestClient(respond)
	var err error
	client.ResourceSKUs, err = armcompute.NewResourceSKUsClient("test", nil, client.ArmOptions)
	require.NoError(t, err)
	return client
}

const skuTestFirstPage = `{
	"value": [
		{"name":"size-a","resourceType":"disks"},
		{"name":"size-a","resourceType":"virtualMachines","capabilities":[
			{"name":"HyperVGenerations","value":"V2"},
			{"name":"DiskControllerTypes","value":"SCSI, NVMe"}
		]}
	],
	"nextLink":"https://management.azure.com/subscriptions/test/providers/Microsoft.Compute/skus?page=2"
}`

const skuTestLastPage = `{"value":[
	{"name":"size-b","resourceType":"virtualMachines","capabilities":[
		{"name":"HyperVGenerations","value":"V1,V2"},
		{"name":"DiskControllerTypes","value":"SCSI"}
	]}
]}`

func TestListVMResourceSKUsLoadsAllPages(t *testing.T) {
	calls := 0
	client := skuTestClient(t, func(req *http.Request) (int, string) {
		calls++
		assert.Equal(t, http.MethodGet, req.Method)
		if req.URL.Query().Get("page") == "2" {
			return http.StatusOK, skuTestLastPage
		}
		assert.Equal(t, "location eq 'westus3'", req.URL.Query().Get("$filter"))
		return http.StatusOK, skuTestFirstPage
	})
	catalogue, err := client.ListVMResourceSKUs(t.Context(), "westus3")
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	require.Len(t, catalogue, 2)
	require.Contains(t, catalogue, "SIZE-A")
	require.Contains(t, catalogue, "SIZE-B")
	assert.True(t, SkuSupportsOnlyGen2(catalogue["SIZE-A"]))
	assert.True(t, SkuSupportsNVMe(catalogue["SIZE-A"]))
	assert.False(t, SkuSupportsOnlyGen2(catalogue["SIZE-B"]))
	assert.False(t, SkuSupportsNVMe(catalogue["SIZE-B"]))
}

func TestListVMResourceSKUsDoesNotPublishIncompletePages(t *testing.T) {
	client := skuTestClient(t, func(req *http.Request) (int, string) {
		if req.URL.Query().Get("page") == "2" {
			return http.StatusForbidden, `{"error":{"code":"AuthorizationFailed","message":"denied"}}`
		}
		return http.StatusOK, skuTestFirstPage
	})
	catalogue, err := client.ListVMResourceSKUs(t.Context(), "westus3")
	var responseErr *azcore.ResponseError
	require.ErrorAs(t, err, &responseErr)
	assert.Equal(t, "AuthorizationFailed", responseErr.ErrorCode)
	assert.Nil(t, catalogue, "a failed later page must not expose the first-page match")
}
