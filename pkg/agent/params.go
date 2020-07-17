//"copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"fmt"
	"strconv"

	"github.com/Azure/go-autorest/autorest/to"

	"github.com/Azure/aks-engine/pkg/api"
)

func getParameters(cs *api.ContainerService, profile *api.AgentPoolProfile, generatorCode string, bakerVersion string) paramsMap {
	properties := cs.Properties
	location := cs.Location
	parametersMap := paramsMap{}
	cloudSpecConfig := cs.GetCloudSpecConfig()

	addValue(parametersMap, "bakerVersion", bakerVersion)
	addValue(parametersMap, "location", location)

	// Identify Master distro
	if properties.MasterProfile != nil {
		addValue(parametersMap, "osImageOffer", cloudSpecConfig.OSImageConfig[properties.MasterProfile.Distro].ImageOffer)
		addValue(parametersMap, "osImageSKU", cloudSpecConfig.OSImageConfig[properties.MasterProfile.Distro].ImageSku)
		addValue(parametersMap, "osImagePublisher", cloudSpecConfig.OSImageConfig[properties.MasterProfile.Distro].ImagePublisher)
		addValue(parametersMap, "osImageVersion", cloudSpecConfig.OSImageConfig[properties.MasterProfile.Distro].ImageVersion)
		if properties.MasterProfile.ImageRef != nil {
			addValue(parametersMap, "osImageName", properties.MasterProfile.ImageRef.Name)
			addValue(parametersMap, "osImageResourceGroup", properties.MasterProfile.ImageRef.ResourceGroup)
		}
	}

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
	if properties.MasterProfile != nil {
		// MasterProfile exists, uses master DNS prefix
		addValue(parametersMap, "masterEndpointDNSNamePrefix", properties.MasterProfile.DNSPrefix)
	} else if properties.HostedMasterProfile != nil {
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
		assignKubernetesParameters(properties, parametersMap, cloudSpecConfig, generatorCode)
		if profile != nil {
			assignKubernetesParametersFromAgentProfile(profile, parametersMap, cloudSpecConfig, generatorCode)
		}
	}

	// Agent parameters
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
		if !(agentProfile.OSType == api.Windows) {
			if agentProfile.ImageRef != nil {
				addValue(parametersMap, fmt.Sprintf("%sosImageName", agentProfile.Name), agentProfile.ImageRef.Name)
				addValue(parametersMap, fmt.Sprintf("%sosImageResourceGroup", agentProfile.Name), agentProfile.ImageRef.ResourceGroup)
			}
			addValue(parametersMap, fmt.Sprintf("%sosImageOffer", agentProfile.Name), cloudSpecConfig.OSImageConfig[agentProfile.Distro].ImageOffer)
			addValue(parametersMap, fmt.Sprintf("%sosImageSKU", agentProfile.Name), cloudSpecConfig.OSImageConfig[agentProfile.Distro].ImageSku)
			addValue(parametersMap, fmt.Sprintf("%sosImagePublisher", agentProfile.Name), cloudSpecConfig.OSImageConfig[agentProfile.Distro].ImagePublisher)
			addValue(parametersMap, fmt.Sprintf("%sosImageVersion", agentProfile.Name), cloudSpecConfig.OSImageConfig[agentProfile.Distro].ImageVersion)
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

func assignKubernetesParametersFromAgentProfile(profile *api.AgentPoolProfile, parametersMap paramsMap,
	cloudSpecConfig api.AzureEnvironmentSpecConfig, generatorCode string) {
	if profile.KubernetesConfig != nil && profile.KubernetesConfig.ContainerRuntime != "" {
		// override containerRuntime parameter value if specified in AgentPoolProfile
		// this allows for heteregenous clusters
		addValue(parametersMap, "containerRuntime", profile.KubernetesConfig.ContainerRuntime)
	}
}

func assignKubernetesParameters(properties *api.Properties, parametersMap paramsMap,
	cloudSpecConfig api.AzureEnvironmentSpecConfig, generatorCode string) {
	addValue(parametersMap, "generatorCode", generatorCode)

	orchestratorProfile := properties.OrchestratorProfile

	if orchestratorProfile.IsKubernetes() {
		k8sVersion := orchestratorProfile.OrchestratorVersion
		addValue(parametersMap, "kubernetesVersion", k8sVersion)

		k8sComponents := api.K8sComponentsByVersionMap[k8sVersion]
		kubernetesConfig := orchestratorProfile.KubernetesConfig
		kubernetesImageBase := kubernetesConfig.KubernetesImageBase
		mcrKubernetesImageBase := kubernetesConfig.MCRKubernetesImageBase
		hyperkubeImageBase := kubernetesConfig.KubernetesImageBase

		if kubernetesConfig != nil {

			kubeProxySpec := kubernetesImageBase + k8sComponents["kube-proxy"]
			if kubernetesConfig.CustomKubeProxyImage != "" {
				kubeProxySpec = kubernetesConfig.CustomKubeProxyImage
			}
			addValue(parametersMap, "kubeProxySpec", kubeProxySpec)
			if kubernetesConfig.CustomKubeBinaryURL != "" {
				addValue(parametersMap, "kubeBinaryURL", kubernetesConfig.CustomKubeBinaryURL)
			}

			kubernetesHyperkubeSpec := hyperkubeImageBase + k8sComponents["hyperkube"]
			if properties.IsAzureStackCloud() {
				kubernetesHyperkubeSpec = kubernetesHyperkubeSpec + AzureStackSuffix
			}
			if kubernetesConfig.CustomHyperkubeImage != "" {
				kubernetesHyperkubeSpec = kubernetesConfig.CustomHyperkubeImage
			}
			addValue(parametersMap, "kubernetesHyperkubeSpec", kubernetesHyperkubeSpec)

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
			addValue(parametersMap, "kubernetesPodInfraContainerSpec", mcrKubernetesImageBase+k8sComponents["pause"])
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
				// Kubernetes packages as zip file as created by scripts/build-windows-k8s.sh
				// will be removed in future release as if gets phased out (https://github.com/Azure/aks-engine/issues/3851)
				kubeBinariesSASURL := kubernetesConfig.CustomWindowsPackageURL
				if kubeBinariesSASURL == "" {
					if properties.IsAzureStackCloud() {
						kubeBinariesSASURL = cloudSpecConfig.KubernetesSpecConfig.KubeBinariesSASURLBase + AzureStackPrefix + k8sComponents["windowszip"]
					} else {
						kubeBinariesSASURL = cloudSpecConfig.KubernetesSpecConfig.KubeBinariesSASURLBase + k8sComponents["windowszip"]
					}
				}
				addValue(parametersMap, "kubeBinariesSASURL", kubeBinariesSASURL)

				// Kubernetes node binaries as packaged by upstream kubernetes
				// example at https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG-1.11.md#node-binaries-1
				addValue(parametersMap, "windowsKubeBinariesURL", kubernetesConfig.WindowsNodeBinariesURL)
				addValue(parametersMap, "kubeServiceCidr", kubernetesConfig.ServiceCIDR)
				addValue(parametersMap, "kubeBinariesVersion", k8sVersion)
				addValue(parametersMap, "windowsTelemetryGUID", cloudSpecConfig.KubernetesSpecConfig.WindowsTelemetryGUID)
			}
		}

		if kubernetesConfig == nil ||
			!kubernetesConfig.UseManagedIdentity ||
			properties.IsHostedMasterProfile() {
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
			if properties.MasterProfile != nil {
				addSecret(parametersMap, "etcdServerCertificate", certificateProfile.EtcdServerCertificate, true)
				addSecret(parametersMap, "etcdServerPrivateKey", certificateProfile.EtcdServerPrivateKey, true)
				addSecret(parametersMap, "etcdClientCertificate", certificateProfile.EtcdClientCertificate, true)
				addSecret(parametersMap, "etcdClientPrivateKey", certificateProfile.EtcdClientPrivateKey, true)
				for i, pc := range certificateProfile.EtcdPeerCertificates {
					addSecret(parametersMap, "etcdPeerCertificate"+strconv.Itoa(i), pc, true)
				}
				for i, pk := range certificateProfile.EtcdPeerPrivateKeys {
					addSecret(parametersMap, "etcdPeerPrivateKey"+strconv.Itoa(i), pk, true)
				}
			}
		}

		if properties.OrchestratorProfile.KubernetesConfig.MobyVersion != "" {
			addValue(parametersMap, "mobyVersion", properties.OrchestratorProfile.KubernetesConfig.MobyVersion)
		}

		if properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion != "" {
			addValue(parametersMap, "containerdVersion", properties.OrchestratorProfile.KubernetesConfig.ContainerdVersion)
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
