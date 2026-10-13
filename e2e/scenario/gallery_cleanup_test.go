package scenario

import (
	"context"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type galleryCleanupTransport func(*http.Request) (*http.Response, error)

func (f galleryCleanupTransport) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGalleryImageCleanupPropagatesErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		wantErr string
	}{
		{name: "delete failure", status: http.StatusForbidden, wantErr: "AuthorizationFailed"},
		{name: "already absent", status: http.StatusNotFound},
		{name: "asynchronous deletion", status: http.StatusAccepted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldAzure, oldKeep := config.Azure, config.Config.KeepVMSS
			oldGallery, oldImage := CachedCreateGallery, CachedCreateGalleryImage
			t.Cleanup(func() {
				config.Azure, config.Config.KeepVMSS = oldAzure, oldKeep
				CachedCreateGallery, CachedCreateGalleryImage = oldGallery, oldImage
			})
			config.Config.KeepVMSS = true
			CachedCreateGallery = func(context.Context, CreateGalleryRequest) (armcompute.Gallery, error) {
				return armcompute.Gallery{Name: to.Ptr("gallery")}, nil
			}
			CachedCreateGalleryImage = func(context.Context, CreateGalleryImageRequest) (armcompute.GalleryImage, error) {
				return armcompute.GalleryImage{ID: to.Ptr("image-id"), Name: to.Ptr("image")}, nil
			}
			var deleted bool
			client, err := armcompute.NewGalleryImageVersionsClient("subscription", &fake.TokenCredential{}, &arm.ClientOptions{
				ClientOptions: policy.ClientOptions{
					Retry: policy.RetryOptions{MaxRetries: -1},
					Transport: galleryCleanupTransport(func(req *http.Request) (*http.Response, error) {
						header := http.Header{"Content-Type": {"application/json"}}
						status, body := http.StatusOK, `{"name":"1.0.0","properties":{"provisioningState":"Succeeded"}}`
						switch req.Method {
						case http.MethodPut:
							assert.True(t, strings.HasSuffix(req.URL.Path, "/galleries/gallery/images/image/versions/1.0.0"))
						case http.MethodDelete:
							deleted = true
							status = tc.status
							body = `{"error":{"code":"AuthorizationFailed","message":"delete denied"}}`
							if status == http.StatusAccepted {
								header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/delete-image")
								body = ""
							}
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
			config.Azure = &config.AzureClient{GalleryImageVersions: client}
			s := &Scenario{
				Config: Config{VHD: &config.Image{}},
				Runtime: &ScenarioRuntime{VM: &ScenarioVM{VM: &armcompute.VirtualMachineScaleSetVM{
					Properties: &armcompute.VirtualMachineScaleSetVMProperties{InstanceView: &armcompute.VirtualMachineScaleSetVMInstanceView{}},
				}}},
				cleanup: &scenarioCleanup{},
			}
			_, err = CreateSIGImageVersionFromDisk(t.Context(), s, "1.0.0", "disk-id")
			require.NoError(t, err)
			err = runScenarioCleanup(t.Context(), s.cleanup)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.True(t, deleted)
		})
	}
}
