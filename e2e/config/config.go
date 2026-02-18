package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
)

var (
	Config         = mustLoadConfig()
	Azure          = mustNewAzureClient()
	VMIdentityName = "abe2e-vm-identity"

	DefaultPollUntilDoneOptions = &runtime.PollUntilDoneOptions{
		Frequency: time.Second,
	}
	VMSSHPublicKey, VMSSHPrivateKey, SysSSHPublicKey, SysSSHPrivateKey []byte
	VMSSHPrivateKeyFileName, SysSSHPrivateKeyFileName                  string
)

func ResourceGroupName(location string) string {
	return "abe2e-" + location
}

func PrivateACRNameNotAnon(location string) string {
	return "e2eprivateacrnonanon" + location // will have anonymous pull enabled
}

func PrivateACRName(location string) string {
	return "e2eprivateacr" + location // will not have anonymous pull enabled
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
	SIGVersionTagValue                     string        `env:"SIG_VERSION_TAG_VALUE" envDefault:"refs/heads/main"`
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
	SysSSHPublicKey                        string        `env:"SYS_SSH_PUBLIC_KEY"`
	SysSSHPrivateKeyB64                    string        `env:"SYS_SSH_PRIVATE_KEY_B64"`
	EnableScriptlessCSECmd                 bool          `env:"ENABLE_SCRIPTLESS_CSE_CMD" envDefault:"false"`
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
	VMSSHPrivateKey, VMSSHPublicKey, VMSSHPrivateKeyFileName = mustGetNewRSAKeyPair()
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Printf("Error loading .env file: %s\n", err)
	}
	cfg := &Configuration{}
	if err := env.Parse(cfg); err != nil {
		panic(err)
	}
	if cfg.SysSSHPublicKey == "" {
		SysSSHPublicKey = VMSSHPublicKey
	} else {
		SysSSHPublicKey = []byte(cfg.SysSSHPublicKey)
	}
	if cfg.SysSSHPrivateKeyB64 == "" {
		SysSSHPrivateKey, SysSSHPublicKey, SysSSHPrivateKeyFileName, err = getOrCreateRSAKeyPair()
		if err != nil {
			panic(fmt.Sprintf("failed to get or create RSA key pair: %v", err))
		}
	} else {
		SysSSHPrivateKey, err = base64.StdEncoding.DecodeString(cfg.SysSSHPrivateKeyB64)
		if err != nil {
			panic(err)
		}

		SysSSHPrivateKeyFileName, err = writePrivateKeyToTempFile(SysSSHPrivateKey)
		if err != nil {
			panic(err)
		}
	}

	return cfg
}

// Returns a newly generated RSA public/private key pair with the private key in PEM format.
func mustGetNewRSAKeyPair() ([]byte, []byte, string) {
	// Generate new key pair
	privatePEMBytes, publicKeyBytes, err := getNewRSAKeyPair()
	if err != nil {
		panic(fmt.Sprintf("failed to generate RSA key pair: %v", err))
	}

	privateKeyFileName, err := writePrivateKeyToTempFile(privatePEMBytes)
	if err != nil {
		panic(fmt.Sprintf("failed to write private key to temp file: %w", err))
	}

	return privatePEMBytes, publicKeyBytes, privateKeyFileName
}

// Returns a newly generated RSA public/private key pair with the private key in PEM format.
// We need to use RSA keys because AKS doesnt currently support ED25519 keys for node SSH access.
func getNewRSAKeyPair() (privatePEMBytes []byte, publicKeyBytes []byte, e error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create rsa private key: %w", err)
	}

	err = privateKey.Validate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate rsa private key: %w", err)
	}

	publicRsaKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert private to public key: %w", err)
	}

	publicKeyBytes = ssh.MarshalAuthorizedKey(publicRsaKey)

	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// pem.Block
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	// Private key in PEM format
	privatePEMBytes = pem.EncodeToMemory(&privBlock)

	return
}

// getOrCreateRSAKeyPair checks if an RSA key pair exists at ~/.ssh/ssh_rsa_agentbaker_e2e.
// If it exists, it reads and returns the existing key pair.
// If not, it generates a new key pair and saves it to that location.
func getOrCreateRSAKeyPair() (privatePEMBytes []byte, publicKeyBytes []byte, privateKeyFileName string, e error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	privateKeyPath := filepath.Join(sshDir, "ssh_rsa_agentbaker_e2e")
	publicKeyPath := privateKeyPath + ".pub"

	// Check if the private key file already exists
	if _, err := os.Stat(privateKeyPath); err == nil {
		// File exists, read it
		privatePEMBytes, err = os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to read existing private key: %w", err)
		}

		publicKeyBytes, err = os.ReadFile(publicKeyPath)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to read existing public key: %w", err)
		}

		return privatePEMBytes, publicKeyBytes, privateKeyPath, nil
	}

	// Generate new key pair
	privatePEMBytes, publicKeyBytes, err = getNewRSAKeyPair()
	if err != nil {
		return nil, nil, "", err
	}

	// Ensure .ssh directory exists
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return nil, nil, "", fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Write private key
	if err := os.WriteFile(privateKeyPath, privatePEMBytes, 0600); err != nil {
		return nil, nil, "", fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key
	if err := os.WriteFile(publicKeyPath, publicKeyBytes, 0644); err != nil {
		return nil, nil, "", fmt.Errorf("failed to write public key: %w", err)
	}

	return privatePEMBytes, publicKeyBytes, privateKeyPath, nil
}

func writePrivateKeyToTempFile(key []byte) (string, error) {
	// Create temp file with secure permissions
	tmpFile, err := os.CreateTemp("", "private-key-*")
	if err != nil {
		return "", err
	}

	// Ensure file permissions are restricted (owner read/write only)
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}

	// Write key
	if _, err := tmpFile.Write(key); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}

	// Close file (important!)
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

func GetPrivateACRName(isNonAnonymousPull bool, location string) string {
	privateACRName := PrivateACRName(location)
	if isNonAnonymousPull {
		privateACRName = PrivateACRNameNotAnon(location)
	}
	return privateACRName
}
