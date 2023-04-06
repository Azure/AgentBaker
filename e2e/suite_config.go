package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

type suiteConfig struct {
	subscription      string
	location          string
	resourceGroupName string
	clusterName       string
	testsToRun        map[string]bool
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
		testsToRun:        map[string]bool{"base": true},
	}, nil
}

type scenarioConfig struct {
	// bootstrapConfig          *datamodel.NodeBootstrappingConfiguration
	bootstrapConfigMutator func(*datamodel.NodeBootstrappingConfiguration)
	vmConfigMutator        func(*armcompute.VirtualMachineScaleSet)
	validator              func(context.Context, *scenarioValidationInput) error
}

type scenarioValidationInput struct {
	privateIP     string
	sshPrivateKey string
}

func parseTestNames(testNames string) map[string]bool {
	testNames = strings.ReplaceAll(testNames, " ", "")

	if testNames == "" {
		return nil
	}

	testParts := strings.SplitN(testNames, ",", -1)

	tests := make(map[string]bool, len(testParts))

	for _, tp := range testParts {
		tests[tp] = true
	}

	return tests
}
