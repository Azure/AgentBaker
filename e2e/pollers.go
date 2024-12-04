package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/yaml"
)

const (
	defaultPollInterval = time.Second
)

func waitUntilNodeReady(ctx context.Context, t *testing.T, kube *Kubeclient, vmssName string) string {
	var nodeName string
	nodeStatus := corev1.NodeStatus{}
	found := false

	t.Logf("waiting for node %s to be ready", vmssName)

	err := wait.PollUntilContextCancel(ctx, defaultPollInterval, true, func(ctx context.Context) (bool, error) {
		nodes, err := kube.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, node := range nodes.Items {
			if strings.HasPrefix(node.Name, vmssName) {
				found = true
				nodeStatus = node.Status

				for _, cond := range node.Status.Conditions {
					if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
						nodeName = node.Name
						return true, nil
					}
				}
			}
		}

		return false, nil
	})
	if !found {
		t.Logf("node %q isn't connected to the AKS cluster", vmssName)
	}
	require.NoError(t, err, "failed to find or wait for %q to be ready %+v", vmssName, nodeStatus)
	t.Logf("node %s is ready", nodeName)

	return nodeName
}

func waitUntilPodReady(ctx context.Context, kube *Kubeclient, podName string, t *testing.T) error {
	const logInterval = 5 * time.Minute // log every 5 minutes

	t.Logf("waiting for pod %s to be ready", podName)

	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	lastLogTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pod, err := kube.Typed.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					t.Logf("pod %s not found yet. Err %v", podName, err)
					continue
				}
				return err
			}

			if time.Since(lastLogTime) > logInterval {
				lastLogTime = time.Now()
				logPodDebugInfo(ctx, kube, pod, t)
				if deadline, ok := ctx.Deadline(); ok {
					remaining := time.Until(deadline)
					t.Logf("time before timeout: %v\n", remaining)
				}
			}

			if pod.Status.Phase == corev1.PodPending {
				continue
			}

			if pod.Status.Phase != corev1.PodRunning {
				// Check for CrashLoopBackOff
				for _, containerStatus := range pod.Status.ContainerStatuses {
					if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
						logPodDebugInfo(ctx, kube, pod, t)
						return fmt.Errorf("pod %s is in CrashLoopBackOff state", podName)
					}
				}

				podStatus, _ := yaml.Marshal(pod.Status)
				logPodDebugInfo(ctx, kube, pod, t)
				return fmt.Errorf("pod %s is in %s phase, status: %s", podName, pod.Status.Phase, string(podStatus))
			}

			// Check if the pod is ready
			for _, cond := range pod.Status.Conditions {
				if cond.Type == "Ready" && cond.Status == "True" {
					t.Logf("pod %s is ready", podName)
					return nil
				}
			}
		}
	}
}

func logPodDebugInfo(ctx context.Context, kube *Kubeclient, pod *corev1.Pod, t *testing.T) {
	// Collects all log lines and outputs them in a single log message.
	var logBuilder strings.Builder

	logBuilder.WriteString("-- pod metadata --\n")
	logBuilder.WriteString(fmt.Sprintf("   Name: %s\n               Namespace: %s\n               Node: %s\n               Status: %s\n               Start Time: %s\n", pod.Name, pod.Namespace, pod.Spec.NodeName, pod.Status.Phase, pod.Status.StartTime))
	for _, condition := range pod.Status.Conditions {
		logBuilder.WriteString(fmt.Sprintf("   Reason: %s\n", condition.Reason))
		logBuilder.WriteString(fmt.Sprintf("   Message: %s\n", condition.Message))
	}

	logBuilder.WriteString("-- container(s) info --\n")
	for _, container := range pod.Spec.Containers {
		logBuilder.WriteString(fmt.Sprintf("   Container: %s\n               Image: %s\n               Ports: %v\n", container.Name, container.Image, container.Ports))
	}

	logBuilder.WriteString("-- pod events --")
	events, _ := kube.Typed.CoreV1().Events(defaultNamespace).List(ctx, metav1.ListOptions{FieldSelector: "involvedObject.name=" + pod.Name})
	for _, event := range events.Items {
		logBuilder.WriteString(fmt.Sprintf("   Reason: %s, Message: %s, Count: %d, Last Timestamp: %s\n", event.Reason, event.Message, event.Count, event.LastTimestamp))
	}

	t.Log(logBuilder.String())
}

func waitUntilClusterReady(ctx context.Context, name string) (*armcontainerservice.ManagedCluster, error) {
	var cluster armcontainerservice.ManagedClustersClientGetResponse
	err := wait.PollUntilContextCancel(ctx, defaultPollInterval, true, func(ctx context.Context) (bool, error) {
		var err error
		cluster, err = config.Azure.AKS.Get(ctx, config.ResourceGroupName, name, nil)
		if err != nil {
			return false, err
		}
		switch *cluster.ManagedCluster.Properties.ProvisioningState {
		case "Succeeded":
			return true, nil
		case "Updating", "Assigned", "Creating":
			return false, nil
		default:
			return false, fmt.Errorf("cluster %s is in state %s", name, *cluster.ManagedCluster.Properties.ProvisioningState)
		}
	})
	if err != nil {
		return nil, err
	}
	return &cluster.ManagedCluster, err
}
