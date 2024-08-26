package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

type Config struct {
	Location                       string `json:"Location"`
	CACertificate                  string `json:"CACertificate"`
	KubeletClientTLSBootstrapToken string `json:"KubeletClientTLSBootstrapToken"`
	FQDN                           string `json:"FQDN"`
}

func (c *Config) Validate() error {
	errs := make([]error, 0)
	if c.Location == "" {
		errs = append(errs, errors.New("Location is required"))
	}
	if c.CACertificate == "" {
		errs = append(errs, errors.New("CACertificate is required"))
	}
	if c.KubeletClientTLSBootstrapToken == "" {
		errs = append(errs, errors.New("KubeletClientTLSBootstrapToken is required"))
	}
	if c.FQDN == "" {
		errs = append(errs, errors.New("FQDN is required"))
	}
	return errors.Join(errs...)
}

func main() {
	slog.Info("Installer started")
	ctx := context.Background()
	if err := run(ctx); err != nil {
		slog.Error("Installer finished with error", "error", err.Error())
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	slog.Info("Installer finished")
}

func run(ctx context.Context) error {
	var config *Config
	// TODO: should it be absolute path?
	// We probably want to limit configuration ability.
	configFile, err := os.Open("config.json")
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}
	if err := config.Validate(); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}

	if err := provisionStart(ctx, config); err != nil {
		return fmt.Errorf("provision start: %w", err)
	}
	return nil
}

func provisionStart(ctx context.Context, config *Config) error {
	slog.Info("Running provision_start.sh")
	defer slog.Info("Finished provision_start.sh")
	cse, err := CSEScript(ctx, config)
	if err != nil {
		return fmt.Errorf("cse script: %w", err)
	}
	slog.Info("Running command", "command", cse)
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", cse)
	cmd.Dir = "/"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func CSEScript(ctx context.Context, config *Config) (string, error) {
	tmpl := baseTemplate(config)
	nbc, err := getNodeBootstrappingForValidation(ctx, tmpl)
	if err != nil {
		return "", fmt.Errorf("get node bootstrapping: %w", err)
	}
	return nbc.CSE, nil
}

func getNodeBootstrappingForValidation(ctx context.Context, nbc *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error) {
	ab, err := agent.NewAgentBaker()
	if err != nil {
		return nil, err
	}
	nodeBootstrapping, err := ab.GetNodeBootstrapping(ctx, nbc)
	if err != nil {
		return nil, err
	}
	return nodeBootstrapping, nil
}

func baseTemplate(config *Config) *datamodel.NodeBootstrappingConfiguration {
	var (
		trueConst  = true
		falseConst = false
	)
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			ID:       "",
			Location: config.Location,
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
						NetworkPlugin:                     "kubenet",
						CustomKubeProxyImage:              "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.26.0.1",
						CustomKubeBinaryURL:               "https://acs-mirror.azureedge.net/kubernetes/v1.26.0/binaries/kubernetes-node-linux-amd64.tar.gz",
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
						LoadBalancerSku:                   "Standard",
						ExcludeMasterFromStandardLB:       nil,
						AzureCNIURLLinux:                  "https://acs-mirror.azureedge.net/azure-cni/v1.1.8/binaries/azure-vnet-cni-linux-amd64-v1.1.8.tgz",
						MaximumLoadBalancerRuleCount:      250,
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
							ContainerRuntime: "containerd",
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
					ClientID:          "msi",
					Secret:            "msi",
					ObjectID:          "",
					KeyvaultSecretRef: nil,
				},
				CertificateProfile: &datamodel.CertificateProfile{
					CaCertificate: config.CACertificate,
				},
				AADProfile:    nil,
				CustomProfile: nil,
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					FQDN: config.FQDN,
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
			PodInfraContainerImageURL: "mcr.microsoft.com/oss/kubernetes/pause:3.6",
			HyperkubeImageURL:         "mcr.microsoft.com/oss/kubernetes/",
			WindowsPackageURL:         "windowspackage",
		},
		AgentPoolProfile: &datamodel.AgentPoolProfile{
			Name:                "nodepool2",
			VMSize:              "Standard_D2ds_v5",
			OSType:              "Linux",
			AvailabilityProfile: "VirtualMachineScaleSets",
			StorageProfile:      "ManagedDisks",
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
		},
		KubeletClientTLSBootstrapToken: &config.KubeletClientTLSBootstrapToken,
		FIPSEnabled:                    false,
		HTTPProxyConfig: &datamodel.HTTPProxyConfig{
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
			"--keep-terminated-pod-volumes":       "false",
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
		KubeproxyConfig: map[string]string(nil),
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
		DisableUnattendedUpgrades: true,
	}
}
