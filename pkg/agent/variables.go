// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// getCustomDataVariables returns rendered, unencoded file contents used by Linux.
func getCustomDataVariables(config *datamodel.NodeBootstrappingConfiguration) paramsMap {
	cs := config.ContainerService
	cloudInitFiles := map[string]interface{}{
		"cloudInitData": paramsMap{
			"provisionStartScript":                  getCustomScript(kubernetesCSEStartScript, config),
			"provisionScript":                       getCustomScript(kubernetesCSEMainScript, config),
			"provisionSource":                       getCustomScript(kubernetesCSEHelpersScript, config),
			"provisionSourceUbuntu":                 getCustomScript(kubernetesCSEHelpersScriptUbuntu, config),
			"provisionSourceMariner":                getCustomScript(kubernetesCSEHelpersScriptMariner, config),
			"provisionSourceAzlOSGuard":             getCustomScript(kubernetesCSEHelpersScriptAzlOSGuard, config),
			"provisionSourceFlatcar":                getCustomScript(kubernetesCSEHelpersScriptFlatcar, config),
			"provisionSourceACL":                    getCustomScript(kubernetesCSEHelpersScriptACL, config),
			"provisionInstalls":                     getCustomScript(kubernetesCSEInstall, config),
			"provisionInstallsUbuntu":               getCustomScript(kubernetesCSEInstallUbuntu, config),
			"provisionInstallsMariner":              getCustomScript(kubernetesCSEInstallMariner, config),
			"provisionInstallsAzlOSGuard":           getCustomScript(kubernetesCSEInstallAzlOSGuard, config),
			"provisionInstallsFlatcar":              getCustomScript(kubernetesCSEInstallFlatcar, config),
			"provisionInstallsACL":                  getCustomScript(kubernetesCSEInstallACL, config),
			"provisionConfigs":                      getCustomScript(kubernetesCSEConfig, config),
			"provisionConfigsGPU":                   getCustomScript(kubernetesCSEConfigGPU, config),
			"provisionConfigsLocalDNS":              getCustomScript(kubernetesCSEConfigLocalDNS, config),
			"provisionConfigsKubelet":               getCustomScript(kubernetesCSEConfigKubelet, config),
			"provisionConfigsNetwork":               getCustomScript(kubernetesCSEConfigNetwork, config),
			"provisionConfigsChrony":                getCustomScript(kubernetesCSEConfigChrony, config),
			"provisionSendLogs":                     getCustomScript(kubernetesCSESendLogs, config),
			"provisionRedactCloudConfig":            getCustomScript(kubernetesCSERedactCloudConfig, config),
			"customSearchDomainsScript":             getCustomScript(kubernetesCustomSearchDomainsScript, config),
			"dhcpv6SystemdService":                  getCustomScript(dhcpv6SystemdService, config),
			"dhcpv6ConfigurationScript":             getCustomScript(dhcpv6ConfigurationScript, config),
			"kubeletSystemdService":                 getCustomScript(kubeletSystemdService, config),
			"reconcilePrivateHostsScript":           getCustomScript(reconcilePrivateHostsScript, config),
			"reconcilePrivateHostsService":          getCustomScript(reconcilePrivateHostsService, config),
			"ensureNoDupEbtablesScript":             getCustomScript(ensureNoDupEbtablesScript, config),
			"ensureNoDupEbtablesService":            getCustomScript(ensureNoDupEbtablesService, config),
			"bindMountScript":                       getCustomScript(bindMountScript, config),
			"bindMountSystemdService":               getCustomScript(bindMountSystemdService, config),
			"migPartitionSystemdService":            getCustomScript(migPartitionSystemdService, config),
			"migPartitionScript":                    getCustomScript(migPartitionScript, config),
			"ensureIMDSRestrictionScript":           getCustomScript(ensureIMDSRestrictionScript, config),
			"snapshotUpdateScript":                  getCustomScript(snapshotUpdateScript, config),
			"snapshotUpdateService":                 getCustomScript(snapshotUpdateSystemdService, config),
			"snapshotUpdateTimer":                   getCustomScript(snapshotUpdateSystemdTimer, config),
			"packageUpdateScriptMariner":            getCustomScript(packageUpdateScriptMariner, config),
			"packageUpdateServiceMariner":           getCustomScript(packageUpdateSystemdServiceMariner, config),
			"packageUpdateTimerMariner":             getCustomScript(packageUpdateSystemdTimerMariner, config),
			"componentManifestFile":                 getCustomScript(componentManifestFile, config),
			"validateKubeletCredentialsScript":      getCustomScript(validateKubeletCredentialsScript, config),
			"secureTLSBootstrapService":             getCustomScript(secureTLSBootstrapService, config),
			"cloudInitStatusCheckScript":            getCustomScript(cloudInitStatusCheckScript, config),
			"measureTLSBootstrappingLatencyScript":  getCustomScript(measureTLSBootstrappingLatencyScript, config),
			"measureTLSBootstrappingLatencyService": getCustomScript(measureTLSBootstrappingLatencyService, config),
			"configureAzureNetworkScript":           getCustomScript(configureAzureNetworkScript, config),
			"azureNetworkUdevRule":                  getCustomScript(azureNetworkUdevRule, config),
		},
	}

	cloudInitData := cloudInitFiles["cloudInitData"].(paramsMap) //nolint:errcheck // no error is actually here
	cloudInitData["initAKSCloud"] = getCustomScript(initAKSCloudScript, config)

	if config.IsFlatcar() || config.IsACL() {
		cloudInitData["provisionRedactCloudConfig"] = "" // Flatcar and ACL do not have cloud-init
	}

	if !cs.Properties.IsVHDDistroForAllNodes() {
		cloudInitData["kmsSystemdService"] = getCustomScript(kmsSystemdService, config)
		cloudInitData["aptPreferences"] = getCustomScript(aptPreferences, config)
		cloudInitData["dockerClearMountPropagationFlags"] = getCustomScript(dockerClearMountPropagationFlags, config)
	}

	return cloudInitFiles
}

// getWindowsCustomDataVariables returns custom data for Windows.
func getWindowsCustomDataVariables(config *datamodel.NodeBootstrappingConfiguration) paramsMap {
	return getCSECommandVariables(config)
}

func getCSECommandVariables(config *datamodel.NodeBootstrappingConfiguration) paramsMap {
	cs := config.ContainerService
	profile := config.AgentPoolProfile
	httpProxy, httpsProxy, noProxy := "", "", ""
	if config.HTTPProxyConfig != nil {
		if config.HTTPProxyConfig.HTTPProxy != nil {
			httpProxy = *config.HTTPProxyConfig.HTTPProxy
		}
		if config.HTTPProxyConfig.HTTPSProxy != nil {
			httpsProxy = *config.HTTPProxyConfig.HTTPSProxy
		}
		if config.HTTPProxyConfig.NoProxy != nil {
			noProxy = strings.Join(*config.HTTPProxyConfig.NoProxy, ",")
		}
	}

	// this method is called for both windows and linux. If there's no windows profile, then let's just
	// use a blank one.
	windowsProfile := cs.Properties.WindowsProfile
	if windowsProfile == nil {
		windowsProfile = &datamodel.WindowsProfile{}
	}

	agentPoolProfileWindows := profile.GetAgentPoolWindowsProfile()
	if agentPoolProfileWindows == nil {
		agentPoolProfileWindows = &datamodel.AgentPoolWindowsProfile{}
	}

	subnetName := cs.Properties.GetSubnetName()
	if config.SubnetName != nil {
		subnetName = *config.SubnetName
	}
	routeTableName := cs.Properties.GetRouteTableName()
	if config.RouteTableName != nil {
		routeTableName = *config.RouteTableName
	}

	return map[string]interface{}{
		"tenantID":                               config.TenantID,
		"subscriptionId":                         config.SubscriptionID,
		"resourceGroup":                          config.ResourceGroupName,
		"location":                               cs.Location,
		"vmType":                                 cs.Properties.GetVMType(),
		"subnetName":                             subnetName,
		"nsgName":                                cs.Properties.GetNSGName(),
		"virtualNetworkName":                     cs.Properties.GetVirtualNetworkName(),
		"virtualNetworkResourceGroupName":        cs.Properties.GetVNetResourceGroupName(),
		"routeTableName":                         routeTableName,
		"primaryAvailabilitySetName":             cs.Properties.GetPrimaryAvailabilitySetName(),
		"primaryScaleSetName":                    config.PrimaryScaleSetName,
		"useManagedIdentityExtension":            useManagedIdentity(cs),
		"useInstanceMetadata":                    useInstanceMetadata(cs),
		"loadBalancerSku":                        cs.Properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku,
		"excludeMasterFromStandardLB":            true,
		"maximumLoadBalancerRuleCount":           getMaximumLoadBalancerRuleCount(cs),
		"userAssignedIdentityID":                 config.UserAssignedIdentityClientID,
		"isVHD":                                  isVHD(profile),
		"gpuNode":                                strconv.FormatBool(config.EnableNvidia),
		"sgxNode":                                strconv.FormatBool(datamodel.IsSgxEnabledSKU(profile.VMSize)),
		"configGPUDriverIfNeeded":                config.ConfigGPUDriverIfNeeded,
		"enableGPUDevicePluginIfNeeded":          config.EnableGPUDevicePluginIfNeeded,
		"migNode":                                strconv.FormatBool(datamodel.IsMIGNode(config.GPUInstanceProfile, config.MIGProfileLayout)),
		"gpuInstanceProfile":                     config.GPUInstanceProfile,
		"migProfileLayout":                       strings.Join(config.MIGProfileLayout, ","),
		"windowsEnableCSIProxy":                  windowsProfile.IsCSIProxyEnabled(),
		"windowsPauseImageURL":                   windowsProfile.WindowsPauseImageURL,
		"windowsCSIProxyURL":                     windowsProfile.CSIProxyURL,
		"windowsProvisioningScriptsPackageURL":   windowsProfile.ProvisioningScriptsPackageURL,
		"alwaysPullWindowsPauseImage":            strconv.FormatBool(windowsProfile.IsAlwaysPullWindowsPauseImage()),
		"windowsCalicoPackageURL":                windowsProfile.WindowsCalicoPackageURL,
		"windowsSecureTlsEnabled":                windowsProfile.IsWindowsSecureTlsEnabled(),
		"windowsGmsaPackageUrl":                  windowsProfile.WindowsGmsaPackageUrl,
		"windowsGpuDriverURL":                    windowsProfile.GpuDriverURL,
		"windowsCSEScriptsPackageURL":            windowsProfile.CseScriptsPackageURL,
		"isDisableWindowsOutboundNat":            strconv.FormatBool(config.AgentPoolProfile.IsDisableWindowsOutboundNat()),
		"isSkipCleanupNetwork":                   strconv.FormatBool(config.AgentPoolProfile.IsSkipCleanupNetwork()),
		"nextGenNetworkingEnabled":               strconv.FormatBool(agentPoolProfileWindows.IsNextGenNetworkingEnabled()),
		"nextGenNetworkingConfig":                agentPoolProfileWindows.GetNextGenNetworkingConfig(),
		"serviceAccountImagePullBindingEnabled":  strconv.FormatBool(getServiceAccountImagePullEnabled(cs)),
		"serviceAccountImagePullDefaultClientID": getServiceAccountImagePullDefaultClientID(cs),
		"serviceAccountImagePullDefaultTenantID": getServiceAccountImagePullDefaultTenantID(cs),
		"identityBindingsLocalAuthoritySNI":      getServiceAccountImagePullLocalAuthoritySNI(cs),
		"httpProxyShellQuoted":                   shellQuote(httpProxy),
		"httpsProxyShellQuoted":                  shellQuote(httpsProxy),
		"noProxyShellQuoted":                     shellQuote(noProxy),
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

func getServiceAccountImagePullEnabled(cs *datamodel.ContainerService) bool {
	if cs.Properties.ServiceAccountImagePullProfile == nil {
		return false
	}
	return cs.Properties.ServiceAccountImagePullProfile.Enabled
}

func getServiceAccountImagePullDefaultClientID(cs *datamodel.ContainerService) string {
	if cs.Properties.ServiceAccountImagePullProfile == nil {
		return ""
	}
	return cs.Properties.ServiceAccountImagePullProfile.DefaultClientID
}

func getServiceAccountImagePullDefaultTenantID(cs *datamodel.ContainerService) string {
	if cs.Properties.ServiceAccountImagePullProfile == nil {
		return ""
	}
	return cs.Properties.ServiceAccountImagePullProfile.DefaultTenantID
}

func getServiceAccountImagePullLocalAuthoritySNI(cs *datamodel.ContainerService) string {
	if cs.Properties.ServiceAccountImagePullProfile == nil {
		return ""
	}
	return cs.Properties.ServiceAccountImagePullProfile.LocalAuthoritySNI
}

func isVHD(profile *datamodel.AgentPoolProfile) string {
	//NOTE: update as new distro is introduced.
	return strconv.FormatBool(profile.IsVHDDistro())
}

func getOutBoundCmd(nbc *datamodel.NodeBootstrappingConfiguration, cloudSpecConfig *datamodel.AzureEnvironmentSpecConfig) string {
	cs := nbc.ContainerService
	if cs.Properties.FeatureFlags.IsFeatureEnabled("BlockOutboundInternet") {
		return ""
	}

	if strings.EqualFold(nbc.OutboundType, datamodel.OutboundTypeBlock) || strings.EqualFold(nbc.OutboundType, datamodel.OutboundTypeNone) {
		return ""
	}

	var registry string
	switch {
	case cloudSpecConfig.CloudName == datamodel.AzureChinaCloud:
		registry = `gcr.azk8s.cn`
	case cs.IsAKSCustomCloud():
		registry = cs.Properties.CustomCloudEnv.McrURL
	default:
		registry = `mcr.microsoft.com`
	}

	if registry == "" {
		return ""
	}

	connectivityCheckCommand := `curl -v --insecure --proxy-insecure https://` + registry + `/v2/`

	return connectivityCheckCommand
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func getProxyVariables(nbc *datamodel.NodeBootstrappingConfiguration) string {
	if nbc.HTTPProxyConfig == nil ||
		(nbc.HTTPProxyConfig.HTTPProxy == nil && nbc.HTTPProxyConfig.HTTPSProxy == nil && nbc.HTTPProxyConfig.NoProxy == nil) {
		return ""
	}

	// Older VHDs evaluate PROXY_VARS. Keep this payload free of customer-controlled values;
	// those values are shell-quoted separately and referenced only through variables here.
	return `if [ -n "${HTTP_PROXY_URLS}" ]; then export HTTP_PROXY="${HTTP_PROXY_URLS}" http_proxy="${HTTP_PROXY_URLS}"; fi; ` +
		`if [ -n "${HTTPS_PROXY_URLS}" ]; then export HTTPS_PROXY="${HTTPS_PROXY_URLS}" https_proxy="${HTTPS_PROXY_URLS}"; fi; ` +
		`if [ -n "${NO_PROXY_URLS}" ]; then export NO_PROXY="${NO_PROXY_URLS}" no_proxy="${NO_PROXY_URLS}"; fi`
}
