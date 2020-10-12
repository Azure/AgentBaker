// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math/rand"
	neturl "net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Azure/agentbaker/pkg/aks-engine/helpers"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/blang/semver"
)

// TypeMeta describes an individual API model object
type TypeMeta struct {
	// APIVersion is on every object
	APIVersion string `json:"apiVersion"`
}

// CustomNodesDNS represents the Search Domain when the custom vnet for a custom DNS as a nameserver.
type CustomNodesDNS struct {
	DNSServer string `json:"dnsServer,omitempty"`
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

// Distro represents Linux distro to use for Linux VMs
type Distro string

// Distro string consts
const (
	Ubuntu               Distro = "ubuntu"
	Ubuntu1804           Distro = "ubuntu-18.04"
	Ubuntu1804Gen2       Distro = "ubuntu-18.04-gen2"
	AKSUbuntu1604        Distro = "aks-ubuntu-16.04"
	AKSUbuntu1804        Distro = "aks-ubuntu-18.04"
	AKSUbuntuGPU1804     Distro = "aks-ubuntu-gpu-18.04"
	AKSUbuntuGPU1804Gen2 Distro = "aks-ubuntu-gpu-18.04-gen2"
)

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
	FQDN      string `json:"fqdn,omitempty"`
	DNSPrefix string `json:"dnsPrefix"`
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
	// CaPrivateKey is the certificate authority key.
	CaPrivateKey string `json:"caPrivateKey,omitempty" conform:"redact"`
	// ApiServerCertificate is the rest api server certificate, and signed by the CA
	APIServerCertificate string `json:"apiServerCertificate,omitempty" conform:"redact"`
	// ApiServerPrivateKey is the rest api server private key, and signed by the CA
	APIServerPrivateKey string `json:"apiServerPrivateKey,omitempty" conform:"redact"`
	// ClientCertificate is the certificate used by the client kubelet services and signed by the CA
	ClientCertificate string `json:"clientCertificate,omitempty" conform:"redact"`
	// ClientPrivateKey is the private key used by the client kubelet services and signed by the CA
	ClientPrivateKey string `json:"clientPrivateKey,omitempty" conform:"redact"`
	// KubeConfigCertificate is the client certificate used for kubectl cli and signed by the CA
	KubeConfigCertificate string `json:"kubeConfigCertificate,omitempty" conform:"redact"`
	// KubeConfigPrivateKey is the client private key used for kubectl cli and signed by the CA
	KubeConfigPrivateKey string `json:"kubeConfigPrivateKey,omitempty" conform:"redact"`
	// EtcdServerCertificate is the server certificate for etcd, and signed by the CA
	EtcdServerCertificate string `json:"etcdServerCertificate,omitempty" conform:"redact"`
	// EtcdServerPrivateKey is the server private key for etcd, and signed by the CA
	EtcdServerPrivateKey string `json:"etcdServerPrivateKey,omitempty" conform:"redact"`
	// EtcdClientCertificate is etcd client certificate, and signed by the CA
	EtcdClientCertificate string `json:"etcdClientCertificate,omitempty" conform:"redact"`
	// EtcdClientPrivateKey is the etcd client private key, and signed by the CA
	EtcdClientPrivateKey string `json:"etcdClientPrivateKey,omitempty" conform:"redact"`
	// EtcdPeerCertificates is list of etcd peer certificates, and signed by the CA
	EtcdPeerCertificates []string `json:"etcdPeerCertificates,omitempty" conform:"redact"`
	// EtcdPeerPrivateKeys is list of etcd peer private keys, and signed by the CA
	EtcdPeerPrivateKeys []string `json:"etcdPeerPrivateKeys,omitempty" conform:"redact"`
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
	AdminUsername                 string            `json:"adminUsername"`
	AdminPassword                 string            `json:"adminPassword" conform:"redact"`
	CSIProxyURL                   string            `json:"csiProxyURL,omitempty"`
	EnableCSIProxy                *bool             `json:"enableCSIProxy,omitempty"`
	ImageRef                      *ImageReference   `json:"imageReference,omitempty"`
	ImageVersion                  string            `json:"imageVersion"`
	ProvisioningScriptsPackageURL string            `json:"provisioningScriptsPackageURL,omitempty"`
	WindowsImageSourceURL         string            `json:"windowsImageSourceURL"`
	WindowsPublisher              string            `json:"windowsPublisher"`
	WindowsOffer                  string            `json:"windowsOffer"`
	WindowsSku                    string            `json:"windowsSku"`
	WindowsDockerVersion          string            `json:"windowsDockerVersion"`
	Secrets                       []KeyVaultSecrets `json:"secrets,omitempty"`
	SSHEnabled                    *bool             `json:"sshEnabled,omitempty"`
	EnableAutomaticUpdates        *bool             `json:"enableAutomaticUpdates,omitempty"`
	IsCredentialAutoGenerated     *bool             `json:"isCredentialAutoGenerated,omitempty"`
	EnableAHUB                    *bool             `json:"enableAHUB,omitempty"`
	WindowsPauseImageURL          string            `json:"windowsPauseImageURL"`
	AlwaysPullWindowsPauseImage   *bool             `json:"alwaysPullWindowsPauseImage,omitempty"`
}

// LinuxProfile represents the linux parameters passed to the cluster
type LinuxProfile struct {
	AdminUsername string `json:"adminUsername"`
	SSH           struct {
		PublicKeys []PublicKey `json:"publicKeys"`
	} `json:"ssh"`
	Secrets               []KeyVaultSecrets   `json:"secrets,omitempty"`
	Distro                Distro              `json:"distro,omitempty"`
	ScriptRootURL         string              `json:"scriptroot,omitempty"`
	CustomSearchDomain    *CustomSearchDomain `json:"customSearchDomain,omitempty"`
	CustomNodesDNS        *CustomNodesDNS     `json:"CustomNodesDNS,omitempty"`
	IsSSHKeyAutoGenerated *bool               `json:"isSSHKeyAutoGenerated,omitempty"`
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

// KubeProxyMode is for iptables and ipvs (and future others)
type KubeProxyMode string

// We currently support ipvs and iptables
const (
	// KubeProxyModeIPTables is used to set the kube-proxy to iptables mode
	KubeProxyModeIPTables KubeProxyMode = "iptables"
	// KubeProxyModeIPVS is used to set the kube-proxy to ipvs mode
	KubeProxyModeIPVS KubeProxyMode = "ipvs"
	// DefaultKubeProxyMode is the default KubeProxyMode value
	DefaultKubeProxyMode KubeProxyMode = KubeProxyModeIPTables
)

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
	CustomKubeAPIServerImage          string            `json:"customKubeAPIServerImage,omitempty"`
	CustomKubeControllerManagerImage  string            `json:"customKubeControllerManagerImage,omitempty"`
	CustomKubeProxyImage              string            `json:"customKubeProxyImage,omitempty"`
	CustomKubeSchedulerImage          string            `json:"customKubeSchedulerImage,omitempty"`
	CustomKubeBinaryURL               string            `json:"customKubeBinaryURL,omitempty"`
	DockerEngineVersion               string            `json:"dockerEngineVersion,omitempty"` // Deprecated
	MobyVersion                       string            `json:"mobyVersion,omitempty"`
	ContainerdVersion                 string            `json:"containerdVersion,omitempty"`
	CustomCcmImage                    string            `json:"customCcmImage,omitempty"` // Image for cloud-controller-manager
	UseCloudControllerManager         *bool             `json:"useCloudControllerManager,omitempty"`
	CustomWindowsPackageURL           string            `json:"customWindowsPackageURL,omitempty"`
	WindowsNodeBinariesURL            string            `json:"windowsNodeBinariesURL,omitempty"`
	WindowsContainerdURL              string            `json:"windowsContainerdURL,omitempty"`
	WindowsSdnPluginURL               string            `json:"windowsSdnPluginURL,omitempty"`
	UseInstanceMetadata               *bool             `json:"useInstanceMetadata,omitempty"`
	EnableRbac                        *bool             `json:"enableRbac,omitempty"`
	EnableSecureKubelet               *bool             `json:"enableSecureKubelet,omitempty"`
	EnableAggregatedAPIs              bool              `json:"enableAggregatedAPIs,omitempty"`
	PrivateCluster                    *PrivateCluster   `json:"privateCluster,omitempty"`
	GCHighThreshold                   int               `json:"gchighthreshold,omitempty"`
	GCLowThreshold                    int               `json:"gclowthreshold,omitempty"`
	EtcdVersion                       string            `json:"etcdVersion,omitempty"`
	EtcdDiskSizeGB                    string            `json:"etcdDiskSizeGB,omitempty"`
	EtcdEncryptionKey                 string            `json:"etcdEncryptionKey,omitempty"`
	EnableDataEncryptionAtRest        *bool             `json:"enableDataEncryptionAtRest,omitempty"`
	EnableEncryptionWithExternalKms   *bool             `json:"enableEncryptionWithExternalKms,omitempty"`
	EnablePodSecurityPolicy           *bool             `json:"enablePodSecurityPolicy,omitempty"`
	Addons                            []KubernetesAddon `json:"addons,omitempty"`
	KubeletConfig                     map[string]string `json:"kubeletConfig,omitempty"`
	ContainerRuntimeConfig            map[string]string `json:"containerRuntimeConfig,omitempty"`
	ControllerManagerConfig           map[string]string `json:"controllerManagerConfig,omitempty"`
	CloudControllerManagerConfig      map[string]string `json:"cloudControllerManagerConfig,omitempty"`
	APIServerConfig                   map[string]string `json:"apiServerConfig,omitempty"`
	SchedulerConfig                   map[string]string `json:"schedulerConfig,omitempty"`
	PodSecurityPolicyConfig           map[string]string `json:"podSecurityPolicyConfig,omitempty"` // Deprecated
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
	NonMasqueradeCidr                 string            `json:"nonMasqueradeCidr,omitempty"`
	NodeStatusUpdateFrequency         string            `json:"nodeStatusUpdateFrequency,omitempty"`
	HardEvictionThreshold             string            `json:"hardEvictionThreshold,omitempty"`
	CtrlMgrNodeMonitorGracePeriod     string            `json:"ctrlMgrNodeMonitorGracePeriod,omitempty"`
	CtrlMgrPodEvictionTimeout         string            `json:"ctrlMgrPodEvictionTimeout,omitempty"`
	CtrlMgrRouteReconciliationPeriod  string            `json:"ctrlMgrRouteReconciliationPeriod,omitempty"`
	LoadBalancerSku                   string            `json:"loadBalancerSku,omitempty"`
	ExcludeMasterFromStandardLB       *bool             `json:"excludeMasterFromStandardLB,omitempty"`
	AzureCNIVersion                   string            `json:"azureCNIVersion,omitempty"`
	AzureCNIURLLinux                  string            `json:"azureCNIURLLinux,omitempty"`
	AzureCNIURLWindows                string            `json:"azureCNIURLWindows,omitempty"`
	KeyVaultSku                       string            `json:"keyVaultSku,omitempty"`
	MaximumLoadBalancerRuleCount      int               `json:"maximumLoadBalancerRuleCount,omitempty"`
	ProxyMode                         KubeProxyMode     `json:"kubeProxyMode,omitempty"`
	PrivateAzureRegistryServer        string            `json:"privateAzureRegistryServer,omitempty"`
	OutboundRuleIdleTimeoutInMinutes  int32             `json:"outboundRuleIdleTimeoutInMinutes,omitempty"`
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

// AgentPoolProfileRole represents an agent role
type AgentPoolProfileRole string

// AgentPoolProfile represents an agent pool definition
type AgentPoolProfile struct {
	Name                                string               `json:"name"`
	Count                               int                  `json:"count"`
	VMSize                              string               `json:"vmSize"`
	OSDiskSizeGB                        int                  `json:"osDiskSizeGB,omitempty"`
	DNSPrefix                           string               `json:"dnsPrefix,omitempty"`
	OSType                              OSType               `json:"osType,omitempty"`
	Ports                               []int                `json:"ports,omitempty"`
	ProvisioningState                   ProvisioningState    `json:"provisioningState,omitempty"`
	AvailabilityProfile                 string               `json:"availabilityProfile"`
	ScaleSetPriority                    string               `json:"scaleSetPriority,omitempty"`
	ScaleSetEvictionPolicy              string               `json:"scaleSetEvictionPolicy,omitempty"`
	SpotMaxPrice                        *float64             `json:"spotMaxPrice,omitempty"`
	StorageProfile                      string               `json:"storageProfile,omitempty"`
	DiskSizesGB                         []int                `json:"diskSizesGB,omitempty"`
	VnetSubnetID                        string               `json:"vnetSubnetID,omitempty"`
	Subnet                              string               `json:"subnet"`
	IPAddressCount                      int                  `json:"ipAddressCount,omitempty"`
	Distro                              Distro               `json:"distro,omitempty"`
	Role                                AgentPoolProfileRole `json:"role,omitempty"`
	AcceleratedNetworkingEnabled        *bool                `json:"acceleratedNetworkingEnabled,omitempty"`
	AcceleratedNetworkingEnabledWindows *bool                `json:"acceleratedNetworkingEnabledWindows,omitempty"`
	VMSSOverProvisioningEnabled         *bool                `json:"vmssOverProvisioningEnabled,omitempty"`
	FQDN                                string               `json:"fqdn,omitempty"`
	CustomNodeLabels                    map[string]string    `json:"customNodeLabels,omitempty"`
	PreprovisionExtension               *Extension           `json:"preProvisionExtension"`
	Extensions                          []Extension          `json:"extensions"`
	KubernetesConfig                    *KubernetesConfig    `json:"kubernetesConfig,omitempty"`
	OrchestratorVersion                 string               `json:"orchestratorVersion"`
	ImageRef                            *ImageReference      `json:"imageReference,omitempty"`
	MaxCount                            *int                 `json:"maxCount,omitempty"`
	MinCount                            *int                 `json:"minCount,omitempty"`
	EnableAutoScaling                   *bool                `json:"enableAutoScaling,omitempty"`
	AvailabilityZones                   []string             `json:"availabilityZones,omitempty"`
	PlatformFaultDomainCount            *int                 `json:"platformFaultDomainCount"`
	PlatformUpdateDomainCount           *int                 `json:"platformUpdateDomainCount"`
	SinglePlacementGroup                *bool                `json:"singlePlacementGroup,omitempty"`
	VnetCidrs                           []string             `json:"vnetCidrs,omitempty"`
	PreserveNodesProperties             *bool                `json:"preserveNodesProperties,omitempty"`
	WindowsNameVersion                  string               `json:"windowsNameVersion,omitempty"`
	EnableVMSSNodePublicIP              *bool                `json:"enableVMSSNodePublicIP,omitempty"`
	LoadBalancerBackendAddressPoolIDs   []string             `json:"loadBalancerBackendAddressPoolIDs,omitempty"`
	AuditDEnabled                       *bool                `json:"auditDEnabled,omitempty"`
	CustomVMTags                        map[string]string    `json:"customVMTags,omitempty"`
	DiskEncryptionSetID                 string               `json:"diskEncryptionSetID,omitempty"`
	UltraSSDEnabled                     *bool                `json:"ultraSSDEnabled,omitempty"`
	EncryptionAtHost                    *bool                `json:"encryptionAtHost,omitempty"`
	ProximityPlacementGroupID           string               `json:"proximityPlacementGroupID,omitempty"`
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

// GetLocations returns all supported regions.
// If AzurePublicCloud, AzureChinaCloud,AzureGermanCloud or AzureUSGovernmentCloud, GetLocations provides all azure regions in prod.
func (cs *ContainerService) GetLocations() []string {
	return helpers.GetAzureLocations()
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

// TotalNodes returns the total number of nodes in the cluster configuration
func (p *Properties) TotalNodes() int {
	var totalNodes int
	for _, pool := range p.AgentPoolProfiles {
		totalNodes += pool.Count
	}
	return totalNodes
}

// HasAvailabilityZones returns true if the cluster contains a profile with zones
func (p *Properties) HasAvailabilityZones() bool {
	hasZones := false
	if p.AgentPoolProfiles != nil {
		for _, agentPoolProfile := range p.AgentPoolProfiles {
			if agentPoolProfile.HasAvailabilityZones() {
				hasZones = true
				break
			}
		}
	}
	return hasZones
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

// AnyAgentIsLinux checks whether any of the agents in the AgentPools are linux
func (p *Properties) AnyAgentIsLinux() bool {
	for _, agentProfile := range p.AgentPoolProfiles {
		if agentProfile.IsLinux() {
			return true
		}
	}
	return false
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
	return environmentJSON, nil
}

// HasNSeriesSKU returns whether or not there is an N series SKU agent pool
func (p *Properties) HasNSeriesSKU() bool {
	for _, profile := range p.AgentPoolProfiles {
		if strings.Contains(profile.VMSize, "Standard_N") {
			return true
		}
	}
	return false
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

// GetPrimaryScaleSetName returns the name of the primary scale set node of the cluster
func (p *Properties) GetPrimaryScaleSetName() string {
	if len(p.AgentPoolProfiles) > 0 {
		if strings.EqualFold(p.AgentPoolProfiles[0].AvailabilityProfile, VirtualMachineScaleSets) {
			return p.GetAgentVMPrefix(p.AgentPoolProfiles[0], 0)
		}
	}
	return ""
}

// GetAgentVMPrefix returns the VM prefix for an agentpool.
func (p *Properties) GetAgentVMPrefix(a *AgentPoolProfile, index int) string {
	nameSuffix := p.GetClusterID()
	vmPrefix := ""
	if index != -1 {
		if a.IsWindows() {
			if strings.EqualFold(a.WindowsNameVersion, "v2") {
				vmPrefix = p.K8sOrchestratorName() + a.Name
			} else {
				vmPrefix = nameSuffix[:4] + p.K8sOrchestratorName() + fmt.Sprintf("%02d", index)
			}
		} else {
			vmPrefix = p.K8sOrchestratorName() + "-" + a.Name + "-" + nameSuffix + "-"
			if a.IsVirtualMachineScaleSets() {
				vmPrefix += "vmss"
			}
		}
	}
	return vmPrefix
}

// IsVHDDistro returns true if the distro uses VHD SKUs
func (a *AgentPoolProfile) IsVHDDistro() bool {
	return strings.EqualFold(string(a.Distro), string(AKSUbuntu1604)) || strings.EqualFold(string(a.Distro), string(AKSUbuntu1804)) || strings.EqualFold(string(a.Distro), string(Ubuntu1804Gen2)) || strings.EqualFold(string(a.Distro), string(AKSUbuntuGPU1804)) || strings.EqualFold(string(a.Distro), string(AKSUbuntuGPU1804Gen2))
}

// IsUbuntu1804 returns true if the agent pool profile distro is based on Ubuntu 16.04
func (a *AgentPoolProfile) IsUbuntu1804() bool {
	if !strings.EqualFold(string(a.OSType), string(Windows)) {
		switch a.Distro {
		case AKSUbuntu1804, Ubuntu1804, Ubuntu1804Gen2, AKSUbuntuGPU1804, AKSUbuntuGPU1804Gen2:
			return true
		default:
			return false
		}
	}
	return false
}

// HasAvailabilityZones returns true if the agent pool has availability zones
func (a *AgentPoolProfile) HasAvailabilityZones() bool {
	return a.AvailabilityZones != nil && len(a.AvailabilityZones) > 0
}

// IsLinux returns true if the agent pool is linux
func (a *AgentPoolProfile) IsLinux() bool {
	return strings.EqualFold(string(a.OSType), string(Linux))
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

// IsSpotScaleSet returns true if the VMSS is Spot Scale Set
func (a *AgentPoolProfile) IsSpotScaleSet() bool {
	return strings.EqualFold(a.AvailabilityProfile, VirtualMachineScaleSets) && strings.EqualFold(a.ScaleSetPriority, ScaleSetPrioritySpot)
}

// GetKubernetesLabels returns a k8s API-compliant labels string for nodes in this profile
func (a *AgentPoolProfile) GetKubernetesLabels(rg string, deprecated bool, nvidiaEnabled bool) string {
	var buf bytes.Buffer
	buf.WriteString("kubernetes.azure.com/role=agent")
	if deprecated {
		buf.WriteString(",node-role.kubernetes.io/agent=")
		buf.WriteString(",kubernetes.io/role=agent")
	}
	buf.WriteString(fmt.Sprintf(",agentpool=%s", a.Name))
	if strings.EqualFold(a.StorageProfile, ManagedDisks) {
		storagetier, _ := GetStorageAccountType(a.VMSize)
		buf.WriteString(fmt.Sprintf(",storageprofile=managed,storagetier=%s", storagetier))
	}
	if nvidiaEnabled {
		accelerator := "nvidia"
		buf.WriteString(fmt.Sprintf(",accelerator=%s", accelerator))
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

// HasDisks returns true if the customer specified disks
func (a *AgentPoolProfile) HasDisks() bool {
	return len(a.DiskSizesGB) > 0
}

// IsAuditDEnabled returns true if the master profile is configured for auditd
func (a *AgentPoolProfile) IsAuditDEnabled() bool {
	return to.Bool(a.AuditDEnabled)
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

// GetAPIServerEtcdAPIVersion Used to set apiserver's etcdapi version
func (o *OrchestratorProfile) GetAPIServerEtcdAPIVersion() string {
	if o.KubernetesConfig != nil {
		// if we are here, version has already been validated..
		etcdVersion, _ := semver.Make(o.KubernetesConfig.EtcdVersion)
		return "etcd" + strconv.FormatUint(etcdVersion.Major, 10)
	}
	return ""
}

// IsAzureCNI returns true if Azure CNI network plugin is enabled
func (o *OrchestratorProfile) IsAzureCNI() bool {
	if o.KubernetesConfig != nil {
		return strings.EqualFold(o.KubernetesConfig.NetworkPlugin, NetworkPluginAzure)
	}
	return false
}

// HasCustomNodesDNS returns true if the customer specified a dns server
func (l *LinuxProfile) HasCustomNodesDNS() bool {
	if l.CustomNodesDNS != nil {
		if l.CustomNodesDNS.DNSServer != "" {
			return true
		}
	}
	return false
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

// IsKubernetes returns true if this template is for Kubernetes orchestrator
func (o *OrchestratorProfile) IsKubernetes() bool {
	return strings.EqualFold(o.OrchestratorType, Kubernetes)
}

// IsPrivateCluster returns true if this deployment is a private cluster
func (o *OrchestratorProfile) IsPrivateCluster() bool {
	if !o.IsKubernetes() {
		return false
	}
	return o.KubernetesConfig != nil && o.KubernetesConfig.PrivateCluster != nil && to.Bool(o.KubernetesConfig.PrivateCluster.Enabled)
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

// GetAzureCNIURLWindows returns the full URL to source Azure CNI binaries from
func (k *KubernetesConfig) GetAzureCNIURLWindows(cloudSpecConfig *AzureEnvironmentSpecConfig) string {
	if k.AzureCNIURLWindows != "" {
		return k.AzureCNIURLWindows
	}
	return cloudSpecConfig.KubernetesSpecConfig.VnetCNIWindowsPluginsDownloadURL
}

// GetOrderedKubeletConfigStringForPowershell returns an ordered string of key/val pairs for Powershell script consumption
func (k *KubernetesConfig) GetOrderedKubeletConfigStringForPowershell() string {
	keys := []string{}
	for key := range k.KubeletConfig {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("\"%s=%s\", ", key, k.KubeletConfig[key]))
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
