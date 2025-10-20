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
	"k8s.io/apimachinery/pkg/util/wait"
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

func (k *Kubeclient) WaitUntilPodRunning(ctx context.Context, namespace string, labelSelector string, fieldSelector string) (*corev1.Pod, error) {
	logf(ctx, "waiting for pod %s %s in %q namespace to be ready", labelSelector, fieldSelector, namespace)

	var pod *corev1.Pod

	err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		pods, err := k.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fieldSelector,
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, err
		}

		if len(pods.Items) == 0 {
			return false, nil // Keep polling
		}

		pod = &pods.Items[0]

		// Check for container failure states
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
				logPodDebugInfo(ctx, k, pod)
				return false, fmt.Errorf("pod %s is in CrashLoopBackOff state", pod.Name)
			}
		}

		// Check for FailedCreatePodSandBox events
		events, err := k.Typed.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{FieldSelector: "involvedObject.name=" + pod.Name})
		if err == nil {
			for _, event := range events.Items {
				if event.Reason == "FailedCreatePodSandBox" {
					return false, fmt.Errorf("pod %s has FailedCreatePodSandBox event: %s", pod.Name, event.Message)
				}
			}
		}

		switch pod.Status.Phase {
		case corev1.PodFailed:
			logPodDebugInfo(ctx, k, pod)
			return false, fmt.Errorf("pod %s has failed", pod.Name)
		case corev1.PodPending:
			return false, nil // Keep polling
		case corev1.PodSucceeded:
			return true, nil // Pod completed successfully
		case corev1.PodRunning:
			// Check if the pod is ready
			for _, cond := range pod.Status.Conditions {
				if cond.Type == "Ready" && cond.Status == "True" {
					logf(ctx, "pod %s is ready", pod.Name)
					return true, nil
				}
			}
			return false, nil // Running but not ready yet
		default:
			logPodDebugInfo(ctx, k, pod)
			return false, fmt.Errorf("pod %s is in unexpected phase %s", pod.Name, pod.Status.Phase)
		}
	})

	return pod, err
}

func (k *Kubeclient) WaitUntilNodeReady(ctx context.Context, t testing.TB, vmssName string) string {
	startTime := time.Now()
	t.Logf("waiting for node %s to be ready in k8s API", vmssName)
	defer func() {
		t.Logf("waited for node %s to be ready in k8s API for %s", vmssName, time.Since(startTime))
	}()

	var node *corev1.Node = nil
	watcher, err := k.Typed.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{})
	require.NoError(t, err, "failed to start watching nodes")
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		if event.Type != watch.Added && event.Type != watch.Modified {
			continue
		}

		var nodeFromEvent *corev1.Node
		switch v := event.Object.(type) {
		case *corev1.Node:
			nodeFromEvent = v

		default:
			t.Logf("skipping object type %T", event.Object)
			continue
		}

		if !strings.HasPrefix(nodeFromEvent.Name, vmssName) {
			continue
		}

		// found the right node. Use it!
		node = nodeFromEvent
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
		t.Fatalf("%q haven't appeared in k8s API server", vmssName)
		return ""
	}

	nodeString, _ := json.Marshal(node)
	t.Fatalf("failed to wait for %q (%s) to be ready %+v. Detail: %s", vmssName, node.Name, node.Status, string(nodeString))
	return node.Name
}

// GetHostNetworkDebugPod returns a pod that's a member of the 'debug' daemonset, running on an aks-nodepool node.
func (k *Kubeclient) GetHostNetworkDebugPod(ctx context.Context) (*corev1.Pod, error) {
	return k.WaitUntilPodRunning(ctx, defaultNamespace, fmt.Sprintf("app=%s", hostNetworkDebugAppLabel), "")
}

// GetPodNetworkDebugPodForNode returns a pod that's a member of the 'debugnonhost' daemonset running in the cluster - this will return
// the name of the pod that is running on the node created for specifically for the test case which is running validation checks.
func (k *Kubeclient) GetPodNetworkDebugPodForNode(ctx context.Context, kubeNodeName string) (*corev1.Pod, error) {
	if kubeNodeName == "" {
		return nil, fmt.Errorf("kubeNodeName must not be empty")
	}
	return k.WaitUntilPodRunning(ctx, defaultNamespace, fmt.Sprintf("app=%s", podNetworkDebugAppLabel), "spec.nodeName="+kubeNodeName)
}

func logPodDebugInfo(ctx context.Context, kube *Kubeclient, pod *corev1.Pod) {
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
		logf(ctx, "couldn't debug info: %s", info)
	}
	logf(ctx, string(info))
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
func (k *Kubeclient) EnsureDebugDaemonsets(ctx context.Context, isAirgap bool, privateACRName string) error {
	ds := daemonsetDebug(ctx, hostNetworkDebugAppLabel, "nodepool1", privateACRName, true, isAirgap)
	err := k.CreateDaemonset(ctx, ds)
	if err != nil {
		return err
	}

	nonHostDS := daemonsetDebug(ctx, podNetworkDebugAppLabel, "nodepool2", privateACRName, false, isAirgap)
	err = k.CreateDaemonset(ctx, nonHostDS)
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

func (k *Kubeclient) createKubernetesSecret(ctx context.Context, namespace, secretName, registryName, username, password string) error {
	logf(ctx, "Creating Kubernetes secret %s in namespace %s", secretName, namespace)
	clientset, err := kubernetes.NewForConfig(k.RESTConfig)
	if err != nil {
		logf(ctx, "failed to create Kubernetes client: %v", err)
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
			logf(ctx, "failed to create Kubernetes secret: %v", err)
			return err
		}
	}
	logf(ctx, "Kubernetes secret %s created", secretName)
	return nil
}

func daemonsetDebug(ctx context.Context, deploymentName, targetNodeLabel, privateACRName string, isHostNetwork, isAirgap bool) *appsv1.DaemonSet {
	image := "mcr.microsoft.com/cbl-mariner/base/core:2.0"
	secretName := ""
	if isAirgap {
		image = fmt.Sprintf("%s.azurecr.io/cbl-mariner/base/core:2.0", privateACRName)
		secretName = config.Config.ACRSecretName
	}
	logf(ctx, "Creating daemonset %s with image %s", deploymentName, image)

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
					ImagePullSecrets: func() []corev1.LocalObjectReference {
						if secretName == "" {
							return nil
						}
						return []corev1.LocalObjectReference{
							{
								Name: secretName,
							},
						}
					}(),
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
						{
							Key:      "node.cloudprovider.kubernetes.io/uninitialized",
							Operator: corev1.TolerationOpExists,
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
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-test-pod", s.Runtime.VM.KubeName),
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
				"kubernetes.io/hostname": s.Runtime.VM.KubeName,
			},
		},
	}
}

func podWindows(s *Scenario, podName string, imageName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-test-%s-pod", s.Runtime.VM.KubeName, podName),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            podName,
					Image:           imageName,
					ImagePullPolicy: "IfNotPresent",
					// this should exist on both servercore and nanoserve
					Command: []string{"cmd", "/c", "ping", "-t", "localhost"},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.VM.KubeName,
			},
		},
	}
}

func podRunNvidiaWorkload(s *Scenario) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-gpu-validation", s.Runtime.VM.KubeName),
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
