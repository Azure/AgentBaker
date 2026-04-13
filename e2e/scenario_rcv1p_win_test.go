// scenario_rcv1p_win_test.go contains end-to-end tests for the RCV1P cert mode on Windows.
// Windows uses a different cert installation path than Linux: certificates are downloaded to
// C:\ca and imported into the Windows certificate store (Cert:\LocalMachine\Root) via
// Import-Certificate. A scheduled task (aks-ca-certs-refresh-task) is registered to
// periodically refresh the certificates.
//
// These tests run against the same RCV1P subscription and require the same VM opt-in tag
// as the Linux tests (see scenario_rcv1p_test.go for details on the two-layer access control).
package e2e

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

// Test_RCV1P_Windows2022 validates RCV1P cert download and Windows certificate store
// installation on Windows Server 2022.
func Test_RCV1P_Windows2022(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on Windows Server 2022 with VM opt-in tag",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:                ClusterRCV1PKubenet,
			VHD:                    config.VHDWindows2022Containerd,
			VMConfigMutator:        rcv1pOptInVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PCertModeWindows(ctx, s)
			},
		},
	})
}

// Test_RCV1P_Windows23H2 validates RCV1P on Windows Server 23H2, the annual channel release.
func Test_RCV1P_Windows23H2(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on Windows Server 23H2 with VM opt-in tag",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:                ClusterRCV1PKubenet,
			VHD:                    config.VHDWindows23H2,
			VMConfigMutator:        rcv1pOptInVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PCertModeWindows(ctx, s)
			},
		},
	})
}

// Test_RCV1P_Windows2025 validates RCV1P on Windows Server 2025. This SKU requires
// Trusted Launch, so the VMConfigMutator combines both TrustedLaunch and opt-in tag settings.
func Test_RCV1P_Windows2025(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on Windows Server 2025 with VM opt-in tag",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster: ClusterRCV1PKubenet,
			VHD:     config.VHDWindows2025,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
				rcv1pOptInVMConfigMutator(vmss)
			},
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				Windows2025BootstrapConfigMutator(t, nbc)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PCertModeWindows(ctx, s)
			},
		},
	})
}
