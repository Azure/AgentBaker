package datamodel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/parts"
)

const Nvidia470CudaDriverVersion = "cuda-470.82.01"

//nolint:gochecknoglobals
var (
	NvidiaCudaDriverVersion string
	NvidiaGridDriverVersion string
	AKSGPUCudaVersionSuffix string
	AKSGPUGridVersionSuffix string
)

type gpuVersion struct {
	RenovateTag   string `json:"renovateTag"`
	LatestVersion string `json:"latestVersion"`
}

type gpuContainerImage struct {
	DownloadURL string     `json:"downloadURL"`
	GPUVersion  gpuVersion `json:"gpuVersion"`
}

type componentsConfig struct {
	GPUContainerImages []gpuContainerImage `json:"GPUContainerImages"`
}

func LoadConfig() error {
	// Read the embedded components.json file
	data, err := parts.Templates.ReadFile("common/components.json")
	if err != nil {
		return fmt.Errorf("failed to read components.json: %w", err)
	}

	var config componentsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal components.json: %w", err)
	}

	const driverIndex = 0
	const suffixIndex = 1
	const expectedLength = 2

	for _, image := range config.GPUContainerImages {
		parts := strings.Split(image.GPUVersion.LatestVersion, "-")
		if len(parts) != expectedLength {
			continue
		}
		version, suffix := parts[driverIndex], parts[suffixIndex]

		if strings.Contains(image.DownloadURL, "aks-gpu-cuda") {
			NvidiaCudaDriverVersion = version
			AKSGPUCudaVersionSuffix = suffix
		} else if strings.Contains(image.DownloadURL, "aks-gpu-grid") {
			NvidiaGridDriverVersion = version
			AKSGPUGridVersionSuffix = suffix
		}
	}
	return nil
}

//nolint:gochecknoinits
func init() {
	if err := LoadConfig(); err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}
}

/* convergedGPUDriverSizes : these sizes use a "converged" driver to support both cuda/grid workloads.
how do you figure this out? ask HPC or find out by trial and error.
installing vanilla cuda drivers will fail to install with opaque errors.
nvidia-bug-report.sh may be helpful, but usually it tells you the pci card id is incompatible.
That sends me to HPC folks.
see https://github.com/Azure/azhpc-extensions/blob/daaefd78df6f27012caf30f3b54c3bd6dc437652/NvidiaGPU/resources.json
*/
//nolint:gochecknoglobals
var ConvergedGPUDriverSizes = map[string]bool{
	"standard_nv6ads_a10_v5":   true,
	"standard_nv12ads_a10_v5":  true,
	"standard_nv18ads_a10_v5":  true,
	"standard_nv36ads_a10_v5":  true,
	"standard_nv72ads_a10_v5":  true,
	"standard_nv36adms_a10_v5": true,
	"standard_nc8ads_a10_v4":   true,
	"standard_nc16ads_a10_v4":  true,
	"standard_nc32ads_a10_v4":  true,
}

//nolint:gochecknoglobals
var FabricManagerGPUSizes = map[string]bool{
	// A100
	"standard_nd96asr_v4":        true,
	"standard_nd112asr_a100_v4":  true,
	"standard_nd120asr_a100_v4":  true,
	"standard_nd96amsr_a100_v4":  true,
	"standard_nd112amsr_a100_v4": true,
	"standard_nd120amsr_a100_v4": true,
	// TODO(ace): one of these is probably dupe...
	// confirm with HPC/SKU owners.
	"standard_nd96ams_a100_v4": true,
	"standard_nd96ams_v4":      true,
	// H100.
	"standard_nd46s_h100_v5":    true,
	"standard_nd48s_h100_v5":    true,
	"standard_nd50s_h100_v5":    true,
	"standard_nd92is_h100_v5":   true,
	"standard_nd96is_h100_v5":   true,
	"standard_nd100is_h100_v5":  true,
	"standard_nd92isr_h100_v5":  true,
	"standard_nd96isr_h100_v5":  true,
	"standard_nd100isr_h100_v5": true,
	// H200
	"standard_nd96is_h200_v5":   true,
	"standard_nd96isr_h200_v5":  true,
	"standard_nd96isrf_h200_v5": true,
	// GB200 (Grace Blackwell)
	"standard_nd128isr_ndr_gb200_v6": true,
	// A100 oddballs.
	"standard_nc24ads_a100_v4": false, // NCads_v4 will fail to start fabricmanager.
	"standard_nc48ads_a100_v4": false,
	"standard_nc96ads_a100_v4": false,
}
