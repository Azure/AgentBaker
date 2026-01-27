package parser

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCSECmd(t *testing.T) {
	tests := []struct {
		name                 string
		folder               string
		k8sVersion           string
		aksNodeConfigUpdator func(*aksnodeconfigv1.Configuration)
		validator            func(cmd *exec.Cmd)
	}{
		{
			name:       "AKSUbuntu2204 containerd with multi-instance GPU",
			folder:     "AKSUbuntu2204+Containerd+MIG",
			k8sVersion: "1.19.13",
			aksNodeConfigUpdator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				aksNodeConfig.GpuConfig.GpuInstanceProfile = "MIG7g"
				// Skip GPU driver install
				aksNodeConfig.GpuConfig.EnableNvidia = to.Ptr(false)
				aksNodeConfig.VmSize = "Standard_ND96asr_v4"
			},
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				assert.Equal(t, "false", vars["GPU_NODE"])
				assert.NotEmpty(t, vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"])
				// Ensure the containerd config does not use the
				// nvidia container runtime when skipping the
				// GPU driver install, since it will fail to run even non-GPU
				// pods, as it will not be installed.
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]))
				require.NoError(t, err)
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
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
`
				require.Equal(t, expectedShimConfig, containerdConfigFileContent)
			},
		},
		{
			name:       "AKSUbuntu2204 DisableSSH with enabled ssh",
			folder:     "AKSUbuntu2204+SSHStatusOn",
			k8sVersion: "1.24.2",
			aksNodeConfigUpdator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				aksNodeConfig.EnableSsh = to.Ptr(true)
			},
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				assert.Equal(t, "false", vars["DISABLE_SSH"])
			},
		},
		{
			name:       "AKSUbuntu2204 DISABLE_PUBKEY_AUTH with disabled pubkey auth",
			folder:     "AKSUbuntu2204+DisablePubkeyAuth",
			k8sVersion: "1.24.2",
			aksNodeConfigUpdator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				aksNodeConfig.DisablePubkeyAuth = to.Ptr(true)
			},
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				assert.Equal(t, "true", vars["DISABLE_PUBKEY_AUTH"])
			},
		},
		{
			name:       "AKSUbuntu2204 DISABLE_PUBKEY_AUTH with enabled pubkey auth",
			folder:     "AKSUbuntu2204+EnablePubkeyAuth",
			k8sVersion: "1.24.2",
			aksNodeConfigUpdator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				aksNodeConfig.DisablePubkeyAuth = to.Ptr(false)
			},
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				assert.Equal(t, "false", vars["DISABLE_PUBKEY_AUTH"])
			},
		},
		{
			name:       "AKSUbuntu2204 DISABLE_PUBKEY_AUTH with default (nil) pubkey auth",
			folder:     "AKSUbuntu2204+DefaultPubkeyAuth",
			k8sVersion: "1.24.2",
			aksNodeConfigUpdator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				// DisablePubkeyAuth is nil by default, which should result in "false"
			},
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				assert.Equal(t, "false", vars["DISABLE_PUBKEY_AUTH"])
			},
		},
		{
			name:       "AKSUbuntu2204 in China",
			folder:     "AKSUbuntu2204+China",
			k8sVersion: "1.24.2",
			aksNodeConfigUpdator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				aksNodeConfig.ClusterConfig.Location = "chinaeast2"
				aksNodeConfig.CustomCloudConfig.CustomCloudEnvName = "AzureChinaCloud"
			},
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				assert.Equal(t, "AzureChinaCloud", vars["TARGET_ENVIRONMENT"])
				assert.Equal(t, "AzureChinaCloud", vars["TARGET_CLOUD"])
				assert.Equal(t, "false", vars["IS_CUSTOM_CLOUD"])
			},
		},
		{
			name:       "AKSUbuntu2204 with custom cloud",
			folder:     "AKSUbuntu2204+CustomCloud",
			k8sVersion: "1.24.2",
			aksNodeConfigUpdator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				aksNodeConfig.CustomCloudConfig.CustomCloudEnvName = helpers.AksCustomCloudName
			},
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				assert.Equal(t, helpers.AksCustomCloudName, vars["TARGET_ENVIRONMENT"])
				assert.Equal(t, helpers.AzureStackCloud, vars["TARGET_CLOUD"])
				assert.Equal(t, "true", vars["IS_CUSTOM_CLOUD"])
			},
		},
		{
			name:       "AKSUbuntu2204 with custom osConfig",
			folder:     "AKSUbuntu2204+CustomOSConfig",
			k8sVersion: "1.24.2",
			aksNodeConfigUpdator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				aksNodeConfig.CustomLinuxOsConfig = &aksnodeconfigv1.CustomLinuxOsConfig{
					EnableSwapConfig:           true,
					SwapFileSize:               int32(1500),
					TransparentHugepageSupport: "never",
					TransparentDefrag:          "defer+madvise",
					SysctlConfig: &aksnodeconfigv1.SysctlConfig{
						NetCoreSomaxconn:             to.Ptr[int32](1638499),
						NetCoreRmemDefault:           to.Ptr[int32](456000),
						NetCoreWmemDefault:           to.Ptr[int32](89000),
						NetIpv4TcpTwReuse:            to.Ptr(true),
						NetIpv4IpLocalPortRange:      to.Ptr("32768 65400"),
						NetIpv4TcpMaxSynBacklog:      to.Ptr[int32](1638498),
						NetIpv4NeighDefaultGcThresh1: to.Ptr[int32](10001),
					},
				}
			},
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				sysctlContent, err := getBase64DecodedValue([]byte(vars["SYSCTL_CONTENT"]))
				require.NoError(t, err)
				assert.Contains(t, sysctlContent, "net.core.somaxconn=1638499")
				assert.Contains(t, sysctlContent, "net.ipv4.tcp_max_syn_backlog=1638498")
				assert.Contains(t, sysctlContent, "net.ipv4.neigh.default.gc_thresh1=10001")
				assert.Contains(t, sysctlContent, "net.ipv4.neigh.default.gc_thresh2=8192")
				assert.Contains(t, sysctlContent, "net.ipv4.neigh.default.gc_thresh3=16384")
				assert.Contains(t, sysctlContent, "net.ipv4.ip_local_reserved_ports=65330")
			},
		},
		{
			name:       "AzureLinux v2 with kata and DisableUnattendedUpgrades=false",
			folder:     "AzureLinuxv2+Kata+DisableUnattendedUpgrades=false",
			k8sVersion: "1.28.0",
			aksNodeConfigUpdator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				aksNodeConfig.IsKata = true
				aksNodeConfig.EnableUnattendedUpgrade = true
				aksNodeConfig.NeedsCgroupv2 = to.Ptr(true)
			},
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				assert.Equal(t, "true", vars["IS_KATA"])
				assert.Equal(t, "true", vars["ENABLE_UNATTENDED_UPGRADES"])
				assert.Equal(t, "true", vars["NEEDS_CGROUPV2"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &datamodel.ContainerService{
				Location: "southcentralus",
				Type:     "Microsoft.ContainerService/ManagedClusters",
				Properties: &datamodel.Properties{
					OrchestratorProfile: &datamodel.OrchestratorProfile{
						OrchestratorType:    datamodel.Kubernetes,
						OrchestratorVersion: tt.k8sVersion,
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
							Distro:              datamodel.AKSUbuntuContainerd2404,
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
				KeyData: "testsshkey",
			}}

			agentPool := cs.Properties.AgentPoolProfiles[0]

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
				"--container-log-max-size":            "50M",
				"--register-with-taints":              "testkey1=value1:NoSchedule,testkey2=value2:NoSchedule",
			}

			helpers.ValidateAndSetLinuxKubeletFlags(kubeletConfig, cs, agentPool)
			aksNodeConfig := &aksnodeconfigv1.Configuration{
				LinuxAdminUsername: "azureuser",
				VmSize:             "Standard_DS1_v2",
				ClusterConfig: &aksnodeconfigv1.ClusterConfig{
					Location:      "southcentralus",
					ResourceGroup: "resourceGroupName",
					VmType:        aksnodeconfigv1.VmType_VM_TYPE_VMSS,
					ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{
						SecurityGroupName: "aks-agentpool-36873793-nsg",
						VnetName:          "aks-vnet-07752737",
						VnetResourceGroup: "MC_rg",
						Subnet:            "subnet1",
						RouteTable:        "aks-agentpool-36873793-routetable",
					},
					PrimaryScaleSet: "aks-agent2-36873793-vmss",
				},
				AuthConfig: &aksnodeconfigv1.AuthConfig{
					ServicePrincipalId:     "ClientID",
					ServicePrincipalSecret: "Secret",
					TenantId:               "tenantID",
					SubscriptionId:         "subID",
					AssignedIdentityId:     "userAssignedID",
				},
				NetworkConfig: &aksnodeconfigv1.NetworkConfig{
					VnetCniPluginsUrl: "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-linux-amd64-v1.1.3.tgz",
				},
				GpuConfig: &aksnodeconfigv1.GpuConfig{
					ConfigGpuDriver: true,
					GpuDevicePlugin: false,
				},
				EnableUnattendedUpgrade: true,
				KubernetesVersion:       tt.k8sVersion,
				ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{
					ContainerdDownloadUrlBase: "https://storage.googleapis.com/cri-containerd-release/",
				},
				OutboundCommand: helpers.GetDefaultOutboundCommand(),
				KubeletConfig: &aksnodeconfigv1.KubeletConfig{
					EnableKubeletConfigFile: false,
					KubeletFlags:            helpers.GetKubeletConfigFlag(kubeletConfig, cs, agentPool, false),
					KubeletNodeLabels:       helpers.GetKubeletNodeLabels(agentPool),
				},
				CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{},
			}

			if tt.aksNodeConfigUpdator != nil {
				tt.aksNodeConfigUpdator(aksNodeConfig)
			}

			cseCMD, err := BuildCSECmd(context.TODO(), aksNodeConfig)
			require.NoError(t, err)

			generateTestDataIfRequested(t, tt.folder, cseCMD)

			if tt.validator != nil {
				tt.validator(cseCMD)
			}
		})
	}
}

func TestAKSNodeConfigCompatibilityFromJsonToCSECommand(t *testing.T) {
	tests := []struct {
		name      string
		folder    string
		validator func(cmd *exec.Cmd)
	}{
		{
			name:   "with empty config. Parser Should provide default values to unset fields.",
			folder: "Compatibility+EmptyConfig",
			validator: func(cmd *exec.Cmd) {
				vars := environToMap(cmd.Env)
				sysctlContent, err := getBase64DecodedValue([]byte(vars["SYSCTL_CONTENT"]))
				require.NoError(t, err)
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.tcp_retries2=%v", 8))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.core.message_burst=%v", 80))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.core.message_cost=%v", 40))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.core.somaxconn=%v", 16384))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.tcp_max_syn_backlog=%v", 16384))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.neigh.default.gc_thresh1=%v", 4096))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.neigh.default.gc_thresh2=%v", 8192))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.neigh.default.gc_thresh3=%v", 16384))
				assert.Equal(t, "false", vars["IS_KATA"])
				assert.Equal(t, "false", vars["ENABLE_UNATTENDED_UPGRADES"])
				assert.Equal(t, "false", vars["NEEDS_CGROUPV2"])
				assert.Equal(t, "azureuser", vars["ADMINUSER"])
				assert.Equal(t, "0", vars["SWAP_FILE_SIZE_MB"])
				assert.Equal(t, "false", vars["SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE"])
				assert.Equal(t, "", vars["THP_ENABLED"])
				assert.Equal(t, "", vars["THP_DEFRAG"])
				assert.Equal(t, "false", vars["DISABLE_SSH"])
				assert.Equal(t, "true", vars["IS_VHD"])
				assert.Equal(t, "", vars["MOBY_VERSION"])
				assert.Equal(t, "", vars["LOAD_BALANCER_SKU"])
				assert.Equal(t, "", vars["NETWORK_POLICY"])
				assert.Equal(t, "", vars["NETWORK_PLUGIN"])
				assert.Equal(t, "", vars["VNET_CNI_PLUGINS_URL"])
				assert.Equal(t, "false", vars["GPU_NODE"])
				assert.Equal(t, "", vars["GPU_INSTANCE_PROFILE"])
				assert.Equal(t, "0", vars["CUSTOM_CA_TRUST_COUNT"])
				assert.Equal(t, "false", vars["SHOULD_CONFIGURE_CUSTOM_CA_TRUST"])
				assert.Equal(t, "", vars["KUBELET_FLAGS"])
				assert.Equal(t, "", vars["KUBELET_NODE_LABELS"])
				assert.Equal(t, "", vars["HTTP_PROXY"])
				assert.Equal(t, "", vars["HTTPS_PROXY"])
				assert.Equal(t, "", vars["NO_PROXY"])
				assert.Equal(t, "", vars["PROXY_TRUSTED_CA"])
				assert.Equal(t, helpers.DefaultCloudName, vars["TARGET_ENVIRONMENT"])
				assert.Equal(t, "", vars["TLS_BOOTSTRAP_TOKEN"])
				assert.Equal(t, "false", vars["ENABLE_SECURE_TLS_BOOTSTRAPPING"])
				assert.Equal(t, "", vars["SECURE_TLS_BOOTSTRAPPING_DEADLINE"])
				assert.Equal(t, "", vars["SECURE_TLS_BOOTSTRAPPING_AAD_RESOURCE"])
				assert.Equal(t, "", vars["SECURE_TLS_BOOTSTRAPPING_USER_ASSIGNED_IDENTITY_ID"])
				assert.Equal(t, "", vars["CUSTOM_SECURE_TLS_BOOTSTRAPPING_CLIENT_URL"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cseCMD, err := BuildCSECmd(context.TODO(), &aksnodeconfigv1.Configuration{})
			require.NoError(t, err)

			generateTestDataIfRequested(t, tt.folder, cseCMD)

			if tt.validator != nil {
				tt.validator(cseCMD)
			}
		})
	}
}

func environToMap(env []string) map[string]string {
	envMap := make(map[string]string)
	for _, e := range env {
		kv := strings.SplitN(e, "=", 2)
		if len(kv) == 2 {
			envMap[kv[0]] = kv[1]
		}
	}
	return envMap
}

func TestContractCompatibilityHandledByProtobuf(t *testing.T) {
	t.Run("with unexpected new fields in json should be ignored", func(t *testing.T) {
		// The unexpected fields will natively be ignored when unmarshalling the json to the contract object.
		// We use this test to ensure it.
		assert.Equal(t,
			loadAKSNodeConfig("./testdata/test_aksnodeconfig.json"),
			loadAKSNodeConfig("./testdata/test_aksnodeconfig_fields_unexpected.json"),
		)
	})

	t.Run("with missing fields in json should be set with default values", func(t *testing.T) {
		aksNodeConfigUT := loadAKSNodeConfig("./testdata/test_aksnodeconfig_fields_missing.json")
		assert.Equal(t, "", aksNodeConfigUT.GetLinuxAdminUsername())

		// if an optional (explicit presence) bool field is unset, it will be set to nil by protobuf by default.
		// Here we don't use the getter because getter is nil safe and will default to false.
		assert.Nil(t, aksNodeConfigUT.IsVhd)

		// if an optional (explicit presence) field is unset, it will be set to nil by protobuf by default.
		// Here we don't use the getter because getter is nil safe and will default to false.
		assert.Nil(t, aksNodeConfigUT.ClusterConfig.LoadBalancerConfig.ExcludeMasterFromStandardLoadBalancer)

		// if an optional enum field is unset, it will be set to 0 (in this case LoadBalancerConfig_UNSPECIFIED) by protobuf by default.
		assert.Equal(t, aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_UNSPECIFIED, aksNodeConfigUT.ClusterConfig.LoadBalancerConfig.GetLoadBalancerSku())
	})

	t.Run("marshal/unmarshal", func(t *testing.T) {
		content, err := os.ReadFile("./testdata/test_aksnodeconfig.json")
		require.NoError(t, err)
		cfg, err := nodeconfigutils.UnmarshalConfigurationV1(content)
		require.NoError(t, err)
		marshalled, err := nodeconfigutils.MarshalConfigurationV1(cfg)
		require.NoError(t, err)
		assert.JSONEq(t, string(content), string(marshalled))
	})
}

func getBase64DecodedValue(data []byte) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func loadAKSNodeConfig(jsonFilePath string) *aksnodeconfigv1.Configuration {
	content, err := os.ReadFile(jsonFilePath)
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := nodeconfigutils.UnmarshalConfigurationV1(content)
	if err != nil {
		log.Printf("Failed to unmarshal the aksnodeconfigv1 from json: %v", err)
	}
	return cfg
}

func generateTestDataIfRequested(t *testing.T, folder string, cmd *exec.Cmd) {
	if os.Getenv("GENERATE_TEST_DATA") == "true" {
		if _, err := os.Stat(fmt.Sprintf("./testdata/%s", folder)); os.IsNotExist(err) {
			e := os.MkdirAll(fmt.Sprintf("./testdata/%s", folder), 0755)
			assert.NoError(t, e)
		}
		err := os.WriteFile(fmt.Sprintf("./testdata/%s/generatedCSECommand", folder), []byte(cmd.String()), 0644)
		assert.NoError(t, err)
	}
}
