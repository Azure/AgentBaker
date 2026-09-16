// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// getCustomDataVariables returns cloudinit data used by Linux.
func getCustomDataVariables(config *datamodel.NodeBootstrappingConfiguration) paramsMap {
	return getCustomDataVariablesWithEncoding(config, true)
}

func getCustomDataVariablesWithEncoding(config *datamodel.NodeBootstrappingConfiguration, compressFiles bool) paramsMap {
	renderScript := getBase64EncodedGzippedCustomScript
	encoding, contentPrefix := "gzip", "!!binary |\n    "
	if !compressFiles {
		renderScript = getYAMLQuotedCustomScript
		encoding, contentPrefix = `""`, ""
	}
	cs := config.ContainerService
	cloudInitFiles := map[string]interface{}{
		"cloudInitFile": paramsMap{
			"encoding":      encoding,
			"contentPrefix": contentPrefix,
		},
		"cloudInitData": paramsMap{
			"provisionStartScript":                  renderScript(kubernetesCSEStartScript, config),
			"provisionScript":                       renderScript(kubernetesCSEMainScript, config),
			"provisionSource":                       renderScript(kubernetesCSEHelpersScript, config),
			"provisionSourceUbuntu":                 renderScript(kubernetesCSEHelpersScriptUbuntu, config),
			"provisionSourceMariner":                renderScript(kubernetesCSEHelpersScriptMariner, config),
			"provisionSourceAzlOSGuard":             renderScript(kubernetesCSEHelpersScriptAzlOSGuard, config),
			"provisionSourceFlatcar":                renderScript(kubernetesCSEHelpersScriptFlatcar, config),
			"provisionSourceACL":                    renderScript(kubernetesCSEHelpersScriptACL, config),
			"provisionInstalls":                     renderScript(kubernetesCSEInstall, config),
			"provisionInstallsUbuntu":               renderScript(kubernetesCSEInstallUbuntu, config),
			"provisionInstallsMariner":              renderScript(kubernetesCSEInstallMariner, config),
			"provisionInstallsAzlOSGuard":           renderScript(kubernetesCSEInstallAzlOSGuard, config),
			"provisionInstallsFlatcar":              renderScript(kubernetesCSEInstallFlatcar, config),
			"provisionInstallsACL":                  renderScript(kubernetesCSEInstallACL, config),
			"provisionConfigs":                      renderScript(kubernetesCSEConfig, config),
			"provisionConfigsGPU":                   renderScript(kubernetesCSEConfigGPU, config),
			"provisionConfigsLocalDNS":              renderScript(kubernetesCSEConfigLocalDNS, config),
			"provisionConfigsKubelet":               renderScript(kubernetesCSEConfigKubelet, config),
			"provisionConfigsNetwork":               renderScript(kubernetesCSEConfigNetwork, config),
			"provisionConfigsAddons":                renderScript(kubernetesCSEConfigAddons, config),
			"provisionSendLogs":                     renderScript(kubernetesCSESendLogs, config),
			"provisionRedactCloudConfig":            renderScript(kubernetesCSERedactCloudConfig, config),
			"customSearchDomainsScript":             renderScript(kubernetesCustomSearchDomainsScript, config),
			"dhcpv6SystemdService":                  renderScript(dhcpv6SystemdService, config),
			"dhcpv6ConfigurationScript":             renderScript(dhcpv6ConfigurationScript, config),
			"kubeletSystemdService":                 renderScript(kubeletSystemdService, config),
			"reconcilePrivateHostsScript":           renderScript(reconcilePrivateHostsScript, config),
			"reconcilePrivateHostsService":          renderScript(reconcilePrivateHostsService, config),
			"ensureNoDupEbtablesScript":             renderScript(ensureNoDupEbtablesScript, config),
			"ensureNoDupEbtablesService":            renderScript(ensureNoDupEbtablesService, config),
			"bindMountScript":                       renderScript(bindMountScript, config),
			"bindMountSystemdService":               renderScript(bindMountSystemdService, config),
			"migPartitionSystemdService":            renderScript(migPartitionSystemdService, config),
			"migPartitionScript":                    renderScript(migPartitionScript, config),
			"ensureIMDSRestrictionScript":           renderScript(ensureIMDSRestrictionScript, config),
			"snapshotUpdateScript":                  renderScript(snapshotUpdateScript, config),
			"snapshotUpdateService":                 renderScript(snapshotUpdateSystemdService, config),
			"snapshotUpdateTimer":                   renderScript(snapshotUpdateSystemdTimer, config),
			"packageUpdateScriptMariner":            renderScript(packageUpdateScriptMariner, config),
			"packageUpdateServiceMariner":           renderScript(packageUpdateSystemdServiceMariner, config),
			"packageUpdateTimerMariner":             renderScript(packageUpdateSystemdTimerMariner, config),
			"componentManifestFile":                 renderScript(componentManifestFile, config),
			"validateKubeletCredentialsScript":      renderScript(validateKubeletCredentialsScript, config),
			"secureTLSBootstrapService":             renderScript(secureTLSBootstrapService, config),
			"cloudInitStatusCheckScript":            renderScript(cloudInitStatusCheckScript, config),
			"measureTLSBootstrappingLatencyScript":  renderScript(measureTLSBootstrappingLatencyScript, config),
			"measureTLSBootstrappingLatencyService": renderScript(measureTLSBootstrappingLatencyService, config),
			"configureAzureNetworkScript":           renderScript(configureAzureNetworkScript, config),
			"azureNetworkUdevRule":                  renderScript(azureNetworkUdevRule, config),
		},
	}

	cloudInitData := cloudInitFiles["cloudInitData"].(paramsMap) //nolint:errcheck // no error is actually here
	cloudInitData["initAKSCloud"] = renderScript(initAKSCloudScript, config)

	if config.IsFlatcar() || config.IsACL() {
		cloudInitData["provisionRedactCloudConfig"] = "" // Flatcar and ACL do not have cloud-init
	}

	if !cs.Properties.IsVHDDistroForAllNodes() {
		cloudInitData["kmsSystemdService"] = renderScript(kmsSystemdService, config)
		cloudInitData["aptPreferences"] = renderScript(aptPreferences, config)
		cloudInitData["dockerClearMountPropagationFlags"] = renderScript(dockerClearMountPropagationFlags, config)
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

	return map[string]interface{}{
		"tenantID":                               config.TenantID,
		"subscriptionId":                         config.SubscriptionID,
		"resourceGroup":                          config.ResourceGroupName,
		"location":                               cs.Location,
		"vmType":                                 cs.Properties.GetVMType(),
		"subnetName":                             cs.Properties.GetSubnetName(),
		"nsgName":                                cs.Properties.GetNSGName(),
		"virtualNetworkName":                     cs.Properties.GetVirtualNetworkName(),
		"virtualNetworkResourceGroupName":        cs.Properties.GetVNetResourceGroupName(),
		"routeTableName":                         cs.Properties.GetRouteTableName(),
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
