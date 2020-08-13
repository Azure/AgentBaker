// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"bytes"
	"fmt"
	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/aks-engine/pkg/helpers"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/blang/semver"
	"hash/fnv"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// CustomCloudEnv represents the custom cloud env info of the AKS cluster.
type CustomCloudEnv struct {
	Name                         string                  `json:"Name,omitempty"`
	McrURL                       string                  `json:"mcrURL,omitempty"`
	RepoDepotEndpoint            string                  `json:"repoDepotEndpoint,omitempty"`
	ManagementPortalURL          string                  `json:"managementPortalURL,omitempty"`
	PublishSettingsURL           string                  `json:"publishSettingsURL,omitempty"`
	ServiceManagementEndpoint    string                  `json:"serviceManagementEndpoint,omitempty"`
	ResourceManagerEndpoint      string                  `json:"resourceManagerEndpoint,omitempty"`
	ActiveDirectoryEndpoint      string                  `json:"activeDirectoryEndpoint,omitempty"`
	GalleryEndpoint              string                  `json:"galleryEndpoint,omitempty"`
	KeyVaultEndpoint             string                  `json:"keyVaultEndpoint,omitempty"`
	GraphEndpoint                string                  `json:"graphEndpoint,omitempty"`
	ServiceBusEndpoint           string                  `json:"serviceBusEndpoint,omitempty"`
	BatchManagementEndpoint      string                  `json:"batchManagementEndpoint,omitempty"`
	StorageEndpointSuffix        string                  `json:"storageEndpointSuffix,omitempty"`
	SQLDatabaseDNSSuffix         string                  `json:"sqlDatabaseDNSSuffix,omitempty"`
	TrafficManagerDNSSuffix      string                  `json:"trafficManagerDNSSuffix,omitempty"`
	KeyVaultDNSSuffix            string                  `json:"keyVaultDNSSuffix,omitempty"`
	ServiceBusEndpointSuffix     string                  `json:"serviceBusEndpointSuffix,omitempty"`
	ServiceManagementVMDNSSuffix string                  `json:"serviceManagementVMDNSSuffix,omitempty"`
	ResourceManagerVMDNSSuffix   string                  `json:"resourceManagerVMDNSSuffix,omitempty"`
	ContainerRegistryDNSSuffix   string                  `json:"containerRegistryDNSSuffix,omitempty"`
	CosmosDBDNSSuffix            string                  `json:"cosmosDBDNSSuffix,omitempty"`
	TokenAudience                string                  `json:"tokenAudience,omitempty"`
	ResourceIdentifiers          api.ResourceIdentifiers `json:"resourceIdentifiers,omitempty"`
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
	Identity *api.UserAssignedIdentity `json:"identity,omitempty"`
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
	Authenticator api.AuthenticatorType `json:"authenticator"`
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
	ClientID          string                 `json:"clientId"`
	Secret            string                 `json:"secret,omitempty" conform:"redact"`
	ObjectID          string                 `json:"objectId,omitempty"`
	KeyvaultSecretRef *api.KeyvaultSecretRef `json:"keyvaultSecretRef,omitempty"`
}

// JumpboxProfile describes properties of the jumpbox setup
// in the AKS container cluster.
type JumpboxProfile struct {
	OSType    api.OSType `json:"osType"`
	DNSPrefix string     `json:"dnsPrefix"`

	// Jumpbox public endpoint/FQDN with port
	// The format will be FQDN:2376
	// Not used during PUT, returned as part of GET
	FQDN string `json:"fqdn,omitempty"`
}

// DiagnosticsProfile setting to enable/disable capturing
// diagnostics for VMs hosting container cluster.
type DiagnosticsProfile struct {
	VMDiagnostics *api.VMDiagnostics `json:"vmDiagnostics"`
}

// ExtensionProfile represents an extension definition
type ExtensionProfile struct {
	Name                           string                 `json:"name"`
	Version                        string                 `json:"version"`
	ExtensionParameters            string                 `json:"extensionParameters,omitempty"`
	ExtensionParametersKeyVaultRef *api.KeyvaultSecretRef `json:"parametersKeyvaultSecretRef,omitempty"`
	RootURL                        string                 `json:"rootURL,omitempty"`
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
	AdminUsername                 string                `json:"adminUsername"`
	AdminPassword                 string                `json:"adminPassword" conform:"redact"`
	CSIProxyURL                   string                `json:"csiProxyURL,omitempty"`
	EnableCSIProxy                *bool                 `json:"enableCSIProxy,omitempty"`
	ImageRef                      *api.ImageReference   `json:"imageReference,omitempty"`
	ImageVersion                  string                `json:"imageVersion"`
	ProvisioningScriptsPackageURL string                `json:"provisioningScriptsPackageURL,omitempty"`
	WindowsImageSourceURL         string                `json:"windowsImageSourceURL"`
	WindowsPublisher              string                `json:"windowsPublisher"`
	WindowsOffer                  string                `json:"windowsOffer"`
	WindowsSku                    string                `json:"windowsSku"`
	WindowsDockerVersion          string                `json:"windowsDockerVersion"`
	Secrets                       []api.KeyVaultSecrets `json:"secrets,omitempty"`
	SSHEnabled                    *bool                 `json:"sshEnabled,omitempty"`
	EnableAutomaticUpdates        *bool                 `json:"enableAutomaticUpdates,omitempty"`
	IsCredentialAutoGenerated     *bool                 `json:"isCredentialAutoGenerated,omitempty"`
	EnableAHUB                    *bool                 `json:"enableAHUB,omitempty"`
	WindowsPauseImageURL          string                `json:"windowsPauseImageURL"`
	AlwaysPullWindowsPauseImage   *bool                 `json:"alwaysPullWindowsPauseImage,omitempty"`
}

// LinuxProfile represents the linux parameters passed to the cluster
type LinuxProfile struct {
	AdminUsername string `json:"adminUsername"`
	SSH           struct {
		PublicKeys []api.PublicKey `json:"publicKeys"`
	} `json:"ssh"`
	Secrets               []api.KeyVaultSecrets   `json:"secrets,omitempty"`
	Distro                api.Distro              `json:"distro,omitempty"`
	ScriptRootURL         string                  `json:"scriptroot,omitempty"`
	CustomSearchDomain    *api.CustomSearchDomain `json:"customSearchDomain,omitempty"`
	CustomNodesDNS        *api.CustomNodesDNS     `json:"CustomNodesDNS,omitempty"`
	IsSSHKeyAutoGenerated *bool                   `json:"isSSHKeyAutoGenerated,omitempty"`
}

// MasterProfile represents the definition of the master cluster
type MasterProfile struct {
	Count                     int                   `json:"count"`
	DNSPrefix                 string                `json:"dnsPrefix"`
	SubjectAltNames           []string              `json:"subjectAltNames"`
	VMSize                    string                `json:"vmSize"`
	OSDiskSizeGB              int                   `json:"osDiskSizeGB,omitempty"`
	VnetSubnetID              string                `json:"vnetSubnetID,omitempty"`
	VnetCidr                  string                `json:"vnetCidr,omitempty"`
	AgentVnetSubnetID         string                `json:"agentVnetSubnetID,omitempty"`
	FirstConsecutiveStaticIP  string                `json:"firstConsecutiveStaticIP,omitempty"`
	Subnet                    string                `json:"subnet"`
	SubnetIPv6                string                `json:"subnetIPv6"`
	IPAddressCount            int                   `json:"ipAddressCount,omitempty"`
	StorageProfile            string                `json:"storageProfile,omitempty"`
	HTTPSourceAddressPrefix   string                `json:"HTTPSourceAddressPrefix,omitempty"`
	OAuthEnabled              bool                  `json:"oauthEnabled"`
	PreprovisionExtension     *api.Extension        `json:"preProvisionExtension"`
	Extensions                []api.Extension       `json:"extensions"`
	Distro                    api.Distro            `json:"distro,omitempty"`
	KubernetesConfig          *api.KubernetesConfig `json:"kubernetesConfig,omitempty"`
	ImageRef                  *api.ImageReference   `json:"imageReference,omitempty"`
	CustomFiles               *[]api.CustomFile     `json:"customFiles,omitempty"`
	AvailabilityProfile       string                `json:"availabilityProfile"`
	PlatformFaultDomainCount  *int                  `json:"platformFaultDomainCount"`
	PlatformUpdateDomainCount *int                  `json:"platformUpdateDomainCount"`
	AgentSubnet               string                `json:"agentSubnet,omitempty"`
	AvailabilityZones         []string              `json:"availabilityZones,omitempty"`
	SinglePlacementGroup      *bool                 `json:"singlePlacementGroup,omitempty"`
	AuditDEnabled             *bool                 `json:"auditDEnabled,omitempty"`
	UltraSSDEnabled           *bool                 `json:"ultraSSDEnabled,omitempty"`
	EncryptionAtHost          *bool                 `json:"encryptionAtHost,omitempty"`
	CustomVMTags              map[string]string     `json:"customVMTags,omitempty"`
	// Master LB public endpoint/FQDN with port
	// The format will be FQDN:2376
	// Not used during PUT, returned as part of GET
	FQDN string `json:"fqdn,omitempty"`
	// True: uses cosmos etcd endpoint instead of installing etcd on masters
	CosmosEtcd                *bool  `json:"cosmosEtcd,omitempty"`
	ProximityPlacementGroupID string `json:"proximityPlacementGroupID,omitempty"`
}

// OrchestratorProfile contains Orchestrator properties
type OrchestratorProfile struct {
	OrchestratorType    string                `json:"orchestratorType"`
	OrchestratorVersion string                `json:"orchestratorVersion"`
	KubernetesConfig    *api.KubernetesConfig `json:"kubernetesConfig,omitempty"`
	DcosConfig          *api.DcosConfig       `json:"dcosConfig,omitempty"`
}

// ProvisioningState represents the current state of container service resource.
type ProvisioningState string

// AgentPoolProfile represents an agent pool definition
type AgentPoolProfile struct {
	Name                                string                   `json:"name"`
	Count                               int                      `json:"count"`
	VMSize                              string                   `json:"vmSize"`
	OSDiskSizeGB                        int                      `json:"osDiskSizeGB,omitempty"`
	DNSPrefix                           string                   `json:"dnsPrefix,omitempty"`
	OSType                              api.OSType               `json:"osType,omitempty"`
	Ports                               []int                    `json:"ports,omitempty"`
	ProvisioningState                   ProvisioningState        `json:"provisioningState,omitempty"`
	AvailabilityProfile                 string                   `json:"availabilityProfile"`
	ScaleSetPriority                    string                   `json:"scaleSetPriority,omitempty"`
	ScaleSetEvictionPolicy              string                   `json:"scaleSetEvictionPolicy,omitempty"`
	SpotMaxPrice                        *float64                 `json:"spotMaxPrice,omitempty"`
	StorageProfile                      string                   `json:"storageProfile,omitempty"`
	DiskSizesGB                         []int                    `json:"diskSizesGB,omitempty"`
	VnetSubnetID                        string                   `json:"vnetSubnetID,omitempty"`
	Subnet                              string                   `json:"subnet"`
	IPAddressCount                      int                      `json:"ipAddressCount,omitempty"`
	Distro                              api.Distro               `json:"distro,omitempty"`
	Role                                api.AgentPoolProfileRole `json:"role,omitempty"`
	AcceleratedNetworkingEnabled        *bool                    `json:"acceleratedNetworkingEnabled,omitempty"`
	AcceleratedNetworkingEnabledWindows *bool                    `json:"acceleratedNetworkingEnabledWindows,omitempty"`
	VMSSOverProvisioningEnabled         *bool                    `json:"vmssOverProvisioningEnabled,omitempty"`
	FQDN                                string                   `json:"fqdn,omitempty"`
	CustomNodeLabels                    map[string]string        `json:"customNodeLabels,omitempty"`
	PreprovisionExtension               *api.Extension           `json:"preProvisionExtension"`
	Extensions                          []api.Extension          `json:"extensions"`
	KubernetesConfig                    *api.KubernetesConfig    `json:"kubernetesConfig,omitempty"`
	OrchestratorVersion                 string                   `json:"orchestratorVersion"`
	ImageRef                            *api.ImageReference      `json:"imageReference,omitempty"`
	MaxCount                            *int                     `json:"maxCount,omitempty"`
	MinCount                            *int                     `json:"minCount,omitempty"`
	EnableAutoScaling                   *bool                    `json:"enableAutoScaling,omitempty"`
	AvailabilityZones                   []string                 `json:"availabilityZones,omitempty"`
	PlatformFaultDomainCount            *int                     `json:"platformFaultDomainCount"`
	PlatformUpdateDomainCount           *int                     `json:"platformUpdateDomainCount"`
	SinglePlacementGroup                *bool                    `json:"singlePlacementGroup,omitempty"`
	VnetCidrs                           []string                 `json:"vnetCidrs,omitempty"`
	PreserveNodesProperties             *bool                    `json:"preserveNodesProperties,omitempty"`
	WindowsNameVersion                  string                   `json:"windowsNameVersion,omitempty"`
	EnableVMSSNodePublicIP              *bool                    `json:"enableVMSSNodePublicIP,omitempty"`
	LoadBalancerBackendAddressPoolIDs   []string                 `json:"loadBalancerBackendAddressPoolIDs,omitempty"`
	AuditDEnabled                       *bool                    `json:"auditDEnabled,omitempty"`
	CustomVMTags                        map[string]string        `json:"customVMTags,omitempty"`
	DiskEncryptionSetID                 string                   `json:"diskEncryptionSetID,omitempty"`
	UltraSSDEnabled                     *bool                    `json:"ultraSSDEnabled,omitempty"`
	EncryptionAtHost                    *bool                    `json:"encryptionAtHost,omitempty"`
	ProximityPlacementGroupID           string                   `json:"proximityPlacementGroupID,omitempty"`
}

// Properties represents the AKS cluster definition
type Properties struct {
	ClusterID               string
	ProvisioningState       ProvisioningState        `json:"provisioningState,omitempty"`
	OrchestratorProfile     *OrchestratorProfile     `json:"orchestratorProfile,omitempty"`
	MasterProfile           *MasterProfile           `json:"masterProfile,omitempty"`
	AgentPoolProfiles       []*AgentPoolProfile      `json:"agentPoolProfiles,omitempty"`
	LinuxProfile            *LinuxProfile            `json:"linuxProfile,omitempty"`
	WindowsProfile          *WindowsProfile          `json:"windowsProfile,omitempty"`
	ExtensionProfiles       []*ExtensionProfile      `json:"extensionProfiles"`
	DiagnosticsProfile      *DiagnosticsProfile      `json:"diagnosticsProfile,omitempty"`
	JumpboxProfile          *JumpboxProfile          `json:"jumpboxProfile,omitempty"`
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

// GetCloudSpecConfig returns the Kubernetes container images URL configurations based on the deploy target environment.
//for example: if the target is the public azure, then the default container image url should be k8s.gcr.io/...
//if the target is azure china, then the default container image should be mirror.azure.cn:5000/google_container/...
func (cs *ContainerService) GetCloudSpecConfig() api.AzureEnvironmentSpecConfig {
	targetEnv := helpers.GetTargetEnv(cs.Location, cs.Properties.GetCustomCloudName())
	return api.AzureCloudSpecEnvMap[targetEnv]
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

// IsHostedMasterProfile returns true if the cluster has a hosted master
func (p *Properties) IsHostedMasterProfile() bool {
	return p.HostedMasterProfile != nil
}

// HasWindows returns true if the cluster contains windows
func (p *Properties) HasWindows() bool {
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if strings.EqualFold(string(agentPoolProfile.OSType), string(api.Windows)) {
			return true
		}
	}
	return false
}

// SetCloudProviderRateLimitDefaults sets default cloudprovider rate limiter config
func (p *Properties) SetCloudProviderRateLimitDefaults() {
	if p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucket == 0 {
		var agentPoolProfilesCount = len(p.AgentPoolProfiles)
		if agentPoolProfilesCount == 0 {
			p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucket = api.DefaultKubernetesCloudProviderRateLimitBucket
		} else {
			p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucket = agentPoolProfilesCount * common.MaxAgentCount
		}
	}
	if p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPS == 0 {
		if (api.DefaultKubernetesCloudProviderRateLimitQPS / float64(p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucket)) < common.MinCloudProviderQPSToBucketFactor {
			p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPS = float64(p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucket) * common.MinCloudProviderQPSToBucketFactor
		} else {
			p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPS = api.DefaultKubernetesCloudProviderRateLimitQPS
		}
	}
	if p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucketWrite == 0 {
		var agentPoolProfilesCount = len(p.AgentPoolProfiles)
		if agentPoolProfilesCount == 0 {
			p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucketWrite = api.DefaultKubernetesCloudProviderRateLimitBucketWrite
		} else {
			p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucketWrite = agentPoolProfilesCount * common.MaxAgentCount
		}
	}
	if p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPSWrite == 0 {
		if (api.DefaultKubernetesCloudProviderRateLimitQPSWrite / float64(p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucketWrite)) < common.MinCloudProviderQPSToBucketFactor {
			p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPSWrite = float64(p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucketWrite) * common.MinCloudProviderQPSToBucketFactor
		} else {
			p.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPSWrite = api.DefaultKubernetesCloudProviderRateLimitQPSWrite
		}
	}
}

// TotalNodes returns the total number of nodes in the cluster configuration
func (p *Properties) TotalNodes() int {
	var totalNodes int
	if p.MasterProfile != nil {
		totalNodes = p.MasterProfile.Count
	}
	for _, pool := range p.AgentPoolProfiles {
		totalNodes += pool.Count
	}
	return totalNodes
}

// HasAvailabilityZones returns true if the cluster contains a profile with zones
func (p *Properties) HasAvailabilityZones() bool {
	hasZones := p.MasterProfile != nil && p.MasterProfile.HasAvailabilityZones()
	if !hasZones && p.AgentPoolProfiles != nil {
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
		if p.MasterProfile != nil {
			h.Write([]byte(p.MasterProfile.DNSPrefix))
		} else if p.HostedMasterProfile != nil {
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

// HasCoreOS returns true if the cluster contains coreos nodes
func (p *Properties) HasCoreOS() bool {
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if strings.EqualFold(string(agentPoolProfile.Distro), string(api.CoreOS)) {
			return true
		}
	}
	return false
}

// K8sOrchestratorName returns the 3 character orchestrator code for kubernetes-based clusters.
func (p *Properties) K8sOrchestratorName() string {
	if p.OrchestratorProfile.IsKubernetes() {
		if p.HostedMasterProfile != nil {
			return api.DefaultHostedProfileMasterName
		}
		return api.DefaultOrchestratorName
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
	if p.MasterProfile != nil {
		return p.MasterProfile.IsVHDDistro()
	}
	return true
}

// GetVMType returns the type of VM "vmss" or "standard" to be passed to the cloud provider
func (p *Properties) GetVMType() string {
	if p.HasVMSSAgentPool() {
		return api.VMSSVMType
	}
	return api.StandardVMType
}

// HasVMSSAgentPool returns true if the cluster contains Virtual Machine Scale Sets agent pools
func (p *Properties) HasVMSSAgentPool() bool {
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if strings.EqualFold(agentPoolProfile.AvailabilityProfile, api.VirtualMachineScaleSets) {
			return true
		}
	}
	return false
}

// GetSubnetName returns the subnet name of the cluster based on its current configuration.
func (p *Properties) GetSubnetName() string {
	var subnetName string

	if !p.IsHostedMasterProfile() {
		if p.MasterProfile.IsCustomVNET() {
			subnetName = strings.Split(p.MasterProfile.VnetSubnetID, "/")[api.DefaultSubnetNameResourceSegmentIndex]
		} else if p.MasterProfile.IsVirtualMachineScaleSets() {
			subnetName = "subnetmaster"
		} else {
			subnetName = p.K8sOrchestratorName() + "-subnet"
		}
	} else {
		if p.AreAgentProfilesCustomVNET() {
			subnetName = strings.Split(p.AgentPoolProfiles[0].VnetSubnetID, "/")[api.DefaultSubnetNameResourceSegmentIndex]
		} else {
			subnetName = p.K8sOrchestratorName() + "-subnet"
		}
	}
	return subnetName
}

// GetNSGName returns the name of the network security group of the cluster.
func (p *Properties) GetNSGName() string {
	return p.GetResourcePrefix() + "nsg"
}

// GetResourcePrefix returns the prefix to use for naming cluster resources
func (p *Properties) GetResourcePrefix() string {
	if p.IsHostedMasterProfile() {
		return p.K8sOrchestratorName() + "-agentpool-" + p.GetClusterID() + "-"
	}
	return p.K8sOrchestratorName() + "-master-" + p.GetClusterID() + "-"
}

// GetVirtualNetworkName returns the virtual network name of the cluster
func (p *Properties) GetVirtualNetworkName() string {
	var vnetName string
	if p.IsHostedMasterProfile() && p.AreAgentProfilesCustomVNET() {
		vnetName = strings.Split(p.AgentPoolProfiles[0].VnetSubnetID, "/")[api.DefaultVnetNameResourceSegmentIndex]
	} else if !p.IsHostedMasterProfile() && p.MasterProfile.IsCustomVNET() {
		vnetName = strings.Split(p.MasterProfile.VnetSubnetID, "/")[api.DefaultVnetNameResourceSegmentIndex]
	} else {
		vnetName = p.K8sOrchestratorName() + "-vnet-" + p.GetClusterID()
	}
	return vnetName
}

// GetVNetResourceGroupName returns the virtual network resource group name of the cluster
func (p *Properties) GetVNetResourceGroupName() string {
	var vnetResourceGroupName string
	if p.IsHostedMasterProfile() && p.AreAgentProfilesCustomVNET() {
		vnetResourceGroupName = strings.Split(p.AgentPoolProfiles[0].VnetSubnetID, "/")[api.DefaultVnetResourceGroupSegmentIndex]
	} else if !p.IsHostedMasterProfile() && p.MasterProfile.IsCustomVNET() {
		vnetResourceGroupName = strings.Split(p.MasterProfile.VnetSubnetID, "/")[api.DefaultVnetResourceGroupSegmentIndex]
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
		if strings.EqualFold(p.AgentPoolProfiles[0].AvailabilityProfile, api.AvailabilitySet) {
			return p.AgentPoolProfiles[0].Name + "-availabilitySet-" + p.GetClusterID()
		}
	}
	return ""
}

// GetPrimaryScaleSetName returns the name of the primary scale set node of the cluster
func (p *Properties) GetPrimaryScaleSetName() string {
	if len(p.AgentPoolProfiles) > 0 {
		if strings.EqualFold(p.AgentPoolProfiles[0].AvailabilityProfile, api.VirtualMachineScaleSets) {
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
	return strings.EqualFold(string(a.Distro), string(api.AKSUbuntu1604)) || strings.EqualFold(string(a.Distro), string(api.AKSUbuntu1804)) || strings.EqualFold(string(a.Distro), string(api.Ubuntu1804Gen2)) || strings.EqualFold(string(a.Distro), string(api.AKSUbuntuGPU1804)) || strings.EqualFold(string(a.Distro), string(api.AKSUbuntuGPU1804Gen2))
}

// IsUbuntu1804 returns true if the agent pool profile distro is based on Ubuntu 16.04
func (a *AgentPoolProfile) IsUbuntu1804() bool {
	if !strings.EqualFold(string(a.OSType), string(api.Windows)) {
		switch a.Distro {
		case api.AKSUbuntu1804, api.Ubuntu1804, api.Ubuntu1804Gen2, api.AKSUbuntuGPU1804, api.AKSUbuntuGPU1804Gen2:
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
	return strings.EqualFold(string(a.OSType), string(api.Linux))
}

// IsCustomVNET returns true if the customer brought their own VNET
func (a *AgentPoolProfile) IsCustomVNET() bool {
	return len(a.VnetSubnetID) > 0
}

// IsWindows returns true if the agent pool is windows
func (a *AgentPoolProfile) IsWindows() bool {
	return strings.EqualFold(string(a.OSType), string(api.Windows))
}

// IsVirtualMachineScaleSets returns true if the agent pool availability profile is VMSS
func (a *AgentPoolProfile) IsVirtualMachineScaleSets() bool {
	return strings.EqualFold(a.AvailabilityProfile, api.VirtualMachineScaleSets)
}

// IsAvailabilitySets returns true if the customer specified disks
func (a *AgentPoolProfile) IsAvailabilitySets() bool {
	return strings.EqualFold(a.AvailabilityProfile, api.AvailabilitySet)
}

// IsSpotScaleSet returns true if the VMSS is Spot Scale Set
func (a *AgentPoolProfile) IsSpotScaleSet() bool {
	return strings.EqualFold(a.AvailabilityProfile, api.VirtualMachineScaleSets) && strings.EqualFold(a.ScaleSetPriority, api.ScaleSetPrioritySpot)
}

// GetKubernetesLabels returns a k8s API-compliant labels string for nodes in this profile
func (a *AgentPoolProfile) GetKubernetesLabels(rg string, deprecated bool) string {
	var buf bytes.Buffer
	buf.WriteString("kubernetes.azure.com/role=agent")
	if deprecated {
		buf.WriteString(",node-role.kubernetes.io/agent=")
		buf.WriteString(",kubernetes.io/role=agent")
	}
	buf.WriteString(fmt.Sprintf(",agentpool=%s", a.Name))
	if strings.EqualFold(a.StorageProfile, api.ManagedDisks) {
		storagetier, _ := common.GetStorageAccountType(a.VMSize)
		buf.WriteString(fmt.Sprintf(",storageprofile=managed,storagetier=%s", storagetier))
	}
	if common.IsNvidiaEnabledSKU(a.VMSize) {
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

// IsCoreOS returns true if the agent specified a CoreOS distro
func (a *AgentPoolProfile) IsCoreOS() bool {
	return strings.EqualFold(string(a.OSType), string(api.Linux)) && strings.EqualFold(string(a.Distro), string(api.CoreOS))
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
		return strings.EqualFold(o.KubernetesConfig.NetworkPlugin, api.NetworkPluginAzure)
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
	return api.DefaultWindowsSSHEnabled
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
	return api.KubernetesDefaultWindowsSku
}

// GetWindowsDockerVersion gets the docker version specified or returns default value
func (w *WindowsProfile) GetWindowsDockerVersion() string {
	if w.WindowsDockerVersion != "" {
		return w.WindowsDockerVersion
	}
	return api.KubernetesWindowsDockerVersion
}

// IsKubernetes returns true if this template is for Kubernetes orchestrator
func (o *OrchestratorProfile) IsKubernetes() bool {
	return strings.EqualFold(o.OrchestratorType, api.Kubernetes)
}

// IsSwarmMode returns true if this template is for Swarm Mode orchestrator
func (o *OrchestratorProfile) IsSwarmMode() bool {
	return strings.EqualFold(o.OrchestratorType, api.SwarmMode)
}

// IsPrivateCluster returns true if this deployment is a private cluster
func (o *OrchestratorProfile) IsPrivateCluster() bool {
	if !o.IsKubernetes() {
		return false
	}
	return o.KubernetesConfig != nil && o.KubernetesConfig.PrivateCluster != nil && to.Bool(o.KubernetesConfig.PrivateCluster.Enabled)
}

// GetPodInfraContainerSpec returns the sandbox image as a string (ex: k8s.gcr.io/pause-amd64:3.1)
func (o *OrchestratorProfile) GetPodInfraContainerSpec() string {
	return o.KubernetesConfig.MCRKubernetesImageBase + api.K8sComponentsByVersionMap[o.OrchestratorVersion]["pause"]
}

// HasCosmosEtcd returns true if cosmos etcd configuration is enabled
func (m *MasterProfile) HasCosmosEtcd() bool {
	return to.Bool(m.CosmosEtcd)
}

// GetCosmosEndPointURI returns the URI string for the cosmos etcd endpoint
func (m *MasterProfile) GetCosmosEndPointURI() string {
	if m.HasCosmosEtcd() {
		return fmt.Sprintf("%sk8s.etcd.cosmosdb.azure.com", m.DNSPrefix)
	}
	return ""
}

// IsVHDDistro returns true if the distro uses VHD SKUs
func (m *MasterProfile) IsVHDDistro() bool {
	return strings.EqualFold(string(m.Distro), string(api.AKSUbuntu1604)) || strings.EqualFold(string(m.Distro), string(api.AKSUbuntu1804))
}

// IsUbuntu1804 returns true if the master profile distro is based on Ubuntu 18.04
func (m *MasterProfile) IsUbuntu1804() bool {
	switch m.Distro {
	case api.AKSUbuntu1804, api.Ubuntu1804, api.Ubuntu1804Gen2:
		return true
	default:
		return false
	}
}

// IsCustomVNET returns true if the customer brought their own VNET
func (m *MasterProfile) IsCustomVNET() bool {
	return len(m.VnetSubnetID) > 0
}

// IsVirtualMachineScaleSets returns true if the master availability profile is VMSS
func (m *MasterProfile) IsVirtualMachineScaleSets() bool {
	return strings.EqualFold(m.AvailabilityProfile, api.VirtualMachineScaleSets)
}

// GetFirstConsecutiveStaticIPAddress returns the first static IP address of the given subnet.
func (m *MasterProfile) GetFirstConsecutiveStaticIPAddress(subnetStr string) string {
	_, subnet, err := net.ParseCIDR(subnetStr)
	if err != nil {
		return api.DefaultFirstConsecutiveKubernetesStaticIP
	}

	// Find the first and last octet of the host bits.
	ones, bits := subnet.Mask.Size()
	firstOctet := ones / 8
	lastOctet := bits/8 - 1

	if m.IsVirtualMachineScaleSets() {
		subnet.IP[lastOctet] = api.DefaultKubernetesFirstConsecutiveStaticIPOffsetVMSS
	} else {
		// Set the remaining host bits in the first octet.
		subnet.IP[firstOctet] |= (1 << byte((8 - (ones % 8)))) - 1

		// Fill the intermediate octets with 1s and last octet with offset. This is done so to match
		// the existing behavior of allocating static IP addresses from the last /24 of the subnet.
		for i := firstOctet + 1; i < lastOctet; i++ {
			subnet.IP[i] = 255
		}
		subnet.IP[lastOctet] = api.DefaultKubernetesFirstConsecutiveStaticIPOffset
	}

	return subnet.IP.String()
}

// HasAvailabilityZones returns true if the master profile has availability zones
func (m *MasterProfile) HasAvailabilityZones() bool {
	return m.AvailabilityZones != nil && len(m.AvailabilityZones) > 0
}

// HasMultipleNodes returns true if there are more than one master nodes
func (m *MasterProfile) HasMultipleNodes() bool {
	return m.Count > 1
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
