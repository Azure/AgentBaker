// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"fmt"
	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/go-autorest/autorest/to"
	"strconv"
	"strings"
)

func getCustomDataVariables(cs *api.ContainerService, generatorCode string, aksEngineVersion string) paramsMap {
	return map[string]interface{}{
		"cloudInitData": paramsMap{
			"provisionScript":           getBase64EncodedGzippedCustomScript(kubernetesCSEMainScript, cs),
			"provisionSource":           getBase64EncodedGzippedCustomScript(kubernetesCSEHelpersScript, cs),
			"provisionInstalls":         getBase64EncodedGzippedCustomScript(kubernetesCSEInstall, cs),
			"provisionConfigs":          getBase64EncodedGzippedCustomScript(kubernetesCSEConfig, cs),
			"customSearchDomainsScript": getBase64EncodedGzippedCustomScript(kubernetesCustomSearchDomainsScript, cs),
			"dhcpv6SystemdService":      getBase64EncodedGzippedCustomScript(dhcpv6SystemdService, cs),
			"dhcpv6ConfigurationScript": getBase64EncodedGzippedCustomScript(dhcpv6ConfigurationScript, cs),
			"kubeletSystemdService":     getBase64EncodedGzippedCustomScript(kubeletSystemdService, cs),
			"systemdBPFMount":           getBase64EncodedGzippedCustomScript(systemdBPFMount, cs),
		},
	}
}

func getCSECommandVariables(cs *api.ContainerService, profile *api.AgentPoolProfile, params paramsMap, userAssignedIdentityID, generatorCode string, aksEngineVersion string) paramsMap {
	variables := map[string]interface{}{
		"outBoundCmd":                  getOutBoundCmd(cs),
		"tenantID":                     getTenantID(),
		"subscriptionId":               getSubscriptionID(),
		"resourceGroup":                getResourceGroupName(),
		"location":                     getLocation(),
		"vmType":                       getVMType(cs),
		"agentNamePrefix":              fmt.Sprintf("%s-agentpool-%s-", params["orchestratorName"].(paramsMap)["value"], params["nameSuffix"].(paramsMap)["value"]),
		"primaryAvailabilitySetName":   getPrimaryAvailabilitySetName(cs, params),
		"primaryScaleSetName":          cs.Properties.GetPrimaryScaleSetName(),
		"useManagedIdentityExtension":  useManagedIdentity(cs),
		"useInstanceMetadata":          useInstanceMetadata(cs),
		"loadBalancerSku":              cs.Properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku,
		"excludeMasterFromStandardLB":  true,
		"maximumLoadBalancerRuleCount": getMaximumLoadBalancerRuleCount(cs),
		"userAssignedIdentityID":       userAssignedIdentityID,
		"isVHD":                        isVHD(profile),
		"gpuNode":                      strconv.FormatBool(common.IsNvidiaEnabledSKU(profile.VMSize)),
		"sgxNode":                      strconv.FormatBool(common.IsSgxEnabledSKU(profile.VMSize)),
		"auditdEnabled":                strconv.FormatBool(to.Bool(profile.AuditDEnabled)),
	}
	variables["nsgName"] = fmt.Sprintf("%snsg", variables["agentNamePrefix"])
	variables["routeTableName"] = fmt.Sprintf("%sroutetable", variables["agentNamePrefix"])

	vnetSubnetID := ""
	subnetName := ""
	vnetID := ""
	virtualNetworkName := ""
	virtualNetworkResourceGroupName := ""
	if cs.Properties.AreAgentProfilesCustomVNET() {
		vnetSubnetID = params[fmt.Sprintf("%sVnetSubnetID", profile.Name)].(paramsMap)["value"].(string)
		subnetName = strings.Split(vnetSubnetID, "/")[10]
		virtualNetworkName = strings.Split(vnetSubnetID, "/")[8]
		virtualNetworkResourceGroupName = strings.Split(vnetSubnetID, "/")[4]
	} else {
		virtualNetworkName = fmt.Sprintf("%s-vnet-%s", params["orchestratorName"].(paramsMap)["value"], params["nameSuffix"].(paramsMap)["value"])
		vnetID = getResourceID("Microsoft.Network/virtualNetworks", virtualNetworkName)
		subnetName = fmt.Sprintf("%s-subnet", params["orchestratorName"].(paramsMap)["value"])
		vnetSubnetID = getSubResourceID(vnetID, "subnets", subnetName)
		variables["vnetID"] = vnetID
	}
	variables["vnetSubnetID"] = vnetSubnetID
	variables["subnetName"] = subnetName
	variables["virtualNetworkName"] = virtualNetworkName
	variables["virtualNetworkResourceGroupName"] = virtualNetworkResourceGroupName

	return variables
}

func getTenantID() string {
	return ""
}

func getSubscriptionID() string {
	return ""
}

func getLocation() string {
	return ""
}

func getResourceGroupName() string {
	return ""
}

func getVMType(cs *api.ContainerService) string {
	if cs.Properties.AnyAgentUsesVirtualMachineScaleSets() {
		return "vmss"
	}
	return "standard"
}

func getPrimaryAvailabilitySetName(cs *api.ContainerService, params paramsMap) string {
	if cs.Properties.AnyAgentUsesVirtualMachineScaleSets() || len(cs.Properties.AgentPoolProfiles) == 0 {
		return ""
	}
	return fmt.Sprintf("%s-availabilitySet-%s", cs.Properties.AgentPoolProfiles[0].Name, params["nameSuffix"].(paramsMap)["value"])
}

func useManagedIdentity(cs *api.ContainerService) string {
	useManagedIdentity := cs.Properties.OrchestratorProfile.KubernetesConfig != nil &&
		cs.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity
	return strconv.FormatBool(useManagedIdentity)
}

func useInstanceMetadata(cs *api.ContainerService) string {
	useInstanceMetadata := cs.Properties.OrchestratorProfile.KubernetesConfig != nil &&
		cs.Properties.OrchestratorProfile.KubernetesConfig.UseInstanceMetadata != nil &&
		*cs.Properties.OrchestratorProfile.KubernetesConfig.UseInstanceMetadata
	return strconv.FormatBool(useInstanceMetadata)
}

func getMaximumLoadBalancerRuleCount(cs *api.ContainerService) int {
	if cs.Properties.OrchestratorProfile.KubernetesConfig != nil {
		return cs.Properties.OrchestratorProfile.KubernetesConfig.MaximumLoadBalancerRuleCount
	}
	return 0
}

func isVHD(profile *api.AgentPoolProfile) string {
	//NOTE: update as new distro is introduced
	return strconv.FormatBool(profile.IsVHDDistro())
}

func getResourceID(resourceType, resourceName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/%s/%s",
		getSubscriptionID(),
		getResourceGroupName(),
		resourceType,
		resourceName)
}

func getSubResourceID(resourceID, subResourceType, subResourceName string) string {
	return fmt.Sprintf("%s/%s/%s", resourceID, subResourceType, subResourceName)
}

func getOutBoundCmd(cs *api.ContainerService) string {
	if cs.Properties.FeatureFlags.IsFeatureEnabled("BlockOutboundInternet") {
		return ""
	}
	registry := ""
	ncBinary := "nc"
	if cs.GetCloudSpecConfig().CloudName == api.AzureChinaCloud {
		registry = `gcr.azk8s.cn 443`
	} else {
		registry = `aksrepos.azurecr.io 443`
	}
	return `retrycmd_if_failure() { r=$1; w=$2; t=$3; shift && shift && shift; for i in $(seq 1 $r); do timeout $t ${@}; [ $? -eq 0  ] && break || if [ $i -eq $r ]; then return 1; else sleep $w; fi; done }; ERR_OUTBOUND_CONN_FAIL=50; retrycmd_if_failure 50 1 3 ` + ncBinary + ` -vz ` + registry + ` || exit $ERR_OUTBOUND_CONN_FAIL;`
}
