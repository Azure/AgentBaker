package e2e_test

import (
	"fmt"
	"os"
)

type suiteConfig struct {
	subscription       string
	location           string
	resourceGroupName  string
	scenariosToRun     map[string]bool
	scenariosToExclude map[string]bool
	keepVMSS           bool
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

	config := &suiteConfig{
		subscription:      environment["SUBSCRIPTION_ID"],
		location:          environment["LOCATION"],
		resourceGroupName: environment["RESOURCE_GROUP_NAME"],
		scenariosToRun:    strToBoolMap(os.Getenv("SCENARIOS_TO_RUN")),
		keepVMSS:          os.Getenv("KEEP_VMSS") == "true",
	}

	include := os.Getenv("SCENARIOS_TO_RUN")
	exclude := os.Getenv("SCENARIOS_TO_EXCLUDE")

	// enforce SCENARIOS_TO_RUN over SCENARIOS_TO_EXCLUDE
	if include != "" {
		config.scenariosToRun = strToBoolMap(include)
	} else if exclude != "" {
		config.scenariosToExclude = strToBoolMap(exclude)
	}

	return config, nil
}
