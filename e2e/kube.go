package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/Azure/agentbakere2e/config"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

type Kubeclient struct {
	Dynamic    client.Client
	Typed      kubernetes.Interface
	Rest       *rest.Config
	KubeConfig []byte
}

func (k *Kubeclient) clientCertificate() string {
	var kc map[string]any
	if err := yaml.Unmarshal(k.KubeConfig, &kc); err != nil {
		return ""
	}
	encoded := kc["users"].([]interface{})[0].(map[string]any)["user"].(map[string]any)["client-certificate-data"].(string)
	cert, _ := base64.URLEncoding.DecodeString(encoded)
	return string(cert)
}

func getClusterKubeClient(ctx context.Context, resourceGroupName, clusterName string) (*Kubeclient, error) {
	data, err := getClusterKubeconfigBytes(ctx, resourceGroupName, clusterName)
	if err != nil {
		return nil, fmt.Errorf("get cluster kubeconfig bytes: %w", err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, fmt.Errorf("convert kubeconfig bytes to rest config: %w", err)
	}
	restConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	restConfig.APIPath = "/api"
	restConfig.GroupVersion = &schema.GroupVersion{
		Version: "v1",
	}

	dynamic, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("create dynamic Kubeclient: %w", err)
	}

	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create rest kube client: %w", err)
	}

	typed := kubernetes.New(restClient)

	return &Kubeclient{
		Dynamic:    dynamic,
		Typed:      typed,
		Rest:       restConfig,
		KubeConfig: data,
	}, nil
}

func getClusterKubeconfigBytes(ctx context.Context, resourceGroupName, clusterName string) ([]byte, error) {
	credentialList, err := config.Azure.AKS.ListClusterAdminCredentials(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return nil, fmt.Errorf("list cluster admin credentials: %w", err)
	}

	if len(credentialList.Kubeconfigs) < 1 {
		return nil, fmt.Errorf("no kubeconfigs available for the managed cluster cluster")
	}

	return credentialList.Kubeconfigs[0].Value, nil
}

// this is a bit ugly, but we don't want to execute this piece concurrently with other tests
func ensureDebugDaemonsets(ctx context.Context, t *testing.T, kube *Kubeclient, isAirgap bool) error {
	// airgap set to false since acr does not exist during cluster creation
	hostDS := getDebugDaemonsetTemplate(t, hostNetworkDebugAppLabel, "nodepool1", true, isAirgap)
	if err := createDebugDaemonset(ctx, kube, hostDS); err != nil {
		return err
	}
	nonHostDS := getDebugDaemonsetTemplate(t, podNetworkDebugAppLabel, "nodepool2", false, isAirgap)
	if err := createDebugDaemonset(ctx, kube, nonHostDS); err != nil {
		return err
	}
	return nil
}

func getDebugDaemonsetTemplate(t *testing.T, deploymentName, targetNodeLabel string, isHostNetwork, isAirgap bool) string {
	image := "mcr.microsoft.com/cbl-mariner/base/core:2.0"
	if isAirgap {
		image = fmt.Sprintf("%s.azurecr.io/aks/cbl-mariner/base/core:2.0", config.PrivateACRName)
	}
	t.Logf("using image %s for debug daemonset", image)

	return fmt.Sprintf(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: &name %[1]s 
  namespace: default
  labels:
    app: *name
spec:
  replicas: 1
  selector:
    matchLabels:
      app: *name
  template:
    metadata:
      labels:
        app: *name
    spec:
      hostNetwork: %[2]t 
      nodeSelector:
        kubernetes.azure.com/agentpool: %[3]s 
      hostPID: true
      containers:
      - image: %[4]s
        name: mariner
        command: ["sleep", "infinity"]
        resources:
          requests: {}
          limits: {}
        securityContext:
          privileged: true
          capabilities:
            add: ["SYS_PTRACE", "SYS_RAWIO"]
`, deploymentName, isHostNetwork, targetNodeLabel, image)
}

func createDebugDaemonset(ctx context.Context, kube *Kubeclient, manifest string) error {
	var ds v1.DaemonSet

	if err := yaml.Unmarshal([]byte(manifest), &ds); err != nil {
		return fmt.Errorf("failed to unmarshal debug daemonset manifest: %w", err)
	}

	desired := ds.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, kube.Dynamic, &ds, func() error {
		ds = *desired
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to apply debug daemonset: %w for manifest %s", err, manifest)
	}
	return nil
}

func getClusterSubnetID(ctx context.Context, mcResourceGroupName string, t *testing.T) (string, error) {
	pager := config.Azure.VNet.NewListPager(mcResourceGroupName, nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("advance page: %w", err)
		}
		for _, v := range nextResult.Value {
			if v == nil {
				return "", fmt.Errorf("aks vnet was empty")
			}
			return fmt.Sprintf("%s/subnets/%s", *v.ID, "aks-subnet"), nil
		}
	}
	return "", fmt.Errorf("failed to find aks vnet")
}

func getHTTPServerTemplate(podName, nodeName string, isAirgap bool) string {
	image := "mcr.microsoft.com/cbl-mariner/busybox:2.0"
	if isAirgap {
		image = fmt.Sprintf("%s.azurecr.io/aks/cbl-mariner/busybox:2.0", config.PrivateACRName)
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
spec:
  containers:
  - name: mariner
    image: %s
    imagePullPolicy: IfNotPresent
    command: ["sh", "-c"]
    args:
    - |
      mkdir -p /www &&
      echo '<!DOCTYPE html><html><head><title></title></head><body></body></html>' > /www/index.html &&
      httpd -f -p 80 -h /www
    ports:
    - containerPort: 80
  nodeSelector:
    kubernetes.io/hostname: %s
  readinessProbe:
      periodSeconds: 1
      httpGet:
        path: /
        port: 80
`, podName, image, nodeName)
}

func getWasmSpinPodTemplate(podName, nodeName string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
spec:
  runtimeClassName: wasmtime-spin
  containers:
  - name: spin-hello
    image: ghcr.io/spinkube/containerd-shim-spin/examples/spin-rust-hello:v0.15.1
    imagePullPolicy: IfNotPresent
    command: ["/"]
    resources: # limit the resources to 128Mi of memory and 100m of CPU
      limits:
        cpu: 100m
        memory: 128Mi
      requests:
        cpu: 100m
        memory: 128Mi
    readinessProbe:
      periodSeconds: 1
      httpGet:
        path: /hello
        port: 80
  nodeSelector:
    kubernetes.io/hostname: %s
`, podName, nodeName)
}
