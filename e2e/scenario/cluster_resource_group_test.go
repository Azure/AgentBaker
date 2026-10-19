package scenario

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerregistry/armcontainerregistry/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/require"
)

func TestClusterResourceGroupLifecycle(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "create"
		if existing {
			name = "reuse"
		}
		t.Run(name, func(t *testing.T) {
			previousAzure, previousConfig := config.Azure, config.Config
			t.Cleanup(func() { config.Azure, config.Config = previousAzure, previousConfig })
			config.Config = &config.Configuration{SuiteTimeout: time.Hour}
			const parentRG = "westus3-abe2e-test-12345"
			const nodeRG = parentRG + "-nodes"
			const parentPath = "/subscriptions/test/resourcegroups/" + parentRG
			const nodePath = "/subscriptions/test/resourcegroups/" + nodeRG
			const clusterPath = parentPath + "/providers/microsoft.containerservice/managedclusters/abe2e-test-12345"
			const acrCredentialsPath = "/subscriptions/test/resourcegroups/abe2e-westus3/providers/microsoft.containerregistry/registries/abe2eprivatenonanonwestus3/listcredentials"
			cluster := &armcontainerservice.ManagedCluster{
				ID: to.Ptr(clusterPath), Name: to.Ptr("abe2e-test-12345"), Location: to.Ptr("westus3"),
				Properties: &armcontainerservice.ManagedClusterProperties{
					ProvisioningState: to.Ptr("Succeeded"), NodeResourceGroup: to.Ptr(nodeRG),
				},
			}
			clusterBody, err := json.Marshal(cluster)
			require.NoError(t, err)
			kubeconfigBody, err := json.Marshal(map[string]any{
				"kubeconfigs": []map[string]any{{"name": "admin", "value": []byte(`
apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://127.0.0.1
contexts:
- name: test
  context:
    cluster: test
current-context: test
`)}},
			})
			require.NoError(t, err)
			parentExists, clusterExists := existing, existing
			methods := map[string][]string{}
			renewed := map[string]bool{}
			respond := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				path := strings.ToLower(req.URL.Path)
				methods[path] = append(methods[path], req.Method)
				switch path {
				case parentPath, nodePath:
					if path == parentPath && !parentExists && req.Method == http.MethodGet {
						return gcResponse(req, http.StatusNotFound, `{}`)
					}
					if req.Method == http.MethodPut {
						require.Equal(t, parentPath, path)
						parentExists = true
					} else {
						require.Equal(t, http.MethodGet, req.Method)
					}
					return gcResponse(req, http.StatusOK, `{"id":"`+path+`","name":"`+strings.Split(path, "/")[4]+`","properties":{"provisioningState":"Succeeded"}}`)
				case parentPath + "/providers/microsoft.resources/tags/default",
					nodePath + "/providers/microsoft.resources/tags/default":
					require.Equal(t, http.MethodPatch, req.Method)
					var patch struct {
						Properties struct {
							Tags map[string]string
						}
					}
					require.NoError(t, json.NewDecoder(req.Body).Decode(&patch))
					require.NotEmpty(t, patch.Properties.Tags[deletionDueTimeTag])
					renewed[strings.Split(path, "/")[4]] = true
					return gcResponse(req, http.StatusOK, `{"properties":{"tags":{}}}`)
				case clusterPath:
					require.True(t, parentExists, "parent RG must exist before accessing AKS")
					switch req.Method {
					case http.MethodGet:
						if !clusterExists {
							return gcResponse(req, http.StatusNotFound, `{}`)
						}
					case http.MethodPut:
						var model armcontainerservice.ManagedCluster
						require.NoError(t, json.NewDecoder(req.Body).Decode(&model))
						require.Equal(t, nodeRG, *model.Properties.NodeResourceGroup)
						clusterExists = true
					case http.MethodDelete:
						clusterExists = false
						return gcResponse(req, http.StatusNoContent, "")
					default:
						t.Fatalf("unexpected AKS request: %s %s", req.Method, req.URL)
					}
					return gcResponse(req, http.StatusOK, string(clusterBody))
				case nodePath + "/providers/microsoft.compute/virtualmachinescalesets":
					return gcResponse(req, http.StatusOK, `{"value":[{"name":"aks-nodepool1"}]}`)
				case clusterPath + "/maintenanceconfigurations/default":
					if req.Method == http.MethodGet {
						return gcResponse(req, http.StatusNotFound, `{}`)
					}
					require.Equal(t, http.MethodPut, req.Method)
					return gcResponse(req, http.StatusOK, `{}`)
				case clusterPath + "/listclusteradmincredential":
					return gcResponse(req, http.StatusOK, string(kubeconfigBody))
				case acrCredentialsPath:
					return gcResponse(req, http.StatusForbidden, `{}`)
				default:
					t.Fatalf("request escaped cluster scope: %s %s", req.Method, req.URL)
					return nil
				}
			})
			config.Azure = gcTestAzure(t, respond)
			opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}}}
			config.Azure.AKS, err = armcontainerservice.NewManagedClustersClient("test", nil, opts)
			require.NoError(t, err)
			config.Azure.Maintenance, err = armcontainerservice.NewMaintenanceConfigurationsClient("test", nil, opts)
			require.NoError(t, err)
			config.Azure.VMSS, err = armcompute.NewVirtualMachineScaleSetsClient("test", nil, opts)
			require.NoError(t, err)
			config.Azure.RegistriesClient, err = armcontainerregistry.NewRegistriesClient("test", nil, opts)
			require.NoError(t, err)
			ctx := logging.WithLogger(t.Context(), t)
			ready, err := getOrCreateCluster(ctx, cluster)
			require.NoError(t, err)
			require.Equal(t, cluster, ready)
			require.Equal(t, map[string]bool{parentRG: true, nodeRG: true}, renewed)
			ready, err = waitUntilClusterReady(ctx, *cluster.Name, *cluster.Location)
			require.NoError(t, err)
			require.Equal(t, cluster, ready)
			require.NoError(t, ensureMaintenanceConfiguration(ctx, ready))
			_, err = getClusterKubeClient(ctx, ready)
			require.NoError(t, err)
			home := t.TempDir()
			t.Setenv("HOME", home)
			require.NoError(t, os.Mkdir(filepath.Join(home, ".kube"), 0700))
			err = createPrivateAzureContainerRegistryPullSecret(ctx, ready, nil, "abe2e-westus3", true)
			require.Error(t, err)
			require.Equal(t, []string{http.MethodPost}, methods[acrCredentialsPath], "ACR credentials must still use the shared RG")
			require.FileExists(t, filepath.Join(home, ".kube", "config"))
			require.NoError(t, deleteCluster(ctx, *cluster.Name, parentRG))
			require.NoError(t, waitForClusterDeletion(ctx, *cluster.Name, parentRG))
			if existing {
				require.Equal(t, []string{http.MethodGet}, methods[parentPath])
				require.NotContains(t, methods[clusterPath], http.MethodPut)
			} else {
				require.Equal(t, []string{http.MethodGet, http.MethodPut}, methods[parentPath])
				require.Contains(t, methods[clusterPath], http.MethodPut)
			}
		})
	}
}

func TestACRRecreationDeletesClusterInDedicatedResourceGroup(t *testing.T) {
	previousAzure, previousConfig := config.Azure, config.Config
	t.Cleanup(func() { config.Azure, config.Config = previousAzure, previousConfig })
	config.Config = &config.Configuration{AzureContainerRegistrytargetRepository: "cache/*"}
	const sharedRG = "abe2e-westus3"
	const registryPath = "/subscriptions/test/resourcegroups/" + sharedRG + "/providers/microsoft.containerregistry/registries/abe2eprivatewestus3"
	const cachePath = registryPath + "/cacherules/aks-managed-rule"
	const clusterPath = "/subscriptions/test/resourcegroups/westus3-abe2e-test-12345/providers/microsoft.containerservice/managedclusters/abe2e-test-12345"
	cluster := &armcontainerservice.ManagedCluster{
		Name: to.Ptr("abe2e-test-12345"), Location: to.Ptr("westus3"),
	}
	var requests []string
	respond := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
		path := strings.ToLower(req.URL.Path)
		request := req.Method + " " + path
		requests = append(requests, request)
		switch request {
		case http.MethodGet + " " + registryPath, http.MethodPut + " " + registryPath:
			return gcResponse(req, http.StatusOK, `{"id":"`+registryPath+`","location":"westus3","sku":{"name":"Premium"},"properties":{"provisioningState":"Succeeded"}}`)
		case http.MethodGet + " " + cachePath:
			return gcResponse(req, http.StatusNotFound, `{}`)
		case http.MethodDelete + " " + registryPath, http.MethodDelete + " " + clusterPath:
			return gcResponse(req, http.StatusNoContent, "")
		case http.MethodGet + " " + clusterPath:
			return gcResponse(req, http.StatusOK, `{"id":"`+clusterPath+`","name":"abe2e-test-12345","location":"westus3","properties":{"provisioningState":"Succeeded"}}`)
		case http.MethodPut + " " + cachePath:
			return gcResponse(req, http.StatusOK, `{"id":"`+cachePath+`","properties":{"provisioningState":"Succeeded","targetRepository":"cache/*"}}`)
		default:
			t.Fatalf("unexpected request: %s", request)
			return nil
		}
	})
	opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}}}
	config.Azure = &config.AzureClient{}
	var err error
	config.Azure.AKS, err = armcontainerservice.NewManagedClustersClient("test", nil, opts)
	require.NoError(t, err)
	config.Azure.RegistriesClient, err = armcontainerregistry.NewRegistriesClient("test", nil, opts)
	require.NoError(t, err)
	config.Azure.CacheRulesClient, err = armcontainerregistry.NewCacheRulesClient("test", nil, opts)
	require.NoError(t, err)

	err = createPrivateAzureContainerRegistry(logging.WithLogger(t.Context(), t), cluster, sharedRG, false)
	require.NoError(t, err)
	require.Equal(t, []string{
		http.MethodGet + " " + registryPath,
		http.MethodGet + " " + cachePath,
		http.MethodDelete + " " + registryPath,
		http.MethodGet + " " + clusterPath,
		http.MethodDelete + " " + clusterPath,
		http.MethodPut + " " + registryPath,
		http.MethodPut + " " + cachePath,
	}, requests)
}

func TestPrepareClusterIsolatesLegacyResources(t *testing.T) {
	previousAzure, previousConfig := config.Azure, config.Config
	previousInfra, previousSubnet := CachedEnsureSharedInfra, CachedEnsureClusterSubnet
	t.Cleanup(func() {
		config.Azure, config.Config = previousAzure, previousConfig
		CachedEnsureSharedInfra, CachedEnsureClusterSubnet = previousInfra, previousSubnet
	})
	config.Config = &config.Configuration{TestTimeoutCluster: time.Minute}
	CachedEnsureSharedInfra = func(context.Context, string) (*SharedInfra, error) {
		return &SharedInfra{IdentityID: "/subscriptions/test/resourceGroups/abe2e-westus3/providers/Microsoft.ManagedIdentity/userAssignedIdentities/shared"}, nil
	}
	var subnetClusterName string
	CachedEnsureClusterSubnet = func(_ context.Context, request ClusterSubnetRequest) (string, error) {
		subnetClusterName = request.ClusterName
		return "/subscriptions/test/resourceGroups/abe2e-westus3/providers/Microsoft.Network/virtualNetworks/shared/subnets/" + clusterSubnetName(request.ClusterName), nil
	}
	model := getAzureNetworkClusterModel("abe2e-latest-k8s-azure-bootstrapprofile-cache-v1", "australiasoutheast", "Standard_D2ds_v5")
	_, err := configureSharedVNet(t.Context(), model, *model.Location)
	require.NoError(t, err)
	legacyJSON, err := json.Marshal(model)
	require.NoError(t, err)
	legacyHash := sha256.Sum256(legacyJSON)
	legacyName := *model.Name + "-" + hex.EncodeToString(legacyHash[:])[:5]
	config.Azure = gcTestAzure(t, func(req *http.Request) *http.Response {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "/subscriptions/test/resourcegroups/australiasoutheast-"+*model.Name, strings.ToLower(req.URL.Path))
		return gcResponse(req, http.StatusForbidden, `{}`)
	})
	_, err = prepareCluster(logging.WithLogger(t.Context(), t), model, false, false)
	require.ErrorContains(t, err, "ensure cluster resource group")
	require.NotEqual(t, clusterSubnetName(legacyName), clusterSubnetName(subnetClusterName))
	require.Equal(t, *model.Name, subnetClusterName)
	require.Equal(t, "australiasoutheast-"+*model.Name+"-nodes", *model.Properties.NodeResourceGroup)
	require.LessOrEqual(t, len(*model.Properties.NodeResourceGroup), 90)
}
