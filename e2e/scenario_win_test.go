package e2e

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

func EmptyBootstrapConfigMutator(configuration *datamodel.NodeBootstrappingConfiguration) {}
func EmptyVMConfigMutator(vmss *armcompute.VirtualMachineScaleSet)                        {}

func Test_Windows2019Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2019 with Containerd",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2019Containerd,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2019-containerd")
				ValidateWindowsProductName(ctx, s, "Windows Server 2019 Datacenter")
				// TODO: currently the command used to get the display name returns an empty string on WS2019. Need to find a better command.
				//ValidateWindowsDisplayVersion(ctx, s, "???")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows2019Containerd_NewScripts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2019 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2019Containerd,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				WithNewScripts(context.Background(), t, nbc)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2019-containerd")
				ValidateWindowsProductName(ctx, s, "Windows Server 2019 Datacenter")
				// TODO: currently the command used to get the display name returns an empty string on WS2019. Need to find a better command.
				//ValidateWindowsDisplayVersion(ctx, s, "???")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows2022Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2022Containerd,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "21H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows2022Containerd_NewScripts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2022Containerd,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				WithNewScripts(context.Background(), t, nbc)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "21H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows2022ContainerdGen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd - hyperv gen 2",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2022ContainerdGen2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "21H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows23H2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Containerd",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows23H2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "23H2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "23H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows23H2Gen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Containerd - hyperv gen2",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows23H2Gen2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "23H2-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "23H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows23H2Gen2_NewScripts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Containerd - hyperv gen2",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows23H2Gen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				WithNewScripts(context.Background(), t, nbc)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "23H2-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "23H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows2025(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 with Containerd",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2025,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025")
				ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "24H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
			},
		},
	})
}

func Test_Windows2025Gen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 with Containerd - hyperv gen 2",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2025Gen2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "24H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
			},
		},
	})
}

func Test_Windows2025Gen2_NewScripts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 with Containerd - hyperv gen 2",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2025Gen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				WithNewScripts(context.Background(), t, nbc)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "24H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
			},
		},
	})
}

// TODO: enable this test once production AKS supports Cilium Windows
func Test_Windows23H2_Cilium2(t *testing.T) {
	t.Skip("skipping test for Cilium on Windows 23H2, as it is not supported in production AKS yet")
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster:         ClusterCiliumNetwork,
			VHD:             config.VHDWindows23H2Gen2,
			VMConfigMutator: EmptyVMConfigMutator,
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

// Windows scripts are released independently from AgentBaker, which can create version mismatches:
// - Old scripts may be used with new CSE/CustomData
// - New scripts may be used with old CSE/CustomData
//
// To maintain compatibility, ensure all changes work with both script versions.
// By default, tests use the latest released scripts. This helper overrides them with scripts
// from the current repository state for testing unreleased changes.
func WithNewScripts(ctx context.Context, t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
	nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = windowsScripts(ctx, t)
}
