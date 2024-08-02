/*
Portions Copyright (c) Microsoft Corporation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	nvidia470CudaDriverVersion = "cuda-470.82.01"
	nvidia550CudaDriverVersion = "cuda-550.90.07"
	nvidia535GridDriverVersion = "grid-535.161.08"
)

// These SHAs will change once we update aks-gpu images in aks-gpu repository. We do that fairly rarely at this time.
// So for now these will be kept here like this.
const (
	aksGPUCudaSHA = "sha-b40b85"
	aksGPUGridSHA = "sha-7b2b12"
)

/*
	nvidiaEnabledSKUs :  If a new GPU sku becomes available, add a key to this map, but only if you have a confirmation
	that we have an agreement with NVIDIA for this specific gpu.
*/
//nolint:gochecknoglobals
var nvidiaEnabledSKUs = sets.NewString(
	// K80: https://learn.microsoft.com/en-us/azure/virtual-machines/nc-series
	"standard_nc6",
	"standard_nc12",
	"standard_nc24",
	"standard_nc24r",
	// M60: https://learn.microsoft.com/en-us/azure/virtual-machines/nv-series
	"standard_nv6",
	"standard_nv12",
	"standard_nv12s_v3",
	"standard_nv24",
	"standard_nv24s_v3",
	"standard_nv24r",
	"standard_nv48s_v3",
	// P40: https://learn.microsoft.com/en-us/azure/virtual-machines/nd-series
	"standard_nd6s",
	"standard_nd12s",
	"standard_nd24s",
	"standard_nd24rs",
	// P100: https://learn.microsoft.com/en-us/azure/virtual-machines/ncv2-series
	"standard_nc6s_v2",
	"standard_nc12s_v2",
	"standard_nc24s_v2",
	"standard_nc24rs_v2",
	// V100: https://learn.microsoft.com/en-us/azure/virtual-machines/ncv3-series
	"standard_nc6s_v3",
	"standard_nc12s_v3",
	"standard_nc24s_v3",
	"standard_nc24rs_v3",
	"standard_nd40s_v3",
	"standard_nd40rs_v2",
	// T4: https://learn.microsoft.com/en-us/azure/virtual-machines/nct4-v3-series
	"standard_nc4as_t4_v3",
	"standard_nc8as_t4_v3",
	"standard_nc16as_t4_v3",
	"standard_nc64as_t4_v3",
	// A100 40GB: https://learn.microsoft.com/en-us/azure/virtual-machines/nda100-v4-series
	"standard_nd96asr_v4",
	"standard_nd112asr_a100_v4",
	"standard_nd120asr_a100_v4",
	// A100 80GB: https://learn.microsoft.com/en-us/azure/virtual-machines/ndm-a100-v4-series
	"standard_nd96amsr_a100_v4",
	"standard_nd112amsr_a100_v4",
	"standard_nd120amsr_a100_v4",
	// A100 PCIE 80GB: https://learn.microsoft.com/en-us/azure/virtual-machines/nc-a100-v4-series
	"standard_nc24ads_a100_v4",
	"standard_nc48ads_a100_v4",
	"standard_nc96ads_a100_v4",
	// A10
	"standard_nc8ads_a10_v4",
	"standard_nc16ads_a10_v4",
	"standard_nc32ads_a10_v4",
	// A10, GRID only: https://learn.microsoft.com/en-us/azure/virtual-machines/nva10v5-series
	"standard_nv6ads_a10_v5",
	"standard_nv12ads_a10_v5",
	"standard_nv18ads_a10_v5",
	"standard_nv36ads_a10_v5",
	"standard_nv36adms_a10_v5",
	"standard_nv72ads_a10_v5",
	// A100
	"standard_nd96ams_v4",
	"standard_nd96ams_a100_v4",
	"standard_nd96amsrf_a100_v4",
	"standard_nd96amsf_a100_v4",
	// H100: https://learn.microsoft.com/en-us/azure/virtual-machines/nd-h100-v5-series
	"standard_nd96isr_h100_v5",
	"standard_nd96is_h100_v5",
	"standard_nd96isrf_h100_v5",
	"standard_nd96isf_h100_v5",
	// NC: https://learn.microsoft.com/en-us/azure/virtual-machines/ncads-h100-v5
	"standard_nc40ads_h100_v5",
	"standard_nc80adis_h100_v5",
)

/*
	marinerNvidiaEnabledSKUs :  List of GPU SKUs currently enabled and validated for Mariner. Will expand the support
	to cover other SKUs available in Azure.
*/
//nolint:gochecknoglobals
var marinerNvidiaEnabledSKUs = sets.NewString(
	// V100
	"standard_nc6s_v3",
	"standard_nc12s_v3",
	"standard_nc24s_v3",
	"standard_nc24rs_v3",
	"standard_nd40s_v3",
	"standard_nd40rs_v2",
	// T4
	"standard_nc4as_t4_v3",
	"standard_nc8as_t4_v3",
	"standard_nc16as_t4_v3",
	"standard_nc64as_t4_v3",
)

/* convergedGPUDriverSizes : these sizes use a "converged" driver to support both cuda/grid workloads.
how do you figure this out? ask HPC or find out by trial and error.
installing vanilla cuda drivers will fail to install with opaque errors.
nvidia-bug-report.sh may be helpful, but usually it tells you the pci card id is incompatible.
That sends me to HPC folks.
see https://github.com/Azure/azhpc-extensions/blob/daaefd78df6f27012caf30f3b54c3bd6dc437652/NvidiaGPU/resources.json
*/
//nolint:gochecknoglobals
var convergedGPUDriverSizes = sets.NewString(
	"standard_nv6ads_a10_v5",
	"standard_nv12ads_a10_v5",
	"standard_nv18ads_a10_v5",
	"standard_nv36ads_a10_v5",
	"standard_nv72ads_a10_v5",
	"standard_nv36adms_a10_v5",
	"standard_nc8ads_a10_v4",
	"standard_nc16ads_a10_v4",
	"standard_nc32ads_a10_v4",
)

/*
fabricManagerGPUSizes list should be updated as needed if AKS supports
new MIG-capable skus which require fabricmanager for nvlink training.
Specifically, the 8-board VM sizes (ND96 and larger).
Check with HPC or SKU API folks if we can improve this...
*/
//nolint:gochecknoglobals
var fabricManagerGPUSizes = sets.NewString(
	// A100
	"standard_nd96asr_v4",
	"standard_nd112asr_a100_v4",
	"standard_nd120asr_a100_v4",
	"standard_nd96amsr_a100_v4",
	"standard_nd112amsr_a100_v4",
	"standard_nd120amsr_a100_v4",
	// TODO(ace): one of these is probably dupe...
	// confirm with HPC/SKU owners.
	"standard_nd96ams_a100_v4",
	"standard_nd96ams_v4",
	// H100.
	"standard_nd46s_h100_v5",
	"standard_nd48s_h100_v5",
	"standard_nd50s_h100_v5",
	"standard_nd92is_h100_v5",
	"standard_nd96is_h100_v5",
	"standard_nd100is_h100_v5",
	"standard_nd92isr_h100_v5",
	"standard_nd96isr_h100_v5",
	"standard_nd100isr_h100_v5",
)

// IsNvidiaEnabledSKU determines if an VM SKU has nvidia driver support.
func IsNvidiaEnabledSKU(vmSize string) bool {
	// Trim the optional _Promo suffix.
	vmSize = strings.ToLower(vmSize)
	vmSize = strings.TrimSuffix(vmSize, "_promo")
	return nvidiaEnabledSKUs.Has(vmSize)
}

// IsMarinerNvidiaEnabledSKU determines if an Mariner VM SKU has nvidia driver support.
func IsMarinerEnabledGPUSKU(vmSize string) bool {
	// Trim the optional _Promo suffix.
	vmSize = strings.ToLower(vmSize)
	vmSize = strings.TrimSuffix(vmSize, "_promo")
	return marinerNvidiaEnabledSKUs.Has(vmSize)
}

func UseWindowsCudaGPUDriver(vmSize string) bool {
	lowerVMSize := strings.ToLower(vmSize)
	return strings.Contains(lowerVMSize, "_nc") || strings.Contains(lowerVMSize, "_nd")
}

func UseWindowsGridGPUDriver(vmSize string) bool {
	lowerVMSize := strings.ToLower(vmSize)
	return strings.Contains(lowerVMSize, "_nv")
}

// Below GPU helper functions are used for populating cse_cmd,
// may move these to a separate package specifically for bootstrapping for self-contained later on.

// IsMIGNode check if the node should be partitioned.
func IsMIGNode(gpuInstanceProfile string) bool {
	return gpuInstanceProfile != ""
}

func GetAKSGPUImageSHA(size string) string {
	if useGridDrivers(size) {
		return aksGPUGridSHA
	}
	return aksGPUCudaSHA
}

func GPUNeedsFabricManager(size string) bool {
	return fabricManagerGPUSizes.Has(strings.ToLower(size))
}

func GetCommaSeparatedGPUSizes() string {
	return strings.Join(nvidiaEnabledSKUs.List(), ",")
}

func GetCommaSeparatedMarinerGPUSizes() string {
	return strings.Join(nvidiaEnabledSKUs.List(), ",")
}

// NV series GPUs target graphics workloads vs NC which targets compute.
// they typically use GRID, not CUDA drivers, and will fail to install CUDA drivers.
// NVv1 seems to run with CUDA, NVv5 requires GRID.
// NVv3 is untested on AKS, NVv4 is AMD so n/a, and NVv2 no longer seems to exist (?).
func GetGPUDriverVersion(size string) string {
	if useGridDrivers(size) {
		return nvidia535GridDriverVersion
	}
	if isStandardNCv1(size) {
		return nvidia470CudaDriverVersion
	}
	return nvidia550CudaDriverVersion
}

func isStandardNCv1(size string) bool {
	tmp := strings.ToLower(size)
	return strings.HasPrefix(tmp, "standard_nc") && !strings.Contains(tmp, "_v")
}

func useGridDrivers(size string) bool {
	return convergedGPUDriverSizes.Has(strings.ToLower(size))
}
