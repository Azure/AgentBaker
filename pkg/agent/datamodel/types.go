// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	neturl "net/url"
	"sort"
	"strings"
	"sync"

	"github.com/Azure/go-autorest/autorest/to"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logsapi "k8s.io/component-base/config/v1alpha1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
)

// TypeMeta describes an individual API model object
type TypeMeta struct {
	// APIVersion is on every object
	APIVersion string `json:"apiVersion"`
}

// CustomSearchDomain represents the Search Domain when the custom vnet has a windows server DNS as a nameserver.
type CustomSearchDomain struct {
	Name          string `json:"name,omitempty"`
	RealmUser     string `json:"realmUser,omitempty"`
	RealmPassword string `json:"realmPassword,omitempty"`
}

// PublicKey represents an SSH key for LinuxProfile
type PublicKey struct {
	KeyData string `json:"keyData"`
}

// KeyVaultCertificate specifies a certificate to install
// On Linux, the certificate file is placed under the /var/lib/waagent directory
// with the file name <UppercaseThumbprint>.crt for the X509 certificate file
// and <UppercaseThumbprint>.prv for the private key. Both of these files are .pem formatted.
// On windows the certificate will be saved in the specified store.
type KeyVaultCertificate struct {
	CertificateURL   string `json:"certificateUrl,omitempty"`
	CertificateStore string `json:"certificateStore,omitempty"`
}

// KeyVaultID specifies a key vault
type KeyVaultID struct {
	ID string `json:"id,omitempty"`
}

// KeyVaultRef represents a reference to KeyVault instance on Azure
type KeyVaultRef struct {
	KeyVault      KeyVaultID `json:"keyVault"`
	SecretName    string     `json:"secretName"`
	SecretVersion string     `json:"secretVersion,omitempty"`
}

// KeyVaultSecrets specifies certificates to install on the pool
// of machines from a given key vault
// the key vault specified must have been granted read permissions to CRP
type KeyVaultSecrets struct {
	SourceVault       *KeyVaultID           `json:"sourceVault,omitempty"`
	VaultCertificates []KeyVaultCertificate `json:"vaultCertificates,omitempty"`
}

// ImageReference represents a reference to an Image resource in Azure.
type ImageReference struct {
	Name           string `json:"name,omitempty"`
	ResourceGroup  string `json:"resourceGroup,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
	Gallery        string `json:"gallery,omitempty"`
	Version        string `json:"version,omitempty"`
}

// VMDiagnostics contains settings to on/off boot diagnostics collection
// in RD Host
type VMDiagnostics struct {
	Enabled bool `json:"enabled"`

	// Specifies storage account Uri where Boot Diagnostics (CRP &
	// VMSS BootDiagostics) and VM Diagnostics logs (using Linux
	// Diagnostics Extension) will be stored. Uri will be of standard
	// blob domain. i.e. https://storageaccount.blob.core.windows.net/
	// This field is readonly as ACS RP will create a storage account
	// for the customer.
	StorageURL *neturl.URL `json:"storageUrl"`
}

// OSType represents OS types of agents
type OSType string

// the OSTypes supported by vlabs
const (
	Windows OSType = "Windows"
	Linux   OSType = "Linux"
)

// KubeletDiskType describes options for placement of the primary kubelet partition,
// docker images, emptyDir volumes, and pod logs.
type KubeletDiskType string

const (
	// OSDisk indicates data wil be shared with the OS.
	OSDisk KubeletDiskType = "OS"
	// TempDisk indicates date will be isolated on the temporary disk.
	TempDisk KubeletDiskType = "Temporary"
)

// WorkloadRuntime describes choices for the type of workload: container or wasm-wasi, currently.
type WorkloadRuntime string

const (
	// OCIContainer indicates that kubelet will be used for a container workload.
	OCIContainer WorkloadRuntime = "OCIContainer"
	// WasmWasi indicates Krustlet will be used for a WebAssembly workload.
	WasmWasi WorkloadRuntime = "WasmWasi"
)

// Distro represents Linux distro to use for Linux VMs
type Distro string

// Distro string consts
const (
	Ubuntu                             Distro = "ubuntu"
	Ubuntu1804                         Distro = "ubuntu-18.04"
	Ubuntu1804Gen2                     Distro = "ubuntu-18.04-gen2"
	AKSUbuntu1804Gen2                  Distro = "ubuntu-18.04-gen2" // same distro as Ubuntu1804Gen2, renamed for clarity
	AKSUbuntu1604                      Distro = "aks-ubuntu-16.04"
	AKSUbuntu1804                      Distro = "aks-ubuntu-18.04"
	AKSUbuntuGPU1804                   Distro = "aks-ubuntu-gpu-18.04"
	AKSUbuntuGPU1804Gen2               Distro = "aks-ubuntu-gpu-18.04-gen2"
	AKSUbuntuContainerd1804            Distro = "aks-ubuntu-containerd-18.04"
	AKSUbuntuContainerd1804Gen2        Distro = "aks-ubuntu-containerd-18.04-gen2"
	AKSUbuntuGPUContainerd1804         Distro = "aks-ubuntu-gpu-containerd-18.04"
	AKSUbuntuGPUContainerd1804Gen2     Distro = "aks-ubuntu-gpu-containerd-18.04-gen2"
	AKSCBLMarinerV1                    Distro = "aks-cblmariner-v1"
	AKSCBLMarinerV2Gen2                Distro = "aks-cblmariner-v2-gen2"
	AKSUbuntuFipsContainerd1804        Distro = "aks-ubuntu-fips-containerd-18.04"
	AKSUbuntuFipsContainerd1804Gen2    Distro = "aks-ubuntu-fips-containerd-18.04-gen2"
	AKSUbuntuFipsGPUContainerd1804     Distro = "aks-ubuntu-fips-gpu-containerd-18.04"
	AKSUbuntuFipsGPUContainerd1804Gen2 Distro = "aks-ubuntu-fips-gpu-containerd-18.04-gen2"
	AKSUbuntuArm64Containerd1804Gen2   Distro = "aks-ubuntu-arm64-containerd-18.04-gen2"
	AKSUbuntuContainerd2204            Distro = "aks-ubuntu-containerd-22.04"
	AKSUbuntuContainerd2204Gen2        Distro = "aks-ubuntu-containerd-22.04-gen2"
	AKSUbuntuContainerd2004CVMGen2     Distro = "aks-ubuntu-containerd-20.04-cvm-gen2"
	AKSUbuntuArm64Containerd2204Gen2   Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
	AKSCBLMarinerV2Arm64Gen2           Distro = "aks-cblmariner-v2-arm64-gen2"
	RHEL                               Distro = "rhel"
	CoreOS                             Distro = "coreos"
	AKS1604Deprecated                  Distro = "aks"      // deprecated AKS 16.04 distro. Equivalent to aks-ubuntu-16.04.
	AKS1804Deprecated                  Distro = "aks-1804" // deprecated AKS 18.04 distro. Equivalent to aks-ubuntu-18.04.

	// Windows string const
	// AKSWindows2019 stands for distro of windows server 2019 SIG image with docker
	AKSWindows2019 Distro = "aks-windows-2019"
	// AKSWindows2019Containerd stands for distro for windows server 2019 SIG image with containerd
	AKSWindows2019Containerd Distro = "aks-windows-2019-containerd"
	// AKSWindows2022Containerd stands for distro for windows server 2022 SIG image with containerd
	AKSWindows2022Containerd Distro = "aks-windows-2022-containerd"
	// AKSWindows2019PIR stands for distro of windows server 2019 PIR image with docker
	AKSWindows2019PIR        Distro = "aks-windows-2019-pir"
	CustomizedImage          Distro = "CustomizedImage"
	CustomizedWindowsOSImage Distro = "CustomizedWindowsOSImage"

	// USNatCloud is a const string reference identifier for USNat
	USNatCloud = "USNatCloud"
	// USSecCloud is a const string reference identifier for USSec
	USSecCloud = "USSecCloud"
)

var AKSDistrosAvailableOnVHD []Distro = []Distro{
	AKSUbuntu1604,
	AKSUbuntu1804,
	AKSUbuntu1804Gen2,
	AKSUbuntuGPU1804,
	AKSUbuntuGPU1804Gen2,
	AKSUbuntuContainerd1804,
	AKSUbuntuContainerd1804Gen2,
	AKSUbuntuGPUContainerd1804,
	AKSUbuntuGPUContainerd1804Gen2,
	AKSCBLMarinerV1,
	AKSCBLMarinerV2Gen2,
	AKSUbuntuFipsContainerd1804,
	AKSUbuntuFipsContainerd1804Gen2,
	AKSUbuntuFipsGPUContainerd1804,
	AKSUbuntuFipsGPUContainerd1804Gen2,
	AKSUbuntuArm64Containerd1804Gen2,
	AKSUbuntuContainerd2204,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuContainerd2004CVMGen2,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSCBLMarinerV2Arm64Gen2,
}

type CustomConfigurationComponent string

const (
	ComponentkubeProxy CustomConfigurationComponent = "kube-proxy"
	Componentkubelet   CustomConfigurationComponent = "kubelet"
)

func (d Distro) IsVHDDistro() bool {
	for _, distro := range AKSDistrosAvailableOnVHD {
		if d == distro {
			return true
		}
	}
	return false
}

// KeyvaultSecretRef specifies path to the Azure keyvault along with secret name and (optionaly) version
// for Service Principal's secret
type KeyvaultSecretRef struct {
	VaultID       string `json:"vaultID"`
	SecretName    string `json:"secretName"`
	SecretVersion string `json:"version,omitempty"`
}

// AuthenticatorType represents the authenticator type the cluster was
// set up with.
type AuthenticatorType string

const (
	// OIDC represent cluster setup in OIDC auth mode
	OIDC AuthenticatorType = "oidc"
	// Webhook represent cluster setup in wehhook auth mode
	Webhook AuthenticatorType = "webhook"
)

// UserAssignedIdentity contains information that uniquely identifies an identity
type UserAssignedIdentity struct {
	ResourceID string `json:"resourceId,omitempty"`
	ClientID   string `json:"clientId,omitempty"`
	ObjectID   string `json:"objectId,omitempty"`
}

// ResourceIdentifiers represents resource ids
type ResourceIdentifiers struct {
	Graph               string `json:"graph,omitempty"`
	KeyVault            string `json:"keyVault,omitempty"`
	Datalake            string `json:"datalake,omitempty"`
	Batch               string `json:"batch,omitempty"`
	OperationalInsights string `json:"operationalInsights,omitempty"`
	Storage             string `json:"storage,omitempty"`
}

// CustomCloudEnv represents the custom cloud env info of the AKS cluster.
type CustomCloudEnv struct {
	Name                         string              `json:"Name,omitempty"`
	McrURL                       string              `json:"mcrURL,omitempty"`
	RepoDepotEndpoint            string              `json:"repoDepotEndpoint,omitempty"`
	ManagementPortalURL          string              `json:"managementPortalURL,omitempty"`
	PublishSettingsURL           string              `json:"publishSettingsURL,omitempty"`
	ServiceManagementEndpoint    string              `json:"serviceManagementEndpoint,omitempty"`
	ResourceManagerEndpoint      string              `json:"resourceManagerEndpoint,omitempty"`
	ActiveDirectoryEndpoint      string              `json:"activeDirectoryEndpoint,omitempty"`
	GalleryEndpoint              string              `json:"galleryEndpoint,omitempty"`
	KeyVaultEndpoint             string              `json:"keyVaultEndpoint,omitempty"`
	GraphEndpoint                string              `json:"graphEndpoint,omitempty"`
	ServiceBusEndpoint           string              `json:"serviceBusEndpoint,omitempty"`
	BatchManagementEndpoint      string              `json:"batchManagementEndpoint,omitempty"`
	StorageEndpointSuffix        string              `json:"storageEndpointSuffix,omitempty"`
	SQLDatabaseDNSSuffix         string              `json:"sqlDatabaseDNSSuffix,omitempty"`
	TrafficManagerDNSSuffix      string              `json:"trafficManagerDNSSuffix,omitempty"`
	KeyVaultDNSSuffix            string              `json:"keyVaultDNSSuffix,omitempty"`
	ServiceBusEndpointSuffix     string              `json:"serviceBusEndpointSuffix,omitempty"`
	ServiceManagementVMDNSSuffix string              `json:"serviceManagementVMDNSSuffix,omitempty"`
	ResourceManagerVMDNSSuffix   string              `json:"resourceManagerVMDNSSuffix,omitempty"`
	ContainerRegistryDNSSuffix   string              `json:"containerRegistryDNSSuffix,omitempty"`
	CosmosDBDNSSuffix            string              `json:"cosmosDBDNSSuffix,omitempty"`
	TokenAudience                string              `json:"tokenAudience,omitempty"`
	ResourceIdentifiers          ResourceIdentifiers `json:"resourceIdentifiers,omitempty"`
}

// TelemetryProfile contains settings for collecting telemtry.
// Note telemtry is currently enabled/disabled with the 'EnableTelemetry' feature flag.
type TelemetryProfile struct {
	ApplicationInsightsKey string `json:"applicationInsightsKey,omitempty"`
}

// FeatureFlags defines feature-flag restricted functionality
type FeatureFlags struct {
	EnableCSERunInBackground bool `json:"enableCSERunInBackground,omitempty"`
	BlockOutboundInternet    bool `json:"blockOutboundInternet,omitempty"`
	EnableIPv6DualStack      bool `json:"enableIPv6DualStack,omitempty"`
	EnableTelemetry          bool `json:"enableTelemetry,omitempty"`
	EnableIPv6Only           bool `json:"enableIPv6Only,omitempty"`
	EnableWinDSR             bool `json:"enableWinDSR,omitempty"`
}

// AddonProfile represents an addon for managed cluster
type AddonProfile struct {
	Enabled bool              `json:"enabled"`
	Config  map[string]string `json:"config"`
	// Identity contains information of the identity associated with this addon.
	// This property will only appear in an MSI-enabled cluster.
	Identity *UserAssignedIdentity `json:"identity,omitempty"`
}

// HostedMasterProfile defines properties for a hosted master
type HostedMasterProfile struct {
	// Master public endpoint/FQDN with port
	// The format will be FQDN:2376
	// Not used during PUT, returned as part of GETFQDN
	FQDN string `json:"fqdn,omitempty"`
	// IPAddress
	// if both FQDN and IPAddress are specified, we should use IPAddress
	IPAddress string `json:"ipAddress,omitempty"`
	DNSPrefix string `json:"dnsPrefix"`
	// FQDNSubdomain is used by private cluster without dnsPrefix so they have fixed FQDN
	FQDNSubdomain string `json:"fqdnSubdomain"`
	// Subnet holds the CIDR which defines the Azure Subnet in which
	// Agents will be provisioned. This is stored on the HostedMasterProfile
	// and will become `masterSubnet` in the compiled template.
	Subnet string `json:"subnet"`
	// ApiServerWhiteListRange is a comma delimited CIDR which is whitelisted to AKS
	APIServerWhiteListRange *string `json:"apiServerWhiteListRange"`
	IPMasqAgent             bool    `json:"ipMasqAgent"`
}

// CustomProfile specifies custom properties that are used for
// cluster instantiation.  Should not be used by most users.
type CustomProfile struct {
	Orchestrator string `json:"orchestrator,omitempty"`
}

// AADProfile specifies attributes for AAD integration
type AADProfile struct {
	// The client AAD application ID.
	ClientAppID string `json:"clientAppID,omitempty"`
	// The server AAD application ID.
	ServerAppID string `json:"serverAppID,omitempty"`
	// The server AAD application secret
	ServerAppSecret string `json:"serverAppSecret,omitempty" conform:"redact"`
	// The AAD tenant ID to use for authentication.
	// If not specified, will use the tenant of the deployment subscription.
	// Optional
	TenantID string `json:"tenantID,omitempty"`
	// The Azure Active Directory Group Object ID that will be assigned the
	// cluster-admin RBAC role.
	// Optional
	AdminGroupID string `json:"adminGroupID,omitempty"`
	// The authenticator to use, either "oidc" or "webhook".
	Authenticator AuthenticatorType `json:"authenticator"`
}

// CertificateProfile represents the definition of the master cluster
type CertificateProfile struct {
	// CaCertificate is the certificate authority certificate.
	CaCertificate string `json:"caCertificate,omitempty" conform:"redact"`
	// ApiServerCertificate is the rest api server certificate, and signed by the CA
	APIServerCertificate string `json:"apiServerCertificate,omitempty" conform:"redact"`
	// ClientCertificate is the certificate used by the client kubelet services and signed by the CA
	ClientCertificate string `json:"clientCertificate,omitempty" conform:"redact"`
	// ClientPrivateKey is the private key used by the client kubelet services and signed by the CA
	ClientPrivateKey string `json:"clientPrivateKey,omitempty" conform:"redact"`
	// KubeConfigCertificate is the client certificate used for kubectl cli and signed by the CA
	KubeConfigCertificate string `json:"kubeConfigCertificate,omitempty" conform:"redact"`
	// KubeConfigPrivateKey is the client private key used for kubectl cli and signed by the CA
	KubeConfigPrivateKey string `json:"kubeConfigPrivateKey,omitempty" conform:"redact"`
}

// ServicePrincipalProfile contains the client and secret used by the cluster for Azure Resource CRUD
type ServicePrincipalProfile struct {
	ClientID          string             `json:"clientId"`
	Secret            string             `json:"secret,omitempty" conform:"redact"`
	ObjectID          string             `json:"objectId,omitempty"`
	KeyvaultSecretRef *KeyvaultSecretRef `json:"keyvaultSecretRef,omitempty"`
}

// DiagnosticsProfile setting to enable/disable capturing
// diagnostics for VMs hosting container cluster.
type DiagnosticsProfile struct {
	VMDiagnostics *VMDiagnostics `json:"vmDiagnostics"`
}

// ExtensionProfile represents an extension definition
type ExtensionProfile struct {
	Name                           string             `json:"name"`
	Version                        string             `json:"version"`
	ExtensionParameters            string             `json:"extensionParameters,omitempty"`
	ExtensionParametersKeyVaultRef *KeyvaultSecretRef `json:"parametersKeyvaultSecretRef,omitempty"`
	RootURL                        string             `json:"rootURL,omitempty"`
	// This is only needed for preprovision extensions and it needs to be a bash script
	Script   string `json:"script,omitempty"`
	URLQuery string `json:"urlQuery,omitempty"`
}

// ResourcePurchasePlan defines resource plan as required by ARM
// for billing purposes.
type ResourcePurchasePlan struct {
	Name          string `json:"name"`
	Product       string `json:"product"`
	PromotionCode string `json:"promotionCode"`
	Publisher     string `json:"publisher"`
}

// WindowsProfile represents the windows parameters passed to the cluster
type WindowsProfile struct {
	AdminUsername                 string                     `json:"adminUsername"`
	AdminPassword                 string                     `json:"adminPassword" conform:"redact"`
	CSIProxyURL                   string                     `json:"csiProxyURL,omitempty"`
	EnableCSIProxy                *bool                      `json:"enableCSIProxy,omitempty"`
	ImageRef                      *ImageReference            `json:"imageReference,omitempty"`
	ImageVersion                  string                     `json:"imageVersion"`
	ProvisioningScriptsPackageURL string                     `json:"provisioningScriptsPackageURL,omitempty"`
	WindowsImageSourceURL         string                     `json:"windowsImageSourceURL"`
	WindowsPublisher              string                     `json:"windowsPublisher"`
	WindowsOffer                  string                     `json:"windowsOffer"`
	WindowsSku                    string                     `json:"windowsSku"`
	WindowsDockerVersion          string                     `json:"windowsDockerVersion"`
	Secrets                       []KeyVaultSecrets          `json:"secrets,omitempty"`
	SSHEnabled                    *bool                      `json:"sshEnabled,omitempty"`
	EnableAutomaticUpdates        *bool                      `json:"enableAutomaticUpdates,omitempty"`
	IsCredentialAutoGenerated     *bool                      `json:"isCredentialAutoGenerated,omitempty"`
	EnableAHUB                    *bool                      `json:"enableAHUB,omitempty"`
	WindowsPauseImageURL          string                     `json:"windowsPauseImageURL"`
	AlwaysPullWindowsPauseImage   *bool                      `json:"alwaysPullWindowsPauseImage,omitempty"`
	ContainerdWindowsRuntimes     *ContainerdWindowsRuntimes `json:"containerdWindowsRuntimes,omitempty"`
	WindowsCalicoPackageURL       string                     `json:"windowsCalicoPackageURL,omitempty"`
	WindowsSecureTlsEnabled       *bool                      `json:"windowsSecureTlsEnabled,omitempty"`
	WindowsGmsaPackageUrl         string                     `json:"windowsGmsaPackageUrl,omitempty"`
	CseScriptsPackageURL          string                     `json:"cseScriptsPackageURL,omitempty"`
}

// ContainerdWindowsRuntimes configures containerd runtimes that are available on the windows nodes
type ContainerdWindowsRuntimes struct {
	DefaultSandboxIsolation string            `json:"defaultSandboxIsolation,omitempty"`
	RuntimeHandlers         []RuntimeHandlers `json:"runtimesHandlers,omitempty"`
}

// RuntimeHandlers configures the runtime settings in containerd
type RuntimeHandlers struct {
	BuildNumber string `json:"buildNumber,omitempty"`
}

// LinuxProfile represents the linux parameters passed to the cluster
type LinuxProfile struct {
	AdminUsername string `json:"adminUsername"`
	SSH           struct {
		PublicKeys []PublicKey `json:"publicKeys"`
	} `json:"ssh"`
	Secrets            []KeyVaultSecrets   `json:"secrets,omitempty"`
	Distro             Distro              `json:"distro,omitempty"`
	CustomSearchDomain *CustomSearchDomain `json:"customSearchDomain,omitempty"`
}

// Extension represents an extension definition in the master or agentPoolProfile
type Extension struct {
	Name        string `json:"name"`
	SingleOrAll string `json:"singleOrAll"`
	Template    string `json:"template"`
}

// PrivateJumpboxProfile represents a jumpbox definition
type PrivateJumpboxProfile struct {
	Name           string `json:"name" validate:"required"`
	VMSize         string `json:"vmSize" validate:"required"`
	OSDiskSizeGB   int    `json:"osDiskSizeGB,omitempty" validate:"min=0,max=2048"`
	Username       string `json:"username,omitempty"`
	PublicKey      string `json:"publicKey" validate:"required"`
	StorageProfile string `json:"storageProfile,omitempty"`
}

// PrivateCluster defines the configuration for a private cluster
type PrivateCluster struct {
	Enabled                *bool                  `json:"enabled,omitempty"`
	EnableHostsConfigAgent *bool                  `json:"enableHostsConfigAgent,omitempty"`
	JumpboxProfile         *PrivateJumpboxProfile `json:"jumpboxProfile,omitempty"`
}

// KubernetesContainerSpec defines configuration for a container spec
type KubernetesContainerSpec struct {
	Name           string `json:"name,omitempty"`
	Image          string `json:"image,omitempty"`
	CPURequests    string `json:"cpuRequests,omitempty"`
	MemoryRequests string `json:"memoryRequests,omitempty"`
	CPULimits      string `json:"cpuLimits,omitempty"`
	MemoryLimits   string `json:"memoryLimits,omitempty"`
}

// AddonNodePoolsConfig defines configuration for pool-specific cluster-autoscaler configuration
type AddonNodePoolsConfig struct {
	Name   string            `json:"name,omitempty"`
	Config map[string]string `json:"config,omitempty"`
}

// KubernetesAddon defines a list of addons w/ configuration to include with the cluster deployment
type KubernetesAddon struct {
	Name       string                    `json:"name,omitempty"`
	Enabled    *bool                     `json:"enabled,omitempty"`
	Mode       string                    `json:"mode,omitempty"`
	Containers []KubernetesContainerSpec `json:"containers,omitempty"`
	Config     map[string]string         `json:"config,omitempty"`
	Pools      []AddonNodePoolsConfig    `json:"pools,omitempty"`
	Data       string                    `json:"data,omitempty"`
}

// KubernetesConfig contains the Kubernetes config structure, containing
// Kubernetes specific configuration
type KubernetesConfig struct {
	KubernetesImageBase               string            `json:"kubernetesImageBase,omitempty"`
	MCRKubernetesImageBase            string            `json:"mcrKubernetesImageBase,omitempty"`
	ClusterSubnet                     string            `json:"clusterSubnet,omitempty"`
	NetworkPolicy                     string            `json:"networkPolicy,omitempty"`
	NetworkPlugin                     string            `json:"networkPlugin,omitempty"`
	NetworkMode                       string            `json:"networkMode,omitempty"`
	ContainerRuntime                  string            `json:"containerRuntime,omitempty"`
	MaxPods                           int               `json:"maxPods,omitempty"`
	DockerBridgeSubnet                string            `json:"dockerBridgeSubnet,omitempty"`
	DNSServiceIP                      string            `json:"dnsServiceIP,omitempty"`
	ServiceCIDR                       string            `json:"serviceCidr,omitempty"`
	UseManagedIdentity                bool              `json:"useManagedIdentity,omitempty"`
	UserAssignedID                    string            `json:"userAssignedID,omitempty"`
	UserAssignedClientID              string            `json:"userAssignedClientID,omitempty"` //Note: cannot be provided in config. Used *only* for transferring this to azure.json.
	CustomHyperkubeImage              string            `json:"customHyperkubeImage,omitempty"`
	CustomKubeProxyImage              string            `json:"customKubeProxyImage,omitempty"`
	CustomKubeBinaryURL               string            `json:"customKubeBinaryURL,omitempty"`
	MobyVersion                       string            `json:"mobyVersion,omitempty"`
	ContainerdVersion                 string            `json:"containerdVersion,omitempty"`
	WindowsNodeBinariesURL            string            `json:"windowsNodeBinariesURL,omitempty"`
	WindowsContainerdURL              string            `json:"windowsContainerdURL,omitempty"`
	WindowsSdnPluginURL               string            `json:"windowsSdnPluginURL,omitempty"`
	UseInstanceMetadata               *bool             `json:"useInstanceMetadata,omitempty"`
	EnableRbac                        *bool             `json:"enableRbac,omitempty"`
	EnableSecureKubelet               *bool             `json:"enableSecureKubelet,omitempty"`
	PrivateCluster                    *PrivateCluster   `json:"privateCluster,omitempty"`
	GCHighThreshold                   int               `json:"gchighthreshold,omitempty"`
	GCLowThreshold                    int               `json:"gclowthreshold,omitempty"`
	EnableEncryptionWithExternalKms   *bool             `json:"enableEncryptionWithExternalKms,omitempty"`
	Addons                            []KubernetesAddon `json:"addons,omitempty"`
	ContainerRuntimeConfig            map[string]string `json:"containerRuntimeConfig,omitempty"`
	ControllerManagerConfig           map[string]string `json:"controllerManagerConfig,omitempty"`
	SchedulerConfig                   map[string]string `json:"schedulerConfig,omitempty"`
	CloudProviderBackoffMode          string            `json:"cloudProviderBackoffMode"`
	CloudProviderBackoff              *bool             `json:"cloudProviderBackoff,omitempty"`
	CloudProviderBackoffRetries       int               `json:"cloudProviderBackoffRetries,omitempty"`
	CloudProviderBackoffJitter        float64           `json:"cloudProviderBackoffJitter,omitempty"`
	CloudProviderBackoffDuration      int               `json:"cloudProviderBackoffDuration,omitempty"`
	CloudProviderBackoffExponent      float64           `json:"cloudProviderBackoffExponent,omitempty"`
	CloudProviderRateLimit            *bool             `json:"cloudProviderRateLimit,omitempty"`
	CloudProviderRateLimitQPS         float64           `json:"cloudProviderRateLimitQPS,omitempty"`
	CloudProviderRateLimitQPSWrite    float64           `json:"cloudProviderRateLimitQPSWrite,omitempty"`
	CloudProviderRateLimitBucket      int               `json:"cloudProviderRateLimitBucket,omitempty"`
	CloudProviderRateLimitBucketWrite int               `json:"cloudProviderRateLimitBucketWrite,omitempty"`
	CloudProviderDisableOutboundSNAT  *bool             `json:"cloudProviderDisableOutboundSNAT,omitempty"`
	NodeStatusUpdateFrequency         string            `json:"nodeStatusUpdateFrequency,omitempty"`
	LoadBalancerSku                   string            `json:"loadBalancerSku,omitempty"`
	ExcludeMasterFromStandardLB       *bool             `json:"excludeMasterFromStandardLB,omitempty"`
	AzureCNIURLLinux                  string            `json:"azureCNIURLLinux,omitempty"`
	AzureCNIURLARM64Linux             string            `json:"azureCNIURLARM64Linux,omitempty"`
	AzureCNIURLWindows                string            `json:"azureCNIURLWindows,omitempty"`
	MaximumLoadBalancerRuleCount      int               `json:"maximumLoadBalancerRuleCount,omitempty"`
	PrivateAzureRegistryServer        string            `json:"privateAzureRegistryServer,omitempty"`
}

// CustomFile has source as the full absolute source path to a file and dest
// is the full absolute desired destination path to put the file on a master node
type CustomFile struct {
	Source string `json:"source,omitempty"`
	Dest   string `json:"dest,omitempty"`
}

// OrchestratorProfile contains Orchestrator properties
type OrchestratorProfile struct {
	OrchestratorType    string            `json:"orchestratorType"`
	OrchestratorVersion string            `json:"orchestratorVersion"`
	KubernetesConfig    *KubernetesConfig `json:"kubernetesConfig,omitempty"`
}

// ProvisioningState represents the current state of container service resource.
type ProvisioningState string

// CustomKubeletConfig represents custom kubelet configurations for agent pool nodes
type CustomKubeletConfig struct {
	CPUManagerPolicy      string    `json:"cpuManagerPolicy,omitempty"`
	CPUCfsQuota           *bool     `json:"cpuCfsQuota,omitempty"`
	CPUCfsQuotaPeriod     string    `json:"cpuCfsQuotaPeriod,omitempty"`
	ImageGcHighThreshold  *int32    `json:"imageGcHighThreshold,omitempty"`
	ImageGcLowThreshold   *int32    `json:"imageGcLowThreshold,omitempty"`
	TopologyManagerPolicy string    `json:"topologyManagerPolicy,omitempty"`
	AllowedUnsafeSysctls  *[]string `json:"allowedUnsafeSysctls,omitempty"`
	FailSwapOn            *bool     `json:"failSwapOn,omitempty"`
	ContainerLogMaxSizeMB *int32    `json:"containerLogMaxSizeMB,omitempty"`
	ContainerLogMaxFiles  *int32    `json:"containerLogMaxFiles,omitempty"`
	PodMaxPids            *int32    `json:"podMaxPids,omitempty"`
}

// CustomLinuxOSConfig represents custom os configurations for agent pool nodes
type CustomLinuxOSConfig struct {
	Sysctls                    *SysctlConfig `json:"sysctls,omitempty"`
	TransparentHugePageEnabled string        `json:"transparentHugePageEnabled,omitempty"`
	TransparentHugePageDefrag  string        `json:"transparentHugePageDefrag,omitempty"`
	SwapFileSizeMB             *int32        `json:"swapFileSizeMB,omitempty"`
}

// SysctlConfig represents sysctl configs in customLinuxOsConfig
type SysctlConfig struct {
	NetCoreSomaxconn               *int32 `json:"netCoreSomaxconn,omitempty"`
	NetCoreNetdevMaxBacklog        *int32 `json:"netCoreNetdevMaxBacklog,omitempty"`
	NetCoreRmemDefault             *int32 `json:"netCoreRmemDefault,omitempty"`
	NetCoreRmemMax                 *int32 `json:"netCoreRmemMax,omitempty"`
	NetCoreWmemDefault             *int32 `json:"netCoreWmemDefault,omitempty"`
	NetCoreWmemMax                 *int32 `json:"netCoreWmemMax,omitempty"`
	NetCoreOptmemMax               *int32 `json:"netCoreOptmemMax,omitempty"`
	NetIpv4TcpMaxSynBacklog        *int32 `json:"netIpv4TcpMaxSynBacklog,omitempty"`
	NetIpv4TcpMaxTwBuckets         *int32 `json:"netIpv4TcpMaxTwBuckets,omitempty"`
	NetIpv4TcpFinTimeout           *int32 `json:"netIpv4TcpFinTimeout,omitempty"`
	NetIpv4TcpKeepaliveTime        *int32 `json:"netIpv4TcpKeepaliveTime,omitempty"`
	NetIpv4TcpKeepaliveProbes      *int32 `json:"netIpv4TcpKeepaliveProbes,omitempty"`
	NetIpv4TcpkeepaliveIntvl       *int32 `json:"netIpv4TcpkeepaliveIntvl,omitempty"`
	NetIpv4TcpTwReuse              *bool  `json:"netIpv4TcpTwReuse,omitempty"`
	NetIpv4IpLocalPortRange        string `json:"netIpv4IpLocalPortRange,omitempty"`
	NetIpv4NeighDefaultGcThresh1   *int32 `json:"netIpv4NeighDefaultGcThresh1,omitempty"`
	NetIpv4NeighDefaultGcThresh2   *int32 `json:"netIpv4NeighDefaultGcThresh2,omitempty"`
	NetIpv4NeighDefaultGcThresh3   *int32 `json:"netIpv4NeighDefaultGcThresh3,omitempty"`
	NetNetfilterNfConntrackMax     *int32 `json:"netNetfilterNfConntrackMax,omitempty"`
	NetNetfilterNfConntrackBuckets *int32 `json:"netNetfilterNfConntrackBuckets,omitempty"`
	FsInotifyMaxUserWatches        *int32 `json:"fsInotifyMaxUserWatches,omitempty"`
	FsFileMax                      *int32 `json:"fsFileMax,omitempty"`
	FsAioMaxNr                     *int32 `json:"fsAioMaxNr,omitempty"`
	FsNrOpen                       *int32 `json:"fsNrOpen,omitempty"`
	KernelThreadsMax               *int32 `json:"kernelThreadsMax,omitempty"`
	VMMaxMapCount                  *int32 `json:"vmMaxMapCount,omitempty"`
	VMSwappiness                   *int32 `json:"vmSwappiness,omitempty"`
	VMVfsCachePressure             *int32 `json:"vmVfsCachePressure,omitempty"`
}

type CustomConfiguration struct {
	KubernetesConfigurations        map[string]*ComponentConfiguration
	WindowsKubernetesConfigurations map[string]*ComponentConfiguration
}

type ComponentConfiguration struct {
	Image       *string
	Config      map[string]string
	DownloadURL *string
}

// AgentPoolProfile represents an agent pool definition
type AgentPoolProfile struct {
	Name                  string               `json:"name"`
	VMSize                string               `json:"vmSize"`
	KubeletDiskType       KubeletDiskType      `json:"kubeletDiskType,omitempty"`
	WorkloadRuntime       WorkloadRuntime      `json:"workloadRuntime,omitempty"`
	DNSPrefix             string               `json:"dnsPrefix,omitempty"`
	OSType                OSType               `json:"osType,omitempty"`
	Ports                 []int                `json:"ports,omitempty"`
	AvailabilityProfile   string               `json:"availabilityProfile"`
	StorageProfile        string               `json:"storageProfile,omitempty"`
	VnetSubnetID          string               `json:"vnetSubnetID,omitempty"`
	Distro                Distro               `json:"distro,omitempty"`
	CustomNodeLabels      map[string]string    `json:"customNodeLabels,omitempty"`
	PreprovisionExtension *Extension           `json:"preProvisionExtension"`
	KubernetesConfig      *KubernetesConfig    `json:"kubernetesConfig,omitempty"`
	VnetCidrs             []string             `json:"vnetCidrs,omitempty"`
	WindowsNameVersion    string               `json:"windowsNameVersion,omitempty"`
	CustomKubeletConfig   *CustomKubeletConfig `json:"customKubeletConfig,omitempty"`
	CustomLinuxOSConfig   *CustomLinuxOSConfig `json:"customLinuxOSConfig,omitempty"`
	MessageOfTheDay       string               `json:"messageOfTheDay,omitempty"`
	// This is a new property and all old agent pools do no have this field. We need to keep the default
	// behavior to reboot Windows node when it is nil
	NotRebootWindowsNode *bool `json:"notRebootWindowsNode,omitempty"`
}

// Properties represents the AKS cluster definition
type Properties struct {
	ClusterID               string
	ProvisioningState       ProvisioningState        `json:"provisioningState,omitempty"`
	OrchestratorProfile     *OrchestratorProfile     `json:"orchestratorProfile,omitempty"`
	AgentPoolProfiles       []*AgentPoolProfile      `json:"agentPoolProfiles,omitempty"`
	LinuxProfile            *LinuxProfile            `json:"linuxProfile,omitempty"`
	WindowsProfile          *WindowsProfile          `json:"windowsProfile,omitempty"`
	ExtensionProfiles       []*ExtensionProfile      `json:"extensionProfiles"`
	DiagnosticsProfile      *DiagnosticsProfile      `json:"diagnosticsProfile,omitempty"`
	ServicePrincipalProfile *ServicePrincipalProfile `json:"servicePrincipalProfile,omitempty"`
	CertificateProfile      *CertificateProfile      `json:"certificateProfile,omitempty"`
	AADProfile              *AADProfile              `json:"aadProfile,omitempty"`
	CustomProfile           *CustomProfile           `json:"customProfile,omitempty"`
	HostedMasterProfile     *HostedMasterProfile     `json:"hostedMasterProfile,omitempty"`
	AddonProfiles           map[string]AddonProfile  `json:"addonProfiles,omitempty"`
	FeatureFlags            *FeatureFlags            `json:"featureFlags,omitempty"`
	TelemetryProfile        *TelemetryProfile        `json:"telemetryProfile,omitempty"`
	CustomCloudEnv          *CustomCloudEnv          `json:"customCloudEnv,omitempty"`
	CustomConfiguration     *CustomConfiguration     `json:"customConfiguration,omitempty"`
}

// ContainerService complies with the ARM model of
// resource definition in a JSON template.
type ContainerService struct {
	ID       string                `json:"id"`
	Location string                `json:"location"`
	Name     string                `json:"name"`
	Plan     *ResourcePurchasePlan `json:"plan,omitempty"`
	Tags     map[string]string     `json:"tags"`
	Type     string                `json:"type"`

	Properties *Properties `json:"properties,omitempty"`
}

// IsAKSCustomCloud checks if it's in AKS custom cloud
func (cs *ContainerService) IsAKSCustomCloud() bool {
	return cs.Properties.CustomCloudEnv != nil &&
		strings.EqualFold(cs.Properties.CustomCloudEnv.Name, "akscustom")
}

// HasAadProfile returns true if the has aad profile
func (p *Properties) HasAadProfile() bool {
	return p.AADProfile != nil
}

// GetCustomCloudName returns name of environment if customCloudProfile is provided, returns empty string if customCloudProfile is empty.
// Because customCloudProfile is empty for deployment is AzurePublicCloud, AzureChinaCloud,AzureGermanCloud,AzureUSGovernmentCloud,
// the return value will be empty string for those clouds
func (p *Properties) GetCustomCloudName() string {
	var cloudProfileName string
	if p.IsAKSCustomCloud() {
		cloudProfileName = p.CustomCloudEnv.Name
	}
	return cloudProfileName
}

// IsIPMasqAgentDisabled returns true if the ip-masq-agent functionality is disabled
func (p *Properties) IsIPMasqAgentDisabled() bool {
	if p.HostedMasterProfile != nil {
		return !p.HostedMasterProfile.IPMasqAgent
	}
	if p.OrchestratorProfile != nil && p.OrchestratorProfile.KubernetesConfig != nil {
		return p.OrchestratorProfile.KubernetesConfig.IsIPMasqAgentDisabled()
	}
	return false
}

// HasWindows returns true if the cluster contains windows
func (p *Properties) HasWindows() bool {
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if strings.EqualFold(string(agentPoolProfile.OSType), string(Windows)) {
			return true
		}
	}
	return false
}

// IsAKSCustomCloud checks if it's in AKS custom cloud
func (p *Properties) IsAKSCustomCloud() bool {
	return p.CustomCloudEnv != nil &&
		strings.EqualFold(p.CustomCloudEnv.Name, "akscustom")
}

// IsIPMasqAgentEnabled returns true if the cluster has a hosted master and IpMasqAgent is disabled
func (p *Properties) IsIPMasqAgentEnabled() bool {
	if p.HostedMasterProfile != nil {
		return p.HostedMasterProfile.IPMasqAgent
	}
	return p.OrchestratorProfile.KubernetesConfig.IsIPMasqAgentEnabled()
}

// GetClusterID creates a unique 8 string cluster ID.
func (p *Properties) GetClusterID() string {
	var mutex = &sync.Mutex{}
	if p.ClusterID == "" {
		uniqueNameSuffixSize := 8
		// the name suffix uniquely identifies the cluster and is generated off a hash
		// from the master dns name
		h := fnv.New64a()
		if p.HostedMasterProfile != nil {
			h.Write([]byte(p.HostedMasterProfile.DNSPrefix))
		} else if len(p.AgentPoolProfiles) > 0 {
			h.Write([]byte(p.AgentPoolProfiles[0].Name))
		}
		r := rand.New(rand.NewSource(int64(h.Sum64())))
		mutex.Lock()
		p.ClusterID = fmt.Sprintf("%08d", r.Uint32())[:uniqueNameSuffixSize]
		mutex.Unlock()
	}
	return p.ClusterID
}

// AreAgentProfilesCustomVNET returns true if all of the agent profiles in the clusters are configured with VNET.
func (p *Properties) AreAgentProfilesCustomVNET() bool {
	if p.AgentPoolProfiles != nil {
		for _, agentPoolProfile := range p.AgentPoolProfiles {
			if !agentPoolProfile.IsCustomVNET() {
				return false
			}
		}
		return true
	}
	return false
}

// GetCustomEnvironmentJSON return the JSON format string for custom environment
func (p *Properties) GetCustomEnvironmentJSON(escape bool) (string, error) {
	var environmentJSON string
	if p.IsAKSCustomCloud() {
		// Workaround to set correct name in AzureStackCloud.json
		oldName := p.CustomCloudEnv.Name
		p.CustomCloudEnv.Name = AzureStackCloud
		defer func() {
			// Restore p.CustomCloudEnv to old value
			p.CustomCloudEnv.Name = oldName
		}()
		bytes, err := json.Marshal(p.CustomCloudEnv)
		if err != nil {
			return "", fmt.Errorf("Could not serialize CustomCloudEnv object - %s", err.Error())
		}
		environmentJSON = string(bytes)
		if escape {
			environmentJSON = strings.Replace(environmentJSON, "\"", "\\\"", -1)
		}
	}
	return environmentJSON, nil
}

// HasDCSeriesSKU returns whether or not there is an DC series SKU agent pool
func (p *Properties) HasDCSeriesSKU() bool {
	for _, profile := range p.AgentPoolProfiles {
		if strings.Contains(profile.VMSize, "Standard_DC") {
			return true
		}
	}
	return false
}

// K8sOrchestratorName returns the 3 character orchestrator code for kubernetes-based clusters.
func (p *Properties) K8sOrchestratorName() string {
	if p.OrchestratorProfile.IsKubernetes() {
		if p.HostedMasterProfile != nil {
			return DefaultHostedProfileMasterName
		}
		return DefaultOrchestratorName
	}
	return ""
}

// IsVHDDistroForAllNodes returns true if all of the agent pools plus masters are running the VHD image
func (p *Properties) IsVHDDistroForAllNodes() bool {
	if len(p.AgentPoolProfiles) > 0 {
		for _, ap := range p.AgentPoolProfiles {
			if !ap.IsVHDDistro() {
				return false
			}
		}
	}
	return true
}

// GetVMType returns the type of VM "vmss" or "standard" to be passed to the cloud provider
func (p *Properties) GetVMType() string {
	if p.HasVMSSAgentPool() {
		return VMSSVMType
	}
	return StandardVMType
}

// HasVMSSAgentPool returns true if the cluster contains Virtual Machine Scale Sets agent pools
func (p *Properties) HasVMSSAgentPool() bool {
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if strings.EqualFold(agentPoolProfile.AvailabilityProfile, VirtualMachineScaleSets) {
			return true
		}
	}
	return false
}

// GetSubnetName returns the subnet name of the cluster based on its current configuration.
func (p *Properties) GetSubnetName() string {
	var subnetName string

	if p.AreAgentProfilesCustomVNET() {
		subnetName = strings.Split(p.AgentPoolProfiles[0].VnetSubnetID, "/")[DefaultSubnetNameResourceSegmentIndex]
	} else {
		subnetName = p.K8sOrchestratorName() + "-subnet"
	}

	return subnetName
}

// GetNSGName returns the name of the network security group of the cluster.
func (p *Properties) GetNSGName() string {
	return p.GetResourcePrefix() + "nsg"
}

// GetResourcePrefix returns the prefix to use for naming cluster resources
func (p *Properties) GetResourcePrefix() string {
	return p.K8sOrchestratorName() + "-agentpool-" + p.GetClusterID() + "-"
}

// GetVirtualNetworkName returns the virtual network name of the cluster
func (p *Properties) GetVirtualNetworkName() string {
	var vnetName string
	if p.AreAgentProfilesCustomVNET() {
		vnetName = strings.Split(p.AgentPoolProfiles[0].VnetSubnetID, "/")[DefaultVnetNameResourceSegmentIndex]
	} else {
		vnetName = p.K8sOrchestratorName() + "-vnet-" + p.GetClusterID()
	}
	return vnetName
}

// GetVNetResourceGroupName returns the virtual network resource group name of the cluster
func (p *Properties) GetVNetResourceGroupName() string {
	var vnetResourceGroupName string
	if p.AreAgentProfilesCustomVNET() {
		vnetResourceGroupName = strings.Split(p.AgentPoolProfiles[0].VnetSubnetID, "/")[DefaultVnetResourceGroupSegmentIndex]
	}
	return vnetResourceGroupName
}

// GetRouteTableName returns the route table name of the cluster.
func (p *Properties) GetRouteTableName() string {
	return p.GetResourcePrefix() + "routetable"
}

// GetPrimaryAvailabilitySetName returns the name of the primary availability set of the cluster
func (p *Properties) GetPrimaryAvailabilitySetName() string {
	if len(p.AgentPoolProfiles) > 0 {
		if strings.EqualFold(p.AgentPoolProfiles[0].AvailabilityProfile, AvailabilitySet) {
			return p.AgentPoolProfiles[0].Name + "-availabilitySet-" + p.GetClusterID()
		}
	}
	return ""
}

func (p *Properties) GetComponentKubernetesConfiguration(component CustomConfigurationComponent) *ComponentConfiguration {
	if p.CustomConfiguration == nil {
		return nil
	}
	if p.CustomConfiguration.KubernetesConfigurations == nil {
		return nil
	}
	if configuration, ok := p.CustomConfiguration.KubernetesConfigurations[string(component)]; ok {
		return configuration
	}

	return nil
}

func (p *Properties) GetComponentWindowsKubernetesConfiguration(component CustomConfigurationComponent) *ComponentConfiguration {
	if p.CustomConfiguration == nil {
		return nil
	}
	if p.CustomConfiguration.WindowsKubernetesConfigurations == nil {
		return nil
	}
	if configuration, ok := p.CustomConfiguration.WindowsKubernetesConfigurations[string(component)]; ok {
		return configuration
	}

	return nil
}

// GetKubeProxyFeatureGatesWindowsArguments returns the feature gates string for the kube-proxy arguments in Windows nodes
func (p *Properties) GetKubeProxyFeatureGatesWindowsArguments() string {
	featureGates := map[string]bool{}

	if p.FeatureFlags.IsFeatureEnabled("EnableIPv6DualStack") {
		featureGates["IPv6DualStack"] = true
	}
	if p.FeatureFlags.IsFeatureEnabled("EnableWinDSR") {
		// WinOverlay must be set to false
		featureGates["WinDSR"] = true
		featureGates["WinOverlay"] = false
	}

	keys := []string{}
	for key := range featureGates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("\"%s=%t\", ", key, featureGates[key]))
	}
	return strings.TrimSuffix(buf.String(), ", ")
}

// IsVHDDistro returns true if the distro uses VHD SKUs
func (a *AgentPoolProfile) IsVHDDistro() bool {
	return a.Distro.IsVHDDistro()
}

// IsCustomVNET returns true if the customer brought their own VNET
func (a *AgentPoolProfile) IsCustomVNET() bool {
	return len(a.VnetSubnetID) > 0
}

// IsWindows returns true if the agent pool is windows
func (a *AgentPoolProfile) IsWindows() bool {
	return strings.EqualFold(string(a.OSType), string(Windows))
}

// IsVirtualMachineScaleSets returns true if the agent pool availability profile is VMSS
func (a *AgentPoolProfile) IsVirtualMachineScaleSets() bool {
	return strings.EqualFold(a.AvailabilityProfile, VirtualMachineScaleSets)
}

// IsAvailabilitySets returns true if the customer specified disks
func (a *AgentPoolProfile) IsAvailabilitySets() bool {
	return strings.EqualFold(a.AvailabilityProfile, AvailabilitySet)
}

// GetKubernetesLabels returns a k8s API-compliant labels string for nodes in this profile
func (a *AgentPoolProfile) GetKubernetesLabels(rg string, deprecated bool, nvidiaEnabled bool, fipsEnabled bool, osSku string) string {
	var buf bytes.Buffer
	buf.WriteString("kubernetes.azure.com/role=agent")
	if deprecated {
		buf.WriteString(",node-role.kubernetes.io/agent=")
		buf.WriteString(",kubernetes.io/role=agent")
	}
	// label key agentpool will be depreated soon
	buf.WriteString(fmt.Sprintf(",agentpool=%s", a.Name))
	buf.WriteString(fmt.Sprintf(",kubernetes.azure.com/agentpool=%s", a.Name))

	if strings.EqualFold(a.StorageProfile, ManagedDisks) {
		storagetier, _ := GetStorageAccountType(a.VMSize)
		// label key storageprofile and storagetier will be depreated soon
		buf.WriteString(fmt.Sprintf(",storageprofile=managed,storagetier=%s", storagetier))
		buf.WriteString(fmt.Sprintf(",kubernetes.azure.com/storageprofile=managed,kubernetes.azure.com/storagetier=%s", storagetier))
	}
	if nvidiaEnabled {
		accelerator := "nvidia"
		// label key accelerator will be depreated soon
		buf.WriteString(fmt.Sprintf(",accelerator=%s", accelerator))
		buf.WriteString(fmt.Sprintf(",kubernetes.azure.com/accelerator=%s", accelerator))
	}
	if fipsEnabled {
		buf.WriteString(",kubernetes.azure.com/fips_enabled=true")
	}
	if osSku != "" {
		buf.WriteString(fmt.Sprintf(",kubernetes.azure.com/os-sku=%s", osSku))
	}
	buf.WriteString(fmt.Sprintf(",kubernetes.azure.com/cluster=%s", rg))
	keys := []string{}
	for key := range a.CustomNodeLabels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf(",%s=%s", key, a.CustomNodeLabels[key]))
	}
	return buf.String()
}

// IsNotRebootWindowsNode returns true if it does not need to reboot Windows node
func (w *AgentPoolProfile) IsNotRebootWindowsNode() bool {
	return w.NotRebootWindowsNode != nil && *w.NotRebootWindowsNode
}

// HasSecrets returns true if the customer specified secrets to install
func (l *LinuxProfile) HasSecrets() bool {
	return len(l.Secrets) > 0
}

// HasSearchDomain returns true if the customer specified secrets to install
func (l *LinuxProfile) HasSearchDomain() bool {
	if l.CustomSearchDomain != nil {
		if l.CustomSearchDomain.Name != "" && l.CustomSearchDomain.RealmPassword != "" && l.CustomSearchDomain.RealmUser != "" {
			return true
		}
	}
	return false
}

// IsAzureCNI returns true if Azure CNI network plugin is enabled
func (o *OrchestratorProfile) IsAzureCNI() bool {
	if o.KubernetesConfig != nil {
		return strings.EqualFold(o.KubernetesConfig.NetworkPlugin, NetworkPluginAzure)
	}
	return false
}

// IsNoneCNI returns true if network plugin none is enabled
func (o *OrchestratorProfile) IsNoneCNI() bool {
	if o.KubernetesConfig != nil {
		return strings.EqualFold(o.KubernetesConfig.NetworkPlugin, NetworkPluginNone)
	}
	return false
}

// IsCSIProxyEnabled returns true if csi proxy service should be enable for Windows nodes
func (w *WindowsProfile) IsCSIProxyEnabled() bool {
	if w.EnableCSIProxy != nil {
		return *w.EnableCSIProxy
	}
	return DefaultEnableCSIProxyWindows
}

// HasSecrets returns true if the customer specified secrets to install
func (w *WindowsProfile) HasSecrets() bool {
	return len(w.Secrets) > 0
}

// HasCustomImage returns true if there is a custom windows os image url specified
func (w *WindowsProfile) HasCustomImage() bool {
	return len(w.WindowsImageSourceURL) > 0
}

// GetSSHEnabled gets it ssh should be enabled for Windows nodes
func (w *WindowsProfile) GetSSHEnabled() bool {
	if w.SSHEnabled != nil {
		return *w.SSHEnabled
	}
	return DefaultWindowsSSHEnabled
}

// HasImageRef returns true if the customer brought os image
func (w *WindowsProfile) HasImageRef() bool {
	return w.ImageRef != nil && w.ImageRef.IsValid()
}

// GetWindowsSku gets the marketplace sku specified (such as Datacenter-Core-1809-with-Containers-smalldisk) or returns default value
func (w *WindowsProfile) GetWindowsSku() string {
	if w.WindowsSku != "" {
		return w.WindowsSku
	}
	return KubernetesDefaultWindowsSku
}

// GetWindowsDockerVersion gets the docker version specified or returns default value
func (w *WindowsProfile) GetWindowsDockerVersion() string {
	if w.WindowsDockerVersion != "" {
		return w.WindowsDockerVersion
	}
	return KubernetesWindowsDockerVersion
}

// GetDefaultContainerdWindowsSandboxIsolation gets the default containerd runtime handler or return default value
func (w *WindowsProfile) GetDefaultContainerdWindowsSandboxIsolation() string {
	if w.ContainerdWindowsRuntimes != nil && w.ContainerdWindowsRuntimes.DefaultSandboxIsolation != "" {
		return w.ContainerdWindowsRuntimes.DefaultSandboxIsolation
	}

	return KubernetesDefaultContainerdWindowsSandboxIsolation
}

// GetContainerdWindowsRuntimeHandlers gets comma separated list of runtimehandler names
func (w *WindowsProfile) GetContainerdWindowsRuntimeHandlers() string {
	if w.ContainerdWindowsRuntimes != nil && len(w.ContainerdWindowsRuntimes.RuntimeHandlers) > 0 {
		handlernames := []string{}
		for _, h := range w.ContainerdWindowsRuntimes.RuntimeHandlers {
			handlernames = append(handlernames, h.BuildNumber)
		}
		return strings.Join(handlernames, ",")
	}

	return ""
}

// IsAlwaysPullWindowsPauseImage returns true if the windows pause image always needs a force pull
func (w *WindowsProfile) IsAlwaysPullWindowsPauseImage() bool {
	return w.AlwaysPullWindowsPauseImage != nil && *w.AlwaysPullWindowsPauseImage
}

// IsWindowsSecureTlsEnabled returns true if secure TLS should be enabled for Windows nodes
func (w *WindowsProfile) IsWindowsSecureTlsEnabled() bool {
	if w.WindowsSecureTlsEnabled != nil {
		return *w.WindowsSecureTlsEnabled
	}
	return DefaultWindowsSecureTlsEnabled
}

// IsKubernetes returns true if this template is for Kubernetes orchestrator
func (o *OrchestratorProfile) IsKubernetes() bool {
	return strings.EqualFold(o.OrchestratorType, Kubernetes)
}

// IsFeatureEnabled returns true if a feature flag is on for the provided feature
func (f *FeatureFlags) IsFeatureEnabled(feature string) bool {
	if f != nil {
		switch feature {
		case "CSERunInBackground":
			return f.EnableCSERunInBackground
		case "BlockOutboundInternet":
			return f.BlockOutboundInternet
		case "EnableIPv6DualStack":
			return f.EnableIPv6DualStack
		case "EnableTelemetry":
			return f.EnableTelemetry
		case "EnableIPv6Only":
			return f.EnableIPv6Only
		case "EnableWinDSR":
			return f.EnableWinDSR
		default:
			return false
		}
	}
	return false
}

// IsValid returns true if ImageRefernce contains at least Name and ResourceGroup
func (i *ImageReference) IsValid() bool {
	return len(i.Name) > 0 && len(i.ResourceGroup) > 0
}

// IsAddonEnabled checks whether a k8s addon with name "addonName" is enabled or not based on the Enabled field of KubernetesAddon.
// If the value of Enabled is nil, the "defaultValue" is returned.
func (k *KubernetesConfig) IsAddonEnabled(addonName string) bool {
	kubeAddon := k.GetAddonByName(addonName)
	return kubeAddon.IsEnabled()
}

// PrivateJumpboxProvision checks if a private cluster has jumpbox auto-provisioning
func (k *KubernetesConfig) PrivateJumpboxProvision() bool {
	if k != nil && k.PrivateCluster != nil && *k.PrivateCluster.Enabled && k.PrivateCluster.JumpboxProfile != nil {
		return true
	}
	return false
}

// IsRBACEnabled checks if RBAC is enabled
func (k *KubernetesConfig) IsRBACEnabled() bool {
	if k.EnableRbac != nil {
		return to.Bool(k.EnableRbac)
	}
	return false
}

// UserAssignedIDEnabled checks if the user assigned ID is enabled or not.
func (k *KubernetesConfig) UserAssignedIDEnabled() bool {
	return k.UseManagedIdentity && k.UserAssignedID != ""
}

// IsIPMasqAgentDisabled checks if the ip-masq-agent addon is disabled
func (k *KubernetesConfig) IsIPMasqAgentDisabled() bool {
	return k.IsAddonDisabled(IPMASQAgentAddonName)
}

// IsIPMasqAgentEnabled checks if the ip-masq-agent addon is enabled
func (k *KubernetesConfig) IsIPMasqAgentEnabled() bool {
	return k.IsAddonEnabled(IPMASQAgentAddonName)
}

// GetAddonByName returns the KubernetesAddon instance with name `addonName`
func (k *KubernetesConfig) GetAddonByName(addonName string) KubernetesAddon {
	var kubeAddon KubernetesAddon
	for _, addon := range k.Addons {
		if strings.EqualFold(addon.Name, addonName) {
			kubeAddon = addon
			break
		}
	}
	return kubeAddon
}

// IsAddonDisabled checks whether a k8s addon with name "addonName" is explicitly disabled based on the Enabled field of KubernetesAddon.
// If the value of Enabled is nil, we return false (not explicitly disabled)
func (k *KubernetesConfig) IsAddonDisabled(addonName string) bool {
	kubeAddon := k.GetAddonByName(addonName)
	return kubeAddon.IsDisabled()
}

// NeedsContainerd returns whether or not we need the containerd runtime configuration
// E.g., kata configuration requires containerd config
func (k *KubernetesConfig) NeedsContainerd() bool {
	return strings.EqualFold(k.ContainerRuntime, KataContainers) || strings.EqualFold(k.ContainerRuntime, Containerd)
}

// RequiresDocker returns if the kubernetes settings require docker binary to be installed.
func (k *KubernetesConfig) RequiresDocker() bool {
	if k == nil {
		return false
	}

	return strings.EqualFold(k.ContainerRuntime, Docker) || k.ContainerRuntime == ""
}

// IsAADPodIdentityEnabled checks if the AAD pod identity addon is enabled
func (k *KubernetesConfig) IsAADPodIdentityEnabled() bool {
	return k.IsAddonEnabled(AADPodIdentityAddonName)
}

// GetAzureCNIURLLinux returns the full URL to source Azure CNI binaries from
func (k *KubernetesConfig) GetAzureCNIURLLinux(cloudSpecConfig *AzureEnvironmentSpecConfig) string {
	if k.AzureCNIURLLinux != "" {
		return k.AzureCNIURLLinux
	}
	return cloudSpecConfig.KubernetesSpecConfig.VnetCNILinuxPluginsDownloadURL
}

// GetAzureCNIURLARM64Linux returns the full URL to source Azure CNI binaries for ARM64 Linux from
func (k *KubernetesConfig) GetAzureCNIURLARM64Linux(cloudSpecConfig *AzureEnvironmentSpecConfig) string {
	if k.AzureCNIURLARM64Linux != "" {
		return k.AzureCNIURLARM64Linux
	}
	return cloudSpecConfig.KubernetesSpecConfig.VnetCNIARM64LinuxPluginsDownloadURL
}

// GetAzureCNIURLWindows returns the full URL to source Azure CNI binaries from
func (k *KubernetesConfig) GetAzureCNIURLWindows(cloudSpecConfig *AzureEnvironmentSpecConfig) string {
	if k.AzureCNIURLWindows != "" {
		return k.AzureCNIURLWindows
	}
	return cloudSpecConfig.KubernetesSpecConfig.VnetCNIWindowsPluginsDownloadURL
}

// GetOrderedKubeletConfigStringForPowershell returns an ordered string of key/val pairs for Powershell script consumption
func (config *NodeBootstrappingConfiguration) GetOrderedKubeletConfigStringForPowershell() string {
	kubeletConfig := config.KubeletConfig
	if kubeletConfig == nil {
		kubeletConfig = map[string]string{}
	}

	// override default kubelet configuration with customzied ones
	if config.ContainerService != nil && config.ContainerService.Properties != nil {
		kubeletCustomConfiguration := config.ContainerService.Properties.GetComponentWindowsKubernetesConfiguration(Componentkubelet)
		if kubeletCustomConfiguration != nil {
			config := kubeletCustomConfiguration.Config
			for k, v := range config {
				kubeletConfig[k] = v
			}
		}
	}

	if len(kubeletConfig) == 0 {
		return ""
	}

	keys := []string{}
	for key := range kubeletConfig {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("\"%s=%s\", ", key, kubeletConfig[key]))
	}
	return strings.TrimSuffix(buf.String(), ", ")
}

// GetOrderedKubeproxyConfigStringForPowershell returns an ordered string of key/val pairs for Powershell script consumption
func (config *NodeBootstrappingConfiguration) GetOrderedKubeproxyConfigStringForPowershell() string {
	kubeproxyConfig := config.KubeproxyConfig
	if kubeproxyConfig == nil {
		// https://kubernetes.io/docs/reference/command-line-tools-reference/kube-proxy/
		// --metrics-bind-address ipport     Default: 127.0.0.1:10249
		// The IP address with port for the metrics server to serve on (set to '0.0.0.0:10249' for all IPv4 interfaces and '[::]:10249' for all IPv6 interfaces). Set empty to disable.
		// This only works with Windows provisioning package v0.0.15+.
		// https://github.com/Azure/aks-engine/blob/master/docs/topics/windows-provisioning-scripts-release-notes.md#v0015
		kubeproxyConfig = map[string]string{"--metrics-bind-address": "0.0.0.0:10249"}
	}

	if _, ok := kubeproxyConfig["--metrics-bind-address"]; !ok {
		kubeproxyConfig["--metrics-bind-address"] = "0.0.0.0:10249"
	}

	// override kube proxy configuration with the customzied ones.
	kubeProxyCustomConfiguration := config.ContainerService.Properties.GetComponentWindowsKubernetesConfiguration(ComponentkubeProxy)
	if kubeProxyCustomConfiguration != nil {
		customConfig := kubeProxyCustomConfiguration.Config
		for k, v := range customConfig {
			kubeproxyConfig[k] = v
		}
	}
	keys := []string{}
	for key := range kubeproxyConfig {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("\"%s=%s\", ", key, kubeproxyConfig[key]))
	}
	return strings.TrimSuffix(buf.String(), ", ")
}

// IsEnabled returns true if the addon is enabled
func (a *KubernetesAddon) IsEnabled() bool {
	if a.Enabled == nil {
		return false
	}
	return *a.Enabled
}

// IsDisabled returns true if the addon is explicitly disabled
func (a *KubernetesAddon) IsDisabled() bool {
	if a.Enabled == nil {
		return false
	}
	return !*a.Enabled
}

// GetAddonContainersIndexByName returns the KubernetesAddon containers index with the name `containerName`
func (a KubernetesAddon) GetAddonContainersIndexByName(containerName string) int {
	for i := range a.Containers {
		if strings.EqualFold(a.Containers[i].Name, containerName) {
			return i
		}
	}
	return -1
}

// FormatProdFQDNByLocation constructs an Azure prod fqdn with custom cloud profile
// CustomCloudName is name of environment if customCloudProfile is provided, it will be empty string if customCloudProfile is empty.
// Because customCloudProfile is empty for deployment for AzurePublicCloud, AzureChinaCloud,AzureGermanCloud,AzureUSGovernmentCloud,
// The customCloudName value will be empty string for those clouds
func FormatProdFQDNByLocation(fqdnPrefix string, location string, cloudSpecConfig *AzureEnvironmentSpecConfig) string {
	FQDNFormat := cloudSpecConfig.EndpointConfig.ResourceManagerVMDNSSuffix
	return fmt.Sprintf("%s.%s."+FQDNFormat, fqdnPrefix, location)
}

type K8sComponents struct {
	// Full path to the "pause" image. Used for --pod-infra-container-image
	// For example: "mcr.microsoft.com/oss/kubernetes/pause:1.3.1"
	PodInfraContainerImageURL string

	// Full path to the hyperkube image.
	// For example: "mcr.microsoft.com/hyperkube-amd64:v1.16.13"
	HyperkubeImageURL string

	// Full path to the Windows package (windowszip) to use.
	// For example: https://acs-mirror.azureedge.net/kubernetes/v1.17.8/windowszip/v1.17.8-1int.zip
	WindowsPackageURL string
}

// NodeBootstrappingConfiguration represents configurations for node bootstrapping
type NodeBootstrappingConfiguration struct {
	ContainerService              *ContainerService
	CloudSpecConfig               *AzureEnvironmentSpecConfig
	K8sComponents                 *K8sComponents
	AgentPoolProfile              *AgentPoolProfile
	TenantID                      string
	SubscriptionID                string
	ResourceGroupName             string
	UserAssignedIdentityClientID  string
	OSSKU                         string
	ConfigGPUDriverIfNeeded       bool
	Disable1804SystemdResolved    bool
	EnableGPUDevicePluginIfNeeded bool
	EnableKubeletConfigFile       bool
	EnableNvidia                  bool
	EnableACRTeleportPlugin       bool
	TeleportdPluginURL            string
	ContainerdVersion             string
	RuncVersion                   string
	// ContainerdPackageURL and RuncPackageURL are beneficial for testing non-official
	// containerd and runc, like the pre-released ones.
	// Currently both configurations are for test purpose, and only deb package is supported
	ContainerdPackageURL string
	RuncPackageURL       string
	// KubeletClientTLSBootstrapToken - kubelet client TLS bootstrap token to use.
	// When this feature is enabled, we skip kubelet kubeconfig generation and replace it with bootstrap kubeconfig.
	// ref: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping
	KubeletClientTLSBootstrapToken *string
	FIPSEnabled                    bool
	HTTPProxyConfig                *HTTPProxyConfig
	KubeletConfig                  map[string]string
	KubeproxyConfig                map[string]string
	EnableRuncShimV2               bool
	GPUInstanceProfile             string
	PrimaryScaleSetName            string
	SIGConfig                      SIGConfig
	IsARM64                        bool
}

// NodeBootstrapping represents the custom data, CSE, and OS image info needed for node bootstrapping.
type NodeBootstrapping struct {
	CustomData     string
	CSE            string
	OSImageConfig  *AzureOSImageConfig
	SigImageConfig *SigImageConfig
}

// HTTPProxyConfig represents configurations of http proxy
type HTTPProxyConfig struct {
	HTTPProxy  *string   `json:"httpProxy,omitempty"`
	HTTPSProxy *string   `json:"httpsProxy,omitempty"`
	NoProxy    *[]string `json:"noProxy,omitempty"`
	TrustedCA  *string   `json:"trustedCa,omitempty"`
}

// below are copied from Kubernetes
// KubeletConfiguration contains the configuration for the Kubelet
type AKSKubeletConfiguration struct {
	// Kind is a string value representing the REST resource this object represents.
	// Servers may infer this from the endpoint the client submits requests to.
	// Cannot be updated.
	// In CamelCase.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// APIVersion defines the versioned schema of this representation of an object.
	// Servers should convert recognized schemas to the latest internal value, and
	// may reject unrecognized values.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
	// +optional
	APIVersion string `json:"apiVersion" protobuf:"bytes,2,opt,name=apiVersion"`
	// enableServer enables Kubelet's secured server.
	// Note: Kubelet's insecure port is controlled by the readOnlyPort option.
	// Default: true
	EnableServer *bool `json:"enableServer"`
	// staticPodPath is the path to the directory containing local (static) pods to
	// run, or the path to a single static pod file.
	// Default: ""
	// +optional
	StaticPodPath string `json:"staticPodPath"`
	// syncFrequency is the max period between synchronizing running
	// containers and config.
	// Default: "1m"
	// +optional
	SyncFrequency metav1.Duration `json:"syncFrequency"`
	// fileCheckFrequency is the duration between checking config files for
	// new data.
	// Default: "20s"
	// +optional
	FileCheckFrequency metav1.Duration `json:"fileCheckFrequency"`
	// httpCheckFrequency is the duration between checking http for new data.
	// Default: "20s"
	// +optional
	HTTPCheckFrequency metav1.Duration `json:"httpCheckFrequency"`
	// staticPodURL is the URL for accessing static pods to run.
	// Default: ""
	// +optional
	StaticPodURL string `json:"staticPodURL"`
	// staticPodURLHeader is a map of slices with HTTP headers to use when accessing the podURL.
	// Default: nil
	// +optional
	StaticPodURLHeader map[string][]string `json:"staticPodURLHeader"`
	// address is the IP address for the Kubelet to serve on (set to 0.0.0.0
	// for all interfaces).
	// Default: "0.0.0.0"
	// +optional
	Address string `json:"address"`
	// port is the port for the Kubelet to serve on.
	// The port number must be between 1 and 65535, inclusive.
	// Default: 10250
	// +optional
	Port int32 `json:"port"`
	// readOnlyPort is the read-only port for the Kubelet to serve on with
	// no authentication/authorization.
	// The port number must be between 1 and 65535, inclusive.
	// Setting this field to 0 disables the read-only service.
	// Default: 0 (disabled)
	// +optional
	ReadOnlyPort int32 `json:"readOnlyPort"`
	// tlsCertFile is the file containing x509 Certificate for HTTPS. (CA cert,
	// if any, concatenated after server cert). If tlsCertFile and
	// tlsPrivateKeyFile are not provided, a self-signed certificate
	// and key are generated for the public address and saved to the directory
	// passed to the Kubelet's --cert-dir flag.
	// Default: ""
	// +optional
	TLSCertFile string `json:"tlsCertFile"`
	// tlsPrivateKeyFile is the file containing x509 private key matching tlsCertFile.
	// Default: ""
	// +optional
	TLSPrivateKeyFile string `json:"tlsPrivateKeyFile"`
	// tlsCipherSuites is the list of allowed cipher suites for the server.
	// Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	// Default: nil
	// +optional
	TLSCipherSuites []string `json:"tlsCipherSuites"`
	// tlsMinVersion is the minimum TLS version supported.
	// Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	// Default: ""
	// +optional
	TLSMinVersion string `json:"tlsMinVersion"`
	// rotateCertificates enables client certificate rotation. The Kubelet will request a
	// new certificate from the certificates.k8s.io API. This requires an approver to approve the
	// certificate signing requests.
	// Default: false
	// +optional
	RotateCertificates bool `json:"rotateCertificates"`
	// serverTLSBootstrap enables server certificate bootstrap. Instead of self
	// signing a serving certificate, the Kubelet will request a certificate from
	// the 'certificates.k8s.io' API. This requires an approver to approve the
	// certificate signing requests (CSR). The RotateKubeletServerCertificate feature
	// must be enabled when setting this field.
	// Default: false
	// +optional
	ServerTLSBootstrap bool `json:"serverTLSBootstrap"`
	// authentication specifies how requests to the Kubelet's server are authenticated.
	// Defaults:
	//   anonymous:
	//     enabled: false
	//   webhook:
	//     enabled: true
	//     cacheTTL: "2m"
	// +optional
	Authentication kubeletconfigv1beta1.KubeletAuthentication `json:"authentication"`
	// authorization specifies how requests to the Kubelet's server are authorized.
	// Defaults:
	//   mode: Webhook
	//   webhook:
	//     cacheAuthorizedTTL: "5m"
	//     cacheUnauthorizedTTL: "30s"
	// +optional
	Authorization kubeletconfigv1beta1.KubeletAuthorization `json:"authorization"`
	// registryPullQPS is the limit of registry pulls per second.
	// The value must not be a negative number.
	// Setting it to 0 means no limit.
	// Default: 5
	// +optional
	RegistryPullQPS *int32 `json:"registryPullQPS"`
	// registryBurst is the maximum size of bursty pulls, temporarily allows
	// pulls to burst to this number, while still not exceeding registryPullQPS.
	// The value must not be a negative number.
	// Only used if registryPullQPS is greater than 0.
	// Default: 10
	// +optional
	RegistryBurst int32 `json:"registryBurst"`
	// eventRecordQPS is the maximum event creations per second. If 0, there
	// is no limit enforced. The value cannot be a negative number.
	// Default: 5
	// +optional
	EventRecordQPS *int32 `json:"eventRecordQPS"`
	// eventBurst is the maximum size of a burst of event creations, temporarily
	// allows event creations to burst to this number, while still not exceeding
	// eventRecordQPS. This field canot be a negative number and it is only used
	// when eventRecordQPS > 0.
	// Default: 10
	// +optional
	EventBurst int32 `json:"eventBurst"`
	// enableDebuggingHandlers enables server endpoints for log access
	// and local running of containers and commands, including the exec,
	// attach, logs, and portforward features.
	// Default: true
	// +optional
	EnableDebuggingHandlers *bool `json:"enableDebuggingHandlers"`
	// enableContentionProfiling enables lock contention profiling, if enableDebuggingHandlers is true.
	// Default: false
	// +optional
	EnableContentionProfiling bool `json:"enableContentionProfiling"`
	// healthzPort is the port of the localhost healthz endpoint (set to 0 to disable).
	// A valid number is between 1 and 65535.
	// Default: 10248
	// +optional
	HealthzPort *int32 `json:"healthzPort"`
	// healthzBindAddress is the IP address for the healthz server to serve on.
	// Default: "127.0.0.1"
	// +optional
	HealthzBindAddress string `json:"healthzBindAddress"`
	// oomScoreAdj is The oom-score-adj value for kubelet process. Values
	// must be within the range [-1000, 1000].
	// Default: -999
	// +optional
	OOMScoreAdj *int32 `json:"oomScoreAdj"`
	// clusterDomain is the DNS domain for this cluster. If set, kubelet will
	// configure all containers to search this domain in addition to the
	// host's search domains.
	// Default: ""
	// +optional
	ClusterDomain string `json:"clusterDomain"`
	// clusterDNS is a list of IP addresses for the cluster DNS server. If set,
	// kubelet will configure all containers to use this for DNS resolution
	// instead of the host's DNS servers.
	// Default: nil
	// +optional
	ClusterDNS []string `json:"clusterDNS"`
	// streamingConnectionIdleTimeout is the maximum time a streaming connection
	// can be idle before the connection is automatically closed.
	// Default: "4h"
	// +optional
	StreamingConnectionIdleTimeout metav1.Duration `json:"streamingConnectionIdleTimeout"`
	// nodeStatusUpdateFrequency is the frequency that kubelet computes node
	// status. If node lease feature is not enabled, it is also the frequency that
	// kubelet posts node status to master.
	// Note: When node lease feature is not enabled, be cautious when changing the
	// constant, it must work with nodeMonitorGracePeriod in nodecontroller.
	// Default: "10s"
	// +optional
	NodeStatusUpdateFrequency metav1.Duration `json:"nodeStatusUpdateFrequency"`
	// nodeStatusReportFrequency is the frequency that kubelet posts node
	// status to master if node status does not change. Kubelet will ignore this
	// frequency and post node status immediately if any change is detected. It is
	// only used when node lease feature is enabled. nodeStatusReportFrequency's
	// default value is 5m. But if nodeStatusUpdateFrequency is set explicitly,
	// nodeStatusReportFrequency's default value will be set to
	// nodeStatusUpdateFrequency for backward compatibility.
	// Default: "5m"
	// +optional
	NodeStatusReportFrequency metav1.Duration `json:"nodeStatusReportFrequency"`
	// nodeLeaseDurationSeconds is the duration the Kubelet will set on its corresponding Lease.
	// NodeLease provides an indicator of node health by having the Kubelet create and
	// periodically renew a lease, named after the node, in the kube-node-lease namespace.
	// If the lease expires, the node can be considered unhealthy.
	// The lease is currently renewed every 10s, per KEP-0009. In the future, the lease renewal
	// interval may be set based on the lease duration.
	// The field value must be greater than 0.
	// Default: 40
	// +optional
	NodeLeaseDurationSeconds int32 `json:"nodeLeaseDurationSeconds"`
	// imageMinimumGCAge is the minimum age for an unused image before it is
	// garbage collected.
	// Default: "2m"
	// +optional
	ImageMinimumGCAge metav1.Duration `json:"imageMinimumGCAge"`
	// imageGCHighThresholdPercent is the percent of disk usage after which
	// image garbage collection is always run. The percent is calculated by
	// dividing this field value by 100, so this field must be between 0 and
	// 100, inclusive. When specified, the value must be greater than
	// imageGCLowThresholdPercent.
	// Default: 85
	// +optional
	ImageGCHighThresholdPercent *int32 `json:"imageGCHighThresholdPercent"`
	// imageGCLowThresholdPercent is the percent of disk usage before which
	// image garbage collection is never run. Lowest disk usage to garbage
	// collect to. The percent is calculated by dividing this field value by 100,
	// so the field value must be between 0 and 100, inclusive. When specified, the
	// value must be less than imageGCHighThresholdPercent.
	// Default: 80
	// +optional
	ImageGCLowThresholdPercent *int32 `json:"imageGCLowThresholdPercent"`
	// volumeStatsAggPeriod is the frequency for calculating and caching volume
	// disk usage for all pods.
	// Default: "1m"
	// +optional
	VolumeStatsAggPeriod metav1.Duration `json:"volumeStatsAggPeriod"`
	// kubeletCgroups is the absolute name of cgroups to isolate the kubelet in
	// Default: ""
	// +optional
	KubeletCgroups string `json:"kubeletCgroups"`
	// systemCgroups is absolute name of cgroups in which to place
	// all non-kernel processes that are not already in a container. Empty
	// for no container. Rolling back the flag requires a reboot.
	// The cgroupRoot must be specified if this field is not empty.
	// Default: ""
	// +optional
	SystemCgroups string `json:"systemCgroups"`
	// cgroupRoot is the root cgroup to use for pods. This is handled by the
	// container runtime on a best effort basis.
	// +optional
	CgroupRoot string `json:"cgroupRoot"`
	// cgroupsPerQOS enable QoS based CGroup hierarchy: top level CGroups for QoS classes
	// and all Burstable and BestEffort Pods are brought up under their specific top level
	// QoS CGroup.
	// Default: true
	// +optional
	CgroupsPerQOS *bool `json:"cgroupsPerQOS"`
	// cgroupDriver is the driver kubelet uses to manipulate CGroups on the host (cgroupfs
	// or systemd).
	// Default: "cgroupfs"
	// +optional
	CgroupDriver string `json:"cgroupDriver"`
	// cpuManagerPolicy is the name of the policy to use.
	// Requires the CPUManager feature gate to be enabled.
	// Default: "None"
	// +optional
	CPUManagerPolicy string `json:"cpuManagerPolicy"`
	// cpuManagerPolicyOptions is a set of key=value which 	allows to set extra options
	// to fine tune the behaviour of the cpu manager policies.
	// Requires  both the "CPUManager" and "CPUManagerPolicyOptions" feature gates to be enabled.
	// Default: nil
	// +optional
	CPUManagerPolicyOptions map[string]string `json:"cpuManagerPolicyOptions"`
	// cpuManagerReconcilePeriod is the reconciliation period for the CPU Manager.
	// Requires the CPUManager feature gate to be enabled.
	// Default: "10s"
	// +optional
	CPUManagerReconcilePeriod metav1.Duration `json:"cpuManagerReconcilePeriod"`
	// memoryManagerPolicy is the name of the policy to use by memory manager.
	// Requires the MemoryManager feature gate to be enabled.
	// Default: "none"
	// +optional
	MemoryManagerPolicy string `json:"memoryManagerPolicy"`
	// topologyManagerPolicy is the name of the topology manager policy to use.
	// Valid values include:
	//
	// - `restricted`: kubelet only allows pods with optimal NUMA node alignment for
	//   requested resources;
	// - `best-effort`: kubelet will favor pods with NUMA alignment of CPU and device
	//   resources;
	// - `none`: kubelet has no knowledge of NUMA alignment of a pod's CPU and device resources.
	// - `single-numa-node`: kubelet only allows pods with a single NUMA alignment
	//   of CPU and device resources.
	//
	// Policies other than "none" require the TopologyManager feature gate to be enabled.
	// Default: "none"
	// +optional
	TopologyManagerPolicy string `json:"topologyManagerPolicy"`
	// topologyManagerScope represents the scope of topology hint generation
	// that topology manager requests and hint providers generate. Valid values include:
	//
	// - `container`: topology policy is applied on a per-container basis.
	// - `pod`: topology policy is applied on a per-pod basis.
	//
	// "pod" scope requires the TopologyManager feature gate to be enabled.
	// Default: "container"
	// +optional
	TopologyManagerScope string `json:"topologyManagerScope"`
	// qosReserved is a set of resource name to percentage pairs that specify
	// the minimum percentage of a resource reserved for exclusive use by the
	// guaranteed QoS tier.
	// Currently supported resources: "memory"
	// Requires the QOSReserved feature gate to be enabled.
	// Default: nil
	// +optional
	QOSReserved map[string]string `json:"qosReserved"`
	// runtimeRequestTimeout is the timeout for all runtime requests except long running
	// requests - pull, logs, exec and attach.
	// Default: "2m"
	// +optional
	RuntimeRequestTimeout metav1.Duration `json:"runtimeRequestTimeout"`
	// hairpinMode specifies how the Kubelet should configure the container
	// bridge for hairpin packets.
	// Setting this flag allows endpoints in a Service to loadbalance back to
	// themselves if they should try to access their own Service. Values:
	//
	// - "promiscuous-bridge": make the container bridge promiscuous.
	// - "hairpin-veth":       set the hairpin flag on container veth interfaces.
	// - "none":               do nothing.
	//
	// Generally, one must set `--hairpin-mode=hairpin-veth to` achieve hairpin NAT,
	// because promiscuous-bridge assumes the existence of a container bridge named cbr0.
	// Default: "promiscuous-bridge"
	// +optional
	HairpinMode string `json:"hairpinMode"`
	// maxPods is the maximum number of Pods that can run on this Kubelet.
	// The value must be a non-negative integer.
	// Default: 110
	// +optional
	MaxPods int32 `json:"maxPods"`
	// podCIDR is the CIDR to use for pod IP addresses, only used in standalone mode.
	// In cluster mode, this is obtained from the control plane.
	// Default: ""
	// +optional
	PodCIDR string `json:"podCIDR"`
	// podPidsLimit is the maximum number of PIDs in any pod.
	// Default: -1
	// +optional
	PodPidsLimit *int64 `json:"podPidsLimit"`
	// resolvConf is the resolver configuration file used as the basis
	// for the container DNS resolution configuration.
	// If set to the empty string, will override the default and effectively disable DNS lookups.
	// Default: "/etc/resolv.conf"
	// +optional
	ResolverConfig *string `json:"resolvConf"`
	// runOnce causes the Kubelet to check the API server once for pods,
	// run those in addition to the pods specified by static pod files, and exit.
	// Default: false
	// +optional
	RunOnce bool `json:"runOnce"`
	// cpuCFSQuota enables CPU CFS quota enforcement for containers that
	// specify CPU limits.
	// Default: true
	// +optional
	CPUCFSQuota *bool `json:"cpuCFSQuota"`
	// cpuCFSQuotaPeriod is the CPU CFS quota period value, `cpu.cfs_period_us`.
	// The value must be between 1 us and 1 second, inclusive.
	// Requires the CustomCPUCFSQuotaPeriod feature gate to be enabled.
	// Default: "100ms"
	// +optional
	CPUCFSQuotaPeriod *metav1.Duration `json:"cpuCFSQuotaPeriod"`
	// nodeStatusMaxImages caps the number of images reported in Node.status.images.
	// The value must be greater than -2.
	// Note: If -1 is specified, no cap will be applied. If 0 is specified, no image is returned.
	// Default: 50
	// +optional
	NodeStatusMaxImages *int32 `json:"nodeStatusMaxImages"`
	// maxOpenFiles is Number of files that can be opened by Kubelet process.
	// The value must be a non-negative number.
	// Default: 1000000
	// +optional
	MaxOpenFiles int64 `json:"maxOpenFiles"`
	// contentType is contentType of requests sent to apiserver.
	// Default: "application/vnd.kubernetes.protobuf"
	// +optional
	ContentType string `json:"contentType"`
	// kubeAPIQPS is the QPS to use while talking with kubernetes apiserver.
	// Default: 5
	// +optional
	KubeAPIQPS *int32 `json:"kubeAPIQPS"`
	// kubeAPIBurst is the burst to allow while talking with kubernetes API server.
	// This field cannot be a negative number.
	// Default: 10
	// +optional
	KubeAPIBurst int32 `json:"kubeAPIBurst"`
	// serializeImagePulls when enabled, tells the Kubelet to pull images one
	// at a time. We recommend *not* changing the default value on nodes that
	// run docker daemon with version  < 1.9 or an Aufs storage backend.
	// Issue #10959 has more details.
	// Default: true
	// +optional
	SerializeImagePulls *bool `json:"serializeImagePulls"`
	// evictionHard is a map of signal names to quantities that defines hard eviction
	// thresholds. For example: `{"memory.available": "300Mi"}`.
	// To explicitly disable, pass a 0% or 100% threshold on an arbitrary resource.
	// Default:
	//   memory.available:  "100Mi"
	//   nodefs.available:  "10%"
	//   nodefs.inodesFree: "5%"
	//   imagefs.available: "15%"
	// +optional
	EvictionHard map[string]string `json:"evictionHard"`
	// evictionSoft is a map of signal names to quantities that defines soft eviction thresholds.
	// For example: `{"memory.available": "300Mi"}`.
	// Default: nil
	// +optional
	EvictionSoft map[string]string `json:"evictionSoft"`
	// evictionSoftGracePeriod is a map of signal names to quantities that defines grace
	// periods for each soft eviction signal. For example: `{"memory.available": "30s"}`.
	// Default: nil
	// +optional
	EvictionSoftGracePeriod map[string]string `json:"evictionSoftGracePeriod"`
	// evictionPressureTransitionPeriod is the duration for which the kubelet has to wait
	// before transitioning out of an eviction pressure condition.
	// Default: "5m"
	// +optional
	EvictionPressureTransitionPeriod metav1.Duration `json:"evictionPressureTransitionPeriod"`
	// evictionMaxPodGracePeriod is the maximum allowed grace period (in seconds) to use
	// when terminating pods in response to a soft eviction threshold being met. This value
	// effectively caps the Pod's terminationGracePeriodSeconds value during soft evictions.
	// Note: Due to issue #64530, the behavior has a bug where this value currently just
	// overrides the grace period during soft eviction, which can increase the grace
	// period from what is set on the Pod. This bug will be fixed in a future release.
	// Default: 0
	// +optional
	EvictionMaxPodGracePeriod int32 `json:"evictionMaxPodGracePeriod"`
	// evictionMinimumReclaim is a map of signal names to quantities that defines minimum reclaims,
	// which describe the minimum amount of a given resource the kubelet will reclaim when
	// performing a pod eviction while that resource is under pressure.
	// For example: `{"imagefs.available": "2Gi"}`.
	// Default: nil
	// +optional
	EvictionMinimumReclaim map[string]string `json:"evictionMinimumReclaim"`
	// podsPerCore is the maximum number of pods per core. Cannot exceed maxPods.
	// The value must be a non-negative integer.
	// If 0, there is no limit on the number of Pods.
	// Default: 0
	// +optional
	PodsPerCore int32 `json:"podsPerCore"`
	// enableControllerAttachDetach enables the Attach/Detach controller to
	// manage attachment/detachment of volumes scheduled to this node, and
	// disables kubelet from executing any attach/detach operations.
	// Note: attaching/detaching CSI volumes is not supported by the kubelet,
	// so this option needs to be true for that use case.
	// Default: true
	// +optional
	EnableControllerAttachDetach *bool `json:"enableControllerAttachDetach"`
	// protectKernelDefaults, if true, causes the Kubelet to error if kernel
	// flags are not as it expects. Otherwise the Kubelet will attempt to modify
	// kernel flags to match its expectation.
	// Default: false
	// +optional
	ProtectKernelDefaults bool `json:"protectKernelDefaults"`
	// makeIPTablesUtilChains, if true, causes the Kubelet ensures a set of iptables rules
	// are present on host.
	// These rules will serve as utility rules for various components, e.g. kube-proxy.
	// The rules will be created based on iptablesMasqueradeBit and iptablesDropBit.
	// Default: true
	// +optional
	MakeIPTablesUtilChains *bool `json:"makeIPTablesUtilChains"`
	// iptablesMasqueradeBit is the bit of the iptables fwmark space to mark for SNAT.
	// Values must be within the range [0, 31]. Must be different from other mark bits.
	// Warning: Please match the value of the corresponding parameter in kube-proxy.
	// TODO: clean up IPTablesMasqueradeBit in kube-proxy.
	// Default: 14
	// +optional
	IPTablesMasqueradeBit *int32 `json:"iptablesMasqueradeBit"`
	// iptablesDropBit is the bit of the iptables fwmark space to mark for dropping packets.
	// Values must be within the range [0, 31]. Must be different from other mark bits.
	// Default: 15
	// +optional
	IPTablesDropBit *int32 `json:"iptablesDropBit"`
	// featureGates is a map of feature names to bools that enable or disable experimental
	// features. This field modifies piecemeal the built-in default values from
	// "k8s.io/kubernetes/pkg/features/kube_features.go".
	// Default: nil
	// +optional
	FeatureGates map[string]bool `json:"featureGates"`
	// failSwapOn tells the Kubelet to fail to start if swap is enabled on the node.
	// Default: true
	// +optional
	FailSwapOn *bool `json:"failSwapOn"`
	// memorySwap configures swap memory available to container workloads.
	// +featureGate=NodeSwap
	// +optional
	MemorySwap kubeletconfigv1beta1.MemorySwapConfiguration `json:"memorySwap"`
	// containerLogMaxSize is a quantity defining the maximum size of the container log
	// file before it is rotated. For example: "5Mi" or "256Ki".
	// Default: "10Mi"
	// +optional
	ContainerLogMaxSize string `json:"containerLogMaxSize"`
	// containerLogMaxFiles specifies the maximum number of container log files that can
	// be present for a container.
	// Default: 5
	// +optional
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles"`
	// configMapAndSecretChangeDetectionStrategy is a mode in which ConfigMap and Secret
	// managers are running. Valid values include:
	//
	// - `Get`: kubelet fetches necessary objects directly from the API server;
	// - `Cache`: kubelet uses TTL cache for object fetched from the API server;
	// - `Watch`: kubelet uses watches to observe changes to objects that are in its interest.
	//
	// Default: "Watch"
	// +optional
	ConfigMapAndSecretChangeDetectionStrategy kubeletconfigv1beta1.ResourceChangeDetectionStrategy `json:"configMapAndSecretChangeDetectionStrategy"`

	/* the following fields are meant for Node Allocatable */

	// systemReserved is a set of ResourceName=ResourceQuantity (e.g. cpu=200m,memory=150G)
	// pairs that describe resources reserved for non-kubernetes components.
	// Currently only cpu and memory are supported.
	// See http://kubernetes.io/docs/user-guide/compute-resources for more detail.
	// Default: nil
	// +optional
	SystemReserved map[string]string `json:"systemReserved"`
	// kubeReserved is a set of ResourceName=ResourceQuantity (e.g. cpu=200m,memory=150G) pairs
	// that describe resources reserved for kubernetes system components.
	// Currently cpu, memory and local storage for root file system are supported.
	// See https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// for more details.
	// Default: nil
	// +optional
	KubeReserved map[string]string `json:"kubeReserved"`
	// The reservedSystemCPUs option specifies the CPU list reserved for the host
	// level system threads and kubernetes related threads. This provide a "static"
	// CPU list rather than the "dynamic" list by systemReserved and kubeReserved.
	// This option does not support systemReservedCgroup or kubeReservedCgroup.
	ReservedSystemCPUs string `json:"reservedSystemCPUs"`
	// showHiddenMetricsForVersion is the previous version for which you want to show
	// hidden metrics.
	// Only the previous minor version is meaningful, other values will not be allowed.
	// The format is `<major>.<minor>`, e.g.: `1.16`.
	// The purpose of this format is make sure you have the opportunity to notice
	// if the next release hides additional metrics, rather than being surprised
	// when they are permanently removed in the release after that.
	// Default: ""
	// +optional
	ShowHiddenMetricsForVersion string `json:"showHiddenMetricsForVersion"`
	// systemReservedCgroup helps the kubelet identify absolute name of top level CGroup used
	// to enforce `systemReserved` compute resource reservation for OS system daemons.
	// Refer to [Node Allocatable](https://git.k8s.io/community/contributors/design-proposals/node/node-allocatable.md)
	// doc for more information.
	// Default: ""
	// +optional
	SystemReservedCgroup string `json:"systemReservedCgroup"`
	// kubeReservedCgroup helps the kubelet identify absolute name of top level CGroup used
	// to enforce `KubeReserved` compute resource reservation for Kubernetes node system daemons.
	// Refer to [Node Allocatable](https://git.k8s.io/community/contributors/design-proposals/node/node-allocatable.md)
	// doc for more information.
	// Default: ""
	// +optional
	KubeReservedCgroup string `json:"kubeReservedCgroup"`
	// This flag specifies the various Node Allocatable enforcements that Kubelet needs to perform.
	// This flag accepts a list of options. Acceptable options are `none`, `pods`,
	// `system-reserved` and `kube-reserved`.
	// If `none` is specified, no other options may be specified.
	// When `system-reserved` is in the list, systemReservedCgroup must be specified.
	// When `kube-reserved` is in the list, kubeReservedCgroup must be specified.
	// This field is supported only when `cgroupsPerQOS` is set to true.
	// Refer to [Node Allocatable](https://git.k8s.io/community/contributors/design-proposals/node/node-allocatable.md)
	// for more information.
	// Default: ["pods"]
	// +optional
	EnforceNodeAllocatable []string `json:"enforceNodeAllocatable"`
	// A comma separated whitelist of unsafe sysctls or sysctl patterns (ending in `*`).
	// Unsafe sysctl groups are `kernel.shm*`, `kernel.msg*`, `kernel.sem`, `fs.mqueue.*`,
	// and `net.*`. For example: "`kernel.msg*,net.ipv4.route.min_pmtu`"
	// Default: []
	// +optional
	AllowedUnsafeSysctls []string `json:"allowedUnsafeSysctls"`
	// volumePluginDir is the full path of the directory in which to search
	// for additional third party volume plugins.
	// Default: "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/"
	// +optional
	VolumePluginDir string `json:"volumePluginDir"`
	// providerID, if set, sets the unique ID of the instance that an external
	// provider (i.e. cloudprovider) can use to identify a specific node.
	// Default: ""
	// +optional
	ProviderID string `json:"providerID"`
	// kernelMemcgNotification, if set, instructs the kubelet to integrate with the
	// kernel memcg notification for determining if memory eviction thresholds are
	// exceeded rather than polling.
	// Default: false
	// +optional
	KernelMemcgNotification bool `json:"kernelMemcgNotification"`
	// logging specifies the options of logging.
	// Refer to [Logs Options](https://github.com/kubernetes/component-base/blob/master/logs/options.go)
	// for more information.
	// Default:
	//   Format: text
	// + optional
	Logging logsapi.LoggingConfiguration `json:"logging"`
	// enableSystemLogHandler enables system logs via web interface host:port/logs/
	// Default: true
	// +optional
	EnableSystemLogHandler *bool `json:"enableSystemLogHandler"`
	// shutdownGracePeriod specifies the total duration that the node should delay the
	// shutdown and total grace period for pod termination during a node shutdown.
	// Default: "0s"
	// +featureGate=GracefulNodeShutdown
	// +optional
	ShutdownGracePeriod metav1.Duration `json:"shutdownGracePeriod"`
	// shutdownGracePeriodCriticalPods specifies the duration used to terminate critical
	// pods during a node shutdown. This should be less than shutdownGracePeriod.
	// For example, if shutdownGracePeriod=30s, and shutdownGracePeriodCriticalPods=10s,
	// during a node shutdown the first 20 seconds would be reserved for gracefully
	// terminating normal pods, and the last 10 seconds would be reserved for terminating
	// critical pods.
	// Default: "0s"
	// +featureGate=GracefulNodeShutdown
	// +optional
	ShutdownGracePeriodCriticalPods metav1.Duration `json:"shutdownGracePeriodCriticalPods"`
	// shutdownGracePeriodByPodPriority specifies the shutdown grace period for Pods based
	// on their associated priority class value.
	// When a shutdown request is received, the Kubelet will initiate shutdown on all pods
	// running on the node with a grace period that depends on the priority of the pod,
	// and then wait for all pods to exit.
	// Each entry in the array represents the graceful shutdown time a pod with a priority
	// class value that lies in the range of that value and the next higher entry in the
	// list when the node is shutting down.
	// For example, to allow critical pods 10s to shutdown, priority>=10000 pods 20s to
	// shutdown, and all remaining pods 30s to shutdown.
	//
	// shutdownGracePeriodByPodPriority:
	//   - priority: 2000000000
	//     shutdownGracePeriodSeconds: 10
	//   - priority: 10000
	//     shutdownGracePeriodSeconds: 20
	//   - priority: 0
	//     shutdownGracePeriodSeconds: 30
	//
	// The time the Kubelet will wait before exiting will at most be the maximum of all
	// shutdownGracePeriodSeconds for each priority class range represented on the node.
	// When all pods have exited or reached their grace periods, the Kubelet will release
	// the shutdown inhibit lock.
	// Requires the GracefulNodeShutdown feature gate to be enabled.
	// This configuration must be empty if either ShutdownGracePeriod or ShutdownGracePeriodCriticalPods is set.
	// Default: nil
	// +featureGate=GracefulNodeShutdownBasedOnPodPriority
	// +optional
	ShutdownGracePeriodByPodPriority []kubeletconfigv1beta1.ShutdownGracePeriodByPodPriority `json:"shutdownGracePeriodByPodPriority"`
	// reservedMemory specifies a comma-separated list of memory reservations for NUMA nodes.
	// The parameter makes sense only in the context of the memory manager feature.
	// The memory manager will not allocate reserved memory for container workloads.
	// For example, if you have a NUMA0 with 10Gi of memory and the reservedMemory was
	// specified to reserve 1Gi of memory at NUMA0, the memory manager will assume that
	// only 9Gi is available for allocation.
	// You can specify a different amount of NUMA node and memory types.
	// You can omit this parameter at all, but you should be aware that the amount of
	// reserved memory from all NUMA nodes should be equal to the amount of memory specified
	// by the [node allocatable](https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#node-allocatable).
	// If at least one node allocatable parameter has a non-zero value, you will need
	// to specify at least one NUMA node.
	// Also, avoid specifying:
	//
	// 1. Duplicates, the same NUMA node, and memory type, but with a different value.
	// 2. zero limits for any memory type.
	// 3. NUMAs nodes IDs that do not exist under the machine.
	// 4. memory types except for memory and hugepages-<size>
	//
	// Default: nil
	// +optional
	ReservedMemory []kubeletconfigv1beta1.MemoryReservation `json:"reservedMemory"`
	// enableProfilingHandler enables profiling via web interface host:port/debug/pprof/
	// Default: true
	// +optional
	EnableProfilingHandler *bool `json:"enableProfilingHandler"`
	// enableDebugFlagsHandler enables flags endpoint via web interface host:port/debug/flags/v
	// Default: true
	// +optional
	EnableDebugFlagsHandler *bool `json:"enableDebugFlagsHandler"`
	// SeccompDefault enables the use of `RuntimeDefault` as the default seccomp profile for all workloads.
	// This requires the corresponding SeccompDefault feature gate to be enabled as well.
	// Default: false
	// +optional
	SeccompDefault *bool `json:"seccompDefault"`
	// MemoryThrottlingFactor specifies the factor multiplied by the memory limit or node allocatable memory
	// when setting the cgroupv2 memory.high value to enforce MemoryQoS.
	// Decreasing this factor will set lower high limit for container cgroups and put heavier reclaim pressure
	// while increasing will put less reclaim pressure.
	// See http://kep.k8s.io/2570 for more details.
	// Default: 0.8
	// +featureGate=MemoryQoS
	// +optional
	MemoryThrottlingFactor *float64 `json:"memoryThrottlingFactor"`
	// registerWithTaints are an array of taints to add to a node object when
	// the kubelet registers itself. This only takes effect when registerNode
	// is true and upon the initial registration of the node.
	// Default: nil
	// +optional
	RegisterWithTaints []v1.Taint `json:"registerWithTaints"`
	// registerNode enables automatic registration with the apiserver.
	// Default: true
	// +optional
	RegisterNode *bool `json:"registerNode"`
}

type CSEStatus struct {
	// ExitCode stores the exitCode from CSE output.
	ExitCode string `json:"exitCode,omitempty"`
	// Output stores the output from CSE output.
	Output string `json:"output,omitempty"`
	// Error stores the error from CSE output.
	Error string `json:"error,omitempty"`
	// ExecDuration stores the execDuration in seconds from CSE output.
	ExecDuration string `json:"execDuration,omitempty"`
	// KernelStartTime of current boot, output from systemctl show -p KernelTimestamp
	KernelStartTime string `json:"kernelStartTime,omitempty"`
	// SystemdSummary of current boot, output from systemd-analyze
	SystemdSummary string `json:"systemdSummary,omitempty"`
	// CSEStartTime indicate starttime of CSE
	CSEStartTime string `json:"cseStartTime,omitempty"`
	// GuestAgentStartTime indicate starttime of GuestAgent, output from systemctl show walinuxagent.service -p ExecMainStartTimestamp
	GuestAgentStartTime string `json:"guestAgentStartTime,omitempty"`
	// BootDatapoints contains datapoints (key-value pair) from VM boot process.
	BootDatapoints map[string]string `json:"bootDatapoints,omitempty"`
}

type CSEStatusParsingErrorCode string

const (
	// CSEMessageUnmarshalError is the error code for unmarshal cse message
	CSEMessageUnmarshalError CSEStatusParsingErrorCode = "CSEMessageUnmarshalError"
	// CSEMessageExitCodeEmptyError is the error code for empty cse message exit code
	CSEMessageExitCodeEmptyError CSEStatusParsingErrorCode = "CSEMessageExitCodeEmptyError"
	// InvalidCSEMessage is the error code for cse invalid message
	InvalidCSEMessage CSEStatusParsingErrorCode = "InvalidCSEMessage"
)

type CSEStatusParsingError struct {
	Code    CSEStatusParsingErrorCode
	Message string
}

func NewError(code CSEStatusParsingErrorCode, message string) *CSEStatusParsingError {
	return &CSEStatusParsingError{Code: code, Message: message}
}

func (err *CSEStatusParsingError) Error() string {
	return fmt.Sprintf("CSE has invalid message=%q, InstanceErrorCode=%s", err.Message, err.Code)
}
