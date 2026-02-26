package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"testing"
)

// Test_Ubuntu2204_NvidiaDevicePlugin_Daemonset tests that a GPU node can function correctly
// with the NVIDIA device plugin deployed as a Kubernetes DaemonSet instead of a systemd service.
// This is the "upstream" deployment model commonly used by customers who manage their own
// NVIDIA device plugin deployment.
func Test_Ubuntu2204_NvidiaDevicePlugin_Daemonset(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin works when deployed as a DaemonSet (not systemd service)",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
				nbc.ConfigGPUDriverIfNeeded = true
				// Disable the systemd-based device plugin - we'll deploy it as a DaemonSet instead
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// First, validate that GPU drivers are installed
				ValidateNvidiaModProbeInstalled(ctx, s)

				// Deploy the NVIDIA device plugin as a DaemonSet
				deployNvidiaDevicePluginDaemonset(ctx, s)

				// Wait for the DaemonSet pod to be running on our node
				waitForNvidiaDevicePluginDaemonsetReady(ctx, s)

				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s, 1)

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 1)

				s.T.Logf("NVIDIA device plugin DaemonSet is functioning correctly")
			},
		},
	})
}

// nvidiaDevicePluginDaemonset returns the NVIDIA device plugin DaemonSet spec
// based on the official upstream deployment from:
// https://github.com/NVIDIA/k8s-device-plugin/blob/main/deployments/static/nvidia-device-plugin.yml
func nvidiaDevicePluginDaemonset(nodeName string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
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
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "nvidia-device-plugin-ds",
					},
				},
				Spec: corev1.PodSpec{
					// Target only our specific test node
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": nodeName,
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					PriorityClassName: "system-node-critical",
					Containers: []corev1.Container{
						{
							Name:  "nvidia-device-plugin-ctr",
							Image: "mcr.microsoft.com/oss/v2/nvidia/k8s-device-plugin:v0.18.2",
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

// deployNvidiaDevicePluginDaemonset creates the NVIDIA device plugin DaemonSet in the cluster
func deployNvidiaDevicePluginDaemonset(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("Deploying NVIDIA device plugin as DaemonSet...")

	ds := nvidiaDevicePluginDaemonset(s.Runtime.VM.KubeName)
	err := s.Runtime.Cluster.Kube.CreateDaemonset(ctx, ds)
	require.NoError(s.T, err, "failed to create NVIDIA device plugin DaemonSet")

	s.T.Logf("NVIDIA device plugin DaemonSet created successfully")
}

// waitForNvidiaDevicePluginDaemonsetReady waits for the NVIDIA device plugin pod to be running on the test node
func waitForNvidiaDevicePluginDaemonsetReady(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("Waiting for NVIDIA device plugin DaemonSet pod to be ready on node %s...", s.Runtime.VM.KubeName)

	// Wait for the pod to be running
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		pods, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: "name=nvidia-device-plugin-ds",
			FieldSelector: fmt.Sprintf("spec.nodeName=%s", s.Runtime.VM.KubeName),
		})
		if err != nil {
			return false, err
		}

		if len(pods.Items) == 0 {
			s.T.Logf("No NVIDIA device plugin pod found yet on node %s", s.Runtime.VM.KubeName)
			return false, nil
		}

		pod := &pods.Items[0]
		s.T.Logf("NVIDIA device plugin pod %s is in phase %s", pod.Name, pod.Status.Phase)

		if pod.Status.Phase == corev1.PodRunning {
			// Check if all containers are ready
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if !containerStatus.Ready {
					s.T.Logf("Container %s is not ready yet", containerStatus.Name)
					return false, nil
				}
			}
			return true, nil
		}

		return false, nil
	})

	require.NoError(s.T, err, "timed out waiting for NVIDIA device plugin DaemonSet pod to be ready")
	s.T.Logf("NVIDIA device plugin DaemonSet pod is ready")
}
