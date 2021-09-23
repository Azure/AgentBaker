package agent

import (
	"context"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AgentBaker API implementation tests", func() {
	var (
		cs        *datamodel.ContainerService
		config    *datamodel.NodeBootstrappingConfiguration
		sigConfig *datamodel.SIGConfig
	)

	BeforeEach(func() {
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
			"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256",
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
			"AKSUbuntu": datamodel.SIGGalleryConfig{
				GalleryName:   "aksubuntu",
				ResourceGroup: "resourcegroup",
			},
			"AKSCBLMariner": datamodel.SIGGalleryConfig{
				GalleryName:   "akscblmariner",
				ResourceGroup: "resourcegroup",
			},
			"AKSWindows": datamodel.SIGGalleryConfig{
				GalleryName:   "akswindows",
				ResourceGroup: "resourcegroup",
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

			nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())

			// baker_test.go tested the correctness of the generated Custom Data and CSE, so here
			// we just do a sanity check of them not being empty.
			Expect(nodeBootStrapping.CustomData).NotTo(Equal(""))
			Expect(nodeBootStrapping.CSE).NotTo(Equal(""))

			Expect(nodeBootStrapping.OSImageConfig.ImageOffer).To(Equal("aks"))
			Expect(nodeBootStrapping.OSImageConfig.ImageSku).To(Equal("aks-ubuntu-1604-2021-q3"))
			Expect(nodeBootStrapping.OSImageConfig.ImagePublisher).To(Equal("microsoft-aks"))
			Expect(nodeBootStrapping.OSImageConfig.ImageVersion).To(Equal("2021.09.22"))

			Expect(nodeBootStrapping.SigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(nodeBootStrapping.SigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(nodeBootStrapping.SigImageConfig.Definition).To(Equal("1604"))
			Expect(nodeBootStrapping.SigImageConfig.Version).To(Equal("2021.09.22"))
		})

		It("should return an error if cloud is not found", func() {
			config.CloudSpecConfig.CloudName = "UnknownCloud"
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if distro is neither found in PIR nor found in SIG", func() {
			config.AgentPoolProfile.Distro = "unknown"
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("GetLatestSigImageConfig", func() {
		It("should return correct value for existing distro", func() {
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			sigImageConfig, err := agentBaker.GetLatestSigImageConfig(config.SIGConfig, cs.Location, datamodel.AKSUbuntu1604)
			Expect(err).NotTo(HaveOccurred())

			Expect(sigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(sigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(sigImageConfig.Definition).To(Equal("1604"))
			Expect(sigImageConfig.Version).To(Equal("2021.09.22"))
		})

		It("should return error if image config not found for distro", func() {
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			_, err = agentBaker.GetLatestSigImageConfig(config.SIGConfig, cs.Location, "unknown")
			Expect(err).To(HaveOccurred())
		})
	})
})
