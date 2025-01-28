package e2e

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

func Test_Windows2019Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2019 with Containerd",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2019Containerd,
			VMConfigMutator:        func(vmss *armcompute.VirtualMachineScaleSet) {},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
			},
		},
	})
}

func Test_Windows2022Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2022Containerd,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
			},
		},
	})
}

func Test_Windows2022ContainerdGen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2022ContainerdGen2,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
			},
		},
	})
}

func Test_Windows23H2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows23H2,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
			},
		},
	})
}

func Test_Windows23H2Gen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows23H2Gen2,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
			},
		},
	})
}
