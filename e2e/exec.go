package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var freePort int64 = 30000

type podExecResult struct {
	exitCode       string
	stderr, stdout string
}

func (r podExecResult) String() string {
	return fmt.Sprintf(`exit code: %s
----------------------------------- begin stderr -----------------------------------
%s
------------------------------------ end stderr ------------------------------------
----------------------------------- begin stdout -----------------------------------,
%s
----------------------------------- end stdout ------------------------------------
`, r.exitCode, r.stderr, r.stdout)
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

func waitForPort(port string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout(
			"tcp",
			fmt.Sprintf("127.0.0.1:%s", port),
			time.Second,
		)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("port %s not ready", port)
}

func cleanupBastionTunnel(sshClient *ssh.Client) {
	// We have to do this because az network tunnel creates a new detached process for tunnel
	if sshClient != nil {
		_ = sshClient.Close()
	}
}

func runSSHCommand(
	ctx context.Context,
	client *ssh.Client,
	command string,
) (*podExecResult, error) {
	return runSSHCommandWithPrivateKeyFile(ctx, client, command)
}

func copyScriptToRemoteIfRequired(ctx context.Context, client *ssh.Client, command string) (string, error) {
	if !strings.Contains(command, "\n") {
		return command, nil
	}

	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	tmpFileName := fmt.Sprintf("remote-script-%x.sh", randBytes)

	remotePath := filepath.Join("/home/azureuser", tmpFileName)

	scpClient, err := scp.NewClientBySSH(client)
	if err != nil {
		return "", err
	}
	defer scpClient.Close()

	copyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return remotePath, scpClient.Copy(copyCtx,
		strings.NewReader(command),
		remotePath,
		"0755",
		int64(len(command)))
}

func runSSHCommandWithPrivateKeyFile(
	ctx context.Context,
	client *ssh.Client,
	command string,
) (*podExecResult, error) {
	if client == nil {
		return nil, fmt.Errorf("Permission denied: ssh client is nil")
	}
	var err error
	command, err = copyScriptToRemoteIfRequired(ctx, client, command)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	stdout := bufferPool.Get().(*bytes.Buffer)
	stderr := bufferPool.Get().(*bytes.Buffer)
	stdout.Reset()
	stderr.Reset()

	defer bufferPool.Put(stdout)
	defer bufferPool.Put(stderr)
	session.Stdout = stdout
	session.Stderr = stderr

	err = session.Run(command)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else if _, ok := err.(*ssh.ExitMissingError); ok {
			// Bastion closed channel early – ignore
			err = nil
		} else {
			return nil, err // real SSH failure
		}
	}

	return &podExecResult{
		exitCode: strconv.Itoa(exitCode),
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}, nil
}

func execScriptOnVm(ctx context.Context, s *Scenario, vm *ScenarioVM, jumpboxPodName string, script string) (*podExecResult, error) {
	// Assuming uploadSSHKey has been called before this function
	s.T.Helper()
	/*
		This works in a way that doesn't rely on the node having joined the cluster:
		* We create a linux pod on a different node.
		* on that pod, we create a script file containing the script passed into this method.
		* Then we scp the script to the node under test.
		* Then we execute the script using an interpreter (powershell or bash) based on the OS of the node.
	*/

	if s.IsWindows() {
		identifier := uuid.New().String()
		var scriptFileName, remoteScriptFileName, interpreter string
		interpreter = "powershell"
		scriptFileName = fmt.Sprintf("script_file_%s.ps1", identifier)
		remoteScriptFileName = fmt.Sprintf("c:/%s", scriptFileName)
		steps := []string{
			"set -x",
			fmt.Sprintf("echo %[1]s > %[2]s", quoteForBash(script), scriptFileName),
			fmt.Sprintf("chmod 0755 %s", scriptFileName),
			fmt.Sprintf(`scp -i %[1]s -o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5 %[3]s azureuser@%[2]s:%[4]s`, sshKeyName(vm.PrivateIP), vm.PrivateIP, scriptFileName, remoteScriptFileName),
			fmt.Sprintf("%s %s %s", sshString(vm.PrivateIP), interpreter, remoteScriptFileName),
		}

		joinedSteps := strings.Join(steps, " && ")

		kube := s.Runtime.Cluster.Kube
		execResult, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, jumpboxPodName, joinedSteps)
		if err != nil {
			return nil, fmt.Errorf("error executing command on pod: %w", err)
		}

		return execResult, nil
	}

	return runSSHCommand(ctx, vm.SSHClient, script)
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
	return strings.Contains(errorMsg, "error dialing backend") ||
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
		stdout:   stdout.String(),
		stderr:   stderr.String(),
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

func uploadSSHKey(ctx context.Context, s *Scenario, vmPrivateIP string) error {
	s.T.Helper()
	steps := []string{
		fmt.Sprintf("echo '%[1]s' > %[2]s", string(config.VMSSHPrivateKey), sshKeyName(vmPrivateIP)),
		fmt.Sprintf("chmod 0600 %s", sshKeyName(vmPrivateIP)),
	}
	joinedSteps := strings.Join(steps, " && ")
	kube := s.Runtime.Cluster.Kube
	_, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, s.Runtime.Cluster.DebugPodName, joinedSteps)
	if err != nil {
		return fmt.Errorf("error executing command on pod: %w", err)
	}
	if config.Config.KeepVMSS {
		s.T.Logf("VM will be preserved after the test finishes, PLEASE MANUALLY DELETE THE VMSS. Set KEEP_VMSS=false to delete it automatically after the test finishes")
	} else {
		s.T.Logf("VM will be automatically deleted after the test finishes, to preserve it for debugging purposes set KEEP_VMSS=true or pause the test with a breakpoint before the test finishes or failed")
	}
	result := "SSH Instructions: (may take a few minutes for the VM to be ready for SSH)\n========================\n"
	// We combine the az aks get credentials in the same line so we don't overwrite the user's kubeconfig.
	result += fmt.Sprintf(`kubectl --kubeconfig <(az aks get-credentials --subscription "%s" --resource-group "%s"  --name "%s" -f -) exec -it %s -- bash -c "chroot /proc/1/root /bin/bash -c '%s'"`, config.Config.SubscriptionID, config.ResourceGroupName(s.Location), *s.Runtime.Cluster.Model.Name, s.Runtime.Cluster.DebugPodName, sshString(vmPrivateIP))
	s.T.Log(result)

	return nil
}
