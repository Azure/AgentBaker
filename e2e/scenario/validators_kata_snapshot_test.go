package scenario

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSingleSandboxID(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{name: "one sandbox", output: "abc123\n", want: "abc123"},
		{name: "no sandbox", output: "\n", wantErr: true},
		{name: "two sandboxes", output: "abc123\ndef456\n", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := singleSandboxID(test.output)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestKataSnapshotCaseNewPod(t *testing.T) {
	t.Parallel()

	c := &kataSnapshotCase{
		s:                &Scenario{Runtime: &ScenarioRuntime{VM: &ScenarioVM{KubeName: "kata-node"}}},
		runtimeClassName: "kata-v2-rc",
	}

	source := c.newPod(kataSnapshotPodSpec{name: "source"})
	require.NotContains(t, source.Annotations, kataSnapshotAnnotation)
	require.Equal(t, "kata-v2-rc", *source.Spec.RuntimeClassName)
	require.Equal(t, corev1.RestartPolicyNever, source.Spec.RestartPolicy)
	require.Equal(t, "kata-node", source.Spec.NodeSelector["kubernetes.io/hostname"])
	require.Empty(t, source.Spec.InitContainers)
	require.Empty(t, source.Spec.Volumes)

	restored := c.newPod(kataSnapshotPodSpec{
		name:          "restored",
		restoreFrom:   source.Name,
		configMap:     "snap-config",
		configValue:   "target-generation",
		initContainer: true,
		restartPolicy: corev1.RestartPolicyAlways,
	})
	require.Equal(t, source.Name, restored.Annotations[kataSnapshotAnnotation])
	require.Equal(t, corev1.RestartPolicyAlways, restored.Spec.RestartPolicy)
	require.Len(t, restored.Spec.InitContainers, 1)
	require.Equal(t, "snap-config", restored.Spec.Volumes[0].ConfigMap.Name)
	require.Equal(t, "/etc/snap", restored.Spec.Containers[0].VolumeMounts[0].MountPath)
	require.Contains(t, restored.Spec.Containers[0].ReadinessProbe.Exec.Command[2], `= target-generation`)
}

func TestAssertInitContainersSucceeded(t *testing.T) {
	t.Parallel()

	podWith := func(states ...corev1.ContainerState) *corev1.Pod {
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod"}}
		for _, state := range states {
			pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, corev1.ContainerStatus{Name: "setup", State: state})
		}
		return pod
	}
	succeeded := corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}
	failed := corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}
	running := corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}

	require.NoError(t, assertInitContainersSucceeded(podWith(succeeded)))
	require.Error(t, assertInitContainersSucceeded(podWith()))
	require.Error(t, assertInitContainersSucceeded(podWith(failed)))
	require.Error(t, assertInitContainersSucceeded(podWith(running)))
}

func TestKataSnapshotCaseTeardownDeletesTrackedObjects(t *testing.T) {
	t.Parallel()

	tracked := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "tracked", Namespace: defaultNamespace}}
	untracked := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "untracked", Namespace: defaultNamespace}}
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config", Namespace: defaultNamespace}}
	client := fake.NewSimpleClientset(tracked, untracked, configMap)
	c := &kataSnapshotCase{
		s:          &Scenario{Runtime: &ScenarioRuntime{Kube: &Kubeclient{Typed: client}}},
		pods:       []string{tracked.Name, "already-gone"},
		configMaps: []string{configMap.Name},
	}

	require.NoError(t, c.teardown(context.Background()))

	pods := client.CoreV1().Pods(defaultNamespace)
	_, err := pods.Get(context.Background(), tracked.Name, metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err), "tracked pod should be deleted, got %v", err)
	_, err = pods.Get(context.Background(), untracked.Name, metav1.GetOptions{})
	require.NoError(t, err, "teardown must not delete pods it did not create")
	_, err = client.CoreV1().ConfigMaps(defaultNamespace).Get(context.Background(), configMap.Name, metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err), "tracked ConfigMap should be deleted, got %v", err)
}
