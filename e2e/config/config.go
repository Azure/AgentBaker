package config

import (
	"os"
	"strings"

	"github.com/Azure/agentbakere2e/toolkit"
	"github.com/joho/godotenv"
)

var (
	BuildID            string
	VHDBuildID         string
	Subscription       string
	Location           string
	ResourceGroupName  string
	ScenariosToRun     map[string]bool
	ScenariosToExclude map[string]bool
	KeepVMSS           bool
	Azure              *AzureClient
)

func init() {
	_ = godotenv.Load()
	BuildID = os.Getenv("BUILD_ID")
	VHDBuildID = os.Getenv("VHD_BUILD_ID")
	Subscription = mustenv("SUBSCRIPTION_ID")
	Location = mustenv("LOCATION")
	ResourceGroupName = "abe2e-" + Location
	ScenariosToRun = envmap("SCENARIOS_TO_RUN")
	ScenariosToExclude = envmap("SCENARIOS_TO_EXCLUDE")
	KeepVMSS = strings.EqualFold(os.Getenv("KEEP_VMSS"), "true")
	Azure = MustNewAzureClient(mustenv("SUBSCRIPTION_ID"))
}

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
