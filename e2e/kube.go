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
	v1 "k8s.io/api/core/v1"
	errorsk8s "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Kubeclient struct {
	Dynamic    client.Client
	Typed      kubernetes.Interface
	RESTConfig *rest.Config
	KubeConfig []byte
}

const (
	hostNetworkDebugAppLabel = "debug-mariner-tolerated"
	podNetworkDebugAppLabel  = "debugnonhost-mariner-tolerated"
)

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

				// Annoyingly, when a pod sandbox is failed to create, the pod is left in Pending state and no events are sent
				// to the ResultChan from the watcher below.
				// The lack of events means we can't abort in the case statement below. So we have to check here
				// for the FailedCreatePodSandBox event manually.
				events, err := k.Typed.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{FieldSelector: "involvedObject.name=" + pod.Name})
				if err == nil {
					for _, event := range events.Items {
						if event.Reason == "FailedCreatePodSandBox" {
							return nil, fmt.Errorf("pod %s has FailedCreatePodSandBox event: %s", pod.Name, event.Message)
						}
					}
				}
			}
		case event := <-watcher.ResultChan():
			if event.Type != "ADDED" && event.Type != "MODIFIED" {
				if event.Type != "" {
					t.Logf("skipping event %s", event.Type)
				}
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
			case corev1.PodFailed:
				logPodDebugInfo(ctx, k, pod, t)
				return nil, fmt.Errorf("pod %s is has failed", pod.Name)
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
	var node *corev1.Node = nil
	t.Logf("waiting for node %s to be ready", vmssName)

	watcher, err := k.Typed.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{})
	require.NoError(t, err, "failed to start watching nodes")
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		if event.Type != watch.Added && event.Type != watch.Modified {
			continue
		}

		castNode := event.Object.(*corev1.Node)
		if !strings.HasPrefix(castNode.Name, vmssName) {
			continue
		}

		// found the right node. Use it!
		node = castNode
		nodeTaints, _ := json.Marshal(node.Spec.Taints)
		nodeConditions, _ := json.Marshal(node.Status.Conditions)

		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				t.Logf("node %s is ready. Taints: %s Conditions: %s", node.Name, string(nodeTaints), string(nodeConditions))
				return node.Name
			}
		}

		t.Logf("node %s is not ready. Taints: %s Conditions: %s", node.Name, string(nodeTaints), string(nodeConditions))
	}

	if node == nil {
		t.Fatalf("failed to find wait for %q to appear in the API server", vmssName)
		return ""
	}

	nodeString, _ := json.Marshal(node)
	t.Fatalf("failed to wait for %q (%s) to be ready %+v. Detail: %s", vmssName, node.Name, node.Status, string(nodeString))
	return node.Name
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
func (k *Kubeclient) EnsureDebugDaemonsets(ctx context.Context, t *testing.T, isAirgap bool, privateACRName string) error {
	ds := daemonsetDebug(t, hostNetworkDebugAppLabel, "nodepool1", privateACRName, true, isAirgap)
	err := k.CreateDaemonset(ctx, ds)
	if err != nil {
		return err
	}

	nonHostDS := daemonsetDebug(t, podNetworkDebugAppLabel, "nodepool2", privateACRName, false, isAirgap)
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

func (k *Kubeclient) createKubernetesSecret(ctx context.Context, t *testing.T, namespace, secretName, registryName, username, password string) error {
	t.Logf("Creating Kubernetes secret %s in namespace %s", secretName, namespace)
	clientset, err := kubernetes.NewForConfig(k.RESTConfig)
	if err != nil {
		t.Logf("failed to create Kubernetes client: %v", err)
		return err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	dockerConfigJSON := fmt.Sprintf(`{
		"auths": {
			"%s.azurecr.io": {
				"username": "%s",
				"password": "%s",
				"auth": "%s"
			}
		}
	}`, registryName, username, password, auth)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: v1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			v1.DockerConfigJsonKey: []byte(dockerConfigJSON),
		},
	}
	_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if !errorsk8s.IsAlreadyExists(err) {
			t.Logf("failed to create Kubernetes secret: %v", err)
			return err
		}
	}
	t.Logf("Kubernetes secret %s created", secretName)
	return nil
}

func daemonsetDebug(t *testing.T, deploymentName, targetNodeLabel, privateACRName string, isHostNetwork, isAirgap bool) *appsv1.DaemonSet {
	image := "mcr.microsoft.com/cbl-mariner/base/core:2.0"
	secretName := ""
	if isAirgap {
		image = fmt.Sprintf("%s.azurecr.io/cbl-mariner/base/core:2.0", privateACRName)
		secretName = config.Config.ACRSecretName
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
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: secretName,
						},
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
					// Set Tolerations to tolerate the node with test taints "testkey1=value1:NoSchedule,testkey2=value2:NoSchedule".
					// This is to ensure that the pod can be scheduled on the node with the taints.
					// It won't affect other pods running on the same node.
					Tolerations: []corev1.Toleration{
						{
							Key:      "testkey1",
							Operator: corev1.TolerationOpEqual,
							Value:    "value1",
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "testkey2",
							Operator: corev1.TolerationOpEqual,
							Value:    "value2",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}
}

func getClusterSubnetID(ctx context.Context, mcResourceGroupName string) (string, error) {
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
	secretName := ""
	if s.Tags.Airgap {
		image = fmt.Sprintf("%s.azurecr.io/cbl-mariner/busybox:2.0", config.GetPrivateACRName(s.Tags.NonAnonymousACR, s.Location))
		secretName = config.Config.ACRSecretName
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
			// Set Tolerations to tolerate the node with test taints "testkey1=value1:NoSchedule,testkey2=value2:NoSchedule".
			// This is to ensure that the pod can be scheduled on the node with the taints.
			// It won't affect other pods running on the same node.
			Tolerations: []corev1.Toleration{
				{
					Key:      "testkey1",
					Operator: corev1.TolerationOpEqual,
					Value:    "value1",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "testkey2",
					Operator: corev1.TolerationOpEqual,
					Value:    "value2",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.KubeNodeName,
			},
			ImagePullSecrets: []corev1.LocalObjectReference{
				{
					Name: secretName,
				},
			},
		},
	}
}

func podWindows(s *Scenario, podName string, imageName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-test-%s-pod", s.Runtime.KubeNodeName, podName),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  podName,
					Image: imageName,
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.KubeNodeName,
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
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
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
