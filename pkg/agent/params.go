// "copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func getParameters(config *datamodel.NodeBootstrappingConfiguration) paramsMap {
	cs := config.ContainerService
	profile := config.AgentPoolProfile
	properties := cs.Properties
	parametersMap := paramsMap{}
	cloudSpecConfig := config.CloudSpecConfig

	linuxProfile := properties.LinuxProfile
	if linuxProfile != nil {
		addValue(parametersMap, "linuxAdminUsername", linuxProfile.AdminUsername)
	}
	// masterEndpointDNSNamePrefix is the basis for storage account creation across dcos, swarm, and k8s
	// looks like masterEndpointDNSNamePrefix is only used in windows cse kubeconfig cluster/context name and it's not
	// required since linux uses static value for that.
	if properties.HostedMasterProfile != nil {
		// Agents only, use cluster DNS prefix
		switch {
		case properties.HostedMasterProfile.DNSPrefix != "":
			addValue(parametersMap, "masterEndpointDNSNamePrefix", properties.HostedMasterProfile.DNSPrefix)
		case properties.HostedMasterProfile.FQDNSubdomain != "":
			addValue(parametersMap, "masterEndpointDNSNamePrefix", properties.HostedMasterProfile.FQDNSubdomain)
		default:
			// should not happen but just in case, we fill in value "localcluster" just like linux
			addValue(parametersMap, "masterEndpointDNSNamePrefix", "localcluster")
		}
	}
	if properties.HostedMasterProfile != nil {
		addValue(parametersMap, "vnetCidr", DefaultVNETCIDR)
	}

	// Kubernetes Parameters
	if properties.OrchestratorProfile.IsKubernetes() {
		assignKubernetesParameters(properties, parametersMap, cloudSpecConfig, config.K8sComponents, config)
		if profile != nil {
			assignKubernetesParametersFromAgentProfile(profile, parametersMap, config)
		}
	}

	// Agent parameters
	// We should always use profile to remove the dependency on cs.Properties.AgentPoolProfiles since RP always set
	// config.AgentPoolProfile to the target agent pool
	if len(profile.VnetCidrs) != 0 {
		// For AKS (properties.HostedMasterProfile != nil), set vnetCidr if a custom vnet is used so the address space can be
		// added into the ExceptionList of Windows nodes. Otherwise, the default value `10.0.0.0/8` will
		// be added into the ExceptionList and it does not work if users use other ip address ranges.
		// All agent pools in the same cluster share a same VnetCidrs
		addValue(parametersMap, "vnetCidr", strings.Join(profile.VnetCidrs, ","))
	}

	if properties.CustomConfiguration != nil && properties.CustomConfiguration.KubernetesConfigurations != nil {
		if configuration, ok := properties.CustomConfiguration.KubernetesConfigurations["kubelet"]; ok && configuration.DownloadURL != nil {
			addValue(parametersMap, "customKubeBinaryURL", configuration.DownloadURL)
		}
	}

	// Windows parameters
	if properties.HasWindows() {
		addValue(parametersMap, "windowsDockerVersion", properties.WindowsProfile.GetWindowsDockerVersion())
		addValue(parametersMap, "defaultContainerdWindowsSandboxIsolation", properties.WindowsProfile.GetDefaultContainerdWindowsSandboxIsolation())
		addValue(parametersMap, "containerdWindowsRuntimeHandlers", properties.WindowsProfile.GetContainerdWindowsRuntimeHandlers())
	}

	return parametersMap
}

func assignKubernetesParametersFromAgentProfile(profile *datamodel.AgentPoolProfile, parametersMap paramsMap,
	config *datamodel.NodeBootstrappingConfiguration) {
	if config.RuncVersion != "" {
		addValue(parametersMap, "runcVersion", config.RuncVersion)
	}
	addValue(parametersMap, "runcPackageURL", config.RuncPackageURL)
	if profile.KubernetesConfig == nil || profile.KubernetesConfig.ContainerRuntime == "" {
		return
	}
	// override containerRuntime parameter value if specified in AgentPoolProfile
	// this allows for heteregenous clusters
	addValue(parametersMap, "containerRuntime", profile.KubernetesConfig.ContainerRuntime)
	if profile.KubernetesConfig.ContainerRuntime == "containerd" {
		addValue(parametersMap, "cliTool", "ctr")
		if config.ContainerdVersion != "" {
			addValue(parametersMap, "containerdVersion", config.ContainerdVersion)
		}
		if config.TeleportdPluginURL != "" {
			addValue(parametersMap, "teleportdPluginURL", config.TeleportdPluginURL)
		}
		addValue(parametersMap, "containerdPackageURL", config.ContainerdPackageURL)
	} else {
		addValue(parametersMap, "cliTool", "docker")
	}
}

func assignKubernetesParametersfromKubernetesConfig(properties *datamodel.Properties, parametersMap paramsMap,
	cloudSpecConfig *datamodel.AzureEnvironmentSpecConfig,
	k8sComponents *datamodel.K8sComponents,
	config *datamodel.NodeBootstrappingConfiguration) {
	orchestratorProfile := properties.OrchestratorProfile

	if !orchestratorProfile.IsKubernetes() {
		return
	}

	k8sVersion := orchestratorProfile.OrchestratorVersion
	addValue(parametersMap, "kubernetesVersion", k8sVersion)

	kubernetesConfig := orchestratorProfile.KubernetesConfig

	if kubernetesConfig == nil {
		return
	}

	if kubernetesConfig.CustomKubeProxyImage != "" {
		// kubernetesConfig.CustomKubeProxyImage is ap level property, AKS default CustomKubeProxyImage
		// is 'multi-arch', no need to differentiate amd64/arm64 ap
		addValue(parametersMap, "kubeProxySpec", kubernetesConfig.CustomKubeProxyImage)
	}

	if kubernetesConfig.CustomKubeBinaryURL != "" {
		// kubernetesConfig.CustomKubeBinaryURL is ap level property, CustomKubeBinaryURL is
		// set to different for amd64/arm64 ap in RP side.
		addValue(parametersMap, "kubeBinaryURL", kubernetesConfig.CustomKubeBinaryURL)
	}

	addValue(parametersMap, "kubernetesHyperkubeSpec", k8sComponents.HyperkubeImageURL)

	addValue(parametersMap, "kubeDNSServiceIP", kubernetesConfig.DNSServiceIP)
	addValue(parametersMap, "cloudproviderConfig", paramsMap{
		"cloudProviderBackoffMode":          kubernetesConfig.CloudProviderBackoffMode,
		"cloudProviderBackoff":              kubernetesConfig.CloudProviderBackoff,
		"cloudProviderBackoffRetries":       kubernetesConfig.CloudProviderBackoffRetries,
		"cloudProviderBackoffJitter":        strconv.FormatFloat(kubernetesConfig.CloudProviderBackoffJitter, 'f', -1, 64),
		"cloudProviderBackoffDuration":      kubernetesConfig.CloudProviderBackoffDuration,
		"cloudProviderBackoffExponent":      strconv.FormatFloat(kubernetesConfig.CloudProviderBackoffExponent, 'f', -1, 64),
		"cloudProviderRateLimit":            kubernetesConfig.CloudProviderRateLimit,
		"cloudProviderRateLimitQPS":         strconv.FormatFloat(kubernetesConfig.CloudProviderRateLimitQPS, 'f', -1, 64),
		"cloudProviderRateLimitQPSWrite":    strconv.FormatFloat(kubernetesConfig.CloudProviderRateLimitQPSWrite, 'f', -1, 64),
		"cloudProviderRateLimitBucket":      kubernetesConfig.CloudProviderRateLimitBucket,
		"cloudProviderRateLimitBucketWrite": kubernetesConfig.CloudProviderRateLimitBucketWrite,
		"cloudProviderDisableOutboundSNAT":  kubernetesConfig.CloudProviderDisableOutboundSNAT,
	})
	addValue(parametersMap, "kubeClusterCidr", kubernetesConfig.ClusterSubnet)
	addValue(parametersMap, "dockerBridgeCidr", kubernetesConfig.DockerBridgeSubnet)
	addValue(parametersMap, "networkPolicy", kubernetesConfig.NetworkPolicy)
	addValue(parametersMap, "networkPlugin", kubernetesConfig.NetworkPlugin)
	addValue(parametersMap, "networkMode", kubernetesConfig.NetworkMode)
	addValue(parametersMap, "containerRuntime", kubernetesConfig.ContainerRuntime)
	addValue(parametersMap, "containerdDownloadURLBase", cloudSpecConfig.KubernetesSpecConfig.ContainerdDownloadURLBase)
	if config.IsARM64 {
		addValue(parametersMap, "vnetCniLinuxPluginsURL", kubernetesConfig.GetAzureCNIURLARM64Linux(cloudSpecConfig))
	} else {
		addValue(parametersMap, "vnetCniLinuxPluginsURL", kubernetesConfig.GetAzureCNIURLLinux(cloudSpecConfig))
	}
	addValue(parametersMap, "vnetCniWindowsPluginsURL", kubernetesConfig.GetAzureCNIURLWindows(cloudSpecConfig))
	addValue(parametersMap, "linuxCredentialProviderURL", k8sComponents.LinuxCredentialProviderURL)

	if properties.HasWindows() {
		addValue(parametersMap, "kubeBinariesSASURL", k8sComponents.WindowsPackageURL)
		addValue(parametersMap, "windowsContainerdURL", kubernetesConfig.WindowsContainerdURL)
		addValue(parametersMap, "kubeServiceCidr", kubernetesConfig.ServiceCIDR)
		addValue(parametersMap, "kubeBinariesVersion", k8sVersion)
		addValue(parametersMap, "windowsTelemetryGUID", cloudSpecConfig.KubernetesSpecConfig.WindowsTelemetryGUID)
		addValue(parametersMap, "windowsSdnPluginURL", kubernetesConfig.WindowsSdnPluginURL)
		addValue(parametersMap, "windowsCredentialProviderURL", k8sComponents.WindowsCredentialProviderURL)
	}
}

func assignKubernetesParameters(properties *datamodel.Properties, parametersMap paramsMap,
	cloudSpecConfig *datamodel.AzureEnvironmentSpecConfig,
	k8sComponents *datamodel.K8sComponents,
	config *datamodel.NodeBootstrappingConfiguration) {
	orchestratorProfile := properties.OrchestratorProfile

	if orchestratorProfile.IsKubernetes() {
		k8sVersion := orchestratorProfile.OrchestratorVersion
		addValue(parametersMap, "kubernetesVersion", k8sVersion)

		assignKubernetesParametersfromKubernetesConfig(properties, parametersMap, cloudSpecConfig, k8sComponents, config)

		servicePrincipalProfile := properties.ServicePrincipalProfile

		if servicePrincipalProfile != nil {
			addValue(parametersMap, "servicePrincipalClientId", servicePrincipalProfile.ClientID)
			encodedServicePrincipalClientSecret := base64.StdEncoding.EncodeToString([]byte(servicePrincipalProfile.Secret))
			addValue(parametersMap, "servicePrincipalClientSecret", servicePrincipalProfile.Secret)
			// base64 encoding is to escape special characters like quotes in service principal
			// reference: https://github.com/Azure/aks-engine/pull/1174
			addValue(parametersMap, "encodedServicePrincipalClientSecret", encodedServicePrincipalClientSecret)
		}

		/**
		 The following parameters could be either a plain text, or referenced to a secret in a keyvault:
		 - apiServerCertificate
		 - clientCertificate
		 - clientPrivateKey
		 - kubeConfigCertificate
		 - kubeConfigPrivateKey
		 - servicePrincipalClientSecret

		 To refer to a keyvault secret, the value of the parameter in the api model file should be formatted as:

		 "<PARAMETER>": "/subscriptions/<SUB_ID>/resourceGroups/<RG_NAME>/providers/Microsoft.KeyVault/vaults/<KV_NAME>/secrets/<NAME>[/<VERSION>]"
		 where:
		   <SUB_ID> is the subscription ID of the keyvault
		   <RG_NAME> is the resource group of the keyvault
		   <KV_NAME> is the name of the keyvault
		   <NAME> is the name of the secret.
		   <VERSION> (optional) is the version of the secret (default: the latest version)

		 This will generate a reference block in the parameters file:

		 "reference": {
		   "keyVault": {
		     "id": "/subscriptions/<SUB_ID>/resourceGroups/<RG_NAME>/providers/Microsoft.KeyVault/vaults/<KV_NAME>"
		   },
		   "secretName": "<NAME>"
		   "secretVersion": "<VERSION>"
		}
		**/

		certificateProfile := properties.CertificateProfile
		if certificateProfile != nil {
			addSecret(parametersMap, "apiServerCertificate", certificateProfile.APIServerCertificate, true)
			addSecret(parametersMap, "caCertificate", certificateProfile.CaCertificate, true)
			addSecret(parametersMap, "clientCertificate", certificateProfile.ClientCertificate, true)
			addSecret(parametersMap, "clientPrivateKey", certificateProfile.ClientPrivateKey, true)
		}
	}
}
