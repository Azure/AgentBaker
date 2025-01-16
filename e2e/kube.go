package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
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
	RESTConfig *rest.Config
	KubeConfig []byte
}

const (
	hostNetworkDebugAppLabel = "debug-mariner"
	podNetworkDebugAppLabel  = "debugnonhost-mariner"
)

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

	config, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, fmt.Errorf("convert kubeconfig bytes to rest config: %w", err)
	}
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	config.APIPath = "/api"
	config.GroupVersion = &schema.GroupVersion{
		Version: "v1",
	}
	// it's test cluster avoid unnecessary rate limiting
	config.QPS = 200
	config.Burst = 400

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
		Dynamic:    dynamic,
		Typed:      typed,
		RESTConfig: config,
		KubeConfig: data,
	}, nil
}

func (k *Kubeclient) WaitUntilPodRunning(ctx context.Context, t *testing.T, namespace string, labelSelector string, fieldSelector string) (*corev1.Pod, error) {
	t.Logf("waiting for pod %s %s in %q namespace to be ready", labelSelector, fieldSelector, namespace)

	watcher, err := k.Typed.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
		LabelSelector: labelSelector,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to start watching pod: %v", err)
	}
	defer watcher.Stop()

	logTicker := time.NewTicker(5 * time.Minute)
	var pod *corev1.Pod
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-logTicker.C:
			if deadline, ok := ctx.Deadline(); ok {
				remaining := time.Until(deadline)
				t.Logf("time before timeout: %v\n", remaining)
			}
			if pod != nil {
				logPodDebugInfo(ctx, k, pod, t)
			}
		case event := <-watcher.ResultChan():
			if event.Type != "ADDED" && event.Type != "MODIFIED" {
				continue
			}
			pod = event.Object.(*corev1.Pod)

			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
					logPodDebugInfo(ctx, k, pod, t)
					return nil, fmt.Errorf("pod %s is in CrashLoopBackOff state", pod.Name)
				}
			}

			switch pod.Status.Phase {
			case corev1.PodPending:
				continue
			case corev1.PodSucceeded:
				//return pod, nil
			case corev1.PodRunning:
				// Check if the pod is ready
				for _, cond := range pod.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == "True" {
						t.Logf("pod %s is ready", pod.Name)
						return pod, nil
					}
				}
			default:
				logPodDebugInfo(ctx, k, pod, t)
				return nil, fmt.Errorf("pod %s is in %s phase", pod.Name, pod.Status.Phase)
			}
		}
	}
}

func (k *Kubeclient) WaitUntilNodeReady(ctx context.Context, t *testing.T, vmssName string) string {
	nodeStatus := corev1.NodeStatus{}
	t.Logf("waiting for node %s to be ready", vmssName)

	watcher, err := k.Typed.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{})
	require.NoError(t, err, "failed to start watching nodes")
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		if event.Type != watch.Added && event.Type != watch.Modified {
			continue
		}
		node := event.Object.(*corev1.Node)

		if !strings.HasPrefix(node.Name, vmssName) {
			continue
		}
		nodeStatus = node.Status
		if len(node.Spec.Taints) > 0 {
			continue
		}

		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				t.Logf("node %s is ready", node.Name)
				return node.Name
			}
		}
	}

	t.Fatalf("failed to find or wait for %q to be ready %+v", vmssName, nodeStatus)
	return ""
}

// GetHostNetworkDebugPod returns a pod that's a member of the 'debug' daemonset, running on an aks-nodepool node.
func (k *Kubeclient) GetHostNetworkDebugPod(ctx context.Context, t *testing.T) (*corev1.Pod, error) {
	return k.WaitUntilPodRunning(ctx, t, defaultNamespace, fmt.Sprintf("app=%s", hostNetworkDebugAppLabel), "")
}

// GetPodNetworkDebugPodForNode returns a pod that's a member of the 'debugnonhost' daemonset running in the cluster - this will return
// the name of the pod that is running on the node created for specifically for the test case which is running validation checks.
func (k *Kubeclient) GetPodNetworkDebugPodForNode(ctx context.Context, kubeNodeName string, t *testing.T) (*corev1.Pod, error) {
	return k.WaitUntilPodRunning(ctx, t, defaultNamespace, fmt.Sprintf("app=%s", podNetworkDebugAppLabel), "spec.nodeName="+kubeNodeName)
}

func logPodDebugInfo(ctx context.Context, kube *Kubeclient, pod *corev1.Pod, t *testing.T) {
	if pod == nil {
		return
	}
	logs, _ := kube.Typed.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{TailLines: to.Ptr(int64(5))}).DoRaw(ctx)
	type Condition struct {
		Reason  string
		Message string
	}
	type Container struct {
		Name  string
		Image string
		Ports []corev1.ContainerPort
	}
	type Event struct {
		Reason        string
		Message       string
		Count         int32
		LastTimestamp metav1.Time
	}
	type Pod struct {
		Name       string
		Namespace  string
		Containers []Container
		Conditions []Condition
		Phase      corev1.PodPhase
		StartTime  *metav1.Time
		Events     []Event
		Logs       string
	}
	var formattedEvents []Event

	events, err := kube.Typed.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{FieldSelector: "involvedObject.name=" + pod.Name})
	if err == nil {
		formattedEvents = make([]Event, 0, len(events.Items))
		for _, event := range events.Items {
			formattedEvents = append(formattedEvents, Event{
				Reason:        event.Reason,
				Message:       event.Message,
				Count:         event.Count,
				LastTimestamp: event.LastTimestamp,
			})
		}
	}

	conditions := make([]Condition, 0)
	for _, cond := range pod.Status.Conditions {
		conditions = append(conditions, Condition{Reason: cond.Reason, Message: cond.Message})
	}

	containers := make([]Container, 0)
	for _, container := range pod.Spec.Containers {
		containers = append(containers, Container{
			Name:  container.Name,
			Image: container.Image,
			Ports: container.Ports,
		})
	}

	info, err := json.MarshalIndent(Pod{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Phase:      pod.Status.Phase,
		StartTime:  pod.Status.StartTime,
		Events:     formattedEvents,
		Containers: containers,
		Logs:       string(logs),
	}, "", "  ")
	if err != nil {
		t.Logf("couldn't debug info: %s", info)
	}
	t.Log(string(info))
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
func (k *Kubeclient) EnsureDebugDaemonsets(ctx context.Context, t *testing.T, isAirgap bool) error {
	ds := daemonsetDebug(t, hostNetworkDebugAppLabel, "nodepool1", true, isAirgap)
	err := k.CreateDaemonset(ctx, ds)
	if err != nil {
		return err
	}

	nonHostDS := daemonsetDebug(t, podNetworkDebugAppLabel, "nodepool2", false, isAirgap)
	err = k.CreateDaemonset(ctx, nonHostDS)
	if err != nil {
		return err
	}

	err = k.CreateDaemonset(ctx, nvidiaDevicePluginDaemonSet())
	if err != nil {
		return err
	}
	return nil
}

func (k *Kubeclient) CreateDaemonset(ctx context.Context, ds *appsv1.DaemonSet) error {
	desired := ds.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, k.Dynamic, ds, func() error {
		ds = desired
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func daemonsetDebug(t *testing.T, deploymentName, targetNodeLabel string, isHostNetwork, isAirgap bool) *appsv1.DaemonSet {
	image := "mcr.microsoft.com/cbl-mariner/base/core:2.0"
	if isAirgap {
		image = fmt.Sprintf("%s.azurecr.io/cbl-mariner/base/core:2.0", config.PrivateACRName)
	}
	t.Logf("Creating daemonset %s with image %s", deploymentName, image)

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
					ReadinessProbe: &corev1.Probe{
						PeriodSeconds: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/",
								Port: intstr.FromInt32(80),
							},
						},
					},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.KubeNodeName,
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
			Name:      fmt.Sprintf("%s-gpu-validation", s.Runtime.KubeNodeName),
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
		},
	}
}

func nvidiaDevicePluginDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nvidia-device-plugin-daemonset",
			Namespace: "kube-system",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "nvidia-device-plugin-ds",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "nvidia-device-plugin-ds",
					},
				},
				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:      "sku",
							Operator: corev1.TolerationOpEqual,
							Value:    "gpu",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					Containers: []corev1.Container{
						{
							Image: "nvcr.io/nvidia/k8s-device-plugin:v0.15.0",
							Name:  "nvidia-device-plugin-ctr",
							Env: []corev1.EnvVar{
								{
									Name:  "FAIL_ON_INIT_ERROR",
									Value: "false",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: to.Ptr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
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
				},
			},
		},
	}
}
