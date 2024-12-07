package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	hostNetworkDebugAppLabel = "debug-mariner"
	podNetworkDebugAppLabel  = "debugnonhost-mariner"
)

// Returns the name of a pod that's a member of the 'debug' daemonset, running on an aks-nodepool node.
func getHostNetworkDebugPodName(ctx context.Context, kube *Kubeclient, t *testing.T) (string, error) {
	podList := corev1.PodList{}
	if err := kube.Dynamic.List(ctx, &podList, client.MatchingLabels{"app": hostNetworkDebugAppLabel}); err != nil {
		return "", fmt.Errorf("failed to list debug pod: %w", err)
	}
	if podList.Size() == 0 {
		return "", fmt.Errorf("failed to find host debug pod")
	}
	pod := podList.Items[0]
	err := waitUntilPodReady(ctx, kube, pod.Name, t)
	if err != nil {
		return "", fmt.Errorf("failed to wait for pod to be in running state: %w", err)
	}
	return pod.Name, nil
}

// Returns the name of a pod that's a member of the 'debugnonhost' daemonset running in the cluster - this will return
// the name of the pod that is running on the node created for specifically for the test case which is running validation checks.
func getPodNetworkDebugPodNameForVMSS(ctx context.Context, kube *Kubeclient, vmssName string, t *testing.T) (string, error) {
	podList := corev1.PodList{}
	if err := kube.Dynamic.List(ctx, &podList, client.MatchingLabels{"app": podNetworkDebugAppLabel}); err != nil {
		return "", fmt.Errorf("failed to list debug pod: %w", err)
	}

	for _, pod := range podList.Items {
		if strings.Contains(pod.Spec.NodeName, vmssName) {
			err := waitUntilPodReady(ctx, kube, pod.Name, t)
			if err != nil {
				return "", fmt.Errorf("failed to wait for pod to be in running state: %w", err)
			}
			return pod.Name, nil
		}
	}
	return "", fmt.Errorf("failed to find non host debug pod on node %s", vmssName)
}

func ensurePod(ctx context.Context, s *Scenario, pod *corev1.Pod) {
	kube := s.Runtime.Cluster.Kube
	if len(pod.Name) > 63 {
		pod.Name = pod.Name[:63]
		s.T.Logf("truncated pod name to %q", pod.Name)
	}
	s.T.Logf("creating pod %q", pod.Name)
	_, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	require.NoErrorf(s.T, err, "failed to create pod %q", pod.Name)
	s.T.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		err := kube.Typed.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))})
		if err != nil {
			s.T.Logf("couldn't not delete pod %s: %v", pod.Name, err)
		}
		s.T.Logf("deleted pod %q", pod.Name)
	})
	err = waitUntilPodReady(ctx, kube, pod.Name, s.T)
	s.T.Logf("pod %q is ready", pod.Name)
	require.NoErrorf(s.T, err, "failed to wait for pod %q to be in running state", pod.Name)
}
