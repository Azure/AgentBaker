package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	sshCommandTemplate                  = `echo '%s' > sshkey && chmod 0600 sshkey && ssh -i sshkey -o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5 azureuser@%s sudo`
	listVMSSNetworkInterfaceURLTemplate = "https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s/virtualMachines/%d/networkInterfaces?api-version=2018-10-01"
)

type remoteCommandExecutor struct {
	ctx           context.Context
	kube          *kubeclient
	namespace     string
	debugPodName  string
	vmPrivateIP   string
	sshPrivateKey string
}

func (e remoteCommandExecutor) onVM(command string) (*podExecResult, error) {
	result, err := pollExecOnVM(e.ctx, e.kube, e.debugPodName, e.vmPrivateIP, e.sshPrivateKey, command)
	if err != nil {
		return nil, fmt.Errorf("encountered an error while running vm command: %w", err)
	}
	return result, nil
}

func (e remoteCommandExecutor) onPod(command string) (*podExecResult, error) {
	result, err := pollExecOnPod(e.ctx, e.kube, e.namespace, e.debugPodName, command)
	if err != nil {
		return nil, fmt.Errorf("encountered an error while running pod command: %w", err)
	}
	return result, nil
}

func (e remoteCommandExecutor) onPrivilegedPod(command string) (*podExecResult, error) {
	result, err := pollExecOnPriviledgedPod(e.ctx, e.kube, e.namespace, e.debugPodName, command)
	if err != nil {
		return nil, fmt.Errorf("encountererd an error while running privileged pod command: %w", err)
	}
	return result, nil
}

type podExecResult struct {
	exitCode       string
	stderr, stdout *bytes.Buffer
}

func (r podExecResult) dumpAll() {
	r.dumpStdout()
	r.dumpStderr()
}

func (r podExecResult) dumpStdout() {
	if r.stdout != nil {
		stdoutContent := r.stdout.String()
		if stdoutContent != "" && stdoutContent != "<nil>" {
			log.Printf("%s\n%s\n%s\n%s",
				"dumping stdout:",
				"----------------------------------- begin stdout -----------------------------------",
				stdoutContent,
				"------------------------------------ end stdout ------------------------------------")
		}
	}
}

func (r podExecResult) dumpStderr() {
	if r.stderr != nil {
		stderrContent := r.stderr.String()
		if stderrContent != "" && stderrContent != "<nil>" {
			log.Printf("%s\n%s\n%s\n%s",
				"dumping stderr:",
				"----------------------------------- begin stderr -----------------------------------",
				stderrContent,
				"------------------------------------ end stderr ------------------------------------")
		}

	}
}

func extractLogsFromVM(ctx context.Context, executor remoteCommandExecutor) (map[string]string, error) {
	commandList := map[string]string{
		"/var/log/azure/cluster-provision.log": "cat /var/log/azure/cluster-provision.log",
		"kubelet.log":                          "journalctl -u kubelet",
	}

	var result = map[string]string{}
	for file, sourceCmd := range commandList {
		log.Printf("executing remote VM command: %q", sourceCmd)

		execResult, err := executor.onVM(sourceCmd)
		if err != nil {
			return nil, err
		}

		if execResult.exitCode != "0" {
			execResult.dumpAll()
			return nil, fmt.Errorf("failed to extract VM logs, command terminated with exit code %s", execResult.exitCode)
		}

		result[file] = execResult.stdout.String()
	}
	return result, nil
}

func extractClusterParameters(ctx context.Context, kube *kubeclient) (map[string]string, error) {
	commandList := map[string]string{
		"/etc/kubernetes/azure.json":            "cat /etc/kubernetes/azure.json",
		"/etc/kubernetes/certs/ca.crt":          "cat /etc/kubernetes/certs/ca.crt",
		"/var/lib/kubelet/bootstrap-kubeconfig": "cat /var/lib/kubelet/bootstrap-kubeconfig",
	}

	podName, err := getDebugPodName(kube)
	if err != nil {
		return nil, err
	}

	var result = map[string]string{}
	for file, sourceCmd := range commandList {
		log.Printf("executing privileged command on pod %s/%s: %q", defaultNamespace, podName, sourceCmd)

		execResult, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, podName, sourceCmd)
		if execResult != nil {
			execResult.dumpStderr()
		}
		if err != nil {
			return nil, err
		}

		result[file] = execResult.stdout.String()
	}

	return result, nil
}

func execOnVM(ctx context.Context, kube *kubeclient, vmPrivateIP, jumpboxPodName, sshPrivateKey, command string) (*podExecResult, error) {
	sshCommand := fmt.Sprintf(sshCommandTemplate, sshPrivateKey, vmPrivateIP)
	commandToExecute := fmt.Sprintf("%s %s", sshCommand, command)

	execResult, err := execOnPrivilegedPod(ctx, kube, defaultNamespace, jumpboxPodName, commandToExecute)
	if err != nil {
		return nil, fmt.Errorf("error executing command on pod: %w", err)
	}

	return execResult, nil
}

func execOnPrivilegedPod(ctx context.Context, kube *kubeclient, namespace, podName string, command string) (*podExecResult, error) {
	privilegedCommand := append(nsenterCommandArray(), command)
	return execOnPod(ctx, kube, namespace, podName, privilegedCommand)
}

func execOnPod(ctx context.Context, kube *kubeclient, namespace, podName string, command []string) (*podExecResult, error) {
	req := kube.typed.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")

	option := &corev1.PodExecOptions{
		Command: command,
		Stdout:  true,
		Stderr:  true,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(kube.rest, "POST", req.URL())
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

func curlCommand(url string) string {
	return fmt.Sprintf(`curl \
--connect-timeout 5 \
--max-time 10 \
--retry 10 \
--retry-max-time 100 \
%s`, url)
}

func bashCommandArray() []string {
	return []string{
		"/bin/bash",
		"-c",
	}
}

func nsenterCommandArray() []string {
	return []string{
		"nsenter",
		"-t",
		"1",
		"-m",
		"bash",
		"-c",
	}
}
