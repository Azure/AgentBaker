package v1

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

// UbuntuNvidiaEnabledSKUs - Sets of GPU SKUs with Nvidia driver support for AKS 
var UbuntuNvidiaEnabledSKUs = sets.New(
	"standard_nv6", "standard_nv12", "standard_nv12s_v3", "standard_nv24",
	"standard_nv24s_v3", "standard_nv24r", "standard_nv48s_v3",
	"standard_nd6s", "standard_nd12s", "standard_nd24s", "standard_nd24rs",
	"standard_nc6s_v2", "standard_nc12s_v2", "standard_nc24s_v2", "standard_nc24rs_v2",
	"standard_nc6s_v3", "standard_nc12s_v3", "standard_nc24s_v3", "standard_nc24rs_v3",
	"standard_nd40s_v3", "standard_nd40rs_v2",
	"standard_nc4as_t4_v3", "standard_nc8as_t4_v3", "standard_nc16as_t4_v3", "standard_nc64as_t4_v3",
	"standard_nd96asr_v4", "standard_nd112asr_a100_v4", "standard_nd120asr_a100_v4",
	"standard_nd96amsr_a100_v4", "standard_nd112amsr_a100_v4", "standard_nd120amsr_a100_v4",
	"standard_nc24ads_a100_v4", "standard_nc48ads_a100_v4", "standard_nc96ads_a100_v4", "standard_ncads_a100_v4",
	"standard_nc8ads_a10_v4", "standard_nc16ads_a10_v4", "standard_nc32ads_a10_v4",
	"standard_nv6ads_a10_v5", "standard_nv12ads_a10_v5", "standard_nv18ads_a10_v5",
	"standard_nv36ads_a10_v5", "standard_nv36adms_a10_v5", "standard_nv72ads_a10_v5",
	"standard_nd96ams_v4", "standard_nd96ams_a100_v4",
)

// AzureLinuxNvidiaEnabledSKUs - Sets of GPU SKUs with Nvidia driver support for AzureLinux
var AzureLinuxNvidiaEnabledSKUs = sets.New(
	"standard_nc6s_v3", "standard_nc12s_v3", "standard_nc24s_v3", "standard_nc24rs_v3",
	"standard_nd40s_v3", "standard_nd40rs_v2",
	"standard_nc4as_t4_v3", "standard_nc8as_t4_v3", "standard_nc16as_t4_v3", "standard_nc64as_t4_v3",
)


/* ConvergedGPUDriverSizes : these sizes use a "converged" driver to support both cuda/grid workloads.
how do you figure this out? ask HPC or find out by trial and error.
installing vanilla cuda drivers will fail to install with opaque errors.
nvidia-bug-report.sh may be helpful, but usually it tells you the pci card id is incompatible.
That sends me to HPC folks.
see https://github.com/Azure/azhpc-extensions/blob/daaefd78df6f27012caf30f3b54c3bd6dc437652/NvidiaGPU/resources.json
*/
var ConvergedGPUDriverSizes = sets.New(
	"standard_nv6ads_a10_v5", "standard_nv12ads_a10_v5", "standard_nv18ads_a10_v5",
	"standard_nv36ads_a10_v5", "standard_nv72ads_a10_v5", "standard_nv36adms_a10_v5",
	"standard_nc8ads_a10_v4", "standard_nc16ads_a10_v4", "standard_nc32ads_a10_v4",
)

func GetAKSGPUImageSHA(size string) string {
	if useGridDrivers(size) {
		return AKSGPUGridSHA
	}
	return AKSGPUCudaSHA
}

// IsNvidiaEnabledSKU determines if an VM SKU has nvidia driver support
func IsNvidiaEnabledSKU(vmSize string) bool {
	vmSize = formatSKUName(vmSize)
	return UbuntuNvidiaEnabledSKUs.Has(vmSize)
}

// IsAzureLinuxEnabledGPUSKU determines if an VM SKU has nvidia driver support for AzureLinux
func IsAzureLinuxEnabledGPUSKU(vmSize string) bool {
	vmSize = formatSKUName(vmSize)
	return AzureLinuxNvidiaEnabledSKUs.Has(vmSize)
}

func GetGPUDriverVersion(size string) string {
	if useGridDrivers(size) {
		return Nvidia535GridDriverVersion
	}
	if isStandardNCv1(size) {
		return Nvidia470CudaDriverVersion
	}
	return Nvidia535CudaDriverVersion
}

func isStandardNCv1(size string) bool {
	size = formatSKUName(size)
	return strings.HasPrefix(size, "standard_nc") && !strings.Contains(size, "_v")
}

func useGridDrivers(size string) bool {
	size = formatSKUName(size)
	return ConvergedGPUDriverSizes.Has(size)
}

func formatSKUName(size string) string {
	return strings.ToLower(strings.TrimSuffix(size, "_promo"))
}

