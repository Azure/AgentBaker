package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/yaml"
)

const (
	// Polling intervals
	execOnVMPollInterval                 = 5 * time.Second
	extractClusterParametersPollInterval = 5 * time.Second
	extractVMLogsPollInterval            = 5 * time.Second
	waitUntilPodRunningPollInterval      = 5 * time.Second
	waitUntilNodeReadyPollingInterval    = 5 * time.Second
)

func pollExecOnVM(ctx context.Context, t *testing.T, kube *Kubeclient, vmPrivateIP, jumpboxPodName string, sshPrivateKey, command string, isShellBuiltIn bool) (*podExecResult, error) {
	var execResult *podExecResult
	err := wait.PollUntilContextCancel(ctx, execOnVMPollInterval, true, func(ctx context.Context) (bool, error) {
		res, err := execOnVM(ctx, kube, vmPrivateIP, jumpboxPodName, sshPrivateKey, command, isShellBuiltIn)
		if err != nil {
			t.Logf("unable to execute command on VM: %s", err)

			// fail hard on non-retriable error
			if strings.Contains(err.Error(), "error extracting exit code") {
				return false, err
			}
			return false, nil
		}

		// this denotes a retriable SSH failure
		if res.exitCode == "255" {
			return false, nil
		}

		execResult = res
		return true, nil
	})

	if err != nil {
		return nil, err
	}

	return execResult, nil
}

// Wraps extractClusterParameters in a poller with a 15-second wait interval and 5-minute timeout
func pollExtractClusterParameters(ctx context.Context, t *testing.T, kube *Kubeclient) (map[string]string, error) {
	var clusterParams map[string]string
	err := wait.PollUntilContextCancel(ctx, extractClusterParametersPollInterval, true, func(ctx context.Context) (bool, error) {
		params, err := extractClusterParameters(ctx, t, kube)
		if err != nil {
			t.Logf("error extracting cluster parameters: %s", err)
			return false, nil
		}
		clusterParams = params
		return true, nil
	})

	if err != nil {
		return nil, err
	}

	return clusterParams, nil
}

// Wraps extractLogsFromVM and dumpFileMapToDir in a poller with a 15-second wait interval and 5-minute timeout
func pollExtractVMLogs(ctx context.Context, t *testing.T, vmssName, privateIP string, privateKeyBytes []byte, opts *scenarioRunOpts) error {
	err := wait.PollUntilContextCancel(ctx, extractVMLogsPollInterval, true, func(ctx context.Context) (bool, error) {
		t.Logf("on %s attempting to extract VM logs", vmssName)

		logFiles, err := extractLogsFromVM(ctx, t, vmssName, privateIP, string(privateKeyBytes), opts)
		if err != nil {
			t.Logf("on %s error extracting VM logs: %q", vmssName, err)
			return false, nil
		}

		if err = dumpFileMapToDir(t, logFiles); err != nil {
			t.Logf("on %s error extracting VM logs: %q", vmssName, err)
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return err
	}

	return nil
}

func waitUntilNodeReady(ctx context.Context, t *testing.T, kube *Kubeclient, vmssName string) string {
	var nodeName string
	nodeStatus := corev1.NodeStatus{}
	found := false

	t.Logf("waiting for node %s to be ready", vmssName)

	err := wait.PollUntilContextCancel(ctx, waitUntilNodeReadyPollingInterval, true, func(ctx context.Context) (bool, error) {
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
	return wait.PollUntilContextCancel(ctx, waitUntilPodRunningPollInterval, true, func(ctx context.Context) (bool, error) {
		pod, err := kube.Typed.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
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
