//"copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
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
		addValue(parametersMap, "masterSubnet", properties.HostedMasterProfile.Subnet)
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
		addValue(parametersMap, fmt.Sprintf("%sCount", agentProfile.Name), agentProfile.Count)
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

		// Unless distro is defined, default distro is configured by defaults#setAgentProfileDefaults
		//   Ignores Windows OS
		if !(agentProfile.OSType == datamodel.Windows) {
			if agentProfile.ImageRef != nil {
				addValue(parametersMap, fmt.Sprintf("%sosImageName", agentProfile.Name), agentProfile.ImageRef.Name)
				addValue(parametersMap, fmt.Sprintf("%sosImageResourceGroup", agentProfile.Name), agentProfile.ImageRef.ResourceGroup)
			}
			addValue(parametersMap, fmt.Sprintf("%sosImageOffer", agentProfile.Name), cloudSpecConfig.OSImageConfig[datamodel.Distro(agentProfile.Distro)].ImageOffer)
			addValue(parametersMap, fmt.Sprintf("%sosImageSKU", agentProfile.Name), cloudSpecConfig.OSImageConfig[datamodel.Distro(agentProfile.Distro)].ImageSku)
			addValue(parametersMap, fmt.Sprintf("%sosImagePublisher", agentProfile.Name), cloudSpecConfig.OSImageConfig[datamodel.Distro(agentProfile.Distro)].ImagePublisher)
			addValue(parametersMap, fmt.Sprintf("%sosImageVersion", agentProfile.Name), cloudSpecConfig.OSImageConfig[datamodel.Distro(agentProfile.Distro)].ImageVersion)
		} else {
			// Set ImageRef if it is not nil and always set the Windows VHD information in WindowsProfile.
			// ImageRef will be used to generate ARM template for the agent pool if it is set.
			// Otherwise, the Windows VHD information in WindowsProfile will be used to generate ARM template.
			// Priority:
			//   1. ImageRef in agent pool
			//   2. ImageRef in WindowsProfile
			//   3. PIR image in WindowsProfile
			if agentProfile.ImageRef != nil {
				addValue(parametersMap, fmt.Sprintf("%sosImageName", agentProfile.Name), agentProfile.ImageRef.Name)
				addValue(parametersMap, fmt.Sprintf("%sosImageResourceGroup", agentProfile.Name), agentProfile.ImageRef.ResourceGroup)
			} else if properties.WindowsProfile.HasImageRef() {
				addValue(parametersMap, fmt.Sprintf("%sosImageName", agentProfile.Name), properties.WindowsProfile.ImageRef.Name)
				addValue(parametersMap, fmt.Sprintf("%sosImageResourceGroup", agentProfile.Name), properties.WindowsProfile.ImageRef.ResourceGroup)
			}
			addValue(parametersMap, fmt.Sprintf("%sosImageOffer", agentProfile.Name), properties.WindowsProfile.WindowsOffer)
			addValue(parametersMap, fmt.Sprintf("%sosImageSKU", agentProfile.Name), properties.WindowsProfile.GetWindowsSku())
			addValue(parametersMap, fmt.Sprintf("%sosImagePublisher", agentProfile.Name), properties.WindowsProfile.WindowsPublisher)
			addValue(parametersMap, fmt.Sprintf("%sosImageVersion", agentProfile.Name), properties.WindowsProfile.ImageVersion)
		}

		if !isSetVnetCidrs && properties.HostedMasterProfile != nil && len(agentProfile.VnetCidrs) != 0 {
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
		addValue(parametersMap, "windowsAdminUsername", properties.WindowsProfile.AdminUsername)
		addSecret(parametersMap, "windowsAdminPassword", properties.WindowsProfile.AdminPassword, false)

		if properties.WindowsProfile.HasCustomImage() {
			addValue(parametersMap, "agentWindowsSourceUrl", properties.WindowsProfile.WindowsImageSourceURL)
		} else if properties.WindowsProfile.HasImageRef() {
			addValue(parametersMap, "agentWindowsImageResourceGroup", properties.WindowsProfile.ImageRef.ResourceGroup)
			addValue(parametersMap, "agentWindowsImageName", properties.WindowsProfile.ImageRef.Name)
		} else {
			addValue(parametersMap, "agentWindowsPublisher", properties.WindowsProfile.WindowsPublisher)
			addValue(parametersMap, "agentWindowsOffer", properties.WindowsProfile.WindowsOffer)
			addValue(parametersMap, "agentWindowsSku", properties.WindowsProfile.GetWindowsSku())
			addValue(parametersMap, "agentWindowsVersion", properties.WindowsProfile.ImageVersion)

		}

		addValue(parametersMap, "windowsDockerVersion", properties.WindowsProfile.GetWindowsDockerVersion())

		for i, s := range properties.WindowsProfile.Secrets {
			addValue(parametersMap, fmt.Sprintf("windowsKeyVaultID%d", i), s.SourceVault.ID)
			for j, c := range s.VaultCertificates {
				addValue(parametersMap, fmt.Sprintf("windowsKeyVaultID%dCertificateURL%d", i, j), c.CertificateURL)
				addValue(parametersMap, fmt.Sprintf("windowsKeyVaultID%dCertificateStore%d", i, j), c.CertificateStore)
			}
		}
		addValue(parametersMap, "defaultContainerdRuntimeHandler", properties.WindowsProfile.GetWindowsDefaultRuntimeHandler())
		addValue(parametersMap, "hypervRuntimeHandlers", properties.WindowsProfile.GetWindowsHypervRuntimeHandlers())
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
	if profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime != "" {
		// override containerRuntime parameter value if specified in AgentPoolProfile
		// this allows for heteregenous clusters
		addValue(parametersMap, "containerRuntime", profile.KubernetesConfig.ContainerRuntime)
		if profile.KubernetesConfig.ContainerRuntime == "containerd" {
			addValue(parametersMap, "cliTool", "ctr")
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
			addValue(parametersMap, "etcdDownloadURLBase", cloudSpecConfig.KubernetesSpecConfig.EtcdDownloadURLBase)
			addValue(parametersMap, "etcdVersion", kubernetesConfig.EtcdVersion)
			addValue(parametersMap, "etcdDiskSizeGB", kubernetesConfig.EtcdDiskSizeGB)
			addValue(parametersMap, "etcdEncryptionKey", kubernetesConfig.EtcdEncryptionKey)

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
			keyVaultSecretRef := servicePrincipalProfile.KeyvaultSecretRef
			if keyVaultSecretRef != nil {
				addKeyvaultReference(parametersMap, "servicePrincipalClientSecret",
					keyVaultSecretRef.VaultID,
					keyVaultSecretRef.SecretName,
					keyVaultSecretRef.SecretVersion)
			} else {
				addValue(parametersMap, "servicePrincipalClientSecret", servicePrincipalProfile.Secret)
			}

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
		 - apiServerPrivateKey
		 - caCertificate
		 - clientCertificate
		 - clientPrivateKey
		 - kubeConfigCertificate
		 - kubeConfigPrivateKey
		 - servicePrincipalClientSecret
		 - etcdClientCertificate
		 - etcdClientPrivateKey
		 - etcdServerCertificate
		 - etcdServerPrivateKey
		 - etcdPeerCertificates
		 - etcdPeerPrivateKeys

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
			addSecret(parametersMap, "apiServerPrivateKey", certificateProfile.APIServerPrivateKey, true)
			addSecret(parametersMap, "caCertificate", certificateProfile.CaCertificate, true)
			addSecret(parametersMap, "caPrivateKey", certificateProfile.CaPrivateKey, true)
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
