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
	return getCustomDataVariablesWithEncoder(config, usePlainCloudInit(config))
}

func getCustomDataVariablesWithEncoder(config *datamodel.NodeBootstrappingConfiguration, plain bool) paramsMap {
	encodeScript := getBase64EncodedGzippedCustomScript
	if plain {
		encodeScript = getCustomScript
	}
	cs := config.ContainerService
	cloudInitFiles := map[string]interface{}{
		"cloudInitData": paramsMap{
			"provisionStartScript":                  encodeScript(kubernetesCSEStartScript, config),
			"provisionScript":                       encodeScript(kubernetesCSEMainScript, config),
			"provisionSource":                       encodeScript(kubernetesCSEHelpersScript, config),
			"provisionSourceUbuntu":                 encodeScript(kubernetesCSEHelpersScriptUbuntu, config),
			"provisionSourceMariner":                encodeScript(kubernetesCSEHelpersScriptMariner, config),
			"provisionSourceAzlOSGuard":             encodeScript(kubernetesCSEHelpersScriptAzlOSGuard, config),
			"provisionSourceFlatcar":                encodeScript(kubernetesCSEHelpersScriptFlatcar, config),
			"provisionSourceACL":                    encodeScript(kubernetesCSEHelpersScriptACL, config),
			"provisionInstalls":                     encodeScript(kubernetesCSEInstall, config),
			"provisionInstallsUbuntu":               encodeScript(kubernetesCSEInstallUbuntu, config),
			"provisionInstallsMariner":              encodeScript(kubernetesCSEInstallMariner, config),
			"provisionInstallsAzlOSGuard":           encodeScript(kubernetesCSEInstallAzlOSGuard, config),
			"provisionInstallsFlatcar":              encodeScript(kubernetesCSEInstallFlatcar, config),
			"provisionInstallsACL":                  encodeScript(kubernetesCSEInstallACL, config),
			"provisionConfigs":                      encodeScript(kubernetesCSEConfig, config),
			"provisionConfigsGPU":                   encodeScript(kubernetesCSEConfigGPU, config),
			"provisionConfigsLocalDNS":              encodeScript(kubernetesCSEConfigLocalDNS, config),
			"provisionConfigsKubelet":               encodeScript(kubernetesCSEConfigKubelet, config),
			"provisionConfigsNetwork":               encodeScript(kubernetesCSEConfigNetwork, config),
			"provisionConfigsAddons":                encodeScript(kubernetesCSEConfigAddons, config),
			"provisionConfigsChrony":                encodeScript(kubernetesCSEConfigChrony, config),
			"provisionSendLogs":                     encodeScript(kubernetesCSESendLogs, config),
			"provisionRedactCloudConfig":            encodeScript(kubernetesCSERedactCloudConfig, config),
			"customSearchDomainsScript":             encodeScript(kubernetesCustomSearchDomainsScript, config),
			"dhcpv6SystemdService":                  encodeScript(dhcpv6SystemdService, config),
			"dhcpv6ConfigurationScript":             encodeScript(dhcpv6ConfigurationScript, config),
			"kubeletSystemdService":                 encodeScript(kubeletSystemdService, config),
			"reconcilePrivateHostsScript":           encodeScript(reconcilePrivateHostsScript, config),
			"reconcilePrivateHostsService":          encodeScript(reconcilePrivateHostsService, config),
			"ensureNoDupEbtablesScript":             encodeScript(ensureNoDupEbtablesScript, config),
			"ensureNoDupEbtablesService":            encodeScript(ensureNoDupEbtablesService, config),
			"bindMountScript":                       encodeScript(bindMountScript, config),
			"bindMountSystemdService":               encodeScript(bindMountSystemdService, config),
			"migPartitionSystemdService":            encodeScript(migPartitionSystemdService, config),
			"migPartitionScript":                    encodeScript(migPartitionScript, config),
			"ensureIMDSRestrictionScript":           encodeScript(ensureIMDSRestrictionScript, config),
			"snapshotUpdateScript":                  encodeScript(snapshotUpdateScript, config),
			"snapshotUpdateService":                 encodeScript(snapshotUpdateSystemdService, config),
			"snapshotUpdateTimer":                   encodeScript(snapshotUpdateSystemdTimer, config),
			"packageUpdateScriptMariner":            encodeScript(packageUpdateScriptMariner, config),
			"packageUpdateServiceMariner":           encodeScript(packageUpdateSystemdServiceMariner, config),
			"packageUpdateTimerMariner":             encodeScript(packageUpdateSystemdTimerMariner, config),
			"componentManifestFile":                 encodeScript(componentManifestFile, config),
			"validateKubeletCredentialsScript":      encodeScript(validateKubeletCredentialsScript, config),
			"secureTLSBootstrapService":             encodeScript(secureTLSBootstrapService, config),
			"cloudInitStatusCheckScript":            encodeScript(cloudInitStatusCheckScript, config),
			"measureTLSBootstrappingLatencyScript":  encodeScript(measureTLSBootstrappingLatencyScript, config),
			"measureTLSBootstrappingLatencyService": encodeScript(measureTLSBootstrappingLatencyService, config),
			"configureAzureNetworkScript":           encodeScript(configureAzureNetworkScript, config),
			"azureNetworkUdevRule":                  encodeScript(azureNetworkUdevRule, config),
		},
	}

	cloudInitData := cloudInitFiles["cloudInitData"].(paramsMap) //nolint:errcheck // no error is actually here
	cloudInitData["initAKSCloud"] = encodeScript(initAKSCloudScript, config)

	if config.IsFlatcar() || config.IsACL() {
		cloudInitData["provisionRedactCloudConfig"] = "" // Flatcar and ACL do not have cloud-init
	}

	if !cs.Properties.IsVHDDistroForAllNodes() {
		cloudInitData["kmsSystemdService"] = encodeScript(kmsSystemdService, config)
		cloudInitData["aptPreferences"] = encodeScript(aptPreferences, config)
		cloudInitData["dockerClearMountPropagationFlags"] = encodeScript(dockerClearMountPropagationFlags, config)
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
