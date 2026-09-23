package scenario

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestFullyManagedGPUNetworkIsolatedConfiguration(t *testing.T) {
	s := newUbuntu2404FullyManagedGPUNetworkIsolatedScenario()
	require.Equal(t, "Ubuntu2404_FullyManagedGPU_NetworkIsolated", s.Name)
	require.True(t, s.Tags.GPU)
	require.True(t, s.Tags.NetworkIsolated)
	require.True(t, s.Tags.NonAnonymousACR)
	require.Same(t, config.VHDUbuntu2404Gen2Containerd, s.VHD)
	require.NotNil(t, s.Cluster)
	require.NotNil(t, s.Validator)
	require.False(t, s.SkipDefaultValidation)
	require.Empty(t, s.SkipReason)
	require.Nil(t, s.SkipIf)
	count := 0
	for _, registered := range List() {
		if registered.Name == s.Name {
			count++
		}
	}
	require.Equal(t, 1, count, "scenario must be discoverable by the E2E runner")

	for _, location := range []string{"westus2", "westus3"} {
		t.Run(location, func(t *testing.T) {
			cluster := &Cluster{Model: &armcontainerservice.ManagedCluster{Location: to.Ptr(location)}}
			nbc := &datamodel.NodeBootstrappingConfiguration{
				ContainerService: &datamodel.ContainerService{Properties: &datamodel.Properties{
					OrchestratorProfile: &datamodel.OrchestratorProfile{
						OrchestratorVersion: "1.34.8",
						KubernetesConfig:    &datamodel.KubernetesConfig{},
					},
				}},
				AgentPoolProfile: &datamodel.AgentPoolProfile{KubernetesConfig: &datamodel.KubernetesConfig{}},
				K8sComponents:    &datamodel.K8sComponents{},
				KubeletConfig:    map[string]string{},
			}
			s.BootstrapConfigMutator(cluster, nbc)
			vmss := &armcompute.VirtualMachineScaleSet{SKU: &armcompute.SKU{}}
			s.VMConfigMutator(vmss)
			require.Equal(t, "Standard_NC4as_T4_v3", nbc.AgentPoolProfile.VMSize)
			require.Equal(t, nbc.AgentPoolProfile.VMSize, *vmss.SKU.Name)
			require.NotContains(t, vmss.Tags, "EnableManagedGPUExperience")
			require.True(t, nbc.EnableManagedGPU)
			require.True(t, nbc.ConfigGPUDriverIfNeeded)
			require.True(t, nbc.EnableGPUDevicePluginIfNeeded)
			require.True(t, nbc.EnableNvidia)
			require.True(t, nbc.ManagedGPUExperienceAFECEnabled)
			require.Equal(t, datamodel.OutboundTypeBlock, nbc.OutboundType)
			privateEgress := nbc.ContainerService.Properties.SecurityProfile.PrivateEgress
			require.True(t, privateEgress.Enabled)
			require.Equal(t, config.PrivateACRNameNotAnon(location)+".azurecr.io/aks-managed-repository", privateEgress.ContainerRegistryServer)
			require.True(t, nbc.AgentPoolProfile.KubernetesConfig.UseManagedIdentity)
			require.True(t, nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity)
			require.Equal(t, "/var/lib/kubelet/credential-provider-config.yaml", nbc.KubeletConfig["--image-credential-provider-config"])
			require.Equal(t, "/var/lib/kubelet/credential-provider", nbc.KubeletConfig["--image-credential-provider-bin-dir"])
			require.Contains(t, nbc.K8sComponents.LinuxCredentialProviderURL, "/v1.34.8/binaries/")
		})
	}
}

func TestCreateVMExtensionLinuxAKSNodeTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	require.NoError(t, config.LoadDotEnv())
	command := &cli.Command{
		Name:  "e2e-integration-test",
		Flags: config.Flags(),
		Action: func(context.Context, *cli.Command) error {
			return config.Initialize()
		},
	}
	require.NoError(t, command.Run(t.Context(), []string{command.Name}))

	start := time.Now()
	first, err := createVMExtensionLinuxAKSNode(t.Context(), nil)
	firstDuration := time.Since(start)
	require.NoError(t, err)
	require.NotNil(t, first)

	start = time.Now()
	second, err := createVMExtensionLinuxAKSNode(t.Context(), nil)
	secondDuration := time.Since(start)
	require.NoError(t, err)
	require.NotNil(t, second)

	require.NotNil(t, first.Properties)
	require.NotNil(t, second.Properties)
	require.NotNil(t, first.Properties.TypeHandlerVersion)
	require.NotNil(t, second.Properties.TypeHandlerVersion)
	require.NotEmpty(t, *first.Properties.TypeHandlerVersion)
	require.NotEqual(t, "1.413", *first.Properties.TypeHandlerVersion, "extension version is the hardcoded fallback")
	require.Equal(t, *first.Properties.TypeHandlerVersion, *second.Properties.TypeHandlerVersion)
	t.Logf("first call: %s; cached call: %s", firstDuration, secondDuration)
}

func TestVersionConsistencyGPUManagedComponents(t *testing.T) {
	allPackageVariants := [][]packageOSVariant{
		{
			{"nvidia-device-plugin", "ubuntu", "r2404"},
			{"nvidia-device-plugin", "ubuntu", "r2204"},
			{"nvidia-device-plugin", "azurelinux", "v3.0"},
		},
		{
			{"datacenter-gpu-manager-4-core", "ubuntu", "r2404"},
			{"datacenter-gpu-manager-4-core", "ubuntu", "r2204"},
			{"datacenter-gpu-manager-4-core", "azurelinux", "v3.0"},
		},
		{
			{"datacenter-gpu-manager-4-proprietary", "ubuntu", "r2404"},
			{"datacenter-gpu-manager-4-proprietary", "ubuntu", "r2204"},
			{"datacenter-gpu-manager-4-proprietary", "azurelinux", "v3.0"},
		},
		{
			{"dcgm-exporter", "ubuntu", "r2404"},
			{"dcgm-exporter", "ubuntu", "r2204"},
			{"dcgm-exporter", "azurelinux", "v3.0"},
		},
	}

	for _, packageGroup := range allPackageVariants {
		expectedVersion := ""
		for _, pkgVar := range packageGroup {
			componentVersions := components.GetExpectedPackageVersions(pkgVar.pkgName, pkgVar.osName, pkgVar.osRelease)
			require.Lenf(t, componentVersions, 1,
				"Expected exactly one %s version for %s %s but got %d",
				pkgVar.pkgName, pkgVar.osName, pkgVar.osRelease, len(componentVersions))

			pkgVersion := extractMajorMinorPatchVersion(componentVersions[0])
			require.NotEmptyf(t, pkgVersion, "Failed to extract major.minor.patch version from %s for %s %s",
				componentVersions[0], pkgVar.osName, pkgVar.osRelease)

			if expectedVersion == "" {
				expectedVersion = pkgVersion
				continue
			}

			require.Equalf(t, expectedVersion, pkgVersion,
				"Expected all %s versions to have the same major.minor.patch version, but found mismatch: %s vs %s for %s.%s",
				pkgVar.pkgName, expectedVersion, pkgVersion, pkgVar.osName, pkgVar.osRelease)
		}
	}
}
