package gpu

import (
	"testing"
)

const testComponentsJSON = `{
	"GPUContainerImages": [
		{
			"downloadURL": "mcr.microsoft.com/aks/aks-gpu-cuda-lts:580.11.07-20240101120000",
			"gpuVersion": {
				"renovateTag": "name=aks-gpu-cuda-lts",
				"latestVersion": "580.11.07-20240101120000"
			}
		},
		{
			"downloadURL": "mcr.microsoft.com/aks/aks-gpu-cuda:535.54.03-20240201130000",
			"gpuVersion": {
				"renovateTag": "name=aks-gpu-cuda",
				"latestVersion": "535.54.03-20240201130000"
			}
		},
		{
			"downloadURL": "mcr.microsoft.com/aks/aks-gpu-grid:550.90.12-20240301140000",
			"gpuVersion": {
				"renovateTag": "name=aks-gpu-grid",
				"latestVersion": "550.90.12-20240301140000"
			}
		},
		{
			"downloadURL": "mcr.microsoft.com/aks/aks-gpu-grid-v20:595.58.03-20240401150000",
			"gpuVersion": {
				"renovateTag": "name=aks-gpu-grid-v20",
				"latestVersion": "595.58.03-20240401150000"
			}
		}
	]
}`

func TestLoadConfig(t *testing.T) {
	config, err := LoadConfig([]byte(testComponentsJSON))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"NvidiaCudaLTSDriverVersion", config.NvidiaCudaLTSDriverVersion, "580.11.07"},
		{"AKSGPUCudaLTSVersionSuffix", config.AKSGPUCudaLTSVersionSuffix, "20240101120000"},
		{"NvidiaCudaDriverVersion", config.NvidiaCudaDriverVersion, "535.54.03"},
		{"AKSGPUCudaVersionSuffix", config.AKSGPUCudaVersionSuffix, "20240201130000"},
		{"NvidiaGridDriverVersion", config.NvidiaGridDriverVersion, "550.90.12"},
		{"AKSGPUGridVersionSuffix", config.AKSGPUGridVersionSuffix, "20240301140000"},
		{"NvidiaGridV20DriverVersion", config.NvidiaGridV20DriverVersion, "595.58.03"},
		{"AKSGPUGridV20VersionSuffix", config.AKSGPUGridV20VersionSuffix, "20240401150000"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	_, err := LoadConfig([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadConfig_SkipsMalformedVersions(t *testing.T) {
	json := `{"GPUContainerImages": [{"downloadURL": "mcr.microsoft.com/aks/aks-gpu-cuda-lts:nodash", "gpuVersion": {"latestVersion": "nodash"}}]}`
	config, err := LoadConfig([]byte(json))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if config.NvidiaCudaLTSDriverVersion != "" {
		t.Errorf("expected empty, got %q", config.NvidiaCudaLTSDriverVersion)
	}
}

func testGPUConfig(t *testing.T) *GPUConfiguration {
	t.Helper()
	config, err := LoadConfig([]byte(testComponentsJSON))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	return config
}

func TestGetGPUDriverVersion(t *testing.T) {
	config := testGPUConfig(t)

	tests := []struct {
		size string
		want string
	}{
		// NCv1 (legacy K80) → pinned 470 driver
		{"standard_nc6", Nvidia470CudaDriverVersion},
		{"Standard_NC12", Nvidia470CudaDriverVersion},
		// Converged GRID SKUs
		{"standard_nv6ads_a10_v5", "550.90.12"},
		{"Standard_NV36adms_A10_V5", "550.90.12"},
		// RTX PRO 6000 → grid-v20
		{"standard_nc144ds_xl_rtxpro6000bse_v6", "595.58.03"},
		{"Standard_NC288lds_xl_RTXPRO6000BSE_v6", "595.58.03"},
		// Default CUDA LTS
		{"standard_nc6_v3", "580.11.07"},
		{"Standard_ND96asr_v4", "580.11.07"},
	}
	for _, tt := range tests {
		if got := config.GetGPUDriverVersion(tt.size); got != tt.want {
			t.Errorf("GetGPUDriverVersion(%q) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func TestGetGPUDriverVersion_NilReceiver(t *testing.T) {
	var config *GPUConfiguration

	tests := []struct {
		size string
		want string
	}{
		// NCv1 is a pinned constant, unaffected by a nil/unloaded config.
		{"standard_nc6", Nvidia470CudaDriverVersion},
		// Anything sourced from components.json is empty when unloaded, not a panic.
		{"standard_nv6ads_a10_v5", ""},
		{"standard_nc144ds_xl_rtxpro6000bse_v6", ""},
		{"standard_nc6_v3", ""},
	}
	for _, tt := range tests {
		if got := config.GetGPUDriverVersion(tt.size); got != tt.want {
			t.Errorf("nil.GetGPUDriverVersion(%q) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func TestGetGPUDriverType(t *testing.T) {
	tests := []struct {
		size string
		want string
	}{
		{"standard_nc6", "cuda"},
		{"standard_nc6_v3", "cuda-lts"},
		{"standard_nv6ads_a10_v5", "grid"},
		{"standard_nc144ds_xl_rtxpro6000bse_v6", "grid-v20"},
	}
	for _, tt := range tests {
		if got := GetGPUDriverType(tt.size); got != tt.want {
			t.Errorf("GetGPUDriverType(%q) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func TestGetAKSGPUImageSHA(t *testing.T) {
	config := testGPUConfig(t)

	tests := []struct {
		size string
		want string
	}{
		{"standard_nc144ds_xl_rtxpro6000bse_v6", "20240401150000"},
		{"standard_nv6ads_a10_v5", "20240301140000"},
		{"standard_nc6_v3", "20240101120000"},
	}
	for _, tt := range tests {
		if got := config.GetAKSGPUImageSHA(tt.size); got != tt.want {
			t.Errorf("GetAKSGPUImageSHA(%q) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func TestGetAKSGPUImageSHA_NilReceiver(t *testing.T) {
	var config *GPUConfiguration
	if got := config.GetAKSGPUImageSHA("standard_nc6_v3"); got != "" {
		t.Errorf("nil.GetAKSGPUImageSHA(...) = %q, want empty string", got)
	}
}

func TestGPUNeedsFabricManager(t *testing.T) {
	tests := []struct {
		size string
		want bool
	}{
		{"standard_nd96asr_v4", true},
		{"standard_nd96is_h100_v5", true},
		{"standard_nd96isr_h200_v5", true},
		{"standard_nc24ads_a100_v4", false}, // oddball: returns false
		{"standard_nc6_v3", false},          // not in map
	}
	for _, tt := range tests {
		if got := GPUNeedsFabricManager(tt.size); got != tt.want {
			t.Errorf("GPUNeedsFabricManager(%q) = %v, want %v", tt.size, got, tt.want)
		}
	}
}

func TestGpuImageRepo(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"mcr.microsoft.com/aks/aks-gpu-cuda-lts:580.11.07-20240101", "aks-gpu-cuda-lts"},
		{"mcr.microsoft.com/aks/aks-gpu-grid:*", "aks-gpu-grid"},
		{"mcr.microsoft.com/aks/aks-gpu-grid-v20:595.58.03-1", "aks-gpu-grid-v20"},
		{"aks-gpu-grid-v20", "aks-gpu-grid-v20"},
	}
	for _, tt := range tests {
		if got := gpuImageRepo(tt.url); got != tt.want {
			t.Errorf("gpuImageRepo(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
