// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"strconv"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/go-autorest/autorest/to"
)

func getCustomDataVariables(config *NodeBootstrappingConfiguration) paramsMap {
	cs := config.ContainerService
	cloudInitFiles := map[string]interface{}{
		"cloudInitData": paramsMap{
			"provisionScript":              getBase64EncodedGzippedCustomScript(kubernetesCSEMainScript, config),
			"provisionSource":              getBase64EncodedGzippedCustomScript(kubernetesCSEHelpersScript, config),
			"provisionInstalls":            getBase64EncodedGzippedCustomScript(kubernetesCSEInstall, config),
			"provisionConfigs":             getBase64EncodedGzippedCustomScript(kubernetesCSEConfig, config),
			"customSearchDomainsScript":    getBase64EncodedGzippedCustomScript(kubernetesCustomSearchDomainsScript, config),
			"dhcpv6SystemdService":         getBase64EncodedGzippedCustomScript(dhcpv6SystemdService, config),
			"dhcpv6ConfigurationScript":    getBase64EncodedGzippedCustomScript(dhcpv6ConfigurationScript, config),
			"kubeletSystemdService":        getBase64EncodedGzippedCustomScript(kubeletSystemdService, config),
			"systemdBPFMount":              getBase64EncodedGzippedCustomScript(systemdBPFMount, config),
			"reconcilePrivateHostsScript":  getBase64EncodedGzippedCustomScript(reconcilePrivateHostsScript, config),
			"reconcilePrivateHostsService": getBase64EncodedGzippedCustomScript(reconcilePrivateHostsService, config),
		},
	}

	cloudInitData := cloudInitFiles["cloudInitData"].(paramsMap)
	if cs.IsAKSCustomCloud() {
		cloudInitData["initAKSCustomCloud"] = getBase64EncodedGzippedCustomScript(initAKSCustomCloudScript, config)
	}

	if !cs.Properties.IsVHDDistroForAllNodes() {
		cloudInitData["provisionCIS"] = getBase64EncodedGzippedCustomScript(kubernetesCISScript, config)
		cloudInitData["kmsSystemdService"] = getBase64EncodedGzippedCustomScript(kmsSystemdService, config)
		cloudInitData["labelNodesScript"] = getBase64EncodedGzippedCustomScript(labelNodesScript, config)
		cloudInitData["labelNodesSystemdService"] = getBase64EncodedGzippedCustomScript(labelNodesSystemdService, config)
		cloudInitData["aptPreferences"] = getBase64EncodedGzippedCustomScript(aptPreferences, config)
		cloudInitData["healthMonitorScript"] = getBase64EncodedGzippedCustomScript(kubernetesHealthMonitorScript, config)
		cloudInitData["kubeletMonitorSystemdService"] = getBase64EncodedGzippedCustomScript(kubernetesKubeletMonitorSystemdService, config)
		cloudInitData["dockerMonitorSystemdService"] = getBase64EncodedGzippedCustomScript(kubernetesDockerMonitorSystemdService, config)
		cloudInitData["dockerMonitorSystemdTimer"] = getBase64EncodedGzippedCustomScript(kubernetesDockerMonitorSystemdTimer, config)
		cloudInitData["dockerClearMountPropagationFlags"] = getBase64EncodedGzippedCustomScript(dockerClearMountPropagationFlags, config)
		cloudInitData["auditdRules"] = getBase64EncodedGzippedCustomScript(auditdRules, config)
		cloudInitData["containerdSystemdService"] = getBase64EncodedGzippedCustomScript(containerdSystemdService, config)
	}

	return cloudInitFiles
}

func getCSECommandVariables(config *NodeBootstrappingConfiguration) paramsMap {
	cs := config.ContainerService
	profile := config.AgentPoolProfile
	return map[string]interface{}{
		"outBoundCmd":                     getOutBoundCmd(cs, config.CloudSpecConfig),
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

func useManagedIdentity(cs *datamodel.ContainerService) string {
	useManagedIdentity := cs.Properties.OrchestratorProfile.KubernetesConfig != nil &&
		cs.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity
	return strconv.FormatBool(useManagedIdentity)
}

func useInstanceMetadata(cs *datamodel.ContainerService) string {
	useInstanceMetadata := cs.Properties.OrchestratorProfile.KubernetesConfig != nil &&
		cs.Properties.OrchestratorProfile.KubernetesConfig.UseInstanceMetadata != nil &&
		*cs.Properties.OrchestratorProfile.KubernetesConfig.UseInstanceMetadata
	return strconv.FormatBool(useInstanceMetadata)
}

func getMaximumLoadBalancerRuleCount(cs *datamodel.ContainerService) int {
	if cs.Properties.OrchestratorProfile.KubernetesConfig != nil {
		return cs.Properties.OrchestratorProfile.KubernetesConfig.MaximumLoadBalancerRuleCount
	}
	return 0
}

func isVHD(profile *datamodel.AgentPoolProfile) string {
	//NOTE: update as new distro is introduced
	return strconv.FormatBool(profile.IsVHDDistro())
}

func getOutBoundCmd(cs *datamodel.ContainerService, cloudSpecConfig *datamodel.AzureEnvironmentSpecConfig) string {
	if cs.Properties.FeatureFlags.IsFeatureEnabled("BlockOutboundInternet") {
		return ""
	}
	registry := ""
	ncBinary := "nc"
	if cloudSpecConfig.CloudName == datamodel.AzureChinaCloud {
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
