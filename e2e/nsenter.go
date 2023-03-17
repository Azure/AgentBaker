package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

const (
	sshCommandTemplate                  = `echo '%s' > sshkey && chmod 0600 sshkey && ssh -i sshkey -o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5 azureuser@%s sudo`
	listVMSSNetworkInterfaceURLTemplate = "https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s/virtualMachines/%d/networkInterfaces?api-version=2018-10-01"
)

func extractLogsFromVM(ctx context.Context, t *testing.T, cloud *azureClient, kube *kubeclient, subscription, vmssName, sshPrivateKey string) (map[string]string, error) {
	pl := cloud.coreClient.Pipeline()
	url := fmt.Sprintf(listVMSSNetworkInterfaceURLTemplate,
		subscription,
		agentbakerTestResourceGroupName,
		vmssName,
		0,
	)
	req, err := runtime.NewRequest(ctx, "GET", url)
	if err != nil {
		return nil, err
	}

	resp, err := pl.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var instanceNICResult listVMSSVMNetworkInterfaceResult

	if err := json.Unmarshal(respBytes, &instanceNICResult); err != nil {
		return nil, err
	}

	privateIP := instanceNICResult.Value[0].Properties.IPConfigurations[0].Properties.PrivateIPAddress

	sshCommand := fmt.Sprintf(sshCommandTemplate, sshPrivateKey, privateIP)

	commandList := map[string]string{
		"/var/log/azure/cluster-provision.log": "cat /var/log/azure/cluster-provision.log",
	}

	podList := corev1.PodList{}
	if err := kube.dynamic.List(context.Background(), &podList, client.MatchingLabels{"app": "debug"}); err != nil {
		return nil, fmt.Errorf("failed to list debug pod: %q", err)
	}

	if len(podList.Items) < 1 {
		return nil, fmt.Errorf("failed to find debug pod, list by selector returned no results")
	}

	podName := podList.Items[0].ObjectMeta.Name

	var result = map[string]string{}
	for file, sourceCmd := range commandList {
		mergedCmd := fmt.Sprintf("%s %s", sshCommand, sourceCmd)
		cmd := append(nsenterCommandArray(), mergedCmd)
		stdout, stderr, err := execOnPod(ctx, kube, defaultNamespace, podName, cmd)
		if err != nil {
			return nil, err
		}

		t.Log("----------------------------------- begin stdout -----------------------------------")
		t.Log(stdout.String())
		t.Log("------------------------------------ end stdout ------------------------------------")

		t.Log("----------------------------------- begin stderr -----------------------------------")
		t.Log(stderr.String())
		t.Log("------------------------------------ end stderr ------------------------------------")

		result[file] = stdout.String()
	}

	return result, nil
}

func extractClusterParameters(ctx context.Context, t *testing.T, kube *kubeclient) (map[string]string, error) {
	commandList := map[string]string{
		"/etc/kubernetes/azure.json":            "cat /etc/kubernetes/azure.json",
		"/etc/kubernetes/certs/ca.crt":          "cat /etc/kubernetes/certs/ca.crt",
		"/var/lib/kubelet/bootstrap-kubeconfig": "cat /var/lib/kubelet/bootstrap-kubeconfig",
	}

	podList := corev1.PodList{}
	if err := kube.dynamic.List(context.Background(), &podList, client.MatchingLabels{"app": "debug"}); err != nil {
		return nil, fmt.Errorf("failed to list debug pod: %q", err)
	}

	if len(podList.Items) < 1 {
		return nil, fmt.Errorf("failed to find debug pod, list by selector returned no results")
	}

	podName := podList.Items[0].ObjectMeta.Name

	var result = map[string]string{}
	for file, sourceCmd := range commandList {
		cmd := append(nsenterCommandArray(), sourceCmd)
		stdout, _, err := execOnPod(ctx, kube, defaultNamespace, podName, cmd)
		if err != nil {
			return nil, err
		}

		result[file] = stdout.String()
	}

	return result, nil
}

func ensureDebugDaemonset(ctx context.Context, kube *kubeclient, resourceGroupName, clusterName string) error {
	manifest := getDebugDaemonset()
	var ds appsv1.DaemonSet

	if err := yaml.Unmarshal([]byte(manifest), &ds); err != nil {
		return fmt.Errorf("failed to unmarshal debug daemonset manifest: %q", err)
	}

	desired := ds.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, kube.dynamic, &ds, func() error {
		ds = *desired
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to apply debug daemonset: %q", err)
	}

	return nil
}

func ensureTestNginxPod(ctx context.Context, kube *kubeclient, nodeName string) error {
	manifest := getNginxPodTemplate(nodeName)
	var obj corev1.Pod

	if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
		return fmt.Errorf("failed to unmarshal nginx pod manifest: %q", err)
	}

	desired := obj.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, kube.dynamic, &obj, func() error {
		obj = *desired
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to apply nginx test pod: %q", err)
	}

	return nil
}

func waitUntilPodRunning(ctx context.Context, kube *kubeclient, podName string) error {
	return wait.PollImmediateUntilWithContext(ctx, 5*time.Second, func(ctx context.Context) (bool, error) {
		pod, err := kube.typed.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return pod.Status.Phase == corev1.PodPhase("Running"), nil
	})
}

func ensurePodDeleted(ctx context.Context, kube *kubeclient, podName string) error {
	return wait.PollImmediateWithContext(ctx, 5*time.Second, 1*time.Minute, func(ctx context.Context) (bool, error) {
		err := kube.typed.CoreV1().Pods(defaultNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
		return err == nil, err
	})
}

func getBaseBootstrappingConfig(ctx context.Context, t *testing.T, cloud *azureClient, suiteConfig *suiteConfig, clusterParams map[string]string) (*datamodel.NodeBootstrappingConfiguration, error) {
	nbc := baseTemplate()
	nbc.ContainerService.Properties.CertificateProfile.CaCertificate = clusterParams["/etc/kubernetes/certs/ca.crt"]

	bootstrapKubeconfig := clusterParams["/var/lib/kubelet/bootstrap-kubeconfig"]

	bootstrapToken, err := extractKeyValuePair("token", bootstrapKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to extract bootstrap token via regex: %q", err)
	}

	bootstrapToken, err = strconv.Unquote(bootstrapToken)
	if err != nil {
		return nil, fmt.Errorf("failed to unquote bootstrap token: %q", err)
	}

	server, err := extractKeyValuePair("server", bootstrapKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to extract fqdn via regex: %q", err)
	}
	tokens := strings.Split(server, ":")
	if len(tokens) != 3 {
		return nil, fmt.Errorf("expected 3 tokens from fqdn %q, got %d", server, len(tokens))
	}
	// strip off the // prefix from https://
	fqdn := tokens[1][2:]

	nbc.KubeletClientTLSBootstrapToken = &bootstrapToken
	nbc.ContainerService.Properties.HostedMasterProfile.FQDN = fqdn

	return nbc, nil
}

func execOnPod(ctx context.Context, kube *kubeclient, namespace, podName string, command []string) (*bytes.Buffer, *bytes.Buffer, error) {
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
		return nil, nil, err
	}

	var stdout, stderr bytes.Buffer

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return nil, nil, err
	}

	return &stdout, &stderr, nil
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
