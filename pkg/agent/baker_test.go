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
			"--container-log-max-size":            "50M",
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
			FIPSEnabled:                   false,
			KubeletConfig:                 kubeletConfig,
			PrimaryScaleSetName:           "aks-agent2-36873793-vmss",
			IsARM64:                       false,
			DisableUnattendedUpgrades:     false,
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

			config.KubeletConfig = map[string]string{}
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
				ContainerRuntime: datamodel.Containerd,
			}

			config.KubeletConfig = map[string]string{}
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
				ContainerRuntime: datamodel.Containerd,
			}

			config.KubeletConfig = map[string]string{}
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
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
			config.EnableNvidia = true
			config.KubeletConfig = map[string]string{}
		}),
		Entry("AKSUbuntu1804 with containerd and kubenet cni", "AKSUbuntu1804+Containerd+Kubenet", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
			config.KubeletConfig = map[string]string{}
		}),
		Entry("AKSUbuntu1804 with containerd and kubenet cni and calico policy", "AKSUbuntu1804+Containerd+Kubenet+Calico", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyCalico
			config.KubeletConfig = map[string]string{}
		}),
		Entry("AKSUbuntu1804 with containerd and teleport enabled", "AKSUbuntu1804+Containerd+Teleport", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableACRTeleportPlugin = true
			config.TeleportdPluginURL = "some url"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1804 with containerd and ipmasqagent enabled", "AKSUbuntu1804+Containerd+IPMasqAgent", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableACRTeleportPlugin = true
			config.TeleportdPluginURL = "some url"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
			config.ContainerService.Properties.HostedMasterProfile.IPMasqAgent = true
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1804 with containerd and version specified", "AKSUbuntu1804+Containerd+ContainerdVersion", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerdVersion = "1.4.4"
			config.KubeletConfig = map[string]string{}
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

		Entry("AKSUbuntu1604 - dynamic-config-dir should always be removed with custom kubelet config", "AKSUbuntu1604+CustomKubeletConfig+DynamicKubeletConfig", "1.16.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
				CPUManagerPolicy:      "static",
				CPUCfsQuota:           to.BoolPtr(false),
				CPUCfsQuotaPeriod:     "200ms",
				ImageGcHighThreshold:  to.Int32Ptr(90),
				ImageGcLowThreshold:   to.Int32Ptr(70),
				TopologyManagerPolicy: "best-effort",
				AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
				ContainerLogMaxSizeMB: to.Int32Ptr(1000),
				ContainerLogMaxFiles:  to.Int32Ptr(99),
				PodMaxPids:            to.Int32Ptr(12345),
			}
			config.KubeletConfig = map[string]string{
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
				"--dynamic-config-dir":                "",
			}
		}),

		Entry("AKSUbuntu1604 - dynamic-config-dir should always be removed", "AKSUbuntu1604+DynamicKubeletConfig", "1.16.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig = map[string]string{
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
				"--dynamic-config-dir":                "",
			}
		}),

		Entry("RawUbuntu with Containerd", "RawUbuntuContainerd", "1.19.1", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.Ubuntu
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1604 with Disable1804SystemdResolved=true", "AKSUbuntu1604+Disable1804SystemdResolved=true", "1.16.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.Disable1804SystemdResolved = true
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Docker,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1604 with Disable1804SystemdResolved=false", "AKSUbuntu1604+Disable1804SystemdResolved=false", "1.16.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.Disable1804SystemdResolved = false
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Docker,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1804 with Disable1804SystemdResolved=true", "AKSUbuntu1804+Disable1804SystemdResolved=true", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.Disable1804SystemdResolved = true
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1804 with Disable1804SystemdResolved=false", "AKSUbuntu1804+Disable1804SystemdResolved=false", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.Disable1804SystemdResolved = false
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1804 with kubelet client TLS bootstrapping enabled", "AKSUbuntu1804+KubeletClientTLSBootstrapping", "1.18.3", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletClientTLSBootstrapToken = to.StringPtr("07401b.f395accd246ae52d")
		}),

		Entry("Mariner v2 with kata", "MarinerV2+Kata", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}),

		Entry("AKSUbuntu1804 with containerd and kubenet cni", "AKSUbuntu1804+Containerd+Kubenet+FIPSEnabled", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
			config.FIPSEnabled = true
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1804 with http proxy config", "AKSUbuntu1804+HTTPProxy", "1.18.14", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.HTTPProxyConfig = &datamodel.HTTPProxyConfig{
				HTTPProxy:  to.StringPtr("http://myproxy.server.com:8080/"),
				HTTPSProxy: to.StringPtr("https://myproxy.server.com:8080/"),
				NoProxy: to.StringSlicePtr([]string{
					"localhost",
					"127.0.0.1",
				}),
				TrustedCA: to.StringPtr(EncodedTestCert),
			}
		}),

		Entry("AKSUbuntu1804 with custom ca trust", "AKSUbuntu1804+CustomCATrust", "1.18.14", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
				CustomCATrustCerts: []string{EncodedTestCert, EncodedTestCert, EncodedTestCert},
			}
		}),

		Entry("AKSUbuntu1804 with containerd and runcshimv2", "AKSUbuntu1804+Containerd+runcshimv2", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableRuncShimV2 = true
		}),

		Entry("AKSUbuntu1804 with containerd and motd", "AKSUbuntu1804+Containerd+MotD", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].MessageOfTheDay = "Zm9vYmFyDQo=" // foobar in b64
		}),

		Entry("AKSUbuntu1804containerd with custom runc verison", "AKSUbuntu1804Containerd+RuncVersion", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.RuncVersion = "1.0.0-rc96"
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1804 with containerd+gpu and runcshimv2", "AKSUbuntu1804+Containerd++GPU+runcshimv2", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.AgentPoolProfile.VMSize = "Standard_NC6"
			config.EnableNvidia = true
			config.EnableRuncShimV2 = true
			config.KubeletConfig = map[string]string{}
		}),

		Entry("AKSUbuntu1804 containerd with multi-instance GPU", "AKSUbuntu1804+Containerd+MIG", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.KubeletConfig = map[string]string{}
			config.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
			config.EnableNvidia = true
			config.GPUInstanceProfile = "MIG7g"
		}),

		Entry("AKSUbuntu1804 containerd with multi-instance non-fabricmanager GPU", "AKSUbuntu1804+Containerd+MIG+NoFabricManager", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.KubeletConfig = map[string]string{}
			config.AgentPoolProfile.VMSize = "Standard_NC24ads_A100_v4"
			config.EnableNvidia = true
			config.GPUInstanceProfile = "MIG7g"
		}),

		Entry("AKSUbuntu1804 with krustlet", "AKSUbuntu1804+krustlet", "1.20.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].WorkloadRuntime = datamodel.WasmWasi
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.CertificateProfile = &datamodel.CertificateProfile{
				CaCertificate: "fooBarBaz",
			}
			config.KubeletClientTLSBootstrapToken = to.StringPtr("07401b.f395accd246ae52d")
		}),
		Entry("AKSUbuntu1804 with NoneCNI", "AKSUbuntu1804+NoneCNI", "1.20.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = datamodel.NetworkPluginNone
		}),
		Entry("AKSUbuntu1804 with Containerd and certs.d", "AKSUbuntu1804+Containerd+Certsd", "1.22.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}),
		Entry("AKSUbuntu1804ARM64containerd with kubenet", "AKSUbuntu1804ARM64Containerd+NoCustomKubeImageandBinaries", "1.22.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.OrchestratorType = "azure"
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/1.22.2/binaries/kubernetes-node-linux-arm64.tar.gz"
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeProxyImage = "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.22.2"
			config.IsARM64 = true
			config.KubeletConfig = map[string]string{}
		}),
		Entry("AKSUbuntu1804ARM64containerd with kubenet", "AKSUbuntu1804ARM64Containerd+CustomKubeImageandBinaries", "1.22.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.OrchestratorType = datamodel.Kubernetes
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/1.22.2/binaries/kubernetes-node-linux-arm64.tar.gz"
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeProxyImage = "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.22.2"
			config.IsARM64 = true
			config.KubeletConfig = map[string]string{}
		}),
		Entry("AKSUbuntu1804 with IPAddress and FQDN", "AKSUbuntu1804+Containerd+IPAddress+FQDN", "1.22.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.HostedMasterProfile.FQDN = "a.hcp.eastus.azmk8s.io"
			config.ContainerService.Properties.HostedMasterProfile.IPAddress = "1.2.3.4"
		}),
		Entry("AKSUbuntu2204 VHD, cgroupv2", "AKSUbuntu2204+cgroupv2", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
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
			"--keep-terminated-pod-volumes":       "false",
			"--kube-reserved":                     "cpu=100m,memory=1843Mi",
			"--kubeconfig":                        "c:\\k\\config",
			"--max-pods":                          "30",
			"--network-plugin":                    "cni",
			"--node-status-update-frequency":      "10s",
			"--pod-infra-container-image":         "kubletwin/pause",
			"--pod-max-pids":                      "-1",
			"--read-only-port":                    "0",
			"--resolv-conf":                       `""`,
			"--rotate-certificates":               "false",
			"--streaming-connection-idle-timeout": "4h",
			"--system-reserved":                   "memory=2Gi",
			"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256",
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
			config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID = "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet2"
			config.KubeletConfig["--cluster-dns"] = "172.17.255.10"
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
		Entry("AKSWindows2019 with kubelet client TLS bootstrapping enabled", "AKSWindows2019+KubeletClientTLSBootstrapping", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletClientTLSBootstrapToken = to.StringPtr("07401b.f395accd246ae52d")
		}),
		Entry("AKSWindows2019 with k8s version 1.19 + FIPS", "AKSWindows2019+K8S119+FIPS", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.FIPSEnabled = true
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

var _ = Describe("getGPUDriverVersion", func() {
	It("should use 470 with nc v1", func() {
		Expect(getGPUDriverVersion("standard_nc6")).To(Equal("cuda-470.82.01"))
	})
	It("should use 510 cuda with nc v3", func() {
		Expect(getGPUDriverVersion("standard_nc6_v3")).To(Equal("cuda-510.47.03"))
	})
	It("should use 510 grid with nv v5", func() {
		Expect(getGPUDriverVersion("standard_nv6ads_a10_v5")).To(Equal("grid-510.73.08"))
		Expect(getGPUDriverVersion("Standard_nv36adms_A10_V5")).To(Equal("grid-510.73.08"))
	})
	It("should use 510 cuda with nv v1 (although we don't know if that works)", func() {
		Expect(getGPUDriverVersion("standard_nv6")).To(Equal("cuda-510.47.03"))
	})
})

var EncodedTestCert string = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUgvVENDQmVXZ0F3SUJBZ0lRYUJZRTMvTTA4WEhZQ25OVm1jRkJjakFOQmdrcWhraUc5dzBCQVFzRkFEQnkKTVFzd0NRWURWUVFHRXdKVlV6RU9NQXdHQTFVRUNBd0ZWR1Y0WVhNeEVEQU9CZ05WQkFjTUIwaHZkWE4wYjI0eApFVEFQQmdOVkJBb01DRk5UVENCRGIzSndNUzR3TEFZRFZRUUREQ1ZUVTB3dVkyOXRJRVZXSUZOVFRDQkpiblJsCmNtMWxaR2xoZEdVZ1EwRWdVbE5CSUZJek1CNFhEVEl3TURRd01UQXdOVGd6TTFvWERUSXhNRGN4TmpBd05UZ3oKTTFvd2diMHhDekFKQmdOVkJBWVRBbFZUTVE0d0RBWURWUVFJREFWVVpYaGhjekVRTUE0R0ExVUVCd3dIU0c5MQpjM1J2YmpFUk1BOEdBMVVFQ2d3SVUxTk1JRU52Y25BeEZqQVVCZ05WQkFVVERVNVdNakF3T0RFMk1UUXlORE14CkZEQVNCZ05WQkFNTUMzZDNkeTV6YzJ3dVkyOXRNUjB3R3dZRFZRUVBEQlJRY21sMllYUmxJRTl5WjJGdWFYcGgKZEdsdmJqRVhNQlVHQ3lzR0FRUUJnamM4QWdFQ0RBWk9aWFpoWkdFeEV6QVJCZ3NyQmdFRUFZSTNQQUlCQXhNQwpWVk13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRREhoZVJrYmIxRkNjN3hSS3N0CndLMEpJR2FLWTh0N0piUzJiUTJiNllJSkRnbkh1SVlIcUJyQ1VWNzlvZWxpa2tva1JrRnZjdnBhS2luRkhEUUgKVXBXRUk2UlVFUlltU0NnM084V2k0MnVPY1YyQjVaYWJtWENrd2R4WTVFY2w1MUJiTThVbkdkb0FHYmRObWlSbQpTbVRqY3MrbGhNeGc0ZkZZNmxCcGlFVkZpR1VqR1JSKzYxUjY3THo2VTRLSmVMTmNDbTA3UXdGWUtCbXBpMDhnCmR5Z1N2UmRVdzU1Sm9wcmVkaitWR3RqVWtCNGhGVDRHUVgvZ2h0NjlSbHF6Lys4dTBkRVFraHVVdXVjcnFhbG0KU0d5NDNIUndCZkRLRndZZVdNN0NQTWQ1ZS9kTyt0MDh0OFBianpWVFR2NWhRRENzRVlJVjJUN0FGSTlTY054TQpraDcvQWdNQkFBR2pnZ05CTUlJRFBUQWZCZ05WSFNNRUdEQVdnQlMvd1ZxSC95ajZRVDM5dDAva0hhK2dZVmdwCnZUQi9CZ2dyQmdFRkJRY0JBUVJ6TUhFd1RRWUlLd1lCQlFVSE1BS0dRV2gwZEhBNkx5OTNkM2N1YzNOc0xtTnYKYlM5eVpYQnZjMmwwYjNKNUwxTlRUR052YlMxVGRXSkRRUzFGVmkxVFUwd3RVbE5CTFRRd09UWXRVak11WTNKMApNQ0FHQ0NzR0FRVUZCekFCaGhSb2RIUndPaTh2YjJOemNITXVjM05zTG1OdmJUQWZCZ05WSFJFRUdEQVdnZ3QzCmQzY3VjM05zTG1OdmJZSUhjM05zTG1OdmJUQmZCZ05WSFNBRVdEQldNQWNHQldlQkRBRUJNQTBHQ3lxRWFBR0cKOW5jQ0JRRUJNRHdHRENzR0FRUUJncWt3QVFNQkJEQXNNQ29HQ0NzR0FRVUZCd0lCRmg1b2RIUndjem92TDNkMwpkeTV6YzJ3dVkyOXRMM0psY0c5emFYUnZjbmt3SFFZRFZSMGxCQll3RkFZSUt3WUJCUVVIQXdJR0NDc0dBUVVGCkJ3TUJNRWdHQTFVZEh3UkJNRDh3UGFBN29EbUdOMmgwZEhBNkx5OWpjbXh6TG5OemJDNWpiMjB2VTFOTVkyOXQKTFZOMVlrTkJMVVZXTFZOVFRDMVNVMEV0TkRBNU5pMVNNeTVqY213d0hRWURWUjBPQkJZRUZBREFGVUlhenc1cgpaSUhhcG5SeElVbnB3K0dMTUE0R0ExVWREd0VCL3dRRUF3SUZvRENDQVgwR0Npc0dBUVFCMW5rQ0JBSUVnZ0Z0CkJJSUJhUUZuQUhjQTlseVVMOUYzTUNJVVZCZ0lNSlJXanVOTkV4a3p2OThNTHlBTHpFN3haT01BQUFGeE0waG8KYndBQUJBTUFTREJHQWlFQTZ4ZWxpTlI4R2svNjNwWWRuUy92T3gvQ2pwdEVNRXY4OVdXaDEvdXJXSUVDSVFEeQpCcmVIVTI1RHp3dWtRYVJRandXNjU1WkxrcUNueGJ4UVdSaU9lbWo5SkFCMUFKUWd2QjZPMVkxc2lITWZnb3NpCkxBM1IyazFlYkUrVVBXSGJUaTlZVGFMQ0FBQUJjVE5JYU53QUFBUURBRVl3UkFJZ0dSRTR3emFiTlJkRDhrcS8KdkZQM3RRZTJobTB4NW5YdWxvd2g0SWJ3M2xrQ0lGWWIvM2xTRHBsUzdBY1I0citYcFd0RUtTVEZXSm1OQ1JiYwpYSnVyMlJHQkFIVUE3c0NWN28xeVpBK1M0OE81RzhjU28ybHFDWHRMYWhvVU9PWkhzc3Z0eGZrQUFBRnhNMGhvCjh3QUFCQU1BUmpCRUFpQjZJdmJvV3NzM1I0SXRWd2plYmw3RDN5b0ZhWDBORGgyZFdoaGd3Q3hySHdJZ0NmcTcKb2NNQzV0KzFqaTVNNXhhTG1QQzRJK1dYM0kvQVJrV1N5aU83SVFjd0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dJQgpBQ2V1dXI0UW51anFtZ3VTckhVM21oZitjSm9kelRRTnFvNHRkZStQRDEvZUZkWUFFTHU4eEYrMEF0N3hKaVBZCmk1Ukt3aWx5UDU2diszaVkyVDlsdzdTOFRKMDQxVkxoYUlLcDE0TXpTVXpSeWVvT0FzSjdRQURNQ2xIS1VEbEgKVVUycE51bzg4WTZpZ292VDNic253Sk5pRVFOcXltU1NZaGt0dzB0YWR1b3FqcVhuMDZnc1Zpb1dUVkRYeXNkNQpxRXg0dDZzSWdJY01tMjZZSDF2SnBDUUVoS3BjMnkwN2dSa2tsQlpSdE1qVGh2NGNYeXlNWDd1VGNkVDdBSkJQCnVlaWZDb1YyNUp4WHVvOGQ1MTM5Z3dQMUJBZTdJQlZQeDJ1N0tOL1V5T1hkWm13TWYvVG1GR3dEZENmc3lIZi8KWnNCMndMSG96VFlvQVZtUTlGb1UxSkxnY1ZpdnFKK3ZObEJoSFhobHhNZE4wajgwUjlOejZFSWdsUWplSzNPOApJL2NGR20vQjgrNDJoT2xDSWQ5WmR0bmRKY1JKVmppMHdEMHF3ZXZDYWZBOWpKbEh2L2pzRStJOVV6NmNwQ3loCnN3K2xyRmR4VWdxVTU4YXhxZUs4OUZSK05vNHEwSUlPK0ppMXJKS3I5bmtTQjBCcVhvelZuRTFZQi9LTHZkSXMKdVlaSnVxYjJwS2t1K3p6VDZnVXdIVVRadkJpTk90WEw0Tnh3Yy9LVDdXek9TZDJ3UDEwUUk4REtnNHZmaU5EcwpIV21CMWM0S2ppNmdPZ0E1dVNVemFHbXEvdjRWbmNLNVVyK245TGJmbmZMYzI4SjVmdC9Hb3Rpbk15RGszaWFyCkYxMFlscWNPbWVYMXVGbUtiZGkvWG9yR2xrQ29NRjNURHg4cm1wOURCaUIvCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0="
