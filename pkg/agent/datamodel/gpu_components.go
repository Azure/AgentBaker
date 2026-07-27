package datamodel

import (
	"fmt"

	"github.com/Azure/agentbaker/aks-node-controller/pkg/gpu"
	"github.com/Azure/agentbaker/parts"
)

//nolint:gochecknoinits
func init() {
	data, err := parts.Templates.ReadFile("common/components.json")
	if err != nil {
		panic(fmt.Sprintf("Failed to read components.json: %v", err))
	}
	if err := gpu.LoadGPUConfig(data); err != nil {
		panic(fmt.Sprintf("Failed to load GPU configuration: %v", err))
	}
}
