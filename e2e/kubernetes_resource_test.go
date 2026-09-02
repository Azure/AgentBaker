package e2e

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes/fake"
)

func TestUniqueKubernetesResourceName(t *testing.T) {
	t.Parallel()

	if first, second := uniqueKubernetesResourceName("test"), uniqueKubernetesResourceName("test"); first == second {
		t.Fatalf("expected unique names, got %q twice", first)
	}

	for _, test := range []struct {
		name       string
		base       string
		wantPrefix string
		wantLength int
	}{
		{
			name:       "long base",
			base:       strings.Repeat("a", 80) + "-",
			wantPrefix: strings.Repeat("a", 52) + "-",
			wantLength: 63,
		},
		{
			name:       "hyphen at truncation boundary",
			base:       strings.Repeat("a", 51) + "--" + strings.Repeat("b", 30),
			wantPrefix: strings.Repeat("a", 51) + "-",
			wantLength: 62,
		},
		{
			name:       "uppercase base",
			base:       "Kata-Node",
			wantPrefix: "kata-node-",
			wantLength: 20,
		},
		{
			name:       "empty base",
			base:       "---",
			wantPrefix: "",
			wantLength: 10,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := uniqueKubernetesResourceName(test.base)
			if len(name) != test.wantLength {
				t.Fatalf("expected a %d-character name, got %d characters: %q", test.wantLength, len(name), name)
			}
			if !strings.HasPrefix(name, test.wantPrefix) {
				t.Fatalf("expected prefix %q, got %q", test.wantPrefix, name)
			}
			if strings.Contains(name, "--") {
				t.Fatalf("expected no empty DNS label in %q", name)
			}
			if validationErrors := utilvalidation.IsDNS1123Label(name); len(validationErrors) > 0 {
				t.Fatalf("expected a valid DNS-1123 label, got %q: %v", name, validationErrors)
			}

			suffix := strings.TrimPrefix(name, test.wantPrefix)
			if len(suffix) != 10 {
				t.Fatalf("expected a 10-character suffix, got %q", suffix)
			}
			for _, char := range suffix {
				if !strings.ContainsRune(safeLowerBytes, char) {
					t.Fatalf("expected a DNS-safe suffix, got %q", suffix)
				}
			}
		})
	}
}

func TestAvailablePodSlots(t *testing.T) {
	t.Parallel()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "scenario-node"},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourcePods: resource.MustParse("10"),
			},
		},
	}
	pods := []corev1.Pod{
		{Spec: corev1.PodSpec{NodeName: node.Name}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{Spec: corev1.PodSpec{NodeName: node.Name}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
		{Spec: corev1.PodSpec{NodeName: node.Name}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
		{Spec: corev1.PodSpec{NodeName: node.Name}, Status: corev1.PodStatus{Phase: corev1.PodFailed}},
		{
			ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time}},
			Spec:       corev1.PodSpec{NodeName: node.Name},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		{Spec: corev1.PodSpec{NodeName: "another-node"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
	}

	got, err := availablePodSlots(node, pods)
	if err != nil {
		t.Fatalf("availablePodSlots returned an error: %v", err)
	}
	if want := int32(8); got != want {
		t.Fatalf("availablePodSlots = %d, want %d", got, want)
	}
}

func TestAvailablePodSlotsRejectsMissingOrExhaustedCapacity(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		node *corev1.Node
		pods []corev1.Pod
	}{
		{
			name: "missing capacity",
			node: &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "scenario-node"}},
		},
		{
			name: "exhausted capacity",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "scenario-node"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("1")},
				},
			},
			pods: []corev1.Pod{{
				Spec:   corev1.PodSpec{NodeName: "scenario-node"},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := availablePodSlots(test.node, test.pods); err == nil {
				t.Fatal("availablePodSlots returned nil error")
			}
		})
	}
}

func TestSetScenarioNodeOwnerReference(t *testing.T) {
	t.Parallel()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scenario-node",
			UID:  types.UID("scenario-node-uid"),
		},
	}
	s := &Scenario{
		Runtime: &ScenarioRuntime{
			Kube: &Kubeclient{Typed: fake.NewSimpleClientset(node)},
			VM:   &ScenarioVM{KubeName: node.Name},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "apps/v1", Kind: "Deployment", Name: "existing-owner", UID: types.UID("existing-owner-uid")},
			},
		},
	}

	if err := setScenarioNodeOwnerReference(context.Background(), s, pod); err != nil {
		t.Fatalf("set owner reference: %v", err)
	}
	if err := setScenarioNodeOwnerReference(context.Background(), s, pod); err != nil {
		t.Fatalf("set owner reference again: %v", err)
	}

	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("expected one scenario Node owner reference, got %+v", pod.OwnerReferences)
	}
	if owner := pod.OwnerReferences[0]; owner.Name != node.Name || owner.UID != node.UID {
		t.Fatalf("expected scenario Node owner reference, got %+v", owner)
	}
}

func TestIsNodeReady(t *testing.T) {
	t.Parallel()

	readyNode := &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
		Type:   corev1.NodeReady,
		Status: corev1.ConditionTrue,
	}}}}
	notReadyNode := readyNode.DeepCopy()
	notReadyNode.Status.Conditions[0].Status = corev1.ConditionFalse
	unknownNode := readyNode.DeepCopy()
	unknownNode.Status.Conditions[0].Status = corev1.ConditionUnknown

	if !isNodeReady(readyNode) {
		t.Fatal("isNodeReady returned false for a Ready node")
	}
	if isNodeReady(notReadyNode) {
		t.Fatal("isNodeReady returned true for a NotReady node")
	}
	if isNodeReady(unknownNode) {
		t.Fatal("isNodeReady returned true for a node with unknown readiness")
	}
	if isNodeReady(&corev1.Node{}) {
		t.Fatal("isNodeReady returned true for a node without a Ready condition")
	}
}
