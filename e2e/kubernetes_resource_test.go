package e2e

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
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
