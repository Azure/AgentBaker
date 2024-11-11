package config

import (
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

var (
	Config            = mustLoadConfig()
	Azure             = mustNewAzureClient(Config.SubscriptionID)
	ResourceGroupName = "abe2e-" + Config.Location
	VMIdentityName    = "abe2e-vm-identity"
	PrivateACRName    = "privateacre2e"

	DefaultPollUntilDoneOptions = &runtime.PollUntilDoneOptions{
		Frequency: time.Second,
	}
)

type Configuration struct {
	AirgapNSGName                 string        `env:"AIRGAP_NSG_NAME" envDefault:"abe2e-airgap-securityGroup"`
	DefaultSubnetName             string        `env:"DEFAULT_SUBNET_NAME" envDefault:"aks-subnet"`
	BuildID                       string        `env:"BUILD_ID" envDefault:"local"`
	Location                      string        `env:"LOCATION" envDefault:"westus3"`
	SubscriptionID                string        `env:"SUBSCRIPTION_ID" envDefault:"c4c3550e-a965-4993-a50c-628fd38cd3e1"`
	GalleryResourceGroupName      string        `env:"GALLERY_RESOURCE_GROUP_NAME" envDefault:"aksvhdtestbuildrg"`
	GalleryName                   string        `env:"GALLERY_NAME" envDefault:"PackerSigGalleryEastUS"`
	SIGVersionTagName             string        `env:"SIG_VERSION_TAG_NAME" envDefault:"branch"`
	SIGVersionTagValue            string        `env:"SIG_VERSION_TAG_VALUE" envDefault:"refs/heads/master"`
	TagsToRun                     string        `env:"TAGS_TO_RUN"`
	TagsToSkip                    string        `env:"TAGS_TO_SKIP"`
	TestTimeout                   time.Duration `env:"TEST_TIMEOUT" envDefault:"35m"`
	E2ELoggingDir                 string        `env:"LOGGING_DIR" envDefault:"scenario-logs"`
	IgnoreScenariosWithMissingVHD bool          `env:"IGNORE_SCENARIOS_WITH_MISSING_VHD"`
	SkipTestsWithSKUCapacityIssue bool          `env:"SKIP_TESTS_WITH_SKU_CAPACITY_ISSUE"`
	KeepVMSS                      bool          `env:"KEEP_VMSS"`
	BlobStorageAccountPrefix      string        `env:"BLOB_STORAGE_ACCOUNT_PREFIX" envDefault:"abe2e"`
	BlobContainer                 string        `env:"BLOB_CONTAINER" envDefault:"abe2e"`
	EnableAKSNodeControllerTest   bool          `env:"ENABLE_AKS_NODE_CONTROLLER_TEST"`
}

func (c *Configuration) BlobStorageAccount() string {
	return c.BlobStorageAccountPrefix + c.Location
}

func (c *Configuration) BlobStorageAccountURL() string {
	return "https://" + c.BlobStorageAccount() + ".blob.core.windows.net"
}

func (c *Configuration) E2EGalleryResourceID() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s", c.SubscriptionID, c.GalleryResourceGroupName, c.GalleryName)
}

func mustLoadConfig() Configuration {
	_ = godotenv.Load(".env")
	cfg := Configuration{}
	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}
	return cfg
}
