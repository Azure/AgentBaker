package clients

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KubeClient struct {
	Dynamic client.Client
	Typed   kubernetes.Interface
	Rest    *rest.Config
}

func newKubeClient(config *rest.Config) (*KubeClient, error) {
	dynamic, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic kubeclient: %w", err)
	}

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create rest kube client: %w", err)
	}

	typed := kubernetes.New(restClient)

	return &KubeClient{
		Dynamic: dynamic,
		Typed:   typed,
		Rest:    config,
	}, nil
}

func GetClusterKubeClient(ctx context.Context, cloud *AzureClient, resourceGroupName, clusterName string) (*KubeClient, error) {
	data, err := getClusterKubeconfigBytes(ctx, cloud, resourceGroupName, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster kubeconfig bytes: %w", err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubeconfig bytes to rest config: %w", err)
	}
	restConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	restConfig.APIPath = "/api"
	restConfig.GroupVersion = &schema.GroupVersion{
		Version: "v1",
	}

	return newKubeClient(restConfig)
}

func getClusterKubeconfigBytes(ctx context.Context, azureClient *AzureClient, resourceGroupName, clusterName string) ([]byte, error) {
	credentialList, err := azureClient.AKSClient.ListClusterAdminCredentials(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster admin credentials: %w", err)
	}

	if len(credentialList.Kubeconfigs) < 1 {
		return nil, fmt.Errorf("no kubeconfigs available for the managed cluster cluster")
	}

	return credentialList.Kubeconfigs[0].Value, nil
}
