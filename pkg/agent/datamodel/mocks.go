// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

// GetK8sDefaultProperties returns a struct of type Properties for testing purposes.
func GetK8sDefaultProperties(hasWindows bool) *Properties {
	p := &Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType: Kubernetes,
			KubernetesConfig: &KubernetesConfig{},
		},
		HostedMasterProfile: &HostedMasterProfile{
			DNSPrefix: "foo",
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				AvailabilityProfile: AvailabilitySet,
			},
		},
		ServicePrincipalProfile: &ServicePrincipalProfile{
			ClientID: "clientID",
			Secret:   "clientSecret",
		},
	}

	if hasWindows {
		p.AgentPoolProfiles = []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				AvailabilityProfile: AvailabilitySet,
				OSType:              Windows,
			},
		}
		p.WindowsProfile = &WindowsProfile{
			AdminUsername: "azureuser",
			AdminPassword: "replacepassword1234$",
		}
	}

	return p
}

func getMockProperitesWithCustomClouEnv() Properties {
	properties := Properties{
		CustomCloudEnv: &CustomCloudEnv{
			Name:                         "akscustom",
			McrURL:                       "mcr.microsoft.fakecustomcloud",
			RepoDepotEndpoint:            "https://repodepot.azure.microsoft.fakecustomcloud/ubuntu",
			ManagementPortalURL:          "https://portal.azure.microsoft.fakecustomcloud/",
			PublishSettingsURL:           "",
			ServiceManagementEndpoint:    "https://management.core.microsoft.fakecustomcloud/",
			ResourceManagerEndpoint:      "https://management.azure.microsoft.fakecustomcloud/",
			ActiveDirectoryEndpoint:      "https://login.microsoftonline.microsoft.fakecustomcloud/",
			GalleryEndpoint:              "",
			KeyVaultEndpoint:             "https://vault.cloudapi.microsoft.fakecustomcloud/",
			GraphEndpoint:                "https://graph.cloudapi.microsoft.fakecustomcloud/",
			ServiceBusEndpoint:           "",
			BatchManagementEndpoint:      "",
			StorageEndpointSuffix:        "core.microsoft.fakecustomcloud",
			SQLDatabaseDNSSuffix:         "database.cloudapi.microsoft.fakecustomcloud",
			TrafficManagerDNSSuffix:      "",
			KeyVaultDNSSuffix:            "vault.cloudapi.microsoft.fakecustomcloud",
			ServiceBusEndpointSuffix:     "",
			ServiceManagementVMDNSSuffix: "",
			ResourceManagerVMDNSSuffix:   "cloudapp.azure.microsoft.fakecustomcloud/",
			ContainerRegistryDNSSuffix:   ".azurecr.microsoft.fakecustomcloud",
			CosmosDBDNSSuffix:            "documents.core.microsoft.fakecustomcloud/",
			TokenAudience:                "https://management.core.microsoft.fakecustomcloud/",
			ResourceIdentifiers: ResourceIdentifiers{
				Graph:               "",
				KeyVault:            "",
				Datalake:            "",
				Batch:               "",
				OperationalInsights: "",
				Storage:             "",
			},
		},
	}
	return properties
}

func getMockAddon(name string) KubernetesAddon {
	return KubernetesAddon{
		Name: name,
		Containers: []KubernetesContainerSpec{
			{
				Name:           name,
				CPURequests:    "50m",
				MemoryRequests: "150Mi",
				CPULimits:      "50m",
				MemoryLimits:   "150Mi",
			},
		},
		Pools: []AddonNodePoolsConfig{
			{
				Name: "pool1",
				Config: map[string]string{
					"min-nodes": "3",
					"max-nodes": "3",
				},
			},
		},
	}
}

//nolint:gochecknoglobals
var (
	AzurePublicCloudSpecForTest = &AzureEnvironmentSpecConfig{
		CloudName: "AzurePublicCloud",
		// KubernetesSpecConfig is the default kubernetes container image url.
		KubernetesSpecConfig: KubernetesSpecConfig{
			KubernetesImageBase:    "k8s.gcr.io/",
			TillerImageBase:        "gcr.io/kubernetes-helm/",
			ACIConnectorImageBase:  "microsoft/",
			NVIDIAImageBase:        "nvidia/",
			CalicoImageBase:        "calico/",
			AzureCNIImageBase:      "mcr.microsoft.com/containernetworking/",
			MCRKubernetesImageBase: "mcr.microsoft.com/",

			KubeBinariesSASURLBase:               "https://acs-mirror.azureedge.net/kubernetes/",
			WindowsTelemetryGUID:                 "fb801154-36b9-41bc-89c2-f4d4f05472b0",
			CNIPluginsDownloadURL:                "https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-v0.7.6.tgz",
			VnetCNILinuxPluginsDownloadURL:       "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-linux-amd64-v1.1.3.tgz",
			VnetCNIWindowsPluginsDownloadURL:     "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.1.3.zip",
			ContainerdDownloadURLBase:            "https://storage.googleapis.com/cri-containerd-release/",
			CSIProxyDownloadURL:                  "https://acs-mirror.azureedge.net/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz",
			WindowsProvisioningScriptsPackageURL: "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.2.2.zip",
			WindowsPauseImageURL:                 "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.1",
			AlwaysPullWindowsPauseImage:          false,
			CseScriptsPackageURL:                 "https://acs-mirror.azureedge.net/aks/windows/cse/csescripts-v0.0.1.zip",
			CNIARM64PluginsDownloadURL:           "https://acs-mirror.azureedge.net/cni-plugins/v0.8.7/binaries/cni-plugins-linux-arm64-v0.8.7.tgz",
			VnetCNIARM64LinuxPluginsDownloadURL:  "https://acs-mirror.azureedge.net/azure-cni/v1.4.13/binaries/azure-vnet-cni-linux-arm64-v1.4.14.tgz",
		},

		EndpointConfig: AzureEndpointConfig{
			ResourceManagerVMDNSSuffix: "cloudapp.azure.com",
		},
	}
)
