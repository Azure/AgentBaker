package agent

import (
	"strings"

	"encoding/json"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Assert generated customData and cseCmd", func() {
	DescribeTable("Generated customData and CSE for Linux + Ubuntu", CustomDataCSECommandTestTemplate,
		Entry("AKSUbuntu1604 with k8s version less than 1.18", "AKSUbuntu1604+K8S115", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig["--dynamic-config-dir"] = "/var/lib/kubelet/"
		}, func(o *nodeBootstrappingOutput) {

			Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "DynamicKubeletConfig")).To(BeTrue())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "DynamicKubeletConfig")).To(BeTrue())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--dynamic-config-dir")).To(BeFalse())
			Expect(strings.Contains(o.cseCmd, "DynamicKubeletConfig")).To(BeTrue())

			// sanity check that no other files/variables set the flag
			for _, f := range o.files {
				Expect(strings.Contains(f.value, "--dynamic-config-dir")).To(BeFalse())
			}
			for _, v := range o.vars {
				Expect(strings.Contains(v, "--dynamic-config-dir")).To(BeFalse())
			}

			kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
			Expect(err).To(BeNil())

			var kubeletConfigFile datamodel.AKSKubeletConfiguration

			err = json.Unmarshal([]byte(kubeletConfigFileContent), &kubeletConfigFile)
			Expect(err).To(BeNil())

			dynamicConfigFeatureGate, dynamicConfigFeatureGateExists := kubeletConfigFile.FeatureGates["DynamicKubeletConfig"]
			Expect(dynamicConfigFeatureGateExists).To(Equal(true))
			Expect(dynamicConfigFeatureGate).To(Equal(false))
		}),
		Entry("AKSUbuntu1604 with k8s version 1.18", "AKSUbuntu1604+K8S118", "1.18.2", nil, nil),
		Entry("AKSUbuntu1604 with k8s version 1.17", "AKSUbuntu1604+K8S117", "1.17.7", nil, nil),
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
		}, nil),
		Entry("AKSUbuntu11604 with containerd", "AKSUbuntu1604+Containerd", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerdPackageURL = "containerd-package-url"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["CLI_TOOL"]).To(Equal("ctr"))
			Expect(o.vars["CONTAINERD_PACKAGE_URL"]).To(Equal("containerd-package-url"))
		}),

		Entry("AKSUbuntu11604 with docker and containerd package url", "AKSUbuntu1604+Docker", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Docker,
			}
			config.ContainerdPackageURL = "containerd-package-url"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["CLI_TOOL"]).To(Equal("docker"))
			Expect(o.vars["CONTAINERD_PACKAGE_URL"]).To(Equal(""))
		}),
		Entry("AKSUbuntu1604 with temp disk (api field)", "AKSUbuntu1604+TempDiskExplicit", "1.15.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				// also tests prioritization, but now the API property should take precedence
				config.AgentPoolProfile.KubeletDiskType = datamodel.TempDisk
			}, nil),
		Entry("AKSUbuntu1604 with OS disk", "AKSUbuntu1604+OSKubeletDisk", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			// also tests prioritization, but now the API property should take precedence
			config.AgentPoolProfile.KubeletDiskType = datamodel.OSDisk
		}, nil),
		Entry("AKSUbuntu1604 with Temp Disk and containerd", "AKSUbuntu1604+TempDisk+Containerd", "1.15.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntimeConfig: map[string]string{
						datamodel.ContainerDataDirKey: "/mnt/containers",
					},
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}

				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1604 with RawUbuntu", "RawUbuntu", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.Ubuntu
		}, nil),
		Entry("AKSUbuntu1604 EnablePrivateClusterHostsConfigAgent", "AKSUbuntu1604+EnablePrivateClusterHostsConfigAgent", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				cs := config.ContainerService
				if cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster == nil {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &datamodel.PrivateCluster{EnableHostsConfigAgent: to.BoolPtr(true)}
				} else {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent = to.BoolPtr(true)
				}
			}, nil),
		Entry("AKSUbuntu1804 with GPU dedicated VHD", "AKSUbuntu1604+GPUDedicatedVHD", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuGPU1804
			config.AgentPoolProfile.VMSize = "Standard_NC6"
			config.ConfigGPUDriverIfNeeded = false
			config.EnableGPUDevicePluginIfNeeded = true
			config.EnableNvidia = true
		}, nil),
		Entry("AKSUbuntu1604 with KubeletConfigFile", "AKSUbuntu1604+KubeletConfigFile", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableKubeletConfigFile = true
		}, nil),

		Entry("AKSUbuntu1804 with containerd and private ACR", "AKSUbuntu1804+Containerd+PrivateACR", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
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
			}, nil),
		Entry("AKSUbuntu1804 with containerd and GPU SKU", "AKSUbuntu1804+Containerd+NSeriesSku", "1.15.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.EnableNvidia = true
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804 with containerd and kubenet cni", "AKSUbuntu1804+Containerd+Kubenet", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804 with containerd and kubenet cni and calico policy", "AKSUbuntu1804+Containerd+Kubenet+Calico", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyCalico
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804 with containerd and teleport enabled", "AKSUbuntu1804+Containerd+Teleport", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableACRTeleportPlugin = true
				config.TeleportdPluginURL = "some url"
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with containerd and ipmasqagent enabled", "AKSUbuntu1804+Containerd+IPMasqAgent", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableACRTeleportPlugin = true
				config.TeleportdPluginURL = "some url"
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.ContainerService.Properties.HostedMasterProfile.IPMasqAgent = true
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with containerd and version specified", "AKSUbuntu1804+Containerd+ContainerdVersion", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerdVersion = "1.4.4"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1604 with custom kubeletConfig and osConfig", "AKSUbuntu1604+CustomKubeletConfig+CustomLinuxOSConfig", "1.16.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
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
						NetIpv4IpLocalPortRange:      "32768 65400",
						NetIpv4TcpMaxSynBacklog:      to.Int32Ptr(1638498),
						NetIpv4NeighDefaultGcThresh1: to.Int32Ptr(10001),
					},
					TransparentHugePageEnabled: "never",
					TransparentHugePageDefrag:  "defer+madvise",
					SwapFileSizeMB:             &swapFileSizeMB,
				}
			}, func(o *nodeBootstrappingOutput) {
				sysctlContent, err := getBase64DecodedValue([]byte(o.vars["SYSCTL_CONTENT"]))
				Expect(err).To(BeNil())
				// assert defaults for gc_thresh2 and gc_thresh3
				// assert custom values for all others.
				Expect(sysctlContent).To(ContainSubstring("net.core.somaxconn=1638499"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.tcp_max_syn_backlog=1638498"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh1=10001"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh2=8192"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh3=16384"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.ip_local_reserved_ports=65330"))
			}),

		Entry("AKSUbuntu1604 - dynamic-config-dir should always be removed with custom kubelet config",
			"AKSUbuntu1604+CustomKubeletConfig+DynamicKubeletConfig", "1.16.13", func(config *datamodel.NodeBootstrappingConfiguration) {
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
					"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint: lll
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
			}, func(o *nodeBootstrappingOutput) {
				sysctlContent, err := getBase64DecodedValue([]byte(o.vars["SYSCTL_CONTENT"]))
				Expect(err).To(BeNil())
				// assert defaults for all.
				Expect(sysctlContent).To(ContainSubstring("net.core.somaxconn=16384"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.tcp_max_syn_backlog=16384"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh1=4096"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh2=8192"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh3=16384"))
			}),

		Entry("AKSUbuntu1604 - dynamic-config-dir should always be removed", "AKSUbuntu1604+DynamicKubeletConfig", "1.16.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
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
					"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint: lll
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
			}, nil),

		Entry("RawUbuntu with Containerd", "RawUbuntuContainerd", "1.19.1", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.Ubuntu
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.KubeletConfig = map[string]string{}
		}, nil),

		Entry("AKSUbuntu1604 with Disable1804SystemdResolved=true", "AKSUbuntu1604+Disable1804SystemdResolved=true", "1.16.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.Disable1804SystemdResolved = true
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Docker,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1604 with Disable1804SystemdResolved=false", "AKSUbuntu1604+Disable1804SystemdResolved=false", "1.16.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.Disable1804SystemdResolved = false
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Docker,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with Disable1804SystemdResolved=true", "AKSUbuntu1804+Disable1804SystemdResolved=true", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.Disable1804SystemdResolved = true
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with Disable1804SystemdResolved=false", "AKSUbuntu1804+Disable1804SystemdResolved=false", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.Disable1804SystemdResolved = false
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation implicitly disabled", "AKSUbuntu2204+ImplicitlyDisableKubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("false"))
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation explicitly disabled", "AKSUbuntu2204+DisableKubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig["--rotate-server-certificates"] = "false"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("false"))
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--rotate-server-certificates=false")).To(BeTrue())
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation enabled", "AKSUbuntu2204+KubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig["--rotate-server-certificates"] = "true"
				config.KubeletConfig["--tls-cert-file"] = "cert.crt"
				config.KubeletConfig["--tls-private-key-file"] = "cert.key"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("true"))
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--rotate-server-certificates=true")).To(BeTrue())
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation disabled and custom kubelet config",
			"AKSUbuntu2204+DisableKubeletServingCertificateRotation+CustomKubeletConfig", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				failSwapOn := false
				config.KubeletConfig["--rotate-server-certificates"] = "false"
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
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("false"))
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(kubeletConfigFileContent).ToNot(ContainSubstring("serverTLSBootstrap")) // because of: "bool `json:"serverTLSBootstrap,omitempty"`"
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation enabled and custom kubelet config",
			"AKSUbuntu2204+KubeletServingCertificateRotation+CustomKubeletConfig", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				failSwapOn := false
				config.KubeletConfig["--rotate-server-certificates"] = "true"
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
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("true"))
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(kubeletConfigFileContent).To(ContainSubstring(`"serverTLSBootstrap": true`))
			}),

		Entry("AKSUbuntu1804 with DisableCustomData = true", "AKSUbuntu1804+DisableCustomData", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableCustomData = true
			}, nil),

		Entry("Mariner v2 with kata", "MarinerV2+Kata", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),

		Entry("Mariner v2 with custom cloud", "MarinerV2+CustomCloud", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),

		Entry("AzureLinux v2 with kata", "AzureLinuxV2+Kata", "1.28.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "AzureLinux"
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2Kata
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),

		Entry("AzureLinux v3 with kata", "AzureLinuxV3+Kata", "1.28.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "AzureLinux"
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV3Gen2Kata
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),

		Entry("Mariner v2 with DisableUnattendedUpgrades=true", "Marinerv2+DisableUnattendedUpgrades=true", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("Mariner v2 with DisableUnattendedUpgrades=false", "Marinerv2+DisableUnattendedUpgrades=false", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("Mariner v2 with kata and DisableUnattendedUpgrades=true", "Marinerv2+Kata+DisableUnattendedUpgrades=true", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("Mariner v2 with kata and DisableUnattendedUpgrades=false", "Marinerv2+Kata+DisableUnattendedUpgrades=false", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AzureLinux v2 with DisableUnattendedUpgrades=true", "AzureLinuxv2+DisableUnattendedUpgrades=true", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("AzureLinux v2 with DisableUnattendedUpgrades=false", "AzureLinuxv2+DisableUnattendedUpgrades=false", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AzureLinux v2 with kata and DisableUnattendedUpgrades=true", "AzureLinuxv2+Kata+DisableUnattendedUpgrades=true", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("AzureLinux v2 with kata and DisableUnattendedUpgrades=false", "AzureLinuxv2+Kata+DisableUnattendedUpgrades=false", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AzureLinux v3 with kata and DisableUnattendedUpgrades=true", "AzureLinuxV3+Kata+DisableUnattendedUpgrades=true", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV3Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("AzureLinux v3 with kata and DisableUnattendedUpgrades=false", "AzureLinuxV3+Kata+DisableUnattendedUpgrades=false", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV3Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AKSUbuntu1804 with containerd and kubenet cni", "AKSUbuntu1804+Containerd+Kubenet+FIPSEnabled", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.FIPSEnabled = true
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with http proxy config", "AKSUbuntu1804+HTTPProxy", "1.18.14", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.HTTPProxyConfig = &datamodel.HTTPProxyConfig{
				HTTPProxy:  to.StringPtr("http://myproxy.server.com:8080/"),
				HTTPSProxy: to.StringPtr("https://myproxy.server.com:8080/"),
				NoProxy: to.StringSlicePtr([]string{
					"localhost",
					"127.0.0.1",
				}),
				TrustedCA: to.StringPtr(encodedTestCert),
			}
		},
			func(o *nodeBootstrappingOutput) {
				Expect(o).ShouldNot(BeNil())
				Expect(o.files["/opt/azure/containers/provision.sh"]).ShouldNot(BeNil())
				Expect(o.files["/opt/azure/containers/provision.sh"].encoding).To(Equal(cseVariableEncodingGzip))
				cseMain := o.files["/opt/azure/containers/provision.sh"].value
				httpProxyStr := "export http_proxy=\"http://myproxy.server.com:8080/\""
				Expect(strings.Contains(cseMain, "eval $PROXY_VARS")).To(BeTrue())
				Expect(strings.Contains(cseMain, "$OUTBOUND_COMMAND")).To(BeTrue())
				// assert we eval exporting the proxy vars before checking outbound connectivity
				Expect(strings.Index(cseMain, "eval $PROXY_VARS") < strings.Index(cseMain, "$OUTBOUND_COMMAND")).To(BeTrue())
				Expect(strings.Contains(o.cseCmd, httpProxyStr)).To(BeTrue())
			},
		),

		Entry("AKSUbuntu1804 with http proxy config cert newlines", "AKSUbuntu1804+HTTPProxy", "1.18.14", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.HTTPProxyConfig = &datamodel.HTTPProxyConfig{
				HTTPProxy:  to.StringPtr("http://myproxy.server.com:8080/"),
				HTTPSProxy: to.StringPtr("https://myproxy.server.com:8080/"),
				NoProxy: to.StringSlicePtr([]string{
					"localhost",
					"127.0.0.1",
				}),
				TrustedCA: to.StringPtr(testCertWithNewline),
			}
		},
			func(o *nodeBootstrappingOutput) {
				Expect(o).ShouldNot(BeNil())
				Expect(o.files["/opt/azure/containers/provision.sh"]).ShouldNot(BeNil())
				Expect(o.files["/opt/azure/containers/provision.sh"].encoding).To(Equal(cseVariableEncodingGzip))
				cseMain := o.files["/opt/azure/containers/provision.sh"].value
				httpProxyStr := "export http_proxy=\"http://myproxy.server.com:8080/\""
				Expect(strings.Contains(cseMain, "eval $PROXY_VARS")).To(BeTrue())
				Expect(strings.Contains(cseMain, "$OUTBOUND_COMMAND")).To(BeTrue())
				Expect(o.vars["HTTP_PROXY_TRUSTED_CA"]).To(Equal(encodedTestCert))
				err := verifyCertsEncoding(o.vars["HTTP_PROXY_TRUSTED_CA"])
				Expect(err).ShouldNot(HaveOccurred())
				// assert we eval exporting the proxy vars before checking outbound connectivity
				Expect(strings.Index(cseMain, "eval $PROXY_VARS") < strings.Index(cseMain, "$OUTBOUND_COMMAND")).To(BeTrue())
				Expect(strings.Contains(o.cseCmd, httpProxyStr)).To(BeTrue())
			},
		),

		Entry("AKSUbuntu2204 with outbound type blocked", "AKSUbuntu2204+OutboundTypeBlocked", "1.25.6", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OutboundType = datamodel.OutboundTypeBlock
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["BLOCK_OUTBOUND_NETWORK"]).To(Equal("true"))
		}),

		Entry("AKSUbuntu2204 with outbound type none", "AKSUbuntu2204+OutboundTypeNone", "1.25.6", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OutboundType = datamodel.OutboundTypeNone
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["BLOCK_OUTBOUND_NETWORK"]).To(Equal("true"))
		}),

		Entry("AKSUbuntu2204 with no outbound type", "AKSUbuntu2204+OutboundTypeNil", "1.25.6", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OutboundType = ""
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["BLOCK_OUTBOUND_NETWORK"]).To(Equal("false"))
		}),

		Entry("AKSUbuntu2204 with SerializeImagePulls=false and k8s 1.31", "AKSUbuntu2204+SerializeImagePulls", "1.31.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig["--serialize-image-pulls"] = "false"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--serialize-image-pulls=false")).To(BeTrue())
		}),

		Entry("AKSUbuntu1804 with custom ca trust", "AKSUbuntu1804+CustomCATrust", "1.18.14", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
				CustomCATrustCerts: []string{encodedTestCert, encodedTestCert, testCertWithNewline},
			}
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["CUSTOM_CA_TRUST_COUNT"]).To(Equal("3"))
			Expect(o.vars["SHOULD_CONFIGURE_CUSTOM_CA_TRUST"]).To(Equal("true"))
			Expect(o.vars["CUSTOM_CA_CERT_0"]).To(Equal(encodedTestCert))
			err := verifyCertsEncoding(o.vars["CUSTOM_CA_CERT_0"])
			Expect(err).To(BeNil())
			Expect(o.vars["CUSTOM_CA_CERT_2"]).To(Equal(encodedTestCert))
			err = verifyCertsEncoding(o.vars["CUSTOM_CA_CERT_2"])
			Expect(err).To(BeNil())
		}),

		Entry("AKSUbuntu1804 with containerd and runcshimv2", "AKSUbuntu1804+Containerd+runcshimv2", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableRuncShimV2 = true
			}, nil),

		Entry("AKSUbuntu1804 with containerd and motd", "AKSUbuntu1804+Containerd+MotD", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {

			config.ContainerService.Properties.AgentPoolProfiles[0].MessageOfTheDay = "Zm9vYmFyDQo=" // foobar in b64
		}, nil),

		Entry("AKSUbuntu1804containerd with custom runc verison", "AKSUbuntu1804Containerd+RuncVersion", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.RuncVersion = "1.0.0-rc96"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with containerd+gpu and runcshimv2", "AKSUbuntu1804+Containerd++GPU+runcshimv2", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.AgentPoolProfile.VMSize = "Standard_NC6"
				config.EnableNvidia = true
				config.EnableRuncShimV2 = true
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 containerd with multi-instance GPU", "AKSUbuntu1804+Containerd+MIG", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.KubeletConfig = map[string]string{}
				config.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
				config.EnableNvidia = true
				config.GPUInstanceProfile = "MIG7g"
			}, nil),

		Entry("AKSUbuntu1804 containerd with multi-instance non-fabricmanager GPU", "AKSUbuntu1804+Containerd+MIG+NoFabricManager", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.KubeletConfig = map[string]string{}
				config.AgentPoolProfile.VMSize = "Standard_NC24ads_A100_v4"
				config.EnableNvidia = true
				config.GPUInstanceProfile = "MIG7g"
			}, nil),

		Entry("AKSUbuntu2204 with artifact streaming", "AKSUbuntu1804+ArtifactStreaming", "1.25.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableArtifactStreaming = true
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
			config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.25.7"
		},
			func(o *nodeBootstrappingOutput) {

				Expect(o.vars["CONTAINERD_CONFIG_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				Expect(err).To(BeNil())
				expectedOverlaybdConfig := `version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  enable_cdi = true
  [plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"
`
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedOverlaybdConfig))
				expectedOverlaybdPlugin := `[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"`
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedOverlaybdPlugin))
			},
		),
		Entry("AKSUbuntu2204 w/o artifact streaming", "AKSUbuntu1804+NoArtifactStreaming", "1.25.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableArtifactStreaming = false
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
		},
			func(o *nodeBootstrappingOutput) {

				Expect(o.vars["CONTAINERD_CONFIG_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				Expect(err).To(BeNil())
				expectedOverlaybdConfig := `[plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    default_runtime_name = "runc"`
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(expectedOverlaybdConfig))
				expectedOverlaybdPlugin := `[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"`
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(expectedOverlaybdPlugin))
			},
		),
		Entry("AKSUbuntu1804 with NoneCNI", "AKSUbuntu1804+NoneCNI", "1.20.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = datamodel.NetworkPluginNone
		}, nil),
		Entry("AKSUbuntu1804 with Containerd and certs.d", "AKSUbuntu1804+Containerd+Certsd", "1.22.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
			}, nil),
		Entry("AKSUbuntu1804ARM64containerd with kubenet", "AKSUbuntu1804ARM64Containerd+NoCustomKubeImageandBinaries", "1.22.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorType = "azure"
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/1.22.2/binaries/kubernetes-node-linux-arm64.tar.gz" //nolint:lll
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeProxyImage = "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.22.2"                                           //nolint:lll
				config.IsARM64 = true
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804ARM64containerd with kubenet", "AKSUbuntu1804ARM64Containerd+CustomKubeImageandBinaries", "1.22.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorType = datamodel.Kubernetes
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/1.22.2/binaries/kubernetes-node-linux-arm64.tar.gz" //nolint:lll
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeProxyImage = "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.22.2"                                           //nolint:lll
				config.IsARM64 = true
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804 with IPAddress and FQDN", "AKSUbuntu1804+Containerd+IPAddress+FQDN", "1.22.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.HostedMasterProfile.FQDN = "a.hcp.eastus.azmk8s.io"
				config.ContainerService.Properties.HostedMasterProfile.IPAddress = "1.2.3.4"
			}, nil),
		Entry("AKSUbuntu2204 VHD, cgroupv2", "AKSUbuntu2204+cgroupv2", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
		}, nil),
		Entry("AKSUbuntu2204 with containerd and CDI enabled", "AKSUbuntu2204+Containerd+CDI", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
			config.KubeletConfig = map[string]string{}
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["CONTAINERD_CONFIG_CONTENT"]).NotTo(BeEmpty())
			containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
			Expect(err).To(BeNil())
			Expect(containerdConfigFileContent).To(ContainSubstring("enable_cdi = true"))
		}),
		Entry("AKSUbuntu2204 containerd with multi-instance GPU", "AKSUbuntu2204+Containerd+MIG", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
				config.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
				// the purpose of this unit test is to ensure the containerd config
				// does not use the nvidia container runtime when skipping the
				// GPU driver install, since it will fail to run even non-GPU
				// pods, as it will not be installed.
				config.EnableNvidia = true
				config.ConfigGPUDriverIfNeeded = true
				config.GPUInstanceProfile = "MIG7g"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]))
				Expect(err).To(BeNil())
				expectedShimConfig := `version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  [plugins."io.containerd.grpc.v1.cri".containerd]
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
`

				Expect(containerdConfigFileContent).To(Equal(expectedShimConfig))
			}),
		Entry("AKSUbuntu2204 containerd with multi-instance GPU and artifact streaming", "AKSUbuntu2204+Containerd+MIG+ArtifactStreaming", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.25.7"
				config.EnableArtifactStreaming = true
				config.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
				// the purpose of this unit test is to ensure the containerd config
				// does not use the nvidia container runtime when skipping the
				// GPU driver install, since it will fail to run even non-GPU
				// pods, as it will not be installed.
				config.EnableNvidia = true
				config.ConfigGPUDriverIfNeeded = true
				config.GPUInstanceProfile = "MIG7g"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]))
				Expect(err).To(BeNil())
				expectedShimConfig := `version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  [plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"
`

				Expect(containerdConfigFileContent).To(Equal(expectedShimConfig))
			}),
		Entry("CustomizedImage VHD should not have provision_start.sh", "CustomizedImage", "1.24.2",
			func(c *datamodel.NodeBootstrappingConfiguration) {
				c.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				c.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.CustomizedImage
			}, func(o *nodeBootstrappingOutput) {
				_, exist := o.files["/opt/azure/containers/provision_start.sh"]

				Expect(exist).To(BeFalse())
			},
		),
		Entry("CustomizedImageKata VHD should not have provision_start.sh", "CustomizedImageKata", "1.24.2",
			func(c *datamodel.NodeBootstrappingConfiguration) {
				c.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				c.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.CustomizedImageKata
			}, func(o *nodeBootstrappingOutput) {
				_, exist := o.files["/opt/azure/containers/provision_start.sh"]

				Expect(exist).To(BeFalse())
			},
		),
		Entry("CustomizedImageLinuxGuard VHD should not have provision_start.sh", "CustomizedImageLinuxGuard", "1.24.2",
			func(c *datamodel.NodeBootstrappingConfiguration) {
				c.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				c.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.CustomizedImageLinuxGuard
			}, func(o *nodeBootstrappingOutput) {
				_, exist := o.files["/opt/azure/containers/provision_start.sh"]

				Expect(exist).To(BeFalse())
			},
		),
		Entry("Flatcar", "Flatcar", "1.31.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = datamodel.OSSKUFlatcar
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSFlatcarGen2
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),
		Entry("AKSUbuntu2204 DisableSSH with enabled ssh", "AKSUbuntu2204+SSHStatusOn", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.SSHStatus = datamodel.SSHOn
		}, nil),
		Entry("AKSUbuntu2204 DisableSSH with disabled ssh", "AKSUbuntu2204+SSHStatusOff", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.SSHStatus = datamodel.SSHOff
		}, nil),
		Entry("AKSUbuntu2204 in China", "AKSUbuntu2204+China", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "AzureChinaCloud",
			}
			config.ContainerService.Location = "chinaeast2"
		}, nil),
		Entry("AKSUbuntu2204 custom cloud", "AKSUbuntu2204+CustomCloud", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),
		Entry("AKSUbuntu2204 OOT credentialprovider", "AKSUbuntu2204+ootcredentialprovider", "1.29.10", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
			config.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml")).To(BeTrue())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider")).To(BeTrue())
		}),
		Entry("AKSUbuntu2204 custom cloud and OOT credentialprovider", "AKSUbuntu2204+CustomCloud+ootcredentialprovider", "1.29.10",
			func(config *datamodel.NodeBootstrappingConfiguration) {
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
				config.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				config.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			}, func(o *nodeBootstrappingOutput) {

				Expect(o.vars["AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX"]).NotTo(BeEmpty())
				Expect(o.vars["AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX"]).To(Equal(".azurecr.microsoft.fakecustomcloud"))

				Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml")).To(BeTrue())
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider")).To(BeTrue())
			}),
		Entry("AKSUbuntu2204 with custom kubeletConfig and osConfig", "AKSUbuntu2204+CustomKubeletConfig+CustomLinuxOSConfig", "1.24.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
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
					SeccompDefault:        to.BoolPtr(true),
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetCoreSomaxconn:             &netCoreSomaxconn,
						NetCoreRmemDefault:           to.Int32Ptr(456000),
						NetCoreWmemDefault:           to.Int32Ptr(89000),
						NetIpv4TcpTwReuse:            &netIpv4TcpTwReuse,
						NetIpv4IpLocalPortRange:      "32768 65400",
						NetIpv4TcpMaxSynBacklog:      to.Int32Ptr(1638498),
						NetIpv4NeighDefaultGcThresh1: to.Int32Ptr(10001),
					},
					TransparentHugePageEnabled: "never",
					TransparentHugePageDefrag:  "defer+madvise",
					SwapFileSizeMB:             &swapFileSizeMB,
					UlimitConfig: &datamodel.UlimitConfig{
						MaxLockedMemory: "75000",
						NoFile:          "1048",
					},
				}
			}, func(o *nodeBootstrappingOutput) {
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				var kubeletConfigFile datamodel.AKSKubeletConfiguration
				err = json.Unmarshal([]byte(kubeletConfigFileContent), &kubeletConfigFile)
				Expect(err).To(BeNil())
				Expect(kubeletConfigFile.SeccompDefault).To(Equal(to.BoolPtr(true)))

				sysctlContent, err := getBase64DecodedValue([]byte(o.vars["SYSCTL_CONTENT"]))
				Expect(err).To(BeNil())
				// assert defaults for gc_thresh2 and gc_thresh3
				// assert custom values for all others.
				Expect(sysctlContent).To(ContainSubstring("net.core.somaxconn=1638499"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.tcp_max_syn_backlog=1638498"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh1=10001"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh2=8192"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh3=16384"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.ip_local_reserved_ports=65330"))

				Expect(o.vars["SHOULD_CONFIG_CONTAINERD_ULIMITS"]).To(Equal("true"))
				containerdUlimitContent := o.vars["CONTAINERD_ULIMITS"]
				Expect(containerdUlimitContent).To(ContainSubstring("LimitNOFILE=1048"))
				Expect(containerdUlimitContent).To(ContainSubstring("LimitMEMLOCK=75000"))
			}),
		Entry("AKSUbuntu2204 with k8s 1.31 and custom kubeletConfig and serializeImagePull flag", "AKSUbuntu2204+CustomKubeletConfig+SerializeImagePulls", "1.31.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				failSwapOn := false
				config.KubeletConfig["--serialize-image-pulls"] = "false"
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
			}, func(o *nodeBootstrappingOutput) {
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(kubeletConfigFileContent).To(ContainSubstring(`"serializeImagePulls": false`))
			}),
		Entry("AKSUbuntu2204 with SecurityProfile", "AKSUbuntu2204+SecurityProfile", "1.26.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ProxyAddress:            "https://test-pe-proxy",
						ContainerRegistryServer: "testserver.azurecr.io",
					},
				}
			}, nil),
		Entry("AKSUbuntu2204 IMDSRestriction with enable restriction and insert to mangle table", "AKSUbuntu2204+IMDSRestrictionOnWithMangleTable", "1.24.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableIMDSRestriction = true
				config.InsertIMDSRestrictionRuleToMangleTable = true
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_IMDS_RESTRICTION"]).To(Equal("true"))
				Expect(o.vars["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"]).To(Equal("true"))
			}),
		Entry("AKSUbuntu2204 IMDSRestriction with enable restriction and not insert to mangle table", "AKSUbuntu2204+IMDSRestrictionOnWithFilterTable", "1.24.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableIMDSRestriction = true
				config.InsertIMDSRestrictionRuleToMangleTable = false
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_IMDS_RESTRICTION"]).To(Equal("true"))
				Expect(o.vars["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"]).To(Equal("false"))
			}),
		Entry("AKSUbuntu2204 IMDSRestriction with disable restriction", "AKSUbuntu2204+IMDSRestrictionOff", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableIMDSRestriction = false
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["ENABLE_IMDS_RESTRICTION"]).To(Equal("false"))
			Expect(o.vars["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"]).To(Equal("false"))
		}),
		Entry("AKSUbuntu2404 with custom osConfig for Ulimit", "AKSUbuntu2404+CustomLinuxOSConfigUlimit", ">=1.32.x",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
					UlimitConfig: &datamodel.UlimitConfig{
						MaxLockedMemory: "75000",
						NoFile:          "1048",
					},
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2404
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["SHOULD_CONFIG_CONTAINERD_ULIMITS"]).To(Equal("true"))
				containerdUlimitContent := o.vars["CONTAINERD_ULIMITS"]
				Expect(containerdUlimitContent).NotTo(ContainSubstring("LimitNOFILE=1048"))
				Expect(containerdUlimitContent).To(ContainSubstring("LimitMEMLOCK=75000"))
			}),

		Entry("Mariner v2 with custom cloud", "MarinerV2+CustomCloud+USSec", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Location = "ussecwest"
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),

		Entry("Mariner v2 with custom cloud", "MarinerV2+CustomCloud+USNat", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Location = "usnatwest"
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),
	)
})
