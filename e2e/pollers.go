package e2e_test

import (
	"context"
	"log"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Polling intervals
	execOnVMPollInterval                 = 10 * time.Second
	execOnPodPollInterval                = 10 * time.Second
	extractClusterParametersPollInterval = 15 * time.Second
	extractVMLogsPollInterval            = 15 * time.Second
	getVMPrivateIPAddressPollInterval    = 5 * time.Second
	waitUntilPodRunningPollInterval      = 5 * time.Second
	waitUntilPodDeletedPollInterval      = 5 * time.Second

	// Polling timeouts
	execOnVMPollTimeout                 = 2 * time.Minute
	execOnPodPollTimeout                = 2 * time.Minute
	extractClusterParametersPollTimeout = 3 * time.Minute
	extractVMLogsPollTimeout            = 3 * time.Minute
	getVMPrivateIPAddressPollTimeout    = 1 * time.Minute
	waitUntilPodRunningPollTimeout      = 2 * time.Minute
	waitUntilPodDeletedPollTimeout      = 1 * time.Minute
)

func pollExecOnVM(ctx context.Context, kube *kubeclient, vmPrivateIP, jumpboxPodName string, privateKey, command string) (*podExecResult, error) {
	var execResult *podExecResult
	err := wait.PollImmediateWithContext(ctx, execOnVMPollInterval, execOnVMPollTimeout, func(ctx context.Context) (bool, error) {
		res, err := execOnVM(ctx, kube, vmPrivateIP, jumpboxPodName, privateKey, command)
		if err != nil {
			log.Printf("unable to execute command on VM: %s", err)

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

func pollExecOnPod(ctx context.Context, kube *kubeclient, namespace, podName, command string) (*podExecResult, error) {
	var execResult *podExecResult
	err := wait.PollImmediateWithContext(ctx, execOnPodPollInterval, execOnPodPollTimeout, func(ctx context.Context) (bool, error) {
		res, err := execOnPod(ctx, kube, namespace, podName, append(bashCommandArray(), command))
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

// Wraps extractClusterParameters in a poller with a 15-second wait interval and 5-minute timeout
func pollExtractClusterParameters(ctx context.Context, t *testing.T, kube *kubeclient) (map[string]string, error) {
	var clusterParams map[string]string
	err := wait.PollImmediateWithContext(ctx, extractClusterParametersPollInterval, extractClusterParametersPollTimeout, func(ctx context.Context) (bool, error) {
		params, err := extractClusterParameters(ctx, t, kube)
		if err != nil {
			t.Logf("error extracting cluster parameters: %q", err)
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

// Wraps exctracLogsFromVM and dumpFileMapToDir in a poller with a 15-second wait interval and 5-minute timeout
func pollExtractVMLogs(ctx context.Context, vmssName, privateKey, vmPrivateIP string, opts *scenarioRunOpts) error {
	err := wait.PollImmediateWithContext(ctx, extractVMLogsPollInterval, extractVMLogsPollTimeout, func(ctx context.Context) (bool, error) {
		log.Println("attempting to extract VM logs")

		logFiles, err := extractLogsFromVM(ctx, vmssName, privateKey, vmPrivateIP, opts)
		if err != nil {
			log.Printf("error extracting VM logs: %q", err)
			return false, nil
		}

		log.Printf("dumping VM logs to local directory: %s", opts.loggingDir)
		if err = dumpFileMapToDir(opts.loggingDir, logFiles); err != nil {
			log.Printf("error extracting VM logs: %q", err)
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return err
	}

	return nil
}

func pollGetVMPrivateIP(ctx context.Context, vmssName string, opts *scenarioRunOpts) (string, error) {
	var vmPrivateIP string
	err := wait.PollImmediateWithContext(ctx, getVMPrivateIPAddressPollInterval, getVMPrivateIPAddressPollTimeout, func(ctx context.Context) (bool, error) {
		pip, err := getVMPrivateIPAddress(ctx, opts.cloud, opts.suiteConfig.subscription, *opts.chosenCluster.Properties.NodeResourceGroup, vmssName)
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

func waitUntilPodRunning(ctx context.Context, kube *kubeclient, podName string) error {
	return wait.PollImmediateWithContext(ctx, waitUntilPodRunningPollInterval, waitUntilPodRunningPollTimeout, func(ctx context.Context) (bool, error) {
		pod, err := kube.typed.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return pod.Status.Phase == corev1.PodPhase("Running"), nil
	})
}

func waitUntilPodDeleted(ctx context.Context, kube *kubeclient, podName string) error {
	return wait.PollImmediateWithContext(ctx, waitUntilPodDeletedPollInterval, waitUntilPodDeletedPollTimeout, func(ctx context.Context) (bool, error) {
		err := kube.typed.CoreV1().Pods(defaultNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
		return err == nil, err
	})
}
