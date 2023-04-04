package api

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

	baker, err := agent.NewAgentBaker()
	if err != nil {
		return -1
	}

	_, err = baker.GetNodeBootstrapping(&config)
	if err != nil {
		return -1
	}

	return 1
}
