package k8s

import (
	"context"
	"fmt"

	"github.com/Azure/agentbakere2e/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

func GetPodIP(ctx context.Context, kube *client.Kube, namespaceName, podName string) (string, error) {
	pod, err := kube.Typed.CoreV1().Pods(namespaceName).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get pod %s/%s: %w", namespaceName, podName, err)
	}
	return pod.Status.PodIP, nil
}

func ApplyPodManifest(ctx context.Context, kube *client.Kube, manifest string) error {
	var podObj corev1.Pod
	if err := yaml.Unmarshal([]byte(manifest), &podObj); err != nil {
		return fmt.Errorf("failed to unmarshal Pod manifest: %w", err)
	}

	desired := podObj.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, kube.Dynamic, &podObj, func() error {
		podObj = *desired
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to apply Pod manifest: %w", err)
	}

	return nil
}

func EnsurePod(ctx context.Context, kube *client.Kube, namespace, podName, manifest string) error {
	if err := ApplyPodManifest(ctx, kube, manifest); err != nil {
		return fmt.Errorf("failed to ensure pod: %w", err)
	}
	if err := WaitUntilPodRunning(ctx, kube, namespace, podName); err != nil {
		return fmt.Errorf("failed to wait for pod to be in running state: %w", err)
	}
	return nil
}
