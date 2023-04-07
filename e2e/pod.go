package e2e_test

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

// Returns the name of a pod that's a member of the 'debug' daemonset, running on an aks-nodepool node.
func getDebugPodName(kube *kubeclient) (string, error) {
	podList := corev1.PodList{}
	if err := kube.dynamic.List(context.Background(), &podList, client.MatchingLabels{"app": "debug"}); err != nil {
		return "", fmt.Errorf("failed to list debug pod: %q", err)
	}

	if len(podList.Items) < 1 {
		return "", fmt.Errorf("failed to find debug pod, list by selector returned no results")
	}

	var podName string
	for _, pod := range podList.Items {
		if strings.Contains(pod.Spec.NodeName, "aks-nodepool") {
			podName = pod.ObjectMeta.Name
			break
		}
	}

	if podName == "" {
		return "", fmt.Errorf("expected to find at least one debug pod running on the original AKS nodepool")
	}

	return podName, nil
}

func ensureDebugDaemonset(ctx context.Context, kube *kubeclient) error {
	manifest := getDebugDaemonset()
	var ds appsv1.DaemonSet

	if err := yaml.Unmarshal([]byte(manifest), &ds); err != nil {
		return fmt.Errorf("failed to unmarshal debug daemonset manifest: %q", err)
	}

	desired := ds.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, kube.dynamic, &ds, func() error {
		ds = *desired
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to apply debug daemonset: %q", err)
	}

	return nil
}

func ensureTestNginxPod(ctx context.Context, kube *kubeclient, nodeName string) error {
	manifest := getNginxPodTemplate(nodeName)
	var obj corev1.Pod

	if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
		return fmt.Errorf("failed to unmarshal nginx pod manifest: %q", err)
	}

	desired := obj.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, kube.dynamic, &obj, func() error {
		obj = *desired
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to apply nginx test pod: %q", err)
	}

	return waitUntilPodRunning(ctx, kube, nodeName)
}
