package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	scp "github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

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
	isWindows bool,
) (*podExecResult, error) {
	return runSSHCommandWithPrivateKeyFile(ctx, client, command, isWindows)
}

func copyScriptToRemoteIfRequired(ctx context.Context, client *ssh.Client, command string, isWindows bool) (string, error) {
	if !strings.Contains(command, "\n") && !isWindows {
		return command, nil
	}

	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("generate remote script path: %w", err)
	}

	var remotePath, remoteCommand string
	if isWindows {
		remotePath = fmt.Sprintf("c:/script_file_%x.ps1", randBytes)
		remoteCommand = fmt.Sprintf("powershell %s", remotePath)
	} else {
		remotePath = filepath.Join("/home/azureuser", fmt.Sprintf("remote_script_%x.sh", randBytes))
		remoteCommand = remotePath
	}

	scpClient, err := scp.NewClientBySSH(client)
	if err != nil {
		return "", err
	}
	defer scpClient.Close()

	copyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return remoteCommand, scpClient.Copy(copyCtx,
		strings.NewReader(command),
		remotePath,
		"0755",
		int64(len(command)))
}

func runSSHCommandWithPrivateKeyFile(
	ctx context.Context,
	client *ssh.Client,
	command string,
	isWindows bool,
) (*podExecResult, error) {
	if client == nil {
		return nil, fmt.Errorf("Permission denied: ssh client is nil")
	}
	var err error
	command, err = copyScriptToRemoteIfRequired(ctx, client, command, isWindows)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	runErr := make(chan error, 1)
	go func() {
		runErr <- session.Run(command)
	}()
	select {
	case err = <-runErr:
	case <-ctx.Done():
		_ = session.Close()
		_ = client.Close()
		select {
		case <-runErr:
		case <-time.After(5 * time.Second):
		}
		return nil, fmt.Errorf("SSH command canceled or timed out: %w", ctx.Err())
	}

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

func execScriptOnVm(ctx context.Context, s *Scenario, vm *ScenarioVM, script string) (*podExecResult, error) {
	if vm == nil {
		return nil, fmt.Errorf("cannot execute script on a nil VM")
	}
	return runSSHCommand(ctx, vm.SSHClient, script, s.IsWindows())
}

func execOnUnprivilegedPod(ctx context.Context, kube *Kubeclient, namespace string, podName string, bashCommand string) (*podExecResult, error) {
	nonPrivilegedCommand := append(unprivilegedCommandArray(), bashCommand)
	return execOnPod(ctx, kube, namespace, podName, nonPrivilegedCommand)
}

func execOnVMForScenarioOnUnprivilegedPod(ctx context.Context, s *Scenario, cmd string) (*podExecResult, error) {
	nonHostPod, err := s.Runtime.Kube.GetPodNetworkDebugPodForNode(ctx, s.Runtime.VM.KubeName)
	if err != nil {
		return nil, fmt.Errorf("get non-host debug pod: %w", err)
	}
	execResult, err := execOnUnprivilegedPod(ctx, s.Runtime.Kube, nonHostPod.Namespace, nonHostPod.Name, cmd)
	if err != nil {
		return nil, fmt.Errorf("execute command %q on unprivileged pod: %w", cmd, err)
	}
	return execResult, nil
}

func execScriptOnVMForScenario(ctx context.Context, s *Scenario, cmd string) (*podExecResult, error) {
	result, err := execScriptOnVm(ctx, s, s.Runtime.VM, cmd)
	if err != nil {
		return nil, fmt.Errorf("execute command %q on VM: %w", cmd, err)
	}
	return result, nil
}

func execScriptOnVMForScenarioValidateExitCode(ctx context.Context, s *Scenario, cmd string, expectedExitCode int, additionalErrorMessage string) (*podExecResult, error) {
	execResult, err := execScriptOnVMForScenario(ctx, s, cmd)
	if err != nil {
		return nil, err
	}

	expectedExitCodeStr := fmt.Sprint(expectedExitCode)
	if expectedExitCodeStr != execResult.exitCode {
		s.Logger.Logf("Command: %s\nStdout: %s\nStderr: %s", cmd, execResult.stdout, execResult.stderr)
		return execResult, fmt.Errorf("expected exit code %s, got %s for command %q: %s", expectedExitCodeStr, execResult.exitCode, cmd, additionalErrorMessage)
	}
	return execResult, nil
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
	req := kube.Typed.CoreV1().RESTClient().Get().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")

	option := &corev1.PodExecOptions{
		Command: command,
		Stdout:  true,
		Stderr:  true,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewWebSocketExecutor(kube.RESTConfig, "GET", req.URL().String())
	if err != nil {
		return nil, fmt.Errorf("unable to create new WebSocket executor for pod exec: %w", err)
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

func unprivilegedCommandArray() []string {
	return []string{
		"bash",
		"-c",
	}
}
