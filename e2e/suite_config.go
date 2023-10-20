package e2e_test

import (
	"fmt"
	"os"
	"strings"
)

const (
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

func (c suiteConfig) String() string {
	return fmt.Sprintf(suiteConfigStringTemplate, c.subscription, c.location, c.resourceGroupName, c.keepVMSS)
}

func newSuiteConfig() (*suiteConfig, error) {
	// required environment variables
	var environment = map[string]string{
		"SUBSCRIPTION_ID": "",
		"LOCATION":        "",
	}

	for k := range environment {
		value := os.Getenv(k)
		if value == "" {
			return nil, fmt.Errorf("missing required environment variable %q", k)
		}
		environment[k] = value
	}

	config := &suiteConfig{
		subscription: environment["SUBSCRIPTION_ID"],
		location:     environment["LOCATION"],
		keepVMSS:     strings.EqualFold(os.Getenv("KEEP_VMSS"), "true"),
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
