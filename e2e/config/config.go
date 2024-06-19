package config

import (
	"os"
	"strings"

	"github.com/Azure/agentbakere2e/toolkit"
)

var (
	BuildID            = envDefault(os.Getenv("BUILD_ID"), "local")
	VHDBuildID         = os.Getenv("VHD_BUILD_ID")
	SubscriptionID     = envDefault("SUBSCRIPTION_ID", "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8")
	Location           = envDefault("LOCATION", "eastus")
	ResourceGroupName  = "abe2e-" + Location
	ScenariosToRun     = envmap("SCENARIOS_TO_RUN")
	ScenariosToExclude = envmap("SCENARIOS_TO_EXCLUDE")
	KeepVMSS           = strings.EqualFold(os.Getenv("KEEP_VMSS"), "true")
	Azure              = MustNewAzureClient(SubscriptionID)
)

func envDefault(env string, defaultValue string) string {
	val := os.Getenv(env)
	if val == "" {
		return defaultValue
	}
	return val
}

func envmap(env string) map[string]bool {
	envVal := os.Getenv(env)
	if envVal == "" {
		return nil
	}
	return toolkit.StrToBoolMap(envVal)
}
