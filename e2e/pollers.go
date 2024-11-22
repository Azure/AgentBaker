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
	lastLogTime := time.Now()
	logInterval := 5 * time.Minute // log every 5 minutes

	return wait.PollUntilContextCancel(ctx, defaultPollInterval, true, func(ctx context.Context) (bool, error) {
		currentLogTime := time.Now()

		pod, err := kube.Typed.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})

		printLog := false
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if currentLogTime.Sub(lastLogTime) > logInterval {
				// this logs every 5 minutes to reduce spam, iterations of poller are continuning as normal.
				logPodDebugInfo(ctx, kube, pod, t)
				t.Logf("time before timeout: %v\n\n", remaining)
				lastLogTime = currentLogTime
				printLog = true
			}
		}

		if err != nil {
			// pod might not be created yet, let the poller continue
			if errors.IsNotFound(err) {
				if printLog {
					// this logs every 5 minutes to reduce spam, iterations of poller are continuning as normal.
					t.Logf("pod %s not found yet. Err %v", podName, err)
				}
				return false, nil
			}
			return false, err
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
				return false, fmt.Errorf("pod %s is in CrashLoopBackOff state", podName)
			}
		}

		if pod.Status.Phase == "Pending" {
			return false, nil
		}

		if pod.Status.Phase != "Running" {
			podStatus, _ := yaml.Marshal(pod.Status)
			return false, fmt.Errorf("pod %s is in %s phase, status: %s", podName, pod.Status.Phase, string(podStatus))
		}

		for _, cond := range pod.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				return true, nil
			}
		}
		return false, nil
	})
}

func logPodDebugInfo(ctx context.Context, kube *Kubeclient, pod *corev1.Pod, t *testing.T) {
	t.Logf("-- pod metadata --\n")
	t.Logf("   Name: %s\n               Namespace: %s\n               Node: %s\n              Status: %s\n               Start Time: %s\n", pod.Name, pod.Namespace, pod.Spec.NodeName, pod.Status.Phase, pod.Status.StartTime)
	for _, condition := range pod.Status.Conditions {
		t.Logf("    Reason: %s\n", condition.Reason)
		t.Logf("    Message: %s\n", condition.Message)
	}
	t.Logf("-- container(s) info --\n")
	for _, container := range pod.Spec.Containers {
		t.Logf("   Container: %s\n               Image: %s\n               Ports: %v\n", container.Name, container.Image, container.Ports)
	}
	t.Logf("-- pod events --")
	events, _ := kube.Typed.CoreV1().Events(defaultNamespace).List(ctx, metav1.ListOptions{FieldSelector: "involvedObject.name=" + pod.Name})
	for _, event := range events.Items {
		t.Logf("   Reason: %s, Message: %s, Count: %d, Last Timestamp: %s\n", event.Reason, event.Message, event.Count, event.LastTimestamp)
	}
}

func waitUntilClusterReady(ctx context.Context, rg, name string) (*armcontainerservice.ManagedCluster, error) {
	var cluster armcontainerservice.ManagedClustersClientGetResponse
	err := wait.PollUntilContextCancel(ctx, defaultPollInterval, true, func(ctx context.Context) (bool, error) {
		var err error
		cluster, err = config.Azure.AKS.Get(ctx, rg, name, nil)
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
