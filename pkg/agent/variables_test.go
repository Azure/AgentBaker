// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Windows custom data variables check", func() {
	var (
		config *datamodel.NodeBootstrappingConfiguration
	)

	BeforeEach(func() {
		config = getDefaultNBC()
	})

	It("sets tenantId", func() {
		config.TenantID = "test tenant id"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["tenantID"]).To(Equal("test tenant id"))
	})

	It("sets subscriptionId", func() {
		config.SubscriptionID = "test sub id"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["subscriptionId"]).To(Equal("test sub id"))
	})

	It("sets resourceGroup", func() {
		config.ResourceGroupName = "test rg"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["resourceGroup"]).To(Equal("test rg"))
	})

	It("sets location", func() {
		config.ContainerService.Location = "test loc"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["location"]).To(Equal("test loc"))
	})

	It("sets vmType for vmss", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].AvailabilityProfile = datamodel.VirtualMachineScaleSets
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["vmType"]).To(Equal("vmss"))
	})

	It("sets vmType for vmas", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].AvailabilityProfile = datamodel.AvailabilitySet
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["vmType"]).To(Equal("standard"))
	})

	It("sets subnetName for custom subnet", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID =
			"/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/testSubnetName"

		vars := getWindowsCustomDataVariables(config)
		Expect(vars["subnetName"]).To(Equal("testSubnetName"))
	})

	It("sets subnetName for regular subnet", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID = ""

		vars := getWindowsCustomDataVariables(config)
		Expect(vars["subnetName"]).To(Equal("aks-subnet"))
	})

	It("sets nsgName", func() {
		config.ContainerService.Properties.ClusterID = "36873793"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["nsgName"]).To(Equal("aks-agentpool-36873793-nsg"))
	})

	It("sets virtualNetworkName for custom subnet", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID =
			"/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/testVnetName/subnet/testSubnetName"

		vars := getWindowsCustomDataVariables(config)
		Expect(vars["virtualNetworkName"]).To(Equal("testVnetName"))
	})

	It("sets virtualNetworkName for regular subnet", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID = ""
		config.ContainerService.Properties.ClusterID = "36873793"

		vars := getWindowsCustomDataVariables(config)
		Expect(vars["virtualNetworkName"]).To(Equal("aks-vnet-36873793"))
	})

	It("sets routeTableName", func() {
		config.ContainerService.Properties.ClusterID = "36873793"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["routeTableName"]).To(Equal("aks-agentpool-36873793-routetable"))
	})

	It("sets primaryAvailabilitySetName to nothing when no availability set", func() {
		config.ContainerService.Properties.ClusterID = "36873793"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["primaryAvailabilitySetName"]).To(Equal(""))
	})

	It("sets primaryAvailabilitySetName when there is an availability set", func() {
		config.ContainerService.Properties.ClusterID = "36873793"
		config.ContainerService.Properties.AgentPoolProfiles[0].Name = "agentpoolname"
		config.ContainerService.Properties.AgentPoolProfiles[0].AvailabilityProfile = datamodel.AvailabilitySet
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["primaryAvailabilitySetName"]).To(Equal("agentpoolname-availabilitySet-36873793"))
	})

	It("sets primaryScaleSetName", func() {
		config.PrimaryScaleSetName = "primary ss name"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["primaryScaleSetName"]).To(Equal("primary ss name"))
	})

	It("sets useManagedIdentityExtension to true when using managed identity", func() {
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["useManagedIdentityExtension"]).To(Equal("true"))
	})

	It("sets useManagedIdentityExtension to false when not using managed identity", func() {
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = false
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["useManagedIdentityExtension"]).To(Equal("false"))
	})

	It("sets useInstanceMetadata to true when using instance metadata", func() {
		val := true
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseInstanceMetadata = &val
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["useInstanceMetadata"]).To(Equal("true"))
	})

	It("sets useInstanceMetadata to false when not using instance metadata", func() {
		val := false
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseInstanceMetadata = &val
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["useInstanceMetadata"]).To(Equal("false"))
	})

	It("sets loadBalancerSku ", func() {
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku = "load balencer sku"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["loadBalancerSku"]).To(Equal("load balencer sku"))
	})

	It("sets excludeMasterFromStandardLB", func() {
		// at the time of writing this test, this variable was hard coded to true
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["excludeMasterFromStandardLB"]).To(Equal(true))
	})

	It("sets windowsEnableCSIProxy to true", func() {
		value := true
		config.ContainerService.Properties.WindowsProfile.EnableCSIProxy = &value
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsEnableCSIProxy"]).To(Equal(true))
	})

	It("sets windowsEnableCSIProxy to false", func() {
		value := false
		config.ContainerService.Properties.WindowsProfile.EnableCSIProxy = &value
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsEnableCSIProxy"]).To(Equal(false))
	})

	It("sets windowsEnableCSIProxy to the default when no proxy set", func() {
		config.ContainerService.Properties.WindowsProfile.EnableCSIProxy = nil
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsEnableCSIProxy"]).To(Equal(false))
	})

	It("sets windowsCSIProxyURL", func() {
		config.ContainerService.Properties.WindowsProfile.CSIProxyURL = "csi proxy url"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsCSIProxyURL"]).To(Equal("csi proxy url"))
	})

	It("sets windowsProvisioningScriptsPackageURL", func() {
		config.ContainerService.Properties.WindowsProfile.ProvisioningScriptsPackageURL = "prov script url"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsProvisioningScriptsPackageURL"]).To(Equal("prov script url"))
	})

	It("sets windowsPauseImageURL", func() {
		config.ContainerService.Properties.WindowsProfile.WindowsPauseImageURL = "pause image url"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsPauseImageURL"]).To(Equal("pause image url"))
	})

	It("sets alwaysPullWindowsPauseImage to true when true", func() {
		value := true
		config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = &value
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["alwaysPullWindowsPauseImage"]).To(Equal("true"))
	})

	It("sets alwaysPullWindowsPauseImage to false when false", func() {
		value := false
		config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = &value
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["alwaysPullWindowsPauseImage"]).To(Equal("false"))
	})

	It("sets alwaysPullWindowsPauseImage to false when nil", func() {
		config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = nil
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["alwaysPullWindowsPauseImage"]).To(Equal("false"))
	})

	It("sets windowsCalicoPackageURL", func() {
		config.ContainerService.Properties.WindowsProfile.WindowsCalicoPackageURL = "calico package url"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsCalicoPackageURL"]).To(Equal("calico package url"))
	})

	It("sets configGPUDriverIfNeeded to true", func() {
		config.ConfigGPUDriverIfNeeded = true
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["configGPUDriverIfNeeded"]).To(Equal(true))
	})

	It("sets configGPUDriverIfNeeded to false", func() {
		config.ConfigGPUDriverIfNeeded = false
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["configGPUDriverIfNeeded"]).To(Equal(false))
	})

	It("sets windowsSecureTlsEnabled to true", func() {
		value := true
		config.ContainerService.Properties.WindowsProfile.WindowsSecureTlsEnabled = &value
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsSecureTlsEnabled"]).To(Equal(true))
	})

	It("sets windowsSecureTlsEnabled to false", func() {
		value := false
		config.ContainerService.Properties.WindowsProfile.WindowsSecureTlsEnabled = &value
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsSecureTlsEnabled"]).To(Equal(false))
	})

	It("sets windowsSecureTlsEnabled to false when nil", func() {
		config.ContainerService.Properties.WindowsProfile.WindowsSecureTlsEnabled = nil
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsSecureTlsEnabled"]).To(Equal(false))
	})

	It("sets windowsGmsaPackageUrl", func() {
		config.ContainerService.Properties.WindowsProfile.WindowsGmsaPackageUrl = "gsma package url"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsGmsaPackageUrl"]).To(Equal("gsma package url"))
	})

	It("sets windowsGpuDriverURL", func() {
		config.ContainerService.Properties.WindowsProfile.GpuDriverURL = "gpu driver url"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsGpuDriverURL"]).To(Equal("gpu driver url"))
	})

	It("sets windowsCSEScriptsPackageURL", func() {
		config.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = "cse scripts url"
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["windowsCSEScriptsPackageURL"]).To(Equal("cse scripts url"))
	})

	It("sets isDisableWindowsOutboundNat to true", func() {
		value := true
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			DisableOutboundNat: &value,
		}
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["isDisableWindowsOutboundNat"]).To(Equal("true"))
	})

	It("sets isDisableWindowsOutboundNat to false", func() {
		value := false
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			DisableOutboundNat: &value,
		}
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["isDisableWindowsOutboundNat"]).To(Equal("false"))
	})

	It("sets isDisableWindowsOutboundNat to false when nil", func() {
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			DisableOutboundNat: nil,
		}
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["isDisableWindowsOutboundNat"]).To(Equal("false"))
	})

	It("sets nextGenNetworkingEnabled to true", func() {
		value := true
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			NextGenNetworkingEnabled: &value,
		}
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["nextGenNetworkingEnabled"]).To(Equal("true"))
	})

	It("sets nextGenNetworkingEnabled to false", func() {
		value := false
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			NextGenNetworkingEnabled: &value,
		}
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["nextGenNetworkingEnabled"]).To(Equal("false"))
	})

	It("sets nextGenNetworkingEnabled to false when nil", func() {
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			NextGenNetworkingEnabled: nil,
		}
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["nextGenNetworkingEnabled"]).To(Equal("false"))
	})

	It("sets nextGenNetworkingConfig", func() {
		value := "next gen networking config"
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			NextGenNetworkingConfig: &value,
		}
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["nextGenNetworkingConfig"]).To(Equal("next gen networking config"))
	})

	It("sets nextGenNetworkingConfig with empty config when nil", func() {
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			NextGenNetworkingConfig: nil,
		}
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["nextGenNetworkingConfig"]).To(Equal(""))
	})

	It("sets nextGenNetworkingConfig with empty config when AgentPoolWindowsProfile is nil", func() {
		config.AgentPoolProfile.AgentPoolWindowsProfile = nil
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["nextGenNetworkingConfig"]).To(Equal(""))
	})

	It("sets isSkipCleanupNetwork to true", func() {
		value := true
		config.AgentPoolProfile.NotRebootWindowsNode = &value
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["isSkipCleanupNetwork"]).To(Equal("true"))
	})

	It("sets isSkipCleanupNetwork to false", func() {
		value := false
		config.AgentPoolProfile.NotRebootWindowsNode = &value
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["isSkipCleanupNetwork"]).To(Equal("false"))
	})
	It("sets isSkipCleanupNetwork to false when nil", func() {
		config.AgentPoolProfile.NotRebootWindowsNode = nil
		vars := getWindowsCustomDataVariables(config)
		Expect(vars["isSkipCleanupNetwork"]).To(Equal("false"))
	})

})

var _ = Describe("Windows CSE variables check", func() {
	var (
		config *datamodel.NodeBootstrappingConfiguration
	)

	BeforeEach(func() {
		config = getDefaultNBC()
	})

	It("sets maximumLoadBalancerRuleCount", func() {
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.MaximumLoadBalancerRuleCount = 5
		vars := getCSECommandVariables(config)
		Expect(vars["maximumLoadBalancerRuleCount"]).To(Equal(5))
	})

	It("sets userAssignedIdentityID", func() {
		config.UserAssignedIdentityClientID = "the identity id"
		vars := getCSECommandVariables(config)
		Expect(vars["userAssignedIdentityID"]).To(Equal("the identity id"))
	})

	It("sets tenantId", func() {
		config.TenantID = "test tenant id"
		vars := getCSECommandVariables(config)
		Expect(vars["tenantID"]).To(Equal("test tenant id"))
	})

	It("sets subscriptionId", func() {
		config.SubscriptionID = "test sub id"
		vars := getCSECommandVariables(config)
		Expect(vars["subscriptionId"]).To(Equal("test sub id"))
	})

	It("sets resourceGroup", func() {
		config.ResourceGroupName = "test rg"
		vars := getCSECommandVariables(config)
		Expect(vars["resourceGroup"]).To(Equal("test rg"))
	})

	It("sets location", func() {
		config.ContainerService.Location = "test loc"
		vars := getCSECommandVariables(config)
		Expect(vars["location"]).To(Equal("test loc"))
	})

	It("sets vmType for vmss", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].AvailabilityProfile = datamodel.VirtualMachineScaleSets
		vars := getCSECommandVariables(config)
		Expect(vars["vmType"]).To(Equal("vmss"))
	})

	It("sets vmType for vmas", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].AvailabilityProfile = datamodel.AvailabilitySet
		vars := getCSECommandVariables(config)
		Expect(vars["vmType"]).To(Equal("standard"))
	})

	It("sets subnetName for custom subnet", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID =
			"/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/testSubnetName"

		vars := getCSECommandVariables(config)
		Expect(vars["subnetName"]).To(Equal("testSubnetName"))
	})

	It("sets subnetName for regular subnet", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID = ""

		vars := getCSECommandVariables(config)
		Expect(vars["subnetName"]).To(Equal("aks-subnet"))
	})

	It("sets nsgName", func() {
		config.ContainerService.Properties.ClusterID = "36873793"
		vars := getCSECommandVariables(config)
		Expect(vars["nsgName"]).To(Equal("aks-agentpool-36873793-nsg"))
	})

	It("sets virtualNetworkName for custom subnet", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID =
			"/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/testVnetName/subnet/testSubnetName"

		vars := getCSECommandVariables(config)
		Expect(vars["virtualNetworkName"]).To(Equal("testVnetName"))
	})

	It("sets virtualNetworkName for regular subnet", func() {
		config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID = ""
		config.ContainerService.Properties.ClusterID = "36873793"

		vars := getCSECommandVariables(config)
		Expect(vars["virtualNetworkName"]).To(Equal("aks-vnet-36873793"))
	})

	It("sets routeTableName", func() {
		config.ContainerService.Properties.ClusterID = "36873793"
		vars := getCSECommandVariables(config)
		Expect(vars["routeTableName"]).To(Equal("aks-agentpool-36873793-routetable"))
	})

	It("sets primaryAvailabilitySetName to nothing when no availability set", func() {
		config.ContainerService.Properties.ClusterID = "36873793"
		vars := getCSECommandVariables(config)
		Expect(vars["primaryAvailabilitySetName"]).To(Equal(""))
	})

	It("sets primaryAvailabilitySetName when there is an availability set", func() {
		config.ContainerService.Properties.ClusterID = "36873793"
		config.ContainerService.Properties.AgentPoolProfiles[0].Name = "agentpoolname"
		config.ContainerService.Properties.AgentPoolProfiles[0].AvailabilityProfile = datamodel.AvailabilitySet
		vars := getCSECommandVariables(config)
		Expect(vars["primaryAvailabilitySetName"]).To(Equal("agentpoolname-availabilitySet-36873793"))
	})

	It("sets primaryScaleSetName", func() {
		config.PrimaryScaleSetName = "primary ss name"
		vars := getCSECommandVariables(config)
		Expect(vars["primaryScaleSetName"]).To(Equal("primary ss name"))
	})

	It("sets useManagedIdentityExtension to true when using managed identity", func() {
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
		vars := getCSECommandVariables(config)
		Expect(vars["useManagedIdentityExtension"]).To(Equal("true"))
	})

	It("sets useManagedIdentityExtension to false when not using managed identity", func() {
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = false
		vars := getCSECommandVariables(config)
		Expect(vars["useManagedIdentityExtension"]).To(Equal("false"))
	})

	It("sets useInstanceMetadata to true when using instance metadata", func() {
		val := true
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseInstanceMetadata = &val
		vars := getCSECommandVariables(config)
		Expect(vars["useInstanceMetadata"]).To(Equal("true"))
	})

	It("sets useInstanceMetadata to false when not using instance metadata", func() {
		val := false
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseInstanceMetadata = &val
		vars := getCSECommandVariables(config)
		Expect(vars["useInstanceMetadata"]).To(Equal("false"))
	})

	It("sets loadBalancerSku ", func() {
		config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku = "load balencer sku"
		vars := getCSECommandVariables(config)
		Expect(vars["loadBalancerSku"]).To(Equal("load balencer sku"))
	})

	It("sets excludeMasterFromStandardLB", func() {
		// at the time of writing this test, this variable was hard coded to true
		vars := getCSECommandVariables(config)
		Expect(vars["excludeMasterFromStandardLB"]).To(Equal(true))
	})

	It("sets windowsEnableCSIProxy to true", func() {
		value := true
		config.ContainerService.Properties.WindowsProfile.EnableCSIProxy = &value
		vars := getCSECommandVariables(config)
		Expect(vars["windowsEnableCSIProxy"]).To(Equal(true))
	})

	It("sets windowsEnableCSIProxy to false", func() {
		value := false
		config.ContainerService.Properties.WindowsProfile.EnableCSIProxy = &value
		vars := getCSECommandVariables(config)
		Expect(vars["windowsEnableCSIProxy"]).To(Equal(false))
	})

	It("sets windowsEnableCSIProxy to the default when no proxy set", func() {
		config.ContainerService.Properties.WindowsProfile.EnableCSIProxy = nil
		vars := getCSECommandVariables(config)
		Expect(vars["windowsEnableCSIProxy"]).To(Equal(false))
	})

	It("sets windowsCSIProxyURL", func() {
		config.ContainerService.Properties.WindowsProfile.CSIProxyURL = "csi proxy url"
		vars := getCSECommandVariables(config)
		Expect(vars["windowsCSIProxyURL"]).To(Equal("csi proxy url"))
	})

	It("sets windowsProvisioningScriptsPackageURL", func() {
		config.ContainerService.Properties.WindowsProfile.ProvisioningScriptsPackageURL = "prov script url"
		vars := getCSECommandVariables(config)
		Expect(vars["windowsProvisioningScriptsPackageURL"]).To(Equal("prov script url"))
	})

	It("sets windowsPauseImageURL", func() {
		config.ContainerService.Properties.WindowsProfile.WindowsPauseImageURL = "pause image url"
		vars := getCSECommandVariables(config)
		Expect(vars["windowsPauseImageURL"]).To(Equal("pause image url"))
	})

	It("sets alwaysPullWindowsPauseImage to true when true", func() {
		value := true
		config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = &value
		vars := getCSECommandVariables(config)
		Expect(vars["alwaysPullWindowsPauseImage"]).To(Equal("true"))
	})

	It("sets alwaysPullWindowsPauseImage to false when false", func() {
		value := false
		config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = &value
		vars := getCSECommandVariables(config)
		Expect(vars["alwaysPullWindowsPauseImage"]).To(Equal("false"))
	})

	It("sets alwaysPullWindowsPauseImage to false when nil", func() {
		config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = nil
		vars := getCSECommandVariables(config)
		Expect(vars["alwaysPullWindowsPauseImage"]).To(Equal("false"))
	})

	It("sets windowsCalicoPackageURL", func() {
		config.ContainerService.Properties.WindowsProfile.WindowsCalicoPackageURL = "calico package url"
		vars := getCSECommandVariables(config)
		Expect(vars["windowsCalicoPackageURL"]).To(Equal("calico package url"))
	})

	It("sets configGPUDriverIfNeeded to true", func() {
		config.ConfigGPUDriverIfNeeded = true
		vars := getCSECommandVariables(config)
		Expect(vars["configGPUDriverIfNeeded"]).To(Equal(true))
	})

	It("sets configGPUDriverIfNeeded to false", func() {
		config.ConfigGPUDriverIfNeeded = false
		vars := getCSECommandVariables(config)
		Expect(vars["configGPUDriverIfNeeded"]).To(Equal(false))
	})

	It("sets windowsSecureTlsEnabled to true", func() {
		value := true
		config.ContainerService.Properties.WindowsProfile.WindowsSecureTlsEnabled = &value
		vars := getCSECommandVariables(config)
		Expect(vars["windowsSecureTlsEnabled"]).To(Equal(true))
	})

	It("sets windowsSecureTlsEnabled to false", func() {
		value := false
		config.ContainerService.Properties.WindowsProfile.WindowsSecureTlsEnabled = &value
		vars := getCSECommandVariables(config)
		Expect(vars["windowsSecureTlsEnabled"]).To(Equal(false))
	})

	It("sets windowsSecureTlsEnabled to false when nil", func() {
		config.ContainerService.Properties.WindowsProfile.WindowsSecureTlsEnabled = nil
		vars := getCSECommandVariables(config)
		Expect(vars["windowsSecureTlsEnabled"]).To(Equal(false))
	})

	It("sets windowsGmsaPackageUrl", func() {
		config.ContainerService.Properties.WindowsProfile.WindowsGmsaPackageUrl = "gsma package url"
		vars := getCSECommandVariables(config)
		Expect(vars["windowsGmsaPackageUrl"]).To(Equal("gsma package url"))
	})

	It("sets windowsGpuDriverURL", func() {
		config.ContainerService.Properties.WindowsProfile.GpuDriverURL = "gpu driver url"
		vars := getCSECommandVariables(config)
		Expect(vars["windowsGpuDriverURL"]).To(Equal("gpu driver url"))
	})

	It("sets windowsCSEScriptsPackageURL", func() {
		config.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = "cse scripts url"
		vars := getCSECommandVariables(config)
		Expect(vars["windowsCSEScriptsPackageURL"]).To(Equal("cse scripts url"))
	})

	It("sets isDisableWindowsOutboundNat to true", func() {
		value := true
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			DisableOutboundNat: &value,
		}
		vars := getCSECommandVariables(config)
		Expect(vars["isDisableWindowsOutboundNat"]).To(Equal("true"))
	})

	It("sets isDisableWindowsOutboundNat to false", func() {
		value := false
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			DisableOutboundNat: &value,
		}
		vars := getCSECommandVariables(config)
		Expect(vars["isDisableWindowsOutboundNat"]).To(Equal("false"))
	})

	It("sets isDisableWindowsOutboundNat to false when nil", func() {
		config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
			DisableOutboundNat: nil,
		}
		vars := getCSECommandVariables(config)
		Expect(vars["isDisableWindowsOutboundNat"]).To(Equal("false"))
	})

	It("sets isSkipCleanupNetwork to true", func() {
		value := true
		config.AgentPoolProfile.NotRebootWindowsNode = &value
		vars := getCSECommandVariables(config)
		Expect(vars["isSkipCleanupNetwork"]).To(Equal("true"))
	})

	It("sets isSkipCleanupNetwork to false", func() {
		value := false
		config.AgentPoolProfile.NotRebootWindowsNode = &value
		vars := getCSECommandVariables(config)
		Expect(vars["isSkipCleanupNetwork"]).To(Equal("false"))
	})
	It("sets isSkipCleanupNetwork to false when nil", func() {
		config.AgentPoolProfile.NotRebootWindowsNode = nil
		vars := getCSECommandVariables(config)
		Expect(vars["isSkipCleanupNetwork"]).To(Equal("false"))
	})

})

func getDefaultNBC() *datamodel.NodeBootstrappingConfiguration {
	cs := &datamodel.ContainerService{
		Location: "southcentralus",
		Type:     "Microsoft.ContainerService/ManagedClusters",
		Properties: &datamodel.Properties{
			ClusterID: "36873792",
			OrchestratorProfile: &datamodel.OrchestratorProfile{
				OrchestratorType:    datamodel.Kubernetes,
				OrchestratorVersion: "1.16.15",
				KubernetesConfig:    &datamodel.KubernetesConfig{},
			},
			HostedMasterProfile: &datamodel.HostedMasterProfile{
				DNSPrefix: "uttestdom",
			},
			AgentPoolProfiles: []*datamodel.AgentPoolProfile{
				{
					Name:                "agent2",
					VMSize:              "Standard_DS1_v2",
					StorageProfile:      "ManagedDisks",
					OSType:              datamodel.Linux,
					VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet1",
					AvailabilityProfile: datamodel.VirtualMachineScaleSets,
					Distro:              datamodel.AKSUbuntu1604,
				},
			},
			WindowsProfile: &datamodel.WindowsProfile{},
			LinuxProfile: &datamodel.LinuxProfile{
				AdminUsername: "azureuser",
			},
			ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
				ClientID: "ClientID",
				Secret:   "Secret",
			},
		},
	}
	cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
		KeyData: string("testsshkey"),
	}}

	agentPool := cs.Properties.AgentPoolProfiles[0]

	k8sComponents := &datamodel.K8sComponents{}

	kubeletConfig := map[string]string{
		"--address":                           "0.0.0.0",
		"--pod-manifest-path":                 "/etc/kubernetes/manifests",
		"--cloud-provider":                    "azure",
		"--cloud-config":                      "/etc/kubernetes/azure.json",
		"--azure-container-registry-config":   "/etc/kubernetes/azure.json",
		"--cluster-domain":                    "cluster.local",
		"--cluster-dns":                       "10.0.0.10",
		"--cgroups-per-qos":                   "true",
		"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
		"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
		"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
		"--max-pods":                          "110",
		"--node-status-update-frequency":      "10s",
		"--image-gc-high-threshold":           "85",
		"--image-gc-low-threshold":            "80",
		"--event-qps":                         "0",
		"--pod-max-pids":                      "-1",
		"--enforce-node-allocatable":          "pods",
		"--streaming-connection-idle-timeout": "4h0m0s",
		"--rotate-certificates":               "true",
		"--read-only-port":                    "10255",
		"--protect-kernel-defaults":           "true",
		"--resolv-conf":                       "/etc/resolv.conf",
		"--anonymous-auth":                    "false",
		"--client-ca-file":                    "/etc/kubernetes/certs/ca.crt",
		"--authentication-token-webhook":      "true",
		"--authorization-mode":                "Webhook",
		"--eviction-hard":                     "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%",
		"--feature-gates":                     "RotateKubeletServerCertificate=true,a=b,PodPriority=true,x=y",
		"--system-reserved":                   "cpu=2,memory=1Gi",
		"--kube-reserved":                     "cpu=100m,memory=1638Mi",
	}

	galleries := map[string]datamodel.SIGGalleryConfig{
		"AKSUbuntu": {
			GalleryName:   "aksubuntu",
			ResourceGroup: "resourcegroup",
		},
		"AKSCBLMariner": {
			GalleryName:   "akscblmariner",
			ResourceGroup: "resourcegroup",
		},
		"AKSAzureLinux": {
			GalleryName:   "aksazurelinux",
			ResourceGroup: "resourcegroup",
		},
		"AKSWindows": {
			GalleryName:   "akswindows",
			ResourceGroup: "resourcegroup",
		},
		"AKSUbuntuEdgeZone": {
			GalleryName:   "AKSUbuntuEdgeZone",
			ResourceGroup: "AKS-Ubuntu-EdgeZone",
		},
		"AKSFlatcar": {
			GalleryName:   "aksflatcar",
			ResourceGroup: "resourcegroup",
		},
	}
	sigConfig := &datamodel.SIGConfig{
		TenantID:       "sometenantid",
		SubscriptionID: "somesubid",
		Galleries:      galleries,
	}

	config := &datamodel.NodeBootstrappingConfiguration{
		ContainerService:              cs,
		CloudSpecConfig:               datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:                 k8sComponents,
		AgentPoolProfile:              agentPool,
		TenantID:                      "tenantID",
		SubscriptionID:                "subID",
		ResourceGroupName:             "resourceGroupName",
		UserAssignedIdentityClientID:  "userAssignedID",
		ConfigGPUDriverIfNeeded:       true,
		EnableGPUDevicePluginIfNeeded: false,
		EnableKubeletConfigFile:       false,
		EnableNvidia:                  false,
		FIPSEnabled:                   false,
		KubeletConfig:                 kubeletConfig,
		PrimaryScaleSetName:           "aks-agent2-36873793-vmss",
		SIGConfig:                     *sigConfig,
	}

	return config
}
