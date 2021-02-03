package agent

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func generateTestData() bool {
	return os.Getenv("GENERATE_TEST_DATA") == "true"
}

var _ = Describe("Assert generated customData and cseCmd", func() {
	DescribeTable("Generated customData and CSE", func(folder, k8sVersion string, configUpdator func(*datamodel.NodeBootstrappingConfiguration)) {
		cs := &datamodel.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: k8sVersion,
					KubernetesConfig: &datamodel.KubernetesConfig{
						KubeletConfig: map[string]string{
							"--feature-gates": "RotateKubeletServerCertificate=true,a=b, PodPriority=true, x=y",
						},
					},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					DNSPrefix: "uttestdom",
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "agent2",
						Count:               3,
						VMSize:              "Standard_DS1_v2",
						StorageProfile:      "ManagedDisks",
						OSType:              datamodel.Linux,
						VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet1",
						AvailabilityProfile: datamodel.VirtualMachineScaleSets,
						KubernetesConfig: &datamodel.KubernetesConfig{
							KubeletConfig: map[string]string{
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
							},
						},
						Distro: datamodel.AKSUbuntu1604,
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

		// AKS always pass in te customHyperKubeImage to aks-e, so we don't really rely on
		// the default component version for "hyperkube", which is not set since 1.17
		if IsKubernetesVersionGe(k8sVersion, "1.17.0") {
			cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage = fmt.Sprintf("k8s.gcr.io/hyperkube-amd64:v%v", k8sVersion)
		}

		agentPool := cs.Properties.AgentPoolProfiles[0]
		baker := InitializeTemplateGenerator()

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
		}

		if configUpdator != nil {
			configUpdator(config)
		}

		// customData
		base64EncodedCustomData := baker.GetNodeBootstrappingPayload(config)
		customDataBytes, err := base64.StdEncoding.DecodeString(base64EncodedCustomData)
		customData := string(customDataBytes)
		Expect(err).To(BeNil())

		if generateTestData() {
			backfillCustomData(folder, customData)
		}

		expectedCustomData, err := ioutil.ReadFile(fmt.Sprintf("./testdata/%s/CustomData", folder))
		if err != nil {
			panic(err)
		}
		Expect(customData).To(Equal(string(expectedCustomData)))

		// CSE
		cseCommand := baker.GetNodeBootstrappingCmd(config)
		if generateTestData() {
			ioutil.WriteFile(fmt.Sprintf("./testdata/%s/CSECommand", folder), []byte(cseCommand), 0644)
		}
		expectedCSECommand, err := ioutil.ReadFile(fmt.Sprintf("./testdata/%s/CSECommand", folder))
		if err != nil {
			panic(err)
		}
		Expect(cseCommand).To(Equal(string(expectedCSECommand)))

	}, Entry("AKSUbuntu1604 with k8s version less than 1.18", "AKSUbuntu1604+K8S115", "1.15.7", nil),
		Entry("AKSUbuntu1604 with k8s version 1.18", "AKSUbuntu1604+K8S118", "1.18.2", nil),
		Entry("AKSUbuntu1604 with k8s version 1.17", "AKSUbuntu1604+K8S117", "1.17.7", nil),
		Entry("AKSUbuntu1604 with temp disk (toggle)", "AKSUbuntu1604+TempDiskToggle", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			// this tests prioritization of the new api property vs the old property i'd like to remove.
			// ContainerRuntimeConfig should take priority until we remove it entirely
			config.AgentPoolProfile.KubeletDiskType = datamodel.OSDisk
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntimeConfig: map[string]string{
					datamodel.ContainerDataDirKey: "/mnt/containers",
				},
			}
		}),
		Entry("AKSUbuntu1604 with temp disk (api field)", "AKSUbuntu1604+TempDiskExplicit", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			// also tests prioritization, but now the API property should take precedence
			config.AgentPoolProfile.KubeletDiskType = datamodel.TempDisk
		}),
		Entry("AKSUbuntu1604 with OS disk", "AKSUbuntu1604+OSKubeletDisk", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			// also tests prioritization, but now the API property should take precedence
			config.AgentPoolProfile.KubeletDiskType = datamodel.OSDisk
		}),
		Entry("AKSUbuntu1604 with Temp Disk and containerd", "AKSUbuntu1604+TempDisk+Containerd", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntimeConfig: map[string]string{
					datamodel.ContainerDataDirKey: "/mnt/containers",
				},
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				KubeletConfig:    map[string]string{},
				ContainerRuntime: datamodel.Containerd,
			}
		}),
		Entry("AKSUbuntu1604 with RawUbuntu", "RawUbuntu", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.Ubuntu
		}),
		Entry("AKSUbuntu1604 EnablePrivateClusterHostsConfigAgent", "AKSUbuntu1604+EnablePrivateClusterHostsConfigAgent", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			cs := config.ContainerService
			if cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster == nil {
				cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &datamodel.PrivateCluster{EnableHostsConfigAgent: to.BoolPtr(true)}
			} else {
				cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent = to.BoolPtr(true)
			}
		}),
		Entry("AKSUbuntu1804 with GPU dedicated VHD", "AKSUbuntu1604+GPUDedicatedVHD", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuGPU1804
			config.AgentPoolProfile.VMSize = "Standard_NC6"
			config.ConfigGPUDriverIfNeeded = false
			config.EnableGPUDevicePluginIfNeeded = true
			config.EnableNvidia = true
		}),
		Entry("AKSUbuntu1604 with KubeletConfigFile", "AKSUbuntu1604+KubeletConfigFile", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableKubeletConfigFile = true
		}),

		Entry("AKSUbuntu1804 with containerd and private ACR", "AKSUbuntu1804+Containerd+PrivateACR", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				KubeletConfig:    map[string]string{},
				ContainerRuntime: datamodel.Containerd,
			}
			cs := config.ContainerService
			if cs.Properties.OrchestratorProfile.KubernetesConfig == nil {
				cs.Properties.OrchestratorProfile.KubernetesConfig = &datamodel.KubernetesConfig{}
			}
			cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateAzureRegistryServer = "acr.io/privateacr"
			cs.Properties.ServicePrincipalProfile = &datamodel.ServicePrincipalProfile{
				ClientID: "clientID",
				Secret:   "clientSecret",
			}
		}),
		Entry("AKSUbuntu1804 with containerd and GPU SKU", "AKSUbuntu1804+Containerd+NSeriesSku", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {

			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				KubeletConfig:    map[string]string{},
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
			config.EnableNvidia = true
		}),
		Entry("AKSUbuntu1804 with containerd and kubenet cni", "AKSUbuntu1804+Containerd+Kubenet", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				KubeletConfig:    map[string]string{},
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
		}),
		Entry("AKSUbuntu1804 with containerd and kubenet cni and calico policy", "AKSUbuntu1804+Containerd+Kubenet+Calico", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				KubeletConfig:    map[string]string{},
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyCalico
		}),
		Entry("AKSUbuntu1804 with containerd and teleport enabled", "AKSUbuntu1804+Containerd+Teleport", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableACRTeleportPlugin = true
			config.TeleportdPluginURL = "some url"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				KubeletConfig:    map[string]string{},
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
		}),

		Entry("AKSUbuntu1804 with containerd and ipmasqagent enabled", "AKSUbuntu1804+Containerd+IPMasqAgent", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableACRTeleportPlugin = true
			config.TeleportdPluginURL = "some url"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				KubeletConfig:    map[string]string{},
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
			config.ContainerService.Properties.HostedMasterProfile.IPMasqAgent = true
		}),

		Entry("AKSUbuntu1604 with custom kubeletConfig and osConfig", "AKSUbuntu1604+CustomKubeletConfig+CustomLinuxOSConfig", "1.16.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableKubeletConfigFile = false
			netIpv4TcpTwReuse := true
			failSwapOn := false
			var swapFileSizeMB int32 = 1500
			var netCoreSomaxconn int32 = 1638499
			config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
				CPUManagerPolicy:      "static",
				CPUCfsQuota:           to.BoolPtr(false),
				CPUCfsQuotaPeriod:     "200ms",
				ImageGcHighThreshold:  to.Int32Ptr(90),
				ImageGcLowThreshold:   to.Int32Ptr(70),
				TopologyManagerPolicy: "best-effort",
				AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
				FailSwapOn:            &failSwapOn,
				ContainerLogMaxSizeMB: to.Int32Ptr(1000),
				ContainerLogMaxFiles:  to.Int32Ptr(99),
				PodMaxPids:            to.Int32Ptr(12345),
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
				Sysctls: &datamodel.SysctlConfig{
					NetCoreSomaxconn:             &netCoreSomaxconn,
					NetCoreRmemDefault:           to.Int32Ptr(456000),
					NetCoreWmemDefault:           to.Int32Ptr(89000),
					NetIpv4TcpTwReuse:            &netIpv4TcpTwReuse,
					NetIpv4IpLocalPortRange:      "32768 60999",
					NetIpv4TcpMaxSynBacklog:      to.Int32Ptr(1638498),
					NetIpv4NeighDefaultGcThresh1: to.Int32Ptr(10001),
					NetIpv4NeighDefaultGcThresh2: to.Int32Ptr(10002),
					NetIpv4NeighDefaultGcThresh3: to.Int32Ptr(10003),
				},
				TransparentHugePageEnabled: "never",
				TransparentHugePageDefrag:  "defer+madvise",
				SwapFileSizeMB:             &swapFileSizeMB,
			}
		}),

		Entry("AKSUbuntu1804 with kubelet client TLS bootstrapping enabled", "AKSUbuntu1804+KubeletClientTLSBootstrapping", "1.18.3", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableKubeletClientTLSBootstrapping = true
			config.ContainerService.Properties.AgentPoolProfiles[0].TLSBootstrapToken = &datamodel.TLSBootstrapToken{
				TokenID:     "07401b",
				TokenSecret: "f395accd246ae52d",
			}
		}))

})

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
						KubeletConfig: map[string]string{
							"--feature-gates": "RotateKubeletServerCertificate=true,a=b, PodPriority=true, x=y",
						},
						UseInstanceMetadata: to.BoolPtr(true),
						DNSServiceIP:        "10.0.0.10",
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
						Count:               3,
						VMSize:              "Standard_D2s_v3",
						StorageProfile:      "ManagedDisks",
						OSType:              datamodel.Windows,
						VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-36873793/subnet/aks-subnet",
						WindowsNameVersion:  "v2",
						AvailabilityProfile: datamodel.VirtualMachineScaleSets,
						CustomNodeLabels:    map[string]string{"kubernetes.azure.com/node-image-version": "AKSWindows-2019-17763.1577.201111"},
						KubernetesConfig: &datamodel.KubernetesConfig{
							KubeletConfig: map[string]string{
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
								"--image-pull-progress-deadline":      "20m",
								"--keep-terminated-pod-volumes":       "false",
								"--kube-reserved":                     "cpu=100m,memory=1843Mi",
								"--kubeconfig":                        "c:\\k\\config",
								"--max-pods":                          "30",
								"--network-plugin":                    "cni",
								"--node-status-update-frequency":      "10s",
								"--non-masquerade-cidr":               "0.0.0.0/0",
								"--pod-infra-container-image":         "kubletwin/pause",
								"--pod-max-pids":                      "-1",
								"--read-only-port":                    "0",
								"--resolv-conf":                       `""`,
								"--rotate-certificates":               "false",
								"--streaming-connection-idle-timeout": "4h",
								"--system-reserved":                   "memory=2Gi",
								"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256",
							},
						},
						Distro: datamodel.Distro("aks-windows-2019"),
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
		baker := InitializeTemplateGenerator()

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
		}

		if configUpdator != nil {
			configUpdator(config)
		}

		// customData
		base64EncodedCustomData := baker.GetNodeBootstrappingPayload(config)
		customDataBytes, err := base64.StdEncoding.DecodeString(base64EncodedCustomData)
		customData := string(customDataBytes)
		Expect(err).To(BeNil())

		if generateTestData() {
			backfillCustomData(folder, customData)
		}

		expectedCustomData, err := ioutil.ReadFile(fmt.Sprintf("./testdata/%s/CustomData", folder))
		if err != nil {
			panic(err)
		}
		Expect(customData).To(Equal(string(expectedCustomData)))

		// CSE
		cseCommand := baker.GetNodeBootstrappingCmd(config)
		if generateTestData() {
			ioutil.WriteFile(fmt.Sprintf("./testdata/%s/CSECommand", folder), []byte(cseCommand), 0644)
		}
		expectedCSECommand, err := ioutil.ReadFile(fmt.Sprintf("./testdata/%s/CSECommand", folder))
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
			config.ContainerService.Properties.AgentPoolProfiles[0].Subnet = "172.17.2.0/24"
			config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID = "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet2"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig.KubeletConfig["--cluster-dns"] = "172.17.255.10"
		}),
		Entry("AKSWindows2019 with Managed Identity", "AKSWindows2019+ManagedIdentity", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.ServicePrincipalProfile = &datamodel.ServicePrincipalProfile{ClientID: "msi"}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UserAssignedID = "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/k8s-agentpool"
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
		Entry("AKSWindows2019 EnablePrivateClusterHostsConfigAgent", "AKSWindows2019+EnablePrivateClusterHostsConfigAgent", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			cs := config.ContainerService
			if cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster == nil {
				cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &datamodel.PrivateCluster{EnableHostsConfigAgent: to.BoolPtr(true)}
			} else {
				cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent = to.BoolPtr(true)
			}
		}),
		Entry("AKSWindows2004 with k8s version 1.19 and hyperv", "AKSWindows2004+K8S119+hyperv", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.Distro("aks-windows-2019")
			config.ContainerService.Properties.AgentPoolProfiles[0].CustomNodeLabels = map[string]string{"kubernetes.azure.com/node-image-version": "AKSWindows-2004-17763.1457.201019"}
			config.ContainerService.Properties.AgentPoolProfiles[0].ImageRef = &datamodel.ImageReference{
				Name:           "windows-2004",
				ResourceGroup:  "akswinvhdbuilderrg",
				SubscriptionID: "109a5e88-712a-48ae-9078-9ca8b3c81345",
				Gallery:        "AKSWindows",
				Version:        "17763.1457.201019",
			}
		}))

})

func backfillCustomData(folder, customData string) {
	if _, err := os.Stat(fmt.Sprintf("./testdata/%s", folder)); os.IsNotExist(err) {
		e := os.MkdirAll(fmt.Sprintf("./testdata/%s", folder), 0755)
		Expect(e).To(BeNil())
	}
	ioutil.WriteFile(fmt.Sprintf("./testdata/%s/CustomData", folder), []byte(customData), 0644)
	if strings.Contains(folder, "AKSWindows") {
		return
	}
	err := exec.Command("/bin/sh", "-c", fmt.Sprintf("./testdata/convert.sh testdata/%s", folder)).Run()
	Expect(err).To(BeNil())
}

var _ = Describe("Test normalizeResourceGroupNameForLabel", func() {
	It("should return the correct normalized resource group name", func() {
		Expect(normalizeResourceGroupNameForLabel("hello")).To(Equal("hello"))
		Expect(normalizeResourceGroupNameForLabel("hel(lo")).To(Equal("hel-lo"))
		Expect(normalizeResourceGroupNameForLabel("hel)lo")).To(Equal("hel-lo"))
		var s string
		for i := 0; i < 63; i++ {
			s += "0"
		}
		Expect(normalizeResourceGroupNameForLabel(s)).To(Equal(s))
		Expect(normalizeResourceGroupNameForLabel(s + "1")).To(Equal(s))

		s = ""
		for i := 0; i < 62; i++ {
			s += "0"
		}
		Expect(normalizeResourceGroupNameForLabel(s + "(")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + ")")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + "-")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + "_")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + ".")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel("")).To(Equal(""))
		Expect(normalizeResourceGroupNameForLabel("z")).To(Equal("z"))

		// Add z, not replacing ending - with z, if name is short
		Expect(normalizeResourceGroupNameForLabel("-")).To(Equal("-z"))

		s = ""
		for i := 0; i < 61; i++ {
			s += "0"
		}
		Expect(normalizeResourceGroupNameForLabel(s + "-")).To(Equal(s + "-z"))
	})
})
