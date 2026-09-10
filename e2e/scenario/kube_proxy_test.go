package scenario

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureProxyConfigMapCreatesAndUpdates(t *testing.T) {
	ctx := t.Context()
	typed := fake.NewClientset()
	kube := &Kubeclient{Typed: typed}
	require.NoError(t, kube.ensureProxyConfigMap(ctx))
	created, err := typed.CoreV1().ConfigMaps("default").Get(ctx, "e2e-proxy-config", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, created.Data["proxy.py"])

	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "e2e-proxy-config", Namespace: "default",
			ResourceVersion: "42",
			Annotations:     map[string]string{"other-owner": "keep"},
		},
		Data: map[string]string{"proxy.py": "old script", "other-key": "keep"},
	}
	typed = fake.NewClientset(existing)
	kube.Typed = typed
	require.NoError(t, kube.ensureProxyConfigMap(ctx))
	require.NoError(t, kube.ensureProxyConfigMap(ctx))
	updated, err := typed.CoreV1().ConfigMaps("default").Get(ctx, existing.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, created.Data["proxy.py"], updated.Data["proxy.py"])
	require.Equal(t, "keep", updated.Data["other-key"])
	require.Equal(t, existing.Annotations, updated.Annotations)
}
