package e2e_test

import (
	"fmt"
	"os"
)

type suiteConfig struct {
	subscription      string
	location          string
	resourceGroupName string
	scenariosToRun    map[string]bool
}

func newSuiteConfig() (*suiteConfig, error) {
	var environment = map[string]string{
		"SUBSCRIPTION_ID":     "",
		"LOCATION":            "",
		"RESOURCE_GROUP_NAME": "",
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
		scenariosToRun:    strToBoolMap(os.Getenv("SCENARIOS_TO_RUN")),
	}, nil
}
