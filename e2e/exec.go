package e2e

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	sshCommandTemplate = `echo '%s' > sshkey%[2]s && chmod 0600 sshkey%[2]s && ssh -i sshkey%[2]s -o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5 azureuser@%s`
)

type podExecResult struct {
	exitCode       string
	stderr, stdout *bytes.Buffer
}

func (r podExecResult) dumpAll(t *testing.T) {
	r.dumpStdout(t)
	r.dumpStderr(t)
}

func (r podExecResult) dumpStdout(t *testing.T) {
	if r.stdout != nil {
		stdoutContent := r.stdout.String()
		if stdoutContent != "" && stdoutContent != "<nil>" {
			t.Logf("%s\n%s\n%s\n%s",
				"dumping stdout:",
				"----------------------------------- begin stdout -----------------------------------",
				stdoutContent,
				"------------------------------------ end stdout ------------------------------------")
		}
	}
}

func (r podExecResult) dumpStderr(t *testing.T) {
	if r.stderr != nil {
		stderrContent := r.stderr.String()
		if stderrContent != "" && stderrContent != "<nil>" {
			t.Logf("%s\n%s\n%s\n%s",
				"dumping stderr:",
				"----------------------------------- begin stderr -----------------------------------",
				stderrContent,
				"------------------------------------ end stderr ------------------------------------")
		}

	}
}

type ClusterParams struct {
	CACert         []byte
	BootstrapToken string
	FQDN           string
	APIServerCert  []byte
	ClientKey      []byte
}

func extractClusterParameters(ctx context.Context, t *testing.T, kube *Kubeclient) *ClusterParams {
	podName, err := getHostNetworkDebugPodName(ctx, kube, t)
	require.NoError(t, err)

	execResult, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, podName, "cat /var/lib/kubelet/bootstrap-kubeconfig")
	require.NoError(t, err)

	bootstrapConfig := execResult.stdout.Bytes()
	bootstrapToken, err := extractKeyValuePair("token", string(bootstrapConfig))
	require.NoError(t, err)

	bootstrapToken, err = strconv.Unquote(bootstrapToken)
	require.NoError(t, err)

	server, err := extractKeyValuePair("server", string(bootstrapConfig))
	require.NoError(t, err)
	tokens := strings.Split(server, ":")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens from fqdn %q, got %d", server, len(tokens))
	}
	fqdn := tokens[1][2:]

	caCert, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, podName, "cat /etc/kubernetes/certs/ca.crt")
	require.NoError(t, err)

	cmdAPIServer, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, podName, "cat /etc/kubernetes/certs/apiserver.crt")
	require.NoError(t, err)

	clientKey, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, podName, "cat /etc/kubernetes/certs/client.key")

	return &ClusterParams{
		CACert:         caCert.stdout.Bytes(),
		BootstrapToken: bootstrapToken,
		FQDN:           fqdn,
		APIServerCert:  cmdAPIServer.stdout.Bytes(),
		ClientKey:      clientKey.stdout.Bytes(),
	}
}

func execOnVM(ctx context.Context, kube *Kubeclient, vmPrivateIP, jumpboxPodName, sshPrivateKey, command string, isShellBuiltIn bool) (*podExecResult, error) {
	sshCommand := fmt.Sprintf(sshCommandTemplate, sshPrivateKey, strings.ReplaceAll(vmPrivateIP, ".", ""), vmPrivateIP)
	if !isShellBuiltIn {
		sshCommand = sshCommand + " sudo"
	}
	commandToExecute := fmt.Sprintf("%s %s", sshCommand, command)

	execResult, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, jumpboxPodName, commandToExecute)
	if err != nil {
		return nil, fmt.Errorf("error executing command on pod: %w", err)
	}

	return execResult, nil
}

func execOnPrivilegedPod(ctx context.Context, kube *Kubeclient, namespace, podName string, command string) (*podExecResult, error) {
	privilegedCommand := append(privelegedCommandArray(), command)
	return execOnPod(ctx, kube, namespace, podName, privilegedCommand)
}

func execOnUnprivilegedPod(ctx context.Context, kube *Kubeclient, namespace, podName, command string) (*podExecResult, error) {
	nonPrivilegedCommand := append(unprivilegedCommandArray(), command)
	return execOnPod(ctx, kube, namespace, podName, nonPrivilegedCommand)
}

func execOnPod(ctx context.Context, kube *Kubeclient, namespace, podName string, command []string) (*podExecResult, error) {
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

	exec, err := remotecommand.NewSPDYExecutor(kube.Rest, "POST", req.URL())
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

func privelegedCommandArray() []string {
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
