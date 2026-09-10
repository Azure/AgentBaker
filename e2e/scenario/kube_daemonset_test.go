package scenario

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestCreateDaemonsetUsesSingleApplyRequest(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusOK, http.StatusForbidden, http.StatusUnprocessableEntity} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			ds := daemonsetDebug(t.Context(), hostNetworkDebugAppLabel, map[string]string{"kubernetes.azure.com/mode": "system"}, "", true, false)
			ds.ResourceVersion = "stale"
			ds.UID = types.UID("server-owned")
			ds.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "another-manager"}}
			ds.Status.NumberReady = 3
			original := ds.DeepCopy()
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assert.Equal(t, http.MethodPatch, r.Method)
				assert.Equal(t, "/apis/apps/v1/namespaces/default/daemonsets/debug-mariner-tolerated", r.URL.Path)
				assert.Equal(t, string(types.ApplyPatchType), r.Header.Get("Content-Type"))
				assert.Equal(t, "agentbaker-e2e", r.URL.Query().Get("fieldManager"))
				assert.Equal(t, "true", r.URL.Query().Get("force"))
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); !assert.NoError(t, err) {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				assert.Len(t, body, 4)
				assert.JSONEq(t, `"apps/v1"`, string(body["apiVersion"]))
				assert.JSONEq(t, `"DaemonSet"`, string(body["kind"]))
				assert.JSONEq(t, `{"name":"debug-mariner-tolerated","namespace":"default","labels":{"app":"debug-mariner-tolerated"}}`, string(body["metadata"]))
				var spec appsv1.DaemonSetSpec
				assert.NoError(t, json.Unmarshal(body["spec"], &spec))
				assert.Equal(t, ds.Spec, spec)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status >= 400 {
					reason := metav1.StatusReasonForbidden
					if status == http.StatusUnprocessableEntity {
						reason = metav1.StatusReasonInvalid
					}
					_ = json.NewEncoder(w).Encode(metav1.Status{
						TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"},
						Status:   metav1.StatusFailure, Code: int32(status), Reason: reason, Message: "apply rejected",
					})
					return
				}
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer server.Close()
			typed, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
			require.NoError(t, err)
			kube := &Kubeclient{Typed: typed}
			err = kube.CreateDaemonset(t.Context(), ds)
			switch status {
			case http.StatusForbidden:
				require.True(t, apierrors.IsForbidden(err), "%v", err)
			case http.StatusUnprocessableEntity:
				require.True(t, apierrors.IsInvalid(err), "%v", err)
			default:
				require.NoError(t, err)
			}
			assert.Equal(t, int32(1), requests.Load())
			assert.Equal(t, original, ds)
		})
	}
}

func TestCreateDaemonsetHonorsCancellation(t *testing.T) {
	typed, err := kubernetes.NewForConfig(&rest.Config{Host: "https://unused.invalid"})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ds := daemonsetDebug(ctx, hostNetworkDebugAppLabel, nil, "", true, false)
	err = (&Kubeclient{Typed: typed}).CreateDaemonset(ctx, ds)
	require.ErrorIs(t, err, context.Canceled)
}
