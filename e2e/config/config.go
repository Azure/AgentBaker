package config

import (
	"os"
	"strings"

	"github.com/Azure/agentbakere2e/toolkit"
)

var (
	BuildID            = os.Getenv("BUILD_ID")
	Subscription       = mustenv("SUBSCRIPTION_ID")
	Location           = mustenv("LOCATION")
	ResourceGroupName  = "abe2e-" + mustenv("LOCATION")
	PAT                = os.Getenv("ADO_PAT")
	ScenariosToRun     = envmap("SCENARIOS_TO_RUN")
	ScenariosToExclude = envmap("SCENARIOS_TO_EXCLUDE")
	KeepVMSS           = strings.EqualFold(os.Getenv("KEEP_VMSS"), "true")
)

func mustenv(env string) string {
	result := os.Getenv(env)
	if result == "" {
		panic("missing environment variable: " + env)
	}
	return result
}

func envmap(env string) map[string]bool {
	envVal := os.Getenv(env)
	if envVal == "" {
		return nil
	}
	return toolkit.StrToBoolMap(envVal)
}
