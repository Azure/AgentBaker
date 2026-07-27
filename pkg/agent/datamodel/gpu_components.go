package datamodel

import (
	"fmt"

	"github.com/Azure/agentbaker/aks-node-controller/pkg/gpu"
	"github.com/Azure/agentbaker/parts"
)

// LoadGPUConfig reads the embedded components.json and parses it into a GPUConfiguration.
// Callers (e.g. NewAgentBaker) must call this explicitly and propagate any error, rather
// than relying on a package-level init() that panics: the embedded file is static and
// controlled by this repo, so a failure here indicates a real bug that should surface as
// an explicit, handleable error instead of crashing the whole process.
func LoadGPUConfig() (*gpu.GPUConfiguration, error) {
	data, err := parts.Templates.ReadFile("common/components.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read components.json: %w", err)
	}
	gpuConfig, err := gpu.LoadConfig(data)
	if err != nil {
		return nil, fmt.Errorf("failed to load GPU configuration: %w", err)
	}
	return gpuConfig, nil
}
