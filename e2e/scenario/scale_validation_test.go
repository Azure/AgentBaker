package e2e

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScaleValidationDeploymentTargetsOnlyScenarioNode(t *testing.T) {
	t.Parallel()

	s := &Scenario{Runtime: &ScenarioRuntime{VM: &ScenarioVM{KubeName: "scenario-node"}}}
	deployment := scaleValidationDeployment(s, 42)

	if got, want := *deployment.Spec.Replicas, int32(42); got != want {
		t.Fatalf("replicas = %d, want %d", got, want)
	}
	if got, want := deployment.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"], "scenario-node"; got != want {
		t.Fatalf("node selector hostname = %q, want %q", got, want)
	}
	if got, want := deployment.Spec.Template.Spec.Containers[0].Image, scaleValidationImage; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
	if deployment.Spec.Template.Spec.HostNetwork {
		t.Fatal("scale validation pods must use the normal pod network")
	}
	if got, want := deployment.Spec.Selector.MatchLabels["app"], deployment.Spec.Template.Labels["app"]; got != want {
		t.Fatalf("selector label = %q, pod template label = %q", got, want)
	}
}

func TestAllocatablePodCapacity(t *testing.T) {
	t.Parallel()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "scenario-node"},
		Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{
			corev1.ResourcePods: resource.MustParse("110"),
		}},
	}

	got, err := allocatablePodCapacity(node)
	if err != nil {
		t.Fatalf("allocatablePodCapacity returned an error: %v", err)
	}
	if want := int32(110); got != want {
		t.Fatalf("allocatablePodCapacity = %d, want %d", got, want)
	}

	if _, err := allocatablePodCapacity(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "missing-capacity"}}); err == nil {
		t.Fatal("allocatablePodCapacity returned nil error for missing capacity")
	}
}

func TestAvailableScaleReplicas(t *testing.T) {
	t.Parallel()

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "scale-test"}},
			Spec:       corev1.PodSpec{NodeName: "scenario-node"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "system"}},
			Spec:       corev1.PodSpec{NodeName: "scenario-node"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "scale-test"}},
			Spec:       corev1.PodSpec{NodeName: "scenario-node"},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "system"}},
			Spec:       corev1.PodSpec{NodeName: "scenario-node"},
			Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "scale-test"}},
			Spec:       corev1.PodSpec{NodeName: "another-node"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	}

	got, err := availableScaleReplicas(10, "scenario-node", "scale-test", pods)
	if err != nil {
		t.Fatalf("availableScaleReplicas returned an error: %v", err)
	}
	if want := int32(8); got != want {
		t.Fatalf("availableScaleReplicas = %d, want %d", got, want)
	}

	got, err = availableScaleReplicas(10, "scenario-node", "", pods)
	if err != nil {
		t.Fatalf("availableScaleReplicas for initial target returned an error: %v", err)
	}
	if want := int32(6); got != want {
		t.Fatalf("initial availableScaleReplicas = %d, want %d", got, want)
	}

	failedValidationPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "failed-scale-pod",
			Labels: map[string]string{"app": "scale-test"},
		},
		Spec:   corev1.PodSpec{NodeName: "scenario-node"},
		Status: corev1.PodStatus{Phase: corev1.PodFailed},
	}
	if _, err := availableScaleReplicas(10, "scenario-node", "scale-test", append(pods, failedValidationPod)); err == nil {
		t.Fatal("availableScaleReplicas returned nil error for a failed validation pod")
	}
}

func TestScaleValidationNodeReady(t *testing.T) {
	t.Parallel()

	node := &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
		Type:   corev1.NodeReady,
		Status: corev1.ConditionTrue,
	}}}}
	if !scaleValidationNodeReady(node) {
		t.Fatal("scaleValidationNodeReady returned false for a Ready node")
	}

	node.Status.Conditions[0].Status = corev1.ConditionFalse
	if scaleValidationNodeReady(node) {
		t.Fatal("scaleValidationNodeReady returned true for a NotReady node")
	}
}
