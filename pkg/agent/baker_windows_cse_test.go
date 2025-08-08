package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Assert generated customData and cseCmd for Windows", func() {
	DescribeTable("Generated customData and CSE", func(folder, k8sVersion string, configUpdator func(*datamodel.NodeBootstrappingConfiguration)) {
		cs := &datamodel.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: k8sVersion,
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntime:     "docker",
						KubernetesImageBase:  "mcr.microsoft.com/oss/kubernetes/",
						WindowsContainerdURL: "https://k8swin.blob.core.windows.net/k8s-windows/containerd/containerplat-aks-test-0.0.8.zip",
						LoadBalancerSku:      "Standard",
						CustomHyperkubeImage: "mcr.microsoft.com/oss/kubernetes/hyperkube:v1.16.15-hotfix.20200903",
						ClusterSubnet:        "10.240.0.0/16",
						NetworkPlugin:        "azure",
						DockerBridgeSubnet:   "172.17.0.1/16",
						ServiceCIDR:          "10.0.0.0/16",
						EnableRbac:           to.BoolPtr(true),
						EnableSecureKubelet:  to.BoolPtr(true),
						UseInstanceMetadata:  to.BoolPtr(true),
						DNSServiceIP:         "10.0.0.10",
					},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					DNSPrefix:   "uttestdom",
					FQDN:        "uttestdom-dns-5d7c849e.hcp.southcentralus.azmk8s.io",
					Subnet:      "10.240.0.0/16",
					IPMasqAgent: true,
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "wpool2",
						VMSize:              "Standard_D2s_v3",
						StorageProfile:      "ManagedDisks",
						OSType:              datamodel.Windows,
						VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-36873793/subnet/aks-subnet",
						WindowsNameVersion:  "v2",
						AvailabilityProfile: datamodel.VirtualMachineScaleSets,
						CustomNodeLabels:    map[string]string{"kubernetes.azure.com/node-image-version": "AKSWindows-2019-17763.1577.201111"},
						Distro:              datamodel.Distro("aks-windows-2019"),
					},
				},
				LinuxProfile: &datamodel.LinuxProfile{
					AdminUsername: "azureuser",
				},
				WindowsProfile: &datamodel.WindowsProfile{
					ProvisioningScriptsPackageURL: "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.4.zip",
					WindowsPauseImageURL:          "mcr.microsoft.com/oss/kubernetes/pause:1.4.0",
					AdminUsername:                 "azureuser",
					AdminPassword:                 "replacepassword1234",
					WindowsPublisher:              "microsoft-aks",
					WindowsOffer:                  "aks-windows",
					ImageVersion:                  "17763.1577.201111",
					WindowsSku:                    "aks-2019-datacenter-core-smalldisk-2011",
				},
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
					ClientID: "ClientID",
					Secret:   "Secret",
				},
				FeatureFlags: &datamodel.FeatureFlags{
					EnableWinDSR: false,
				},
			},
		}
		cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
			KeyData: string("testsshkey"),
		}}

		// AKS always pass in te customHyperKubeImage to aks-e, so we don't really rely on
		// the default component version for "hyperkube", which is not set since 1.17
		if IsKubernetesVersionGe(k8sVersion, "1.17.0") {
			cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage = fmt.Sprintf("k8s.gcr.io/hyperkube-amd64:v%v", k8sVersion)
		}

		// WinDSR is only supported since 1.19
		if IsKubernetesVersionGe(k8sVersion, "1.19.0") {
			cs.Properties.FeatureFlags.EnableWinDSR = true
		}

		agentPool := cs.Properties.AgentPoolProfiles[0]

		k8sComponents := &datamodel.K8sComponents{}

		if IsKubernetesVersionGe(k8sVersion, "1.29.0") {
			// This is test only, credential provider version does not align with k8s version
			k8sComponents.WindowsCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-windows-amd64-v%s.tar.gz", k8sVersion, k8sVersion) //nolint:lll
			k8sComponents.LinuxCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz", k8sVersion, k8sVersion)     //nolint:lll
		}

		kubeletConfig := map[string]string{
			"--address":                           "0.0.0.0",
			"--anonymous-auth":                    "false",
			"--authentication-token-webhook":      "true",
			"--authorization-mode":                "Webhook",
			"--cloud-config":                      "c:\\k\\azure.json",
			"--cgroups-per-qos":                   "false",
			"--client-ca-file":                    "c:\\k\\ca.crt",
			"--azure-container-registry-config":   "c:\\k\\azure.json",
			"--cloud-provider":                    "azure",
			"--cluster-dns":                       "10.0.0.10",
			"--cluster-domain":                    "cluster.local",
			"--enforce-node-allocatable":          "",
			"--event-qps":                         "0",
			"--eviction-hard":                     "",
			"--feature-gates":                     "RotateKubeletServerCertificate=true",
			"--hairpin-mode":                      "promiscuous-bridge",
			"--image-gc-high-threshold":           "85",
			"--image-gc-low-threshold":            "80",
			"--kube-reserved":                     "cpu=100m,memory=1843Mi",
			"--kubeconfig":                        "c:\\k\\config",
			"--max-pods":                          "30",
			"--network-plugin":                    "cni",
			"--node-status-update-frequency":      "10s",
			"--pod-infra-container-image":         "mcr.microsoft.com/oss/kubernetes/pause:3.9",
			"--pod-max-pids":                      "-1",
			"--read-only-port":                    "0",
			"--resolv-conf":                       `""`,
			"--rotate-certificates":               "false",
			"--streaming-connection-idle-timeout": "4h",
			"--system-reserved":                   "memory=2Gi",
			"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
		}

		config := &datamodel.NodeBootstrappingConfiguration{
			ContainerService:              cs,
			CloudSpecConfig:               datamodel.AzurePublicCloudSpecForTest,
			K8sComponents:                 k8sComponents,
			AgentPoolProfile:              agentPool,
			TenantID:                      "tenantID",
			SubscriptionID:                "subID",
			ResourceGroupName:             "resourceGroupName",
			UserAssignedIdentityClientID:  "userAssignedID",
			ConfigGPUDriverIfNeeded:       true,
			EnableGPUDevicePluginIfNeeded: false,
			EnableKubeletConfigFile:       false,
			EnableNvidia:                  false,
			KubeletConfig:                 kubeletConfig,
			PrimaryScaleSetName:           "akswpool2",
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
					"AKSFlatcar": {
						GalleryName:   "aksflatcar",
						ResourceGroup: "resourcegroup",
					},
				},
			},
		}

		if configUpdator != nil {
			configUpdator(config)
		}

		// customData
		ab, err := NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err := ab.GetNodeBootstrapping(context.Background(), config)
		Expect(err).To(BeNil())
		base64EncodedCustomData := nodeBootstrapping.CustomData
		customDataBytes, err := base64.StdEncoding.DecodeString(base64EncodedCustomData)
		customData := string(customDataBytes)
		Expect(err).To(BeNil())

		if generateTestData() {
			backfillCustomData(folder, customData)
		}

		expectedCustomData, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CustomData", folder))
		if err != nil {
			panic(err)
		}
		Expect(customData).To(Equal(string(expectedCustomData)))

		// CSE
		ab, err = NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err = ab.GetNodeBootstrapping(context.Background(), config)
		Expect(err).To(BeNil())
		cseCommand := nodeBootstrapping.CSE

		if generateTestData() {
			err = os.WriteFile(fmt.Sprintf("./testdata/%s/CSECommand", folder), []byte(cseCommand), 0644)
			Expect(err).To(BeNil())
		}

		expectedCSECommand, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CSECommand", folder))
		if err != nil {
			panic(err)
		}
		Expect(cseCommand).To(Equal(string(expectedCSECommand)))

	}, Entry("AKSWindows2019 with k8s version 1.16", "AKSWindows2019+K8S116", "1.16.15", func(config *datamodel.NodeBootstrappingConfiguration) {
	}),
		Entry("AKSWindows2019 with k8s version 1.17", "AKSWindows2019+K8S117", "1.17.7", func(config *datamodel.NodeBootstrappingConfiguration) {
		}),
		Entry("AKSWindows2019 with k8s version 1.18", "AKSWindows2019+K8S118", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
		}),
		Entry("AKSWindows2019 with k8s version 1.19", "AKSWindows2019+K8S119", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
		}),
		Entry("AKSWindows2019 with k8s version 1.19 + CSI", "AKSWindows2019+K8S119+CSI", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.WindowsProfile.CSIProxyURL = "https://acs-mirror.azureedge.net/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz"
			config.ContainerService.Properties.WindowsProfile.EnableCSIProxy = to.BoolPtr(true)
		}),
		Entry("AKSWindows2019 with CustomVnet", "AKSWindows2019+CustomVnet", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = "172.17.0.0/24"
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.ServiceCIDR = "172.17.255.0/24"
			config.ContainerService.Properties.AgentPoolProfiles[0].VnetCidrs = []string{"172.17.0.0/16"}
			config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID = "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet2" //nolint:lll
			config.KubeletConfig["--cluster-dns"] = "172.17.255.10"
		}),
		Entry("AKSWindows2019 with Managed Identity", "AKSWindows2019+ManagedIdentity", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.ServicePrincipalProfile = &datamodel.ServicePrincipalProfile{ClientID: "msi"}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UserAssignedID = "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/k8s-agentpool" //nolint:lll
		}),
		Entry("AKSWindows2019 with custom cloud", "AKSWindows2019+CustomCloud", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = to.BoolPtr(true)
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
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
				ResourceIdentifiers: datamodel.ResourceIdentifiers{
					Graph:               "",
					KeyVault:            "",
					Datalake:            "",
					Batch:               "",
					OperationalInsights: "",
					Storage:             "",
				},
			}
		}),
		Entry("AKSWindows2019 EnablePrivateClusterHostsConfigAgent", "AKSWindows2019+EnablePrivateClusterHostsConfigAgent", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				cs := config.ContainerService
				if cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster == nil {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &datamodel.PrivateCluster{EnableHostsConfigAgent: to.BoolPtr(true)}
				} else {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent = to.BoolPtr(true)
				}
			}),
		Entry("AKSWindows2019 with kubelet client TLS bootstrapping enabled", "AKSWindows2019+KubeletClientTLSBootstrapping", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletClientTLSBootstrapToken = to.StringPtr("07401b.f395accd246ae52d")
			}),
		Entry("AKSWindows2019 with kubelet serving certificate rotation enabled", "AKSWindows2019+KubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig["--rotate-server-certificates"] = "true"
			}),
		Entry("AKSWindows2019 with k8s version 1.19 + FIPS", "AKSWindows2019+K8S119+FIPS", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.FIPSEnabled = true
			}),
		Entry("AKSWindows2019 with SecurityProfile", "AKSWindows2019+SecurityProfile", "1.26.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:      true,
						ProxyAddress: "https://test-pe-proxy",
					},
				}
			}),
		Entry("AKSWindows2019 with out of tree credential provider", "AKSWindows2019+ootcredentialprovider", "1.29.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = to.BoolPtr(true)
			config.KubeletConfig["--image-credential-provider-config"] = "c:\\var\\lib\\kubelet\\credential-provider-config.yaml"
			config.KubeletConfig["--image-credential-provider-bin-dir"] = "c:\\var\\lib\\kubelet\\credential-provider"
		}),
		Entry("AKSWindows2019 with custom cloud and out of tree credential provider", "AKSWindows2019+CustomCloud+ootcredentialprovider", "1.29.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = to.BoolPtr(true)
				config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
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
					ResourceIdentifiers: datamodel.ResourceIdentifiers{
						Graph:               "",
						KeyVault:            "",
						Datalake:            "",
						Batch:               "",
						OperationalInsights: "",
						Storage:             "",
					},
				}
				config.KubeletConfig["--image-credential-provider-config"] = "c:\\var\\lib\\kubelet\\credential-provider-config.yaml"
				config.KubeletConfig["--image-credential-provider-bin-dir"] = "c:\\var\\lib\\kubelet\\credential-provider"
			}),
	)
})
