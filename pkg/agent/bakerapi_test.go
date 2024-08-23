package agent

import (
	"context"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	agenttoggles "github.com/Azure/agentbaker/pkg/agent/toggles"
	"github.com/barkimedes/go-deepcopy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AgentBaker API implementation tests", func() {
	var (
		cs        *datamodel.ContainerService
		config    *datamodel.NodeBootstrappingConfiguration
		sigConfig *datamodel.SIGConfig
		toggles   *agenttoggles.Toggles
	)

	BeforeEach(func() {
		toggles = agenttoggles.New()

		cs = &datamodel.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: "1.16.15",
					KubernetesConfig:    &datamodel.KubernetesConfig{},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					DNSPrefix: "uttestdom",
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "agent2",
						VMSize:              "Standard_DS1_v2",
						StorageProfile:      "ManagedDisks",
						OSType:              datamodel.Linux,
						VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet1",
						AvailabilityProfile: datamodel.VirtualMachineScaleSets,
						Distro:              datamodel.AKSUbuntu1604,
					},
				},
				LinuxProfile: &datamodel.LinuxProfile{
					AdminUsername: "azureuser",
				},
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
					ClientID: "ClientID",
					Secret:   "Secret",
				},
			},
		}
		cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
			KeyData: string("testsshkey"),
		}}

		agentPool := cs.Properties.AgentPoolProfiles[0]

		fullK8sComponentsMap := K8sComponentsByVersionMap[cs.Properties.OrchestratorProfile.OrchestratorVersion]
		pauseImage := cs.Properties.OrchestratorProfile.KubernetesConfig.MCRKubernetesImageBase + fullK8sComponentsMap["pause"]

		hyperkubeImageBase := cs.Properties.OrchestratorProfile.KubernetesConfig.KubernetesImageBase
		hyperkubeImage := hyperkubeImageBase + fullK8sComponentsMap["hyperkube"]
		if cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage != "" {
			hyperkubeImage = cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage
		}

		windowsPackage := datamodel.AzurePublicCloudSpecForTest.KubernetesSpecConfig.KubeBinariesSASURLBase + fullK8sComponentsMap["windowszip"]
		k8sComponents := &datamodel.K8sComponents{
			PodInfraContainerImageURL: pauseImage,
			HyperkubeImageURL:         hyperkubeImage,
			WindowsPackageURL:         windowsPackage,
		}

		kubeletConfig := map[string]string{
			"--address":                           "0.0.0.0",
			"--pod-manifest-path":                 "/etc/kubernetes/manifests",
			"--cloud-provider":                    "azure",
			"--cloud-config":                      "/etc/kubernetes/azure.json",
			"--azure-container-registry-config":   "/etc/kubernetes/azure.json",
			"--cluster-domain":                    "cluster.local",
			"--cluster-dns":                       "10.0.0.10",
			"--cgroups-per-qos":                   "true",
			"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
			"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
			"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
			"--max-pods":                          "110",
			"--node-status-update-frequency":      "10s",
			"--image-gc-high-threshold":           "85",
			"--image-gc-low-threshold":            "80",
			"--event-qps":                         "0",
			"--pod-max-pids":                      "-1",
			"--enforce-node-allocatable":          "pods",
			"--streaming-connection-idle-timeout": "4h0m0s",
			"--rotate-certificates":               "true",
			"--read-only-port":                    "10255",
			"--protect-kernel-defaults":           "true",
			"--resolv-conf":                       "/etc/resolv.conf",
			"--anonymous-auth":                    "false",
			"--client-ca-file":                    "/etc/kubernetes/certs/ca.crt",
			"--authentication-token-webhook":      "true",
			"--authorization-mode":                "Webhook",
			"--eviction-hard":                     "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%",
			"--feature-gates":                     "RotateKubeletServerCertificate=true,a=b,PodPriority=true,x=y",
			"--system-reserved":                   "cpu=2,memory=1Gi",
			"--kube-reserved":                     "cpu=100m,memory=1638Mi",
		}

		galleries := map[string]datamodel.SIGGalleryConfig{
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
				GalleryName:   "akswindows",
				ResourceGroup: "resourcegroup",
			},
			"AKSUbuntuEdgeZone": {
				GalleryName:   "AKSUbuntuEdgeZone",
				ResourceGroup: "AKS-Ubuntu-EdgeZone",
			},
		}
		sigConfig = &datamodel.SIGConfig{
			TenantID:       "sometenantid",
			SubscriptionID: "somesubid",
			Galleries:      galleries,
		}

		config = &datamodel.NodeBootstrappingConfiguration{
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
			FIPSEnabled:                   false,
			KubeletConfig:                 kubeletConfig,
			PrimaryScaleSetName:           "aks-agent2-36873793-vmss",
			SIGConfig:                     *sigConfig,
		}
	})

	Context("GetNodeBootstrapping", func() {
		It("should return correct boot strapping data", func() {
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())

			// baker_test.go tested the correctness of the generated Custom Data and CSE, so here
			// we just do a sanity check of them not being empty.
			Expect(nodeBootStrapping.CustomData).NotTo(Equal(""))
			Expect(nodeBootStrapping.CSE).NotTo(Equal(""))

			Expect(nodeBootStrapping.OSImageConfig.ImageOffer).To(Equal("aks"))
			Expect(nodeBootStrapping.OSImageConfig.ImageSku).To(Equal("aks-ubuntu-1604-2021-q3"))
			Expect(nodeBootStrapping.OSImageConfig.ImagePublisher).To(Equal("microsoft-aks"))
			Expect(nodeBootStrapping.OSImageConfig.ImageVersion).To(Equal("2021.11.06"))

			Expect(nodeBootStrapping.SigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(nodeBootStrapping.SigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(nodeBootStrapping.SigImageConfig.Definition).To(Equal("1604"))
			Expect(nodeBootStrapping.SigImageConfig.Version).To(Equal("2021.11.06"))
		})

		It("should return the correct bootstrapping data when linux node image version override is present", func() {
			toggles.Maps = map[string]agenttoggles.MapToggle{
				"linux-node-image-version": func(entity *agenttoggles.Entity) map[string]string {
					return map[string]string{
						string(datamodel.AKSUbuntu1604): "202402.27.0",
					}
				},
			}

			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())

			// baker_test.go tested the correctness of the generated Custom Data and CSE, so here
			// we just do a sanity check of them not being empty.
			Expect(nodeBootStrapping.CustomData).NotTo(Equal(""))
			Expect(nodeBootStrapping.CSE).NotTo(Equal(""))

			Expect(nodeBootStrapping.SigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(nodeBootStrapping.SigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(nodeBootStrapping.SigImageConfig.Definition).To(Equal("1604"))
			Expect(nodeBootStrapping.SigImageConfig.Version).To(Equal("202402.27.0"))
		})

		It("should return the correct bootstrapping data when linux node image version is present but does not specify for distro", func() {
			toggles.Maps = map[string]agenttoggles.MapToggle{
				"linux-node-image-version": func(entity *agenttoggles.Entity) map[string]string {
					return map[string]string{
						string(datamodel.AKSUbuntu1804): "202402.27.0",
					}
				},
			}
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())

			// baker_test.go tested the correctness of the generated Custom Data and CSE, so here
			// we just do a sanity check of them not being empty.
			Expect(nodeBootStrapping.CustomData).NotTo(Equal(""))
			Expect(nodeBootStrapping.CSE).NotTo(Equal(""))

			Expect(nodeBootStrapping.SigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(nodeBootStrapping.SigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(nodeBootStrapping.SigImageConfig.Definition).To(Equal("1604"))
			Expect(nodeBootStrapping.SigImageConfig.Version).To(Equal("2021.11.06"))
		})

		It("should return an error if cloud is not found", func() {
			// this CloudSpecConfig is shared across all AgentBaker UTs,
			// thus we need to make and use a copy when performing mutations for mocking
			cloudSpecConfigCopy, err := deepcopy.Anything(config.CloudSpecConfig)
			Expect(err).To(BeNil())
			cloudSpecConfig, ok := cloudSpecConfigCopy.(*datamodel.AzureEnvironmentSpecConfig)
			Expect(ok).To(BeTrue())
			config.CloudSpecConfig = cloudSpecConfig

			config.CloudSpecConfig.CloudName = "UnknownCloud"
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)
			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if distro is neither found in PIR nor found in SIG", func() {
			config.AgentPoolProfile.Distro = "unknown"
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).To(HaveOccurred())
		})

		It("should not return an error for customized image", func() {
			config.AgentPoolProfile.Distro = datamodel.CustomizedImage
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error for customized kata image", func() {
			config.AgentPoolProfile.Distro = datamodel.CustomizedImageKata
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error for customized windows image", func() {
			config.AgentPoolProfile.Distro = datamodel.CustomizedWindowsOSImage
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("GetLatestSigImageConfig", func() {
		It("should return correct value for existing distro", func() {
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			sigImageConfig, err := agentBaker.GetLatestSigImageConfig(config.SIGConfig, datamodel.AKSUbuntu1604, &datamodel.EnvironmentInfo{
				SubscriptionID: config.SubscriptionID,
				TenantID:       config.TenantID,
				Region:         cs.Location,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(sigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(sigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(sigImageConfig.Definition).To(Equal("1604"))
			Expect(sigImageConfig.Version).To(Equal("2021.11.06"))
		})

		It("should return correct value for existing distro when linux node image version override is provided", func() {
			toggles.Maps = map[string]agenttoggles.MapToggle{
				"linux-node-image-version": func(entity *agenttoggles.Entity) map[string]string {
					return map[string]string{
						string(datamodel.AKSUbuntu1604): "202402.27.0",
					}
				},
			}
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			sigImageConfig, err := agentBaker.GetLatestSigImageConfig(config.SIGConfig, datamodel.AKSUbuntu1604, &datamodel.EnvironmentInfo{
				SubscriptionID: config.SubscriptionID,
				TenantID:       config.TenantID,
				Region:         cs.Location,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(sigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(sigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(sigImageConfig.Definition).To(Equal("1604"))
			Expect(sigImageConfig.Version).To(Equal("202402.27.0"))
		})

		It("should return correct value for existing distro when linux node image version override is provided but not for distro", func() {
			toggles.Maps = map[string]agenttoggles.MapToggle{
				"linux-node-image-version": func(entity *agenttoggles.Entity) map[string]string {
					return map[string]string{
						string(datamodel.AKSUbuntu1804): "202402.27.0",
					}
				},
			}
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			sigImageConfig, err := agentBaker.GetLatestSigImageConfig(config.SIGConfig, datamodel.AKSUbuntu1604, &datamodel.EnvironmentInfo{
				SubscriptionID: config.SubscriptionID,
				TenantID:       config.TenantID,
				Region:         cs.Location,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(sigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(sigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(sigImageConfig.Definition).To(Equal("1604"))
			Expect(sigImageConfig.Version).To(Equal("2021.11.06"))
		})

		It("should return error if image config not found for distro", func() {
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())
			agentBaker = agentBaker.WithToggles(toggles)

			_, err = agentBaker.GetLatestSigImageConfig(config.SIGConfig, "unknown", &datamodel.EnvironmentInfo{
				SubscriptionID: config.SubscriptionID,
				TenantID:       config.TenantID,
				Region:         cs.Location,
			})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("GetDistroSigImageConfig", func() {
		var (
			ubuntuDistros     []datamodel.Distro
			marinerDistros    []datamodel.Distro
			azureLinuxDistros []datamodel.Distro
			allLinuxDistros   []datamodel.Distro
		)

		BeforeEach(func() {
			ubuntuDistros = []datamodel.Distro{
				datamodel.AKSUbuntuContainerd1804,
				datamodel.AKSUbuntuContainerd1804Gen2,
				datamodel.AKSUbuntuGPUContainerd1804,
				datamodel.AKSUbuntuGPUContainerd1804Gen2,
				datamodel.AKSUbuntuFipsContainerd1804,
				datamodel.AKSUbuntuFipsContainerd1804Gen2,
				datamodel.AKSUbuntuFipsContainerd2004,
				datamodel.AKSUbuntuFipsContainerd2004Gen2,
				datamodel.AKSUbuntuContainerd2204,
				datamodel.AKSUbuntuContainerd2204Gen2,
				datamodel.AKSUbuntuContainerd2004CVMGen2,
				datamodel.AKSUbuntuArm64Containerd2204Gen2,
				datamodel.AKSUbuntuContainerd2204TLGen2,
			}

			marinerDistros = []datamodel.Distro{
				datamodel.AKSCBLMarinerV2,
				datamodel.AKSCBLMarinerV2Gen2,
				datamodel.AKSCBLMarinerV2FIPS,
				datamodel.AKSCBLMarinerV2Gen2FIPS,
				datamodel.AKSCBLMarinerV2Gen2Kata,
				datamodel.AKSCBLMarinerV2Arm64Gen2,
				datamodel.AKSCBLMarinerV2Gen2TL,
			}

			azureLinuxDistros = []datamodel.Distro{
				datamodel.AKSAzureLinuxV2,
				datamodel.AKSAzureLinuxV3,
				datamodel.AKSAzureLinuxV2Gen2,
				datamodel.AKSAzureLinuxV3Gen2,
				datamodel.AKSAzureLinuxV2FIPS,
				datamodel.AKSAzureLinuxV3FIPS,
				datamodel.AKSAzureLinuxV2Gen2FIPS,
				datamodel.AKSAzureLinuxV3Gen2FIPS,
				datamodel.AKSAzureLinuxV2Gen2Kata,
				datamodel.AKSAzureLinuxV2Arm64Gen2,
				datamodel.AKSAzureLinuxV3Arm64Gen2,
				datamodel.AKSAzureLinuxV2Gen2TL,
			}

			allLinuxDistros = append(allLinuxDistros, ubuntuDistros...)
			allLinuxDistros = append(allLinuxDistros, marinerDistros...)
			allLinuxDistros = append(allLinuxDistros, azureLinuxDistros...)
		})

		It("should return correct value for all existing distros", func() {
			agentBaker, err := NewAgentBaker()
			Expect(err).To(BeNil())
			agentBaker = agentBaker.WithToggles(toggles)

			configs, err := agentBaker.GetDistroSigImageConfig(config.SIGConfig, &datamodel.EnvironmentInfo{
				SubscriptionID: config.SubscriptionID,
				TenantID:       config.TenantID,
				Region:         cs.Location,
			})
			Expect(err).To(BeNil())

			for _, distro := range allLinuxDistros {
				Expect(configs).To(HaveKey(distro))
				config := configs[distro]
				Expect(config.ResourceGroup).To(Equal("resourcegroup"))
				Expect(config.SubscriptionID).To(Equal("somesubid"))
				Expect(config.Version).To(Equal(datamodel.LinuxSIGImageVersion))
				Expect(config.Definition).ToNot(BeEmpty())
			}

			for _, distro := range ubuntuDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("aksubuntu"))
			}

			for _, distro := range marinerDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("akscblmariner"))
			}

			for _, distro := range azureLinuxDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("aksazurelinux"))
			}
		})

		It("should return correct value for all existing distros with linux node image version override", func() {
			var (
				ubuntuOverrideVersion     = "202402.25.0"
				marinerOverrideVersion    = "202402.25.1"
				azureLinuxOverrideVersion = "202402.25.2"
			)
			imageVersionOverrides := map[string]string{}
			for _, distro := range ubuntuDistros {
				imageVersionOverrides[string(distro)] = ubuntuOverrideVersion
			}
			for _, distro := range marinerDistros {
				imageVersionOverrides[string(distro)] = marinerOverrideVersion
			}
			for _, distro := range azureLinuxDistros {
				imageVersionOverrides[string(distro)] = azureLinuxOverrideVersion
			}
			toggles.Maps = map[string]agenttoggles.MapToggle{
				"linux-node-image-version": func(entity *agenttoggles.Entity) map[string]string {
					return imageVersionOverrides
				},
			}

			agentBaker, err := NewAgentBaker()
			Expect(err).To(BeNil())
			agentBaker = agentBaker.WithToggles(toggles)

			configs, err := agentBaker.GetDistroSigImageConfig(config.SIGConfig, &datamodel.EnvironmentInfo{
				SubscriptionID: config.SubscriptionID,
				TenantID:       config.TenantID,
				Region:         cs.Location,
			})
			Expect(err).To(BeNil())

			for _, distro := range allLinuxDistros {
				Expect(configs).To(HaveKey(distro))
				config := configs[distro]
				Expect(config.ResourceGroup).To(Equal("resourcegroup"))
				Expect(config.SubscriptionID).To(Equal("somesubid"))
				Expect(config.Definition).ToNot(BeEmpty())
			}

			for _, distro := range ubuntuDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("aksubuntu"))
				Expect(config.Version).To(Equal(ubuntuOverrideVersion))
			}

			for _, distro := range marinerDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("akscblmariner"))
				Expect(config.Version).To(Equal(marinerOverrideVersion))
			}

			for _, distro := range azureLinuxDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("aksazurelinux"))
				Expect(config.Version).To(Equal(azureLinuxOverrideVersion))
			}
		})
	})
})
