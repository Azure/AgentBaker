package e2e

import (
	"context"
	"fmt"
	"strings"
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
				ValidateFileHasContentWindows(ctx, s, "/k/config", "--rotate-server-certificates=true")
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
		},
	})
}

func makeExecutablePowershellCommand(steps []string) string {
	stepsWithEchos := make([]string, len(steps)*2)

	for i, s := range steps {
		stepsWithEchos[i*2] = fmt.Sprintf("echo '%s'", cleanse(s))
		stepsWithEchos[i*2+1] = s
	}

	// quote " quotes and $ vars
	joinedCommand := strings.Join(steps, " & ")
	quotedCommand := strings.Replace(joinedCommand, "'", "'\"'\"'", -1)

	command := fmt.Sprintf("powershell -c '%s'", quotedCommand)

	return command
}

func ValidateFileHasContentWindows(ctx context.Context, s *Scenario, fileName string, contents string) {
	steps := []string{
		fmt.Sprintf("dir %[1]s", fileName),
		fmt.Sprintf("Get-Content %[1]s", fileName),
		fmt.Sprintf("if (Select-String -Path %s -Pattern \"%s\" -SimpleMatch -Quiet) { return 1 } else { return 0 }", fileName, contents),
	}

	command := makeExecutablePowershellCommand(steps)
	execOnVMForScenarioValidateExitCode(ctx, s, command, 0, "could not validate file has contents - might mean file does not have contents, might mean something went wrong")
	//podExecResult :=
	//require.Contains(s.T, podExecResult.stdout.String(), contents)
}
