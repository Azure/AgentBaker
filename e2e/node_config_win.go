package e2e

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// this been crafted with a lot of trial and pain, some values are not needed, but it takes a lot of time to figure out which ones.
// and we hope to move on to a different config, so I don't want to invest any more time in this-
func baseTemplateWindows(location string) *datamodel.NodeBootstrappingConfiguration {
	return &datamodel.NodeBootstrappingConfiguration{
		TenantID:          "tenantID",
		SubscriptionID:    config.Config.SubscriptionID,
		ResourceGroupName: "resourcegroup",

		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				HostedMasterProfile: &datamodel.HostedMasterProfile{},
				CertificateProfile:  &datamodel.CertificateProfile{},
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    "Kubernetes",
					OrchestratorVersion: "1.29.9",
					KubernetesConfig: &datamodel.KubernetesConfig{
						AzureCNIURLWindows:   "https://acs-mirror.azureedge.net/azure-cni/v1.4.35/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.4.35.zip",
						ClusterSubnet:        "10.224.0.0/16",
						DNSServiceIP:         "10.0.0.10",
						LoadBalancerSku:      "Standard",
						NetworkPlugin:        "azure",
						NetworkPluginMode:    "overlay",
						ServiceCIDR:          "10.0.0.0/16",
						UseInstanceMetadata:  to.Ptr(true),
						UseManagedIdentity:   true,
						WindowsContainerdURL: "https://acs-mirror.azureedge.net/containerd/windows/",
					},
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name: "winnp",
						//VMSize:              windowsE2EVmSize,
						OSType:              "Windows",
						AvailabilityProfile: "VirtualMachineScaleSets",
						StorageProfile:      "ManagedDisks",
						CustomNodeLabels: map[string]string{
							"kubernetes.azure.com/mode": "user",
						},
						PreprovisionExtension: nil,
						KubernetesConfig: &datamodel.KubernetesConfig{
							ContainerRuntime:         "containerd",
							CloudProviderBackoffMode: "",
						},
						VnetCidrs: []string{
							"10.224.0.0/12",
						},
					},
				},
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
					ClientID: "msi",
					Secret:   "msi",
				},
				FeatureFlags: &datamodel.FeatureFlags{
					EnableWinDSR: true,
				},
				WindowsProfile: &datamodel.WindowsProfile{
					//CseScriptsPackageURL:           csePackageURL,
					//GpuDriverURL:                   windowsGpuDriverURL,
					AlwaysPullWindowsPauseImage:    to.Ptr(false),
					CSIProxyURL:                    "https://acs-mirror.azureedge.net/csi-proxy/v0.2.2/binaries/csi-proxy-v0.2.2.tar.gz",
					EnableAutomaticUpdates:         to.Ptr(false),
					EnableCSIProxy:                 to.Ptr(true),
					HnsRemediatorIntervalInMinutes: to.Ptr[uint32](1),
					ImageVersion:                   "",
					SSHEnabled:                     to.Ptr(true),
					WindowsDockerVersion:           "",
					WindowsImageSourceURL:          "",
					WindowsOffer:                   "aks-windows",
					WindowsPauseImageURL:           "mcr.microsoft.com/oss/kubernetes/pause:3.9",
					WindowsPublisher:               "microsoft-aks",
					WindowsSku:                     "",
				},
				// yes, we need to set linuxprofile
				LinuxProfile: &datamodel.LinuxProfile{
					SSH: struct {
						PublicKeys []datamodel.PublicKey `json:"publicKeys"`
					}{
						PublicKeys: []datamodel.PublicKey{
							{
								KeyData: `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDIs9weXqhc498AY/775zoJAO+bsmgBx2/V2KTaQgbU1I9ePbquox6r1idf1hcyR+wo9bqlMErLlSGdDCZqTfRmZS9gBbicxPuaIDnIvzfNBH/4Eqq6YVcwjkFeHWqL4ABPq/VNzbLr7JkkCVw9Widh3K/HTsfaDx13qOUwzcm2F7FN/+zvrRyz9RDwkzdeOVhG6JwHdQAjLM40z49BP4yPyHl4r
xvDmGUcOYRy+zCf4Sz75Nw+7wOph3X8KUY8EcHqptXMtk+6f17tasZNaiK0sGY+Hq/Craz2ryO3cDtDn+8Kw2Mpwox8qmdKTCVHPkHgh9OwiFPPWcnld4kNg/+V9ONlsJLUTAwOVezqsCWWU/+NpTWhKqLp682FOZ1fhI+jRlMp0Sa6uEXdw9U56J4IbgzXa1RXYmmq8xceMRIRWC9dxVjcv8F1KrpJoCORtrZDQDaF3Kw789dX09MawfdCZscKSV
DXRqvV7TWO2hndliQq3BW385ZkiephlrmpUVM= r2k1@arturs-mbp.lan`,
							},
						},
					},
				},
			},
		},
		CloudSpecConfig: &datamodel.AzureEnvironmentSpecConfig{
			CloudName: "AzurePublicCloud",
			DockerSpecConfig: datamodel.DockerSpecConfig{
				DockerEngineRepo:         "https://aptdocker.azureedge.net/repo",
				DockerComposeDownloadURL: "https://github.com/docker/compose/releases/download",
			},
			KubernetesSpecConfig: datamodel.KubernetesSpecConfig{
				ACIConnectorImageBase:                "microsoft/",
				AlwaysPullWindowsPauseImage:          false,
				AzureCNIImageBase:                    "mcr.microsoft.com/containernetworking/",
				AzureTelemetryPID:                    "",
				CNIARM64PluginsDownloadURL:           "https://acs-mirror.azureedge.net/cni-plugins/v0.8.7/binaries/cni-plugins-linux-arm64-v0.8.7.tgz",
				CNIPluginsDownloadURL:                "https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-v0.7.6.tgz",
				CSIProxyDownloadURL:                  "https://acs-mirror.azureedge.net/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz",
				CalicoImageBase:                      "calico/",
				ContainerdDownloadURLBase:            "https://storage.googleapis.com/cri-containerd-release/",
				CseScriptsPackageURL:                 "https://acs-mirror.azureedge.net/aks/windows/cse/csescripts-v0.0.1.zip",
				EtcdDownloadURLBase:                  "",
				KubeBinariesSASURLBase:               "https://acs-mirror.azureedge.net/kubernetes/",
				KubernetesImageBase:                  "k8s.gcr.io/",
				MCRKubernetesImageBase:               "mcr.microsoft.com/",
				NVIDIAImageBase:                      "nvidia/",
				TillerImageBase:                      "gcr.io/kubernetes-helm/",
				VnetCNIARM64LinuxPluginsDownloadURL:  "https://acs-mirror.azureedge.net/azure-cni/v1.4.13/binaries/azure-vnet-cni-linux-arm64-v1.4.14.tgz",
				VnetCNILinuxPluginsDownloadURL:       "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-linux-amd64-v1.1.3.tgz",
				VnetCNIWindowsPluginsDownloadURL:     "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.1.3.zip",
				WindowsPauseImageURL:                 "mcr.microsoft.com/oss/kubernetes/pause:1.4.0",
				WindowsProvisioningScriptsPackageURL: "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.2.2.zip",
				WindowsTelemetryGUID:                 "fb801154-36b9-41bc-89c2-f4d4f05472b0",
			},
			EndpointConfig: datamodel.AzureEndpointConfig{
				ResourceManagerVMDNSSuffix: "cloudapp.azure.com",
			},
			OSImageConfig: map[datamodel.Distro]datamodel.AzureOSImageConfig(nil),
		},
		K8sComponents: &datamodel.K8sComponents{
			//WindowsPackageURL: windowsPackageURL,
		},
		AgentPoolProfile: &datamodel.AgentPoolProfile{
			Name: "winnp",
			//VMSize:              windowsE2EVmSize,
			OSType:              "Windows",
			AvailabilityProfile: "VirtualMachineScaleSets",
			StorageProfile:      "ManagedDisks",
			CustomNodeLabels: map[string]string{
				"kubernetes.azure.com/mode":    "user",
				"kubernetes.azure.com/cluster": "test",
				"kubernetes.io/os":             "windows",
			},
			PreprovisionExtension: nil,
			KubernetesConfig: &datamodel.KubernetesConfig{
				ContainerRuntime:         "containerd",
				CloudProviderBackoffMode: "",
			},
			NotRebootWindowsNode: to.Ptr(true),
		},
		PrimaryScaleSetName: "akswin30",
		//ConfigGPUDriverIfNeeded: configGpuDriverIfNeeded,
		KubeletConfig: map[string]string{
			"--azure-container-registry-config": "c:\\k\\azure.json",
			"--bootstrap-kubeconfig":            "c:\\k\\bootstrap-config",
			"--cert-dir":                        "c:\\k\\pki",
			"--cgroups-per-qos":                 "false",
			"--client-ca-file":                  "c:\\k\\ca.crt",
			"--cloud-config":                    "c:\\k\\azure.json",
			"--cloud-provider":                  "external",
			"--enforce-node-allocatable":        "\"\"\"\"",
			"--eviction-hard":                   "\"\"\"\"",
			"--feature-gates":                   "DynamicKubeletConfig=false",
			"--hairpin-mode":                    "promiscuous-bridge",
			"--kube-reserved":                   "cpu=100m,memory=3891Mi",
			"--kubeconfig":                      "c:\\k\\config",
			"--max-pods":                        "30",
			"--pod-infra-container-image":       "mcr.microsoft.com/oss/kubernetes/pause:3.9",
			"--resolv-conf":                     "\"\"\"\"",
			"--cluster-dns":                     "10.0.0.10",
			"--cluster-domain":                  "cluster.local",
			"--rotate-certificates":             "true",
			"--tls-cipher-suites":               "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256",
		},
		SIGConfig: datamodel.SIGConfig{
			TenantID:       "tenantID",
			SubscriptionID: "subID",
			Galleries: map[string]datamodel.SIGGalleryConfig{
				"AKSUbuntu": {
					GalleryName:   "aksubuntu",
					ResourceGroup: "resourcegroup",
				},
				"AKSCBLMariner": {
					GalleryName:   "akscblmariner",
					ResourceGroup: "resourcegroup",
				},
				"AKSAzureLinux": {
					GalleryName:   "aksazurelinux",
					ResourceGroup: "resourcegroup",
				},
				"AKSWindows": {
					GalleryName:   "AKSWindows",
					ResourceGroup: "AKS-Windows",
				},
				"AKSUbuntuEdgeZone": {
					GalleryName:   "AKSUbuntuEdgeZone",
					ResourceGroup: "AKS-Ubuntu-EdgeZone",
				},
			},
		},
	}
}
