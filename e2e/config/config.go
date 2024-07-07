package config

import (
	"os"
	"strings"
)

var (
	BuildID           = getEnvWithDefaultIfEmpty(os.Getenv("BUILD_ID"), "local")
	SubscriptionID    = getEnvWithDefaultIfEmpty("SUBSCRIPTION_ID", "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8")
	Location          = getEnvWithDefaultIfEmpty("LOCATION", "eastus")
	ResourceGroupName = "abe2e-" + Location
	TagsToRun         = os.Getenv("TAGS_TO_RUN")
	TagsToSkip        = os.Getenv("TAGS_TO_SKIP")
	KeepVMSS          = strings.EqualFold(os.Getenv("KEEP_VMSS"), "true")
	Azure             = MustNewAzureClient(SubscriptionID)
	// ADO tags every SIG image version with `branch` tag. By specifying `branch=refs/heads/master` we load latest image version from the master branch.
	SIGVersionTagName             = getEnvWithDefaultIfEmpty("SIG_VERSION_TAG_NAME", "branch")
	SIGVersionTagValue            = getEnvWithDefaultIfEmpty("SIG_VERSION_TAG_VALUE", "refs/heads/master")
	IgnoreScenariosWithMissingVHD = strings.EqualFold(os.Getenv("IGNORE_SCENARIOS_WITH_MISSING_VHD"), "true")
)

func getEnvWithDefaultIfEmpty(env string, defaultValue string) string {
	val, ok := os.LookupEnv(env)
	if !ok {
		return defaultValue
	}
	return val
}
