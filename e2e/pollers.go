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
	extractClusterParametersPollInterval = 15 * time.Second
	extractVMLogsPollInterval            = 15 * time.Second
	waitUntilPodRunningPollInterval      = 5 * time.Second
	waitUntilPodDeletedPollInterval      = 5 * time.Second

	// Polling timeouts
	execOnVMPollingTimeout                 = 3 * time.Minute
	extractClusterParametersPollingTimeout = 3 * time.Minute
	extractVMLogsPollingTimeout            = 5 * time.Minute
	waitUntilPodRunningPollingTimeout      = 3 * time.Minute
	waitUntilPodDeletedPollingTimeout      = 1 * time.Minute
)

func pollExecOnVM(ctx context.Context, kube *kubeclient, vmPrivateIP, jumpboxPodName string, sshPrivateKey, command string) (*podExecResult, error) {
	var execResult *podExecResult
	err := wait.PollImmediateWithContext(ctx, execOnVMPollInterval, execOnVMPollingTimeout, func(ctx context.Context) (bool, error) {
		res, err := execOnVM(ctx, kube, vmPrivateIP, jumpboxPodName, sshPrivateKey, command)
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

// Wraps extractClusterParameters in a poller with a 15-second wait interval and 5-minute timeout
func pollExtractClusterParameters(ctx context.Context, t *testing.T, kube *kubeclient) (map[string]string, error) {
	var clusterParams map[string]string
	err := wait.PollImmediateWithContext(ctx, extractClusterParametersPollInterval, extractClusterParametersPollingTimeout, func(ctx context.Context) (bool, error) {
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
func pollExtractVMLogs(ctx context.Context, t *testing.T, vmssName string, privateKeyBytes []byte, opts *scenarioRunOpts) error {
	err := wait.PollImmediateWithContext(ctx, extractVMLogsPollingTimeout, extractVMLogsPollingTimeout, func(ctx context.Context) (bool, error) {
		t.Log("attempting to extract VM logs")

		logFiles, err := extractLogsFromVM(ctx, t, vmssName, string(privateKeyBytes), opts)
		if err != nil {
			t.Logf("error extracting VM logs: %q", err)
			return false, nil
		}

		t.Logf("dumping VM logs to local directory: %s", opts.loggingDir)
		if err = dumpFileMapToDir(opts.loggingDir, logFiles); err != nil {
			t.Logf("error extracting VM logs: %q", err)
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return err
	}

	return nil
}

func waitUntilPodRunning(ctx context.Context, kube *kubeclient, podName string) error {
	return wait.PollImmediateWithContext(ctx, waitUntilPodRunningPollingTimeout, waitUntilPodRunningPollingTimeout, func(ctx context.Context) (bool, error) {
		pod, err := kube.typed.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return pod.Status.Phase == corev1.PodPhase("Running"), nil
	})
}

func waitUntilPodDeleted(ctx context.Context, kube *kubeclient, podName string) error {
	return wait.PollImmediateWithContext(ctx, waitUntilPodDeletedPollInterval, waitUntilPodDeletedPollingTimeout, func(ctx context.Context) (bool, error) {
		err := kube.typed.CoreV1().Pods(defaultNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
		return err == nil, err
	})
}
