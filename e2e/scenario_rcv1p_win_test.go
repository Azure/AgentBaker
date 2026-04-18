// scenario_rcv1p_win_test.go contains end-to-end tests for the RCV1P cert mode on Windows.
// Windows uses a different cert installation path than Linux: certificates are downloaded to
// C:\ca and imported into the Windows certificate store (Cert:\LocalMachine\Root) via
// Import-Certificate. A scheduled task (aks-ca-certs-refresh-task) is registered to
// periodically refresh the certificates.
//
// These tests share the same gating logic as the Linux tests (see scenario_rcv1p_test.go):
// RCV1P_SUBSCRIPTION_ID is optional. When set, a dedicated subscription controls tagging.
// When not set, the default E2E subscription is used if it has the feature flag.
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
		AzureClient:    rcv1pAzureClient(),
		SubscriptionID: rcv1pSubscriptionID(),
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:                rcv1pCluster(),
			VHD:                    config.VHDWindows2022Containerd,
			VMConfigMutator:        rcv1pVMConfigMutator(),
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
		AzureClient:    rcv1pAzureClient(),
		SubscriptionID: rcv1pSubscriptionID(),
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:                rcv1pCluster(),
			VHD:                    config.VHDWindows23H2,
			VMConfigMutator:        rcv1pVMConfigMutator(),
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
		AzureClient:    rcv1pAzureClient(),
		SubscriptionID: rcv1pSubscriptionID(),
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster: rcv1pCluster(),
			VHD:     config.VHDWindows2025,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
				if m := rcv1pVMConfigMutator(); m != nil {
					m(vmss)
				}
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

// Test_RCV1P_Windows_NotOptedIn is a negative test that validates the VM opt-in tag is required
// for cert installation on Windows. The VM is created in the RCV1P subscription (which has
// PlatformSettingsOverride registered) but WITHOUT the opt-in tag on the VMSS.
// This verifies that wireserver returns IsOptedInForRootCerts=false and the provisioning
// script correctly skips certificate download and refresh task registration.
// This test requires RCV1P_SUBSCRIPTION_ID because the platform may auto-inject the opt-in
// tag on the default E2E subscription, making the negative test invalid.
func Test_RCV1P_Windows_NotOptedIn(t *testing.T) {
	skipIfRCV1PNotExplicit(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on Windows without VM opt-in tag; expects no cert installation",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:                ClusterRCV1PKubenet,
			VHD:                    config.VHDWindows2022Containerd,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PNotOptedInWindows(ctx, s)
			},
		},
	})
}
