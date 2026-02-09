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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	errorsk8s "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
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

	// it's a test cluster - avoid unnecessary rate limiting
	config.QPS = 200
	config.Burst = 400

	dynamic, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("create dynamic Kubeclient: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes clientset from rest config: %w", err)
	}

	return &Kubeclient{
		Dynamic:    dynamic,
		Typed:      clientset,
		RESTConfig: config,
		KubeConfig: data,
	}, nil
}

func (k *Kubeclient) WaitUntilPodRunningWithRetry(ctx context.Context, namespace string, labelSelector string, fieldSelector string, maxRetries int) (*corev1.Pod, error) {
	var pod *corev1.Pod

	err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
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
					maxRetries--
					sandboxErr := fmt.Errorf("pod %s has FailedCreatePodSandBox event: %s", pod.Name, event.Message)
					if maxRetries <= 0 {
						return false, sandboxErr
					}
					k.Typed.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))})
					return false, nil // Keep polling
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

func (k *Kubeclient) WaitUntilPodRunning(ctx context.Context, namespace string, labelSelector string, fieldSelector string) (*corev1.Pod, error) {
	return k.WaitUntilPodRunningWithRetry(ctx, namespace, labelSelector, fieldSelector, 0)
}

func (k *Kubeclient) WaitUntilNodeReady(ctx context.Context, t testing.TB, vmssName string) string {
	startTime := time.Now()
	t.Logf("waiting for node %s to be ready", vmssName)
	defer func() {
		t.Logf("waited for node %s to be ready for %s", vmssName, time.Since(startTime))
	}()

	var lastNode *corev1.Node
	for ctx.Err() == nil {
		name := func() string {
			watcher, err := k.Typed.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{})
			if err != nil {
				t.Logf("failed to start node watch: %v, retrying in 5s", err)
				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
				}
				return ""
			}
			defer watcher.Stop()

			for event := range watcher.ResultChan() {
				if event.Type == watch.Error {
					t.Logf("node watch error: %v", event.Object)
					return ""
				}
				node, ok := event.Object.(*corev1.Node)
				if !ok || !strings.HasPrefix(node.Name, vmssName) {
					continue
				}
				if event.Type == watch.Deleted {
					t.Fatalf("node %s was deleted", node.Name)
				}
				lastNode = node
				for _, cond := range node.Status.Conditions {
					if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
						t.Logf("node %s is ready", node.Name)
						return node.Name
					}
				}
			}
			return ""
		}()
		if name != "" {
			return name
		}
	}

	if lastNode != nil {
		nodeJSON, _ := json.Marshal(lastNode)
		t.Fatalf("node %s (%s) not ready: %v\n%s", vmssName, lastNode.Name, ctx.Err(), nodeJSON)
	}
	t.Fatalf("node %q not found: %v", vmssName, ctx.Err())
	return ""
}

// GetPodNetworkDebugPodForNode returns a pod that's a member of the 'debugnonhost' daemonset running in the cluster - this will return
// the name of the pod that is running on the node created for specifically for the test case which is running validation checks.
func (k *Kubeclient) GetPodNetworkDebugPodForNode(ctx context.Context, kubeNodeName string) (*corev1.Pod, error) {
	if kubeNodeName == "" {
		return nil, fmt.Errorf("kubeNodeName must not be empty")
	}
	return k.WaitUntilPodRunningWithRetry(ctx, defaultNamespace, fmt.Sprintf("app=%s", podNetworkDebugAppLabel), "spec.nodeName="+kubeNodeName, 3)
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
	if s.Tags.MockAzureChinaCloud {
		image = "mcr.azk8s.cn/cbl-mariner/busybox:2.0"
	}
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
