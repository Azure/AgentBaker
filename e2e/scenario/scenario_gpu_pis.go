package scenario

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
)

var _ = Register(pisGPUScenario("Ubuntu2404_ManagedGPU_VHDCaching", config.VHDUbuntu2404Gen2Containerd, true))
var _ = Register(pisGPUScenario("AzureLinuxV3_ManagedGPU_VHDCaching", config.VHDAzureLinuxV3Gen2, true))
var _ = Register(pisGPUScenario("Ubuntu2404_GPUDriverOptOut_VHDCaching", config.VHDUbuntu2404Gen2Containerd, false))

func pisGPUScenario(name string, image *config.Image, managed bool) *Scenario {
	const vmSize = "Standard_NC4as_T4_v3"
	s := &Scenario{
		Name:        name,
		Description: "Tests managed GPU driver, device plugin, and workload parity on a PIS-derived T4 node",
		Location:    "westus2",
		Tags:        Tags{GPU: true},
		Config: Config{
			Cluster:    ClusterKubenet,
			VHD:        image,
			VHDCaching: true,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.EnableNvidia = managed
				nbc.ConfigGPUDriverIfNeeded = managed
				nbc.EnableGPUDevicePluginIfNeeded = managed
				nbc.ManagedGPUExperienceAFECEnabled = managed
				// Exercise this branch's scripts in both phases, not an older VHD's scripts.
				nbc.EnableScriptlessCSECmd = false
				nbc.EnableScriptlessNBCCSECmd = false
				nbc.EnableScriptlessAKSNodeConfig = false
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
				if managed {
					if vmss.Tags == nil {
						vmss.Tags = map[string]*string{}
					}
					vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")
				}
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				if err := errors.Join(
					validateNvidiaDriverInstalled(ctx, s),
					ValidateNvidiaDevicePluginServiceRunning(ctx, s),
					ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu"),
				); err != nil {
					return err
				}
				return ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu")
			},
		},
	}
	if !managed {
		s.Description = "Installs a customer NVIDIA driver after basePrep and verifies it works after PIS capture with AKS driver installation disabled"
		s.VHDCachingPostBasePrep = installPISCustomerGPUDriver
		s.Validator = validatePISCustomerGPUDriver
	}
	return s
}

func installPISCustomerGPUDriver(ctx context.Context, s *Scenario) error {
	version := datamodel.NvidiaCudaLTSDriverVersion
	suffix := datamodel.AKSGPUCudaLTSVersionSuffix
	if version == "" || suffix == "" {
		return fmt.Errorf("approved CUDA LTS installer version is unavailable")
	}
	image := "mcr.microsoft.com/aks/aks-gpu-cuda-lts:" + version + "-" + suffix
	script := fmt.Sprintf(`set -euo pipefail
test -f /opt/azure/containers/base_prep.complete
image=%s
version=%s
if [ -z "$(sudo ctr -n k8s.io images ls -q "name==${image}")" ]; then
  echo "Required CUDA installer image is not cached in this VHD: ${image}. Use a VHD built with the matching components.json." >&2
  exit 1
fi
sudo mkdir -p /opt/gpu /opt/actions
sudo ctr -n k8s.io run --privileged --rm --net-host \
  --with-ns pid:/proc/1/ns/pid \
  --mount type=bind,src=/opt/gpu,dst=/mnt/gpu,options=rbind \
  --mount type=bind,src=/opt/actions,dst=/mnt/actions,options=rbind \
  "${image}" gpuinstall /entrypoint.sh copy
cd "/opt/gpu/NVIDIA-Linux-x86_64-${version}"
sudo ./nvidia-installer -s -k="$(uname -r)" --no-drm --dkms \
  --utility-prefix=/usr/local/nvidia --utility-libdir=lib64
sudo /usr/local/nvidia/bin/nvidia-modprobe -u -c0
`, shellSingleQuote(image), shellSingleQuote(version))
	if _, err := RunCommand(ctx, s, script); err != nil {
		return fmt.Errorf("install customer GPU driver after basePrep: %w", err)
	}
	return validatePISCustomerGPUDriver(ctx, s)
}

func validatePISCustomerGPUDriver(ctx context.Context, s *Scenario) error {
	// This real installation lives under GPU_DEST, which the old nodePrep cleanup
	// deleted even without a CUDA prebake marker.
	script := fmt.Sprintf(`set -euo pipefail
version=$(sudo /usr/local/nvidia/bin/nvidia-smi --query-gpu=driver_version --format=csv,noheader)
echo "Customer driver version: ${version}"
test "${version}" = %s
test -x "/opt/gpu/NVIDIA-Linux-x86_64-%s/nvidia-installer"
`, shellSingleQuote(datamodel.NvidiaCudaLTSDriverVersion), datamodel.NvidiaCudaLTSDriverVersion)
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "customer GPU driver must work before and after PIS image capture")
	return err
}
