package api

import (
	"context"
	"encoding/json"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	agentoverrides "github.com/Azure/agentbaker/pkg/agent/overrides"
)

func Fuzz(data []byte) int {
	var config datamodel.NodeBootstrappingConfiguration

	if err := json.Unmarshal(data, &config); err != nil {
		return -1
	}

	overrides := agentoverrides.NewOverrides()
	baker, err := agent.NewAgentBaker(overrides)
	if err != nil {
		return -1
	}

	_, err = baker.GetNodeBootstrapping(context.Background(), &config)
	if err != nil {
		return -1
	}

	return 1
}
