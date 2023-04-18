package e2e_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

type suiteConfig struct {
	subscription      string
	location          string
	resourceGroupName string
	clusterName       string
	scenariosToRun    map[string]bool
}

func newSuiteConfig() (*suiteConfig, error) {
	var environment = map[string]string{
		"SUBSCRIPTION_ID":     "",
		"LOCATION":            "",
		"RESOURCE_GROUP_NAME": "",
		"CLUSTER_NAME":        "",
	}

	for k := range environment {
		value := os.Getenv(k)
		if value == "" {
			return nil, fmt.Errorf("missing required environment variable %q", k)
		}
		environment[k] = value
	}

	return &suiteConfig{
		subscription:      environment["SUBSCRIPTION_ID"],
		location:          environment["LOCATION"],
		resourceGroupName: environment["RESOURCE_GROUP_NAME"],
		clusterName:       environment["CLUSTER_NAME"],
		scenariosToRun:    strToBoolMap(os.Getenv("SCENARIOS_TO_RUN")),
	}, nil
}

type scenarioConfig struct {
	// bootstrapConfig          *datamodel.NodeBootstrappingConfiguration
	bootstrapConfigMutator func(*testing.T, *datamodel.NodeBootstrappingConfiguration)
	vmConfigMutator        func(*armcompute.VirtualMachineScaleSet)
	validator              func(context.Context, *testing.T, *scenarioValidationInput) error
}

type scenarioValidationInput struct {
	privateIP     string
	sshPrivateKey string
}
