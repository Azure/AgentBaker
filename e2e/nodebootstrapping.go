package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"log"
	"net/http"
)

type nodeBootstrappingFn func(ctx context.Context, nbc *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error)

func getNodeBootstrappingFn(e2eMode string) nodeBootstrappingFn {
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
	res, err := http.Post("http://localhost:8080/getnodebootstrapdata", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalf("failed to retrieve node bootstrapping data, error: %s", err)
	}

	if res.StatusCode != http.StatusOK {
		log.Fatalf("failed to retrieve node bootstrapping data, status code: %d", res.StatusCode)
	}

	var nodeBootstrapping *datamodel.NodeBootstrapping
	err = json.NewDecoder(res.Body).Decode(&nodeBootstrapping)
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
