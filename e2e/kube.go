package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"golang.org/x/net/http2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	errorsk8s "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
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
	proxyAppLabel            = "e2e-proxy"
	proxyPort                = 8888
	proxyNodePoolLabel       = "kubernetes.azure.com/agentpool"
	proxyNodePoolName        = "nodepool1"
)

func getClusterKubeClient(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (*Kubeclient, error) {
	resourceGroupName := config.ResourceGroupName(*cluster.Location)
	clusterName := *cluster.Name
	data, err := getClusterKubeconfigBytes(ctx, resourceGroupName, clusterName)
	if err != nil {
		return nil, fmt.Errorf("get cluster kubeconfig bytes: %w", err)
	}
	return NewKubeclient(data)
}

// NewKubeclient creates a Kubeclient from raw kubeconfig bytes.
// Each call returns an independent client with its own rate limiter,
// allowing concurrent operations to avoid starving each other.
func NewKubeclient(kubeconfigBytes []byte) (*Kubeclient, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("convert kubeconfig bytes to rest config: %w", err)
	}

	// it's a test cluster - avoid unnecessary rate limiting
	cfg.QPS = 200
	cfg.Burst = 400

	// Defense-in-depth against silent connection wedges (apiserver SPDY proxy
	// hangs, NAT/LB idle timeouts) which manifest as kube exec calls that hang
	// indefinitely. Bound the TCP dial and enable HTTP/2 keep-alive pings so
	// the transport itself surfaces a dead peer as a connection error,
	// triggering retries instead of consuming the caller's timeout budget.
	cfg.Dial = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	cfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if t, ok := rt.(*http.Transport); ok {
			if h2, err := http2.ConfigureTransports(t); err == nil {
				h2.ReadIdleTimeout = 30 * time.Second
				h2.PingTimeout = 15 * time.Second
			}
		}
		return rt
	}

	dynamic, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("create dynamic Kubeclient: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes clientset from rest config: %w", err)
	}

	return &Kubeclient{
		Dynamic:    dynamic,
		Typed:      clientset,
		RESTConfig: cfg,
		KubeConfig: kubeconfigBytes,
	}, nil
}

func (k *Kubeclient) WaitUntilPodRunning(ctx context.Context, namespace string, labelSelector string, fieldSelector string) (*corev1.Pod, error) {
	defer toolkit.LogStepCtxf(ctx, "waiting for pod %s %s in %q namespace", labelSelector, fieldSelector, namespace)()
	var pod *corev1.Pod

	err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 6*time.Minute, true, func(ctx context.Context) (bool, error) {
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
				return false, fmt.Errorf("pod %s is in CrashLoopBackOff state", pod.Name)
			}
		}

		switch pod.Status.Phase {
		case corev1.PodFailed:
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
			return false, fmt.Errorf("pod %s is in unexpected phase %s", pod.Name, pod.Status.Phase)
		}
	})

	// Dump events/logs/container statuses for any failure to wait for the pod to be running,
	// including a Pending pod that never transitioned before the poll's own deadline expired
	// (e.g. a stuck image pull or sandbox creation) which would otherwise surface as a bare
	// "context deadline exceeded" with no diagnostic information.
	if err != nil && pod != nil {
		debugCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		logPodDebugInfo(debugCtx, k, pod)
	}

	return pod, err
}

func (k *Kubeclient) WaitUntilNodeReady(ctx context.Context, t testing.TB, vmssName string) (string, error) {
	defer toolkit.LogStepf(t, "waiting for node %s to be ready", vmssName)()
	var lastNode *corev1.Node

	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		nodes, err := k.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Logf("error listing nodes: %v", err)
			return false, nil
		}

		for i := range nodes.Items {
			node := &nodes.Items[i]
			if !strings.HasPrefix(node.Name, vmssName) {
				continue
			}

			lastNode = node
			nodeTaints, _ := json.Marshal(node.Spec.Taints)
			nodeConditions, _ := json.Marshal(node.Status.Conditions)

			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
					t.Logf("node %s is ready. Taints: %s Conditions: %s", node.Name, string(nodeTaints), string(nodeConditions))
					return true, nil
				}
			}

			t.Logf("node %s is not ready. Taints: %s Conditions: %s", node.Name, string(nodeTaints), string(nodeConditions))
		}

		return false, nil
	})

	if err != nil {
		if lastNode == nil {
			return "", fmt.Errorf("%q did not appear in the Kubernetes API server: %w", vmssName, err)
		}
		nodeString, _ := json.Marshal(lastNode)
		return "", fmt.Errorf("failed to wait for %q (%s) to be ready: %w; status=%+v detail=%s", vmssName, lastNode.Name, err, lastNode.Status, string(nodeString))
	}

	return lastNode.Name, nil
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
		toolkit.Logf(ctx, "couldn't debug info: %s", info)
	}
	toolkit.Log(ctx, string(info))
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
func (k *Kubeclient) EnsureDebugDaemonsets(ctx context.Context, isNetworkIsolated bool, privateACRName string) error {
	ds := daemonsetDebug(ctx, hostNetworkDebugAppLabel, map[string]string{"kubernetes.azure.com/mode": "system"}, privateACRName, true, isNetworkIsolated)
	err := k.CreateDaemonset(ctx, ds)
	if err != nil {
		return err
	}

	nonHostDS := daemonsetDebug(ctx, podNetworkDebugAppLabel, map[string]string{"kubernetes.azure.com/agentpool": "nodepool2"}, privateACRName, false, isNetworkIsolated)
	err = k.CreateDaemonset(ctx, nonHostDS)
	if err != nil {
		return err
	}

	// proxy is not available on network-isolated clusters
	if !isNetworkIsolated {
		if err := k.ensureProxyConfigMap(ctx); err != nil {
			return err
		}
		proxyDS := daemonsetProxy(ctx)
		if err := k.CreateDaemonset(ctx, proxyDS); err != nil {
			return err
		}
	}

	return nil
}

func (k *Kubeclient) CreateDaemonset(ctx context.Context, ds *appsv1.DaemonSet) error {
	desired := ds.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, k.Dynamic, ds, func() error {
		ds.Spec = desired.Spec
		ds.Labels = desired.Labels
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (k *Kubeclient) createKubernetesSecret(ctx context.Context, namespace, secretName, registryName, username, password string) error {
	defer toolkit.LogStepCtxf(ctx, "creating kubernetes secret %s in namespace %s for registry %s", secretName, namespace, registryName)()
	clientset, err := kubernetes.NewForConfig(k.RESTConfig)
	if err != nil {
		return fmt.Errorf("create Kubernetes client: %w", err)
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
			return fmt.Errorf("create Kubernetes secret: %w", err)
		}
	}
	return nil
}

func daemonsetDebug(ctx context.Context, deploymentName string, nodeSelector map[string]string, privateACRName string, isHostNetwork, isNetworkIsolated bool) *appsv1.DaemonSet {
	image := "mcr.microsoft.com/cbl-mariner/base/core:2.0"
	secretName := ""
	if isNetworkIsolated {
		image = fmt.Sprintf("%s.azurecr.io/aks-managed-repository/cbl-mariner/base/core:2.0", privateACRName)
		secretName = config.Config.ACRSecretName
	}
	toolkit.Logf(ctx, "Creating daemonset %s with image %s", deploymentName, image)

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
					HostNetwork:      isHostNetwork,
					NodeSelector:     nodeSelector,
					ImagePullSecrets: getImagePullSecrets(secretName),
					HostPID:          true,
					Containers: []corev1.Container{
						{
							Image:   image,
							Name:    "mariner",
							Command: []string{"sleep", "infinity"},
							SecurityContext: &corev1.SecurityContext{
								Privileged: to.Ptr(true),
							},
						},
					},
					Tolerations: getPodTolerations(),
				},
			},
		},
	}
}

func getImagePullSecrets(secretName string) []corev1.LocalObjectReference {
	if secretName == "" {
		return nil
	}
	return []corev1.LocalObjectReference{
		{
			Name: secretName,
		},
	}
}

func getPodTolerations() []corev1.Toleration {
	// Set Tolerations to tolerate the node with test taints "testkey1=value1:NoSchedule,testkey2=value2:NoSchedule".
	// This is to ensure that the pod can be scheduled on the node with the taints.
	// It won't affect other pods running on the same node.
	return []corev1.Toleration{
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
	}
}

func (k *Kubeclient) ensureProxyConfigMap(ctx context.Context) error {
	// Minimal HTTP forward proxy in Python. Handles both:
	// - CONNECT tunneling for HTTPS (curl uses this when HTTPS_PROXY is set)
	// - Plain HTTP forwarding (curl uses this when http_proxy is set)
	proxyScript := `import socket,threading,select,sys,re

def relay(client, remote):
    sockets = [client, remote]
    try:
        while True:
            readable, _, errored = select.select(sockets, [], sockets, 60)
            if errored or not readable:
                break
            for s in readable:
                data = s.recv(65536)
                if not data:
                    return
                (remote if s is client else client).sendall(data)
    finally:
        remote.close()

def handle_connect(client, host, port):
    try:
        remote = socket.create_connection((host, int(port)), timeout=30)
    except Exception as e:
        client.sendall(f"HTTP/1.1 502 Bad Gateway\r\n\r\n{e}".encode())
        return
    client.sendall(b"HTTP/1.1 200 Connection Established\r\n\r\n")
    relay(client, remote)

def handle_http(client, data, host, port):
    try:
        remote = socket.create_connection((host, int(port)), timeout=30)
    except Exception as e:
        client.sendall(f"HTTP/1.1 502 Bad Gateway\r\n\r\n{e}".encode())
        return
    # rewrite absolute URL to relative for the origin server
    lines = data.split(b"\r\n")
    parts = lines[0].split(b" ", 2)
    if len(parts) == 3:
        url = parts[1].decode()
        m = re.match(r"https?://[^/]+(/.*)$", url)
        if m:
            parts[1] = m.group(1).encode()
            lines[0] = b" ".join(parts)
            data = b"\r\n".join(lines)
    remote.sendall(data)
    relay(client, remote)

def handle(client):
    try:
        data = client.recv(65536)
        if not data:
            return
        line = data.split(b"\r\n")[0]
        parts = line.split(b" ", 2)
        if len(parts) < 2:
            return
        method, target = parts[0], parts[1]
        if method == b"CONNECT":
            hp = target.decode().split(":")
            handle_connect(client, hp[0], hp[1] if len(hp) > 1 else "443")
        else:
            # plain HTTP proxy: target is absolute URL like http://host:port/path
            url = target.decode()
            m = re.match(r"https?://([^/:]+)(?::(\d+))?", url)
            if m:
                handle_http(client, data, m.group(1), m.group(2) or "80")
            else:
                client.sendall(b"HTTP/1.1 400 Bad Request\r\n\r\n")
    finally:
        client.close()

srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(("0.0.0.0", ` + fmt.Sprintf("%d", proxyPort) + `))
srv.listen(128)
sys.stdout.write("proxy listening on port ` + fmt.Sprintf("%d", proxyPort) + `\n")
sys.stdout.flush()
while True:
    c, _ = srv.accept()
    threading.Thread(target=handle, args=(c,), daemon=True).start()
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-proxy-config", Namespace: "default"},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, k.Dynamic, cm, func() error {
		cm.Data = map[string]string{"proxy.py": proxyScript}
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensuring proxy configmap: %w", err)
	}
	return nil
}

func daemonsetProxy(ctx context.Context) *appsv1.DaemonSet {
	image := "mcr.microsoft.com/cbl-mariner/base/python:3"
	toolkit.Logf(ctx, "Creating proxy daemonset %s with image %s", proxyAppLabel, image)

	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{Kind: "DaemonSet", APIVersion: "apps/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyAppLabel,
			Namespace: "default",
			Labels:    map[string]string{"app": proxyAppLabel},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": proxyAppLabel},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": proxyAppLabel}},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					NodeSelector: map[string]string{
						proxyNodePoolLabel: proxyNodePoolName,
					},
					Tolerations: []corev1.Toleration{
						{Operator: corev1.TolerationOpExists},
					},
					Containers: []corev1.Container{{
						Name:    "proxy",
						Image:   image,
						Command: []string{"python3", "/opt/proxy/proxy.py"},
						Ports:   []corev1.ContainerPort{{ContainerPort: int32(proxyPort), HostPort: int32(proxyPort)}},
						// Check whether proxy has started before starting the readiness and liveness probes.
						// Allow up to 60s total (5 + 12×5) for a slow-starting proxy before giving up.
						StartupProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(proxyPort)},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       5,
							FailureThreshold:    12,
						},
						// Gate readiness on the proxy actually accepting TCP connections on :8888.
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(proxyPort)},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       5,
							FailureThreshold:    3,
						},
						// Restart the container if it stops serving on :8888.
						// Only restart after sustained failure (30s delay + 3×10s) to avoid restart loops on transient blips.
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(proxyPort)},
							},
							InitialDelaySeconds: 30,
							PeriodSeconds:       10,
							FailureThreshold:    3,
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "proxy-script", MountPath: "/opt/proxy", ReadOnly: true},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "proxy-script",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "e2e-proxy-config"},
							},
						},
					}},
				},
			},
		},
	}
}

// GetProxyURL returns the proxy URL after verifying the proxy pod and its
// backing node are ready on the cluster's permanent managed system pool.
func (k *Kubeclient) GetProxyURL(ctx context.Context) (string, error) {
	var proxyURL string
	var lastPodStatuses []string
	selfHealDelay := 3 * time.Minute
	start := time.Now()
	selfHealed := false
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		pods, err := k.Typed.CoreV1().Pods(defaultNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=" + proxyAppLabel,
		})
		if err != nil {
			return false, fmt.Errorf("listing proxy pods: %w", err)
		}
		nodes, err := k.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: proxyNodePoolLabel + "=" + proxyNodePoolName,
		})
		if err != nil {
			return false, fmt.Errorf("listing permanent system pool nodes: %w", err)
		}
		readySystemPoolNodes := readyNodeNames(nodes.Items)

		lastPodStatuses = lastPodStatuses[:0]
		for _, pod := range pods.Items {
			if proxyPodIsReady(&pod, readySystemPoolNodes) {
				proxyURL = fmt.Sprintf("http://%s:%d", pod.Status.HostIP, proxyPort)
				return true, nil
			}
			status := formatPodDiagnostics(&pod)
			if _, ok := readySystemPoolNodes[pod.Spec.NodeName]; !ok {
				status += fmt.Sprintf(" node is not a Ready %s=%s node", proxyNodePoolLabel, proxyNodePoolName)
			}
			lastPodStatuses = append(lastPodStatuses, status)
		}
		if len(pods.Items) == 0 {
			lastPodStatuses = []string{"no proxy pods found"}
		}
		// Self-heal once: if pods exist but none became ready within the grace
		// period, delete them so the DaemonSet reschedules fresh ones
		if !selfHealed && len(pods.Items) > 0 && time.Since(start) >= selfHealDelay {
			selfHealed = true
			if rerr := k.recreateProxyPods(ctx); rerr != nil {
				toolkit.Logf(ctx, "failed to recreate proxy pods after %s: %v", selfHealDelay, rerr)
			} else {
				toolkit.Logf(ctx, "recreated proxy pods after %s without a ready proxy", selfHealDelay)
			}
		}
		return false, nil
	})
	if err != nil {
		k.logProxyTimeoutDiagnostics(ctx, lastPodStatuses)
		return "", fmt.Errorf("waiting for proxy pod to be ready: %w", err)
	}
	return proxyURL, nil
}

func proxyPodIsReady(pod *corev1.Pod, readySystemPoolNodes map[string]struct{}) bool {
	if pod.Status.HostIP == "" {
		return false
	}
	if _, ok := readySystemPoolNodes[pod.Spec.NodeName]; !ok {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func readyNodeNames(nodes []corev1.Node) map[string]struct{} {
	ready := make(map[string]struct{}, len(nodes))
	for i := range nodes {
		node := &nodes[i]
		if node.Labels[proxyNodePoolLabel] != proxyNodePoolName {
			continue
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				ready[node.Name] = struct{}{}
				break
			}
		}
	}
	return ready
}

// recreateProxyPods deletes all proxy pods so the DaemonSet reschedules fresh ones
func (k *Kubeclient) recreateProxyPods(ctx context.Context) error {
	pods, err := k.Typed.CoreV1().Pods(defaultNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=" + proxyAppLabel,
	})
	if err != nil {
		return fmt.Errorf("listing proxy pods for recreate: %w", err)
	}
	for i := range pods.Items {
		name := pods.Items[i].Name
		if err := k.Typed.CoreV1().Pods(defaultNamespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !errorsk8s.IsNotFound(err) {
			return fmt.Errorf("deleting proxy pod %s for recreate: %w", name, err)
		}
	}
	return nil
}

func formatPodDiagnostics(pod *corev1.Pod) string {
	status := fmt.Sprintf("pod=%s node=%s phase=%s", pod.Name, pod.Spec.NodeName, pod.Status.Phase)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			status += fmt.Sprintf(" container=%s waiting(reason=%s, message=%s)", cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
		} else if cs.State.Terminated != nil {
			status += fmt.Sprintf(" container=%s terminated(reason=%s, exitCode=%d)", cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
		} else if cs.State.Running != nil {
			status += fmt.Sprintf(" container=%s running(ready=%v)", cs.Name, cs.Ready)
		}
	}
	for _, c := range pod.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			status += fmt.Sprintf(" condition(%s=%s: %s)", c.Type, c.Status, c.Message)
		}
	}
	return status
}

func (k *Kubeclient) logProxyTimeoutDiagnostics(ctx context.Context, lastPodStatuses []string) {
	toolkit.Logf(ctx, "⚠️  proxy pod readiness timeout — last observed pod statuses:")
	for _, s := range lastPodStatuses {
		toolkit.Logf(ctx, "    %s", s)
	}

	listCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ds, err := k.Typed.AppsV1().DaemonSets(defaultNamespace).Get(listCtx, proxyAppLabel, metav1.GetOptions{})
	if err != nil {
		toolkit.Logf(ctx, "    (failed to get proxy daemonset: %v)", err)
	} else {
		toolkit.Logf(
			ctx,
			"    --- proxy daemonset status: desired=%d current=%d updated=%d ready=%d available=%d unavailable=%d ---",
			ds.Status.DesiredNumberScheduled,
			ds.Status.CurrentNumberScheduled,
			ds.Status.UpdatedNumberScheduled,
			ds.Status.NumberReady,
			ds.Status.NumberAvailable,
			ds.Status.NumberUnavailable,
		)
		for _, condition := range ds.Status.Conditions {
			toolkit.Logf(ctx, "    condition(%s=%s reason=%s message=%s)", condition.Type, condition.Status, condition.Reason, condition.Message)
		}
	}

	events, err := k.Typed.CoreV1().Events(defaultNamespace).List(listCtx, metav1.ListOptions{})
	if err != nil {
		toolkit.Logf(ctx, "    (failed to list proxy events: %v)", err)
	} else {
		eventCount := 0
		for _, event := range events.Items {
			isProxyDaemonSet := event.InvolvedObject.Kind == "DaemonSet" && event.InvolvedObject.Name == proxyAppLabel
			isProxyPod := event.InvolvedObject.Kind == "Pod" && strings.HasPrefix(event.InvolvedObject.Name, proxyAppLabel+"-")
			if !isProxyDaemonSet && !isProxyPod {
				continue
			}
			if eventCount == 0 {
				toolkit.Logf(ctx, "    --- proxy daemonset and pod events ---")
			}
			eventCount++
			toolkit.Logf(
				ctx,
				"    type=%s object=%s/%s reason=%s count=%d message=%s",
				event.Type,
				event.InvolvedObject.Kind,
				event.InvolvedObject.Name,
				event.Reason,
				event.Count,
				event.Message,
			)
		}
		if eventCount == 0 {
			toolkit.Logf(ctx, "    --- no proxy daemonset or pod events found ---")
		}
	}

	// Log ALL nodes with labels and conditions to diagnose scheduling issues
	nodes, err := k.Typed.CoreV1().Nodes().List(listCtx, metav1.ListOptions{})
	if err != nil {
		toolkit.Logf(ctx, "    (failed to list nodes: %v)", err)
		return
	}
	if len(nodes.Items) == 0 {
		toolkit.Logf(ctx, "    ⚠️  no nodes found in cluster")
		return
	}
	toolkit.Logf(ctx, "    --- cluster nodes (%d total) ---", len(nodes.Items))
	for _, node := range nodes.Items {
		// Collect key labels
		labels := ""
		for _, key := range []string{
			"kubernetes.azure.com/agentpool",
			"kubernetes.azure.com/mode",
			"kubernetes.io/os",
			"kubernetes.io/arch",
		} {
			if v, ok := node.Labels[key]; ok {
				labels += fmt.Sprintf(" %s=%s", key, v)
			}
		}
		// Collect conditions
		conditions := ""
		for _, c := range node.Status.Conditions {
			if c.Type == corev1.NodeReady {
				conditions += fmt.Sprintf(" Ready=%s", c.Status)
			} else if c.Status != corev1.ConditionFalse {
				conditions += fmt.Sprintf(" %s=%s(%s)", c.Type, c.Status, c.Message)
			}
		}
		toolkit.Logf(ctx, "    node=%s |%s |%s", node.Name, labels, conditions)
	}
}

func getClusterSubnetID(ctx context.Context, cluster *armcontainerservice.ManagedCluster) (string, error) {
	for _, pool := range cluster.Properties.AgentPoolProfiles {
		if pool.VnetSubnetID != nil && *pool.VnetSubnetID != "" {
			return *pool.VnetSubnetID, nil
		}
	}
	return "", fmt.Errorf("no VnetSubnetID found on any agent pool profile")
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
			Tolerations:  getPodTolerations(),
			NodeSelector: getNodeSelectorForScenario(s),
		},
	}
}

func getNodeSelectorForScenario(s *Scenario) map[string]string {
	return map[string]string{
		"kubernetes.io/hostname": s.Runtime.VM.KubeName,
	}
}

func debugPodWindows(s *Scenario, podName string, imageName string) *corev1.Pod {
	deploymentName := fmt.Sprintf("%s-test-%s-pod", s.Runtime.VM.KubeName, podName)
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
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
			Tolerations:  getPodTolerations(),
			NodeSelector: getNodeSelectorForScenario(s),
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
