package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	// it's test cluster avoid unnecessary rate limiting
	restConfig.QPS = 100
	restConfig.Burst = 200

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
	ds := daemonsetDebug(hostNetworkDebugAppLabel, "nodepool1", true, isAirgap)
	err := createDaemonset(ctx, kube, ds)
	if err != nil {
		return err
	}

	nonHostDS := daemonsetDebug(podNetworkDebugAppLabel, "nodepool2", false, isAirgap)
	err = createDaemonset(ctx, kube, nonHostDS)
	if err != nil {
		return err
	}
	return nil
}

func createDaemonset(ctx context.Context, kube *Kubeclient, ds *appsv1.DaemonSet) error {
	desired := ds.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, kube.Dynamic, ds, func() error {
		ds = desired
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func daemonsetDebug(deploymentName, targetNodeLabel string, isHostNetwork, isAirgap bool) *appsv1.DaemonSet {
	image := "mcr.microsoft.com/cbl-mariner/base/core:2.0"
	if isAirgap {
		image = fmt.Sprintf("%s.azurecr.io/cbl-mariner/base/core:2.0", config.PrivateACRName)
	}

	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: "default",
			Labels: map[string]string{
				"app": deploymentName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": deploymentName,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork: isHostNetwork,
					NodeSelector: map[string]string{
						"kubernetes.azure.com/agentpool": targetNodeLabel,
					},
					HostPID: true,
					Containers: []corev1.Container{
						{
							Image:   image,
							Name:    "mariner",
							Command: []string{"sleep", "infinity"},
							SecurityContext: &corev1.SecurityContext{
								Privileged: to.Ptr(true),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SYS_PTRACE", "SYS_RAWIO"},
								},
							},
						},
					},
				},
			},
		},
	}
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

func podHTTPServerLinux(s *Scenario) *corev1.Pod {
	image := "mcr.microsoft.com/cbl-mariner/busybox:2.0"
	if s.Tags.Airgap {
		image = fmt.Sprintf("%s.azurecr.io/cbl-mariner/busybox:2.0", config.PrivateACRName)
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-test-pod", s.Runtime.KubeNodeName),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "mariner",
					Image: image,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
					Command: []string{"sh", "-c"},
					Args: []string{
						"mkdir -p /www && echo '<!DOCTYPE html><html><head><title></title></head><body></body></html>' > /www/index.html && httpd -f -p 80 -h /www",
					},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.KubeNodeName,
			},
		},
	}
}

func podHTTPServerWindows(s *Scenario) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-test-pod", s.Runtime.KubeNodeName),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "iis-container",
					Image: "mcr.microsoft.com/windows/servercore/iis",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.KubeNodeName,
			},
			ReadinessGates: []corev1.PodReadinessGate{
				{
					ConditionType: "httpGet",
				},
			},
		},
	}
}

func podWASMSpin(s *Scenario) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-wasm-spin", s.Runtime.KubeNodeName),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.KubeNodeName,
			},
			RuntimeClassName: to.Ptr("wasmtime-spin"),
			Containers: []corev1.Container{
				{
					Name:    "spin-hello",
					Image:   "ghcr.io/spinkube/containerd-shim-spin/examples/spin-rust-hello:v0.15.1",
					Command: []string{"/"},
					ReadinessProbe: &corev1.Probe{
						PeriodSeconds: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/hello",
								Port: intstr.FromInt32(80),
							},
						},
					},
				},
			},
		},
	}
}

func podRunNvidiaWorkload(s *Scenario) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-gpu-validation-pod", s.Runtime.KubeNodeName),
			Namespace: defaultNamespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "gpu-validation-container",
					Image: "mcr.microsoft.com/azuredocs/samples-tf-mnist-demo:gpu",
					Args: []string{
						"--max-steps", "1",
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse("1"),
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func podEnableNvidiaResource(s *Scenario) *corev1.Pod {
	enableNvidiaPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-enable-nvidia-device-plugin", s.Runtime.KubeNodeName),
			Namespace: defaultNamespace,
		},
		Spec: corev1.PodSpec{
			PriorityClassName: "system-node-critical",
			Containers: []corev1.Container{
				{
					Name:  "nvidia-device-plugin-ctr",
					Image: "nvcr.io/nvidia/k8s-device-plugin:v0.15.0",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "device-plugin",
							MountPath: "/var/lib/kubelet/device-plugins",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "device-plugin",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/lib/kubelet/device-plugins",
						},
					},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.KubeNodeName,
			},
		},
	}
	return enableNvidiaPod
}
