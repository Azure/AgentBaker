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
	NvidiaCudaDriverVersion    string
	NvidiaGridDriverVersion    string
	NvidiaGridV20DriverVersion string
	AKSGPUCudaVersionSuffix    string
	AKSGPUGridVersionSuffix    string
	AKSGPUGridV20VersionSuffix string
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
		// Named versionParts (not parts) to avoid shadowing the imported parts package.
		versionParts := strings.Split(image.GPUVersion.LatestVersion, "-")
		if len(versionParts) != expectedLength {
			continue
		}
		version, suffix := versionParts[driverIndex], versionParts[suffixIndex]

		// Match on the exact repo name (final path segment, tag stripped) so that
		// repos sharing a prefix (e.g. "aks-gpu-grid" vs "aks-gpu-grid-v20") are not
		// confused by substring matching.
		switch gpuImageRepo(image.DownloadURL) {
		case "aks-gpu-cuda":
			NvidiaCudaDriverVersion = version
			AKSGPUCudaVersionSuffix = suffix
		case "aks-gpu-grid":
			NvidiaGridDriverVersion = version
			AKSGPUGridVersionSuffix = suffix
		case "aks-gpu-grid-v20":
			NvidiaGridV20DriverVersion = version
			AKSGPUGridV20VersionSuffix = suffix
		}
	}
	return nil
}

// gpuImageRepo extracts the bare repo name from a download URL such as
// "mcr.microsoft.com/aks/aks-gpu-grid-v20:*" -> "aks-gpu-grid-v20".
func gpuImageRepo(downloadURL string) string {
	repo := downloadURL
	if idx := strings.LastIndex(repo, "/"); idx != -1 {
		repo = repo[idx+1:]
	}
	if idx := strings.Index(repo, ":"); idx != -1 {
		repo = repo[:idx]
	}
	return repo
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

/* RTXPro6000GPUDriverSizes : NC_RTXPRO6000BSE_v6 (RTX PRO 6000 Blackwell Server
Edition) SKUs require the GRID v20 (595.x) driver, published as the
aks-gpu-grid-v20 image. All other GRID SKUs continue to use aks-gpu-grid.
Each size ships as a ds (higher-memory) and lds (lower-memory) pair; both use
the same GPU and therefore the same driver, so both are listed here.
*/
//nolint:gochecknoglobals
var RTXPro6000GPUDriverSizes = map[string]bool{
	"standard_nc128ds_xl_rtxpro6000bse_v6":  true,
	"standard_nc128lds_xl_rtxpro6000bse_v6": true,
	"standard_nc256ds_xl_rtxpro6000bse_v6":  true,
	"standard_nc256lds_xl_rtxpro6000bse_v6": true,
	"standard_nc320ds_xl_rtxpro6000bse_v6":  true,
	"standard_nc320lds_xl_rtxpro6000bse_v6": true,
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
	// A100 oddballs.
	"standard_nc24ads_a100_v4": false, // NCads_v4 will fail to start fabricmanager.
	"standard_nc48ads_a100_v4": false,
	"standard_nc96ads_a100_v4": false,
}
