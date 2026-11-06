package scenario

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/require"
)

func nodeConfigTestCluster(version string) *Cluster {
	return &Cluster{
		Model: &armcontainerservice.ManagedCluster{
			ID:       to.Ptr("/subscriptions/test-subscription/resourceGroups/test-cluster-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster"),
			Location: to.Ptr("westus2"),
			Properties: &armcontainerservice.ManagedClusterProperties{
				CurrentKubernetesVersion: to.Ptr(version),
				NodeResourceGroup:        to.Ptr("test-resource-group"),
			},
		},
		KubeletIdentity:  &armcontainerservice.UserAssignedIdentity{ClientID: to.Ptr("test-identity")},
		TenantID:         "test-tenant",
		VNetResourceGUID: "test-vnet",
		ClusterParams: &ClusterParams{
			FQDN:           "test-api.example.com",
			CACert:         []byte("test-ca"),
			BootstrapToken: "test-bootstrap-token",
		},
	}
}

func TestCreateAKSNodeConfigUsesClusterAndImageInputs(t *testing.T) {
	for _, version := range []string{"1.33.12", "1.36.5"} {
		for _, arch := range []string{"amd64", "arm64"} {
			t.Run(version+"/"+arch, func(t *testing.T) {
				cluster := nodeConfigTestCluster(version)
				vhd := *config.VHDUbuntu2404Gen2Containerd
				vhd.Arch = arch
				cfg, err := createAKSNodeConfig(cluster, &vhd)
				require.NoError(t, err)
				require.Equal(t, version, cfg.KubernetesVersion)
				require.Equal(t, *cluster.Model.Location, cfg.ClusterConfig.Location)
				require.Equal(t, *cluster.Model.Properties.NodeResourceGroup, cfg.ClusterConfig.ResourceGroup)
				require.Equal(t, cluster.TenantID, cfg.AuthConfig.TenantId)
				require.Equal(t, config.Config.SubscriptionID, cfg.AuthConfig.SubscriptionId)
				require.Equal(t, *cluster.KubeletIdentity.ClientID, cfg.AuthConfig.AssignedIdentityId)
				require.Equal(t, cluster.ClusterParams.FQDN, cfg.ApiServerConfig.ApiServerName)
				require.Equal(t, base64.StdEncoding.EncodeToString(cluster.ClusterParams.CACert), cfg.KubernetesCaCert)
				require.Equal(t, cluster.ClusterParams.BootstrapToken, cfg.BootstrappingConfig.GetTlsBootstrappingToken())
				require.Contains(t, cfg.NetworkConfig.VnetCniPluginsUrl, "linux-"+arch)
				if version == "1.33.12" {
					require.Contains(t, cfg.KubeBinaryConfig.KubeBinaryUrl, "linux-"+arch)
					require.Contains(t, cfg.KubeProxyUrl, version)
				} else {
					require.Empty(t, cfg.KubeBinaryConfig.KubeBinaryUrl)
					require.Empty(t, cfg.KubeProxyUrl)
				}
				cluster.ClusterParams.BootstrapToken = "changed"
				require.NotEqual(t, "changed", cfg.BootstrappingConfig.GetTlsBootstrappingToken())
			})
		}
	}
	for _, secureTLS := range []bool{false, true} {
		t.Run("secureTLS="+strconv.FormatBool(secureTLS), func(t *testing.T) {
			original := config.Config.EnableSecureTLSBootstrapping
			config.Config.EnableSecureTLSBootstrapping = secureTLS
			t.Cleanup(func() { config.Config.EnableSecureTLSBootstrapping = original })
			cfg, err := createAKSNodeConfig(nodeConfigTestCluster("1.36.5"), config.VHDAzureLinuxV3Gen2Kata)
			require.NoError(t, err)
			require.True(t, cfg.IsKata)
			require.Equal(t, secureTLS, cfg.BootstrappingConfig.BootstrappingAuthMethod ==
				aksnodeconfigv1.BootstrappingAuthMethod_BOOTSTRAPPING_AUTH_METHOD_SECURE_TLS_BOOTSTRAPPING)
		})
	}
}

func TestCreateAKSNodeConfigOptionsAndIsolation(t *testing.T) {
	cluster := nodeConfigTestCluster("1.36.5")
	first, err := createAKSNodeConfig(cluster, config.VHDUbuntu2404Gen2Containerd,
		func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) error {
			cfg.KubeletConfig.KubeletDiskType = aksnodeconfigv1.KubeletDisk_KUBELET_DISK_TEMP_DISK
			cfg.KubeletConfig.KubeletNodeLabels["test"] = "first"
			cfg.LocalDnsProfile.VnetDnsOverrides["."].QueryLogging = "Error"
			return nil
		})
	require.NoError(t, err)
	require.Equal(t, datamodel.TempDiskContainerDataDir, first.KubeletConfig.ContainerDataDir)
	second, err := createAKSNodeConfig(cluster, config.VHDUbuntu2404Gen2Containerd)
	require.NoError(t, err)
	require.Empty(t, second.KubeletConfig.ContainerDataDir)
	require.NotContains(t, second.KubeletConfig.KubeletNodeLabels, "test")
	require.Equal(t, "Log", second.LocalDnsProfile.VnetDnsOverrides["."].QueryLogging)
	failure := errors.New("option failed")
	_, err = createAKSNodeConfig(cluster, config.VHDUbuntu2404Gen2Containerd,
		func(_ *Cluster, _ *aksnodeconfigv1.Configuration) error { return failure })
	require.ErrorIs(t, err, failure)
}

func TestCreateAKSNodeConfigRejectsIncompleteInputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Cluster, *config.Image)
	}{
		{"missing model", func(c *Cluster, _ *config.Image) { c.Model = nil }},
		{"missing properties", func(c *Cluster, _ *config.Image) { c.Model.Properties = nil }},
		{"missing identity", func(c *Cluster, _ *config.Image) { c.KubeletIdentity = nil }},
		{"missing connection data", func(c *Cluster, _ *config.Image) { c.ClusterParams = nil }},
		{"invalid version", func(c *Cluster, _ *config.Image) { c.Model.Properties.CurrentKubernetesVersion = to.Ptr("invalid") }},
		{"Windows", func(_ *Cluster, v *config.Image) { v.Distro = config.VHDWindows2022ContainerdGen2.Distro }},
		{"unsupported architecture", func(_ *Cluster, v *config.Image) { v.Arch = "invalid" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cluster := nodeConfigTestCluster("1.36.5")
			vhd := *config.VHDUbuntu2404Gen2Containerd
			tc.change(cluster, &vhd)
			_, err := createAKSNodeConfig(cluster, &vhd)
			require.Error(t, err)
		})
	}
	_, err := createAKSNodeConfig(nil, config.VHDUbuntu2404Gen2Containerd)
	require.Error(t, err)
	_, err = createAKSNodeConfig(nodeConfigTestCluster("1.36.5"), nil)
	require.Error(t, err)
}

func TestPrepareAKSNodeConfigDoesNotReadNBCInputs(t *testing.T) {
	original := config.Config.DisableScriptless
	config.Config.DisableScriptless = false
	t.Cleanup(func() { config.Config.DisableScriptless = original })
	cluster := nodeConfigTestCluster("1.36.5")
	nbc, err := getBaseNBC(t.Context(), cluster, config.VHDUbuntu2404Gen2Containerd)
	require.NoError(t, err)
	s := &Scenario{
		Config: Config{
			VHD: config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.HostedMasterProfile.FQDN = "legacy-api"
				nbc.KubeletClientTLSBootstrapToken = to.Ptr("legacy-token")
				nbc.KubeletConfig["--max-pods"] = "17"
			},
			AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
				cfg.KubeletConfig.KubeletConfigFileConfig.MaxPods = to.Ptr(int32(73))
			},
		},
		Runtime: &ScenarioRuntime{Cluster: cluster},
	}
	require.NoError(t, prepareNodeConfiguration(t.Context(), s, nbc))
	require.Equal(t, cluster.ClusterParams.FQDN, s.Runtime.AKSNodeConfig.ApiServerConfig.ApiServerName)
	require.Equal(t, cluster.ClusterParams.BootstrapToken, s.Runtime.AKSNodeConfig.BootstrappingConfig.GetTlsBootstrappingToken())
	require.Equal(t, int32(73), s.Runtime.AKSNodeConfig.KubeletConfig.KubeletConfigFileConfig.GetMaxPods())
	require.Equal(t, "17", nbc.KubeletConfig["--max-pods"])
}

func TestPrepareAKSNodeConfigErrorMutator(t *testing.T) {
	original := config.Config.DisableScriptless
	config.Config.DisableScriptless = false
	t.Cleanup(func() { config.Config.DisableScriptless = original })
	cluster := nodeConfigTestCluster("1.36.5")
	s := &Scenario{
		Config: Config{
			VHD:                    config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(_ *Cluster, _ *datamodel.NodeBootstrappingConfiguration) {},
			AKSNodeConfigMutatorWithError: func(_ context.Context, _ *Cluster, cfg *aksnodeconfigv1.Configuration) error {
				cfg.KubernetesVersion = "1.33.12"
				return nil
			},
		},
		Runtime: &ScenarioRuntime{Cluster: cluster},
	}
	nbc, err := getBaseNBC(t.Context(), cluster, s.VHD)
	require.NoError(t, err)
	require.NoError(t, prepareNodeConfiguration(t.Context(), s, nbc))
	require.True(t, nbc.EnableScriptlessAKSNodeConfig)
	require.Equal(t, "1.33.12", s.Runtime.AKSNodeConfig.KubernetesVersion)
	failure := errors.New("cannot resolve version")
	s.AKSNodeConfigMutatorWithError = func(_ context.Context, _ *Cluster, _ *aksnodeconfigv1.Configuration) error { return failure }
	nbc, err = getBaseNBC(t.Context(), cluster, s.VHD)
	require.NoError(t, err)
	require.ErrorIs(t, prepareNodeConfiguration(t.Context(), s, nbc), failure)
}
