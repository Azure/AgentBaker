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

	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
)

const (
	DefaultV5VMSKU = "Standard_D2ds_v5"
)

var (
	// Config is populated before scenario goroutines start and is immutable after Initialize.
	Config                                                             = DefaultConfiguration()
	Azure                                                              *AzureClient
	VMIdentityName                                                     = "abe2e-vm-identity"
	VMSSHPublicKey, VMSSHPrivateKey, SysSSHPublicKey, SysSSHPrivateKey []byte
	VMSSHPrivateKeyFileName, SysSSHPrivateKeyFileName                  string
)

func ResourceGroupName(location string) string {
	return "abe2e-" + location
}

func PrivateACRNameNotAnon(location string) string {
	return "abe2eprivatenonanon" + location // will have anonymous pull enabled
}

func PrivateACRName(location string) string {
	return "abe2eprivate" + location // will not have anonymous pull enabled
}

type Configuration struct {
	ACRSecretName                          string
	AzureContainerRegistrytargetRepository string
	BlobContainer                          string
	BlobStorageAccountPrefix               string
	BuildID                                string
	DefaultLocation                        string
	DefaultPollInterval                    time.Duration
	DefaultSubnetName                      string
	DefaultVMSKU                           string
	DisableScriptless                      bool
	DisableScriptLessCompilation           bool
	E2ELoggingDir                          string
	EnableSecureTLSBootstrapping           bool
	ExtendedTests                          string
	GalleryLinux                           Gallery
	GalleryWindows                         Gallery
	IgnoreScenariosWithMissingVHD          bool
	KeepVMSS                               bool
	NetworkIsolatedNSGName                 string
	Parallel                               int
	Retries                                int
	JUnitFile                              string
	OutputMode                             string
	SIGVersionTagName                      string
	SIGVersionTagValue                     string
	SkipTestsWithSKUCapacityIssue          bool
	SubscriptionID                         string
	SysSSHPublicKey                        string
	SysSSHPrivateKeyB64                    string
	TagsToRun                              string
	TagsToSkip                             string
	TestGalleryImagePrefix                 string
	TestGalleryNamePrefix                  string
	TestPreProvision                       bool
	TestTimeout                            time.Duration
	SuiteTimeout                           time.Duration
	VHDMetadataFile                        string
	// Must cover cluster-create AND bastion-create (run serially in prepareCluster, ~10-11m each).
	TestTimeoutCluster   time.Duration
	TestTimeoutVMSS      time.Duration
	WindowsAdminPassword string
	vhdMetadata          map[string]vhdMetadataEntry
}

func DefaultConfiguration() *Configuration {
	return &Configuration{
		ACRSecretName:                          "acr-secret-code2",
		AzureContainerRegistrytargetRepository: "aks-managed-repository/*",
		BlobContainer:                          "abe2e",
		BlobStorageAccountPrefix:               "abe2e",
		BuildID:                                "local",
		DefaultLocation:                        "westus3",
		DefaultPollInterval:                    15 * time.Second,
		DefaultSubnetName:                      "aks-subnet",
		DefaultVMSKU:                           "Standard_D2ds_v5",
		E2ELoggingDir:                          "scenario-logs",
		EnableSecureTLSBootstrapping:           true,
		GalleryLinux: Gallery{
			Name:              "PackerSigGalleryEastUS",
			ResourceGroupName: "aksvhdtestbuildrg",
			SubscriptionID:    "c4c3550e-a965-4993-a50c-628fd38cd3e1",
		},
		GalleryWindows: Gallery{
			Name:              "PackerSigGalleryEastUS",
			ResourceGroupName: "aksvhdtestbuildrg",
			SubscriptionID:    "c4c3550e-a965-4993-a50c-628fd38cd3e1",
		},
		NetworkIsolatedNSGName: "abe2e-networkisolated-securityGroup",
		Parallel:               60,
		OutputMode:             "auto",
		SIGVersionTagName:      "branch",
		SIGVersionTagValue:     "refs/heads/main",
		SubscriptionID:         "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8",
		TestGalleryImagePrefix: "abe2etest",
		TestGalleryNamePrefix:  "abe2etest",
		TestTimeout:            50 * time.Minute,
		SuiteTimeout:           80 * time.Minute,
		TestTimeoutCluster:     30 * time.Minute,
		TestTimeoutVMSS:        17 * time.Minute,
	}
}

func (c *Configuration) BlobStorageAccount() string {
	// Storage account names are GLOBALLY unique in Azure (not per-subscription).
	// Two subscriptions cannot own an account with the same name; the second
	// one to call BeginCreate fails with StorageAccountAlreadyTaken even
	// though BeginCreate is otherwise idempotent within a single sub.
	//
	// This bites whenever a pipeline that normally targets subscription A
	// (with BLOB_STORAGE_ACCOUNT_PREFIX baked into a variable group) is
	// redirected at runtime to subscription B — for example when the aks-rp
	// orchestrator routes the RCV1P phase to its dedicated testing sub via
	// --subscription-id. The prefix was chosen for sub A, the account
	// "<prefix><location>" already exists in sub A, and creation in sub B
	// fails before any test can run.
	//
	// Embedding a deterministic subscription-derived suffix in the account
	// name guarantees a globally-unique name per subscription with zero
	// per-environment configuration: every new sub gets its own account
	// the first time it runs and reuses it thereafter. The framework stays
	// completely agnostic to which subscription is "special" — there is no
	// subscription identity check anywhere in this repo.
	//
	// DefaultLocation is included for the historical reason captured below
	// (the blob client is keyed off the storage account URL, which is per
	// location, even though tests run across multiple locations).
	//
	// Here DefaultLocation is used because the azure blob client requires the
	// full URL to the storage account, which means creating a new client per
	// location. While everything else for running AB tests is sharded per
	// location, but we continue to use the same storage account for all
	// locations.
	suffix := subscriptionSuffix(c.SubscriptionID)
	base := c.BlobStorageAccountPrefix + c.DefaultLocation
	// Azure storage account names are limited to 24 chars (lowercase alphanumeric).
	// Truncate the base portion if a long region name + prefix would otherwise
	// overflow once the deterministic suffix is appended. The suffix is kept whole
	// so two subscriptions never collide; truncation is deterministic too, so the
	// same (prefix, location, sub) always resolves to the same account name.
	if maxBase := 24 - len(suffix); len(base) > maxBase {
		if maxBase < 0 {
			maxBase = 0
		}
		base = base[:maxBase]
	}
	return base + suffix
}

// subscriptionSuffix returns a short, deterministic suffix derived from the
// subscription ID, suitable for embedding in resource names that have
// global-uniqueness constraints (e.g. storage accounts). It takes the first
// 6 hex characters of the subscription UUID after stripping hyphens, which
// keeps the resulting name within the Azure storage account length limit
// (3-24 chars) while giving ~16M-way collision resistance — sufficient for
// the handful of subscriptions this test framework will ever run against.
//
// Lowercase hex is also valid for storage account names (lowercase
// alphanumeric only), so no further sanitization is required.
func subscriptionSuffix(subscriptionID string) string {
	cleaned := strings.ToLower(strings.ReplaceAll(subscriptionID, "-", ""))
	if len(cleaned) < 6 {
		return cleaned
	}
	return cleaned[:6]
}

func (c *Configuration) IsLocalBuild() bool {
	return c.BuildID == "local"
}

func (c *Configuration) BlobStorageAccountURL() string {
	return "https://" + c.BlobStorageAccount() + ".blob.core.windows.net"
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
		if field.PkgPath != "" {
			continue
		}
		switch field.Name {
		case "SysSSHPrivateKeyB64", "WindowsAdminPassword":
			continue
		}
		data = append(data, fmt.Sprintf("%s=%v", field.Name, v.Field(i)))
	}
	sort.Strings(data)
	return strings.Join(data, "\n")
}

func (c *Configuration) VMIdentityResourceID(location string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s", c.SubscriptionID, ResourceGroupName(location), VMIdentityName)
}

func LoadDotEnv() error {
	err := godotenv.Load(".env")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}
	return nil
}

func Initialize() error {
	VMSSHPrivateKey, VMSSHPublicKey, VMSSHPrivateKeyFileName = mustGetNewRSAKeyPair()
	var err error
	Config.vhdMetadata = nil
	if Config.VHDMetadataFile != "" {
		Config.vhdMetadata, err = loadVHDMetadata(Config.VHDMetadataFile)
		if err != nil {
			return fmt.Errorf("load E2E VHD metadata: %w", err)
		}
	}
	if Config.SysSSHPublicKey == "" {
		SysSSHPublicKey = VMSSHPublicKey
	} else {
		SysSSHPublicKey = []byte(Config.SysSSHPublicKey)
	}
	if Config.SysSSHPrivateKeyB64 == "" {
		SysSSHPrivateKey, SysSSHPublicKey, SysSSHPrivateKeyFileName, err = getOrCreateRSAKeyPair()
		if err != nil {
			return fmt.Errorf("get or create system SSH key pair: %w", err)
		}
	} else {
		SysSSHPrivateKey, err = base64.StdEncoding.DecodeString(Config.SysSSHPrivateKeyB64)
		if err != nil {
			return fmt.Errorf("decode system SSH private key: %w", err)
		}

		SysSSHPrivateKeyFileName, err = writePrivateKeyToTempFile(SysSSHPrivateKey)
		if err != nil {
			return fmt.Errorf("write system SSH private key: %w", err)
		}
	}

	Azure, err = NewAzureClient()
	if err != nil {
		return err
	}
	return nil
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
		panic(fmt.Sprintf("failed to write private key to temp file: %v", err))
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
