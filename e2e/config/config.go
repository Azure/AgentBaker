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
	Config         = mustLoadConfig()
	Azure          = mustNewAzureClient()
	VMIdentityName = "abe2e-vm-identity"

	DefaultPollUntilDoneOptions = &runtime.PollUntilDoneOptions{
		Frequency: time.Second,
	}
)

func ResourceGroupName(location string) string {
	return "abe2e-" + location
}

func PrivateACRNameNotAnon(location string) string {
	return "privateace2enonanonpull" + location // will have anonymous pull enabled
}

func PrivateACRName(location string) string {
	return "privateacre2e" + location // will not have anonymous pull enabled
}

type Configuration struct {
	// The defaults should only be used when running tests locally, as the CI will set these env vars.
	// We have separate Linux and Windows consts to have different defaults - they use the same env vars.
	ACRSecretName                          string        `env:"ACR_SECRET_NAME" envDefault:"acr-secret-code2"`
	AirgapNSGName                          string        `env:"AIRGAP_NSG_NAME" envDefault:"abe2e-airgap-securityGroup"`
	AzureContainerRegistrytargetRepository string        `env:"ACR_TARGET_REPOSITORY" envDefault:"*"`
	BlobContainer                          string        `env:"BLOB_CONTAINER" envDefault:"abe2e"`
	BlobStorageAccountPrefix               string        `env:"BLOB_STORAGE_ACCOUNT_PREFIX" envDefault:"abe2e"`
	BuildID                                string        `env:"BUILD_ID" envDefault:"local"`
	DefaultLocation                        string        `env:"E2E_LOCATION" envDefault:"westus3"`
	DefaultPollInterval                    time.Duration `env:"DEFAULT_POLL_INTERVAL" envDefault:"1s"`
	DefaultSubnetName                      string        `env:"DEFAULT_SUBNET_NAME" envDefault:"aks-subnet"`
	DefaultVMSKU                           string        `env:"DEFAULT_VM_SKU" envDefault:"Standard_D2ds_v5"`
	DisableScriptLessCompilation           bool          `env:"DISABLE_SCRIPTLESS_COMPILATION"`
	E2ELoggingDir                          string        `env:"LOGGING_DIR" envDefault:"scenario-logs"`
	GalleryNameLinux                       string        `env:"GALLERY_NAME" envDefault:"PackerSigGalleryEastUS"`
	GalleryNameWindows                     string        `env:"GALLERY_NAME" envDefault:"PackerSigGalleryEastUS"`
	GalleryResourceGroupNameLinux          string        `env:"GALLERY_RESOURCE_GROUP" envDefault:"aksvhdtestbuildrg"`
	GalleryResourceGroupNameWindows        string        `env:"GALLERY_RESOURCE_GROUP" envDefault:"aksvhdtestbuildrg"`
	GallerySubscriptionIDLinux             string        `env:"GALLERY_SUBSCRIPTION_ID" envDefault:"c4c3550e-a965-4993-a50c-628fd38cd3e1"`
	GallerySubscriptionIDWindows           string        `env:"GALLERY_SUBSCRIPTION_ID" envDefault:"c4c3550e-a965-4993-a50c-628fd38cd3e1"`
	IgnoreScenariosWithMissingVHD          bool          `env:"IGNORE_SCENARIOS_WITH_MISSING_VHD"`
	KeepVMSS                               bool          `env:"KEEP_VMSS"`
	SIGVersionTagName                      string        `env:"SIG_VERSION_TAG_NAME" envDefault:"branch"`
	SIGVersionTagValue                     string        `env:"SIG_VERSION_TAG_VALUE" envDefault:"refs/heads/master"`
	SkipTestsWithSKUCapacityIssue          bool          `env:"SKIP_TESTS_WITH_SKU_CAPACITY_ISSUE"`
	SubscriptionID                         string        `env:"SUBSCRIPTION_ID" envDefault:"8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8"`
	TagsToRun                              string        `env:"TAGS_TO_RUN"`
	TagsToSkip                             string        `env:"TAGS_TO_SKIP"`
	TestGalleryImagePrefix                 string        `env:"TEST_GALLERY_IMAGE_PREFIX" envDefault:"abe2etest"`
	TestGalleryNamePrefix                  string        `env:"TEST_GALLERY_NAME_PREFIX" envDefault:"abe2etest"`
	TestPreProvision                       bool          `env:"TEST_PRE_PROVISION" envDefault:"false"`
	TestTimeout                            time.Duration `env:"TEST_TIMEOUT" envDefault:"35m"`
	TestTimeoutCluster                     time.Duration `env:"TEST_TIMEOUT_CLUSTER" envDefault:"20m"`
	TestTimeoutVMSS                        time.Duration `env:"TEST_TIMEOUT_VMSS" envDefault:"17m"`
	WindowsAdminPassword                   string        `env:"WINDOWS_ADMIN_PASSWORD"`
}

func (c *Configuration) BlobStorageAccount() string {
	// Here DefaultLocation is used because the azure blob client requires the
	// full URL to the storage account, which means creating a new client per
	// location. While everything else for running AB tests is sharded per
	// location, but we continue to use the same storage account for all
	// locations.
	return c.BlobStorageAccountPrefix + c.DefaultLocation
}

func (c *Configuration) IsLocalBuild() bool {
	return c.BuildID == "local"
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

func (c *Configuration) VMIdentityResourceID(location string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s", c.SubscriptionID, ResourceGroupName(location), VMIdentityName)
}

func mustLoadConfig() *Configuration {
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Printf("Error loading .env file: %s\n", err)
	}
	cfg := &Configuration{}
	if err := env.Parse(cfg); err != nil {
		panic(err)
	}
	return cfg
}

func GetPrivateACRName(isNonAnonymousPull bool, location string) string {
	privateACRName := PrivateACRName(location)
	if isNonAnonymousPull {
		privateACRName = PrivateACRNameNotAnon(location)
	}
	return privateACRName
}
