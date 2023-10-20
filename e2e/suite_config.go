package e2e_test

import (
	"fmt"
	"os"
	"strings"
)

const (
	subscriptionIdEnvironmentVarName     = "SUBSCRIPTION_ID"
	locationEnvironmentVarName           = "LOCATION"
	keepVMSSEnvironmentVarName           = "KEEP_VMSS"
	scenariosToRunEnvironmentVarName     = "SCENARIOS_TO_RUN"
	scenariosToExcludeEnvironmentVarName = "SCENARIOS_TO_EXCLUDE"

	suiteConfigStringTemplate = `subscription: %[1]s,
location: %[2]s,
resource group: %[3]s,
keep vmss: %[4]t`
)

type suiteConfig struct {
	subscription       string
	location           string
	resourceGroupName  string
	scenariosToRun     map[string]bool
	scenariosToExclude map[string]bool
	keepVMSS           bool
}

func (c *suiteConfig) String() string {
	return fmt.Sprintf(suiteConfigStringTemplate, c.subscription, c.location, c.resourceGroupName, c.keepVMSS)
}

func newSuiteConfig() (*suiteConfig, error) {
	// required environment variables
	var environment = map[string]string{
		subscriptionIdEnvironmentVarName: "",
		locationEnvironmentVarName:       "",
	}

	for k := range environment {
		value := os.Getenv(k)
		if value == "" {
			return nil, fmt.Errorf("missing required environment variable %q", k)
		}
		environment[k] = value
	}

	config := &suiteConfig{
		subscription:      environment[subscriptionIdEnvironmentVarName],
		location:          environment[locationEnvironmentVarName],
		resourceGroupName: fmt.Sprintf(abe2eResourceGroupNameTemplate, environment[locationEnvironmentVarName]),
		keepVMSS:          strings.EqualFold(os.Getenv(keepVMSSEnvironmentVarName), "true"),
	}

	include := os.Getenv(scenariosToRunEnvironmentVarName)
	exclude := os.Getenv(scenariosToExcludeEnvironmentVarName)

	// enforce SCENARIOS_TO_RUN over SCENARIOS_TO_EXCLUDE
	if include != "" {
		config.scenariosToRun = strToBoolMap(include)
	} else if exclude != "" {
		config.scenariosToExclude = strToBoolMap(exclude)
	}

	return config, nil
}
