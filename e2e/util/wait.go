package util

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/agentbakere2e/clients"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	waitUntilNodeReadyPollInterval  = 5 * time.Second
	waitUntilPodRunningPollInterval = 5 * time.Second
	waitUntilPodDeletedPollInterval = 5 * time.Second

	waitUntilNodeReadyPollTimeout     = 1 * time.Minute
	waitUntilPodRunningPollingTimeout = 3 * time.Minute
	waitUntilPodDeletedPollingTimeout = 1 * time.Minute
)

func WaitUntilNodeReady(ctx context.Context, kube *clients.KubeClient, nodeName string) error {
	err := wait.PollImmediateWithContext(ctx, waitUntilNodeReadyPollInterval, waitUntilNodeReadyPollTimeout, func(ctx context.Context) (bool, error) {
		node, err := kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil {
		return fmt.Errorf("failed to find or wait for node %q to be ready: %w", nodeName, err)
	}

	return nil
}

func WaitUntilPodRunning(ctx context.Context, kube *clients.KubeClient, namespace, podName string) error {
	return wait.PollImmediateWithContext(ctx, waitUntilPodRunningPollInterval, waitUntilPodRunningPollingTimeout, func(ctx context.Context) (bool, error) {
		pod, err := kube.Typed.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return pod.Status.Phase == corev1.PodPhase("Running"), nil
	})
}

func WaitUntilPodDeleted(ctx context.Context, kube *clients.KubeClient, namespace, podName string) error {
	return wait.PollImmediateWithContext(ctx, waitUntilPodDeletedPollInterval, waitUntilPodDeletedPollingTimeout, func(ctx context.Context) (bool, error) {
		err := kube.Typed.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		return err == nil, err
	})
}
