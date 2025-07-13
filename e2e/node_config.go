package e2e

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Masterminds/semver"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
)

// this is a base kubelet config for Scriptless e2e test
var baseKubeletConfig = &aksnodeconfigv1.KubeletConfig{
	EnableKubeletConfigFile: true,
	KubeletFlags: map[string]string{
		"--cloud-config":              "",
		"--cloud-provider":            "external",
		"--kubeconfig":                "/var/lib/kubelet/kubeconfig",
		"--pod-infra-container-image": "mcr.microsoft.com/oss/kubernetes/pause:3.6",
	},
	KubeletNodeLabels: map[string]string{
		"agentpool":                               "nodepool2",
		"kubernetes.azure.com/agentpool":          "nodepool2",
		"kubernetes.azure.com/cluster":            "test-cluster",
		"kubernetes.azure.com/mode":               "system",
		"kubernetes.azure.com/node-image-version": "AKSUbuntu-1804gen2containerd-2022.01.19",
	},
	KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{
		Kind:              "KubeletConfiguration",
		ApiVersion:        "kubelet.config.k8s.io/v1beta1",
		StaticPodPath:     "/etc/kubernetes/manifests",
		Address:           "0.0.0.0",
		TlsCertFile:       "/etc/kubernetes/certs/kubeletserver.crt",
		TlsPrivateKeyFile: "/etc/kubernetes/certs/kubeletserver.key",
		TlsCipherSuites: []string{
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_RSA_WITH_AES_128_GCM_SHA256",
		},
		RotateCertificates: true,
		ServerTlsBootstrap: false,
		Authentication: &aksnodeconfigv1.KubeletAuthentication{
			X509: &aksnodeconfigv1.KubeletX509Authentication{
				ClientCaFile: "/etc/kubernetes/certs/ca.crt",
			},
			Webhook: &aksnodeconfigv1.KubeletWebhookAuthentication{
				Enabled: true,
			},
		},
		Authorization: &aksnodeconfigv1.KubeletAuthorization{
			Mode: "Webhook",
		},
		EventRecordQps: to.Ptr(int32(0)),
		ClusterDomain:  "cluster.local",
		ClusterDns: []string{
			"10.0.0.10",
		},
		StreamingConnectionIdleTimeout: "4h",
		NodeStatusUpdateFrequency:      "10s",
		ImageGcHighThresholdPercent:    to.Ptr(int32(85)),
		ImageGcLowThresholdPercent:     to.Ptr(int32(80)),
		CgroupsPerQos:                  to.Ptr(true),
		MaxPods:                        to.Ptr(int32(110)),
		PodPidsLimit:                   to.Ptr(int32(-1)),
		ResolvConf:                     "/run/systemd/resolve/resolv.conf",
		EvictionHard: map[string]string{
			"memory.available":  "750Mi",
			"nodefs.available":  "10%",
			"nodefs.inodesFree": "5%",
		},
		ProtectKernelDefaults: true,
		FeatureGates:          map[string]bool{},
		FailSwapOn:            to.Ptr(false),
		KubeReserved: map[string]string{
			"cpu":    "100m",
			"memory": "1638Mi",
		},
		EnforceNodeAllocatable: []string{
			"pods",
		},
		AllowedUnsafeSysctls: []string{
			"kernel.msg*",
			"net.ipv4.route.min_pmtu",
		},
	},
}

func getBaseNBC(t *testing.T, cluster *Cluster, vhd *config.Image) *datamodel.NodeBootstrappingConfiguration {
	var nbc *datamodel.NodeBootstrappingConfiguration

	if vhd.Distro.IsWindowsDistro() {
		nbc = baseTemplateWindows(t, *cluster.Model.Location)

		// these aren't needed since we use TLS bootstrapping instead, though windows bootstrapping expects non-empty values
		nbc.ContainerService.Properties.CertificateProfile.ClientCertificate = "none"
		nbc.ContainerService.Properties.CertificateProfile.ClientPrivateKey = "none"

		nbc.ContainerService.Properties.ClusterID = *cluster.Model.ID
		nbc.SubscriptionID = config.Config.SubscriptionID
		nbc.ResourceGroupName = *cluster.Model.Properties.NodeResourceGroup
		nbc.TenantID = *cluster.Model.Identity.TenantID
	} else {
		nbc = baseTemplateLinux(t, *cluster.Model.Location, *cluster.Model.Properties.CurrentKubernetesVersion, vhd.Arch)
	}

	nbc.ContainerService.Properties.CertificateProfile.CaCertificate = string(cluster.ClusterParams.CACert)
	nbc.KubeletClientTLSBootstrapToken = &cluster.ClusterParams.BootstrapToken
	nbc.ContainerService.Properties.HostedMasterProfile.FQDN = cluster.ClusterParams.FQDN
	nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = vhd.Distro
	nbc.AgentPoolProfile.Distro = vhd.Distro
	return nbc
}

// is a temporary workaround
// eventually we want to phase out usage of nbc
func nbcToAKSNodeConfigV1(nbc *datamodel.NodeBootstrappingConfiguration) *aksnodeconfigv1.Configuration {
	cs := nbc.ContainerService
	agent.ValidateAndSetLinuxNodeBootstrappingConfiguration(nbc)

	config := &aksnodeconfigv1.Configuration{
		Version:            "v0",
		DisableCustomData:  false,
		LinuxAdminUsername: "azureuser",
		VmSize:             config.Config.DefaultVMSKU,
		ClusterConfig: &aksnodeconfigv1.ClusterConfig{
			Location:      nbc.ContainerService.Location,
			ResourceGroup: nbc.ResourceGroupName,
			VmType:        aksnodeconfigv1.VmType_VM_TYPE_VMSS,
			ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{
				SecurityGroupName: cs.Properties.GetNSGName(),
				VnetName:          cs.Properties.GetVirtualNetworkName(),
				VnetResourceGroup: cs.Properties.GetVNetResourceGroupName(),
				Subnet:            cs.Properties.GetSubnetName(),
				RouteTable:        cs.Properties.GetRouteTableName(),
			},
			PrimaryScaleSet: nbc.PrimaryScaleSetName,
		},
		ApiServerConfig: &aksnodeconfigv1.ApiServerConfig{
			ApiServerName: cs.Properties.HostedMasterProfile.FQDN,
		},
		AuthConfig: &aksnodeconfigv1.AuthConfig{
			ServicePrincipalId:     cs.Properties.ServicePrincipalProfile.ClientID,
			ServicePrincipalSecret: cs.Properties.ServicePrincipalProfile.Secret,
			TenantId:               nbc.TenantID,
			SubscriptionId:         nbc.SubscriptionID,
			AssignedIdentityId:     nbc.UserAssignedIdentityClientID,
		},
		NetworkConfig: &aksnodeconfigv1.NetworkConfig{
			NetworkPlugin:     aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_KUBENET,
			CniPluginsUrl:     nbc.CloudSpecConfig.KubernetesSpecConfig.CNIPluginsDownloadURL,
			VnetCniPluginsUrl: cs.Properties.OrchestratorProfile.KubernetesConfig.AzureCNIURLLinux,
		},
		GpuConfig: &aksnodeconfigv1.GpuConfig{
			ConfigGpuDriver: true,
			GpuDevicePlugin: false,
		},
		EnableUnattendedUpgrade: true,
		KubernetesVersion:       cs.Properties.OrchestratorProfile.OrchestratorVersion,
		ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{
			ContainerdDownloadUrlBase: nbc.CloudSpecConfig.KubernetesSpecConfig.ContainerdDownloadURLBase,
		},
		OutboundCommand: helpers.GetDefaultOutboundCommand(),
		BootstrappingConfig: &aksnodeconfigv1.BootstrappingConfig{
			TlsBootstrappingToken: nbc.KubeletClientTLSBootstrapToken,
		},
		KubernetesCaCert: base64.StdEncoding.EncodeToString([]byte(cs.Properties.CertificateProfile.CaCertificate)),
		KubeBinaryConfig: &aksnodeconfigv1.KubeBinaryConfig{
			KubeBinaryUrl:             cs.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL,
			PodInfraContainerImageUrl: nbc.K8sComponents.PodInfraContainerImageURL,
		},
		KubeProxyUrl: cs.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeProxyImage,
		HttpProxyConfig: &aksnodeconfigv1.HttpProxyConfig{
			NoProxyEntries: *nbc.HTTPProxyConfig.NoProxy,
		},
		NeedsCgroupv2: to.Ptr(true),
		// Before scriptless, absvc combined kubelet configs from multiple sources such as nbc.AgentPoolProfile.CustomKubeletConfig, nbc.KubeletConfig and more.
		// Now in scriptless, we don't have absvc to process nbc and nbc is no longer a dependency.
		// Therefore, we require client (e.g. AKS-RP) to provide the final kubelet config that is ready to be written to the final kubelet config file on a node.
		KubeletConfig: baseKubeletConfig,
	}
	return config
}

// this is huge, but accurate, so leave it here.
// TODO(ace): minimize the actual required defaults.
// this is what we previously used for bash e2e from e2e/nodebootstrapping_template.json.
// which itself was extracted from baker_test.go logic, which was inherited from aks-engine.
func baseTemplateLinux(t *testing.T, location string, k8sVersion string, arch string) *datamodel.NodeBootstrappingConfiguration {
	config := &datamodel.NodeBootstrappingConfiguration{
		Version: "v0",
		ContainerService: &datamodel.ContainerService{
			ID:       "",
			Location: location,
			Name:     "",
			Plan:     nil,
			Tags:     map[string]string(nil),
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				ClusterID:         "",
				ProvisioningState: "",
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    "Kubernetes",
					OrchestratorVersion: k8sVersion,
					KubernetesConfig: &datamodel.KubernetesConfig{
						KubernetesImageBase:               "",
						MCRKubernetesImageBase:            "",
						ClusterSubnet:                     "",
						NetworkPolicy:                     "",
						NetworkPlugin:                     "kubenet",
						NetworkMode:                       "",
						ContainerRuntime:                  "",
						MaxPods:                           0,
						DockerBridgeSubnet:                "",
						DNSServiceIP:                      "",
						ServiceCIDR:                       "",
						UseManagedIdentity:                false,
						UserAssignedID:                    "",
						UserAssignedClientID:              "",
						CustomHyperkubeImage:              "",
						CustomKubeProxyImage:              fmt.Sprintf("mcr.microsoft.com/oss/kubernetes/kube-proxy:v%s", k8sVersion),
						CustomKubeBinaryURL:               fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/binaries/kubernetes-node-linux-%s.tar.gz", k8sVersion, arch),
						MobyVersion:                       "",
						ContainerdVersion:                 "",
						WindowsNodeBinariesURL:            "",
						WindowsContainerdURL:              "",
						WindowsSdnPluginURL:               "",
						UseInstanceMetadata:               to.Ptr(true),
						EnableRbac:                        nil,
						EnableSecureKubelet:               nil,
						PrivateCluster:                    nil,
						GCHighThreshold:                   0,
						GCLowThreshold:                    0,
						EnableEncryptionWithExternalKms:   nil,
						Addons:                            nil,
						ContainerRuntimeConfig:            map[string]string(nil),
						ControllerManagerConfig:           map[string]string(nil),
						SchedulerConfig:                   map[string]string(nil),
						CloudProviderBackoffMode:          "v2",
						CloudProviderBackoff:              to.Ptr(true),
						CloudProviderBackoffRetries:       6,
						CloudProviderBackoffJitter:        0.0,
						CloudProviderBackoffDuration:      5,
						CloudProviderBackoffExponent:      0.0,
						CloudProviderRateLimit:            to.Ptr(true),
						CloudProviderRateLimitQPS:         10.0,
						CloudProviderRateLimitQPSWrite:    10.0,
						CloudProviderRateLimitBucket:      100,
						CloudProviderRateLimitBucketWrite: 100,
						CloudProviderDisableOutboundSNAT:  to.Ptr(false),
						NodeStatusUpdateFrequency:         "",
						LoadBalancerSku:                   "Standard",
						ExcludeMasterFromStandardLB:       nil,
						AzureCNIURLLinux:                  "https://packages.aks.azure.com/azure-cni/v1.1.8/binaries/azure-vnet-cni-linux-amd64-v1.1.8.tgz",
						AzureCNIURLARM64Linux:             "",
						AzureCNIURLWindows:                "",
						MaximumLoadBalancerRuleCount:      250,
						PrivateAzureRegistryServer:        "",
						NetworkPluginMode:                 "",
					},
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "nodepool2",
						VMSize:              config.Config.DefaultVMSKU,
						KubeletDiskType:     "",
						WorkloadRuntime:     "",
						DNSPrefix:           "",
						OSType:              "Linux",
						Ports:               nil,
						AvailabilityProfile: "VirtualMachineScaleSets",
						StorageProfile:      "ManagedDisks",
						VnetSubnetID:        "",
						Distro:              "aks-ubuntu-containerd-18.04-gen2",
						CustomNodeLabels: map[string]string{
							"kubernetes.azure.com/cluster":            "test-cluster", // Some AKS daemonsets require that this exists, but the value doesn't matter.
							"kubernetes.azure.com/mode":               "system",
							"kubernetes.azure.com/node-image-version": "AKSUbuntu-1804gen2containerd-2022.01.19",
						},
						PreprovisionExtension: nil,
						KubernetesConfig: &datamodel.KubernetesConfig{
							KubernetesImageBase:               "",
							MCRKubernetesImageBase:            "",
							ClusterSubnet:                     "",
							NetworkPolicy:                     "",
							NetworkPlugin:                     "",
							NetworkMode:                       "",
							ContainerRuntime:                  "containerd",
							MaxPods:                           0,
							DockerBridgeSubnet:                "",
							DNSServiceIP:                      "",
							ServiceCIDR:                       "",
							UseManagedIdentity:                false,
							UserAssignedID:                    "",
							UserAssignedClientID:              "",
							CustomHyperkubeImage:              "",
							CustomKubeProxyImage:              "",
							CustomKubeBinaryURL:               "",
							MobyVersion:                       "",
							ContainerdVersion:                 "",
							WindowsNodeBinariesURL:            "",
							WindowsContainerdURL:              "",
							WindowsSdnPluginURL:               "",
							UseInstanceMetadata:               nil,
							EnableRbac:                        nil,
							EnableSecureKubelet:               nil,
							PrivateCluster:                    nil,
							GCHighThreshold:                   0,
							GCLowThreshold:                    0,
							EnableEncryptionWithExternalKms:   nil,
							Addons:                            nil,
							ContainerRuntimeConfig:            map[string]string(nil),
							ControllerManagerConfig:           map[string]string(nil),
							SchedulerConfig:                   map[string]string(nil),
							CloudProviderBackoffMode:          "",
							CloudProviderBackoff:              nil,
							CloudProviderBackoffRetries:       0,
							CloudProviderBackoffJitter:        0.0,
							CloudProviderBackoffDuration:      0,
							CloudProviderBackoffExponent:      0.0,
							CloudProviderRateLimit:            nil,
							CloudProviderRateLimitQPS:         0.0,
							CloudProviderRateLimitQPSWrite:    0.0,
							CloudProviderRateLimitBucket:      0,
							CloudProviderRateLimitBucketWrite: 0,
							CloudProviderDisableOutboundSNAT:  nil,
							NodeStatusUpdateFrequency:         "",
							LoadBalancerSku:                   "",
							ExcludeMasterFromStandardLB:       nil,
							AzureCNIURLLinux:                  "",
							AzureCNIURLARM64Linux:             "",
							AzureCNIURLWindows:                "",
							MaximumLoadBalancerRuleCount:      0,
							PrivateAzureRegistryServer:        "",
							NetworkPluginMode:                 "",
						},
						VnetCidrs:               nil,
						WindowsNameVersion:      "",
						CustomKubeletConfig:     nil,
						CustomLinuxOSConfig:     nil,
						MessageOfTheDay:         "",
						NotRebootWindowsNode:    nil,
						AgentPoolWindowsProfile: nil,
						LocalDNSProfile: &datamodel.LocalDNSProfile{
							EnableLocalDNS:       true,
							CPULimitInMilliCores: to.Ptr(int32(2008)),
							MemoryLimitInMB:      to.Ptr(int32(128)),
							VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
								".": {
									QueryLogging:                "Log",
									Protocol:                    "PreferUDP",
									ForwardDestination:          "VnetDNS",
									ForwardPolicy:               "Sequential",
									MaxConcurrent:               to.Ptr(int32(1000)),
									CacheDurationInSeconds:      to.Ptr(int32(3600)),
									ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
									ServeStale:                  "Verify",
								},
								"cluster.local": {
									QueryLogging:                "Error",
									Protocol:                    "ForceTCP",
									ForwardDestination:          "ClusterCoreDNS",
									ForwardPolicy:               "Sequential",
									MaxConcurrent:               to.Ptr(int32(1000)),
									CacheDurationInSeconds:      to.Ptr(int32(3600)),
									ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
									ServeStale:                  "Disable",
								},
								"testdomain456.com": {
									QueryLogging:                "Log",
									Protocol:                    "PreferUDP",
									ForwardDestination:          "ClusterCoreDNS",
									ForwardPolicy:               "Sequential",
									MaxConcurrent:               to.Ptr(int32(1000)),
									CacheDurationInSeconds:      to.Ptr(int32(3600)),
									ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
									ServeStale:                  "Verify",
								},
							},
							KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
								".": {
									QueryLogging:                "Error",
									Protocol:                    "PreferUDP",
									ForwardDestination:          "ClusterCoreDNS",
									ForwardPolicy:               "Sequential",
									MaxConcurrent:               to.Ptr(int32(1000)),
									CacheDurationInSeconds:      to.Ptr(int32(3600)),
									ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
									ServeStale:                  "Verify",
								},
								"cluster.local": {
									QueryLogging:                "Log",
									Protocol:                    "ForceTCP",
									ForwardDestination:          "ClusterCoreDNS",
									ForwardPolicy:               "RoundRobin",
									MaxConcurrent:               to.Ptr(int32(1000)),
									CacheDurationInSeconds:      to.Ptr(int32(3600)),
									ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
									ServeStale:                  "Disable",
								},
								"testdomain567.com": {
									QueryLogging:                "Error",
									Protocol:                    "PreferUDP",
									ForwardDestination:          "VnetDNS",
									ForwardPolicy:               "Random",
									MaxConcurrent:               to.Ptr(int32(1000)),
									CacheDurationInSeconds:      to.Ptr(int32(3600)),
									ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
									ServeStale:                  "Immediate",
								},
							},
						},
					},
				},
				LinuxProfile: &datamodel.LinuxProfile{
					AdminUsername: "azureuser",
					SSH: struct {
						PublicKeys []datamodel.PublicKey "json:\"publicKeys\""
					}{
						PublicKeys: []datamodel.PublicKey{
							{
								KeyData: "dummysshkey",
							},
						},
					},
				},
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
					ClientID: "msi",
					Secret:   "msi",
				},
				CertificateProfile:  &datamodel.CertificateProfile{},
				HostedMasterProfile: &datamodel.HostedMasterProfile{},
			},
		},
		CloudSpecConfig: &datamodel.AzureEnvironmentSpecConfig{
			CloudName: "AzurePublicCloud",
			DockerSpecConfig: datamodel.DockerSpecConfig{
				DockerEngineRepo:         "https://aptdocker.azureedge.net/repo",
				DockerComposeDownloadURL: "https://github.com/docker/compose/releases/download",
			},
			KubernetesSpecConfig: datamodel.KubernetesSpecConfig{
				AzureTelemetryPID:                    "",
				KubernetesImageBase:                  "k8s.gcr.io/",
				TillerImageBase:                      "gcr.io/kubernetes-helm/",
				ACIConnectorImageBase:                "microsoft/",
				MCRKubernetesImageBase:               "mcr.microsoft.com/",
				NVIDIAImageBase:                      "nvidia/",
				AzureCNIImageBase:                    "mcr.microsoft.com/containernetworking/",
				CalicoImageBase:                      "calico/",
				EtcdDownloadURLBase:                  "",
				KubeBinariesSASURLBase:               "https://packages.aks.azure.com/kubernetes/",
				WindowsTelemetryGUID:                 "fb801154-36b9-41bc-89c2-f4d4f05472b0",
				CNIPluginsDownloadURL:                "https://packages.aks.azure.com/cni/cni-plugins-amd64-v0.7.6.tgz",
				VnetCNILinuxPluginsDownloadURL:       "https://packages.aks.azure.com/azure-cni/v1.1.3/binaries/azure-vnet-cni-linux-amd64-v1.1.3.tgz",
				VnetCNIWindowsPluginsDownloadURL:     "https://packages.aks.azure.com/azure-cni/v1.1.3/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.1.3.zip",
				ContainerdDownloadURLBase:            "https://storage.googleapis.com/cri-containerd-release/",
				CSIProxyDownloadURL:                  "https://packages.aks.azure.com/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz",
				WindowsProvisioningScriptsPackageURL: "https://packages.aks.azure.com/aks-engine/windows/provisioning/signedscripts-v0.2.2.zip",
				WindowsPauseImageURL:                 "mcr.microsoft.com/oss/kubernetes/pause:1.4.0",
				AlwaysPullWindowsPauseImage:          false,
				CseScriptsPackageURL:                 "https://packages.aks.azure.com/aks/windows/cse/",
				CNIARM64PluginsDownloadURL:           "https://packages.aks.azure.com/cni-plugins/v0.8.7/binaries/cni-plugins-linux-arm64-v0.8.7.tgz",
				VnetCNIARM64LinuxPluginsDownloadURL:  "https://packages.aks.azure.com/azure-cni/v1.4.13/binaries/azure-vnet-cni-linux-arm64-v1.4.14.tgz",
			},
			EndpointConfig: datamodel.AzureEndpointConfig{
				ResourceManagerVMDNSSuffix: "cloudapp.azure.com",
			},
			OSImageConfig: map[datamodel.Distro]datamodel.AzureOSImageConfig(nil),
		},
		K8sComponents: &datamodel.K8sComponents{
			PodInfraContainerImageURL:  "mcr.microsoft.com/oss/kubernetes/pause:3.6",
			HyperkubeImageURL:          "mcr.microsoft.com/oss/kubernetes/",
			WindowsPackageURL:          "windowspackage",
			LinuxCredentialProviderURL: "",
		},
		AgentPoolProfile: &datamodel.AgentPoolProfile{
			Name:                "nodepool2",
			VMSize:              config.Config.DefaultVMSKU,
			KubeletDiskType:     "",
			WorkloadRuntime:     "",
			DNSPrefix:           "",
			OSType:              "Linux",
			Ports:               nil,
			AvailabilityProfile: "VirtualMachineScaleSets",
			StorageProfile:      "ManagedDisks",
			VnetSubnetID:        "",
			Distro:              "aks-ubuntu-containerd-18.04-gen2",
			CustomNodeLabels: map[string]string{
				"kubernetes.azure.com/cluster":            "test-cluster", // Some AKS daemonsets require that this exists, but the value doesn't matter.
				"kubernetes.azure.com/mode":               "system",
				"kubernetes.azure.com/node-image-version": "AKSUbuntu-1804gen2containerd-2022.01.19",
			},
			PreprovisionExtension: nil,
			KubernetesConfig: &datamodel.KubernetesConfig{
				ContainerRuntime: "containerd",
			},
			LocalDNSProfile: &datamodel.LocalDNSProfile{
				EnableLocalDNS:       true,
				CPULimitInMilliCores: to.Ptr(int32(2008)),
				MemoryLimitInMB:      to.Ptr(int32(128)),
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Log",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "VnetDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Ptr(int32(1000)),
						CacheDurationInSeconds:      to.Ptr(int32(3600)),
						ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
						ServeStale:                  "Verify",
					},
					"cluster.local": {
						QueryLogging:                "Error",
						Protocol:                    "ForceTCP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Ptr(int32(1000)),
						CacheDurationInSeconds:      to.Ptr(int32(3600)),
						ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
						ServeStale:                  "Disable",
					},
					"testdomain456.com": {
						QueryLogging:                "Log",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Ptr(int32(1000)),
						CacheDurationInSeconds:      to.Ptr(int32(3600)),
						ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
						ServeStale:                  "Verify",
					},
				},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Error",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Ptr(int32(1000)),
						CacheDurationInSeconds:      to.Ptr(int32(3600)),
						ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
						ServeStale:                  "Verify",
					},
					"cluster.local": {
						QueryLogging:                "Log",
						Protocol:                    "ForceTCP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "RoundRobin",
						MaxConcurrent:               to.Ptr(int32(1000)),
						CacheDurationInSeconds:      to.Ptr(int32(3600)),
						ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
						ServeStale:                  "Disable",
					},
					"testdomain567.com": {
						QueryLogging:                "Error",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "VnetDNS",
						ForwardPolicy:               "Random",
						MaxConcurrent:               to.Ptr(int32(1000)),
						CacheDurationInSeconds:      to.Ptr(int32(3600)),
						ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
						ServeStale:                  "Immediate",
					},
				},
			},
		},
		ConfigGPUDriverIfNeeded: true,
		HTTPProxyConfig: &datamodel.HTTPProxyConfig{
			HTTPProxy:  nil,
			HTTPSProxy: nil,
			NoProxy: &[]string{
				"localhost",
				"127.0.0.1",
				"168.63.129.16",
				"169.254.169.254",
				"10.0.0.0/16",
				"agentbaker-agentbaker-e2e-t-8ecadf-c82d8251.hcp.eastus.azmk8s.io",
			},
			TrustedCA: nil,
		},
		KubeletConfig: map[string]string{
			"--address":                           "0.0.0.0",
			"--anonymous-auth":                    "false",
			"--authentication-token-webhook":      "true",
			"--authorization-mode":                "Webhook",
			"--azure-container-registry-config":   "/etc/kubernetes/azure.json",
			"--cgroups-per-qos":                   "true",
			"--client-ca-file":                    "/etc/kubernetes/certs/ca.crt",
			"--cloud-config":                      "",
			"--cloud-provider":                    "external",
			"--cluster-dns":                       "10.0.0.10",
			"--cluster-domain":                    "cluster.local",
			"--dynamic-config-dir":                "/var/lib/kubelet",
			"--enforce-node-allocatable":          "pods",
			"--event-qps":                         "0",
			"--eviction-hard":                     "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%",
			"--feature-gates":                     "RotateKubeletServerCertificate=true",
			"--image-gc-high-threshold":           "85",
			"--image-gc-low-threshold":            "80",
			"--kube-reserved":                     "cpu=100m,memory=1638Mi",
			"--kubeconfig":                        "/var/lib/kubelet/kubeconfig",
			"--max-pods":                          "110",
			"--network-plugin":                    "kubenet",
			"--node-status-update-frequency":      "10s",
			"--pod-infra-container-image":         "mcr.microsoft.com/oss/kubernetes/pause:3.6",
			"--pod-manifest-path":                 "/etc/kubernetes/manifests",
			"--pod-max-pids":                      "-1",
			"--protect-kernel-defaults":           "true",
			"--read-only-port":                    "0",
			"--resolv-conf":                       "/run/systemd/resolve/resolv.conf",
			"--rotate-certificates":               "false",
			"--streaming-connection-idle-timeout": "4h",
			"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
			"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256",
			"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
		},
		KubeproxyConfig:     map[string]string(nil),
		EnableRuncShimV2:    false,
		GPUInstanceProfile:  "",
		PrimaryScaleSetName: "",
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
		IsARM64:                   false,
		CustomCATrustConfig:       nil,
		DisableUnattendedUpgrades: true,
		SSHStatus:                 0,
		DisableCustomData:         false,
	}
	config, err := pruneKubeletConfig(k8sVersion, config)
	require.NoError(t, err)
	return config
}

// this been crafted with a lot of trial and pain, some values are not needed, but it takes a lot of time to figure out which ones.
// and we hope to move on to a different config, so I don't want to invest any more time in this-
// please keep the kubernetesVersion in sync with componets.json so that during e2e no extra binaries are required.
func baseTemplateWindows(t *testing.T, location string) *datamodel.NodeBootstrappingConfiguration {
	kubernetesVersion := "1.30.12"
	// kubernetesVersion := "1.31.9"
	// kubernetesVersion := "v1.32.5"
	config := &datamodel.NodeBootstrappingConfiguration{
		TenantID:          "tenantID",
		SubscriptionID:    config.Config.SubscriptionID,
		ResourceGroupName: "resourcegroup",

		ContainerService: &datamodel.ContainerService{
			Location: location,
			Properties: &datamodel.Properties{
				HostedMasterProfile: &datamodel.HostedMasterProfile{},
				CertificateProfile:  &datamodel.CertificateProfile{},
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    "Kubernetes",
					OrchestratorVersion: kubernetesVersion,
					KubernetesConfig: &datamodel.KubernetesConfig{
						AzureCNIURLWindows:   "https://packages.aks.azure.com/azure-cni/v1.6.21/binaries/azure-vnet-cni-windows-amd64-v1.6.21.zip",
						ClusterSubnet:        "10.224.0.0/16",
						DNSServiceIP:         "10.0.0.10",
						LoadBalancerSku:      "Standard",
						NetworkPlugin:        "azure",
						NetworkPluginMode:    "overlay",
						ServiceCIDR:          "10.0.0.0/16",
						UseInstanceMetadata:  to.Ptr(true),
						UseManagedIdentity:   false,
						WindowsContainerdURL: "https://packages.aks.azure.com/containerd/windows/",
					},
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "winnp",
						VMSize:              config.Config.DefaultVMSKU,
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
					AlwaysPullWindowsPauseImage:    to.Ptr(false),
					CSIProxyURL:                    "https://packages.aks.azure.com/csi-proxy/v1.1.2-hotfix.20230807/binaries/csi-proxy-v1.1.2-hotfix.20230807.tar.gz",
					EnableAutomaticUpdates:         to.Ptr(false),
					EnableCSIProxy:                 to.Ptr(true),
					HnsRemediatorIntervalInMinutes: to.Ptr[uint32](1),
					ImageVersion:                   "",
					SSHEnabled:                     to.Ptr(true),
					WindowsDockerVersion:           "",
					WindowsImageSourceURL:          "",
					WindowsOffer:                   "aks-windows",
					WindowsPauseImageURL:           "mcr.microsoft.com/oss/kubernetes/pause:3.9-hotfix-20230808",
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
				ACIConnectorImageBase:       "microsoft/",
				AlwaysPullWindowsPauseImage: false,
				AzureCNIImageBase:           "mcr.microsoft.com/containernetworking/",
				AzureTelemetryPID:           "",
				// CNIARM64PluginsDownloadURL:  "https://packages.aks.azure.com/cni-plugins/v0.8.7/binaries/cni-plugins-linux-arm64-v0.8.7.tgz",
				// CNIPluginsDownloadURL:       "https://packages.aks.azure.com/cni/cni-plugins-amd64-v0.7.6.tgz",
				CSIProxyDownloadURL:       "https://packages.aks.azure.com/csi-proxy/v1.1.2-hotfix.20230807/binaries/csi-proxy-v1.1.2-hotfix.20230807.tar.gz",
				CalicoImageBase:           "calico/",
				ContainerdDownloadURLBase: "https://storage.googleapis.com/cri-containerd-release/",
				// CseScriptsPackageURL is used to download the CSE scripts for Windows nodes, when use filename it is pinned to that version insteaf of current as defined in components.json
				CseScriptsPackageURL:   "https://packages.aks.azure.com/aks/windows/cse/",
				EtcdDownloadURLBase:    "",
				KubeBinariesSASURLBase: "https://packages.aks.azure.com/kubernetes/",
				KubernetesImageBase:    "k8s.gcr.io/",
				MCRKubernetesImageBase: "mcr.microsoft.com/",
				NVIDIAImageBase:        "nvidia/",
				TillerImageBase:        "gcr.io/kubernetes-helm/",
				// VnetCNIARM64LinuxPluginsDownloadURL:  "https://packages.aks.azure.com/azure-cni/v1.4.13/binaries/azure-vnet-cni-linux-arm64-v1.4.14.tgz",
				// VnetCNILinuxPluginsDownloadURL:       "https://packages.aks.azure.com/azure-cni/v1.1.3/binaries/azure-vnet-cni-linux-amd64-v1.1.3.tgz",
				VnetCNIWindowsPluginsDownloadURL:     "https://packages.aks.azure.com/azure-cni/v1.6.21/binaries/azure-vnet-cni-windows-amd64-v1.6.21.zip",
				WindowsPauseImageURL:                 "mcr.microsoft.com/oss/kubernetes/pause:3.9-hotfix-20230808",
				WindowsProvisioningScriptsPackageURL: "https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-v0.0.52.zip",
				WindowsTelemetryGUID:                 "fb801154-36b9-41bc-89c2-f4d4f05472b0",
			},
			EndpointConfig: datamodel.AzureEndpointConfig{
				ResourceManagerVMDNSSuffix: "cloudapp.azure.com",
			},
			OSImageConfig: map[datamodel.Distro]datamodel.AzureOSImageConfig(nil),
		},
		K8sComponents: &datamodel.K8sComponents{
			WindowsPackageURL: fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/windowszip/v%s-1int.zip", kubernetesVersion, kubernetesVersion),
		},
		AgentPoolProfile: &datamodel.AgentPoolProfile{
			Name:                "winnp",
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
			"--pod-infra-container-image":       "mcr.microsoft.com/oss/kubernetes/pause:3.9-hotfix-20230808",
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
	config, err := pruneKubeletConfig(kubernetesVersion, config)
	require.NoError(t, err)
	return config
}

// k8s version > 1.30.0 contains deprecated kubelet flags
func pruneKubeletConfig(kubernetesVersion string, datamodel *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrappingConfiguration, error) {
	version, err := semver.NewVersion(kubernetesVersion)
	if err != nil {
		return nil, err
	}
	constraint, err := semver.NewConstraint(">= 1.30.0")
	if err != nil {
		return nil, err
	}
	if constraint.Check(version) {
		delete(datamodel.KubeletConfig, "--azure-container-registry-config")
	}
	return datamodel, nil
}
