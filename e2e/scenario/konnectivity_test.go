package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestEnsureKonnectivityAgentAutoscaler(t *testing.T) {
	t.Parallel()
	for _, existing := range []bool{false, true} {
		name := "create"
		if existing {
			name = "update"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			builder := fake.NewClientBuilder().WithScheme(scheme)
			key := types.NamespacedName{Namespace: metav1.NamespaceSystem, Name: "konnectivity-agent-autoscaler"}
			if existing {
				builder.WithObjects(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: key.Namespace,
						Name:      key.Name,
						Labels:    map[string]string{"existing": "label"},
					},
					Data: map[string]string{
						"ladder": `{"nodesToReplicas":[[1,2],[100,3]],"coresToReplicas":[[1,4]]}`,
						"other":  "preserved",
					},
				})
			}
			kube := &Kubeclient{Dynamic: builder.Build()}
			require.NoError(t, kube.EnsureKonnectivityAgentAutoscaler(t.Context()))
			var cm corev1.ConfigMap
			require.NoError(t, kube.Dynamic.Get(t.Context(), key, &cm))
			var ladder struct {
				NodesToReplicas [][2]int `json:"nodesToReplicas"`
				CoresToReplicas [][2]int `json:"coresToReplicas"`
			}
			require.NoError(t, json.Unmarshal([]byte(cm.Data["ladder"]), &ladder))
			require.Equal(t, [][2]int{{1, 3}}, ladder.NodesToReplicas)
			require.Equal(t, [][2]int{}, ladder.CoresToReplicas)
			if existing {
				require.Equal(t, "label", cm.Labels["existing"])
				require.Equal(t, "preserved", cm.Data["other"])
			}
			before := cm.DeepCopy()
			require.NoError(t, kube.EnsureKonnectivityAgentAutoscaler(t.Context()))
			require.NoError(t, kube.Dynamic.Get(t.Context(), key, &cm))
			require.Equal(t, before, &cm)
		})
	}
}

func TestEnsureKonnectivityAgentAutoscalerRetriesConcurrentCreation(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	creates := 0
	kube := &Kubeclient{Dynamic: fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			creates++
			require.NoError(t, c.Create(ctx, obj, opts...))
			return apierrors.NewAlreadyExists(schema.GroupResource{Resource: "configmaps"}, obj.GetName())
		},
	}).Build()}
	require.NoError(t, kube.EnsureKonnectivityAgentAutoscaler(t.Context()))
	require.Equal(t, 1, creates)
}

func TestEnsureKonnectivityAgentAutoscalerReturnsAPIErrors(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	wantErr := errors.New("API unavailable")
	kube := &Kubeclient{Dynamic: fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
			return wantErr
		},
	}).Build()}
	require.ErrorIs(t, kube.EnsureKonnectivityAgentAutoscaler(t.Context()), wantErr)
}

func TestEnsureKonnectivityAgentAutoscalerRetriesUpdateConflict(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "konnectivity-agent-autoscaler",
			Namespace: metav1.NamespaceSystem,
		},
	}
	updates := 0
	kube := &Kubeclient{Dynamic: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).WithInterceptorFuncs(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			updates++
			if updates == 1 {
				return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, obj.GetName(), errors.New("concurrent update"))
			}
			return c.Update(ctx, obj, opts...)
		},
	}).Build()}
	require.NoError(t, kube.EnsureKonnectivityAgentAutoscaler(t.Context()))
	require.Equal(t, 2, updates)
	require.NoError(t, kube.Dynamic.Get(t.Context(), client.ObjectKeyFromObject(cm), cm))
	require.JSONEq(t, `{"nodesToReplicas":[[1,3]],"coresToReplicas":[]}`, cm.Data["ladder"])
}
