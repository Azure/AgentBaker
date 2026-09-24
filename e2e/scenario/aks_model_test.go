package scenario

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerregistry/armcontainerregistry/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestPrivateACRPullSecretPreservesLocalKubeconfig(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "missing"
		if existing {
			name = "existing"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			path := filepath.Join(home, ".kube", "config")
			const original = "existing kubeconfig must not be overwritten"
			if existing {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
				require.NoError(t, os.WriteFile(path, []byte(original), 0600))
			}

			registries, err := armcontainerregistry.NewRegistriesClient("test", nil, &arm.ClientOptions{
				ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{
					vmssCreationTestPolicy(func(req *http.Request) *http.Response {
						require.True(t, strings.HasSuffix(req.URL.Path, "/listCredentials"))
						return gcResponse(req, http.StatusOK, `{"username":"test-user","passwords":[{"value":"test-password"}]}`)
					}),
				}},
			})
			require.NoError(t, err)
			oldAzure := config.Azure
			t.Cleanup(func() { config.Azure = oldAzure })
			config.Azure = &config.AzureClient{RegistriesClient: registries}

			var created atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodPost || req.URL.Path != "/api/v1/namespaces/default/secrets" {
					t.Errorf("unexpected Kubernetes request: %s %s", req.Method, req.URL.Path)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				created.Store(true)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = io.WriteString(w, `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"acr-secret"}}`)
			}))
			t.Cleanup(server.Close)
			kube := &Kubeclient{RESTConfig: &rest.Config{Host: server.URL}}
			cluster := &armcontainerservice.ManagedCluster{Name: to.Ptr("cluster"), Location: to.Ptr("westus3")}
			require.NoError(t, createPrivateAzureContainerRegistryPullSecret(t.Context(), cluster, kube, "rg", true))
			require.True(t, created.Load(), "secret must be created using the supplied Kubernetes client")
			if existing {
				content, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, original, string(content))
			} else {
				_, err := os.Stat(path)
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}
