package config

import (
	"log"
	"os"
	"strings"

	"github.com/Azure/agentbakere2e/toolkit"
)

var (
	BuildID            = getEnvWithDefaultIfEmpty(os.Getenv("BUILD_ID"), "local")
	SubscriptionID     = getEnvWithDefaultIfEmpty("SUBSCRIPTION_ID", "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8")
	Location           = getEnvWithDefaultIfEmpty("LOCATION", "eastus")
	ResourceGroupName  = "abe2e-" + Location
	ScenariosToRun     = envmap("SCENARIOS_TO_RUN")
	ScenariosToExclude = envmap("SCENARIOS_TO_EXCLUDE")
	KeepVMSS           = strings.EqualFold(os.Getenv("KEEP_VMSS"), "true")
	Azure              = MustNewAzureClient(SubscriptionID)
	// ADO tags every SIG image version with `branch` tag. By specifying `branch=refs/heads/master` we load latest image version from the master branch.
	SIGVersionTagName             = getEnvWithDefaultIfEmpty("SIG_VERSION_TAG_NAME", "branch")
	SIGVersionTagValue            = getEnvWithDefaultIfEmpty("SIG_VERSION_TAG_VALUE", "refs/heads/master")
	IgnoreScenariosWithMissingVHD = strings.EqualFold(os.Getenv("IGNORE_SCENARIOS_WITH_MISSING_VHD"), "true")
)

func getEnvWithDefaultIfEmpty(env string, defaultValue string) string {
	val, ok := os.LookupEnv(env)
	if !ok {
		log.Printf("could not find value for environment variable %q, will use default value: %s", env, defaultValue)
		return defaultValue
	}
	log.Printf("resolved environment variable %q to %q", env, val)
	return val
}

func envmap(env string) map[string]bool {
	envVal := os.Getenv(env)
	if envVal == "" {
		return nil
	}
	return toolkit.StrToBoolMap(envVal)
}
