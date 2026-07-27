// Package gpu provides GPU SKU classification helpers for AKS node provisioning.
package gpu

import (
	"encoding/json"
	"fmt"
	"strings"
)

const Nvidia470CudaDriverVersion = "cuda-470.82.01"

//nolint:gochecknoglobals
var (
	NvidiaCudaDriverVersion    string
	NvidiaCudaLTSDriverVersion string
	NvidiaGridDriverVersion    string
	NvidiaGridV20DriverVersion string
	AKSGPUCudaVersionSuffix    string
	AKSGPUCudaLTSVersionSuffix string
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

// LoadGPUConfig parses GPU driver versions from the raw JSON content of components.json.
func LoadGPUConfig(data []byte) error {
	var config componentsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal components.json: %w", err)
	}

	const driverIndex = 0
	const suffixIndex = 1
	const expectedLength = 2

	for _, image := range config.GPUContainerImages {
		versionParts := strings.Split(image.GPUVersion.LatestVersion, "-")
		if len(versionParts) != expectedLength {
			continue
		}
		version, suffix := versionParts[driverIndex], versionParts[suffixIndex]

		switch gpuImageRepo(image.DownloadURL) {
		case "aks-gpu-cuda-lts":
			NvidiaCudaLTSDriverVersion = version
			AKSGPUCudaLTSVersionSuffix = suffix
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

// GetGPUDriverVersion returns the NVIDIA driver version string for a given VM size.
func GetGPUDriverVersion(size string) string {
	if UseGridV20Drivers(size) {
		return NvidiaGridV20DriverVersion
	}
	if UseGridDrivers(size) {
		return NvidiaGridDriverVersion
	}
	if IsStandardNCv1(size) {
		return Nvidia470CudaDriverVersion
	}
	return NvidiaCudaLTSDriverVersion
}

// IsStandardNCv1 reports whether the VM size is a legacy NCv1 (K80) SKU.
func IsStandardNCv1(size string) bool {
	tmp := strings.ToLower(size)
	return strings.HasPrefix(tmp, "standard_nc") && !strings.Contains(tmp, "_v")
}

// UseGridDrivers reports whether the VM size requires GRID drivers.
func UseGridDrivers(size string) bool {
	return ConvergedGPUDriverSizes[strings.ToLower(size)]
}

// UseGridV20Drivers reports whether the SKU needs the GRID v20 (595.x) driver.
func UseGridV20Drivers(size string) bool {
	return RTXPro6000GPUDriverSizes[strings.ToLower(size)]
}

// GetAKSGPUImageSHA returns the image version suffix for the appropriate GPU container image.
func GetAKSGPUImageSHA(size string) string {
	if UseGridV20Drivers(size) {
		return AKSGPUGridV20VersionSuffix
	}
	if UseGridDrivers(size) {
		return AKSGPUGridVersionSuffix
	}
	return AKSGPUCudaLTSVersionSuffix
}

// GetGPUDriverType maps a GPU VM size to the aks-gpu image variant.
func GetGPUDriverType(size string) string {
	if UseGridV20Drivers(size) {
		return "grid-v20"
	}
	if UseGridDrivers(size) {
		return "grid"
	}
	if IsStandardNCv1(size) {
		return "cuda"
	}
	return "cuda-lts"
}

// GPUNeedsFabricManager reports whether the VM size requires NVIDIA Fabric Manager.
func GPUNeedsFabricManager(size string) bool {
	return FabricManagerGPUSizes[strings.ToLower(size)]
}
