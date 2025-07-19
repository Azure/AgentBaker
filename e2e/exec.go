package e2e

import (
	"bytes"
	"context"
	"fmt"
	"strings"

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

type Interpreter string

const (
	Powershell Interpreter = "powershell"
	Bash       Interpreter = "bash"
)

type Script struct {
	script      string
	interpreter Interpreter
}

func execScriptOnVm(ctx context.Context, s *Scenario, vmPrivateIP, jumpboxPodName, sshPrivateKey string, script Script) (*podExecResult, error) {
	/*
		This works in a way that doesn't rely on the node having joined the cluster:
		* We create a linux pod on a different node.
		* on that pod, we create a script file containing the script passed into this method.
		* Then we scp the script to the node under test.
		* Then we execute the script using an interpreter (powershell or bash) based on the OS of the node.
	*/
	identifier := uuid.New().String()
	var scriptFileName, remoteScriptFileName, interpreter string

	switch script.interpreter {
	case Powershell:
		interpreter = "powershell"
		scriptFileName = fmt.Sprintf("script_file_%s.ps1", identifier)
		remoteScriptFileName = fmt.Sprintf("c:/%s", scriptFileName)
	default:
		interpreter = "bash"
		scriptFileName = fmt.Sprintf("script_file_%s.sh", identifier)
		remoteScriptFileName = scriptFileName
	}

	steps := []string{
		fmt.Sprintf("echo '%[1]s' > %[2]s", sshPrivateKey, sshKeyName(vmPrivateIP)),
		"set -x",
		fmt.Sprintf("echo %[1]s > %[2]s", quoteForBash(script.script), scriptFileName),
		fmt.Sprintf("chmod 0600 %s", sshKeyName(vmPrivateIP)),
		fmt.Sprintf("chmod 0755 %s", scriptFileName),
		fmt.Sprintf(`scp -i %[1]s -o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5 %[3]s azureuser@%[2]s:%[4]s`, sshKeyName(vmPrivateIP), vmPrivateIP, scriptFileName, remoteScriptFileName),
		fmt.Sprintf("%s %s %s", sshString(vmPrivateIP), interpreter, remoteScriptFileName),
	}

	joinedSteps := strings.Join(steps, " && ")

	s.T.Logf("Executing script %[1]s using %[2]s:\n---START-SCRIPT---\n%[3]s\n---END-SCRIPT---\n", scriptFileName, interpreter, script.script)

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

func logSSHInstructions(s *Scenario) {
	result := "SSH Instructions:"
	if !config.Config.KeepVMSS {
		result += " (VM will be automatically deleted after the test finishes, set KEEP_VMSS=true to preserve it or pause the test with a breakpoint before the test finishes)"
	}
	result += "\n========================\n"
	result += fmt.Sprintf("az account set --subscription %s\n", config.Config.SubscriptionID)
	result += fmt.Sprintf("az aks get-credentials --resource-group %s --name %s --overwrite-existing\n", config.ResourceGroupName(s.Location), *s.Runtime.Cluster.Model.Name)
	result += fmt.Sprintf(`kubectl exec -it %s -- bash -c "chroot /proc/1/root /bin/bash -c '%s'"`, s.Runtime.Cluster.DebugPod.Name, sshString(s.Runtime.VMPrivateIP))
	s.T.Log(result)
	//runtime.Breakpoint() // uncomment to pause the test
}
