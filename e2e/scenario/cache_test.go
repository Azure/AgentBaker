package scenario

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGalleryImageRecoversFailedDefinition(t *testing.T) {
	for _, tc := range []struct {
		name         string
		state        string
		deleteStatus int
		pollFailure  bool
		wantCalls    []string
		wantErr      string
	}{
		{
			name: "reuse succeeded", state: "Succeeded",
			wantCalls: []string{"GET"},
		},
		{
			name: "preserve in progress", state: "Creating",
			wantCalls: []string{"GET"},
		},
		{
			name: "delete failed before recreating", state: "Failed", deleteStatus: http.StatusAccepted,
			wantCalls: []string{"GET", "DELETE", "POLL", "PUT"},
		},
		{
			name: "already deleted by another runner", state: "Failed", deleteStatus: http.StatusNotFound,
			wantCalls: []string{"GET", "DELETE", "PUT"},
		},
		{
			name: "delete rejected", state: "Failed", deleteStatus: http.StatusConflict,
			wantCalls: []string{"GET", "DELETE"},
			wantErr:   "failed to delete failed gallery image",
		},
		{
			name: "delete polling failed", state: "Failed", deleteStatus: http.StatusAccepted, pollFailure: true,
			wantCalls: []string{"GET", "DELETE", "POLL"},
			wantErr:   "failed to poll gallery image deletion",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousAzure := config.Azure
			t.Cleanup(func() { config.Azure = previousAzure })
			imageName := config.Config.TestGalleryImagePrefix + "-westus3-amd64-genV2-linux"
			imagePath := "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery/images/" + imageName
			var calls []string
			deleteComplete := false
			client, err := armcompute.NewGalleryImagesClient("test", &fake.TokenCredential{}, &arm.ClientOptions{
				ClientOptions: policy.ClientOptions{
					Retry: policy.RetryOptions{MaxRetries: -1},
					Transport: galleryCleanupTransport(func(req *http.Request) (*http.Response, error) {
						method := req.Method
						if req.URL.Path == "/operations/delete-image" {
							require.Equal(t, http.MethodGet, method)
							method = "POLL"
						} else {
							require.Equal(t, imagePath, req.URL.Path)
						}
						calls = append(calls, method)
						status := http.StatusOK
						header := http.Header{"Content-Type": {"application/json"}}
						body := fmt.Sprintf(`{"name":%q,"properties":{"provisioningState":%q}}`, imageName, tc.state)
						switch method {
						case http.MethodGet:
						case http.MethodDelete:
							status = tc.deleteStatus
							body = `{"error":{"code":"OperationNotAllowed","message":"image has child versions"}}`
							if status == http.StatusAccepted {
								header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/delete-image")
								body = ""
							} else if status == http.StatusNotFound {
								body = `{"error":{"code":"ResourceNotFound","message":"already deleted"}}`
								deleteComplete = true
							}
						case "POLL":
							body = `{"status":"Succeeded"}`
							deleteComplete = true
							if tc.pollFailure {
								body = `{"status":"Failed","error":{"code":"OperationNotAllowed","message":"deletion failed"}}`
								deleteComplete = false
							}
						case http.MethodPut:
							assert.True(t, deleteComplete, "recreation must wait for deletion")
							body = fmt.Sprintf(`{"name":%q,"properties":{"provisioningState":"Succeeded"}}`, imageName)
						default:
							t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
						}
						return &http.Response{
							StatusCode: status, Header: header,
							Body: io.NopCloser(strings.NewReader(body)), Request: req,
						}, nil
					}),
				},
			})
			require.NoError(t, err)
			config.Azure = &config.AzureClient{GalleryImages: client}
			image, err := createGalleryImage(t.Context(), CreateGalleryImageRequest{
				ResourceGroup: "rg", GalleryName: "gallery", Location: "westus3",
				Arch: "amd64", HyperVGeneration: to.Ptr(armcompute.HyperVGenerationV2),
			})
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				assert.Contains(t, err.Error(), "OperationNotAllowed")
			} else {
				require.NoError(t, err)
				require.NotNil(t, image.Name)
				assert.Equal(t, imageName, *image.Name)
				require.NotNil(t, image.Properties)
				wantState := tc.state
				if tc.state == "Failed" {
					wantState = "Succeeded"
				}
				assert.Equal(t, wantState, string(*image.Properties.ProvisioningState))
			}
			assert.Equal(t, tc.wantCalls, calls)
		})
	}
}

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
						{"name":"size-a","resourceType":"virtualMachines","capabilities":[{"name":"CpuArchitectureType","value":"x64"}]},
						{"name":"size-b","resourceType":"virtualMachines","capabilities":[{"name":"CpuArchitectureType","value":"Arm64"}]}
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
	arch, err := VMSizeArchitecture(t.Context(), location, "SIZE-A")
	require.NoError(t, err)
	assert.Equal(t, "amd64", arch)
	arch, err = VMSizeArchitecture(t.Context(), strings.ToLower(location), "size-b")
	require.NoError(t, err)
	assert.Equal(t, "arm64", arch)
	assert.Equal(t, 1, calls, "architecture selection must reuse the provisioning catalogue")
	_, err = VMSizeArchitecture(t.Context(), location, "missing-size")
	require.ErrorContains(t, err, `VM size "missing-size" not found in location`)
	_, err = getVMSizeSKU(t.Context(), location, "missing-size")
	require.ErrorContains(t, err, `VM size "missing-size" not found in location`)
	assert.Equal(t, 1, calls)
	_, err = getVMSizeSKU(t.Context(), location+"-other", "size-a")
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "different regions need separate catalogues")
}
