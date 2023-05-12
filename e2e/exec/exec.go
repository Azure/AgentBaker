package exec

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Azure/agentbakere2e/clients"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	sshCommandTemplate                  = `echo '%s' > sshkey && chmod 0600 sshkey && ssh -i sshkey -o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5 azureuser@%s sudo`
	listVMSSNetworkInterfaceURLTemplate = "https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s/virtualMachines/%d/networkInterfaces?api-version=2018-10-01"
)

func NewRemoteCommandExecutor(ctx context.Context, kube *clients.KubeClient, namespace, debugPodName, vmPrivateIP, sshPrivateKey string) *RemoteCommandExecutor {
	return &RemoteCommandExecutor{
		Ctx:           ctx,
		Kube:          kube,
		Namespace:     namespace,
		DebugPodName:  debugPodName,
		VMPrivateIP:   vmPrivateIP,
		SSHPrivateKey: sshPrivateKey,
	}
}

func (e RemoteCommandExecutor) OnVM(command string) (*ExecResult, error) {
	sshCommand := fmt.Sprintf(sshCommandTemplate, e.SSHPrivateKey, e.VMPrivateIP)
	commandToExecute := fmt.Sprintf("%s %s", sshCommand, command)

	execResult, err := e.OnPrivilegedPod(commandToExecute)
	if err != nil {
		return nil, fmt.Errorf("error executing command on pod: %w", err)
	}

	return execResult, nil
}

func (e RemoteCommandExecutor) OnPrivilegedPod(command string) (*ExecResult, error) {
	return e.OnPod(append(NSEnterCommandArray(), command))
}

func (e RemoteCommandExecutor) OnPod(command []string) (*ExecResult, error) {
	req := e.Kube.Typed.CoreV1().RESTClient().Post().Resource("pods").Name(e.DebugPodName).Namespace(e.Namespace).SubResource("exec")

	option := &corev1.PodExecOptions{
		Command: command,
		Stdout:  true,
		Stderr:  true,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(e.Kube.Rest, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("unable to create new SPDY executor for pod exec: %w", err)
	}

	var (
		stdout, stderr bytes.Buffer
		exitCode       string = "0"
	)

	err = exec.StreamWithContext(e.Ctx, remotecommand.StreamOptions{
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

	return &ExecResult{
		ExitCode: exitCode,
		Stdout:   &stdout,
		Stderr:   &stderr,
	}, nil
}

func (r ExecResult) DumpAll() {
	r.DumpStdout()
	r.DumpStderr()
}

func (r ExecResult) DumpStdout() {
	if r.Stdout != nil {
		stdoutContent := r.Stdout.String()
		if stdoutContent != "" && stdoutContent != "<nil>" {
			log.Printf("%s\n%s\n%s\n%s",
				"dumping stdout:",
				"----------------------------------- begin stdout -----------------------------------",
				stdoutContent,
				"------------------------------------ end stdout ------------------------------------")
		}
	}
}

func (r ExecResult) DumpStderr() {
	if r.Stderr != nil {
		stderrContent := r.Stderr.String()
		if stderrContent != "" && stderrContent != "<nil>" {
			log.Printf("%s\n%s\n%s\n%s",
				"dumping stderr:",
				"----------------------------------- begin stderr -----------------------------------",
				stderrContent,
				"------------------------------------ end stderr ------------------------------------")
		}

	}
}
