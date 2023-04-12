package e2e_test

import (
	"context"
	"encoding/json"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"log"
	"os/exec"
)

type getNodeBootstrappingFn func(ctx context.Context, nbc *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error)

func getNodeBootstrapping(e2eMode string) getNodeBootstrappingFn {
	switch e2eMode {
	case "coverage":
		return getNodeBootstrappingForCoverage
	default:
		return getNodeBootstrappingForValidation
	}
}

func getNodeBootstrappingForCoverage(ctx context.Context, nbc *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error) {
	payload, err := json.Marshal(nbc)
	if err != nil {
		log.Fatalf("failed to marshal nbc, error: %s", err)
	}
	cmd := exec.Command("curl", "-X", "POST", "-d", string(payload), "localhost:8080/getnodebootstrapdata")
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("failed to retrieve node bootstrapping data, error: %s", err)
	}
	var nodeBootstrapping *datamodel.NodeBootstrapping
	err = json.Unmarshal(output, &nodeBootstrapping)
	if err != nil {
		log.Fatalf("failed to unmarshal node bootstrapping data, error: %s", err)
	}
	return nodeBootstrapping, nil
}

func getNodeBootstrappingForValidation(ctx context.Context, nbc *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error) {
	ab, err := agent.NewAgentBaker()
	if err != nil {
		return nil, err
	}
	nodeBootstrapping, err := ab.GetNodeBootstrapping(ctx, nbc)
	if err != nil {
		return nil, err
	}
	return nodeBootstrapping, nil
}
