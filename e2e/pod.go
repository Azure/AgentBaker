package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ensurePod(ctx context.Context, s *Scenario, pod *corev1.Pod) {
	kube := s.Runtime.Cluster.Kube
	truncatePodName(s.T, pod)
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

	_, err = kube.WaitUntilPodRunning(ctx, s.T, pod.Namespace, "", "metadata.name="+pod.Name)
	require.NoErrorf(s.T, err, "failed to wait for pod %q to be in running state", pod.Name)
}

func truncatePodName(t *testing.T, pod *corev1.Pod) {
	name := pod.Name
	if len(pod.Name) < 63 {
		return
	}
	pod.Name = pod.Name[:63]
	pod.Name = strings.TrimRight(pod.Name, "-")
	t.Logf("truncated pod name %q to %q", name, pod.Name)
}
