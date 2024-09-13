package e2e

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

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

func extractLogsFromVM(ctx context.Context, t *testing.T, vmssName, privateIP, sshPrivateKey string, opts *scenarioRunOpts) (map[string]string, error) {
	commandList := map[string]string{
		"/var/log/azure/cluster-provision": "cat /var/log/azure/cluster-provision.log",
		"kubelet":                          "journalctl -u kubelet",
		"/var/log/azure/cluster-provision-cse-output": "cat /var/log/azure/cluster-provision-cse-output.log",
		"sysctl-out": "sysctl -a",
	}

	podName, err := getHostNetworkDebugPodName(ctx, opts.clusterConfig.Kube)
	if err != nil {
		return nil, fmt.Errorf("unable to get debug pod name: %w", err)
	}

	var result = map[string]string{}
	for file, sourceCmd := range commandList {
		t.Logf("executing command on remote VM at %s of VMSS %s: %q", privateIP, vmssName, sourceCmd)

		execResult, err := execOnVM(ctx, opts.clusterConfig.Kube, privateIP, podName, sshPrivateKey, sourceCmd, false)
		if err != nil {
			t.Logf("error executing command on remote VM at %s of VMSS %s: %s", privateIP, vmssName, err)
			return nil, err
		}

		if execResult.stdout != nil {
			out := execResult.stdout.String()
			if out != "" {
				result[file+".stdout.txt"] = out
			}

		}
		if execResult.stderr != nil {
			out := execResult.stderr.String()
			if out != "" {
				result[file+".stderr.txt"] = out
			}
		}
	}
	return result, nil
}

func extractClusterParameters(ctx context.Context, t *testing.T, kube *Kubeclient) (map[string]string, error) {
	commandList := map[string]string{
		"/etc/kubernetes/azure.json":            "cat /etc/kubernetes/azure.json",
		"/etc/kubernetes/certs/ca.crt":          "cat /etc/kubernetes/certs/ca.crt",
		"/var/lib/kubelet/bootstrap-kubeconfig": "cat /var/lib/kubelet/bootstrap-kubeconfig",
	}

	podName, err := getHostNetworkDebugPodName(ctx, kube)
	if err != nil {
		return nil, err
	}

	var result = map[string]string{}
	for file, sourceCmd := range commandList {
		t.Logf("executing privileged command on pod %s/%s: %q", defaultNamespace, podName, sourceCmd)

		execResult, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, podName, sourceCmd)
		if execResult != nil {
			execResult.dumpStderr(t)
		}
		if err != nil {
			return nil, err
		}

		result[file] = execResult.stdout.String()
	}

	return result, nil
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
