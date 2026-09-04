package e2e

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// uniqueKubernetesResourceName adds a run-unique suffix to a DNS-compatible base name.
func uniqueKubernetesResourceName(base string) string {
	const (
		maxNameLength = 63
		suffixLength  = 10
	)

	base = strings.TrimRight(strings.ToLower(base), "-")
	suffix := randomLowercaseString(suffixLength)
	maxBaseLength := maxNameLength - suffixLength - 1
	if len(base) > maxBaseLength {
		base = strings.TrimRight(base[:maxBaseLength], "-")
	}
	if base == "" {
		return suffix
	}

	return base + "-" + suffix
}

func scenarioNodeOwnerReference(ctx context.Context, s *Scenario) (metav1.OwnerReference, error) {
	nodeName := s.Runtime.VM.KubeName
	node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return metav1.OwnerReference{}, fmt.Errorf("get scenario node %q for resource ownership: %w", nodeName, err)
	}

	return metav1.OwnerReference{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Node",
		Name:       node.Name,
		UID:        node.UID,
	}, nil
}

func setScenarioNodeOwnerReference(ctx context.Context, s *Scenario, object metav1.Object) error {
	ownerReference, err := scenarioNodeOwnerReference(ctx, s)
	if err != nil {
		return err
	}

	object.SetOwnerReferences([]metav1.OwnerReference{ownerReference})
	return nil
}
