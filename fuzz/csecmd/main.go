package csecmd

import (
	"encoding/json"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func Fuzz(data []byte) int {
	var config datamodel.NodeBootstrappingConfiguration

	if err := json.Unmarshal(data, &config); err != nil {
		return -1
	}

	// should not panic
	baker := agent.InitializeTemplateGenerator()
	baker.GetNodeBootstrappingCmd(&config)

	return 1
}
