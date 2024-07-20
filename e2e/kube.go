package e2e

import (
	"context"
	"fmt"

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
	Dynamic client.Client
	Typed   kubernetes.Interface
	Rest    *rest.Config
}

func newKubeclient(config *rest.Config) (*Kubeclient, error) {
	dynamic, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("create dynamic Kubeclient: %w", err)
	}

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("create rest kube client: %w", err)
	}

	typed := kubernetes.New(restClient)

	return &Kubeclient{
		Dynamic: dynamic,
		Typed:   typed,
		Rest:    config,
	}, nil
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

	return newKubeclient(restConfig)
}

func getClusterKubeconfigBytes(ctx context.Context, resourceGroupName, clusterName string) ([]byte, error) {
	credentialList, err := config.CustomConfig.Azure.AKS.ListClusterAdminCredentials(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return nil, fmt.Errorf("list cluster admin credentials: %w", err)
	}

	if len(credentialList.Kubeconfigs) < 1 {
		return nil, fmt.Errorf("no kubeconfigs available for the managed cluster cluster")
	}

	return credentialList.Kubeconfigs[0].Value, nil
}

// this is a bit ugly, but we don't want to execute this piece concurrently with other tests
func ensureDebugDaemonset(ctx context.Context, kube *Kubeclient) error {
	manifest := getDebugDaemonset()
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
		return fmt.Errorf("failed to apply debug daemonset: %w", err)
	}

	return nil
}

func getDebugDaemonset() string {
	return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: &name debug
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
      hostNetwork: true
      nodeSelector:
        kubernetes.azure.com/agentpool: nodepool1
      hostPID: true
      containers:
      - image: mcr.microsoft.com/oss/nginx/nginx:1.21.6
        name: ubuntu
        command: ["sleep", "infinity"]
        resources:
          requests: {}
          limits: {}
        securityContext:
          privileged: true
          capabilities:
            add: ["SYS_PTRACE", "SYS_RAWIO"]
`
}

func getClusterSubnetID(ctx context.Context, mcResourceGroupName string) (string, error) {
	pager := config.CustomConfig.Azure.VNet.NewListPager(mcResourceGroupName, nil)
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
