package scenario

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	amdMI300XVMSize = "Standard_ND96isr_MI300X_v5"
	amdMI300XOptIn  = "AGENTBAKER_E2E_ENABLE_MI300X"
	// Lab-validated upstream images: device plugin 1.31.0.11 and PyTorch 2.13.0 / ROCm 10.0.
	amdDevicePluginImage     = "rocm/k8s-device-plugin@sha256:e4df5dc9a7fa34e2344852256dcc5762171a6d68f1f9a34026ce26786ae335e2"
	amdPyTorchImage          = "rocm/pytorch@sha256:bbdaba66029f905321be3bbc95206a2d9a56a2bc5300877d146ad24d40db9a12"
	amdTrainingSuccessMarker = "AMDGPU_TRAINING_PASS "
)

//go:embed fixtures/amd_gpu_training.py
var amdGPUTrainingScript string

//go:embed fixtures/amd_gpu_host_check.py
var amdGPUHostCheckScript string

var _ = Register(newUbuntu2404MI300XAMDGPUScenario())

func newUbuntu2404MI300XAMDGPUScenario() *Scenario {
	return &Scenario{
		Name:        "Ubuntu2404_MI300X_AMDGPU",
		Description: "Opt-in Ubuntu 24.04 MI300X image with AMDGPU and AMD SMI: eight AMD GPUs, upstream device plugin, and containerized FP32 training checked against a CPU reference",
		Tags:        Tags{GPU: true},
		Location:    "francecentral",
		SkipIf: func(context.Context) string {
			if os.Getenv(amdMI300XOptIn) != "true" {
				return "set " + amdMI300XOptIn + "=true after publishing the dedicated VHD and confirming MI300X quota"
			}
			return ""
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2AMDGPUContainerd,
			VMSize:  amdMI300XVMSize,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = amdMI300XVMSize
				nbc.EnableAMDGPU = true
				nbc.EnableNvidia = false
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
			},
			AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
				cfg.VmSize = amdMI300XVMSize
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(amdMI300XVMSize)
				// Qualify the driver without depending on the SKU's ephemeral disk placement or size.
				osDisk := vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk
				osDisk.DiffDiskSettings = nil
				if osDisk.ManagedDisk == nil {
					osDisk.ManagedDisk = &armcompute.VirtualMachineScaleSetManagedDiskParameters{}
				}
				osDisk.ManagedDisk.StorageAccountType = to.Ptr(armcompute.StorageAccountTypesPremiumLRS)
			},
			Validator: validateAMDMI300X,
		},
	}
}

func validateAMDMI300X(ctx context.Context, s *Scenario) error {
	for _, check := range []struct {
		name string
		run  func(context.Context, *Scenario) error
	}{
		{"driver/amd-baked-driver", validateAMDGPUHost},
		{"bootstrap/kubelet-continuity", ValidateKubeletHasNotStopped},
		{"plugin/amd-deploy", deployAMDDevicePluginDaemonset},
		{"plugin/amd-eight-gpus", validateAMDGPUResources},
		{"workload/amd-pytorch-training", validateAMDGPUTraining},
	} {
		if err := runGPUCheck(ctx, s, check.name, check.run); err != nil {
			return err
		}
	}
	return nil
}

func validateAMDGPUHost(ctx context.Context, s *Scenario) error {
	contents, err := os.ReadFile(repoPath("parts/common/components.json"))
	if err != nil {
		return err
	}
	script, err := amdGPUHostCheckCommand(contents)
	if err != nil {
		return err
	}
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "baked AMDGPU driver, AMD SMI diagnostics or MI300X devices did not match")
	if err == nil {
		logging.Logf(ctx, "AMDGPU host validation: %s", result.stdout)
	}
	return err
}

func amdGPUHostCheckCommand(contents []byte) (string, error) {
	var manifest struct {
		AMDGPUDriver      map[string]string
		AMDGPUDiagnostics map[string]string
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return "", err
	}
	for _, key := range []string{"packageVersion", "firmwarePackageVersion", "moduleVersion", "dkmsVersion"} {
		if manifest.AMDGPUDriver[key] == "" {
			return "", fmt.Errorf("AMDGPUDriver.%s missing from components.json", key)
		}
	}
	for _, key := range []string{"amdsmiPackage", "amdsmiVersion", "sysdepsPackage", "sysdepsVersion", "cliPath"} {
		if manifest.AMDGPUDiagnostics[key] == "" {
			return "", fmt.Errorf("AMDGPUDiagnostics.%s missing from components.json", key)
		}
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	return "python3 - '" + base64.StdEncoding.EncodeToString(encoded) + "' <<'AMDGPU_CHECK'\n" + amdGPUHostCheckScript + "\nAMDGPU_CHECK\n", nil
}

func amdGPUTolerations() []corev1.Toleration {
	return []corev1.Toleration{
		{Key: "CriticalAddonsOnly", Operator: corev1.TolerationOpExists},
		{Key: "amd.com/gpu", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
	}
}

func amdDevicePluginDaemonset(nodeName string, owner metav1.OwnerReference) *appsv1.DaemonSet {
	name := uniqueKubernetesResourceName("amddp-" + nodeName)
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "kube-system", OwnerReferences: []metav1.OwnerReference{owner}},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"name": name}},
				Spec: corev1.PodSpec{
					NodeSelector:                 map[string]string{"kubernetes.io/hostname": nodeName, "kubernetes.io/arch": "amd64"},
					AutomountServiceAccountToken: to.Ptr(false),
					PriorityClassName:            "system-node-critical",
					Tolerations:                  amdGPUTolerations(),
					Containers: []corev1.Container{{
						Name: "device-plugin", Image: amdDevicePluginImage,
						SecurityContext: &corev1.SecurityContext{Privileged: to.Ptr(true)},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "device-plugins", MountPath: "/var/lib/kubelet/device-plugins"},
							{Name: "sys", MountPath: "/sys"},
						},
					}},
					Volumes: []corev1.Volume{
						{Name: "device-plugins", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/device-plugins", Type: to.Ptr(corev1.HostPathDirectory)}}},
						{Name: "sys", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/sys", Type: to.Ptr(corev1.HostPathDirectory)}}},
					},
				},
			},
		},
	}
}

func deployAMDDevicePluginDaemonset(ctx context.Context, s *Scenario) error {
	owner, err := scenarioNodeOwnerReference(ctx, s)
	if err != nil {
		return err
	}
	ds := amdDevicePluginDaemonset(s.Runtime.VM.KubeName, owner)
	created, err := s.Runtime.Kube.Typed.AppsV1().DaemonSets(ds.Namespace).Create(ctx, ds, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create AMD device plugin: %w", err)
	}
	s.Cleanup(func(ctx context.Context) error {
		err := s.Runtime.Kube.Typed.AppsV1().DaemonSets(created.Namespace).Delete(ctx, created.Name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	_, err = s.Runtime.Kube.WaitUntilPodRunning(ctx, created.Namespace, "name="+created.Name, "spec.nodeName="+s.Runtime.VM.KubeName)
	return err
}

func validateAMDGPUResources(ctx context.Context, s *Scenario) error {
	return wait.PollUntilContextTimeout(ctx, 3*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return amdGPUResourcesReady(node), nil
	})
}

func amdGPUResourcesReady(node *corev1.Node) bool {
	capacity, allocatable := node.Status.Capacity["amd.com/gpu"], node.Status.Allocatable["amd.com/gpu"]
	return capacity.Value() == 8 && allocatable.Value() == 8
}

func amdGPUTrainingPod(nodeName string, owner metav1.OwnerReference) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: uniqueKubernetesResourceName("amd-train-" + nodeName), Namespace: "default", OwnerReferences: []metav1.OwnerReference{owner}},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: to.Ptr(false),
			RestartPolicy:                corev1.RestartPolicyNever,
			ActiveDeadlineSeconds:        to.Ptr(int64(1800)),
			NodeSelector:                 map[string]string{"kubernetes.io/hostname": nodeName},
			Tolerations:                  amdGPUTolerations(),
			Containers: []corev1.Container{{
				Name: "training", Image: amdPyTorchImage, ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{"python3", "-u", "-c", amdGPUTrainingScript},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{"amd.com/gpu": resource.MustParse("8"), corev1.ResourceCPU: resource.MustParse("8"), corev1.ResourceMemory: resource.MustParse("16Gi")},
					Limits:   corev1.ResourceList{"amd.com/gpu": resource.MustParse("8"), corev1.ResourceMemory: resource.MustParse("32Gi")},
				},
			}},
		},
	}
}

// Completion is required: a merely Running GPU pod does not prove training passed.
func amdGPUTrainingPodCompleted(pod *corev1.Pod) (bool, error) {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return true, nil
	case corev1.PodFailed:
		return false, fmt.Errorf("AMD training pod %s failed: %s %s", pod.Name, pod.Status.Reason, pod.Status.Message)
	default:
		for _, container := range pod.Status.ContainerStatuses {
			if state := container.State.Terminated; state != nil && state.ExitCode != 0 {
				return false, fmt.Errorf("AMD training container %s exited %d: %s", container.Name, state.ExitCode, state.Message)
			}
		}
		return false, nil
	}
}

func validateAMDGPUTraining(ctx context.Context, s *Scenario) error {
	owner, err := scenarioNodeOwnerReference(ctx, s)
	if err != nil {
		return err
	}
	pod := amdGPUTrainingPod(s.Runtime.VM.KubeName, owner)
	created, err := s.Runtime.Kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create AMD training pod: %w", err)
	}
	s.Cleanup(func(ctx context.Context) error {
		err := s.Runtime.Kube.Typed.CoreV1().Pods(created.Namespace).Delete(ctx, created.Name, metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := s.Runtime.Kube.Typed.CoreV1().Pods(created.Namespace).Get(ctx, created.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return amdGPUTrainingPodCompleted(pod)
	})
	logs, logErr := s.Runtime.Kube.Typed.CoreV1().Pods(created.Namespace).GetLogs(created.Name, &corev1.PodLogOptions{LimitBytes: to.Ptr(int64(512 * 1024))}).DoRaw(ctx)
	logging.Logf(ctx, "AMD training pod %s: %s", created.Name, logs)
	if err != nil {
		logPodDebugInfo(ctx, s.Runtime.Kube, created)
		return fmt.Errorf("AMD training did not complete: %w", err)
	}
	if logErr != nil {
		return fmt.Errorf("read AMD training result: %w", logErr)
	}
	if !strings.Contains(string(logs), amdTrainingSuccessMarker) {
		return fmt.Errorf("AMD training pod completed without its success record")
	}
	return nil
}
