package datamodel

import (
	"regexp"
	"testing"
)

func TestLoadGPUConfig(t *testing.T) {
	config, err := LoadGPUConfig()
	if err != nil {
		t.Fatalf("LoadGPUConfig failed: %v", err)
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	suffixPattern := regexp.MustCompile(`^\d{14}$`)

	checks := []struct {
		name  string
		value string
		re    *regexp.Regexp
	}{
		{"NvidiaCudaDriverVersion", config.NvidiaCudaDriverVersion, versionPattern},
		{"NvidiaCudaLTSDriverVersion", config.NvidiaCudaLTSDriverVersion, versionPattern},
		{"NvidiaGridDriverVersion", config.NvidiaGridDriverVersion, versionPattern},
		{"NvidiaGridV20DriverVersion", config.NvidiaGridV20DriverVersion, versionPattern},
		{"AKSGPUCudaVersionSuffix", config.AKSGPUCudaVersionSuffix, suffixPattern},
		{"AKSGPUCudaLTSVersionSuffix", config.AKSGPUCudaLTSVersionSuffix, suffixPattern},
		{"AKSGPUGridVersionSuffix", config.AKSGPUGridVersionSuffix, suffixPattern},
		{"AKSGPUGridV20VersionSuffix", config.AKSGPUGridV20VersionSuffix, suffixPattern},
	}

	for _, c := range checks {
		if c.value == "" {
			t.Errorf("%s is empty", c.name)
		} else if !c.re.MatchString(c.value) {
			t.Errorf("%s = %q does not match expected format %s", c.name, c.value, c.re.String())
		}
	}
}
