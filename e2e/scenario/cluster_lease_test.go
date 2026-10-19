package scenario

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/stretchr/testify/require"
)

type testClusterLeaseBlob struct {
	leaseID  string
	expires  time.Time
	metadata map[string]string
}

type testClusterLeaseStore struct {
	mu         sync.Mutex
	blobs      map[string]*testClusterLeaseBlob
	failAction string
	renewals   int
}

type clusterLeaseTestTransport func(*http.Request) *http.Response

func (f clusterLeaseTestTransport) Do(req *http.Request) (*http.Response, error) {
	headers := http.Header{}
	for name, values := range req.Header {
		headers[http.CanonicalHeaderKey(name)] = values
	}
	req.Header = headers
	return f(req), nil
}

func clusterLeaseTestClient(t *testing.T, store *testClusterLeaseStore) *azblob.Client {
	t.Helper()
	client, err := azblob.NewClientWithNoCredential("https://test.blob.core.windows.net", &azblob.ClientOptions{
		ClientOptions: policy.ClientOptions{Retry: policy.RetryOptions{MaxRetries: -1},
			Transport: clusterLeaseTestTransport(func(req *http.Request) *http.Response {
				store.mu.Lock()
				defer store.mu.Unlock()
				failure := func(status int, code string) *http.Response {
					response := gcResponse(req, status, "")
					response.Header.Set("x-ms-error-code", code)
					return response
				}
				action := req.Header.Get("x-ms-lease-action")
				if store.failAction != "" && (action == store.failAction || req.URL.Query().Get("comp") == store.failAction) {
					return failure(http.StatusForbidden, "AuthorizationPermissionMismatch")
				}
				if store.blobs == nil {
					store.blobs = map[string]*testClusterLeaseBlob{}
				}
				stored := store.blobs[req.URL.Path]
				if stored != nil && !time.Now().Before(stored.expires) {
					stored.leaseID = ""
				}
				if req.Method == http.MethodPut && req.URL.RawQuery == "" {
					require.Equal(t, "*", req.Header.Get("If-None-Match"))
					if stored != nil {
						return failure(http.StatusPreconditionFailed, "ConditionNotMet")
					}
					store.blobs[req.URL.Path] = &testClusterLeaseBlob{metadata: map[string]string{}}
					return gcResponse(req, http.StatusCreated, "")
				}
				if stored == nil {
					return failure(http.StatusNotFound, "BlobNotFound")
				}
				if action == "acquire" {
					require.Equal(t, "60", req.Header.Get("x-ms-lease-duration"))
					if stored.leaseID != "" {
						return failure(http.StatusConflict, "LeaseAlreadyPresent")
					}
					stored.leaseID = req.Header.Get("x-ms-proposed-lease-id")
					require.NotEmpty(t, stored.leaseID)
					stored.expires = time.Now().Add(time.Minute)
					response := gcResponse(req, http.StatusCreated, "")
					response.Header.Set("x-ms-lease-id", stored.leaseID)
					return response
				}
				if stored.leaseID == "" || req.Header.Get("x-ms-lease-id") != stored.leaseID {
					return failure(http.StatusPreconditionFailed, "LeaseIdMismatchWithBlobOperation")
				}
				switch {
				case action == "renew":
					store.renewals++
					stored.expires = time.Now().Add(time.Minute)
				case action == "release":
					stored.leaseID = ""
				case req.Method == http.MethodHead:
					response := gcResponse(req, http.StatusOK, "")
					for name, value := range stored.metadata {
						response.Header.Set("x-ms-meta-"+name, value)
					}
					return response
				case req.URL.Query().Get("comp") == "metadata":
					require.Equal(t, http.MethodPut, req.Method)
					stored.metadata = map[string]string{}
					for name, values := range req.Header {
						if strings.HasPrefix(strings.ToLower(name), "x-ms-meta-") {
							stored.metadata[strings.ToLower(name[len("x-ms-meta-"):])] = values[0]
						}
					}
				default:
					t.Fatalf("unexpected lease request: %s %s (%s)", req.Method, req.URL, action)
				}
				return gcResponse(req, http.StatusOK, "")
			})},
	})
	require.NoError(t, err)
	return client
}

func TestClusterLeaseSerializesRunners(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		previousAzure, previousConfig := config.Azure, config.Config
		t.Cleanup(func() { config.Azure, config.Config = previousAzure, previousConfig })
		store := &testClusterLeaseStore{}
		config.Azure = &config.AzureClient{Blob: clusterLeaseTestClient(t, store)}
		config.Config = &config.Configuration{BlobContainer: "abe2e"}
		ctx, cancel := context.WithTimeout(logging.WithLogger(t.Context(), t), time.Minute)
		defer cancel()
		first, err := acquireClusterLease(ctx, "westus3-cluster")
		require.NoError(t, err)
		other, err := acquireClusterLease(ctx, "westus3-other-cluster")
		require.NoError(t, err, "independent clusters must not block each other")
		require.NoError(t, other.close())
		waitCtx, stopWaiting := context.WithTimeout(ctx, 6*time.Second)
		_, err = acquireClusterLease(waitCtx, "westus3-cluster")
		stopWaiting()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		acquired := make(chan *clusterLease, 1)
		failed := make(chan error, 1)
		go func() {
			second, err := acquireClusterLease(ctx, "westus3-cluster")
			if err != nil {
				failed <- err
				return
			}
			acquired <- second
		}()
		synctest.Wait()
		select {
		case <-acquired:
			t.Fatal("another runner acquired the same cluster lease")
		default:
		}
		require.NoError(t, first.setRecovery(ctx, "old-node-group"))
		require.NoError(t, first.close())
		select {
		case err := <-failed:
			t.Fatal(err)
		case second := <-acquired:
			require.Equal(t, "old-node-group", second.nodeName, "recovery intent must survive lease handoff")
			require.NoError(t, second.setRecovery(ctx, ""))
			require.NoError(t, second.close())
		}
	})
}

func TestClusterLeaseRenewal(t *testing.T) {
	for _, failRenew := range []bool{false, true} {
		t.Run(fmt.Sprintf("failure=%t", failRenew), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				previousAzure, previousConfig := config.Azure, config.Config
				t.Cleanup(func() { config.Azure, config.Config = previousAzure, previousConfig })
				store := &testClusterLeaseStore{}
				if failRenew {
					store.failAction = "renew"
				}
				config.Azure = &config.AzureClient{Blob: clusterLeaseTestClient(t, store)}
				config.Config = &config.Configuration{BlobContainer: "abe2e"}
				lock, err := acquireClusterLease(logging.WithLogger(t.Context(), t), "westus3-cluster")
				require.NoError(t, err)
				time.Sleep(75 * time.Second)
				if failRenew {
					require.ErrorIs(t, lock.ctx.Err(), context.Canceled)
					require.ErrorContains(t, lock.close(), "renewing cluster lifecycle lease")
				} else {
					require.NoError(t, lock.ctx.Err())
					require.NoError(t, lock.close())
					require.GreaterOrEqual(t, store.renewals, 3)
				}
			})
		})
	}
}

func TestClusterLeaseErrors(t *testing.T) {
	for _, action := range []string{"acquire", "metadata", "release"} {
		t.Run(action, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				previousAzure, previousConfig := config.Azure, config.Config
				t.Cleanup(func() { config.Azure, config.Config = previousAzure, previousConfig })
				config.Azure = &config.AzureClient{Blob: clusterLeaseTestClient(t, &testClusterLeaseStore{failAction: action})}
				config.Config = &config.Configuration{BlobContainer: "abe2e"}
				ctx := logging.WithLogger(t.Context(), t)
				lock, err := acquireClusterLease(ctx, "westus3-cluster")
				if action == "acquire" {
					require.ErrorContains(t, err, "acquiring cluster lifecycle lease")
					return
				}
				require.NoError(t, err)
				if action == "metadata" {
					require.ErrorContains(t, lock.setRecovery(ctx, "old-node-group"), "updating cluster recovery marker")
					require.NoError(t, lock.close())
				} else {
					require.ErrorContains(t, lock.close(), "releasing cluster lifecycle lease")
				}
			})
		})
	}
}

func TestClusterHashIsolatesCoordinationStores(t *testing.T) {
	previousConfig := config.Config
	t.Cleanup(func() { config.Config = previousConfig })
	config.Config = config.DefaultConfiguration()
	cluster := getAzureNetworkClusterModel("abe2e-test", "westus3", "Standard_D2ds_v5")
	original := hash(cluster)
	config.Config.BlobContainer += "-other"
	require.NotEqual(t, original, hash(cluster), "runners using different lease stores must not share a cluster")
	legacy, err := json.Marshal(struct {
		Cluster       *armcontainerservice.ManagedCluster
		ResourceGroup string
	}{cluster, config.ClusterResourceGroupName(*cluster.Location, *cluster.Name)})
	require.NoError(t, err)
	legacyHash := sha256.Sum256(legacy)
	require.NotEqual(t, hex.EncodeToString(legacyHash[:])[:5], original, "uncoordinated runners must use different clusters")
}
