package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// nvidiaDevicePluginImageName is the name of the NVIDIA device plugin container image
	// in components.json E2EContainerImages section. The version is managed via Renovate.
	nvidiaDevicePluginImageName = "nvidia-k8s-device-plugin"
)

// getNvidiaDevicePluginImage returns the full container image URL for the NVIDIA device plugin
// by reading the version from components.json E2EContainerImages section.
func getNvidiaDevicePluginImage() string {
	image := components.GetE2EContainerImage(nvidiaDevicePluginImageName)
	if strings.TrimSpace(image) == "" {
		panic(fmt.Sprintf("nvidia device plugin image %q not found or empty in components.json E2EContainerImages", nvidiaDevicePluginImageName))
	}
	return image
}

// validateNvidiaDevicePluginServiceNotRunning verifies that the systemd-based
// NVIDIA device plugin service is not running (since we're testing the DaemonSet model).
func validateNvidiaDevicePluginServiceNotRunning(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("Verifying that nvidia-device-plugin.service is not running...")

	// Check the current service state using "is-active".
	// This will return "active", "inactive", "failed", "activating", "unknown", etc.
	result := execScriptOnVMForScenario(ctx, s, "systemctl is-active nvidia-device-plugin.service 2>/dev/null || true")
	output := strings.TrimSpace(result.stdout)

	// The service should either not exist or be in a non-running state.
	// Treat both "active" and "activating" as failures, since the service
	// must not be running when validating the DaemonSet-based deployment.
	if output == "active" || output == "activating" {
		s.T.Fatalf("nvidia-device-plugin.service is unexpectedly %s - this test requires the systemd service to be disabled", output)
	}
	s.T.Logf("Confirmed nvidia-device-plugin.service is not active (status: %s)", output)
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
	// Ensure name ends with alphanumeric character (DNS label requirement)
	name = strings.TrimRight(name, "-")
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
							Image: getNvidiaDevicePluginImage(),
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
func deployNvidiaDevicePluginDaemonset(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("Deploying NVIDIA device plugin as DaemonSet...")

	ds := nvidiaDevicePluginDaemonset(s.Runtime.VM.KubeName)
	dsClient := s.Runtime.Cluster.Kube.Typed.AppsV1().DaemonSets(ds.Namespace)

	// Delete any existing DaemonSet from a previous failed run and wait for it to be gone
	deleteCtx, deleteCancel := context.WithTimeout(ctx, 60*time.Second)
	defer deleteCancel()
	_ = dsClient.Delete(deleteCtx, ds.Name, metav1.DeleteOptions{})

	// Wait for deletion to complete to avoid AlreadyExists/conflict on create
	for {
		_, err := dsClient.Get(deleteCtx, ds.Name, metav1.GetOptions{})
		if err != nil {
			break // NotFound or other error means it's gone
		}
		select {
		case <-deleteCtx.Done():
			s.T.Fatalf("timed out waiting for existing DaemonSet %s to be deleted", ds.Name)
		case <-time.After(2 * time.Second):
		}
	}

	// Create the DaemonSet
	err := s.Runtime.Cluster.Kube.CreateDaemonset(ctx, ds)
	require.NoError(s.T, err, "failed to create NVIDIA device plugin DaemonSet")

	s.T.Logf("NVIDIA device plugin DaemonSet %s/%s created successfully", ds.Namespace, ds.Name)

	// Register cleanup to delete the DaemonSet when the test finishes
	s.T.Cleanup(func() {
		s.T.Logf("Cleaning up NVIDIA device plugin DaemonSet %s/%s...", ds.Namespace, ds.Name)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		deleteErr := s.Runtime.Cluster.Kube.Typed.AppsV1().DaemonSets(ds.Namespace).Delete(
			cleanupCtx,
			ds.Name,
			metav1.DeleteOptions{},
		)
		if deleteErr != nil {
			s.T.Logf("Failed to delete NVIDIA device plugin DaemonSet %s/%s: %v", ds.Namespace, ds.Name, deleteErr)
		}
	})
}

// waitForNvidiaDevicePluginDaemonsetReady waits for the NVIDIA device plugin pod to be running on the test node.
// Uses the existing WaitUntilPodRunning helper which handles CrashLoopBackOff and other failure states.
func waitForNvidiaDevicePluginDaemonsetReady(ctx context.Context, s *Scenario) {
	s.T.Helper()

	dsName := nvidiaDevicePluginDaemonsetName(s.Runtime.VM.KubeName)
	s.T.Logf("Waiting for NVIDIA device plugin DaemonSet pod to be ready on node %s...", s.Runtime.VM.KubeName)

	_, err := s.Runtime.Cluster.Kube.WaitUntilPodRunning(
		ctx,
		"kube-system",
		fmt.Sprintf("name=%s", dsName),
		fmt.Sprintf("spec.nodeName=%s", s.Runtime.VM.KubeName),
	)
	require.NoError(s.T, err, "timed out waiting for NVIDIA device plugin DaemonSet pod to be ready")

	s.T.Logf("NVIDIA device plugin DaemonSet pod is ready")
}
