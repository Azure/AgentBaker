package datamodel

import (
	"regexp"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/pkg/gpu"
)

func TestGPUConfigLoaded(t *testing.T) {
	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	suffixPattern := regexp.MustCompile(`^\d{14}$`)

	checks := []struct {
		name  string
		value string
		re    *regexp.Regexp
	}{
		{"NvidiaCudaDriverVersion", gpu.NvidiaCudaDriverVersion, versionPattern},
		{"NvidiaCudaLTSDriverVersion", gpu.NvidiaCudaLTSDriverVersion, versionPattern},
		{"NvidiaGridDriverVersion", gpu.NvidiaGridDriverVersion, versionPattern},
		{"NvidiaGridV20DriverVersion", gpu.NvidiaGridV20DriverVersion, versionPattern},
		{"AKSGPUCudaVersionSuffix", gpu.AKSGPUCudaVersionSuffix, suffixPattern},
		{"AKSGPUCudaLTSVersionSuffix", gpu.AKSGPUCudaLTSVersionSuffix, suffixPattern},
		{"AKSGPUGridVersionSuffix", gpu.AKSGPUGridVersionSuffix, suffixPattern},
		{"AKSGPUGridV20VersionSuffix", gpu.AKSGPUGridV20VersionSuffix, suffixPattern},
	}

	for _, c := range checks {
		if c.value == "" {
			t.Errorf("%s is empty", c.name)
		} else if !c.re.MatchString(c.value) {
			t.Errorf("%s = %q does not match expected format %s", c.name, c.value, c.re.String())
		}
	}
}
