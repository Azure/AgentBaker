package config

import (
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

var (
	Config                      = mustLoadConfig()
	Azure                       = mustNewAzureClient(Config.SubscriptionID)
	ResourceGroupName           = "abe2e-" + Config.Location
	DefaultPollUntilDoneOptions = &runtime.PollUntilDoneOptions{
		Frequency: time.Second,
	}
)

type Configuration struct {
	AirgapNSGName                 string        `env:"AIRGAP_NSG_NAME" envDefault:"abe2e-airgap-securityGroup"`
	DefaultSubnetName             string        `env:"DEFAULT_SUBNET_NAME" envDefault:"aks-subnet"`
	BuildID                       string        `env:"BUILD_ID" envDefault:"local"`
	Location                      string        `env:"LOCATION" envDefault:"eastus2euap"`
	SubscriptionID                string        `env:"SUBSCRIPTION_ID" envDefault:"4f3dc0e4-0c77-40ff-bf9a-6ade1e3048ef"`
	SIGVersionTagName             string        `env:"SIG_VERSION_TAG_NAME" envDefault:"branch"`
	SIGVersionTagValue            string        `env:"SIG_VERSION_TAG_VALUE" envDefault:"refs/heads/master"`
	TagsToRun                     string        `env:"TAGS_TO_RUN"`
	TagsToSkip                    string        `env:"TAGS_TO_SKIP"`
	TestTimeout                   time.Duration `env:"TEST_TIMEOUT" envDefault:"20m"`
	E2ELoggingDir                 string        `env:"LOGGING_DIR" envDefault:"scenario-logs"`
	IgnoreScenariosWithMissingVHD bool          `env:"IGNORE_SCENARIOS_WITH_MISSING_VHD"`
	SkipTestsWithSKUCapacityIssue bool          `env:"SKIP_TESTS_WITH_SKU_CAPACITY_ISSUE"`
	KeepVMSS                      bool          `env:"KEEP_VMSS"`
}

func mustLoadConfig() Configuration {
	_ = godotenv.Load(".env")
	cfg := Configuration{}
	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}
	return cfg
}
