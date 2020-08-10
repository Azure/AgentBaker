// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"
	"sync"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/aks-engine/pkg/helpers"
)

// Properties represents the AKS cluster definition
type Properties struct {
	ClusterID               string
	ProvisioningState       api.ProvisioningState        `json:"provisioningState,omitempty"`
	OrchestratorProfile     *api.OrchestratorProfile     `json:"orchestratorProfile,omitempty"`
	MasterProfile           *api.MasterProfile           `json:"masterProfile,omitempty"`
	AgentPoolProfiles       []*api.AgentPoolProfile      `json:"agentPoolProfiles,omitempty"`
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

// ToAksEngineContainerService converts our ContainerService to aks-engine's
// ContainerService to our. This is temporarily needed until we have finished
// porting all aks-engine code that's used by us into our own code base.
func ToAksEngineContainerService(cs *ContainerService) *api.ContainerService {
	ret := &api.ContainerService{
		ID:       cs.ID,
		Location: cs.Location,
		Name:     cs.Name,
		Plan:     cs.Plan,
		Tags:     cs.Tags,
		Type:     cs.Type,
	}
	if cs.Properties != nil {
		ret.Properties = toAksEngineProperties(cs.Properties)
	}
	return ret
}

func toAksEngineProperties(p *Properties) *api.Properties {
	ret := &api.Properties{
		ClusterID:               p.ClusterID,
		ProvisioningState:       p.ProvisioningState,
		OrchestratorProfile:     p.OrchestratorProfile,
		MasterProfile:           p.MasterProfile,
		AgentPoolProfiles:       p.AgentPoolProfiles,
		LinuxProfile:            p.LinuxProfile,
		WindowsProfile:          p.WindowsProfile,
		ExtensionProfiles:       p.ExtensionProfiles,
		DiagnosticsProfile:      p.DiagnosticsProfile,
		JumpboxProfile:          p.JumpboxProfile,
		ServicePrincipalProfile: p.ServicePrincipalProfile,
		CertificateProfile:      p.CertificateProfile,
		AADProfile:              p.AADProfile,
		CustomProfile:           p.CustomProfile,
		HostedMasterProfile:     p.HostedMasterProfile,
		AddonProfiles:           p.AddonProfiles,
		FeatureFlags:            p.FeatureFlags,
		TelemetryProfile:        p.TelemetryProfile,
		CustomCloudEnv:          p.CustomCloudEnv,
	}
	return ret
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
func (p *Properties) GetAgentVMPrefix(a *api.AgentPoolProfile, index int) string {
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
