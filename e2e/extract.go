package e2e_test

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/agentbakere2e/client"
	"github.com/Azure/agentbakere2e/exec"
)

func extractClusterParameters(ctx context.Context, kube *client.Kube, namespace, podName string) (map[string]string, error) {
	commandList := map[string]string{
		"/etc/kubernetes/azure.json":            "cat /etc/kubernetes/azure.json",
		"/etc/kubernetes/certs/ca.crt":          "cat /etc/kubernetes/certs/ca.crt",
		"/var/lib/kubelet/bootstrap-kubeconfig": "cat /var/lib/kubelet/bootstrap-kubeconfig",
	}

	var result = map[string]string{}
	for file, sourceCmd := range commandList {
		log.Printf("executing command on privileged pod %s/%s: %q", namespace, podName, sourceCmd)

		execResult, err := exec.ExecOnPrivilegedPod(ctx, kube, defaultNamespace, podName, sourceCmd)
		if execResult != nil {
			execResult.DumpStderr()
		}
		if err != nil {
			return nil, err
		}

		result[file] = execResult.Stdout.String()
	}

	return result, nil
}

func extractLogsFromVM(ctx context.Context, executor *exec.RemoteCommandExecutor) (map[string]string, error) {
	commandList := map[string]string{
		"/var/log/azure/cluster-provision.log": "cat /var/log/azure/cluster-provision.log",
		"kubelet.log":                          "journalctl -u kubelet",
	}

	var result = map[string]string{}
	for file, sourceCmd := range commandList {
		log.Printf("executing command on remote VM at %s: %q", executor.VMPrivateIP, sourceCmd)

		execResult, err := executor.OnVM(sourceCmd)
		if err != nil {
			return nil, err
		}

		if execResult.ExitCode != "0" {
			execResult.DumpAll()
			return nil, fmt.Errorf("failed to extract VM logs, command terminated with exit code %s", execResult.ExitCode)
		}

		result[file] = execResult.Stdout.String()
	}
	return result, nil
}
