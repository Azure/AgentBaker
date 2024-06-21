package config

import (
	"os"
	"strings"

	"github.com/Azure/agentbakere2e/toolkit"
)

var (
	BuildID            = envDefault(os.Getenv("BUILD_ID"), "local")
	SubscriptionID     = envDefault("SUBSCRIPTION_ID", "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8")
	Location           = envDefault("LOCATION", "eastus")
	ResourceGroupName  = "abe2e-" + Location
	ScenariosToRun     = envmap("SCENARIOS_TO_RUN")
	ScenariosToExclude = envmap("SCENARIOS_TO_EXCLUDE")
	KeepVMSS           = strings.EqualFold(os.Getenv("KEEP_VMSS"), "true")
	Azure              = MustNewAzureClient(SubscriptionID)
	// ADO tags every SIG image version with `branch` tag. By specifying `branch=refs/heads/master` we load latest image version from the master branch.
	SIGVersionTagName             = envDefault("SIG_VERSION_TAG_NAME", "branch")
	SIGVersionTagValue            = envDefault("SIG_VERSION_TAG_VALUE", "refs/heads/master")
	IgnoreScenariosWithMissingVHD = strings.EqualFold(os.Getenv("IGNORE_SCENARIOS_WITH_MISSING_VHD"), "true")
)

func envDefault(env string, defaultValue string) string {
	val, ok := os.LookupEnv(env)
	if !ok {
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
