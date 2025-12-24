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

	if AKSGPUCudaVersionSuffix == "" {
		t.Error("NvidiaCudaDriverVersion is empty")
	}

	if AKSGPUGridVersionSuffix == "" {
		t.Error(("AKSGPUGridVersionSuffix is empty"))
	}

	if MaiaNpuDriverVersion == "" {
		t.Error("MaiaNpuDriverVersion is empty")
	}

	if AKSNPUMaiaVersionSuffix == "" {
		t.Error("AKSNPUMaiaVersionSuffix is empty")
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

	if !suffixPattern.MatchString(AKSGPUCudaVersionSuffix) {
		t.Errorf("AKSGPUCudaVersionSuffix '%s' does not match expected format", AKSGPUCudaVersionSuffix)
	}

	if !suffixPattern.MatchString(AKSGPUGridVersionSuffix) {
		t.Errorf("AKSGPUGridVersionSuffix '%s' does not match expected format", AKSGPUGridVersionSuffix)
	}

	if !versionPattern.MatchString(MaiaNpuDriverVersion) {
		t.Errorf("MaiaNpuDriverVersion '%s' does not match expected format", MaiaNpuDriverVersion)
	}

	if !suffixPattern.MatchString(AKSNPUMaiaVersionSuffix) {
		t.Errorf("AKSNPUMaiaVersionSuffix '%s' does not match expected format", AKSNPUMaiaVersionSuffix)
	}
}

func TestMaiaNPUDriverSizesExist(t *testing.T) {
	if len(MaiaNPUDriverSizes) == 0 {
		t.Error("MaiaNPUDriverSizes map is empty, expected at least one MAIA SKU")
	}
}
