// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"strconv"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/go-autorest/autorest/to"
)

func getCustomDataVariables(cs *api.ContainerService, profile *api.AgentPoolProfile) paramsMap {
	cloudInitFiles := map[string]interface{}{
		"cloudInitData": paramsMap{
			"provisionScript":              getBase64EncodedGzippedCustomScript(kubernetesCSEMainScript, cs, profile),
			"provisionSource":              getBase64EncodedGzippedCustomScript(kubernetesCSEHelpersScript, cs, profile),
			"provisionInstalls":            getBase64EncodedGzippedCustomScript(kubernetesCSEInstall, cs, profile),
			"provisionConfigs":             getBase64EncodedGzippedCustomScript(kubernetesCSEConfig, cs, profile),
			"customSearchDomainsScript":    getBase64EncodedGzippedCustomScript(kubernetesCustomSearchDomainsScript, cs, profile),
			"dhcpv6SystemdService":         getBase64EncodedGzippedCustomScript(dhcpv6SystemdService, cs, profile),
			"dhcpv6ConfigurationScript":    getBase64EncodedGzippedCustomScript(dhcpv6ConfigurationScript, cs, profile),
			"kubeletSystemdService":        getBase64EncodedGzippedCustomScript(kubeletSystemdService, cs, profile),
			"systemdBPFMount":              getBase64EncodedGzippedCustomScript(systemdBPFMount, cs, profile),
			"reconcilePrivateHostsScript":  getBase64EncodedGzippedCustomScript(reconcilePrivateHostsScript, cs, profile),
			"reconcilePrivateHostsService": getBase64EncodedGzippedCustomScript(reconcilePrivateHostsService, cs, profile),
		},
	}

	cloudInitData := cloudInitFiles["cloudInitData"].(paramsMap)
	if cs.IsAKSCustomCloud() {
		cloudInitData["initAKSCustomCloud"] = getBase64EncodedGzippedCustomScript(initAKSCustomCloudScript, cs, profile)
	}

	if !cs.Properties.IsVHDDistroForAllNodes() {
		cloudInitData["provisionCIS"] = getBase64EncodedGzippedCustomScript(kubernetesCISScript, cs, profile)
		cloudInitData["kmsSystemdService"] = getBase64EncodedGzippedCustomScript(kmsSystemdService, cs, profile)
		cloudInitData["labelNodesScript"] = getBase64EncodedGzippedCustomScript(labelNodesScript, cs, profile)
		cloudInitData["labelNodesSystemdService"] = getBase64EncodedGzippedCustomScript(labelNodesSystemdService, cs, profile)
		cloudInitData["aptPreferences"] = getBase64EncodedGzippedCustomScript(aptPreferences, cs, profile)
		cloudInitData["healthMonitorScript"] = getBase64EncodedGzippedCustomScript(kubernetesHealthMonitorScript, cs, profile)
		cloudInitData["kubeletMonitorSystemdService"] = getBase64EncodedGzippedCustomScript(kubernetesKubeletMonitorSystemdService, cs, profile)
		cloudInitData["dockerMonitorSystemdService"] = getBase64EncodedGzippedCustomScript(kubernetesDockerMonitorSystemdService, cs, profile)
		cloudInitData["dockerMonitorSystemdTimer"] = getBase64EncodedGzippedCustomScript(kubernetesDockerMonitorSystemdTimer, cs, profile)
		cloudInitData["dockerClearMountPropagationFlags"] = getBase64EncodedGzippedCustomScript(dockerClearMountPropagationFlags, cs, profile)
		cloudInitData["auditdRules"] = getBase64EncodedGzippedCustomScript(auditdRules, cs, profile)
		cloudInitData["containerdSystemdService"] = getBase64EncodedGzippedCustomScript(containerdSystemdService, cs, profile)
	}

	return cloudInitFiles
}

func getCSECommandVariables(config *NodeBootstrappingConfiguration) paramsMap {
	cs := config.ContainerService
	profile := config.AgentPoolProfile
	return map[string]interface{}{
		"outBoundCmd":                     getOutBoundCmd(cs),
		"tenantID":                        config.TenantID,
		"subscriptionId":                  config.SubscriptionID,
		"resourceGroup":                   config.ResourceGroupName,
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
		"userAssignedIdentityID":          config.UserAssignedIdentityClientID,
		"isVHD":                           isVHD(profile),
		"gpuNode":                         strconv.FormatBool(common.IsNvidiaEnabledSKU(profile.VMSize)),
		"sgxNode":                         strconv.FormatBool(common.IsSgxEnabledSKU(profile.VMSize)),
		"auditdEnabled":                   strconv.FormatBool(to.Bool(profile.AuditDEnabled)),
		"configGPUDriverIfNeeded":         config.ConfigGPUDriverIfNeeded,
		"enableGPUDevicePluginIfNeeded":   config.EnableGPUDevicePluginIfNeeded,
		"enableDynamicKubelet":            config.EnableDynamicKubelet,
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
		registry = cs.Properties.CustomCloudEnv.McrURL + " 443"
	} else {
		registry = `mcr.microsoft.com 443`
	}

	if registry == "" {
		return ""
	}
	return `retrycmd_if_failure() { r=$1; w=$2; t=$3; shift && shift && shift; for i in $(seq 1 $r); do timeout $t ${@}; [ $? -eq 0  ] && break || if [ $i -eq $r ]; then return 1; else sleep $w; fi; done }; ERR_OUTBOUND_CONN_FAIL=50; retrycmd_if_failure 150 1 3 ` + ncBinary + ` -vz ` + registry + ` 2>&1 || exit $ERR_OUTBOUND_CONN_FAIL;`
}
