package e2e_test

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/agentbakere2e/clients"
	"github.com/Azure/agentbakere2e/exec"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Polling intervals
	execOnVMPollInterval                 = 10 * time.Second
	execOnPodPollInterval                = 10 * time.Second
	extractClusterParametersPollInterval = 10 * time.Second
	extractVMLogsPollInterval            = 10 * time.Second
	getVMPrivateIPAddressPollInterval    = 5 * time.Second

	// Polling timeouts
	execOnVMPollingTimeout                 = 3 * time.Minute
	execOnPodPollingTimeout                = 2 * time.Minute
	extractClusterParametersPollingTimeout = 3 * time.Minute
	extractVMLogsPollingTimeout            = 5 * time.Minute
	getVMPrivateIPAddressPollingTimeout    = 1 * time.Minute
)

func pollExecOnVM(ctx context.Context, executor *exec.RemoteCommandExecutor, command string) (*exec.ExecResult, error) {
	var execResult *exec.ExecResult
	err := wait.PollImmediateWithContext(ctx, execOnVMPollInterval, execOnVMPollingTimeout, func(ctx context.Context) (bool, error) {
		res, err := executor.OnVM(command)
		if err != nil {
			log.Printf("unable to execute command on VM: %s", err)

			// fail hard on non-retriable error
			if strings.Contains(err.Error(), "error extracting exit code") {
				return false, err
			}
			return false, nil
		}

		// this denotes a retriable SSH failure
		if res.ExitCode == "255" {
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

func pollExecOnPod(ctx context.Context, executor *exec.RemoteCommandExecutor, command []string) (*exec.ExecResult, error) {
	var execResult *exec.ExecResult
	err := wait.PollImmediateWithContext(ctx, execOnPodPollInterval, execOnPodPollingTimeout, func(ctx context.Context) (bool, error) {
		res, err := executor.OnPod(command)
		if err != nil {
			log.Printf("unable to execute command on pod: %s", err)

			// fail hard on non-retriable error
			if strings.Contains(err.Error(), "error extracting exit code") {
				return false, err
			}
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

func pollExecOnPriviledgedPod(ctx context.Context, executor *exec.RemoteCommandExecutor, command string) (*exec.ExecResult, error) {
	var execResult *exec.ExecResult
	err := wait.PollImmediateWithContext(ctx, execOnPodPollInterval, execOnPodPollingTimeout, func(ctx context.Context) (bool, error) {
		res, err := executor.OnPrivilegedPod(command)
		if err != nil {
			log.Printf("unable to execute command on priviledged pod: %s", err)

			// fail hard on non-retriable error
			if strings.Contains(err.Error(), "error extracting exit code") {
				return false, err
			}
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

// Wraps extractClusterParameters in a poller with a 10-second wait interval and 3-minute timeout
func pollExtractClusterParameters(ctx context.Context, executor *exec.RemoteCommandExecutor) (map[string]string, error) {
	var clusterParams map[string]string
	err := wait.PollImmediateWithContext(ctx, extractClusterParametersPollInterval, extractClusterParametersPollingTimeout, func(ctx context.Context) (bool, error) {
		params, err := extractClusterParameters(ctx, executor)
		if err != nil {
			log.Printf("error extracting cluster parameters: %q", err)
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

func pollGetVMPrivateIP(ctx context.Context, vmssName string, opts *runOpts) (string, error) {
	var vmPrivateIP string
	err := wait.PollImmediateWithContext(ctx, getVMPrivateIPAddressPollInterval, getVMPrivateIPAddressPollingTimeout, func(ctx context.Context) (bool, error) {
		pip, err := getVMPrivateIPAddress(ctx, opts.cloud, opts.suiteConfig.subscription, *opts.clusterConfig.cluster.Properties.NodeResourceGroup, vmssName)
		if err != nil {
			log.Printf("encountered an error while getting VM private IP address: %s", err)
			return false, nil
		}
		vmPrivateIP = pip
		return true, nil
	})

	if err != nil {
		return "", err
	}

	return vmPrivateIP, nil
}

func pollGetNodeName(ctx context.Context, kube *clients.KubeClient, vmssName string) (string, error) {
	var nodeName string
	err := wait.PollImmediateWithContext(ctx, 5*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		nodes, err := kube.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, node := range nodes.Items {
			if strings.HasPrefix(node.Name, vmssName) {
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

	if err != nil {
		return "", fmt.Errorf("failed to find or wait for node to be ready: %w", err)
	}

	return nodeName, nil
}
