package scenario

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAMDGPUHostValidationRequiresPinnedDiagnostics(t *testing.T) {
	contents, err := os.ReadFile(repoPath("vhdbuilder/packer/amd-gpu-components.json"))
	require.NoError(t, err)
	script, err := amdGPUHostCheckCommand(contents)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(script, "sudo -n python3 - '"), "host diagnostics require noninteractive access to GPU devices")
	encoded := strings.SplitN(script, "'", 3)[1]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	var metadata map[string]map[string]string
	require.NoError(t, json.Unmarshal(decoded, &metadata))
	require.NotEmpty(t, metadata["AMDGPUDriver"]["moduleVersion"])
	require.Equal(t, "amdrocm-amdsmi10.0", metadata["AMDGPUDiagnostics"]["amdsmiPackage"])
	require.Equal(t, "amdrocm-sysdeps10.0", metadata["AMDGPUDiagnostics"]["sysdepsPackage"])

	for _, key := range []string{"amdsmiPackage", "amdsmiVersion", "sysdepsPackage", "sysdepsVersion", "cliPath"} {
		t.Run(key, func(t *testing.T) {
			var manifest map[string]map[string]string
			require.NoError(t, json.Unmarshal(decoded, &manifest))
			delete(manifest["AMDGPUDiagnostics"], key)
			incomplete, err := json.Marshal(manifest)
			require.NoError(t, err)
			script, err := amdGPUHostCheckCommand(incomplete)
			require.Empty(t, script)
			require.ErrorContains(t, err, "AMDGPUDiagnostics."+key)
		})
	}
}

func TestAMDMI300XScenarioSkipsBeforeProvisioning(t *testing.T) {
	t.Setenv(amdMI300XOptIn, "")
	s := newUbuntu2404MI300XAMDGPUScenario()
	outcome := runExecution(t.Context(), s.Name, s.Name, s,
		func(context.Context, string, *Scenario) error {
			t.Fatal("must skip before the provisioning flow resolves the unpublished VHD or creates resources")
			return nil
		})
	require.NoError(t, outcome.Error)
	require.Contains(t, outcome.SkipReason, amdMI300XOptIn)
}

func TestAMDMI300XScenarioIsOptInAndUsesDedicatedImage(t *testing.T) {
	s := newUbuntu2404MI300XAMDGPUScenario()
	for _, value := range []string{"", "false", "1"} {
		t.Setenv(amdMI300XOptIn, value)
		require.NotEmpty(t, s.SkipIf(t.Context()), "MI300X allocation needs explicit opt-in")
	}
	t.Setenv(amdMI300XOptIn, "true")
	require.Empty(t, s.SkipIf(t.Context()))
	require.True(t, s.Tags.GPU)
	require.Equal(t, config.DEFAULT_VMSKU, s.K8sSystemPoolSKU, "the shared AKS system pool must stay on a CPU SKU")
	require.True(t, s.SkipNVMeOSDiskPlacement, "managed OS disks have no ephemeral NVMe placement")
	require.Equal(t, "2404gen2amdgpucontainerd", s.VHD.Name)
	require.Equal(t, datamodel.AKSUbuntuContainerd2404Gen2, s.VHD.Distro)
	require.Equal(t, "2404gen2containerd", config.VHDUbuntu2404Gen2Containerd.Name, "generic image selection must stay unchanged")
	require.GreaterOrEqual(t, s.VHD.OSDiskSizeGB, int32(128), "allow space to unpack the PyTorch image")
	require.False(t, s.SkipDefaultValidation)

	nbc := &datamodel.NodeBootstrappingConfiguration{AgentPoolProfile: &datamodel.AgentPoolProfile{}}
	s.BootstrapConfigMutator(nil, nbc)
	require.Equal(t, amdMI300XVMSize, nbc.AgentPoolProfile.VMSize)
	require.True(t, nbc.EnableAMDGPU)
	require.True(t, nbc.ConfigGPUDriverIfNeeded)
	require.False(t, nbc.EnableNvidia)
	require.False(t, nbc.EnableGPUDevicePluginIfNeeded)

	vmss := &armcompute.VirtualMachineScaleSet{
		SKU: &armcompute.SKU{Name: to.Ptr("Standard_D2s_v3")},
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
					OSDisk: &armcompute.VirtualMachineScaleSetOSDisk{},
				},
			},
		},
	}
	s.VMConfigMutator(vmss)
	require.Equal(t, amdMI300XVMSize, *vmss.SKU.Name)
	anc := &aksnodeconfigv1.Configuration{GpuConfig: &aksnodeconfigv1.GpuConfig{}}
	s.AKSNodeConfigMutator(nil, anc)
	require.Equal(t, amdMI300XVMSize, anc.VmSize)
	require.True(t, anc.GpuConfig.GetEnableAmdGpu())
}

func TestAMDMI300XScenarioUsesManagedOSDisk(t *testing.T) {
	for _, test := range []struct {
		name        string
		managedDisk *armcompute.VirtualMachineScaleSetManagedDiskParameters
	}{
		{name: "default ephemeral disk without managed parameters"},
		{
			name: "existing managed parameters",
			managedDisk: &armcompute.VirtualMachineScaleSetManagedDiskParameters{
				StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
				DiskEncryptionSet: &armcompute.DiskEncryptionSetParameters{
					ID: to.Ptr("/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/diskEncryptionSets/test"),
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newUbuntu2404MI300XAMDGPUScenario()
			osDisk := &armcompute.VirtualMachineScaleSetOSDisk{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
				DiskSizeGB:   to.Ptr(s.VHD.OSDiskSizeGB),
				Caching:      to.Ptr(armcompute.CachingTypesReadOnly),
				DiffDiskSettings: &armcompute.DiffDiskSettings{
					Option:    to.Ptr(armcompute.DiffDiskOptionsLocal),
					Placement: to.Ptr(armcompute.DiffDiskPlacementResourceDisk),
				},
				ManagedDisk: test.managedDisk,
			}
			vmss := &armcompute.VirtualMachineScaleSet{
				SKU: &armcompute.SKU{},
				Properties: &armcompute.VirtualMachineScaleSetProperties{
					VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
						StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{OSDisk: osDisk},
					},
				},
			}
			s.VMConfigMutator(vmss)
			require.True(t, s.SkipNVMeOSDiskPlacement, "only this scenario opts out of ephemeral NVMe placement")
			require.Nil(t, osDisk.DiffDiskSettings, "the 256 GiB image must not depend on ephemeral ResourceDisk capacity")
			require.NotNil(t, osDisk.ManagedDisk)
			require.Equal(t, armcompute.StorageAccountTypesPremiumLRS, *osDisk.ManagedDisk.StorageAccountType)
			require.Equal(t, int32(256), *osDisk.DiskSizeGB)
			require.Equal(t, armcompute.DiskCreateOptionTypesFromImage, *osDisk.CreateOption)
			require.Equal(t, armcompute.CachingTypesReadOnly, *osDisk.Caching)
			if test.managedDisk != nil {
				require.Same(t, test.managedDisk, osDisk.ManagedDisk, "preserve other managed disk settings")
				require.NotNil(t, osDisk.ManagedDisk.DiskEncryptionSet)
			}
		})
	}
}

func TestAMDScenarioEnablesANCFlag(t *testing.T) {
	nbc, err := baseTemplateLinux("francecentral", "1.34.0", "amd64")
	require.NoError(t, err)
	anc, err := nbcToAKSNodeConfigV1(nbc)
	require.NoError(t, err)
	require.Nil(t, anc.GpuConfig.EnableAmdGpu, "default conversion stays unchanged")
	newUbuntu2404MI300XAMDGPUScenario().AKSNodeConfigMutator(nil, anc)
	require.True(t, anc.GpuConfig.GetEnableAmdGpu())
	require.True(t, anc.GpuConfig.ConfigGpuDriver)
}

func TestAMDGPUManifestsUseAssignedDevicesAndPinnedUserspace(t *testing.T) {
	owner := metav1.OwnerReference{APIVersion: "v1", Kind: "Node", Name: "mi300x-node", UID: "node-uid"}
	ds := amdDevicePluginDaemonset(owner.Name, owner)
	require.Equal(t, []metav1.OwnerReference{owner}, ds.OwnerReferences)
	require.Equal(t, ds.Spec.Selector.MatchLabels, ds.Spec.Template.Labels)
	require.Equal(t, owner.Name, ds.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"])
	require.False(t, *ds.Spec.Template.Spec.AutomountServiceAccountToken)
	require.Len(t, ds.Spec.Template.Spec.Containers, 1)
	plugin := ds.Spec.Template.Spec.Containers[0]
	require.Equal(t, "rocm/k8s-device-plugin@sha256:e4df5dc9a7fa34e2344852256dcc5762171a6d68f1f9a34026ce26786ae335e2", plugin.Image)
	require.True(t, *plugin.SecurityContext.Privileged)
	require.NotNil(t, plugin.SecurityContext.Capabilities)
	require.Equal(t, []corev1.Capability{"ALL"}, plugin.SecurityContext.Capabilities.Drop)
	require.Len(t, ds.Spec.Template.Spec.Volumes, 2)
	paths := map[string]bool{}
	for _, volume := range ds.Spec.Template.Spec.Volumes {
		require.NotNil(t, volume.HostPath)
		paths[volume.HostPath.Path] = true
	}
	require.Equal(t, map[string]bool{"/sys": true, "/var/lib/kubelet/device-plugins": true}, paths)

	pod := amdGPUTrainingPod(owner.Name, owner)
	require.Equal(t, []metav1.OwnerReference{owner}, pod.OwnerReferences)
	require.Equal(t, owner.Name, pod.Spec.NodeSelector["kubernetes.io/hostname"])
	require.Empty(t, pod.Spec.NodeName, "the scheduler must allocate extended GPU resources")
	require.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)
	require.NotNil(t, pod.Spec.ActiveDeadlineSeconds)
	require.False(t, *pod.Spec.AutomountServiceAccountToken)
	require.Len(t, pod.Spec.Containers, 1)
	workload := pod.Spec.Containers[0]
	require.Equal(t, "rocm/pytorch@sha256:bbdaba66029f905321be3bbc95206a2d9a56a2bc5300877d146ad24d40db9a12", workload.Image)
	requested, limited := workload.Resources.Requests["amd.com/gpu"], workload.Resources.Limits["amd.com/gpu"]
	require.Equal(t, int64(8), requested.Value())
	require.Equal(t, int64(8), limited.Value())
	require.Empty(t, pod.Spec.Volumes, "workload must use container ROCm and plugin-assigned devices")
	require.Empty(t, workload.VolumeMounts, "do not mount host ROCm libraries or bypass the plugin")
	require.Equal(t, []string{"python3", "-u", "-c", amdGPUTrainingScript}, workload.Command)
	require.NotEmpty(t, amdGPUTrainingScript)

	other := amdGPUTrainingPod(owner.Name, owner)
	require.NotEqual(t, pod.Name, other.Name, "parallel/repeated scenarios must not share pods")
}

func TestAMDGPUResourcesRequireEightHealthyGPUs(t *testing.T) {
	for _, test := range []struct {
		name        string
		capacity    string
		allocatable string
		ready       bool
	}{
		{"ready", "8", "8", true},
		{"unhealthy GPU", "8", "7", false},
		{"missing GPU", "7", "7", false},
		{"inactive render nodes counted", "64", "64", false},
		{"not registered", "0", "0", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			node := &corev1.Node{Status: corev1.NodeStatus{
				Capacity:    corev1.ResourceList{"amd.com/gpu": resource.MustParse(test.capacity)},
				Allocatable: corev1.ResourceList{"amd.com/gpu": resource.MustParse(test.allocatable)},
			}}
			require.Equal(t, test.ready, amdGPUResourcesReady(node))
		})
	}
}

func TestAMDGPUTrainingRequiresSuccessfulCompletion(t *testing.T) {
	for _, test := range []struct {
		phase  corev1.PodPhase
		done   bool
		failed bool
	}{
		{corev1.PodPending, false, false},
		{corev1.PodRunning, false, false},
		{corev1.PodSucceeded, true, false},
		{corev1.PodFailed, false, true},
	} {
		t.Run(string(test.phase), func(t *testing.T) {
			pod := &corev1.Pod{Status: corev1.PodStatus{Phase: test.phase}}
			done, err := amdGPUTrainingPodCompleted(pod)
			require.Equal(t, test.done, done)
			if test.failed {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
	failedContainer := &corev1.Pod{Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{Name: "training", State: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, Message: "reference mismatch"},
		}}},
	}}
	done, err := amdGPUTrainingPodCompleted(failedContainer)
	require.False(t, done)
	require.ErrorContains(t, err, "reference mismatch")
}

func TestAMDMI300XRequiresExplicitRunnerSKU(t *testing.T) {
	original := config.Config.DefaultVMSKU
	t.Cleanup(func() { config.Config.DefaultVMSKU = original })
	s := newUbuntu2404MI300XAMDGPUScenario()
	config.Config.DefaultVMSKU = config.DEFAULT_VMSKU
	require.ErrorContains(t, s.BootstrapConfigMutatorWithError(t.Context(), nil, nil), "--vm-sku")
	config.Config.DefaultVMSKU = amdMI300XVMSize
	require.NoError(t, s.BootstrapConfigMutatorWithError(t.Context(), nil, nil))
	require.Equal(t, amdMI300XVMSize, scenarioVMSize(s), "existing runner SKU selection handles AMD before capability queries")
}
