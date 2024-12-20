package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

var (
	Config            = mustLoadConfig()
	Azure             = mustNewAzureClient()
	ResourceGroupName = "abe2e-" + Config.Location
	VMIdentityName    = "abe2e-vm-identity"
	PrivateACRName    = "privateacre2e" + Config.Location

	DefaultPollUntilDoneOptions = &runtime.PollUntilDoneOptions{
		Frequency: time.Second,
	}
)

type Configuration struct {
	AirgapNSGName                 string        `env:"AIRGAP_NSG_NAME" envDefault:"abe2e-airgap-securityGroup" json:"airgapNSGName"`
	DefaultSubnetName             string        `env:"DEFAULT_SUBNET_NAME" envDefault:"aks-subnet" json:"defaultSubnetName"`
	BuildID                       string        `env:"BUILD_ID" envDefault:"local" json:"buildID"`
	Location                      string        `env:"E2E_LOCATION" envDefault:"westus3" json:"location"`
	SubscriptionID                string        `env:"SUBSCRIPTION_ID" envDefault:"8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8" json:"subscriptionID"`
	GallerySubscriptionID         string        `env:"GALLERY_SUBSCRIPTION_ID" envDefault:"c4c3550e-a965-4993-a50c-628fd38cd3e1" json:"gallerySubscriptionID"`
	GalleryResourceGroupName      string        `env:"GALLERY_RESOURCE_GROUP_NAME" envDefault:"aksvhdtestbuildrg" json:"galleryResourceGroupName"`
	GalleryName                   string        `env:"GALLERY_NAME" envDefault:"PackerSigGalleryEastUS" json:"galleryName"`
	SIGVersionTagName             string        `env:"SIG_VERSION_TAG_NAME" envDefault:"branch" json:"sigVersionTagName"`
	SIGVersionTagValue            string        `env:"SIG_VERSION_TAG_VALUE" envDefault:"refs/heads/master" json:"sigVersionTagValue"`
	TagsToRun                     string        `env:"TAGS_TO_RUN" json:"tagsToRun"`
	TagsToSkip                    string        `env:"TAGS_TO_SKIP" json:"tagsToSkip"`
	TestTimeout                   time.Duration `env:"TEST_TIMEOUT" envDefault:"35m" json:"testTimeout"`
	E2ELoggingDir                 string        `env:"LOGGING_DIR" envDefault:"scenario-logs" json:"e2eLoggingDir"`
	IgnoreScenariosWithMissingVHD bool          `env:"IGNORE_SCENARIOS_WITH_MISSING_VHD" json:"ignoreScenariosWithMissingVHD"`
	SkipTestsWithSKUCapacityIssue bool          `env:"SKIP_TESTS_WITH_SKU_CAPACITY_ISSUE" json:"skipTestsWithSKUCapacityIssue"`
	KeepVMSS                      bool          `env:"KEEP_VMSS" json:"keepVMSS"`
	BlobStorageAccountPrefix      string        `env:"BLOB_STORAGE_ACCOUNT_PREFIX" envDefault:"abe2e" json:"blobStorageAccountPrefix"`
	BlobContainer                 string        `env:"BLOB_CONTAINER" envDefault:"abe2e" json:"blobContainer"`
	EnableNodeBootstrapperTest    bool          `env:"ENABLE_NODE_BOOTSTRAPPER_TEST" json:"enableNodeBootstrapperTest"`
}

func (c *Configuration) BlobStorageAccount() string {
	return c.BlobStorageAccountPrefix + c.Location
}

func (c *Configuration) BlobStorageAccountURL() string {
	return "https://" + c.BlobStorageAccount() + ".blob.core.windows.net"
}

func (c *Configuration) GalleryResourceID() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s", c.GallerySubscriptionID, c.GalleryResourceGroupName, c.GalleryName)
}

func (c Configuration) String() string {
	content, err := json.MarshalIndent(c, "", "	")
	if err != nil {
		panic(err)
	}
	return string(content)
}

func mustLoadConfig() Configuration {
	_ = godotenv.Load(".env")
	cfg := Configuration{}
	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}
	return cfg
}
