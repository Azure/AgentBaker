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
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2019-containerd")
				ValidateWindowsVersionDisplayVersion(ctx, s, "2019")
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
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd")
				ValidateWindowsVersionDisplayVersion(ctx, s, "2022")

			},
		},
	})
}

func Test_Windows2022ContainerdGen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd - hyperv gen 2",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2022ContainerdGen2,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd-gen2")
				ValidateWindowsVersionDisplayVersion(ctx, s, "2022")
			},
		},
	})
}

func Test_Windows23H2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows23H2,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "23H2")
				ValidateWindowsVersionDisplayVersion(ctx, s, "23H2")
			},
		},
	})
}

func Test_Windows23H2Gen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Containerd - hyperv gen2",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows23H2Gen2,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "23H2-gen2")
				ValidateWindowsVersionDisplayVersion(ctx, s, "23H2")
			},
		},
	})
}

// TODO: enable this test once production AKS supports Cilium Windows
/*
func Test_Windows23H2_Cilium2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows23H2Gen2,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
				// cilium is only supported in 1.30 or greater.
				configuration.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.30.9"
				configuration.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.EbpfDataplane = datamodel.EbpfDataplane_cilium
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsRunningWindows(ctx, s)
			},
		},
	})
}
*/
