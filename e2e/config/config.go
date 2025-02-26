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
	Config                = mustLoadConfig()
	Azure                 = mustNewAzureClient()
	ResourceGroupName     = "abe2e-" + Config.Location
	VMIdentityName        = "abe2e-vm-identity"
	PrivateACRNameNotAnon = "privateace2enonanonpull" + Config.Location // will have anonymous pull enabled
	PrivateACRName        = "privateacre2e" + Config.Location           // will not have anonymous pull enabled

	DefaultPollUntilDoneOptions = &runtime.PollUntilDoneOptions{
		Frequency: time.Second,
	}
)

type Configuration struct {
	AirgapNSGName                          string        `env:"AIRGAP_NSG_NAME" envDefault:"abe2e-airgap-securityGroup"`
	AzureContainerRegistrytargetRepository string        `env:"ACR_TARGET_REPOSITORY" envDefault:"*"`
	ACRSecretName                          string        `env:"ACR_SECRET_NAME" envDefault:"acr-secret-code2"`
	BlobContainer                          string        `env:"BLOB_CONTAINER" envDefault:"abe2e"`
	BlobStorageAccountPrefix               string        `env:"BLOB_STORAGE_ACCOUNT_PREFIX" envDefault:"abe2e"`
	BuildID                                string        `env:"BUILD_ID" envDefault:"local"`
	DefaultSubnetName                      string        `env:"DEFAULT_SUBNET_NAME" envDefault:"aks-subnet"`
	DefaultVMSKU                           string        `env:"DEFAULT_VM_SKU" envDefault:"Standard_D2ds_v5"`
	E2ELoggingDir                          string        `env:"LOGGING_DIR" envDefault:"scenario-logs"`
	EnableAKSNodeControllerTest            bool          `env:"ENABLE_AKS_NODE_CONTROLLER_TEST"`
	GalleryNameLinux                       string        `env:"GALLERY_NAME" envDefault:"MarinerAKSSig"`
	GalleryNameWindows                     string        `env:"GALLERY_NAME_WINDOWS" envDefault:"AKSWindows"`
	GalleryResourceGroupNameLinux          string        `env:"GALLERY_RESOURCE_GROUP" envDefault:"MarinerAKSTest"`
	GalleryResourceGroupNameWindows        string        `env:"GALLERY_RESOURCE_GROUP_WINDOWS" envDefault:"AKS-Windows"`
	GallerySubscriptionIDLinux             string        `env:"GALLERY_SUBSCRIPTION_ID" envDefault:"035db282-f1c8-4ce7-b78f-2a7265d5398c"`
	GallerySubscriptionIDWindows           string        `env:"GALLERY_SUBSCRIPTION_ID_WINDOWS" envDefault:"4be8920b-2978-43d7-ab14-04d8549c1d05"`
	IgnoreScenariosWithMissingVHD          bool          `env:"IGNORE_SCENARIOS_WITH_MISSING_VHD"`
	KeepVMSS                               bool          `env:"KEEP_VMSS" envDefault:"true"` 
	Location                               string        `env:"E2E_LOCATION" envDefault:"southcentralus"`
	SIGVersionTagName                      string        `env:"SIG_VERSION_TAG_NAME" envDefault:""`
	SIGVersionTagValue                     string        `env:"SIG_VERSION_TAG_VALUE" envDefault:""`
	SkipTestsWithSKUCapacityIssue          bool          `env:"SKIP_TESTS_WITH_SKU_CAPACITY_ISSUE"`
	SubscriptionID                         string        `env:"SUBSCRIPTION_ID" envDefault:"035db282-f1c8-4ce7-b78f-2a7265d5398c"`
	TagsToRun                              string        `env:"TAGS_TO_RUN"`
	TagsToSkip                             string        `env:"TAGS_TO_SKIP"`
	TestTimeout                            time.Duration `env:"TEST_TIMEOUT" envDefault:"35m"`
	TestTimeoutCluster                     time.Duration `env:"TEST_TIMEOUT_CLUSTER" envDefault:"20m"`
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

func GetPrivateACRName(isNonAnonymousPull bool) string {
	privateACRName := PrivateACRName
	if isNonAnonymousPull {
		privateACRName = PrivateACRNameNotAnon
	}
	return privateACRName
}
