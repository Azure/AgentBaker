package e2e

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type podExecResult struct {
	exitCode       string
	stderr, stdout *bytes.Buffer
}

func (r podExecResult) String() string {
	return fmt.Sprintf(`exit code: %s
----------------------------------- begin stderr -----------------------------------
%s
------------------------------------ end stderr ------------------------------------
----------------------------------- begin stdout -----------------------------------,
%s
----------------------------------- end stdout ------------------------------------
`, r.exitCode, r.stderr.String(), r.stdout.String())
}

func sshKeyName(vmPrivateIP string) string {
	return fmt.Sprintf("sshkey%s", strings.ReplaceAll(vmPrivateIP, ".", ""))

}

func sshString(vmPrivateIP string) string {
	return fmt.Sprintf(`ssh -i %[1]s -o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5 azureuser@%[2]s`, sshKeyName(vmPrivateIP), vmPrivateIP)
}

func quoteForBash(command string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(command, "'", "'\"'\"'"))
}

func execScriptOnVm(ctx context.Context, s *Scenario, vmPrivateIP, jumpboxPodName string, script string) (*podExecResult, error) {
	// Assuming uploadSSHKey has been called before this function
	s.T.Helper()
	/*
		This works in a way that doesn't rely on the node having joined the cluster:
		* We create a linux pod on a different node.
		* on that pod, we create a script file containing the script passed into this method.
		* Then we scp the script to the node under test.
		* Then we execute the script using an interpreter (powershell or bash) based on the OS of the node.
	*/
	identifier := uuid.New().String()
	var scriptFileName, remoteScriptFileName, interpreter string

	if s.IsWindows() {
		interpreter = "powershell"
		scriptFileName = fmt.Sprintf("script_file_%s.ps1", identifier)
		remoteScriptFileName = fmt.Sprintf("c:/%s", scriptFileName)
	} else {
		interpreter = "bash"
		scriptFileName = fmt.Sprintf("script_file_%s.sh", identifier)
		remoteScriptFileName = scriptFileName
	}

	steps := []string{
		"set -x",
		fmt.Sprintf("echo %[1]s > %[2]s", quoteForBash(script), scriptFileName),
		fmt.Sprintf("chmod 0755 %s", scriptFileName),
		fmt.Sprintf(`scp -i %[1]s -o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5 %[3]s azureuser@%[2]s:%[4]s`, sshKeyName(vmPrivateIP), vmPrivateIP, scriptFileName, remoteScriptFileName),
		fmt.Sprintf("%s %s %s", sshString(vmPrivateIP), interpreter, remoteScriptFileName),
	}

	joinedSteps := strings.Join(steps, " && ")

	kube := s.Runtime.Cluster.Kube
	execResult, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, jumpboxPodName, joinedSteps)
	if err != nil {
		return nil, fmt.Errorf("error executing command on pod: %w", err)
	}

	return execResult, nil
}

func execOnPrivilegedPod(ctx context.Context, kube *Kubeclient, namespace string, podName string, bashCommand string) (*podExecResult, error) {
	privilegedCommand := append(privilegedCommandArray(), bashCommand)
	return execOnPod(ctx, kube, namespace, podName, privilegedCommand)
}

func execOnUnprivilegedPod(ctx context.Context, kube *Kubeclient, namespace string, podName string, bashCommand string) (*podExecResult, error) {
	nonPrivilegedCommand := append(unprivilegedCommandArray(), bashCommand)
	return execOnPod(ctx, kube, namespace, podName, nonPrivilegedCommand)
}

// isRetryableConnectionError checks if the error is a transient connection issue that should be retried
func isRetryableConnectionError(err error) bool {
	errorMsg := err.Error()
	return strings.Contains(errorMsg, "error dialing backend: EOF") ||
		strings.Contains(errorMsg, "connection refused") ||
		strings.Contains(errorMsg, "dial tcp") ||
		strings.Contains(errorMsg, "i/o timeout") ||
		strings.Contains(errorMsg, "connection reset by peer")
}

func execOnPod(ctx context.Context, kube *Kubeclient, namespace, podName string, command []string) (*podExecResult, error) {
	maxRetries := 3
	retryDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		result, err := attemptExecOnPod(ctx, kube, namespace, podName, command)
		if err == nil {
			return result, nil
		}

		// If it's a retryable connection error and we have retries left, retry
		if isRetryableConnectionError(err) && attempt < maxRetries-1 {
			select {
			case <-time.After(retryDelay):
				// Continue to next attempt
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry attempt %d: %w", attempt+1, ctx.Err())
			}
			continue
		}

		// For non-retryable errors or final attempt, return the error
		return nil, err
	}

	return nil, fmt.Errorf("failed after %d attempts", maxRetries)
}

func attemptExecOnPod(ctx context.Context, kube *Kubeclient, namespace, podName string, command []string) (*podExecResult, error) {
	req := kube.Typed.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")

	option := &corev1.PodExecOptions{
		Command: command,
		Stdout:  true,
		Stderr:  true,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(kube.RESTConfig, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("unable to create new SPDY executor for pod exec: %w", err)
	}

	var (
		stdout, stderr bytes.Buffer
		exitCode       string = "0"
	)

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		if strings.Contains(err.Error(), "command terminated with exit code") {
			code, err := extractExitCode(err.Error())
			if err != nil {
				return nil, fmt.Errorf("error extracing exit code from remote command execution error msg: %w", err)
			}
			exitCode = code
		} else {
			return nil, fmt.Errorf("encountered unexpected error when executing command on pod: %w", err)
		}
	}

	return &podExecResult{
		exitCode: exitCode,
		stdout:   &stdout,
		stderr:   &stderr,
	}, nil
}

func privilegedCommandArray() []string {
	return []string{
		"chroot",
		"/proc/1/root",
		"bash",
		"-c",
	}
}

func unprivilegedCommandArray() []string {
	return []string{
		"bash",
		"-c",
	}
}

func uploadSSHKey(ctx context.Context, s *Scenario) error {
	s.T.Helper()
	steps := []string{
		fmt.Sprintf("echo '%[1]s' > %[2]s", string(SSHKeyPrivate), sshKeyName(s.Runtime.VMPrivateIP)),
		fmt.Sprintf("chmod 0600 %s", sshKeyName(s.Runtime.VMPrivateIP)),
	}
	joinedSteps := strings.Join(steps, " && ")
	kube := s.Runtime.Cluster.Kube
	_, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, s.Runtime.Cluster.DebugPod.Name, joinedSteps)
	if err != nil {
		return fmt.Errorf("error executing command on pod: %w", err)
	}
	if config.Config.KeepVMSS {
		s.T.Logf("VM will be preserved after the test finishes, PLEASE MANUALLY DELETE THE VMSS. Set KEEP_VMSS=false to delete it automatically after the test finishes")
	} else {
		s.T.Logf("VM will be automatically deleted after the test finishes, set KEEP_VMSS=true to preserve it or pause the test with a breakpoint before the test finishes")
	}
	result := "SSH Instructions: (may take a few minutes for the VM to be ready for SSH)\n========================\n"
	// We combine the az aks get credentials in the same line so we don't overwrite the user's kubeconfig.
	result += fmt.Sprintf(`kubectl --kubeconfig <(az aks get-credentials --subscription "%s" --resource-group "%s"  --name "%s" -f -) exec -it %s -- bash -c "chroot /proc/1/root /bin/bash -c '%s'"`, config.Config.SubscriptionID, config.ResourceGroupName(s.Location), *s.Runtime.Cluster.Model.Name, s.Runtime.Cluster.DebugPod.Name, sshString(s.Runtime.VMPrivateIP))
	s.T.Log(result)

	return nil
}
