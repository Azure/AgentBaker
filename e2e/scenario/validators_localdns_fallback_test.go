package scenario

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestLocalDNSLookupPod(t *testing.T) {
	s := &Scenario{Runtime: &ScenarioRuntime{VM: &ScenarioVM{KubeName: "scenario-node"}}}
	names := []string{"kubernetes.default.svc.cluster.local", "mcr.microsoft.com"}
	pod := localDNSLookupPod(s, "cluster", "169.254.10.11", names)

	assert.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)
	assert.Equal(t, int64(120), *pod.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, "scenario-node", pod.Spec.NodeSelector["kubernetes.io/hostname"])
	assert.False(t, pod.Spec.HostNetwork)
	require.Len(t, pod.Spec.Containers, 2)
	for i, container := range pod.Spec.Containers {
		assert.Equal(t, []string{"nslookup", names[i], "169.254.10.11"}, container.Command)
		assert.Equal(t, corev1.PullIfNotPresent, container.ImagePullPolicy)
	}
	assert.NotEqual(t, pod.Name, localDNSLookupPod(s, "cluster", "169.254.10.11", names).Name)

	pod = localDNSLookupPod(s, "node", "169.254.10.10", names[1:])
	require.Len(t, pod.Spec.Containers, 1)
	assert.Equal(t, []string{"nslookup", "mcr.microsoft.com", "169.254.10.10"}, pod.Spec.Containers[0].Command)
}

func TestLocalDNSLookupWaitRequiresCompletion(t *testing.T) {
	for _, tc := range []struct {
		phase corev1.PodPhase
		ready bool
	}{
		{corev1.PodSucceeded, false},
		{corev1.PodFailed, false},
		{corev1.PodPending, false},
		{corev1.PodRunning, true},
	} {
		t.Run(string(tc.phase), func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "dns-probe", Namespace: "default"},
				Status:     corev1.PodStatus{Phase: tc.phase},
			}
			if tc.ready {
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			}
			if tc.phase == corev1.PodFailed {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
					Name: "lookup-1",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 1, Reason: "Error", Message: "server returned NXDOMAIN",
					}},
				}}
			}
			logger := &executionLogger{}
			ctx, cancel := context.WithTimeout(logging.WithLogger(context.Background(), logger), 50*time.Millisecond)
			defer cancel()
			kube := &Kubeclient{Typed: fake.NewSimpleClientset(pod)}
			err := waitForLocalDNSLookupPod(ctx, kube, pod)
			switch tc.phase {
			case corev1.PodSucceeded:
				require.NoError(t, err)
			case corev1.PodFailed:
				require.ErrorContains(t, err, "DNS pod dns-probe failed")
				assert.ErrorContains(t, err, "lookup-1 terminated(reason=Error, exitCode=1)")
				assert.Contains(t, strings.Join(logger.logs, "\n"), "server returned NXDOMAIN")
			default:
				require.ErrorIs(t, err, context.DeadlineExceeded)
			}
			if err != nil {
				require.NotEmpty(t, logger.logs, "failed and timed-out probes must include pod diagnostics")
			}
		})
	}
}

func TestLocalDNSLookupWaitReportsAPIError(t *testing.T) {
	client := fake.NewSimpleClientset()
	apiErr := errors.New("API unavailable")
	client.PrependReactor("get", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apiErr
	})
	ctx := logging.WithLogger(context.Background(), &executionLogger{})
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "dns-probe", Namespace: "default"}}
	require.ErrorIs(t, waitForLocalDNSLookupPod(ctx, &Kubeclient{Typed: client}, pod), apiErr)
}

func TestLocalDNSLookupPodCleanup(t *testing.T) {
	for _, phase := range []corev1.PodPhase{corev1.PodSucceeded, corev1.PodFailed} {
		t.Run(string(phase), func(t *testing.T) {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "scenario-node", UID: "node-uid"}}
			client := fake.NewSimpleClientset(node)
			ctx := logging.WithLogger(context.Background(), &executionLogger{})
			s := &Scenario{Runtime: &ScenarioRuntime{
				VM: &ScenarioVM{KubeName: node.Name}, Kube: &Kubeclient{Typed: client},
			}}
			pod := localDNSLookupPod(s, "test", "169.254.10.10", []string{"mcr.microsoft.com"})
			client.PrependReactor("get", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
				current := pod.DeepCopy()
				current.Status.Phase = phase
				return true, current, nil
			})
			cleanupErr := errors.New("delete denied")
			deleted := false
			client.PrependReactor("delete", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
				deleted = true
				assert.Equal(t, pod.Name, action.(k8stesting.DeleteAction).GetName())
				return true, nil, cleanupErr
			})

			err := runLocalDNSLookupPod(ctx, s, pod)
			require.ErrorIs(t, err, cleanupErr)
			assert.True(t, deleted)
			if phase == corev1.PodFailed {
				assert.ErrorContains(t, err, "DNS pod "+pod.Name+" failed")
			}
			require.Len(t, pod.OwnerReferences, 1)
			assert.Equal(t, node.UID, pod.OwnerReferences[0].UID)
		})
	}
}

func TestLocalDNSNATRules(t *testing.T) {
	nat := "-N LOCALDNS-FALLBACK\n-A PREROUTING -j LOCALDNS-FALLBACK\n-A OUTPUT -j LOCALDNS-FALLBACK\n"
	require.NoError(t, validateLocalDNSNATRules(nat, "", true))
	require.NoError(t, validateLocalDNSNATRules("", "", false))
	require.Error(t, validateLocalDNSNATRules(nat, "", false))
	for _, missing := range strings.Split(strings.TrimSpace(nat), "\n") {
		t.Run("missing "+missing, func(t *testing.T) {
			require.Error(t, validateLocalDNSNATRules(strings.ReplaceAll(nat, missing+"\n", ""), "", true))
		})
	}
	for _, ip := range []string{"169.254.10.10", "169.254.10.11"} {
		t.Run("NOTRACK "+ip, func(t *testing.T) {
			require.ErrorContains(t, validateLocalDNSNATRules(nat,
				"-A PREROUTING -d "+ip+"/32 -p udp -j NOTRACK", true), "NOTRACK")
		})
	}
	require.NoError(t, validateLocalDNSNATRules(nat,
		"-A PREROUTING -d 169.254.10.100/32 -p udp -j NOTRACK", true))
}
