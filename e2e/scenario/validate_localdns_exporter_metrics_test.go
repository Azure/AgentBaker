package scenario

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestValidateLocalDNSExporterMetricsSkipsOlderVHD(t *testing.T) {
	s := &Scenario{Runtime: &ScenarioRuntime{
		VM: &ScenarioVM{KubeName: "scenario-node"},
		Kube: &Kubeclient{Typed: fake.NewSimpleClientset(&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "scenario-node"},
		})},
	}}
	require.NoError(t, ValidateLocalDNSExporterMetrics(t.Context(), s))
}
