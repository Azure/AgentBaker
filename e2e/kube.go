package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type kubeclient struct {
	dynamic client.Client
	typed   kubernetes.Interface
	rest    *rest.Config
}

func newKubeclient(config *rest.Config) (*kubeclient, error) {
	dynamic, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic kubeclient: %q", err)
	}

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create rest kube client: %q", err)
	}

	typed := kubernetes.New(restClient)

	return &kubeclient{
		dynamic: dynamic,
		typed:   typed,
		rest:    config,
	}, nil
}

func getClusterKubeClient(ctx context.Context, cloud *azureClient, config *suiteConfig) (*kubeclient, error) {
	data, err := getClusterKubeconfigBytes(ctx, cloud, config.resourceGroupName, config.clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster kubeconfig bytes")
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubeconfig bytes to rest config")
	}
	restConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	restConfig.APIPath = "/api"
	restConfig.GroupVersion = &schema.GroupVersion{
		Version: "v1",
	}

	return newKubeclient(restConfig)
}

func getClusterKubeconfigBytes(ctx context.Context, cloud *azureClient, resourceGroupName, clusterName string) ([]byte, error) {
	credentialList, err := cloud.aksClient.ListClusterAdminCredentials(ctx, resourceGroupName, clusterName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster admin credentials: %q", err)
	}

	if len(credentialList.Kubeconfigs) < 1 {
		return nil, fmt.Errorf("no kubeconfigs available for the managed cluster cluster")
	}

	return credentialList.Kubeconfigs[0].Value, nil
}

func waitUntilNodeReady(ctx context.Context, kube *kubeclient, vmssName string) (string, error) {
	var nodeName string
	err := wait.PollImmediateWithContext(ctx, 5*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		nodes, err := kube.typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, node := range nodes.Items {
			if strings.HasPrefix(node.Name, vmssName) {
				for _, cond := range node.Status.Conditions {
					if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
						nodeName = node.Name
						return true, nil
					}
				}
			}
		}

		return false, nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to find or wait for node to be ready: %q", err)
	}

	return nodeName, nil
}
