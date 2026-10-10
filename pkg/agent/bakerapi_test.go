package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	agenttoggles "github.com/Azure/agentbaker/pkg/agent/toggles"
	"github.com/barkimedes/go-deepcopy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

type testToggles struct {
	defaultNodeImageVersionOverride string
	nodeImageVersionOverrides       map[datamodel.Distro]string
}

func (t *testToggles) GetLinuxNodeImageVersion(entity *agenttoggles.Entity, distro datamodel.Distro) string {
	if version := t.nodeImageVersionOverrides[distro]; version != "" {
		return version
	}
	return t.defaultNodeImageVersionOverride
}

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
					OrchestratorVersion: "1.32.1",
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
						Distro:              datamodel.AKSUbuntuContainerd2204Gen2,
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

		k8sComponents := &datamodel.K8sComponents{}

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
			"AKSFlatcar": {
				GalleryName:   "aksflatcar",
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
			Expect(nodeBootStrapping.OSImageConfig.ImageSku).To(Equal("aks-ubuntu-containerd-22.04-gen2"))
			Expect(nodeBootStrapping.OSImageConfig.ImagePublisher).To(Equal("microsoft-aks"))
			Expect(nodeBootStrapping.OSImageConfig.ImageVersion).To(Equal("2025.06.02"))

			Expect(nodeBootStrapping.SigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(nodeBootStrapping.SigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(nodeBootStrapping.SigImageConfig.Definition).To(Equal("2204gen2containerd"))
		})

		It("should render Linux cloud-init files as literal write_files content without changing their bytes", func() {
			testCases := []struct {
				name   string
				distro datamodel.Distro
			}{
				{name: "Ubuntu 22.04", distro: datamodel.AKSUbuntuContainerd2204Gen2},
				{name: "Ubuntu 24.04", distro: datamodel.AKSUbuntuContainerd2404Gen2},
				{name: "Azure Linux 3", distro: datamodel.AKSAzureLinuxV3Gen2},
			}

			for _, testCase := range testCases {
				for _, preProvisionOnly := range []bool{false, true} {
					name := testCase.name
					if preProvisionOnly {
						name += " PIS"
					} else {
						name += " ordinary"
					}
					By(name)

					configCopy, err := deepcopy.Anything(config)
					Expect(err).NotTo(HaveOccurred())
					testConfig, ok := configCopy.(*datamodel.NodeBootstrappingConfiguration)
					Expect(ok).To(BeTrue())
					testConfig.AgentPoolProfile.Distro = testCase.distro
					testConfig.AgentPoolProfile.VMSize = "Standard_NC4as_T4_v3"
					testConfig.PreProvisionOnly = preProvisionOnly
					testConfig.EnableScriptlessCSECmd = false
					testConfig.ConfigGPUDriverIfNeeded = false
					testConfig.EnableGPUDevicePluginIfNeeded = false
					testConfig.EnableNvidia = false

					payload := InitializeTemplateGenerator().getLinuxNodeBootstrappingPayload(testConfig)
					Expect(len(payload)).To(BeNumerically("<", MaxCustomDataLength))
					encoded, err := base64.StdEncoding.Strict().DecodeString(payload)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(encoded)).To(BeNumerically("<=", 65535))
					reader, err := gzip.NewReader(bytes.NewReader(encoded))
					Expect(err).NotTo(HaveOccurred())
					customDataBytes, err := io.ReadAll(reader)
					Expect(err).NotTo(HaveOccurred())
					Expect(reader.Close()).To(Succeed())
					var customData cloudInit
					Expect(yaml.Unmarshal(customDataBytes, &customData)).To(Succeed())
					fmt.Fprintf(
						GinkgoWriter,
						"PIS_CUSTOMDATA distro=%s preProvisionOnly=%t base64=%d gzip=%d yaml=%d\n",
						testCase.distro,
						preProvisionOnly,
						len(payload),
						len(encoded),
						len(customDataBytes),
					)

					customDataVariables := getCustomDataVariables(testConfig)
					variables, ok := customDataVariables["cloudInitFileData"].(paramsMap)
					Expect(ok).To(BeTrue())
					expectedScripts := []struct {
						path string
						key  string
					}{
						{cseHelpersScriptFilepath, "provisionSource"},
						{cseHelpersScriptDistroFilepath, "provisionSourceUbuntu"},
						{"/opt/azure/containers/provision_start.sh", "provisionStartScript"},
						{"/opt/azure/containers/provision.sh", "provisionScript"},
						{cseInstallScriptFilepath, "provisionInstalls"},
						{cseInstallScriptDistroFilepath, "provisionInstallsUbuntu"},
						{cseConfigScriptFilepath, "provisionConfigs"},
						{cseConfigGPUScriptFilepath, "provisionConfigsGPU"},
						{cseConfigLocalDNSScriptFilepath, "provisionConfigsLocalDNS"},
						{cseConfigKubeletScriptFilepath, "provisionConfigsKubelet"},
						{cseConfigNetworkScriptFilepath, "provisionConfigsNetwork"},
						{cseConfigAddonsScriptFilepath, "provisionConfigsAddons"},
						{cseConfigChronyScriptFilepath, "provisionConfigsChrony"},
					}
					if testCase.distro == datamodel.AKSAzureLinuxV3Gen2 {
						expectedScripts[1].key = "provisionSourceMariner"
						expectedScripts[5].key = "provisionInstallsMariner"
					}

					expectedPaths := make([]string, 0, len(customData.WriteFiles))
					for _, script := range expectedScripts {
						expectedPaths = append(expectedPaths, script.path)
					}
					expectedPaths = append(expectedPaths,
						"/etc/systemd/system/reconcile-private-hosts.service",
						"/etc/systemd/system/kubelet.service",
						"/opt/azure-network/configure-azure-network.sh",
						"/etc/udev/rules.d/99-azure-network.rules",
					)
					actualPaths := make([]string, 0, len(customData.WriteFiles))
					for _, file := range customData.WriteFiles {
						actualPaths = append(actualPaths, file.Path)
					}
					Expect(actualPaths).To(Equal(expectedPaths))

					filesByPath := make(map[string]cloudInitWriteFile, len(customData.WriteFiles))
					for _, file := range customData.WriteFiles {
						filesByPath[file.Path] = file
						Expect(file.Owner).To(Equal("root"))
					}
					for _, script := range expectedScripts {
						file := filesByPath[script.path]
						Expect(file.Permissions).To(Equal("0744"))
						Expect(file.Encoding).To(BeEmpty())
						expectedContent, ok := variables[script.key].(string)
						Expect(ok).To(BeTrue())
						Expect(file.Content).To(Equal(expectedContent), script.path)
					}
					expectedLiteralFiles := []struct {
						path        string
						key         string
						permissions string
					}{
						{"/etc/systemd/system/reconcile-private-hosts.service", "reconcilePrivateHostsService", "0644"},
						{"/etc/systemd/system/kubelet.service", "kubeletSystemdService", "0600"},
						{"/opt/azure-network/configure-azure-network.sh", "configureAzureNetworkScript", "0755"},
						{"/etc/udev/rules.d/99-azure-network.rules", "azureNetworkUdevRule", "0644"},
					}
					for _, expected := range expectedLiteralFiles {
						file := filesByPath[expected.path]
						Expect(file.Permissions).To(Equal(expected.permissions))
						Expect(file.Encoding).To(BeEmpty())
						expectedContent, ok := variables[expected.key].(string)
						Expect(ok).To(BeTrue())
						Expect(file.Content).To(Equal(expectedContent), expected.path)
					}
					By(fmt.Sprintf("%s customData size: encoded=%d decoded=%d", name, len(payload), len(customDataBytes)))
				}
			}
		})

		It("should accept supported custom Linux transparent huge page values", func() {
			config.AgentPoolProfile.CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
				TransparentHugePageEnabled: "never",
				TransparentHugePageDefrag:  "defer+madvise",
			}
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject unsupported custom Linux transparent huge page values", func() {
			testCases := []struct {
				name        string
				enabled     string
				defrag      string
				expectedErr string
			}{
				{
					name:        "enabled",
					enabled:     "within_size",
					expectedErr: "customLinuxOSConfig.transparentHugePageEnabled",
				},
				{
					name:        "defrag",
					defrag:      "within_size",
					expectedErr: "customLinuxOSConfig.transparentHugePageDefrag",
				},
			}
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			for _, tc := range testCases {
				configCopy, err := deepcopy.Anything(config)
				Expect(err).To(BeNil())
				testConfig, ok := configCopy.(*datamodel.NodeBootstrappingConfiguration)
				Expect(ok).To(BeTrue())
				testConfig.AgentPoolProfile.CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
					TransparentHugePageEnabled: tc.enabled,
					TransparentHugePageDefrag:  tc.defrag,
				}

				_, err = agentBaker.GetNodeBootstrapping(context.Background(), testConfig)
				Expect(err).To(MatchError(ContainSubstring(tc.expectedErr)), tc.name)
			}
		})

		It("should return the correct bootstrapping data when linux node image version override is present", func() {
			nodeImageVersionOverride := "202402.27.0"
			toggles := &testToggles{
				defaultNodeImageVersionOverride: nodeImageVersionOverride,
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
			Expect(nodeBootStrapping.SigImageConfig.Definition).To(Equal("2204gen2containerd"))
			Expect(nodeBootStrapping.SigImageConfig.Version).To(Equal(nodeImageVersionOverride))
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

		It("should not return an error for customized image", func() {
			config.AgentPoolProfile.Distro = datamodel.CustomizedImage
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error for customized kata image", func() {
			config.AgentPoolProfile.Distro = datamodel.CustomizedImageKata
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error for customized linuxguard image", func() {
			config.AgentPoolProfile.Distro = datamodel.CustomizedImageLinuxGuard
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error for customized trusted launch image", func() {
			config.AgentPoolProfile.Distro = datamodel.CustomizedImageTrustedLaunch
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error for customized windows image", func() {
			config.AgentPoolProfile.Distro = datamodel.CustomizedWindowsOSImage
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			_, err = agentBaker.GetNodeBootstrapping(context.Background(), config)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("GetLatestSigImageConfig", func() {
		It("should return correct value for existing distro", func() {
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

			sigImageConfig, err := agentBaker.GetLatestSigImageConfig(config.SIGConfig, datamodel.AKSUbuntuContainerd2404Gen2, &datamodel.EnvironmentInfo{
				SubscriptionID: config.SubscriptionID,
				TenantID:       config.TenantID,
				Region:         cs.Location,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(sigImageConfig.ResourceGroup).To(Equal("resourcegroup"))
			Expect(sigImageConfig.Gallery).To(Equal("aksubuntu"))
			Expect(sigImageConfig.Definition).To(Equal("2404gen2containerd"))
		})

		It("should return error if image config not found for distro", func() {
			agentBaker, err := NewAgentBaker()
			Expect(err).NotTo(HaveOccurred())

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
			ubuntuDistros             []datamodel.Distro
			marinerDistros            []datamodel.Distro
			azureLinuxDistros         []datamodel.Distro
			flatcarDistros            []datamodel.Distro
			aclDistros                []datamodel.Distro
			ubuntuEdgeZoneDistros     []datamodel.Distro
			azureLinuxEdgeZoneDistros []datamodel.Distro
			allLinuxDistros           []datamodel.Distro
		)

		BeforeEach(func() {
			ubuntuDistros = []datamodel.Distro{
				datamodel.AKSUbuntuFipsContainerd2004,
				datamodel.AKSUbuntuFipsContainerd2004Gen2,
				datamodel.AKSUbuntuContainerd2204,
				datamodel.AKSUbuntuContainerd2204Gen2,
				datamodel.AKSUbuntuContainerd2004CVMGen2,
				datamodel.AKSUbuntuArm64Containerd2204Gen2,
				datamodel.AKSUbuntuContainerd2204TLGen2,
				datamodel.AKSUbuntuContainerd2404CVMGen2,
				datamodel.AKSUbuntuContainerd2404Gen2,
				datamodel.AKSUbuntuArm64Containerd2404Gen2,
				datamodel.AKSUbuntuContainerd2404,
				datamodel.AKSUbuntuContainerd2404TLGen2,
				datamodel.AKSUbuntuMinimalContainerd2604CVMGen2,
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
				datamodel.AKSAzureLinuxV3Gen2Kata,
				datamodel.AKSAzureLinuxV2Arm64Gen2,
				datamodel.AKSAzureLinuxV3Arm64Gen2,
				datamodel.AKSAzureLinuxV2Gen2TL,
				datamodel.AKSAzureLinuxV3Arm64Gen2FIPS,
				datamodel.AKSAzureLinuxV3CVMGen2,
			}

			flatcarDistros = []datamodel.Distro{
				datamodel.AKSFlatcarGen2,
				datamodel.AKSFlatcarArm64Gen2,
			}

			aclDistros = []datamodel.Distro{
				datamodel.AKSACLGen2TL,
				datamodel.AKSACLArm64Gen2TL,
				datamodel.AKSACLGen2FIPSTL,
				datamodel.AKSACLArm64Gen2FIPSTL,
				datamodel.AKSACLCVMGen2,
			}

			ubuntuEdgeZoneDistros = []datamodel.Distro{
				datamodel.AKSUbuntuEdgeZoneContainerd2204,
				datamodel.AKSUbuntuEdgeZoneContainerd2204Gen2,
				datamodel.AKSUbuntuEdgeZoneContainerd2404,
				datamodel.AKSUbuntuEdgeZoneContainerd2404Gen2,
			}

			azureLinuxEdgeZoneDistros = []datamodel.Distro{
				datamodel.AKSAzureLinuxV3EdgeZone,
				datamodel.AKSAzureLinuxV3EdgeZoneGen2,
			}

			allLinuxDistros = append(allLinuxDistros, ubuntuDistros...)
			allLinuxDistros = append(allLinuxDistros, marinerDistros...)
			allLinuxDistros = append(allLinuxDistros, azureLinuxDistros...)
			allLinuxDistros = append(allLinuxDistros, flatcarDistros...)
			allLinuxDistros = append(allLinuxDistros, aclDistros...)
		})

		It("should return correct value for all existing distros", func() {
			// implicitly constructed with default, empty toggle set
			agentBaker, err := NewAgentBaker()
			Expect(err).To(BeNil())

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
				Expect(config.Version).ToNot(BeEmpty())
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

			for _, distro := range flatcarDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("aksflatcar"))
			}

			for _, distro := range aclDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("aksazurelinux"))
			}

			// EdgeZone distros use a hardcoded gallery and resource group (not the
			// gallery config map), so they are validated separately from allLinuxDistros.
			for _, distro := range ubuntuEdgeZoneDistros {
				Expect(configs).To(HaveKey(distro))
				config := configs[distro]
				Expect(config.SubscriptionID).To(Equal("somesubid"))
				Expect(config.Version).ToNot(BeEmpty())
				Expect(config.Definition).ToNot(BeEmpty())
				Expect(config.Gallery).To(Equal("AKSUbuntuEdgeZone"))
				Expect(config.ResourceGroup).To(Equal("AKS-Ubuntu-EdgeZone"))
			}

			for _, distro := range azureLinuxEdgeZoneDistros {
				Expect(configs).To(HaveKey(distro))
				config := configs[distro]
				Expect(config.SubscriptionID).To(Equal("somesubid"))
				Expect(config.Version).ToNot(BeEmpty())
				Expect(config.Definition).ToNot(BeEmpty())
				Expect(config.Gallery).To(Equal("AKSAzureLinuxEdgeZone"))
				Expect(config.ResourceGroup).To(Equal("AKS-AzureLinux-EdgeZone"))
			}
		})

		It("should return correct value for all existing distros with linux node image version override", func() {
			var (
				ubuntuOverrideVersion     = "202402.25.0"
				marinerOverrideVersion    = "202402.25.1"
				azureLinuxOverrideVersion = "202402.25.2"
				flatcarOverrideVersion    = "202402.25.2"
				aclOverrideVersion        = "202402.25.2"
			)
			imageVersionOverrides := map[datamodel.Distro]string{}
			for _, distro := range ubuntuDistros {
				imageVersionOverrides[distro] = ubuntuOverrideVersion
			}
			for _, distro := range marinerDistros {
				imageVersionOverrides[distro] = marinerOverrideVersion
			}
			for _, distro := range azureLinuxDistros {
				imageVersionOverrides[distro] = azureLinuxOverrideVersion
			}
			for _, distro := range flatcarDistros {
				imageVersionOverrides[distro] = flatcarOverrideVersion
			}
			for _, distro := range aclDistros {
				imageVersionOverrides[distro] = aclOverrideVersion
			}
			for _, distro := range ubuntuEdgeZoneDistros {
				imageVersionOverrides[distro] = ubuntuOverrideVersion
			}
			for _, distro := range azureLinuxEdgeZoneDistros {
				imageVersionOverrides[distro] = azureLinuxOverrideVersion
			}
			toggles := &testToggles{
				nodeImageVersionOverrides: imageVersionOverrides,
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

			for _, distro := range flatcarDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("aksflatcar"))
				Expect(config.Version).To(Equal(flatcarOverrideVersion))
			}

			for _, distro := range aclDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("aksazurelinux"))
				Expect(config.Version).To(Equal(aclOverrideVersion))
			}

			// Validate the version override is applied to EdgeZone distros too.
			for _, distro := range ubuntuEdgeZoneDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("AKSUbuntuEdgeZone"))
				Expect(config.ResourceGroup).To(Equal("AKS-Ubuntu-EdgeZone"))
				Expect(config.Version).To(Equal(ubuntuOverrideVersion))
			}

			for _, distro := range azureLinuxEdgeZoneDistros {
				config := configs[distro]
				Expect(config.Gallery).To(Equal("AKSAzureLinuxEdgeZone"))
				Expect(config.ResourceGroup).To(Equal("AKS-AzureLinux-EdgeZone"))
				Expect(config.Version).To(Equal(azureLinuxOverrideVersion))
			}
		})

	})
})
