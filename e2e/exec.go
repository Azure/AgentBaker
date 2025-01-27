package e2e

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type ClusterParams struct {
	CACert         []byte
	BootstrapToken string
	FQDN           string
	APIServerCert  []byte
	ClientKey      []byte
}

func extractClusterParameters(ctx context.Context, t *testing.T, kube *Kubeclient) *ClusterParams {
	pod, err := kube.GetHostNetworkDebugPod(ctx, t)
	require.NoError(t, err)

	execResult, err := execOnPrivilegedPod(ctx, kube, pod.Namespace, pod.Name, "cat /var/lib/kubelet/bootstrap-kubeconfig")
	require.NoError(t, err)

	bootstrapConfig := execResult.stdout.Bytes()

	server, err := extractKeyValuePair("server", string(bootstrapConfig))
	require.NoError(t, err)
	tokens := strings.Split(server, ":")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens from fqdn %q, got %d", server, len(tokens))
	}
	fqdn := tokens[1][2:]

	caCert, err := execOnPrivilegedPod(ctx, kube, pod.Namespace, pod.Name, "cat /etc/kubernetes/certs/ca.crt")
	require.NoError(t, err)

	cmdAPIServer, err := execOnPrivilegedPod(ctx, kube, pod.Namespace, pod.Name, "cat /etc/kubernetes/certs/apiserver.crt")
	require.NoError(t, err)

	clientKey, err := execOnPrivilegedPod(ctx, kube, pod.Namespace, pod.Name, "cat /etc/kubernetes/certs/client.key")
	require.NoError(t, err)

	return &ClusterParams{
		CACert:         caCert.stdout.Bytes(),
		BootstrapToken: getBootstrapToken(ctx, t, kube),
		FQDN:           fqdn,
		APIServerCert:  cmdAPIServer.stdout.Bytes(),
		ClientKey:      clientKey.stdout.Bytes(),
	}
}

func getBootstrapToken(ctx context.Context, t *testing.T, kube *Kubeclient) string {
	secrets, err := kube.Typed.CoreV1().Secrets("kube-system").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	secret := func() *corev1.Secret {
		for _, secret := range secrets.Items {
			if strings.HasPrefix(secret.Name, "bootstrap-token-") {
				return &secret
			}
		}
		t.Fatal("could not find secret with bootstrap-token- prefix")
		return nil
	}()
	id := secret.Data["token-id"]
	token := secret.Data["token-secret"]
	return fmt.Sprintf("%s.%s", id, token)
}

func sshKeyName(vmPrivateIP string) string {
	return fmt.Sprintf("sshkey%s", strings.ReplaceAll(vmPrivateIP, ".", ""))

}

func sshString(vmPrivateIP string) string {
	return fmt.Sprintf(`ssh -i %s -o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5 azureuser@%s`, sshKeyName(vmPrivateIP), vmPrivateIP)
}

func execOnVM(ctx context.Context, kube *Kubeclient, vmPrivateIP, jumpboxPodName, sshPrivateKey, command string) (*podExecResult, error) {
	/*
			This works in an interesting way:
			* We create a linux pod on a different node.
			* On that pod, we add the ssh private key and then run the command on that node using ssh

			This means we can run commands on the node even if they have failed to join to the cluster - as
		    we're not relying on anything on the node under test, except for the ssh server.

			It does mean we get into quoting complexity as we have to quote to run the command on the pod, and quote again to pass the command through ssh.
	*/
	commandToExecute := fmt.Sprintf(`echo '%s' > %[2]s && chmod 0600 %[2]s && %s %s`, sshPrivateKey, sshKeyName(vmPrivateIP), sshString(vmPrivateIP), command)

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

func logSSHInstructions(s *Scenario) {
	result := "SSH Instructions:"
	if !config.Config.KeepVMSS {
		result += fmt.Sprintf(" (VM will be automatically deleted after the test finishes, set KEEP_VMSS=true to preserve it or pause the test with a breakpoint before the test finishes)")
	}
	result += fmt.Sprintf("\n========================\n")
	result += fmt.Sprintf("az account set --subscription %s\n", config.Config.SubscriptionID)
	result += fmt.Sprintf("az aks get-credentials --resource-group %s --name %s --overwrite-existing\n", config.ResourceGroupName, *s.Runtime.Cluster.Model.Name)
	result += fmt.Sprintf(`kubectl exec -it %s -- bash -c "chroot /proc/1/root /bin/bash -c '%s'"`, s.Runtime.DebugHostPod, sshString(s.Runtime.VMPrivateIP))
	s.T.Log(result)
	//runtime.Breakpoint() // uncomment to pause the test
}
