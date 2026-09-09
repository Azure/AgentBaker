package e2e

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// Ubuntu2204_NvidiaDevicePlugin_Daemonset tests the upstream, customer-managed
// NVIDIA device plugin DaemonSet deployment model instead of the systemd service.
var _ = Register(&Scenario{
	Name:        "Ubuntu2204_NvidiaDevicePlugin_Daemonset",
	Description: "Tests that the NVIDIA device plugin works as a DaemonSet instead of a systemd service",
	Tags: Tags{
		GPU: true,
	},
	Config: Config{
		Cluster: ClusterKubenet,
		VHD:     config.VHDUbuntu2204Gen2Containerd,
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
			nbc.ConfigGPUDriverIfNeeded = true
			// Don't enable the managed GPU experience - the test deploys the upstream DaemonSet.
			// By not setting EnableManagedGPU=true or the VMSS tag, the systemd-based device plugin won't start.
			nbc.EnableGPUDevicePluginIfNeeded = false
			nbc.EnableNvidia = true
		},
		VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			// The device plugin is only meaningful once the driver is present and the
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

			if err := deployNvidiaDevicePluginDaemonset(ctx, s); err != nil {
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

// validateNvidiaDevicePluginServiceNotRunning verifies that the systemd-based
// NVIDIA device plugin service is not running because the test uses the DaemonSet model.
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

// nvidiaDevicePluginDaemonset returns the official upstream deployment narrowed
// to the scenario's node.
// https://github.com/NVIDIA/k8s-device-plugin/blob/main/deployments/static/nvidia-device-plugin.yml
func nvidiaDevicePluginDaemonset(nodeName string, ownerReference metav1.OwnerReference) *appsv1.DaemonSet {
	name := uniqueKubernetesResourceName("nvdp-" + nodeName)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       "kube-system",
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"name": name},
				},
				Spec: corev1.PodSpec{
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

func deployNvidiaDevicePluginDaemonset(ctx context.Context, s *Scenario) error {
	s.Logger.Logf("Deploying NVIDIA device plugin as DaemonSet...")
	ownerReference, err := scenarioNodeOwnerReference(ctx, s)
	if err != nil {
		return err
	}

	ds := nvidiaDevicePluginDaemonset(s.Runtime.VM.KubeName, ownerReference)
	created, err := s.Runtime.Kube.Typed.AppsV1().DaemonSets(ds.Namespace).Create(ctx, ds, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create NVIDIA device plugin DaemonSet %s/%s: %w", ds.Namespace, ds.Name, err)
	}

	s.Logger.Logf("NVIDIA device plugin DaemonSet %s/%s created successfully", created.Namespace, created.Name)
	s.Cleanup(func(ctx context.Context) error {
		if err := s.Runtime.Kube.Typed.AppsV1().DaemonSets(created.Namespace).Delete(
			ctx,
			created.Name,
			metav1.DeleteOptions{},
		); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete NVIDIA device plugin DaemonSet %s/%s: %w", created.Namespace, created.Name, err)
		}
		return nil
	})
	s.Logger.Logf("Waiting for NVIDIA device plugin DaemonSet pod to be ready on node %s...", s.Runtime.VM.KubeName)

	if _, err := s.Runtime.Kube.WaitUntilPodRunning(
		ctx,
		created.Namespace,
		"name="+created.Name,
		"spec.nodeName="+s.Runtime.VM.KubeName,
	); err != nil {
		return fmt.Errorf("wait for NVIDIA device plugin DaemonSet %s/%s: %w", created.Namespace, created.Name, err)
	}

	s.Logger.Logf("NVIDIA device plugin DaemonSet pod is ready")
	return nil
}
