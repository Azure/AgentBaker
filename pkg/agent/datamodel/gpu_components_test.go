// pkg/agent/datamodel/config_test.go
package datamodel

import (
	"regexp"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// The configuration is loaded during package initialization
	if NvidiaCudaDriverVersion == "" {
		t.Error("NvidiaCudaDriverVersion is empty")
	}
	if NvidiaGridDriverVersion == "" {
		t.Error("NvidiaGridDriverVersion is empty")
	}

	if NvidiaGridV20DriverVersion == "" {
		t.Error("NvidiaGridV20DriverVersion is empty")
	}

	if AKSGPUCudaVersionSuffix == "" {
		t.Error("NvidiaCudaDriverVersion is empty")
	}

	if AKSGPUGridVersionSuffix == "" {
		t.Error("AKSGPUGridVersionSuffix is empty")
	}

	if AKSGPUGridV20VersionSuffix == "" {
		t.Error("AKSGPUGridV20VersionSuffix is empty")
	}

	// Define regular expressions for expected formats
	versionRegex := `^\d+\.\d+\.\d+$` // match version strings in a format like "X.Y.Z", where each of X, Y, and Z are numbers. e.g., "550.90.12"
	suffixRegex := `^\d{14}$`         //  match a string of exactly 14 digits, which can represent a timestamp e.g., "20241021235610"

	// Compile the regular expressions
	versionPattern := regexp.MustCompile(versionRegex)
	suffixPattern := regexp.MustCompile(suffixRegex)

	// Test NvidiaCudaDriverVersion and other variables' format
	if !versionPattern.MatchString(NvidiaCudaDriverVersion) {
		t.Errorf("NvidiaCudaDriverVersion '%s' does not match expected format", NvidiaCudaDriverVersion)
	}

	if !versionPattern.MatchString(NvidiaGridDriverVersion) {
		t.Errorf("NvidiaGridDriverVersion '%s' does not match expected format", NvidiaGridDriverVersion)
	}

	if !versionPattern.MatchString(NvidiaGridV20DriverVersion) {
		t.Errorf("NvidiaGridV20DriverVersion '%s' does not match expected format", NvidiaGridV20DriverVersion)
	}

	if !suffixPattern.MatchString(AKSGPUCudaVersionSuffix) {
		t.Errorf("AKSGPUCudaVersionSuffix '%s' does not match expected format", AKSGPUCudaVersionSuffix)
	}

	if !suffixPattern.MatchString(AKSGPUGridVersionSuffix) {
		t.Errorf("AKSGPUGridVersionSuffix '%s' does not match expected format", AKSGPUGridVersionSuffix)
	}

	if !suffixPattern.MatchString(AKSGPUGridV20VersionSuffix) {
		t.Errorf("AKSGPUGridV20VersionSuffix '%s' does not match expected format", AKSGPUGridV20VersionSuffix)
	}
}

// TestGPUImageRepo verifies that the bare repo name is extracted via exact final
// path segment (tag stripped), so prefix-sharing repos like "aks-gpu-grid" and
// "aks-gpu-grid-v20" are never confused by substring matching. This guards the
// LoadConfig switch that maps each repo to its own driver version/suffix.
func TestGPUImageRepo(t *testing.T) {
	cases := map[string]string{
		"mcr.microsoft.com/aks/aks-gpu-cuda:*":               "aks-gpu-cuda",
		"mcr.microsoft.com/aks/aks-gpu-grid:*":               "aks-gpu-grid",
		"mcr.microsoft.com/aks/aks-gpu-grid-v20:*":           "aks-gpu-grid-v20",
		"mcr.microsoft.com/aks/aks-gpu-grid-v20:595.58.03-1": "aks-gpu-grid-v20",
		"aks-gpu-grid-v20":                                   "aks-gpu-grid-v20",
	}
	for downloadURL, want := range cases {
		if got := gpuImageRepo(downloadURL); got != want {
			t.Errorf("gpuImageRepo(%q) = %q, want %q", downloadURL, got, want)
		}
	}

	// "aks-gpu-grid-v20" must not be parsed as "aks-gpu-grid" (substring collision).
	if gpuImageRepo("mcr.microsoft.com/aks/aks-gpu-grid-v20:*") == "aks-gpu-grid" {
		t.Error("aks-gpu-grid-v20 URL was incorrectly parsed as aks-gpu-grid")
	}
}
