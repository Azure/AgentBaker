//"copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
)

func getParameters(config *datamodel.NodeBootstrappingConfiguration, generatorCode string, bakerVersion string) paramsMap {
	cs := config.ContainerService
	profile := config.AgentPoolProfile
	properties := cs.Properties
	location := cs.Location
	parametersMap := paramsMap{}
	cloudSpecConfig := config.CloudSpecConfig

	addValue(parametersMap, "bakerVersion", bakerVersion)
	addValue(parametersMap, "location", location)

	addValue(parametersMap, "nameSuffix", cs.Properties.GetClusterID())
	addValue(parametersMap, "targetEnvironment", GetCloudTargetEnv(cs.Location))
	linuxProfile := properties.LinuxProfile
	if linuxProfile != nil {
		addValue(parametersMap, "linuxAdminUsername", linuxProfile.AdminUsername)
		if linuxProfile.CustomNodesDNS != nil {
			addValue(parametersMap, "dnsServer", linuxProfile.CustomNodesDNS.DNSServer)
		}
	}
	// masterEndpointDNSNamePrefix is the basis for storage account creation across dcos, swarm, and k8s
	if properties.HostedMasterProfile != nil {
		// Agents only, use cluster DNS prefix
		addValue(parametersMap, "masterEndpointDNSNamePrefix", properties.HostedMasterProfile.DNSPrefix)
	}
	if properties.HostedMasterProfile != nil {
		addValue(parametersMap, "vnetCidr", DefaultVNETCIDR)
	}

	if linuxProfile != nil {
		addValue(parametersMap, "sshRSAPublicKey", linuxProfile.SSH.PublicKeys[0].KeyData)
		for i, s := range linuxProfile.Secrets {
			addValue(parametersMap, fmt.Sprintf("linuxKeyVaultID%d", i), s.SourceVault.ID)
			for j, c := range s.VaultCertificates {
				addValue(parametersMap, fmt.Sprintf("linuxKeyVaultID%dCertificateURL%d", i, j), c.CertificateURL)
			}
		}
	}

	// Kubernetes Parameters
	if properties.OrchestratorProfile.IsKubernetes() {
		assignKubernetesParameters(properties, parametersMap, cloudSpecConfig, config.K8sComponents, generatorCode)
		if profile != nil {
			assignKubernetesParametersFromAgentProfile(profile, parametersMap, cloudSpecConfig, generatorCode, config)
		}
	}

	// Agent parameters
	isSetVnetCidrs := false
	for _, agentProfile := range properties.AgentPoolProfiles {
		addValue(parametersMap, fmt.Sprintf("%sVMSize", agentProfile.Name), agentProfile.VMSize)
		if agentProfile.HasAvailabilityZones() {
			addValue(parametersMap, fmt.Sprintf("%sAvailabilityZones", agentProfile.Name), agentProfile.AvailabilityZones)
		}
		if agentProfile.IsCustomVNET() {
			addValue(parametersMap, fmt.Sprintf("%sVnetSubnetID", agentProfile.Name), agentProfile.VnetSubnetID)
		} else {
			addValue(parametersMap, fmt.Sprintf("%sSubnet", agentProfile.Name), agentProfile.Subnet)
		}
		if len(agentProfile.Ports) > 0 {
			addValue(parametersMap, fmt.Sprintf("%sEndpointDNSNamePrefix", agentProfile.Name), agentProfile.DNSPrefix)
		}

		if !agentProfile.IsAvailabilitySets() && agentProfile.IsSpotScaleSet() {
			addValue(parametersMap, fmt.Sprintf("%sScaleSetPriority", agentProfile.Name), agentProfile.ScaleSetPriority)
			addValue(parametersMap, fmt.Sprintf("%sScaleSetEvictionPolicy", agentProfile.Name), agentProfile.ScaleSetEvictionPolicy)
		}

		if !isSetVnetCidrs && len(agentProfile.VnetCidrs) != 0 {
			// For AKS (properties.HostedMasterProfile != nil), set vnetCidr if a custom vnet is used so the address space can be
			// added into the ExceptionList of Windows nodes. Otherwise, the default value `10.0.0.0/8` will
			// be added into the ExceptionList and it does not work if users use other ip address ranges.
			// All agent pools in the same cluster share a same VnetCidrs so we only need to set the first non-empty VnetCidrs.
			addValue(parametersMap, "vnetCidr", strings.Join(agentProfile.VnetCidrs, ","))
			isSetVnetCidrs = true
		}
	}

	// Windows parameters
	if properties.HasWindows() {
		addValue(parametersMap, "windowsDockerVersion", properties.WindowsProfile.GetWindowsDockerVersion())
		addValue(parametersMap, "defaultContainerdWindowsSandboxIsolation", properties.WindowsProfile.GetDefaultContainerdWindowsSandboxIsolation())
		addValue(parametersMap, "containerdWindowsRuntimeHandlers", properties.WindowsProfile.GetContainerdWindowsRuntimeHandlers())
	}

	for _, extension := range properties.ExtensionProfiles {
		if extension.ExtensionParametersKeyVaultRef != nil {
			addKeyvaultReference(parametersMap, fmt.Sprintf("%sParameters", extension.Name),
				extension.ExtensionParametersKeyVaultRef.VaultID,
				extension.ExtensionParametersKeyVaultRef.SecretName,
				extension.ExtensionParametersKeyVaultRef.SecretVersion)
		} else {
			addValue(parametersMap, fmt.Sprintf("%sParameters", extension.Name), extension.ExtensionParameters)
		}
	}

	return parametersMap
}

func assignKubernetesParametersFromAgentProfile(profile *datamodel.AgentPoolProfile, parametersMap paramsMap,
	cloudSpecConfig *datamodel.AzureEnvironmentSpecConfig, generatorCode string, config *datamodel.NodeBootstrappingConfiguration) {
	if config.RuncVersion != "" {
		addValue(parametersMap, "runcVersion", config.RuncVersion)
	}
	if profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime != "" {
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
		} else {
			addValue(parametersMap, "cliTool", "docker")
		}
	}
}

func assignKubernetesParameters(properties *datamodel.Properties, parametersMap paramsMap,
	cloudSpecConfig *datamodel.AzureEnvironmentSpecConfig,
	k8sComponents *datamodel.K8sComponents,
	generatorCode string) {
	addValue(parametersMap, "generatorCode", generatorCode)

	orchestratorProfile := properties.OrchestratorProfile

	if orchestratorProfile.IsKubernetes() {
		k8sVersion := orchestratorProfile.OrchestratorVersion
		addValue(parametersMap, "kubernetesVersion", k8sVersion)

		kubernetesConfig := orchestratorProfile.KubernetesConfig

		if kubernetesConfig != nil {
			if kubernetesConfig.CustomKubeProxyImage != "" {
				addValue(parametersMap, "kubeProxySpec", kubernetesConfig.CustomKubeProxyImage)
			}

			if kubernetesConfig.CustomKubeBinaryURL != "" {
				addValue(parametersMap, "kubeBinaryURL", kubernetesConfig.CustomKubeBinaryURL)
			}

			addValue(parametersMap, "kubernetesHyperkubeSpec", k8sComponents.HyperkubeImageURL)

			addValue(parametersMap, "kubeDNSServiceIP", kubernetesConfig.DNSServiceIP)
			if kubernetesConfig.IsAADPodIdentityEnabled() {
				aadPodIdentityAddon := kubernetesConfig.GetAddonByName(AADPodIdentityAddonName)
				aadIndex := aadPodIdentityAddon.GetAddonContainersIndexByName(AADPodIdentityAddonName)
				if aadIndex > -1 {
					addValue(parametersMap, "kubernetesAADPodIdentityEnabled", to.Bool(aadPodIdentityAddon.Enabled))
				}
			}
			if kubernetesConfig.IsAddonEnabled(ACIConnectorAddonName) {
				addValue(parametersMap, "kubernetesACIConnectorEnabled", true)
			} else {
				addValue(parametersMap, "kubernetesACIConnectorEnabled", false)
			}
			addValue(parametersMap, "kubernetesPodInfraContainerSpec", k8sComponents.PodInfraContainerImageURL)
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
			addValue(parametersMap, "cniPluginsURL", cloudSpecConfig.KubernetesSpecConfig.CNIPluginsDownloadURL)
			addValue(parametersMap, "vnetCniLinuxPluginsURL", kubernetesConfig.GetAzureCNIURLLinux(cloudSpecConfig))
			addValue(parametersMap, "vnetCniWindowsPluginsURL", kubernetesConfig.GetAzureCNIURLWindows(cloudSpecConfig))
			addValue(parametersMap, "gchighthreshold", kubernetesConfig.GCHighThreshold)
			addValue(parametersMap, "gclowthreshold", kubernetesConfig.GCLowThreshold)

			addValue(parametersMap, "enableAggregatedAPIs", kubernetesConfig.EnableAggregatedAPIs)

			if properties.HasWindows() {
				addValue(parametersMap, "kubeBinariesSASURL", k8sComponents.WindowsPackageURL)

				// Kubernetes node binaries as packaged by upstream kubernetes
				// example at https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG-1.11.md#node-binaries-1
				addValue(parametersMap, "windowsKubeBinariesURL", kubernetesConfig.WindowsNodeBinariesURL)
				addValue(parametersMap, "windowsContainerdURL", kubernetesConfig.WindowsContainerdURL)
				addValue(parametersMap, "kubeServiceCidr", kubernetesConfig.ServiceCIDR)
				addValue(parametersMap, "kubeBinariesVersion", k8sVersion)
				addValue(parametersMap, "windowsTelemetryGUID", cloudSpecConfig.KubernetesSpecConfig.WindowsTelemetryGUID)
				addValue(parametersMap, "windowsSdnPluginURL", kubernetesConfig.WindowsSdnPluginURL)

			}
		}

		servicePrincipalProfile := properties.ServicePrincipalProfile

		if servicePrincipalProfile != nil {
			addValue(parametersMap, "servicePrincipalClientId", servicePrincipalProfile.ClientID)
			encodedServicePrincipalClientSecret := base64.StdEncoding.EncodeToString([]byte(servicePrincipalProfile.Secret))
			addValue(parametersMap, "servicePrincipalClientSecret", servicePrincipalProfile.Secret)
			// base64 encoding is to escape special characters like quotes in service principal
			// reference: https://github.com/Azure/aks-engine/pull/1174
			addValue(parametersMap, "encodedServicePrincipalClientSecret", encodedServicePrincipalClientSecret)

			if kubernetesConfig != nil && to.Bool(kubernetesConfig.EnableEncryptionWithExternalKms) {
				if kubernetesConfig.KeyVaultSku != "" {
					addValue(parametersMap, "clusterKeyVaultSku", kubernetesConfig.KeyVaultSku)
				}
				if !kubernetesConfig.UseManagedIdentity && servicePrincipalProfile.ObjectID != "" {
					addValue(parametersMap, "servicePrincipalObjectId", servicePrincipalProfile.ObjectID)
				}
			}
		}

		addValue(parametersMap, "orchestratorName", properties.K8sOrchestratorName())

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
			addSecret(parametersMap, "kubeConfigCertificate", certificateProfile.KubeConfigCertificate, true)
			addSecret(parametersMap, "kubeConfigPrivateKey", certificateProfile.KubeConfigPrivateKey, true)
		}

		if properties.AADProfile != nil {
			addValue(parametersMap, "aadTenantId", properties.AADProfile.TenantID)
			if properties.AADProfile.AdminGroupID != "" {
				addValue(parametersMap, "aadAdminGroupId", properties.AADProfile.AdminGroupID)
			}
		}

		if kubernetesConfig != nil && kubernetesConfig.IsAddonEnabled(AppGwIngressAddonName) {
			addValue(parametersMap, "appGwSku", kubernetesConfig.GetAddonByName(AppGwIngressAddonName).Config["appgw-sku"])
			addValue(parametersMap, "appGwSubnet", kubernetesConfig.GetAddonByName(AppGwIngressAddonName).Config["appgw-subnet"])
		}
	}
}
