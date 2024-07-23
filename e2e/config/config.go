package config

import (
	"os"
	"strings"
	"time"
)

var (
	Location                      = lookupEnvWithDefaultString("LOCATION", "westus3")
	SubscriptionID                = lookupEnvWithDefaultString("SUBSCRIPTION_ID", "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8")
	Azure                         = MustNewAzureClient(SubscriptionID)
	CustomConfig                  = NewCustomConfig(Location, SubscriptionID)
	AirgapNSGName                 = "abe2e-airgap-securityGroup"
	BuildID                       = lookupEnvWithDefaultString(os.Getenv("BUILD_ID"), "local")
	DefaultSubnetName             = "aks-subnet"
	IgnoreScenariosWithMissingVHD = lookupEnvWithDefaultBool("IGNORE_SCENARIOS_WITH_MISSING_VHD", false)
	KeepVMSS                      = lookupEnvWithDefaultBool("KEEP_VMSS", false)
	SIGVersionTagName             = lookupEnvWithDefaultString("SIG_VERSION_TAG_NAME", "branch") // ADO tags every SIG image version with `branch` tag. By specifying `branch=refs/heads/master` we load latest image version from the master branch.
	SIGVersionTagValue            = lookupEnvWithDefaultString("SIG_VERSION_TAG_VALUE", "refs/heads/master")
	SkipTestsWithSKUCapacityIssue = lookupEnvWithDefaultBool("SKIP_TESTS_WITH_SKU_CAPACITY_ISSUE", false)
	TagsToRun                     = os.Getenv("TAGS_TO_RUN")
	TagsToSkip                    = os.Getenv("TAGS_TO_SKIP")
	TestTimeout                   = lookupEnvWithDefaultDuration("TEST_TIMEOUT", 12*time.Minute)
	E2ELoggingDir                 = lookupEnvWithDefaultString("LOGGING_DIR", "scenario-logs")
)

type Config struct {
	Location          string
	SubscriptionID    string
	ResourceGroupName string
	Azure             *AzureClient
}

func NewCustomConfig(location string, subscriptionID string) *Config {
	customConfig := &Config{
		Location:       Location,
		SubscriptionID: SubscriptionID,
	}
	if location != "" {
		customConfig.Location = location
	}
	if subscriptionID != "" {
		customConfig.SubscriptionID = subscriptionID
	}
	customConfig.ResourceGroupName = "abe2e-" + customConfig.Location
	customConfig.Azure = MustNewAzureClient(customConfig.SubscriptionID)
	return customConfig
}

func lookupEnvWithDefaultString(env string, defaultValue string) string {
	val, ok := os.LookupEnv(env)
	if !ok {
		return defaultValue
	}
	return val
}

func lookupEnvWithDefaultBool(env string, defaultValue bool) bool {
	if val, ok := os.LookupEnv(env); ok {
		return strings.EqualFold(val, "true")
	}
	return defaultValue
}

func lookupEnvWithDefaultDuration(env string, defaultValue time.Duration) time.Duration {
	if val, ok := os.LookupEnv(env); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultValue
}
