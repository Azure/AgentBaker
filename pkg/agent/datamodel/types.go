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
	"github.com/Masterminds/semver/v3"
)

// TypeMeta describes an individual API model object.
type TypeMeta struct {
	// APIVersion is on every object.
	APIVersion string `json:"apiVersion"`
}

/*
CustomSearchDomain represents the Search Domain when the custom vnet has a windows server DNS as a
nameserver.
*/
type CustomSearchDomain struct {
	Name          string `json:"name,omitempty"`
	RealmUser     string `json:"realmUser,omitempty"`
	RealmPassword string `json:"realmPassword,omitempty"`
}

// PublicKey represents an SSH key for LinuxProfile.
type PublicKey struct {
	KeyData string `json:"keyData"`
}

/*
KeyVaultCertificate specifies a certificate to install.
On Linux, the certificate file is placed under the /var/lib/waagent directory
with the file name <UppercaseThumbprint>.crt for the X509 certificate file
and <UppercaseThumbprint>.prv for the private key. Both of these files are .pem formatted.
On windows the certificate will be saved in the specified store.
*/
type KeyVaultCertificate struct {
	CertificateURL   string `json:"certificateUrl,omitempty"`
	CertificateStore string `json:"certificateStore,omitempty"`
}

// KeyVaultID specifies a key vault.
type KeyVaultID struct {
	ID string `json:"id,omitempty"`
}

// KeyVaultRef represents a reference to KeyVault instance on Azure.
type KeyVaultRef struct {
	KeyVault      KeyVaultID `json:"keyVault"`
	SecretName    string     `json:"secretName"`
	SecretVersion string     `json:"secretVersion,omitempty"`
}

// KeyVaultSecrets specifies certificates to install on the pool of machines from a given key vault.
// the key vault specified must have been granted read permissions to CRP.
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

// VMDiagnostics contains settings to on/off boot diagnostics collection in RD Host.
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

// OSType represents OS types of agents.
type OSType string

// the OSTypes supported by vlabs.
const (
	Windows OSType = "Windows"
	Linux   OSType = "Linux"
)

// KubeletDiskType describes options for placement of the primary kubelet partition.
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

// OutboundType describes the options for outbound internet access.
const (
	OutboundTypeNone  string = "none"
	OutboundTypeBlock string = "block"
)

/*
CommandLineOmittedKubeletConfigFlags are the flags set by RP that should NOT be included within the set of
command line flags when configuring kubelet.
*/
func GetCommandLineOmittedKubeletConfigFlags() map[string]bool {
	flags := map[string]bool{"--node-status-report-frequency": true}
	return flags
}

// Distro represents Linux distro to use for Linux VMs.
type Distro string

// Distro string consts.
const (
	Ubuntu                              Distro = "ubuntu"
	Ubuntu1804                          Distro = "ubuntu-18.04"
	Ubuntu1804Gen2                      Distro = "ubuntu-18.04-gen2"
	AKSUbuntu1804Gen2                   Distro = "ubuntu-18.04-gen2" // same distro as Ubuntu1804Gen2, renamed for clarity
	AKSUbuntu1604                       Distro = "aks-ubuntu-16.04"
	AKSUbuntu1804                       Distro = "aks-ubuntu-18.04"
	AKSUbuntuGPU1804                    Distro = "aks-ubuntu-gpu-18.04"
	AKSUbuntuGPU1804Gen2                Distro = "aks-ubuntu-gpu-18.04-gen2"
	AKSUbuntuContainerd1804             Distro = "aks-ubuntu-containerd-18.04"
	AKSUbuntuContainerd1804Gen2         Distro = "aks-ubuntu-containerd-18.04-gen2"
	AKSUbuntuGPUContainerd1804          Distro = "aks-ubuntu-gpu-containerd-18.04"
	AKSUbuntuGPUContainerd1804Gen2      Distro = "aks-ubuntu-gpu-containerd-18.04-gen2"
	AKSCBLMarinerV1                     Distro = "aks-cblmariner-v1"
	AKSCBLMarinerV2                     Distro = "aks-cblmariner-v2"
	AKSAzureLinuxV2                     Distro = "aks-azurelinux-v2"
	AKSAzureLinuxV3                     Distro = "aks-azurelinux-v3"
	AKSCBLMarinerV2Gen2                 Distro = "aks-cblmariner-v2-gen2"
	AKSAzureLinuxV2Gen2                 Distro = "aks-azurelinux-v2-gen2"
	AKSAzureLinuxV3Gen2                 Distro = "aks-azurelinux-v3-gen2"
	AKSCBLMarinerV2FIPS                 Distro = "aks-cblmariner-v2-fips"
	AKSAzureLinuxV2FIPS                 Distro = "aks-azurelinux-v2-fips"
	AKSAzureLinuxV3FIPS                 Distro = "aks-azurelinux-v3-fips"
	AKSCBLMarinerV2Gen2FIPS             Distro = "aks-cblmariner-v2-gen2-fips"
	AKSAzureLinuxV2Gen2FIPS             Distro = "aks-azurelinux-v2-gen2-fips"
	AKSAzureLinuxV3Gen2FIPS             Distro = "aks-azurelinux-v3-gen2-fips"
	AKSCBLMarinerV2Gen2Kata             Distro = "aks-cblmariner-v2-gen2-kata"
	AKSAzureLinuxV2Gen2Kata             Distro = "aks-azurelinux-v2-gen2-kata"
	AKSCBLMarinerV2Gen2TL               Distro = "aks-cblmariner-v2-gen2-tl"
	AKSAzureLinuxV2Gen2TL               Distro = "aks-azurelinux-v2-gen2-tl"
	AKSCBLMarinerV2KataGen2TL           Distro = "aks-cblmariner-v2-kata-gen2-tl"
	AKSUbuntuFipsContainerd1804         Distro = "aks-ubuntu-fips-containerd-18.04"
	AKSUbuntuFipsContainerd1804Gen2     Distro = "aks-ubuntu-fips-containerd-18.04-gen2"
	AKSUbuntuFipsContainerd2004         Distro = "aks-ubuntu-fips-containerd-20.04"
	AKSUbuntuFipsContainerd2004Gen2     Distro = "aks-ubuntu-fips-containerd-20.04-gen2"
	AKSUbuntuFipsContainerd2204         Distro = "aks-ubuntu-fips-containerd-22.04"
	AKSUbuntuFipsContainerd2204Gen2     Distro = "aks-ubuntu-fips-containerd-22.04-gen2"
	AKSUbuntuEdgeZoneContainerd1804     Distro = "aks-ubuntu-edgezone-containerd-18.04"
	AKSUbuntuEdgeZoneContainerd1804Gen2 Distro = "aks-ubuntu-edgezone-containerd-18.04-gen2"
	AKSUbuntuEdgeZoneContainerd2204     Distro = "aks-ubuntu-edgezone-containerd-22.04"
	AKSUbuntuEdgeZoneContainerd2204Gen2 Distro = "aks-ubuntu-edgezone-containerd-22.04-gen2"
	AKSUbuntuContainerd2204             Distro = "aks-ubuntu-containerd-22.04"
	AKSUbuntuContainerd2204Gen2         Distro = "aks-ubuntu-containerd-22.04-gen2"
	AKSUbuntuContainerd2004CVMGen2      Distro = "aks-ubuntu-containerd-20.04-cvm-gen2"
	AKSUbuntuArm64Containerd2204Gen2    Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
	AKSUbuntuArm64Containerd2404Gen2    Distro = "aks-ubuntu-arm64-containerd-24.04-gen2"
	AKSCBLMarinerV2Arm64Gen2            Distro = "aks-cblmariner-v2-arm64-gen2"
	AKSAzureLinuxV2Arm64Gen2            Distro = "aks-azurelinux-v2-arm64-gen2"
	AKSAzureLinuxV3Arm64Gen2            Distro = "aks-azurelinux-v3-arm64-gen2"
	AKSUbuntuContainerd2204TLGen2       Distro = "aks-ubuntu-containerd-22.04-tl-gen2"
	AKSUbuntuMinimalContainerd2204      Distro = "aks-ubuntu-minimal-containerd-22.04"
	AKSUbuntuMinimalContainerd2204Gen2  Distro = "aks-ubuntu-minimal-containerd-22.04-gen2"
	AKSUbuntuEgressContainerd2204Gen2   Distro = "aks-ubuntu-egress-containerd-22.04-gen2"
	AKSUbuntuContainerd2404             Distro = "aks-ubuntu-containerd-24.04"
	AKSUbuntuContainerd2404Gen2         Distro = "aks-ubuntu-containerd-24.04-gen2"

	RHEL              Distro = "rhel"
	CoreOS            Distro = "coreos"
	AKS1604Deprecated Distro = "aks"      // deprecated AKS 16.04 distro. Equivalent to aks-ubuntu-16.04.
	AKS1804Deprecated Distro = "aks-1804" // deprecated AKS 18.04 distro. Equivalent to aks-ubuntu-18.04.

	// Windows string const.
	// AKSWindows2019 stands for distro of windows server 2019 SIG image with docker.
	AKSWindows2019 Distro = "aks-windows-2019"
	// AKSWindows2019Containerd stands for distro for windows server 2019 SIG image with containerd.
	AKSWindows2019Containerd Distro = "aks-windows-2019-containerd"
	// AKSWindows2022Containerd stands for distro for windows server 2022 SIG image with containerd.
	AKSWindows2022Containerd Distro = "aks-windows-2022-containerd"
	// AKSWindows2022ContainerdGen2 stands for distro for windows server 2022 Gen 2 SIG image with containerd.
	AKSWindows2022ContainerdGen2 Distro = "aks-windows-2022-containerd-gen2"
	// AKSWindows23H2 stands for distro for windows 23H2 SIG image.
	AKSWindows23H2 Distro = "aks-windows-23H2"
	// AKSWindows23H2Gen2 stands for distro for windows 23H2 Gen 2 SIG image.
	AKSWindows23H2Gen2 Distro = "aks-windows-23H2-gen2"
	// AKSWindows2019PIR stands for distro of windows server 2019 PIR image with docker.
	AKSWindows2019PIR        Distro = "aks-windows-2019-pir"
	CustomizedImage          Distro = "CustomizedImage"
	CustomizedImageKata      Distro = "CustomizedImageKata"
	CustomizedWindowsOSImage Distro = "CustomizedWindowsOSImage"

	// USNatCloud is a const string reference identifier for USNat.
	USNatCloud = "USNatCloud"
	// USSecCloud is a const string reference identifier for USSec.
	USSecCloud = "USSecCloud"
)

//nolint:gochecknoglobals
var AKSDistrosAvailableOnVHD = []Distro{
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
	AKSCBLMarinerV2,
	AKSAzureLinuxV2,
	AKSAzureLinuxV3,
	AKSCBLMarinerV2Gen2,
	AKSAzureLinuxV2Gen2,
	AKSAzureLinuxV3Gen2,
	AKSCBLMarinerV2FIPS,
	AKSAzureLinuxV2FIPS,
	AKSAzureLinuxV3FIPS,
	AKSCBLMarinerV2Gen2FIPS,
	AKSAzureLinuxV2Gen2FIPS,
	AKSAzureLinuxV3Gen2FIPS,
	AKSCBLMarinerV2Gen2Kata,
	AKSAzureLinuxV2Gen2Kata,
	AKSCBLMarinerV2Gen2TL,
	AKSAzureLinuxV2Gen2TL,
	AKSCBLMarinerV2KataGen2TL,
	AKSUbuntuFipsContainerd1804,
	AKSUbuntuFipsContainerd1804Gen2,
	AKSUbuntuFipsContainerd2004,
	AKSUbuntuFipsContainerd2004Gen2,
	AKSUbuntuFipsContainerd2204,
	AKSUbuntuFipsContainerd2204Gen2,
	AKSUbuntuEdgeZoneContainerd1804,
	AKSUbuntuEdgeZoneContainerd1804Gen2,
	AKSUbuntuEdgeZoneContainerd2204,
	AKSUbuntuEdgeZoneContainerd2204Gen2,
	AKSUbuntuContainerd2204,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuContainerd2004CVMGen2,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSUbuntuArm64Containerd2404Gen2,
	AKSCBLMarinerV2Arm64Gen2,
	AKSAzureLinuxV2Arm64Gen2,
	AKSAzureLinuxV3Arm64Gen2,
	AKSUbuntuContainerd2204TLGen2,
	AKSUbuntuMinimalContainerd2204,
	AKSUbuntuMinimalContainerd2204Gen2,
	AKSUbuntuContainerd2404,
	AKSUbuntuContainerd2404Gen2,
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

func (d Distro) Is2204VHDDistro() bool {
	for _, distro := range AvailableUbuntu2204Distros {
		if d == distro {
			return true
		}
	}
	return false
}

// This function will later be consumed by CSE to determine cgroupv2 usage.
func (d Distro) Is2404VHDDistro() bool {
	for _, distro := range AvailableUbuntu2404Distros {
		if d == distro {
			return true
		}
	}
	return false
}

func (d Distro) IsAzureLinuxCgroupV2VHDDistro() bool {
	for _, distro := range AvailableAzureLinuxCgroupV2Distros {
		if d == distro {
			return true
		}
	}
	return false
}

func (d Distro) IsKataDistro() bool {
	return d == AKSCBLMarinerV2Gen2Kata || d == AKSAzureLinuxV2Gen2Kata || d == AKSCBLMarinerV2KataGen2TL || d == CustomizedImageKata
}

/*
KeyvaultSecretRef specifies path to the Azure keyvault along with secret name and (optionaly) version
for Service Principal's secret.
*/
type KeyvaultSecretRef struct {
	VaultID       string `json:"vaultID"`
	SecretName    string `json:"secretName"`
	SecretVersion string `json:"version,omitempty"`
}

// AuthenticatorType represents the authenticator type the cluster was.
// set up with.
type AuthenticatorType string

const (
	// OIDC represent cluster setup in OIDC auth mode.
	OIDC AuthenticatorType = "oidc"
	// Webhook represent cluster setup in wehhook auth mode.
	Webhook AuthenticatorType = "webhook"
)

// UserAssignedIdentity contains information that uniquely identifies an identity.
type UserAssignedIdentity struct {
	ResourceID string `json:"resourceId,omitempty"`
	ClientID   string `json:"clientId,omitempty"`
	ObjectID   string `json:"objectId,omitempty"`
}

// ResourceIdentifiers represents resource ids.
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
	// TODO(ace): why is Name uppercase?
	// in Linux, this was historically specified as "name" when serialized.
	// However Windows relies on the json tag as "Name".
	// TODO(ace): can we align on one casing?
	SnakeCaseName                string              `json:"name,omitempty"`
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

// FeatureFlags defines feature-flag restricted functionality.
type FeatureFlags struct {
	EnableCSERunInBackground bool `json:"enableCSERunInBackground,omitempty"`
	BlockOutboundInternet    bool `json:"blockOutboundInternet,omitempty"`
	EnableIPv6DualStack      bool `json:"enableIPv6DualStack,omitempty"`
	EnableIPv6Only           bool `json:"enableIPv6Only,omitempty"`
	EnableWinDSR             bool `json:"enableWinDSR,omitempty"`
}

// AddonProfile represents an addon for managed cluster.
type AddonProfile struct {
	Enabled bool              `json:"enabled"`
	Config  map[string]string `json:"config"`
	// Identity contains information of the identity associated with this addon.
	// This property will only appear in an MSI-enabled cluster.
	Identity *UserAssignedIdentity `json:"identity,omitempty"`
}

// HostedMasterProfile defines properties for a hosted master.
type HostedMasterProfile struct {
	// Master public endpoint/FQDN with port.
	// The format will be FQDN:2376.
	// Not used during PUT, returned as part of GETFQDN.
	FQDN string `json:"fqdn,omitempty"`
	// IPAddress.
	// if both FQDN and IPAddress are specified, we should use IPAddress.
	IPAddress string `json:"ipAddress,omitempty"`
	DNSPrefix string `json:"dnsPrefix"`
	// FQDNSubdomain is used by private cluster without dnsPrefix so they have fixed FQDN.
	FQDNSubdomain string `json:"fqdnSubdomain"`
	/* Subnet holds the CIDR which defines the Azure Subnet in which
	Agents will be provisioned. This is stored on the HostedMasterProfile
	and will become `masterSubnet` in the compiled template. */
	Subnet string `json:"subnet"`
	// ApiServerWhiteListRange is a comma delimited CIDR which is whitelisted to AKS.
	APIServerWhiteListRange *string `json:"apiServerWhiteListRange"`
	IPMasqAgent             bool    `json:"ipMasqAgent"`
}

// CustomProfile specifies custom properties that are used for cluster instantiation.
// Should not be used by most users.
type CustomProfile struct {
	Orchestrator string `json:"orchestrator,omitempty"`
}

// AADProfile specifies attributes for AAD integration.
type AADProfile struct {
	// The client AAD application ID.
	ClientAppID string `json:"clientAppID,omitempty"`
	// The server AAD application ID.
	ServerAppID string `json:"serverAppID,omitempty"`
	// The server AAD application secret.
	ServerAppSecret string `json:"serverAppSecret,omitempty" conform:"redact"`
	// The AAD tenant ID to use for authentication.
	// If not specified, will use the tenant of the deployment subscription.
	// Optional.
	TenantID string `json:"tenantID,omitempty"`
	// The Azure Active Directory Group Object ID that will be assigned the cluster-admin RBAC role.
	// Optional.
	AdminGroupID string `json:"adminGroupID,omitempty"`
	// The authenticator to use, either "oidc" or "webhook".
	Authenticator AuthenticatorType `json:"authenticator"`
}

// CertificateProfile represents the definition of the master cluster.
type CertificateProfile struct {
	// CaCertificate is the certificate authority certificate.
	CaCertificate string `json:"caCertificate,omitempty" conform:"redact"`
	// ApiServerCertificate is the rest api server certificate, and signed by the CA.
	APIServerCertificate string `json:"apiServerCertificate,omitempty" conform:"redact"`
	// ClientCertificate is the certificate used by the client kubelet services and signed by the CA.
	ClientCertificate string `json:"clientCertificate,omitempty" conform:"redact"`
	// ClientPrivateKey is the private key used by the client kubelet services and signed by the CA.
	ClientPrivateKey string `json:"clientPrivateKey,omitempty" conform:"redact"`
	// KubeConfigCertificate is the client certificate used for kubectl cli and signed by the CA.
	KubeConfigCertificate string `json:"kubeConfigCertificate,omitempty" conform:"redact"`
	// KubeConfigPrivateKey is the client private key used for kubectl cli and signed by the CA.
	KubeConfigPrivateKey string `json:"kubeConfigPrivateKey,omitempty" conform:"redact"`
}

// ServicePrincipalProfile contains the client and secret used by the cluster for Azure Resource CRUD.
type ServicePrincipalProfile struct {
	ClientID          string             `json:"clientId"`
	Secret            string             `json:"secret,omitempty" conform:"redact"`
	ObjectID          string             `json:"objectId,omitempty"`
	KeyvaultSecretRef *KeyvaultSecretRef `json:"keyvaultSecretRef,omitempty"`
}

// DiagnosticsProfile setting to enable/disable capturing.
// diagnostics for VMs hosting container cluster.
type DiagnosticsProfile struct {
	VMDiagnostics *VMDiagnostics `json:"vmDiagnostics"`
}

// ExtensionProfile represents an extension definition.
type ExtensionProfile struct {
	Name                           string             `json:"name"`
	Version                        string             `json:"version"`
	ExtensionParameters            string             `json:"extensionParameters,omitempty"`
	ExtensionParametersKeyVaultRef *KeyvaultSecretRef `json:"parametersKeyvaultSecretRef,omitempty"`
	RootURL                        string             `json:"rootURL,omitempty"`
	// This is only needed for preprovision extensions and it needs to be a bash script.
	Script   string `json:"script,omitempty"`
	URLQuery string `json:"urlQuery,omitempty"`
}

// ResourcePurchasePlan defines resource plan as required by ARM for billing purposes.
type ResourcePurchasePlan struct {
	Name          string `json:"name"`
	Product       string `json:"product"`
	PromotionCode string `json:"promotionCode"`
	Publisher     string `json:"publisher"`
}

// WindowsProfile represents the windows parameters passed to the cluster.
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
	//nolint:stylecheck // keep field names the same as RP
	WindowsSecureTlsEnabled *bool `json:"windowsSecureTlsEnabled,omitempty"`
	//nolint:stylecheck // keep field names the same as RP
	WindowsGmsaPackageUrl          string  `json:"windowsGmsaPackageUrl,omitempty"`
	CseScriptsPackageURL           string  `json:"cseScriptsPackageURL,omitempty"`
	GpuDriverURL                   string  `json:"gpuDriverUrl,omitempty"`
	HnsRemediatorIntervalInMinutes *uint32 `json:"hnsRemediatorIntervalInMinutes,omitempty"`
	LogGeneratorIntervalInMinutes  *uint32 `json:"logGeneratorIntervalInMinutes,omitempty"`
}

// ContainerdWindowsRuntimes configures containerd runtimes that are available on the windows nodes.
type ContainerdWindowsRuntimes struct {
	DefaultSandboxIsolation string            `json:"defaultSandboxIsolation,omitempty"`
	RuntimeHandlers         []RuntimeHandlers `json:"runtimesHandlers,omitempty"`
}

// RuntimeHandlers configures the runtime settings in containerd.
type RuntimeHandlers struct {
	BuildNumber string `json:"buildNumber,omitempty"`
}

// LinuxProfile represents the linux parameters passed to the cluster.
type LinuxProfile struct {
	AdminUsername string `json:"adminUsername"`
	SSH           struct {
		PublicKeys []PublicKey `json:"publicKeys"`
	} `json:"ssh"`
	Secrets            []KeyVaultSecrets   `json:"secrets,omitempty"`
	Distro             Distro              `json:"distro,omitempty"`
	CustomSearchDomain *CustomSearchDomain `json:"customSearchDomain,omitempty"`
}

// Extension represents an extension definition in the master or agentPoolProfile.
type Extension struct {
	Name        string `json:"name"`
	SingleOrAll string `json:"singleOrAll"`
	Template    string `json:"template"`
}

// PrivateJumpboxProfile represents a jumpbox definition.
type PrivateJumpboxProfile struct {
	Name           string `json:"name" validate:"required"`
	VMSize         string `json:"vmSize" validate:"required"`
	OSDiskSizeGB   int    `json:"osDiskSizeGB,omitempty" validate:"min=0,max=2048"`
	Username       string `json:"username,omitempty"`
	PublicKey      string `json:"publicKey" validate:"required"`
	StorageProfile string `json:"storageProfile,omitempty"`
}

// PrivateCluster defines the configuration for a private cluster.
type PrivateCluster struct {
	Enabled                *bool                  `json:"enabled,omitempty"`
	EnableHostsConfigAgent *bool                  `json:"enableHostsConfigAgent,omitempty"`
	JumpboxProfile         *PrivateJumpboxProfile `json:"jumpboxProfile,omitempty"`
}

// KubernetesContainerSpec defines configuration for a container spec.
type KubernetesContainerSpec struct {
	Name           string `json:"name,omitempty"`
	Image          string `json:"image,omitempty"`
	CPURequests    string `json:"cpuRequests,omitempty"`
	MemoryRequests string `json:"memoryRequests,omitempty"`
	CPULimits      string `json:"cpuLimits,omitempty"`
	MemoryLimits   string `json:"memoryLimits,omitempty"`
}

// AddonNodePoolsConfig defines configuration for pool-specific cluster-autoscaler configuration.
type AddonNodePoolsConfig struct {
	Name   string            `json:"name,omitempty"`
	Config map[string]string `json:"config,omitempty"`
}

// KubernetesAddon defines a list of addons w/ configuration to include with the cluster deployment.
type KubernetesAddon struct {
	Name       string                    `json:"name,omitempty"`
	Enabled    *bool                     `json:"enabled,omitempty"`
	Mode       string                    `json:"mode,omitempty"`
	Containers []KubernetesContainerSpec `json:"containers,omitempty"`
	Config     map[string]string         `json:"config,omitempty"`
	Pools      []AddonNodePoolsConfig    `json:"pools,omitempty"`
	Data       string                    `json:"data,omitempty"`
}

// KubernetesConfig contains the Kubernetes config structure, containing Kubernetes specific configuration.
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
	UserAssignedClientID              string            `json:"userAssignedClientID,omitempty"` //nolint: lll // Note: cannot be provided in config. Used *only* for transferring this to azure.json.
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
	NetworkPluginMode                 string            `json:"networkPluginMode,omitempty"`
}

/*
CustomFile has source as the full absolute source path to a file and dest
is the full absolute desired destination path to put the file on a master node.
*/
type CustomFile struct {
	Source string `json:"source,omitempty"`
	Dest   string `json:"dest,omitempty"`
}

// OrchestratorProfile contains Orchestrator properties.
type OrchestratorProfile struct {
	OrchestratorType    string            `json:"orchestratorType"`
	OrchestratorVersion string            `json:"orchestratorVersion"`
	KubernetesConfig    *KubernetesConfig `json:"kubernetesConfig,omitempty"`
}

// ProvisioningState represents the current state of container service resource.
type ProvisioningState string

// CustomKubeletConfig represents custom kubelet configurations for agent pool nodes.
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
	SeccompDefault        *bool     `json:"seccompDefault,omitempty"`
}

// CustomLinuxOSConfig represents custom os configurations for agent pool nodes.
type CustomLinuxOSConfig struct {
	Sysctls                    *SysctlConfig `json:"sysctls,omitempty"`
	TransparentHugePageEnabled string        `json:"transparentHugePageEnabled,omitempty"`
	TransparentHugePageDefrag  string        `json:"transparentHugePageDefrag,omitempty"`
	SwapFileSizeMB             *int32        `json:"swapFileSizeMB,omitempty"`
	UlimitConfig               *UlimitConfig `json:"ulimitConfig,omitempty"`
}

func (c *CustomLinuxOSConfig) GetUlimitConfig() *UlimitConfig {
	if c == nil {
		return nil
	}
	return c.UlimitConfig
}

// SysctlConfig represents sysctl configs in customLinuxOsConfig.
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

type UlimitConfig struct {
	MaxLockedMemory string `json:"maxLockedMemory ,omitempty"`
	NoFile          string `json:"noFile,omitempty"`
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

// AgentPoolProfile represents an agent pool definition.
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
	/* This is a new property and all old agent pools do no have this field. We need to keep the default
	behavior to reboot Windows node when it is nil. */
	NotRebootWindowsNode    *bool                    `json:"notRebootWindowsNode,omitempty"`
	AgentPoolWindowsProfile *AgentPoolWindowsProfile `json:"agentPoolWindowsProfile,omitempty"`
}

func (a *AgentPoolProfile) GetCustomLinuxOSConfig() *CustomLinuxOSConfig {
	if a == nil {
		return nil
	}
	return a.CustomLinuxOSConfig
}

func (a *AgentPoolProfile) GetAgentPoolWindowsProfile() *AgentPoolWindowsProfile {
	if a == nil {
		return nil
	}
	return a.AgentPoolWindowsProfile
}

// Properties represents the AKS cluster definition.
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
	CustomCloudEnv          *CustomCloudEnv          `json:"customCloudEnv,omitempty"`
	CustomConfiguration     *CustomConfiguration     `json:"customConfiguration,omitempty"`
	SecurityProfile         *SecurityProfile         `json:"securityProfile,omitempty"`
}

// ContainerService complies with the ARM model of resource definition in a JSON template.
type ContainerService struct {
	ID       string                `json:"id"`
	Location string                `json:"location"`
	Name     string                `json:"name"`
	Plan     *ResourcePurchasePlan `json:"plan,omitempty"`
	Tags     map[string]string     `json:"tags"`
	Type     string                `json:"type"`

	Properties *Properties `json:"properties,omitempty"`
}

// IsAKSCustomCloud checks if it's in AKS custom cloud.
func (cs *ContainerService) IsAKSCustomCloud() bool {
	return cs.Properties.CustomCloudEnv != nil &&
		strings.EqualFold(cs.Properties.CustomCloudEnv.Name, "akscustom")
}

// HasAadProfile returns true if the has aad profile.
func (p *Properties) HasAadProfile() bool {
	return p.AADProfile != nil
}

/*
GetCustomCloudName returns name of environment if customCloudProfile is provided, returns empty string if
customCloudProfile is empty.Because customCloudProfile is empty for deployment is AzurePublicCloud,
AzureChinaCloud, AzureGermanCloud, AzureUSGovernmentCloud, the return value will be empty string for those
clouds.
*/
func (p *Properties) GetCustomCloudName() string {
	var cloudProfileName string
	if p.IsAKSCustomCloud() {
		cloudProfileName = p.CustomCloudEnv.Name
	}
	return cloudProfileName
}

// IsIPMasqAgentDisabled returns true if the ip-masq-agent functionality is disabled.
func (p *Properties) IsIPMasqAgentDisabled() bool {
	if p.HostedMasterProfile != nil {
		return !p.HostedMasterProfile.IPMasqAgent
	}
	if p.OrchestratorProfile != nil && p.OrchestratorProfile.KubernetesConfig != nil {
		return p.OrchestratorProfile.KubernetesConfig.IsIPMasqAgentDisabled()
	}
	return false
}

// HasWindows returns true if the cluster contains windows.
func (p *Properties) HasWindows() bool {
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if strings.EqualFold(string(agentPoolProfile.OSType), string(Windows)) {
			return true
		}
	}
	return false
}

// IsAKSCustomCloud checks if it's in AKS custom cloud.
func (p *Properties) IsAKSCustomCloud() bool {
	return p.CustomCloudEnv != nil &&
		strings.EqualFold(p.CustomCloudEnv.Name, "akscustom")
}

// IsIPMasqAgentEnabled returns true if the cluster has a hosted master and IpMasqAgent is disabled.
func (p *Properties) IsIPMasqAgentEnabled() bool {
	if p.HostedMasterProfile != nil {
		return p.HostedMasterProfile.IPMasqAgent
	}
	return p.OrchestratorProfile.KubernetesConfig.IsIPMasqAgentEnabled()
}

// GetClusterID creates a unique 8 string cluster ID.
func (p *Properties) GetClusterID() string {
	mutex := &sync.Mutex{}
	if p.ClusterID == "" {
		uniqueNameSuffixSize := 8
		/* the name suffix uniquely identifies the cluster and is generated off a hash from the
		master dns name. */
		h := fnv.New64a()
		if p.HostedMasterProfile != nil {
			h.Write([]byte(p.HostedMasterProfile.DNSPrefix))
		} else if len(p.AgentPoolProfiles) > 0 {
			h.Write([]byte(p.AgentPoolProfiles[0].Name))
		}
		//nolint:gosec // I think we want rand not crypto/rand here
		r := rand.New(rand.NewSource(int64(h.Sum64())))
		mutex.Lock()
		p.ClusterID = fmt.Sprintf("%08d", r.Uint32())[:uniqueNameSuffixSize]
		mutex.Unlock()
	}
	return p.ClusterID
}

/*
AreAgentProfilesCustomVNET returns true if all of the agent profiles in the clusters are
configured with VNET.
*/
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

// GetCustomEnvironmentJSON return the JSON format string for custom environment.
func (p *Properties) GetCustomEnvironmentJSON(escape bool) (string, error) {
	var environmentJSON string
	if p.IsAKSCustomCloud() {
		// Workaround to set correct name in AzureStackCloud.json.
		oldName := p.CustomCloudEnv.Name
		p.CustomCloudEnv.Name = AzureStackCloud
		p.CustomCloudEnv.SnakeCaseName = AzureStackCloud
		defer func() {
			// Restore p.CustomCloudEnv to old value.
			p.CustomCloudEnv.Name = oldName
		}()
		bytes, err := json.Marshal(p.CustomCloudEnv)
		if err != nil {
			return "", fmt.Errorf("could not serialize CustomCloudEnv object - %w", err)
		}
		environmentJSON = string(bytes)
		if escape {
			environmentJSON = strings.ReplaceAll(environmentJSON, "\"", "\\\"")
		}
	}
	return environmentJSON, nil
}

// HasDCSeriesSKU returns whether or not there is an DC series SKU agent pool.
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

// IsVHDDistroForAllNodes returns true if all of the agent pools plus masters are running the VHD image.
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

// GetVMType returns the type of VM "vmss" or "standard" to be passed to the cloud provider.
func (p *Properties) GetVMType() string {
	if p.HasVMSSAgentPool() {
		return VMSSVMType
	}
	return StandardVMType
}

// HasVMSSAgentPool returns true if the cluster contains Virtual Machine Scale Sets agent pools.
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

// GetResourcePrefix returns the prefix to use for naming cluster resources.
func (p *Properties) GetResourcePrefix() string {
	return p.K8sOrchestratorName() + "-agentpool-" + p.GetClusterID() + "-"
}

// GetVirtualNetworkName returns the virtual network name of the cluster.
func (p *Properties) GetVirtualNetworkName() string {
	var vnetName string
	if p.AreAgentProfilesCustomVNET() {
		vnetName = strings.Split(p.AgentPoolProfiles[0].VnetSubnetID, "/")[DefaultVnetNameResourceSegmentIndex]
	} else {
		vnetName = p.K8sOrchestratorName() + "-vnet-" + p.GetClusterID()
	}
	return vnetName
}

// GetVNetResourceGroupName returns the virtual network resource group name of the cluster.
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

// GetPrimaryAvailabilitySetName returns the name of the primary availability set of the cluster.
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

/*
GetKubeProxyFeatureGatesWindowsArguments returns the feature gates string for the kube-proxy arguments
in Windows nodes.
*/
func (p *Properties) GetKubeProxyFeatureGatesWindowsArguments() string {
	featureGates := map[string]bool{}

	if p.FeatureFlags.IsFeatureEnabled(EnableIPv6DualStack) &&
		p.OrchestratorProfile.VersionSupportsFeatureFlag(EnableIPv6DualStack) {
		featureGates["IPv6DualStack"] = true
	}
	if p.FeatureFlags.IsFeatureEnabled(EnableWinDSR) {
		// WinOverlay must be set to false.
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

// IsVHDDistro returns true if the distro uses VHD SKUs.
func (a *AgentPoolProfile) IsVHDDistro() bool {
	return a.Distro.IsVHDDistro()
}

// Is2204VHDDistro returns true if the distro uses 2204 VHD.
func (a *AgentPoolProfile) Is2204VHDDistro() bool {
	return a.Distro.Is2204VHDDistro()
}

// Is2404VHDDistro returns true if the distro uses 2404 VHD.
func (a *AgentPoolProfile) Is2404VHDDistro() bool {
	return a.Distro.Is2404VHDDistro()
}

// IsAzureLinuxCgroupV2VHDDistro returns true if the distro uses Azure Linux CgrpupV2 VHD.
func (a *AgentPoolProfile) IsAzureLinuxCgroupV2VHDDistro() bool {
	return a.Distro.IsAzureLinuxCgroupV2VHDDistro()
}

// IsCustomVNET returns true if the customer brought their own VNET.
func (a *AgentPoolProfile) IsCustomVNET() bool {
	return len(a.VnetSubnetID) > 0
}

// IsWindows returns true if the agent pool is windows.
func (a *AgentPoolProfile) IsWindows() bool {
	return strings.EqualFold(string(a.OSType), string(Windows))
}

// IsSkipCleanupNetwork returns true if AKS-RP sets the field NotRebootWindowsNode to true.
func (a *AgentPoolProfile) IsSkipCleanupNetwork() bool {
	// Reuse the existing field NotRebootWindowsNode to avoid adding a new field because it is a temporary toggle value from AKS-RP.
	return a.NotRebootWindowsNode != nil && *a.NotRebootWindowsNode
}

// IsVirtualMachineScaleSets returns true if the agent pool availability profile is VMSS.
func (a *AgentPoolProfile) IsVirtualMachineScaleSets() bool {
	return strings.EqualFold(a.AvailabilityProfile, VirtualMachineScaleSets)
}

// IsAvailabilitySets returns true if the customer specified disks.
func (a *AgentPoolProfile) IsAvailabilitySets() bool {
	return strings.EqualFold(a.AvailabilityProfile, AvailabilitySet)
}

// GetKubernetesLabels returns a k8s API-compliant labels string for nodes in this profile.
func (a *AgentPoolProfile) GetKubernetesLabels() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("agentpool=%s", a.Name))
	buf.WriteString(fmt.Sprintf(",kubernetes.azure.com/agentpool=%s", a.Name))

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

// HasSecrets returns true if the customer specified secrets to install.
func (l *LinuxProfile) HasSecrets() bool {
	return len(l.Secrets) > 0
}

// HasSearchDomain returns true if the customer specified secrets to install.
func (l *LinuxProfile) HasSearchDomain() bool {
	if l.CustomSearchDomain != nil {
		if l.CustomSearchDomain.Name != "" && l.CustomSearchDomain.RealmPassword != "" && l.CustomSearchDomain.RealmUser != "" {
			return true
		}
	}
	return false
}

// IsAzureCNI returns true if Azure CNI network plugin is enabled.
func (o *OrchestratorProfile) IsAzureCNI() bool {
	if o.KubernetesConfig != nil {
		return strings.EqualFold(o.KubernetesConfig.NetworkPlugin, NetworkPluginAzure)
	}
	return false
}

// IsNoneCNI returns true if network plugin none is enabled.
func (o *OrchestratorProfile) IsNoneCNI() bool {
	if o.KubernetesConfig != nil {
		return strings.EqualFold(o.KubernetesConfig.NetworkPlugin, NetworkPluginNone)
	}
	return false
}

func (o *OrchestratorProfile) VersionSupportsFeatureFlag(flag string) bool {
	switch flag {
	case EnableIPv6DualStack:
		// unversioned will retrun true to maintain backwards compatibility
		// IPv6DualStack flag was removed in 1.25.0 and is enabled by default
		// since 1.21. It is supported between 1.15-1.24.
		// https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates-removed/.
		return o == nil || o.OrchestratorVersion == "" || o.VersionIs(">= 1.15.0 < 1.25.0")
	default:
		return false
	}
}

// VersionIs takes a constraint expression to validate
// the OrchestratorVersion meets this constraint. Examples
// of expressions are `>= 1.24` or `!= 1.25.4`.
// More info: https://github.com/Masterminds/semver#checking-version-constraints.
func (o *OrchestratorProfile) VersionIs(expr string) bool {
	if o == nil || o.OrchestratorVersion == "" {
		return false
	}

	version := semver.MustParse(o.OrchestratorVersion)
	constraint, _ := semver.NewConstraint(expr)
	if constraint == nil {
		return false
	}
	return constraint.Check(version)
}

// IsCSIProxyEnabled returns true if csi proxy service should be enable for Windows nodes.
func (w *WindowsProfile) IsCSIProxyEnabled() bool {
	if w.EnableCSIProxy != nil {
		return *w.EnableCSIProxy
	}
	return DefaultEnableCSIProxyWindows
}

// HasSecrets returns true if the customer specified secrets to install.
func (w *WindowsProfile) HasSecrets() bool {
	return len(w.Secrets) > 0
}

// HasCustomImage returns true if there is a custom windows os image url specified.
func (w *WindowsProfile) HasCustomImage() bool {
	return len(w.WindowsImageSourceURL) > 0
}

// GetSSHEnabled gets it ssh should be enabled for Windows nodes.
func (w *WindowsProfile) GetSSHEnabled() bool {
	if w.SSHEnabled != nil {
		return *w.SSHEnabled
	}
	return DefaultWindowsSSHEnabled
}

// HasImageRef returns true if the customer brought os image.
func (w *WindowsProfile) HasImageRef() bool {
	return w.ImageRef != nil && w.ImageRef.IsValid()
}

/*
GetWindowsSku gets the marketplace sku specified (such as Datacenter-Core-1809-with-Containers-smalldisk)
or returns default value.
*/
func (w *WindowsProfile) GetWindowsSku() string {
	if w.WindowsSku != "" {
		return w.WindowsSku
	}
	return KubernetesDefaultWindowsSku
}

// GetWindowsDockerVersion gets the docker version specified or returns default value.
func (w *WindowsProfile) GetWindowsDockerVersion() string {
	if w.WindowsDockerVersion != "" {
		return w.WindowsDockerVersion
	}
	return KubernetesWindowsDockerVersion
}

/*
GetDefaultContainerdWindowsSandboxIsolation gets the default containerd runtime handler
or return default value.
*/
func (w *WindowsProfile) GetDefaultContainerdWindowsSandboxIsolation() string {
	if w.ContainerdWindowsRuntimes != nil && w.ContainerdWindowsRuntimes.DefaultSandboxIsolation != "" {
		return w.ContainerdWindowsRuntimes.DefaultSandboxIsolation
	}

	return KubernetesDefaultContainerdWindowsSandboxIsolation
}

// GetContainerdWindowsRuntimeHandlers gets comma separated list of runtimehandler names.
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

// IsAlwaysPullWindowsPauseImage returns true if the windows pause image always needs a force pull.
func (w *WindowsProfile) IsAlwaysPullWindowsPauseImage() bool {
	return w.AlwaysPullWindowsPauseImage != nil && *w.AlwaysPullWindowsPauseImage
}

// IsWindowsSecureTLSEnabled returns true if secure TLS should be enabled for Windows nodes.
//
//nolint:stylecheck // allign func name with field name
func (w *WindowsProfile) IsWindowsSecureTlsEnabled() bool {
	if w.WindowsSecureTlsEnabled != nil {
		return *w.WindowsSecureTlsEnabled
	}
	return DefaultWindowsSecureTLSEnabled
}

// GetHnsRemediatorIntervalInMinutes gets HnsRemediatorIntervalInMinutes specified or returns default value.
func (w *WindowsProfile) GetHnsRemediatorIntervalInMinutes() uint32 {
	if w.HnsRemediatorIntervalInMinutes != nil {
		return *w.HnsRemediatorIntervalInMinutes
	}
	return 0
}

// GetLogGeneratorIntervalInMinutes gets LogGeneratorIntervalInMinutes specified or returns default value.
func (w *WindowsProfile) GetLogGeneratorIntervalInMinutes() uint32 {
	if w.LogGeneratorIntervalInMinutes != nil {
		return *w.LogGeneratorIntervalInMinutes
	}
	return 0
}

// IsKubernetes returns true if this template is for Kubernetes orchestrator.
func (o *OrchestratorProfile) IsKubernetes() bool {
	return strings.EqualFold(o.OrchestratorType, Kubernetes)
}

// IsFeatureEnabled returns true if a feature flag is on for the provided feature.
func (f *FeatureFlags) IsFeatureEnabled(feature string) bool {
	if f != nil {
		switch feature {
		case CSERunInBackground:
			return f.EnableCSERunInBackground
		case BlockOutboundInternet:
			return f.BlockOutboundInternet
		case EnableIPv6DualStack:
			return f.EnableIPv6DualStack
		case EnableIPv6Only:
			return f.EnableIPv6Only
		case EnableWinDSR:
			return f.EnableWinDSR
		default:
			return false
		}
	}
	return false
}

// IsValid returns true if ImageRefernce contains at least Name and ResourceGroup.
func (i *ImageReference) IsValid() bool {
	return len(i.Name) > 0 && len(i.ResourceGroup) > 0
}

/* IsAddonEnabled checks whether a k8s addon with name "addonName" is enabled or not based on the Enabled
field of KubernetesAddon. */
// If the value of Enabled is nil, the "defaultValue" is returned.
func (k *KubernetesConfig) IsAddonEnabled(addonName string) bool {
	kubeAddon := k.GetAddonByName(addonName)
	return kubeAddon.IsEnabled()
}

// PrivateJumpboxProvision checks if a private cluster has jumpbox auto-provisioning.
func (k *KubernetesConfig) PrivateJumpboxProvision() bool {
	if k != nil && k.PrivateCluster != nil && *k.PrivateCluster.Enabled && k.PrivateCluster.JumpboxProfile != nil {
		return true
	}
	return false
}

// IsRBACEnabled checks if RBAC is enabled.
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

// IsIPMasqAgentDisabled checks if the ip-masq-agent addon is disabled.
func (k *KubernetesConfig) IsIPMasqAgentDisabled() bool {
	return k.IsAddonDisabled(IPMASQAgentAddonName)
}

// IsIPMasqAgentEnabled checks if the ip-masq-agent addon is enabled.
func (k *KubernetesConfig) IsIPMasqAgentEnabled() bool {
	return k.IsAddonEnabled(IPMASQAgentAddonName)
}

// GetAddonByName returns the KubernetesAddon instance with name `addonName`.
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

/* IsAddonDisabled checks whether a k8s addon with name "addonName"
is explicitly disabled based on the Enabled field of KubernetesAddon. */
// If the value of Enabled is nil, we return false (not explicitly disabled).
func (k *KubernetesConfig) IsAddonDisabled(addonName string) bool {
	kubeAddon := k.GetAddonByName(addonName)
	return kubeAddon.IsDisabled()
}

// NeedsContainerd returns whether or not we need the containerd runtime configuration.
// E.g., kata configuration requires containerd config.
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

// IsAADPodIdentityEnabled checks if the AAD pod identity addon is enabled.
func (k *KubernetesConfig) IsAADPodIdentityEnabled() bool {
	return k.IsAddonEnabled(AADPodIdentityAddonName)
}

// GetAzureCNIURLLinux returns the full URL to source Azure CNI binaries from.
func (k *KubernetesConfig) GetAzureCNIURLLinux(cloudSpecConfig *AzureEnvironmentSpecConfig) string {
	if k.AzureCNIURLLinux != "" {
		return k.AzureCNIURLLinux
	}
	return cloudSpecConfig.KubernetesSpecConfig.VnetCNILinuxPluginsDownloadURL
}

// GetAzureCNIURLARM64Linux returns the full URL to source Azure CNI binaries for ARM64 Linux from.
func (k *KubernetesConfig) GetAzureCNIURLARM64Linux(cloudSpecConfig *AzureEnvironmentSpecConfig) string {
	if k.AzureCNIURLARM64Linux != "" {
		return k.AzureCNIURLARM64Linux
	}
	return cloudSpecConfig.KubernetesSpecConfig.VnetCNIARM64LinuxPluginsDownloadURL
}

// GetAzureCNIURLWindows returns the full URL to source Azure CNI binaries from.
func (k *KubernetesConfig) GetAzureCNIURLWindows(cloudSpecConfig *AzureEnvironmentSpecConfig) string {
	if k.AzureCNIURLWindows != "" {
		return k.AzureCNIURLWindows
	}
	return cloudSpecConfig.KubernetesSpecConfig.VnetCNIWindowsPluginsDownloadURL
}

// IsUsingNetworkPluginMode returns true of NetworkPluginMode matches mode param.
func (k *KubernetesConfig) IsUsingNetworkPluginMode(mode string) bool {
	return strings.EqualFold(k.NetworkPluginMode, mode)
}

func setCustomKubletConfigFromSettings(customKc *CustomKubeletConfig, kubeletConfig map[string]string) map[string]string {
	// Settings from customKubeletConfig, only take if it's set.
	if customKc != nil {
		if customKc.ImageGcHighThreshold != nil {
			kubeletConfig["--image-gc-high-threshold"] = fmt.Sprintf("%d", *customKc.ImageGcHighThreshold)
		}
		if customKc.ImageGcLowThreshold != nil {
			kubeletConfig["--image-gc-low-threshold"] = fmt.Sprintf("%d", *customKc.ImageGcLowThreshold)
		}
		if customKc.ContainerLogMaxSizeMB != nil {
			kubeletConfig["--container-log-max-size"] = fmt.Sprintf("%dMi", *customKc.ContainerLogMaxSizeMB)
		}
		if customKc.ContainerLogMaxFiles != nil {
			kubeletConfig["--container-log-max-files"] = fmt.Sprintf("%d", *customKc.ContainerLogMaxFiles)
		}
	}
	return kubeletConfig
}

/*
GetOrderedKubeletConfigStringForPowershell returns an ordered string of key/val pairs for Powershell
script consumption.
*/
func (config *NodeBootstrappingConfiguration) GetOrderedKubeletConfigStringForPowershell(customKc *CustomKubeletConfig) string {
	kubeletConfig := config.KubeletConfig
	if kubeletConfig == nil {
		kubeletConfig = map[string]string{}
	}

	// override default kubelet configuration with customzied ones.
	if config.ContainerService != nil && config.ContainerService.Properties != nil {
		kubeletCustomConfiguration := config.ContainerService.Properties.GetComponentWindowsKubernetesConfiguration(Componentkubelet)
		if kubeletCustomConfiguration != nil {
			config := kubeletCustomConfiguration.Config
			for k, v := range config {
				kubeletConfig[k] = v
			}
		}
	}

	// Settings from customKubeletConfig, only take if it's set.
	kubeletConfig = setCustomKubletConfigFromSettings(customKc, kubeletConfig)

	if len(kubeletConfig) == 0 {
		return ""
	}

	commandLineOmmittedKubeletConfigFlags := GetCommandLineOmittedKubeletConfigFlags()
	keys := []string{}
	for key := range kubeletConfig {
		if !commandLineOmmittedKubeletConfigFlags[key] {
			keys = append(keys, key)
		}
	}

	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("\"%s=%s\", ", key, kubeletConfig[key]))
	}
	return strings.TrimSuffix(buf.String(), ", ")
}

/*
GetOrderedKubeproxyConfigStringForPowershell returns an ordered string of key/val pairs
for Powershell script consumption.
*/
func (config *NodeBootstrappingConfiguration) GetOrderedKubeproxyConfigStringForPowershell() string {
	kubeproxyConfig := config.KubeproxyConfig
	if kubeproxyConfig == nil {
		// https://kubernetes.io/docs/reference/command-line-tools-reference/kube-proxy/.
		// --metrics-bind-address ipport     Default: 127.0.0.1:10249.
		// The IP address with port for the metrics server to serve on
		// (set to '0.0.0.0:10249' for all IPv4 interfaces and '[::]:10249' for all IPv6 interfaces).
		// Set empty to disable.
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

// IsEnabled returns true if the addon is enabled.
func (a *KubernetesAddon) IsEnabled() bool {
	if a.Enabled == nil {
		return false
	}
	return *a.Enabled
}

// IsDisabled returns true if the addon is explicitly disabled.
func (a *KubernetesAddon) IsDisabled() bool {
	if a.Enabled == nil {
		return false
	}
	return !*a.Enabled
}

// GetAddonContainersIndexByName returns the KubernetesAddon containers index with the name `containerName`.
func (a KubernetesAddon) GetAddonContainersIndexByName(containerName string) int {
	for i := range a.Containers {
		if strings.EqualFold(a.Containers[i].Name, containerName) {
			return i
		}
	}
	return -1
}

// FormatProdFQDNByLocation constructs an Azure prod fqdn with custom cloud profile.
/* CustomCloudName is name of environment if customCloudProfile is provided, it will be empty string if
customCloudProfile is empty. Because customCloudProfile is empty for deployment for AzurePublicCloud,
AzureChinaCloud,AzureGermanCloud,AzureUSGovernmentCloud, The customCloudName value will be empty string
for those clouds. */
func FormatProdFQDNByLocation(fqdnPrefix string, location string, cloudSpecConfig *AzureEnvironmentSpecConfig) string {
	fqdnFormat := cloudSpecConfig.EndpointConfig.ResourceManagerVMDNSSuffix
	return fmt.Sprintf("%s.%s."+fqdnFormat, fqdnPrefix, location)
}

type K8sComponents struct {
	// Full path to the "pause" image. Used for --pod-infra-container-image.
	// For example: "mcr.microsoft.com/oss/kubernetes/pause:1.3.1".
	PodInfraContainerImageURL string

	// Full path to the hyperkube image.
	// For example: "mcr.microsoft.com/hyperkube-amd64:v1.16.13".
	HyperkubeImageURL string

	// Full path to the Windows package (windowszip) to use.
	// For example: https://acs-mirror.azureedge.net/kubernetes/v1.17.8/windowszip/v1.17.8-1int.zip.
	WindowsPackageURL string

	// Full path to the Linux package (tar.gz) to use.
	// For example: url=https://acs-mirror.azureedge.net/kubernetes/v1.25.6-hotfix.20230612/binaries/v1.25.6-hotfix.20230612.tar.gz
	LinuxPrivatePackageURL string

	// Full path to the Windows credential provider (tar.gz) to use.
	// For example: https://acs-mirror.azureedge.net/cloud-provider-azure/v1.29.4/binaries/azure-acr-credential-provider-windows-amd64-v1.29.4.tar.gz
	WindowsCredentialProviderURL string

	// Full path to the Linux credential provider (tar.gz) to use.
	// For example: "https://acs-mirror.azureedge.net/cloud-provider-azure/v1.29.4/binaries/azure-acr-credential-provider-linux-amd64-v1.29.4.tar.gz"
	LinuxCredentialProviderURL string
}

// GetLatestSigImageConfigRequest describes the input for a GetLatestSigImageConfig HTTP request.
// This is mostly a wrapper over existing types so RP doesn't have to manually construct JSON.
type GetLatestSigImageConfigRequest struct {
	SIGConfig      SIGConfig
	SubscriptionID string
	TenantID       string
	Region         string
	Distro         Distro
}

// NodeBootstrappingConfiguration represents configurations for node bootstrapping.
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
	EnableArtifactStreaming       bool
	ContainerdVersion             string
	RuncVersion                   string
	// ContainerdPackageURL and RuncPackageURL are beneficial for testing non-official.
	// containerd and runc, like the pre-released ones.
	// Currently both configurations are for test purpose, and only deb package is supported.
	ContainerdPackageURL string
	RuncPackageURL       string
	// KubeletClientTLSBootstrapToken - kubelet client TLS bootstrap token to use.
	/* When this feature is enabled, we skip kubelet kubeconfig generation and replace it with bootstrap
	kubeconfig. */
	// ref: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping.
	KubeletClientTLSBootstrapToken *string
	// EnableSecureTLSBootstraping - when this feature is enabled we don't hard-code TLS bootstrap tokens at all,
	// instead we create a modified bootstrap kubeconfig which points towards the STLS bootstrap client-go
	// credential plugin installed on the VHD, which will be responsible for generating TLS bootstrap tokens on the fly
	EnableSecureTLSBootstrapping bool
	// CustomSecureTLSBootstrapAADServerAppID serves as an optional override of the AAD server application ID
	// used by the secure TLS bootstrap client-go credential plugin when requesting JWTs from AAD
	CustomSecureTLSBootstrapAADServerAppID string
	FIPSEnabled                            bool
	HTTPProxyConfig                        *HTTPProxyConfig
	KubeletConfig                          map[string]string
	KubeproxyConfig                        map[string]string
	EnableRuncShimV2                       bool
	GPUInstanceProfile                     string
	PrimaryScaleSetName                    string
	SIGConfig                              SIGConfig
	IsARM64                                bool
	CustomCATrustConfig                    *CustomCATrustConfig
	DisableUnattendedUpgrades              bool
	SSHStatus                              SSHStatus
	DisableCustomData                      bool
	OutboundType                           string
	EnableIMDSRestriction                  bool
	// InsertIMDSRestrictionRuleToMangleTable is only checked when EnableIMDSRestriction is true.
	// When this is true, iptables rule will be inserted to `mangle` table. This is for Linux Cilium
	// CNI, which will overwrite the `filter` table so that we can only insert to `mangle` table to avoid
	// our added rule is overwritten by Cilium.
	InsertIMDSRestrictionRuleToMangleTable bool
}

type SSHStatus int

const (
	SSHUnspecified SSHStatus = iota
	SSHOff
	SSHOn
)

// NodeBootstrapping represents the custom data, CSE, and OS image info needed for node bootstrapping.
type NodeBootstrapping struct {
	CustomData     string
	CSE            string
	OSImageConfig  *AzureOSImageConfig
	SigImageConfig *SigImageConfig
}

// HTTPProxyConfig represents configurations of http proxy.
type HTTPProxyConfig struct {
	HTTPProxy  *string   `json:"httpProxy,omitempty"`
	HTTPSProxy *string   `json:"httpsProxy,omitempty"`
	NoProxy    *[]string `json:"noProxy,omitempty"`
	TrustedCA  *string   `json:"trustedCa,omitempty"`
}

type CustomCATrustConfig struct {
	CustomCATrustCerts []string `json:"customCATrustCerts,omitempty"`
}

// AKSKubeletConfiguration contains the configuration for the Kubelet that AKS set.
/* this is a subset of KubeletConfiguration defined in
https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubelet/config/v1beta1/types.go
changed metav1.Duration to Duration and pointers to values to simplify translation. */
type AKSKubeletConfiguration struct {
	// Kind is a string value representing the REST resource this object represents.
	// Servers may infer this from the endpoint the client submits requests to.
	// Cannot be updated.
	// In CamelCase.
	// More info:
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds.
	// +optional.
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	/* APIVersion defines the versioned schema of this representation of an object.
	Servers should convert recognized schemas to the latest internal value, and
	may reject unrecognized values.
	More info:
	https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
	+optional. */
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,2,opt,name=apiVersion"`
	/* staticPodPath is the path to the directory containing local (static) pods to
	run, or the path to a single static pod file.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	the set of static pods specified at the new path may be different than the
	ones the Kubelet initially started with, and this may disrupt your node.
	Default: ""
	+optional. */
	StaticPodPath string `json:"staticPodPath,omitempty"`
	/* address is the IP address for the Kubelet to serve on (set to 0.0.0.0
	for all interfaces).
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may disrupt components that interact with the Kubelet server.
	Default: "0.0.0.0"
	+optional. */
	Address string `json:"address,omitempty"`
	/* readOnlyPort is the read-only port for the Kubelet to serve on with
	no authentication/authorization.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may disrupt components that interact with the Kubelet server.
	Default: 0 (disabled)
	+optional. */
	ReadOnlyPort int32 `json:"readOnlyPort,omitempty"`
	/* tlsCertFile is the file containing x509 Certificate for HTTPS. (CA cert,
	if any, concatenated after server cert). If tlsCertFile and
	tlsPrivateKeyFile are not provided, a self-signed certificate
	and key are generated for the public address and saved to the directory
	passed to the Kubelet's --cert-dir flag.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may disrupt components that interact with the Kubelet server.
	Default: ""
	+optional. */
	TLSCertFile string `json:"tlsCertFile,omitempty"`
	/* tlsPrivateKeyFile is the file containing x509 private key matching tlsCertFile
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may disrupt components that interact with the Kubelet server.
	Default: ""
	+optional. */
	TLSPrivateKeyFile string `json:"tlsPrivateKeyFile,omitempty"`
	/* TLSCipherSuites is the list of allowed cipher suites for the server.
	Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may disrupt components that interact with the Kubelet server.
	Default: nil
	+optional. */
	TLSCipherSuites []string `json:"tlsCipherSuites,omitempty"`
	/* rotateCertificates enables client certificate rotation. The Kubelet will request a
	new certificate from the certificates.k8s.io API. This requires an approver to approve the
	certificate signing requests.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	disabling it may disrupt the Kubelet's ability to authenticate with the API server
	after the current certificate expires.
	Default: false
	+optional. */
	RotateCertificates bool `json:"rotateCertificates,omitempty"`
	// serverTLSBootstrap enables server certificate bootstrap. Instead of self
	// signing a serving certificate, the Kubelet will request a certificate from
	// the 'certificates.k8s.io' API. This requires an approver to approve the
	// certificate signing requests (CSR). The RotateKubeletServerCertificate feature
	// must be enabled when setting this field.
	// Default: false
	// +optional
	ServerTLSBootstrap bool `json:"serverTLSBootstrap,omitempty"`
	/* authentication specifies how requests to the Kubelet's server are authenticated
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may disrupt components that interact with the Kubelet server.
	Defaults:
	  anonymous:
	    enabled: false
	  webhook:
	    enabled: true
	    cacheTTL: "2m"
	+optional. */
	Authentication KubeletAuthentication `json:"authentication"`
	/* authorization specifies how requests to the Kubelet's server are authorized
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may disrupt components that interact with the Kubelet server.
	Defaults:
	  mode: Webhook
	  webhook:
	    cacheAuthorizedTTL: "5m"
	    cacheUnauthorizedTTL: "30s"
	+optional. */
	Authorization KubeletAuthorization `json:"authorization"`
	/* eventRecordQPS is the maximum event creations per second. If 0, there
	is no limit enforced.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may impact scalability by changing the amount of traffic produced by
	event creations.
	Default: 5
	+optional. */
	EventRecordQPS *int32 `json:"eventRecordQPS,omitempty"`
	/* clusterDomain is the DNS domain for this cluster. If set, kubelet will
	configure all containers to search this domain in addition to the
	host's search domains.
	Dynamic Kubelet Config (beta): Dynamically updating this field is not recommended,
	as it should be kept in sync with the rest of the cluster.
	Default: ""
	+optional. */
	ClusterDomain string `json:"clusterDomain,omitempty"`
	/* clusterDNS is a list of IP addresses for the cluster DNS server. If set,
	kubelet will configure all containers to use this for DNS resolution
	instead of the host's DNS servers.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	changes will only take effect on Pods created after the update. Draining
	the node is recommended before changing this field.
	Default: nil
	+optional. */
	ClusterDNS []string `json:"clusterDNS,omitempty"`
	/* streamingConnectionIdleTimeout is the maximum time a streaming connection
	can be idle before the connection is automatically closed.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may impact components that rely on infrequent updates over streaming
	connections to the Kubelet server.
	Default: "4h"
	+optional. */
	StreamingConnectionIdleTimeout Duration `json:"streamingConnectionIdleTimeout,omitempty"`
	/* nodeStatusUpdateFrequency is the frequency that kubelet computes node
	status. If node lease feature is not enabled, it is also the frequency that
	kubelet posts node status to master.
	Note: When node lease feature is not enabled, be cautious when changing the
	constant, it must work with nodeMonitorGracePeriod in nodecontroller.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may impact node scalability, and also that the node controller's
	nodeMonitorGracePeriod must be set to N*NodeStatusUpdateFrequency,
	where N is the number of retries before the node controller marks
	the node unhealthy.
	Default: "10s"
	+optional. */
	NodeStatusUpdateFrequency Duration `json:"nodeStatusUpdateFrequency,omitempty"`
	/* nodeStatusReportFrequency is the frequency that kubelet posts node
	status to master if node status does not change. Kubelet will ignore this
	frequency and post node status immediately if any change is detected. It is
	only used when node lease feature is enabled. nodeStatusReportFrequency's
	default value is 5m. But if nodeStatusUpdateFrequency is set explicitly,
	nodeStatusReportFrequency's default value will be set to
	nodeStatusUpdateFrequency for backward compatibility.
	Default: "5m"
	+optional. */
	NodeStatusReportFrequency Duration `json:"nodeStatusReportFrequency,omitempty"`
	/* imageGCHighThresholdPercent is the percent of disk usage after which
	image garbage collection is always run. The percent is calculated as
	this field value out of 100.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may trigger or delay garbage collection, and may change the image overhead
	on the node.
	Default: 85
	+optional. */
	ImageGCHighThresholdPercent *int32 `json:"imageGCHighThresholdPercent,omitempty"`
	/* imageGCLowThresholdPercent is the percent of disk usage before which
	image garbage collection is never run. Lowest disk usage to garbage
	collect to. The percent is calculated as this field value out of 100.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may trigger or delay garbage collection, and may change the image overhead
	on the node.
	Default: 80
	+optional. */
	ImageGCLowThresholdPercent *int32 `json:"imageGCLowThresholdPercent,omitempty"`
	/* Enable QoS based Cgroup hierarchy: top level cgroups for QoS Classes
	And all Burstable and BestEffort pods are brought up under their
	specific top level QoS cgroup.
	Dynamic Kubelet Config (beta): This field should not be updated without a full node
	reboot. It is safest to keep this value the same as the local config.
	Default: true
	+optional. */
	CgroupsPerQOS *bool `json:"cgroupsPerQOS,omitempty"`
	/* CPUManagerPolicy is the name of the policy to use.
	Requires the CPUManager feature gate to be enabled.
	Dynamic Kubelet Config (beta): This field should not be updated without a full node
	reboot. It is safest to keep this value the same as the local config.
	Default: "none"
	+optional. */
	CPUManagerPolicy string `json:"cpuManagerPolicy,omitempty"`
	/* TopologyManagerPolicy is the name of the policy to use.
	Policies other than "none" require the TopologyManager feature gate to be enabled.
	Dynamic Kubelet Config (beta): This field should not be updated without a full node
	reboot. It is safest to keep this value the same as the local config.
	Default: "none"
	+optional. */
	TopologyManagerPolicy string `json:"topologyManagerPolicy,omitempty"`
	/* maxPods is the number of pods that can run on this Kubelet.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	changes may cause Pods to fail admission on Kubelet restart, and may change
	the value reported in Node.Status.Capacity[v1.ResourcePods], thus affecting
	future scheduling decisions. Increasing this value may also decrease performance,
	as more Pods can be packed into a single node.
	Default: 110
	+optional. */
	MaxPods int32 `json:"maxPods,omitempty"`
	/* PodPidsLimit is the maximum number of pids in any pod.
	Requires the SupportPodPidsLimit feature gate to be enabled.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	lowering it may prevent container processes from forking after the change.
	Default: -1
	+optional. */
	PodPidsLimit *int64 `json:"podPidsLimit,omitempty"`
	/* ResolverConfig is the resolver configuration file used as the basis
	for the container DNS resolution configuration.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	changes will only take effect on Pods created after the update. Draining
	the node is recommended before changing this field.
	Default: "/etc/resolv.conf"
	+optional. */
	ResolverConfig string `json:"resolvConf,omitempty"`
	/* cpuCFSQuota enables CPU CFS quota enforcement for containers that
	specify CPU limits.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	disabling it may reduce node stability.
	Default: true
	+optional. */
	CPUCFSQuota *bool `json:"cpuCFSQuota,omitempty"`
	/* CPUCFSQuotaPeriod is the CPU CFS quota period value, cpu.cfs_period_us.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	limits set for containers will result in different cpu.cfs_quota settings. This
	will trigger container restarts on the node being reconfigured.
	Default: "100ms"
	+optional. */
	CPUCFSQuotaPeriod Duration `json:"cpuCFSQuotaPeriod,omitempty"`
	/* Map of signal names to quantities that defines hard eviction thresholds. For example: {"memory.available": "300Mi"}.
	To explicitly disable, pass a 0% or 100% threshold on an arbitrary resource.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may trigger or delay Pod evictions.
	Default:
	  memory.available:  "100Mi"
	  nodefs.available:  "10%"
	  nodefs.inodesFree: "5%"
	  imagefs.available: "15%"
	+optional. */
	EvictionHard map[string]string `json:"evictionHard,omitempty"`
	/* protectKernelDefaults, if true, causes the Kubelet to error if kernel
	flags are not as it expects. Otherwise the Kubelet will attempt to modify
	kernel flags to match its expectation.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	enabling it may cause the Kubelet to crash-loop if the Kernel is not configured as
	Kubelet expects.
	Default: false
	+optional. */
	ProtectKernelDefaults bool `json:"protectKernelDefaults,omitempty"`
	/* featureGates is a map of feature names to bools that enable or disable alpha/experimental
	features. This field modifies piecemeal the built-in default values from
	"k8s.io/kubernetes/pkg/features/kube_features.go".
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider the
	documentation for the features you are enabling or disabling. While we
	encourage feature developers to make it possible to dynamically enable
	and disable features, some changes may require node reboots, and some
	features may require careful coordination to retroactively disable.
	Default: nil
	+optional. */
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
	/* failSwapOn tells the Kubelet to fail to start if swap is enabled on the node.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	setting it to true will cause the Kubelet to crash-loop if swap is enabled.
	Default: true
	+optional. */
	FailSwapOn *bool `json:"failSwapOn,omitempty"`
	/* A quantity defines the maximum size of the container log file before it is rotated.
	For example: "5Mi" or "256Ki".
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may trigger log rotation.
	Default: "10Mi"
	+optional. */
	ContainerLogMaxSize string `json:"containerLogMaxSize,omitempty"`
	/* Maximum number of container log files that can be present for a container.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	lowering it may cause log files to be deleted.
	Default: 5
	+optional. */
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty"`

	/* the following fields are meant for Node Allocatable */

	/* systemReserved is a set of ResourceName=ResourceQuantity (e.g. cpu=200m,memory=150G)
	pairs that describe resources reserved for non-kubernetes components.
	Currently only cpu and memory are supported.
	See http://kubernetes.io/docs/user-guide/compute-resources for more detail.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may not be possible to increase the reserved resources, because this
	requires resizing cgroups. Always look for a NodeAllocatableEnforced event
	after updating this field to ensure that the update was successful.
	Default: nil
	+optional. */
	SystemReserved map[string]string `json:"systemReserved,omitempty"`
	/* A set of ResourceName=ResourceQuantity (e.g. cpu=200m,memory=150G) pairs
	that describe resources reserved for kubernetes system components.
	Currently cpu, memory and local storage for root file system are supported.
	See http://kubernetes.io/docs/user-guide/compute-resources for more detail.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	it may not be possible to increase the reserved resources, because this
	requires resizing cgroups. Always look for a NodeAllocatableEnforced event
	after updating this field to ensure that the update was successful.
	Default: nil
	+optional. */
	KubeReserved map[string]string `json:"kubeReserved,omitempty"`
	/* This flag specifies the various Node Allocatable enforcements that Kubelet needs to perform.
	This flag accepts a list of options. Acceptable options are `none`, `pods`, `system-reserved` &
	`kube-reserved`. If `none` is specified, no other options may be specified.
	Refer to
	[Node Allocatable](https://git.k8s.io/community/contributors/design-proposals/node/node-allocatable.md)
	doc for more information.
	Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	removing enforcements may reduce the stability of the node. Alternatively, adding
	enforcements may reduce the stability of components which were using more than
	the reserved amount of resources; for example, enforcing kube-reserved may cause
	Kubelets to OOM if it uses more than the reserved resources, and enforcing system-reserved
	may cause system daemons to OOM if they use more than the reserved resources.
	Default: ["pods"]
	+optional. */
	EnforceNodeAllocatable []string `json:"enforceNodeAllocatable,omitempty"`
	/* A comma separated whitelist of unsafe sysctls or sysctl patterns (ending in *).
	Unsafe sysctl groups are kernel.shm*, kernel.msg*, kernel.sem, fs.mqueue.*, and net.*.
	These sysctls are namespaced but not allowed by default.
	For example: "kernel.msg*,net.ipv4.route.min_pmtu"
	Default: []
	+optional. */
	AllowedUnsafeSysctls []string `json:"allowedUnsafeSysctls,omitempty"`
	// serializeImagePulls when enabled, tells the Kubelet to pull images one
	// at a time. We recommend *not* changing the default value on nodes that
	// run docker daemon with version  < 1.9 or an Aufs storage backend.
	// Issue #10959 has more details.
	// Default: true
	// +optional
	SerializeImagePulls *bool `json:"serializeImagePulls,omitempty"`
	// SeccompDefault enables the use of `RuntimeDefault` as the default seccomp profile for all workloads.
	// Default: false
	// +optional
	SeccompDefault *bool `json:"seccompDefault,omitempty"`
}

type Duration string

// below are copied from Kubernetes.
type KubeletAuthentication struct {
	// x509 contains settings related to x509 client certificate authentication.
	// +optional.
	X509 KubeletX509Authentication `json:"x509"`
	// webhook contains settings related to webhook bearer token authentication.
	// +optional.
	Webhook KubeletWebhookAuthentication `json:"webhook"`
	// anonymous contains settings related to anonymous authentication.
	// +optional.
	Anonymous KubeletAnonymousAuthentication `json:"anonymous"`
}

type KubeletX509Authentication struct {
	/* clientCAFile is the path to a PEM-encoded certificate bundle. If set, any request presenting a client certificate
	signed by one of the authorities in the bundle is authenticated with a username corresponding to the CommonName,
	and groups corresponding to the Organization in the client certificate.
	+optional. */
	ClientCAFile string `json:"clientCAFile,omitempty"`
}

type KubeletWebhookAuthentication struct {
	// enabled allows bearer token authentication backed by the tokenreviews.authentication.k8s.io API.
	// +optional.
	Enabled bool `json:"enabled,omitempty"`
	// cacheTTL enables caching of authentication results.
	// +optional.
	CacheTTL Duration `json:"cacheTTL,omitempty"`
}

type KubeletAnonymousAuthentication struct {
	// enabled allows anonymous requests to the kubelet server.
	// Requests that are not rejected by another authentication method are treated as anonymous requests.
	// Anonymous requests have a username of system:anonymous, and a group name of system:unauthenticated.
	// +optional.
	Enabled bool `json:"enabled,omitempty"`
}

type KubeletAuthorization struct {
	// mode is the authorization mode to apply to requests to the kubelet server.
	// Valid values are AlwaysAllow and Webhook.
	// Webhook mode uses the SubjectAccessReview API to determine authorization.
	// +optional.
	Mode KubeletAuthorizationMode `json:"mode,omitempty"`

	// webhook contains settings related to Webhook authorization.
	// +optional.
	Webhook KubeletWebhookAuthorization `json:"webhook"`
}

type KubeletAuthorizationMode string

type KubeletWebhookAuthorization struct {
	// cacheAuthorizedTTL is the duration to cache 'authorized' responses from the webhook authorizer.
	// +optional.
	CacheAuthorizedTTL Duration `json:"cacheAuthorizedTTL,omitempty"`
	// cacheUnauthorizedTTL is the duration to cache 'unauthorized' responses from the webhook authorizer.
	// +optional.
	CacheUnauthorizedTTL Duration `json:"cacheUnauthorizedTTL,omitempty"`
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
	// KernelStartTime of current boot, output from systemctl show -p KernelTimestamp.
	KernelStartTime string `json:"kernelStartTime,omitempty"`
	// SystemdSummary of current boot, output from systemd-analyze.
	SystemdSummary string `json:"systemdSummary,omitempty"`
	// CSEStartTime indicate starttime of CSE.
	CSEStartTime string `json:"cseStartTime,omitempty"`
	/* GuestAgentStartTime indicate starttime of GuestAgent, output from systemctl show
	walinuxagent.service -p ExecMainStartTimestamp */
	GuestAgentStartTime string `json:"guestAgentStartTime,omitempty"`
	// BootDatapoints contains datapoints (key-value pair) from VM boot process.
	BootDatapoints map[string]string `json:"bootDatapoints,omitempty"`
}

type CSEStatusParsingErrorCode string

const (
	// CSEMessageUnmarshalError is the error code for unmarshal cse message.
	CSEMessageUnmarshalError CSEStatusParsingErrorCode = "CSEMessageUnmarshalError"
	// CSEMessageExitCodeEmptyError is the error code for empty cse message exit code.
	CSEMessageExitCodeEmptyError CSEStatusParsingErrorCode = "CSEMessageExitCodeEmptyError"
	// InvalidCSEMessage is the error code for cse invalid message.
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

type AgentPoolWindowsProfile struct {
	DisableOutboundNat *bool `json:"disableOutboundNat,omitempty"`

	// Windows next-gen networking uses Windows eBPF for the networking dataplane.
	NextGenNetworkingURL *string `json:"nextGenNetworkingURL,omitempty"`
}

// IsDisableWindowsOutboundNat returns true if the Windows agent pool disable OutboundNAT.
func (a *AgentPoolProfile) IsDisableWindowsOutboundNat() bool {
	return a.AgentPoolWindowsProfile != nil &&
		a.AgentPoolWindowsProfile.DisableOutboundNat != nil &&
		*a.AgentPoolWindowsProfile.DisableOutboundNat
}

func (a *AgentPoolWindowsProfile) IsNextGenNetworkingEnabled() bool {
	return a != nil && a.NextGenNetworkingURL != nil
}

func (a *AgentPoolWindowsProfile) GetNextGenNetworkingURL() string {
	if a == nil || a.NextGenNetworkingURL == nil {
		return ""
	}
	return *a.NextGenNetworkingURL
}

// SecurityProfile begin.
type SecurityProfile struct {
	PrivateEgress *PrivateEgress `json:"privateEgress,omitempty"`
}

type PrivateEgress struct {
	Enabled                 bool   `json:"enabled"`
	ContainerRegistryServer string `json:"containerRegistryServer"`
	ProxyAddress            string `json:"proxyAddress"`
}

func (s *SecurityProfile) GetProxyAddress() string {
	if s != nil && s.PrivateEgress != nil && s.PrivateEgress.Enabled {
		return s.PrivateEgress.ProxyAddress
	}
	return ""
}

func (s *SecurityProfile) GetPrivateEgressContainerRegistryServer() string {
	if s != nil && s.PrivateEgress != nil && s.PrivateEgress.Enabled {
		return s.PrivateEgress.ContainerRegistryServer
	}
	return ""
}

// SecurityProfile end.
