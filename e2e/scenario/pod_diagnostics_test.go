package scenario

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLogPodDebugInfoIncludesConditions(t *testing.T) {
	t.Parallel()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-pod", Namespace: "default"},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{
				Reason:  "Unschedulable",
				Message: "Insufficient cpu",
			}},
		},
	}
	logger := &executionLogger{}
	ctx := logging.WithLogger(context.Background(), logger)
	logPodDebugInfo(ctx, &Kubeclient{Typed: fake.NewSimpleClientset(pod)}, pod)
	require.Len(t, logger.logs, 1)
	var info struct {
		Conditions []corev1.PodCondition
	}
	require.NoError(t, json.Unmarshal([]byte(logger.logs[0]), &info))
	assert.Equal(t, pod.Status.Conditions, info.Conditions)
}
