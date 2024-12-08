package config

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
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
	PrivateACRName    = "privateacre2e" + Config.Location + "dev" // TODO (alburgess): remove dev once CCOA is over

	DefaultPollUntilDoneOptions = &runtime.PollUntilDoneOptions{
		Frequency: time.Second,
	}
)

type Configuration struct {
	AirgapNSGName                          string        `env:"AIRGAP_NSG_NAME" envDefault:"abe2e-airgap-securityGroup"`
	AzureContainerRegistrytargetRepository string        `env:"ACR_TARGET_REPOSITORY" envDefault:"*"`
	BlobContainer                          string        `env:"BLOB_CONTAINER" envDefault:"abe2e"`
	BlobStorageAccountPrefix               string        `env:"BLOB_STORAGE_ACCOUNT_PREFIX" envDefault:"abe2e"`
	BuildID                                string        `env:"BUILD_ID" envDefault:"local"`
	DefaultSubnetName                      string        `env:"DEFAULT_SUBNET_NAME" envDefault:"aks-subnet"`
	E2ELoggingDir                          string        `env:"LOGGING_DIR" envDefault:"scenario-logs"`
	EnableAKSNodeControllerTest            bool          `env:"ENABLE_AKS_NODE_CONTROLLER_TEST"`
	GalleryNameLinux                       string        `env:"GALLERY_NAME" envDefault:"PackerSigGalleryEastUS"`
	GalleryNameWindows                     string        `env:"GALLERY_NAME_WINDOWS" envDefault:"AKSWindows"`
	GalleryResourceGroupNameLinux          string        `env:"GALLERY_RESOURCE_GROUP" envDefault:"aksvhdtestbuildrg"`
	GalleryResourceGroupNameWindows        string        `env:"GALLERY_RESOURCE_GROUP_WINDOWS" envDefault:"AKS-Windows"`
	GallerySubscriptionIDLinux             string        `env:"GALLERY_SUBSCRIPTION_ID" envDefault:"c4c3550e-a965-4993-a50c-628fd38cd3e1"`
	GallerySubscriptionIDWindows           string        `env:"GALLERY_SUBSCRIPTION_ID_WINDOWS" envDefault:"4be8920b-2978-43d7-ab14-04d8549c1d05"`
	IgnoreScenariosWithMissingVHD          bool          `env:"IGNORE_SCENARIOS_WITH_MISSING_VHD"`
	KeepVMSS                               bool          `env:"KEEP_VMSS"`
	Location                               string        `env:"LOCATION" envDefault:"westus3"`
	SIGVersionTagName                      string        `env:"SIG_VERSION_TAG_NAME" envDefault:"branch"`
	SIGVersionTagValue                     string        `env:"SIG_VERSION_TAG_VALUE" envDefault:"refs/heads/dev"`
	SkipTestsWithSKUCapacityIssue          bool          `env:"SKIP_TESTS_WITH_SKU_CAPACITY_ISSUE"`
	SubscriptionID                         string        `env:"SUBSCRIPTION_ID" envDefault:"8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8"`
	TagsToRun                              string        `env:"TAGS_TO_RUN"`
	TagsToSkip                             string        `env:"TAGS_TO_SKIP"`
	TestTimeout                            time.Duration `env:"TEST_TIMEOUT" envDefault:"35m"`
	TestTimeoutVMSS                        time.Duration `env:"TEST_TIMEOUT_VMSS" envDefault:"17m"`
	WindowsAdminPassword                   string        `env:"WINDOWS_ADMIN_PASSWORD"`
}

func (c *Configuration) BlobStorageAccount() string {
	return c.BlobStorageAccountPrefix + c.Location
}

func (c *Configuration) BlobStorageAccountURL() string {
	return "https://" + c.BlobStorageAccount() + ".blob.core.windows.net"
}

func (c *Configuration) GalleryResourceID() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s", c.GallerySubscriptionIDLinux, c.GalleryResourceGroupNameLinux, c.GalleryNameLinux)
}

func (c *Configuration) String() string {
	data := make([]string, 0)
	v := reflect.ValueOf(c)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		if envTag != "" {
			data = append(data, fmt.Sprintf("%s=%v", envTag, v.Field(i)))
		}
	}
	sort.Strings(data)
	return strings.Join(data, "\n")
}

func (c *Configuration) VMIdentityResourceID() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s", c.SubscriptionID, ResourceGroupName, VMIdentityName)
}

func mustLoadConfig() *Configuration {
	_ = godotenv.Load(".env")
	cfg := &Configuration{}
	if err := env.Parse(cfg); err != nil {
		panic(err)
	}
	return cfg
}
