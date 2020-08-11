// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
	"sync"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/aks-engine/pkg/helpers"
	"github.com/Azure/go-autorest/autorest/to"
)

// AgentPoolProfile represents an agent pool definition
type AgentPoolProfile struct {
	Name                                string                   `json:"name"`
	Count                               int                      `json:"count"`
	VMSize                              string                   `json:"vmSize"`
	OSDiskSizeGB                        int                      `json:"osDiskSizeGB,omitempty"`
	DNSPrefix                           string                   `json:"dnsPrefix,omitempty"`
	OSType                              api.OSType               `json:"osType,omitempty"`
	Ports                               []int                    `json:"ports,omitempty"`
	ProvisioningState                   api.ProvisioningState    `json:"provisioningState,omitempty"`
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
	ProvisioningState       api.ProvisioningState        `json:"provisioningState,omitempty"`
	OrchestratorProfile     *api.OrchestratorProfile     `json:"orchestratorProfile,omitempty"`
	MasterProfile           *api.MasterProfile           `json:"masterProfile,omitempty"`
	AgentPoolProfiles       []*AgentPoolProfile          `json:"agentPoolProfiles,omitempty"`
	LinuxProfile            *api.LinuxProfile            `json:"linuxProfile,omitempty"`
	WindowsProfile          *api.WindowsProfile          `json:"windowsProfile,omitempty"`
	ExtensionProfiles       []*api.ExtensionProfile      `json:"extensionProfiles"`
	DiagnosticsProfile      *api.DiagnosticsProfile      `json:"diagnosticsProfile,omitempty"`
	JumpboxProfile          *api.JumpboxProfile          `json:"jumpboxProfile,omitempty"`
	ServicePrincipalProfile *api.ServicePrincipalProfile `json:"servicePrincipalProfile,omitempty"`
	CertificateProfile      *api.CertificateProfile      `json:"certificateProfile,omitempty"`
	AADProfile              *api.AADProfile              `json:"aadProfile,omitempty"`
	CustomProfile           *api.CustomProfile           `json:"customProfile,omitempty"`
	HostedMasterProfile     *api.HostedMasterProfile     `json:"hostedMasterProfile,omitempty"`
	AddonProfiles           map[string]api.AddonProfile  `json:"addonProfiles,omitempty"`
	FeatureFlags            *api.FeatureFlags            `json:"featureFlags,omitempty"`
	TelemetryProfile        *api.TelemetryProfile        `json:"telemetryProfile,omitempty"`
	CustomCloudEnv          *api.CustomCloudEnv          `json:"customCloudEnv,omitempty"`
}

// ContainerService complies with the ARM model of
// resource definition in a JSON template.
type ContainerService struct {
	ID       string                    `json:"id"`
	Location string                    `json:"location"`
	Name     string                    `json:"name"`
	Plan     *api.ResourcePurchasePlan `json:"plan,omitempty"`
	Tags     map[string]string         `json:"tags"`
	Type     string                    `json:"type"`

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
