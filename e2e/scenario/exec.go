package scenario

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
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

func cleanupBastionTunnel(sshClient *SSHClient) {
	if sshClient != nil {
		_ = sshClient.Close()
	}
}

func uploadSSHScript(ctx context.Context, client *ssh.Client, command string, isWindows bool) (remoteCommand string, err error) {
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("generate remote script path: %w", err)
	}

	var remotePath, uploadCommand string
	if isWindows {
		remotePath = fmt.Sprintf("c:/script_file_%x.ps1", randBytes)
		remoteCommand = fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", remotePath)
		// Copy raw bytes so PowerShell does not change the script's encoding or line endings.
		uploadCommand = fmt.Sprintf(`powershell -NoProfile -NonInteractive -Command "$ErrorActionPreference='Stop'; $file=[IO.File]::Open('%s',[IO.FileMode]::CreateNew); try { [Console]::OpenStandardInput().CopyTo($file) } finally { $file.Dispose() }"`, remotePath)
	} else {
		remotePath = filepath.Join("/home/azureuser", fmt.Sprintf("remote_script_%x.sh", randBytes))
		remoteCommand = "bash " + remotePath
		uploadCommand = fmt.Sprintf("umask 077; set -C; cat > %s", remotePath)
	}

	copyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stop := context.AfterFunc(copyCtx, func() { _ = client.Close() })
	defer func() {
		stop()
		if copyCtx.Err() != nil {
			err = fmt.Errorf("upload script: %w", copyCtx.Err())
		}
	}()

	var session *ssh.Session
	err = retrySSHSessionOpen(copyCtx, func() error {
		var openErr error
		session, openErr = client.NewSession()
		return openErr
	})
	if err != nil {
		return "", fmt.Errorf("open script upload session: %w", err)
	}
	defer session.Close()
	session.Stdin = strings.NewReader(command)
	var stderr bytes.Buffer
	session.Stderr = &stderr
	if err := session.Run(uploadCommand); err != nil {
		return "", fmt.Errorf("upload script: %w; stderr: %s", err, stderr.String())
	}
	return remoteCommand, nil
}

func runSSHCommand(
	ctx context.Context,
	client *SSHClient,
	command string,
	isWindows bool,
) (result *podExecResult, err error) {
	if client == nil {
		return nil, fmt.Errorf("Permission denied: ssh client is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case client.operations <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-client.operations }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Closing the transport also interrupts channel opens and uploads, which ignore contexts.
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer func() {
		stop()
		if ctx.Err() != nil {
			result = nil
			err = fmt.Errorf("SSH command canceled or timed out: %w", ctx.Err())
		}
	}()
	command, err = uploadSSHScript(ctx, client.Client, command, isWindows)
	if err != nil {
		return nil, err
	}

	var session *ssh.Session
	err = retrySSHSessionOpen(ctx, func() error {
		var openErr error
		session, openErr = client.NewSession()
		return openErr
	})
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	exitCode := 0
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, err
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

func copyFileFromVM(ctx context.Context, s *Scenario, remotePath string, maxBytes int64) (data []byte, err error) {
	if s == nil || s.Runtime == nil || s.Runtime.VM == nil || s.Runtime.VM.SSHClient == nil {
		return nil, fmt.Errorf("cannot copy remote file without an SSH client")
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("maximum remote file size must be positive")
	}

	client := s.Runtime.VM.SSHClient
	select {
	case client.operations <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-client.operations }()

	scpClient, err := scp.NewClientBySSH(client.Client)
	if err != nil {
		return nil, fmt.Errorf("create SCP client: %w", err)
	}
	defer scpClient.Close()

	var buffer bytes.Buffer
	writer := &boundedWriter{writer: &buffer, remaining: maxBytes}
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer func() {
		stop()
		if ctx.Err() != nil {
			data = nil
			err = fmt.Errorf("SCP download canceled or timed out: %w", ctx.Err())
		}
	}()

	if err := scpClient.CopyFromRemotePassThru(context.WithoutCancel(ctx), writer, remotePath, nil); err != nil {
		return nil, fmt.Errorf("copy remote file %q: %w", remotePath, err)
	}
	return buffer.Bytes(), nil
}

type boundedWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("remote file exceeds maximum size")
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

func execScriptOnVMForScenarioValidateExitCode(ctx context.Context, s *Scenario, cmd string, expectedExitCode int, additionalErrorMessage string) (*podExecResult, error) {
	execResult, err := execScriptOnVMForScenario(ctx, s, cmd)
	if err != nil {
		return nil, err
	}

	expectedExitCodeStr := fmt.Sprint(expectedExitCode)
	if expectedExitCodeStr != execResult.exitCode {
		logging.Logf(ctx, "Command: %s\nStdout: %s\nStderr: %s", cmd, execResult.stdout, execResult.stderr)
		return execResult, fmt.Errorf("expected exit code %s, got %s for command %q: %s", expectedExitCodeStr, execResult.exitCode, cmd, additionalErrorMessage)
	}
	return execResult, nil
}

// isRetryableConnectionError checks if the error is a transient connection issue that should be retried
func isRetryableConnectionError(err error) bool {
	errorMsg := err.Error()
	// client-go flattens proxy failures received after HTTP 101 into plain text.
	proxy502 := strings.Contains(errorMsg, "proxy error from ") &&
		strings.Contains(errorMsg, "while dialing ") &&
		strings.Contains(errorMsg, ", code 502:")
	return proxy502 || strings.Contains(errorMsg, "error dialing backend") ||
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
	ctx = klog.NewContext(ctx, klog.FromContext(ctx).WithValues("operation", "pod exec", "namespace", namespace, "pod", podName))
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
