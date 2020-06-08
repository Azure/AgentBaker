// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"strconv"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/go-autorest/autorest/to"
)

func getCustomDataVariables(cs *api.ContainerService) paramsMap {
	cloudInitFiles := map[string]interface{}{
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
			"initAKSCustomCloud":        getBase64EncodedGzippedCustomScript(initAKSCustomCloudScript, cs),
		},
	}

	cloudInitData := cloudInitFiles["cloudInitData"].(paramsMap)
	if !cs.Properties.IsVHDDistroForAllNodes() {
		cloudInitData["provisionCIS"] = getBase64EncodedGzippedCustomScript(kubernetesCISScript, cs)
		cloudInitData["kmsSystemdService"] = getBase64EncodedGzippedCustomScript(kmsSystemdService, cs)
		cloudInitData["labelNodesScript"] = getBase64EncodedGzippedCustomScript(labelNodesScript, cs)
		cloudInitData["labelNodesSystemdService"] = getBase64EncodedGzippedCustomScript(labelNodesSystemdService, cs)
		cloudInitData["aptPreferences"] = getBase64EncodedGzippedCustomScript(aptPreferences, cs)
		cloudInitData["healthMonitorScript"] = getBase64EncodedGzippedCustomScript(kubernetesHealthMonitorScript, cs)
		cloudInitData["kubeletMonitorSystemdService"] = getBase64EncodedGzippedCustomScript(kubernetesKubeletMonitorSystemdService, cs)
		cloudInitData["dockerMonitorSystemdService"] = getBase64EncodedGzippedCustomScript(kubernetesDockerMonitorSystemdService, cs)
		cloudInitData["dockerMonitorSystemdTimer"] = getBase64EncodedGzippedCustomScript(kubernetesDockerMonitorSystemdTimer, cs)
		cloudInitData["dockerClearMountPropagationFlags"] = getBase64EncodedGzippedCustomScript(dockerClearMountPropagationFlags, cs)
		cloudInitData["auditdRules"] = getBase64EncodedGzippedCustomScript(auditdRules, cs)
	}

	return cloudInitFiles
}

func getCSECommandVariables(cs *api.ContainerService, profile *api.AgentPoolProfile,
	tenantID, subscriptionID, resourceGroupName, userAssignedIdentityID string, needConfigGPUDrivers, enableGPUDevicePlugin bool) paramsMap {
	return map[string]interface{}{
		"outBoundCmd":                     getOutBoundCmd(cs),
		"tenantID":                        tenantID,
		"subscriptionId":                  subscriptionID,
		"resourceGroup":                   resourceGroupName,
		"location":                        cs.Location,
		"vmType":                          cs.Properties.GetVMType(),
		"subnetName":                      cs.Properties.GetSubnetName(),
		"nsgName":                         cs.Properties.GetNSGName(),
		"virtualNetworkName":              cs.Properties.GetVirtualNetworkName(),
		"virtualNetworkResourceGroupName": cs.Properties.GetVNetResourceGroupName(),
		"routeTableName":                  cs.Properties.GetRouteTableName(),
		"primaryAvailabilitySetName":      cs.Properties.GetPrimaryAvailabilitySetName(),
		"primaryScaleSetName":             cs.Properties.GetPrimaryScaleSetName(),
		"useManagedIdentityExtension":     useManagedIdentity(cs),
		"useInstanceMetadata":             useInstanceMetadata(cs),
		"loadBalancerSku":                 cs.Properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku,
		"excludeMasterFromStandardLB":     true,
		"maximumLoadBalancerRuleCount":    getMaximumLoadBalancerRuleCount(cs),
		"userAssignedIdentityID":          userAssignedIdentityID,
		"isVHD":                           isVHD(profile),
		"gpuNode":                         strconv.FormatBool(common.IsNvidiaEnabledSKU(profile.VMSize)),
		"sgxNode":                         strconv.FormatBool(common.IsSgxEnabledSKU(profile.VMSize)),
		"auditdEnabled":                   strconv.FormatBool(to.Bool(profile.AuditDEnabled)),
		"needConfigGPUDrivers":            needConfigGPUDrivers,
		"enableGPUDevicePlugin":           enableGPUDevicePlugin,
	}
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

func getOutBoundCmd(cs *api.ContainerService) string {
	if cs.Properties.FeatureFlags.IsFeatureEnabled("BlockOutboundInternet") {
		return ""
	}
	registry := ""
	ncBinary := "nc"
	if cs.GetCloudSpecConfig().CloudName == api.AzureChinaCloud {
		registry = `gcr.azk8s.cn 443`
	} else if cs.IsAKSCustomCloud() {
		registry = cs.Properties.CustomCloudEnv.McrURL
	} else {
		registry = `mcr.microsoft.com 443`
	}

	if registry == "" {
		return ""
	}
	return `retrycmd_if_failure() { r=$1; w=$2; t=$3; shift && shift && shift; for i in $(seq 1 $r); do timeout $t ${@}; [ $? -eq 0  ] && break || if [ $i -eq $r ]; then return 1; else sleep $w; fi; done }; ERR_OUTBOUND_CONN_FAIL=50; retrycmd_if_failure 50 1 3 ` + ncBinary + ` -vz ` + registry + ` 2>&1 || exit $ERR_OUTBOUND_CONN_FAIL;`
}
