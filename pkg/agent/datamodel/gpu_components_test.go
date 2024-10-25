// pkg/agent/datamodel/config_test.go
package datamodel

import (
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
}
