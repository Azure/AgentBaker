package scenario

import (
	"context"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestNC96A100ManagedGPUScenario(t *testing.T) {
	var matches []*Scenario
	for _, s := range List() {
		if s.Name == "Ubuntu2404_NvidiaDevicePluginRunning_NC96A100" {
			matches = append(matches, s)
		}
	}
	require.Len(t, matches, 1)
	s := matches[0]
	require.Equal(t, "westus2", s.Location)
	require.Equal(t, config.VHDUbuntu2404Gen2Containerd, s.VHD)
	require.True(t, s.Tags.GPU)
	require.Empty(t, s.SkipReason)
	require.False(t, s.SkipDefaultValidation)
	require.NotNil(t, s.Validator)
	require.NotNil(t, s.VMConfigMutatorWithError)
	require.Nil(t, s.Runtime)

	const vmSize = "Standard_NC96ads_A100_v4"
	nbc := &datamodel.NodeBootstrappingConfiguration{AgentPoolProfile: &datamodel.AgentPoolProfile{}}
	s.BootstrapConfigMutator(nil, nbc)
	require.Equal(t, vmSize, nbc.AgentPoolProfile.VMSize)
	require.True(t, nbc.ConfigGPUDriverIfNeeded)
	require.True(t, nbc.EnableGPUDevicePluginIfNeeded)
	require.True(t, nbc.EnableNvidia)
	require.True(t, nbc.ManagedGPUExperienceAFECEnabled)
	require.True(t, nbc.EnableManagedGPU)
	require.Empty(t, nbc.GPUInstanceProfile)
	require.Empty(t, nbc.MIGProfileLayout)

	cfg := &aksnodeconfigv1.Configuration{GpuConfig: &aksnodeconfigv1.GpuConfig{}}
	s.AKSNodeConfigMutator(nil, cfg)
	require.Equal(t, vmSize, cfg.VmSize)
	require.True(t, cfg.GpuConfig.ConfigGpuDriver)
	require.True(t, cfg.GpuConfig.GpuDevicePlugin)
	require.True(t, cfg.GpuConfig.GetEnableNvidia())
	require.True(t, cfg.GpuConfig.ManagedGpuExperienceAfecEnabled)
	require.True(t, cfg.GpuConfig.EnableManagedGpu)
	require.Empty(t, cfg.GpuConfig.GpuInstanceProfile)
	require.Empty(t, cfg.GpuConfig.MigProfileLayout)

	vmss := &armcompute.VirtualMachineScaleSet{SKU: &armcompute.SKU{}}
	s.VMConfigMutator(vmss)
	require.Equal(t, vmSize, *vmss.SKU.Name)
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
