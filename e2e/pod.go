package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
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
	if len(pod.Name) < 63 {
		return
	}
	pod.Name = pod.Name[:63]
	pod.Name = strings.TrimRight(pod.Name, "-")
	t.Logf("truncated pod name to %q", pod.Name)
}

func ensureJob(ctx context.Context, s *Scenario, job *batchv1.Job) {
	s.T.Logf("creating job %q", job.Name)
	kube := s.Runtime.Cluster.Kube
	_, err := kube.Typed.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	require.NoErrorf(s.T, err, "failed to create job %q", job.Name)
	s.T.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		err := kube.Typed.BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))})
		if err != nil {
			s.T.Logf("couldn't not delete job %s: %v", job.Name, err)
		}
	})
	watch, err := kube.Typed.BatchV1().Jobs(job.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", job.Name),
	})
	require.NoErrorf(s.T, err, "failed to watch job %q", job.Name)
	defer watch.Stop()
	s.T.Logf("waiting for job %q to complete", job.Name)
	for event := range watch.ResultChan() {
		job := event.Object.(*batchv1.Job)
		if job.Status.Failed > 0 {
			require.Failf(s.T, "job %q failed", job.Name)
		}

		if job.Status.Succeeded > 0 {
			return
		}
	}
}
