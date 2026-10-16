package scenario

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestKataCCRuncSmoke(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase corev1.PodPhase
		node  string
		exit  int32
		want  string
	}{
		{name: "completed on test node", phase: corev1.PodSucceeded, node: "test-node"},
		{name: "wrong node", phase: corev1.PodSucceeded, node: "other-node", want: "instead of test node"},
		{name: "failed workload", phase: corev1.PodFailed, node: "test-node", exit: 1, want: "runc smoke pod failed"},
		{name: "nonzero exit", phase: corev1.PodSucceeded, node: "test-node", exit: 1, want: "missing successful container termination"},
		{name: "running is insufficient", phase: corev1.PodRunning, node: "test-node", want: "context deadline exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test-node", UID: "test-node-uid"}}
			client := fake.NewSimpleClientset(node)
			s := &Scenario{
				Runtime: &ScenarioRuntime{
					VM:   &ScenarioVM{KubeName: node.Name},
					Kube: &Kubeclient{Typed: client},
				},
				cleanup: &scenarioCleanup{},
			}
			client.PrependReactor("get", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
				if action.GetSubresource() != "" {
					return false, nil, nil
				}
				obj, err := client.Tracker().Get(corev1.SchemeGroupVersion.WithResource("pods"), "default", action.(k8stesting.GetAction).GetName())
				if err != nil {
					return true, nil, err
				}
				pod := obj.(*corev1.Pod).DeepCopy()
				pod.Spec.NodeName = tc.node
				pod.Status.Phase = tc.phase
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
					Name: "busybox", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: tc.exit}},
				}}
				return true, pod, nil
			})
			ctx, cancel := context.WithTimeout(logging.WithLogger(context.Background(), &executionLogger{}), 100*time.Millisecond)
			defer cancel()
			err := validateKataCCRuncPod(ctx, s)
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.want)
			}

			pods, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, pods.Items, 1)
			pod := pods.Items[0]
			require.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)
			require.Equal(t, node.Name, pod.Spec.NodeSelector["kubernetes.io/hostname"])
			require.Equal(t, node.UID, pod.OwnerReferences[0].UID)
			require.NotNil(t, pod.Spec.RuntimeClassName)
			rc, err := client.NodeV1().RuntimeClasses().Get(context.Background(), *pod.Spec.RuntimeClassName, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, "runc", rc.Handler)

			require.NoError(t, s.cleanup.runCleanups(context.Background()))
			pods, err = client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Empty(t, pods.Items)
			runtimeClasses, err := client.NodeV1().RuntimeClasses().List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Empty(t, runtimeClasses.Items)
		})
	}
}
