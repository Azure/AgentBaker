package e2e

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// nvidiaDevicePluginImage is the upstream NVIDIA device plugin image from MCR.
	// This is intentionally different from components.json which tracks the systemd-packaged version.
	// This test validates the upstream container-based deployment model.
	// Update this when a new version is available in MCR.
	nvidiaDevicePluginImage = "mcr.microsoft.com/oss/v2/nvidia/k8s-device-plugin:v0.18.2"
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
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
				nbc.ConfigGPUDriverIfNeeded = true
				// Don't enable the managed GPU experience - we'll deploy the device plugin as a DaemonSet instead.
				// By not setting EnableManagedGPU=true or the VMSS tag, the systemd-based device plugin won't start.
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				// The DaemonSet is only meaningful once the driver is present and the
				// systemd-based plugin is confirmed inactive, so gate the deployment on both.
				if err := errors.Join(
					// First, validate that GPU drivers are installed
					ValidateNvidiaModProbeInstalled(ctx, s),
					// Verify that the systemd-based device plugin is NOT running
					// (managed GPU experience is not enabled, so the service should not be active)
					validateNvidiaDevicePluginServiceNotRunning(ctx, s),
				); err != nil {
					return err
				}

				// Deploy the NVIDIA device plugin as a DaemonSet
				if err := deployNvidiaDevicePluginDaemonset(ctx, s); err != nil {
					return err
				}

				// Wait for the DaemonSet pod to be running on our node
				if err := waitForNvidiaDevicePluginDaemonsetReady(ctx, s); err != nil {
					return err
				}

				// Validate that GPU resources are advertised by the device plugin
				if err := ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu"); err != nil {
					return err
				}

				// Validate that GPU workloads can be scheduled. Only meaningful once the
				// resources above are advertised, otherwise the pod just waits to be scheduled.
				if err := ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu"); err != nil {
					return err
				}

				s.Logger.Logf("NVIDIA device plugin DaemonSet is functioning correctly")
				return nil
			},
		},
	})
}

// validateNvidiaDevicePluginServiceNotRunning verifies that the systemd-based
// NVIDIA device plugin service is not running (since we're testing the DaemonSet model).
func validateNvidiaDevicePluginServiceNotRunning(ctx context.Context, s *Scenario) error {
	s.Logger.Logf("Verifying that nvidia-device-plugin.service is not running...")

	// Check if the service exists and is inactive
	// Using "is-active" which returns non-zero if not active
	result, err := execScriptOnVMForScenario(ctx, s, "systemctl is-active nvidia-device-plugin.service 2>/dev/null || echo 'not-running'")
	if err != nil {
		return fmt.Errorf("check nvidia-device-plugin.service status: %w", err)
	}
	output := strings.TrimSpace(result.stdout)

	// The service should either not exist or be inactive
	if err := assert.NotEqual(output, "active",
		"nvidia-device-plugin.service is unexpectedly running - this test requires the systemd service to be disabled"); err != nil {
		return err
	}
	s.Logger.Logf("Confirmed nvidia-device-plugin.service is not active (status: %s)", output)
	return nil
}

// nvidiaDevicePluginDaemonsetName returns a unique DaemonSet name for the given node.
// The name is truncated to fit within Kubernetes' 63-character limit for resource names.
func nvidiaDevicePluginDaemonsetName(nodeName string) string {
	prefix := "nvdp-" // Short prefix to leave room for node name
	maxLen := 63
	name := prefix + nodeName
	if len(name) > maxLen {
		name = name[:maxLen]
	}
	return name
}

// nvidiaDevicePluginDaemonset returns the NVIDIA device plugin DaemonSet spec
// based on the official upstream deployment from:
// https://github.com/NVIDIA/k8s-device-plugin/blob/main/deployments/static/nvidia-device-plugin.yml
//
// The DaemonSet name includes the node name to avoid collisions when multiple
// GPU tests run against the same shared cluster.
func nvidiaDevicePluginDaemonset(nodeName string) *appsv1.DaemonSet {
	dsName := nvidiaDevicePluginDaemonsetName(nodeName)

	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: "kube-system",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": dsName,
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": dsName,
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
							Image: nvidiaDevicePluginImage,
							Env: []corev1.EnvVar{
								{
									Name:  "FAIL_ON_INIT_ERROR",
									Value: "false",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								// Privileged mode is required for the device plugin to access
								// GPU devices and register with kubelet's device plugin framework.
								// This matches the upstream NVIDIA device plugin deployment spec.
								Privileged: to.Ptr(true),
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
// and registers cleanup to delete it when the test finishes.
func deployNvidiaDevicePluginDaemonset(ctx context.Context, s *Scenario) error {
	s.Logger.Logf("Deploying NVIDIA device plugin as DaemonSet...")

	ds := nvidiaDevicePluginDaemonset(s.Runtime.VM.KubeName)

	// Delete any existing DaemonSet from a previous failed run
	deleteCtx, deleteCancel := context.WithTimeout(ctx, 30*time.Second)
	defer deleteCancel()
	_ = s.Runtime.Kube.Typed.AppsV1().DaemonSets(ds.Namespace).Delete(
		deleteCtx,
		ds.Name,
		metav1.DeleteOptions{},
	)

	// Create the DaemonSet
	if err := s.Runtime.Kube.CreateDaemonset(ctx, ds); err != nil {
		return fmt.Errorf("create NVIDIA device plugin DaemonSet %s/%s: %w", ds.Namespace, ds.Name, err)
	}

	s.Logger.Logf("NVIDIA device plugin DaemonSet %s/%s created successfully", ds.Namespace, ds.Name)

	// Register cleanup to delete the DaemonSet when the test finishes
	s.Cleanup(func(ctx context.Context) error {
		if err := s.Runtime.Kube.Typed.AppsV1().DaemonSets(ds.Namespace).Delete(
			ctx,
			ds.Name,
			metav1.DeleteOptions{},
		); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete NVIDIA device plugin DaemonSet %s/%s: %w", ds.Namespace, ds.Name, err)
		}
		return nil
	})
	return nil
}

// waitForNvidiaDevicePluginDaemonsetReady waits for the NVIDIA device plugin pod to be running on the test node.
// Uses the existing WaitUntilPodRunning helper which handles CrashLoopBackOff and other failure states.
func waitForNvidiaDevicePluginDaemonsetReady(ctx context.Context, s *Scenario) error {
	dsName := nvidiaDevicePluginDaemonsetName(s.Runtime.VM.KubeName)
	s.Logger.Logf("Waiting for NVIDIA device plugin DaemonSet pod to be ready on node %s...", s.Runtime.VM.KubeName)

	if _, err := s.Runtime.Kube.WaitUntilPodRunning(
		ctx,
		"kube-system",
		fmt.Sprintf("name=%s", dsName),
		fmt.Sprintf("spec.nodeName=%s", s.Runtime.VM.KubeName),
	); err != nil {
		return fmt.Errorf("wait for NVIDIA device plugin DaemonSet pod to be ready: %w", err)
	}

	s.Logger.Logf("NVIDIA device plugin DaemonSet pod is ready")
	return nil
}
