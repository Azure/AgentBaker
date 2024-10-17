package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
	require.NoError(t, err, "failed to find or wait for %q to be ready %v", vmssName, nodeStatus)
	t.Logf("node %s is ready", nodeName)

	return nodeName
}

func waitUntilPodReady(ctx context.Context, kube *Kubeclient, podName string) error {
	return wait.PollUntilContextCancel(ctx, defaultPollInterval, true, func(ctx context.Context) (bool, error) {
		pod, err := kube.Typed.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
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
