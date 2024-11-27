package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
)

func getBaseNodeBootstrappingConfiguration(ctx context.Context, t *testing.T, kube *Kubeclient) *datamodel.NodeBootstrappingConfiguration {
	t.Log("getting the node bootstrapping configuration for cluster")
	clusterParams := extractClusterParameters(ctx, t, kube)
	nbc := baseTemplate(config.Config.Location)
	nbc.ContainerService.Properties.CertificateProfile.CaCertificate = string(clusterParams.CACert)
	nbc.KubeletClientTLSBootstrapToken = &clusterParams.BootstrapToken
	nbc.ContainerService.Properties.HostedMasterProfile.FQDN = clusterParams.FQDN
	return nbc
}

// is a temporary workaround
// eventually we want to phase out usage of nbc
func nbcToAKSNodeConfigV1(nbc *datamodel.NodeBootstrappingConfiguration) *aksnodeconfigv1.Configuration {
	cs := nbc.ContainerService
	agentPool := nbc.AgentPoolProfile
	agent.ValidateAndSetLinuxNodeBootstrappingConfiguration(nbc)

	config := &aksnodeconfigv1.Configuration{
		Version:            "v0",
		DisableCustomData:  false,
		LinuxAdminUsername: "azureuser",
		VmSize:             "Standard_D2ds_v5",
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
		KubeletConfig: &aksnodeconfigv1.KubeletConfig{
			KubeletClientKey:         base64.StdEncoding.EncodeToString([]byte(cs.Properties.CertificateProfile.ClientPrivateKey)),
			KubeletConfigFileContent: base64.StdEncoding.EncodeToString([]byte(agent.GetKubeletConfigFileContent(nbc.KubeletConfig, nbc.AgentPoolProfile.CustomKubeletConfig))),
			EnableKubeletConfigFile:  false,
			KubeletFlags:             helpers.GetKubeletConfigFlag(nbc.KubeletConfig, cs, agentPool, false),
			KubeletNodeLabels:        helpers.GetKubeletNodeLabels(agentPool),
		},
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
		NeedsCgroupv2: to.BoolPtr(true),
	}
	return config
}

// this is huge, but accurate, so leave it here.
// TODO(ace): minimize the actual required defaults.
// this is what we previously used for bash e2e from e2e/nodebootstrapping_template.json.
// which itself was extracted from baker_test.go logic, which was inherited from aks-engine.
func baseTemplate(location string) *datamodel.NodeBootstrappingConfiguration {
	var (
		trueConst  = true
		falseConst = false
	)
	return &datamodel.NodeBootstrappingConfiguration{
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
					OrchestratorVersion: "1.29.6",
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
						CustomKubeProxyImage:              "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.26.0.1",
						CustomKubeBinaryURL:               "https://acs-mirror.azureedge.net/kubernetes/v1.26.0/binaries/kubernetes-node-linux-amd64.tar.gz",
						MobyVersion:                       "",
						ContainerdVersion:                 "",
						WindowsNodeBinariesURL:            "",
						WindowsContainerdURL:              "",
						WindowsSdnPluginURL:               "",
						UseInstanceMetadata:               &trueConst,
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
						CloudProviderBackoff:              &trueConst,
						CloudProviderBackoffRetries:       6,
						CloudProviderBackoffJitter:        0.0,
						CloudProviderBackoffDuration:      5,
						CloudProviderBackoffExponent:      0.0,
						CloudProviderRateLimit:            &trueConst,
						CloudProviderRateLimitQPS:         10.0,
						CloudProviderRateLimitQPSWrite:    10.0,
						CloudProviderRateLimitBucket:      100,
						CloudProviderRateLimitBucketWrite: 100,
						CloudProviderDisableOutboundSNAT:  &falseConst,
						NodeStatusUpdateFrequency:         "",
						LoadBalancerSku:                   "Standard",
						ExcludeMasterFromStandardLB:       nil,
						AzureCNIURLLinux:                  "https://acs-mirror.azureedge.net/azure-cni/v1.1.8/binaries/azure-vnet-cni-linux-amd64-v1.1.8.tgz",
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
						VMSize:              "Standard_D2ds_v5",
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
						LocalDnsProfile: &datamodel.LocalDnsProfile{
							CurrentServiceStatus: "Enabled",
							CPULimit:             2,
							MemoryLimitInMB:      128,
							CoreDnsImageUrl:      "mcr.microsoft.com/oss/kubernetes/coredns:v1.9.4-hotfix.20240704",
							VnetDnsOverrides: map[string]datamodel.DnsOverride{
								".": {
									QueryLogging:           "errors",
									ForceTCP:               false,
									ForwardPolicy:          "sequential",
									MaxConcurrent:          1000,
									CacheDurationInSeconds: 3600,
									ServeStale:             "verify",
								},
								"sub.domain1.com": {
									QueryLogging:           "log",
									ForceTCP:               false,
									ForwardPolicy:          "sequential",
									MaxConcurrent:          1000,
									CacheDurationInSeconds: 3600,
									ServeStale:             "verify",
								},
							},
							KubeDnsOverrides: map[string]datamodel.DnsOverride{
								".": {
									QueryLogging:           "errors",
									ForceTCP:               true,
									ForwardPolicy:          "sequential",
									MaxConcurrent:          1000,
									CacheDurationInSeconds: 3600,
									ServeStale:             "verify",
								},
								"sub.domain1.com": {
									QueryLogging:           "errors",
									ForceTCP:               false,
									ForwardPolicy:          "sequential",
									MaxConcurrent:          1000,
									CacheDurationInSeconds: 3600,
									ServeStale:             "verify",
								},
							},
							NodeListenerIP:    "169.254.10.10",
							ClusterListenerIP: "169.254.10.11",
							CoreDnsServiceIP:  "10.0.0.10",
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
					Secrets:            nil,
					Distro:             "",
					CustomSearchDomain: nil,
				},
				WindowsProfile:     nil,
				ExtensionProfiles:  nil,
				DiagnosticsProfile: nil,
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
					ClientID:          "msi",
					Secret:            "msi",
					ObjectID:          "",
					KeyvaultSecretRef: nil,
				},
				CertificateProfile: &datamodel.CertificateProfile{
					CaCertificate:         "",
					APIServerCertificate:  "",
					ClientCertificate:     "",
					ClientPrivateKey:      "",
					KubeConfigCertificate: "",
					KubeConfigPrivateKey:  "",
				},
				AADProfile:    nil,
				CustomProfile: nil,
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					FQDN:                    "",
					IPAddress:               "",
					DNSPrefix:               "",
					FQDNSubdomain:           "",
					Subnet:                  "",
					APIServerWhiteListRange: nil,
					IPMasqAgent:             true,
				},
				AddonProfiles:       map[string]datamodel.AddonProfile(nil),
				FeatureFlags:        nil,
				CustomCloudEnv:      nil,
				CustomConfiguration: nil,
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
				KubeBinariesSASURLBase:               "https://acs-mirror.azureedge.net/kubernetes/",
				WindowsTelemetryGUID:                 "fb801154-36b9-41bc-89c2-f4d4f05472b0",
				CNIPluginsDownloadURL:                "https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-v0.7.6.tgz",
				VnetCNILinuxPluginsDownloadURL:       "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-linux-amd64-v1.1.3.tgz",
				VnetCNIWindowsPluginsDownloadURL:     "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.1.3.zip",
				ContainerdDownloadURLBase:            "https://storage.googleapis.com/cri-containerd-release/",
				CSIProxyDownloadURL:                  "https://acs-mirror.azureedge.net/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz",
				WindowsProvisioningScriptsPackageURL: "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.2.2.zip",
				WindowsPauseImageURL:                 "mcr.microsoft.com/oss/kubernetes/pause:1.4.0",
				AlwaysPullWindowsPauseImage:          false,
				CseScriptsPackageURL:                 "https://acs-mirror.azureedge.net/aks/windows/cse/csescripts-v0.0.1.zip",
				CNIARM64PluginsDownloadURL:           "https://acs-mirror.azureedge.net/cni-plugins/v0.8.7/binaries/cni-plugins-linux-arm64-v0.8.7.tgz",
				VnetCNIARM64LinuxPluginsDownloadURL:  "https://acs-mirror.azureedge.net/azure-cni/v1.4.13/binaries/azure-vnet-cni-linux-arm64-v1.4.14.tgz",
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
			VMSize:              "Standard_D2ds_v5",
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
			LocalDnsProfile: &datamodel.LocalDnsProfile{
				CurrentServiceStatus: "Enabled",
				CPULimit:             2,
				MemoryLimitInMB:      128,
				CoreDnsImageUrl:      "mcr.microsoft.com/oss/kubernetes/coredns:v1.9.4-hotfix.20240704",
				VnetDnsOverrides: map[string]datamodel.DnsOverride{
					".": {
						QueryLogging:           "errors",
						ForceTCP:               false,
						ForwardPolicy:          "sequential",
						MaxConcurrent:          1000,
						CacheDurationInSeconds: 3600,
						ServeStale:             "verify",
					},
					"sub.domain1.com": {
						QueryLogging:           "log",
						ForceTCP:               false,
						ForwardPolicy:          "sequential",
						MaxConcurrent:          1000,
						CacheDurationInSeconds: 3600,
						ServeStale:             "verify",
					},
				},
				KubeDnsOverrides: map[string]datamodel.DnsOverride{
					".": {
						QueryLogging:           "errors",
						ForceTCP:               true,
						ForwardPolicy:          "sequential",
						MaxConcurrent:          1000,
						CacheDurationInSeconds: 3600,
						ServeStale:             "verify",
					},
					"sub.domain1.com": {
						QueryLogging:           "errors",
						ForceTCP:               false,
						ForwardPolicy:          "sequential",
						MaxConcurrent:          1000,
						CacheDurationInSeconds: 3600,
						ServeStale:             "verify",
					},
				},
				NodeListenerIP:    "169.254.10.10",
				ClusterListenerIP: "169.254.10.11",
				CoreDnsServiceIP:  "10.0.0.10",
			},
		},
		TenantID:                       "",
		SubscriptionID:                 "",
		ResourceGroupName:              "",
		UserAssignedIdentityClientID:   "",
		OSSKU:                          "",
		ConfigGPUDriverIfNeeded:        true,
		Disable1804SystemdResolved:     false,
		EnableGPUDevicePluginIfNeeded:  false,
		EnableKubeletConfigFile:        false,
		EnableNvidia:                   false,
		EnableACRTeleportPlugin:        false,
		TeleportdPluginURL:             "",
		ContainerdVersion:              "",
		RuncVersion:                    "",
		ContainerdPackageURL:           "",
		RuncPackageURL:                 "",
		KubeletClientTLSBootstrapToken: nil,
		FIPSEnabled:                    false,
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
}

func getHTTPServerTemplate(podName, nodeName string, isAirgap bool) string {
	image := "mcr.microsoft.com/cbl-mariner/busybox:2.0"
	if isAirgap {
		image = fmt.Sprintf("%s.azurecr.io/aks/cbl-mariner/busybox:2.0", config.PrivateACRName)
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
spec:
  containers:
  - name: mariner
    image: %s
    imagePullPolicy: IfNotPresent
    command: ["sh", "-c"]
    args:
    - |
      mkdir -p /www &&
      echo '<!DOCTYPE html><html><head><title></title></head><body></body></html>' > /www/index.html &&
      httpd -f -p 80 -h /www
    ports:
    - containerPort: 80
  nodeSelector:
    kubernetes.io/hostname: %s
  readinessProbe:
      periodSeconds: 1
      httpGet:
        path: /
        port: 80
`, podName, image, nodeName)
}

func getWasmSpinPodTemplate(podName, nodeName string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
spec:
  runtimeClassName: wasmtime-spin
  containers:
  - name: spin-hello
    image: ghcr.io/spinkube/containerd-shim-spin/examples/spin-rust-hello:v0.15.1
    imagePullPolicy: IfNotPresent
    command: ["/"]
    resources: # limit the resources to 128Mi of memory and 100m of CPU
      limits:
        cpu: 100m
        memory: 128Mi
      requests:
        cpu: 100m
        memory: 128Mi
    readinessProbe:
      periodSeconds: 1
      httpGet:
        path: /hello
        port: 80
  nodeSelector:
    kubernetes.io/hostname: %s
`, podName, nodeName)
}
