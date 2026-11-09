package scenario

import (
	"context"
	"encoding/base64"
	"fmt"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Masterminds/semver/v3"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// this is a base kubelet config for Scriptless e2e test
func baseKubeletConfig() *aksnodeconfigv1.KubeletConfig {
	return &aksnodeconfigv1.KubeletConfig{
		EnableKubeletConfigFile: true,
		KubeletFlags:            baseLinuxKubeletFlags(),
		KubeletNodeLabels: map[string]string{
			"agentpool":                               "nodepool2",
			"kubernetes.azure.com/agentpool":          "nodepool2",
			"kubernetes.azure.com/cluster":            "test-cluster",
			"kubernetes.azure.com/mode":               "system",
			"kubernetes.azure.com/node-image-version": "AKSUbuntu-2404gen2containerd-2025.06.02",
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
			ServerTlsBootstrap: true,
			Authentication: &aksnodeconfigv1.KubeletAuthentication{
				X509: &aksnodeconfigv1.KubeletX509Authentication{
					ClientCaFile: "/etc/kubernetes/certs/ca.crt",
				},
				Webhook: &aksnodeconfigv1.KubeletWebhookAuthentication{
					Enabled: true,
				},
				Anonymous: &aksnodeconfigv1.KubeletAnonymousAuthentication{},
			},
			Authorization: &aksnodeconfigv1.KubeletAuthorization{
				Mode:    "Webhook",
				Webhook: &aksnodeconfigv1.KubeletWebhookAuthorization{},
			},
			EventRecordQps: to.Ptr(int32(0)),
			ClusterDomain:  "cluster.local",
			ClusterDns: []string{
				clusterDNSServiceIP,
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
			FeatureGates: map[string]bool{
				"RotateKubeletServerCertificate": true,
			},
			KubeReserved: map[string]string{
				"cpu":    "100m",
				"memory": "1638Mi",
			},
			EnforceNodeAllocatable: []string{
				"pods",
			},
		},
	}
}

func getBaseNBC(ctx context.Context, cluster *Cluster, vhd *config.Image) (*datamodel.NodeBootstrappingConfiguration, error) {
	if cluster == nil || cluster.Model == nil || cluster.Model.Location == nil || cluster.Model.Properties == nil ||
		cluster.KubeletIdentity == nil || cluster.KubeletIdentity.ClientID == nil {
		return nil, fmt.Errorf("cluster is incomplete")
	}
	if vhd == nil {
		return nil, fmt.Errorf("VHD is nil")
	}
	var nbc *datamodel.NodeBootstrappingConfiguration
	var err error

	if vhd.Distro.IsWindowsDistro() {
		if cluster.Model.ID == nil || cluster.Model.Properties.NodeResourceGroup == nil {
			return nil, fmt.Errorf("Windows cluster is missing its ID or node resource group")
		}
		nbc, err = baseTemplateWindows(*cluster.Model.Location)
		if err != nil {
			return nil, err
		}

		// these aren't needed since we use TLS bootstrapping instead, though windows bootstrapping expects non-empty values
		nbc.ContainerService.Properties.CertificateProfile.ClientCertificate = "none"
		nbc.ContainerService.Properties.CertificateProfile.ClientPrivateKey = "none"

		// this is also set below within NBC's UserAssignedIdentityClientID field, though windows is special and needs
		// these other fields to be set as well for the client ID to actually be included within CSE
		nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
		nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UserAssignedID = *cluster.KubeletIdentity.ClientID

		nbc.ContainerService.Properties.ClusterID = *cluster.Model.ID
		nbc.SubscriptionID = config.Config.SubscriptionID
		nbc.ResourceGroupName = *cluster.Model.Properties.NodeResourceGroup
	} else {
		if cluster.Model.Properties.CurrentKubernetesVersion == nil {
			return nil, fmt.Errorf("cluster has no current Kubernetes version")
		}
		if cluster.Model.Properties.NodeResourceGroup == nil {
			return nil, fmt.Errorf("cluster is missing its node resource group")
		}
		nbc, err = baseTemplateLinux(*cluster.Model.Location, *cluster.Model.Properties.CurrentKubernetesVersion, vhd.Arch)
		if err == nil {
			// RP always sets these; AKSNodeConfig validation requires them.
			nbc.SubscriptionID = config.Config.SubscriptionID
			nbc.ResourceGroupName = *cluster.Model.Properties.NodeResourceGroup
		}
	}
	if err != nil {
		return nil, err
	}

	// use the cluster's kubelet identity to simulate how AKS works in production
	// we assume that all E2E clusters are created with a user-assigned managed identity for the kubelet (not a service principal)
	nbc.UserAssignedIdentityClientID = *cluster.KubeletIdentity.ClientID

	// pass in the bootstrap token and enable secure TLS bootstrapping according to E2E config
	// this allows us to test the following bootstrapping modes:
	// 1. secure TLS bootstrapping
	// 2. secure TLS bootstrapping failure, which falls back to bootstrap token
	// 3. bootstrap token
	nbc.KubeletClientTLSBootstrapToken = &cluster.ClusterParams.BootstrapToken
	// remove bootstrap-kubeconfig within e2e by default when not on the frozen AzureLinux V2 image
	nbc.RemoveBootstrapKubeconfig = vhd.Distro != datamodel.AKSAzureLinuxV2Gen2
	nbc.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{
		Enabled: config.Config.EnableSecureTLSBootstrapping,
	}
	nbc.TenantID = cluster.TenantID
	nbc.ContainerService.Properties.CertificateProfile.CaCertificate = string(cluster.ClusterParams.CACert)
	nbc.ContainerService.Properties.HostedMasterProfile.FQDN = cluster.ClusterParams.FQDN
	nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = vhd.Distro
	nbc.AgentPoolProfile.Distro = vhd.Distro
	return nbc, nil
}

type aksNodeConfigOpt func(*Cluster, *aksnodeconfigv1.Configuration) error

// createAKSNodeConfig constructs provisioning inputs directly from the cluster and image.
func createAKSNodeConfig(cluster *Cluster, vhd *config.Image, opts ...aksNodeConfigOpt) (*aksnodeconfigv1.Configuration, error) {
	if cluster == nil || cluster.Model == nil || cluster.Model.Location == nil || cluster.Model.Properties == nil ||
		cluster.Model.Properties.CurrentKubernetesVersion == nil || cluster.Model.Properties.NodeResourceGroup == nil ||
		cluster.KubeletIdentity == nil || cluster.KubeletIdentity.ClientID == nil || cluster.ClusterParams == nil {
		return nil, fmt.Errorf("cluster is incomplete for AKS node config")
	}
	if vhd == nil || vhd.Distro.IsWindowsDistro() {
		return nil, fmt.Errorf("AKS node config requires a Linux VHD")
	}
	if vhd.Arch != "amd64" && vhd.Arch != "arm64" {
		return nil, fmt.Errorf("unsupported AKS node config architecture %q", vhd.Arch)
	}
	k8sVersion := *cluster.Model.Properties.CurrentKubernetesVersion
	is134OrAbove, err := toolkit.CheckK8sConstraint(k8sVersion, ">=1.34.0")
	if err != nil {
		return nil, fmt.Errorf("parse Kubernetes version %q: %w", k8sVersion, err)
	}
	kubeletConfig := baseKubeletConfig()
	if err := pruneKubeletFlags(k8sVersion, kubeletConfig.KubeletFlags); err != nil {
		return nil, fmt.Errorf("prune AKS kubelet flags: %w", err)
	}
	if is134OrAbove {
		kubeletConfig.KubeletConfigFileConfig.StreamingConnectionIdleTimeout = ""
	}
	bootstrappingConfig := &aksnodeconfigv1.BootstrappingConfig{
		TlsBootstrappingToken:     to.Ptr(cluster.ClusterParams.BootstrapToken),
		RemoveBootstrapKubeconfig: to.Ptr(vhd.Distro != datamodel.AKSAzureLinuxV2Gen2),
	}
	if config.Config.EnableSecureTLSBootstrapping {
		bootstrappingConfig.BootstrappingAuthMethod = aksnodeconfigv1.BootstrappingAuthMethod_BOOTSTRAPPING_AUTH_METHOD_SECURE_TLS_BOOTSTRAPPING
	}
	// Retain the shared E2E resource naming defaults without constructing an NBC.
	networkDefaults := &datamodel.Properties{
		OrchestratorProfile: &datamodel.OrchestratorProfile{OrchestratorType: "Kubernetes"},
		HostedMasterProfile: &datamodel.HostedMasterProfile{},
	}
	cfg := &aksnodeconfigv1.Configuration{
		Version:             "v1",
		BootstrappingConfig: bootstrappingConfig,
		DisableCustomData:   true,
		LinuxAdminUsername:  "azureuser",
		VmSize:              config.Config.VMSKU(),
		IsVhd:               to.Ptr(vhd.Distro.IsVHDDistro()),
		IsKata:              vhd.Distro.IsKataDistro(),
		ClusterConfig: &aksnodeconfigv1.ClusterConfig{
			Location:            *cluster.Model.Location,
			ResourceGroup:       *cluster.Model.Properties.NodeResourceGroup,
			VmType:              aksnodeconfigv1.VmType_VM_TYPE_VMSS,
			UseInstanceMetadata: true,
			ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{
				SecurityGroupName: networkDefaults.GetNSGName(),
				VnetName:          networkDefaults.GetVirtualNetworkName(),
				Subnet:            networkDefaults.GetSubnetName(),
				RouteTable:        networkDefaults.GetRouteTableName(),
				CoreDnsServiceIp:  clusterDNSServiceIP,
			},
			CloudProviderConfig: &aksnodeconfigv1.CloudProviderConfig{
				Backoff:              to.Ptr(true),
				BackoffMode:          "v2",
				BackoffRetries:       to.Ptr(int32(6)),
				BackoffExponent:      to.Ptr(float64(0)),
				BackoffDuration:      to.Ptr(int32(5)),
				BackoffJitter:        to.Ptr(float64(0)),
				RateLimit:            to.Ptr(true),
				RateLimitQps:         to.Ptr(float64(10)),
				RateLimitQpsWrite:    to.Ptr(float64(10)),
				RateLimitBucket:      to.Ptr(int32(100)),
				RateLimitBucketWrite: to.Ptr(int32(100)),
			},
			LoadBalancerConfig: &aksnodeconfigv1.LoadBalancerConfig{
				LoadBalancerSku: aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_STANDARD,
			},
		},
		ApiServerConfig: &aksnodeconfigv1.ApiServerConfig{
			ApiServerName: cluster.ClusterParams.FQDN,
		},
		AuthConfig: &aksnodeconfigv1.AuthConfig{
			ServicePrincipalId:     "msi",
			ServicePrincipalSecret: base64.StdEncoding.EncodeToString([]byte(base64.StdEncoding.EncodeToString([]byte("msi")))),
			TenantId:               cluster.TenantID,
			SubscriptionId:         config.Config.SubscriptionID,
			AssignedIdentityId:     *cluster.KubeletIdentity.ClientID,
		},
		NetworkConfig: &aksnodeconfigv1.NetworkConfig{
			NetworkPlugin:     aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_KUBENET,
			CniPluginsUrl:     "https://packages.aks.azure.com/cni/cni-plugins-amd64-v0.7.6.tgz",
			VnetCniPluginsUrl: "https://packages.aks.azure.com/azure-cni/v1.6.21/binaries/azure-vnet-cni-linux-amd64-v1.6.21.tgz",
		},
		GpuConfig: &aksnodeconfigv1.GpuConfig{
			ConfigGpuDriver: true,
			GpuDevicePlugin: false,
		},
		KubernetesVersion: k8sVersion,
		ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{
			ContainerdDownloadUrlBase: "https://storage.googleapis.com/cri-containerd-release/",
		},
		OutboundCommand:  `curl -v --insecure --proxy-insecure https://mcr.microsoft.com/v2/`,
		KubernetesCaCert: base64.StdEncoding.EncodeToString(cluster.ClusterParams.CACert),
		KubeBinaryConfig: &aksnodeconfigv1.KubeBinaryConfig{
			PodInfraContainerImageUrl: "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2",
		},
		HttpProxyConfig: &aksnodeconfigv1.HttpProxyConfig{
			NoProxyEntries: []string{
				"localhost", "127.0.0.1", "168.63.129.16", "169.254.169.254",
				clusterServiceCIDR, "agentbaker-agentbaker-e2e-t-8ecadf-c82d8251.hcp.eastus.azmk8s.io",
			},
		},
		LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
			EnableLocalDns:       true,
			CpuLimitInMilliCores: to.Ptr(int32(2008)),
			MemoryLimitInMb:      to.Ptr(int32(128)),
			VnetDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
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
			KubeDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
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
			CriticalFqdns: []string{
				"mcr.microsoft.com",
				"login.microsoftonline.com",
				"packages.aks.azure.com",
			},
		},
		NeedsCgroupv2: to.Ptr(true),
		KubeletConfig: kubeletConfig,
	}
	if !is134OrAbove {
		cfg.KubeProxyUrl = fmt.Sprintf("mcr.microsoft.com/oss/kubernetes/kube-proxy:v%s", k8sVersion)
		cfg.KubeBinaryConfig.KubeBinaryUrl = fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/binaries/kubernetes-node-linux-%s.tar.gz", k8sVersion, vhd.Arch)
	}
	if vhd.Arch == "arm64" {
		cfg.NetworkConfig.VnetCniPluginsUrl = "https://packages.aks.azure.com/azure-cni/v1.4.13/binaries/azure-vnet-cni-linux-arm64-v1.4.14.tgz"
	}
	for _, opt := range opts {
		if err := opt(cluster, cfg); err != nil {
			return nil, fmt.Errorf("configure AKS node config: %w", err)
		}
		if cfg.KubeletConfig == nil {
			return nil, fmt.Errorf("AKS node config has no kubelet config")
		}
		if cfg.KubeletConfig.KubeletDiskType == aksnodeconfigv1.KubeletDisk_KUBELET_DISK_TEMP_DISK && cfg.KubeletConfig.ContainerDataDir == "" {
			cfg.KubeletConfig.ContainerDataDir = datamodel.TempDiskContainerDataDir
		}
	}
	omittedFlags := datamodel.GetCommandLineOmittedKubeletConfigFlags()
	for key := range cfg.KubeletConfig.KubeletFlags {
		if omittedFlags[key] || (cfg.KubeletConfig.EnableKubeletConfigFile && agent.TranslatedKubeletConfigFlags[key]) {
			delete(cfg.KubeletConfig.KubeletFlags, key)
		}
	}
	return cfg, nil
}

// this is huge, but accurate, so leave it here.
// TODO(ace): minimize the actual required defaults.
// this is what we previously used for bash e2e from e2e/nodebootstrapping_template.json.
// which itself was extracted from baker_test.go logic, which was inherited from aks-engine.
func baseTemplateLinux(location string, k8sVersion string, arch string) (*datamodel.NodeBootstrappingConfiguration, error) {
	customKubeProxyImage := fmt.Sprintf("mcr.microsoft.com/oss/kubernetes/kube-proxy:v%s", k8sVersion)
	customKubeBinaryURL := fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/binaries/kubernetes-node-linux-%s.tar.gz", k8sVersion, arch)
	is134OrAbove, pErr := toolkit.CheckK8sConstraint(k8sVersion, ">=1.34.0")
	if pErr != nil {
		return nil, fmt.Errorf("parse Kubernetes version %q: %w", k8sVersion, pErr)
	}
	if is134OrAbove {
		customKubeProxyImage = ""
		customKubeBinaryURL = ""
	}
	config := &datamodel.NodeBootstrappingConfiguration{
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
						DNSServiceIP:                      clusterDNSServiceIP,
						ServiceCIDR:                       clusterServiceCIDR,
						UseManagedIdentity:                false,
						UserAssignedID:                    "",
						UserAssignedClientID:              "",
						CustomHyperkubeImage:              "",
						CustomKubeProxyImage:              customKubeProxyImage,
						CustomKubeBinaryURL:               customKubeBinaryURL,
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
						LoadBalancerSku:                   "standard",
						ExcludeMasterFromStandardLB:       nil,
						AzureCNIURLLinux:                  "https://packages.aks.azure.com/azure-cni/v1.6.21/binaries/azure-vnet-cni-linux-amd64-v1.6.21.tgz",
						AzureCNIURLARM64Linux:             "",
						AzureCNIURLWindows:                "",
						MaximumLoadBalancerRuleCount:      148,
						PrivateAzureRegistryServer:        "",
						NetworkPluginMode:                 "",
					},
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "nodepool2",
						VMSize:              config.Config.VMSKU(),
						KubeletDiskType:     "",
						WorkloadRuntime:     "",
						DNSPrefix:           "",
						OSType:              "Linux",
						Ports:               nil,
						AvailabilityProfile: "VirtualMachineScaleSets",
						StorageProfile:      "ManagedDisks",
						VnetSubnetID:        "",
						Distro:              "aks-ubuntu-containerd-24.04-gen2",
						CustomNodeLabels: map[string]string{
							"kubernetes.azure.com/cluster":            "test-cluster", // Some AKS daemonsets require that this exists, but the value doesn't matter.
							"kubernetes.azure.com/mode":               "system",
							"kubernetes.azure.com/node-image-version": "AKSUbuntu-2404gen2containerd-2025.06.02",
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
							DNSServiceIP:                      clusterDNSServiceIP,
							ServiceCIDR:                       clusterServiceCIDR,
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
							CriticalFQDNs: []string{
								"mcr.microsoft.com",
								"login.microsoftonline.com",
								"packages.aks.azure.com",
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
					Secret:   base64.StdEncoding.EncodeToString([]byte("msi")),
				},
				CertificateProfile:  &datamodel.CertificateProfile{},
				HostedMasterProfile: &datamodel.HostedMasterProfile{},
			},
		},
		CloudSpecConfig: &datamodel.AzureEnvironmentSpecConfig{
			CloudName: "AzurePublicCloud",
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
				VnetCNILinuxPluginsDownloadURL:       "https://packages.aks.azure.com/azure-cni/v1.6.21/binaries/azure-vnet-cni-linux-amd64-v1.6.21.tgz",
				VnetCNIWindowsPluginsDownloadURL:     "https://packages.aks.azure.com/azure-cni/v1.1.3/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.1.3.zip",
				ContainerdDownloadURLBase:            "https://storage.googleapis.com/cri-containerd-release/",
				CSIProxyDownloadURL:                  "https://packages.aks.azure.com/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz",
				WindowsProvisioningScriptsPackageURL: "https://packages.aks.azure.com/aks-engine/windows/provisioning/signedscripts-v0.2.2.zip",
				WindowsPauseImageURL:                 "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2",
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
			PodInfraContainerImageURL:  "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2",
			HyperkubeImageURL:          "mcr.microsoft.com/oss/kubernetes/",
			WindowsPackageURL:          "windowspackage",
			LinuxCredentialProviderURL: "",
		},
		AgentPoolProfile: &datamodel.AgentPoolProfile{
			Name:                "nodepool2",
			VMSize:              config.Config.VMSKU(),
			KubeletDiskType:     "",
			WorkloadRuntime:     "",
			DNSPrefix:           "",
			OSType:              "Linux",
			Ports:               nil,
			AvailabilityProfile: "VirtualMachineScaleSets",
			StorageProfile:      "ManagedDisks",
			VnetSubnetID:        "",
			Distro:              "aks-ubuntu-containerd-24.04-gen2",
			CustomNodeLabels: map[string]string{
				"kubernetes.azure.com/cluster":            "test-cluster", // Some AKS daemonsets require that this exists, but the value doesn't matter.
				"kubernetes.azure.com/mode":               "system",
				"kubernetes.azure.com/node-image-version": "AKSUbuntu-2404gen2containerd-2025.06.02",
			},
			PreprovisionExtension: nil,
			KubernetesConfig: &datamodel.KubernetesConfig{
				ContainerRuntime: "containerd",
				DNSServiceIP:     clusterDNSServiceIP,
				ServiceCIDR:      clusterServiceCIDR,
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
				CriticalFQDNs: []string{
					"mcr.microsoft.com",
					"login.microsoftonline.com",
					"packages.aks.azure.com",
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
				clusterServiceCIDR,
				"agentbaker-agentbaker-e2e-t-8ecadf-c82d8251.hcp.eastus.azmk8s.io",
			},
			TrustedCA: nil,
		},
		KubeletConfig:       baseLinuxKubeletFlags(),
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
		EnableKubeletConfigFile:   true,
	}
	config, err := pruneKubeletConfig(k8sVersion, config)
	if err != nil {
		return nil, fmt.Errorf("prune Linux kubelet config: %w", err)
	}
	return config, nil
}

// this been crafted with a lot of trial and pain, some values are not needed, but it takes a lot of time to figure out which ones.
// and we hope to move on to a different config, so I don't want to invest any more time in this-
// please keep the kubernetesVersion in sync with componets.json so that during e2e no extra binaries are required.
func baseTemplateWindows(location string) (*datamodel.NodeBootstrappingConfiguration, error) {
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
						DNSServiceIP:         clusterDNSServiceIP,
						LoadBalancerSku:      "standard",
						NetworkPlugin:        "azure",
						NetworkPluginMode:    "overlay",
						ServiceCIDR:          clusterServiceCIDR,
						UseInstanceMetadata:  to.Ptr(true),
						UseManagedIdentity:   false,
						WindowsContainerdURL: "https://packages.aks.azure.com/containerd/windows/",
					},
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "winnp",
						VMSize:              config.Config.VMSKU(),
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
					Secret:   base64.StdEncoding.EncodeToString([]byte("msi")),
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
					WindowsPauseImageURL:           "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2",
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
				WindowsPauseImageURL:                 "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2",
				WindowsProvisioningScriptsPackageURL: "",
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
			"--resolv-conf":                     "\"\"\"\"",
			"--cluster-dns":                     clusterDNSServiceIP,
			"--cluster-domain":                  "cluster.local",
			"--rotate-certificates":             "true",
			"--rotate-server-certificates":      "true",
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
	if err != nil {
		return nil, fmt.Errorf("prune Windows kubelet config: %w", err)
	}
	return config, nil
}

// k8s version > 1.30.0 contains deprecated kubelet flags
func pruneKubeletConfig(kubernetesVersion string, datamodel *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrappingConfiguration, error) {
	if err := pruneKubeletFlags(kubernetesVersion, datamodel.KubeletConfig); err != nil {
		return nil, err
	}
	return datamodel, nil
}

func pruneKubeletFlags(kubernetesVersion string, flags map[string]string) error {
	version, err := semver.NewVersion(kubernetesVersion)
	if err != nil {
		return err
	}
	constraint130, err := semver.NewConstraint(">= 1.30.0")
	if err != nil {
		return err
	}
	if constraint130.Check(version) {
		delete(flags, "--azure-container-registry-config")
	}
	constraint134, err := semver.NewConstraint(">= 1.34.0")
	if err != nil {
		return err
	}
	if constraint134.Check(version) {
		delete(flags, "--streaming-connection-idle-timeout")
	}
	return nil
}

func baseLinuxKubeletFlags() map[string]string {
	return map[string]string{
		"--address":                           "0.0.0.0",
		"--anonymous-auth":                    "false",
		"--authentication-token-webhook":      "true",
		"--authorization-mode":                "Webhook",
		"--azure-container-registry-config":   "/etc/kubernetes/azure.json",
		"--cgroups-per-qos":                   "true",
		"--client-ca-file":                    "/etc/kubernetes/certs/ca.crt",
		"--cloud-provider":                    "external",
		"--cluster-dns":                       clusterDNSServiceIP,
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
		"--pod-manifest-path":                 "/etc/kubernetes/manifests",
		"--pod-max-pids":                      "-1",
		"--protect-kernel-defaults":           "true",
		"--read-only-port":                    "0",
		"--resolv-conf":                       "/run/systemd/resolve/resolv.conf",
		"--rotate-certificates":               "true",
		"--rotate-server-certificates":        "true",
		"--streaming-connection-idle-timeout": "4h",
		"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
		"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256",
		"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
	}
}
