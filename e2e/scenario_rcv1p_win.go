// REVERT ME: this file uses rcv1pWindowsCSEMutator to override CseScriptsPackageURL with a
// branch-built CSE zip. Remove those overrides once the RCV1P code ships in a published CSE package.
//
// This file contains end-to-end scenarios for the RCV1P cert mode on Windows.
// Windows uses a different cert installation path than Linux: certificates are downloaded to
// C:\ca and imported into the Windows root or intermediate LocalMachine certificate store.
// A scheduled task (aks-ca-certs-refresh-task) is registered to
// periodically refresh the certificates.
package e2e

import (
	"context"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func rcv1pWindowsBootstrapMutator(windows2025 bool) func(context.Context, *Cluster, *datamodel.NodeBootstrappingConfiguration) error {
	return func(_ context.Context, cluster *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) error {
		cseMutator, err := rcv1pWindowsCSEMutator()
		if err != nil {
			return err
		}
		cseMutator(cluster, nbc)
		if windows2025 {
			return Windows2025BootstrapConfigMutator(nbc)
		}
		return nil
	}
}

// RCV1P_Windows2022 validates RCV1P cert download and Windows certificate store
// installation on Windows Server 2022.
var _ = Register(&Scenario{
	Name:        "RCV1P_Windows2022",
	SkipIf:      skipIfRCV1PNotConfigured,
	Description: "Tests RCV1P cert mode on Windows Server 2022 with VM opt-in tag",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster:                         ClusterAzureNetwork,
		VHD:                             config.VHDWindows2022Containerd,
		VMConfigMutator:                 rcv1pVMConfigMutator(),
		BootstrapConfigMutatorWithError: rcv1pWindowsBootstrapMutator(false),
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PCertModeWindows(ctx, s)
		},
	},
})

// RCV1P_Windows2025 validates RCV1P on Windows Server 2025 (non-gen2).
var _ = Register(&Scenario{
	Name:        "RCV1P_Windows2025",
	SkipIf:      skipIfRCV1PNotConfigured,
	Description: "Tests RCV1P cert mode on Windows Server 2025 with VM opt-in tag",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster:                         ClusterAzureNetwork,
		VHD:                             config.VHDWindows2025,
		VMConfigMutator:                 rcv1pVMConfigMutator(),
		BootstrapConfigMutatorWithError: rcv1pWindowsBootstrapMutator(true),
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PCertModeWindows(ctx, s)
		},
	},
})

// RCV1P_Windows2022Gen2 validates RCV1P cert download and Windows certificate store
// installation on Windows Server 2022 Gen2. Covers the gen2 pipeline job.
var _ = Register(&Scenario{
	Name:        "RCV1P_Windows2022Gen2",
	SkipIf:      skipIfRCV1PNotConfigured,
	Description: "Tests RCV1P cert mode on Windows Server 2022 Gen2 with VM opt-in tag",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster:                         ClusterAzureNetwork,
		VHD:                             config.VHDWindows2022ContainerdGen2,
		VMConfigMutator:                 rcv1pVMConfigMutator(),
		BootstrapConfigMutatorWithError: rcv1pWindowsBootstrapMutator(false),
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PCertModeWindows(ctx, s)
		},
	},
})

// RCV1P_Windows2025Gen2 validates RCV1P on Windows Server 2025 Gen2. Covers the gen2 pipeline job.
var _ = Register(&Scenario{
	Name:        "RCV1P_Windows2025Gen2",
	SkipIf:      skipIfRCV1PNotConfigured,
	Description: "Tests RCV1P cert mode on Windows Server 2025 Gen2 with VM opt-in tag",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster:                         ClusterAzureNetwork,
		VHD:                             config.VHDWindows2025Gen2,
		VMConfigMutator:                 rcv1pVMConfigMutator(),
		BootstrapConfigMutatorWithError: rcv1pWindowsBootstrapMutator(true),
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PCertModeWindows(ctx, s)
		},
	},
})

// RCV1P_Windows_NotOptedIn is a negative test that validates the VM opt-in tag is required
// for cert installation on Windows. The VM is created in the RCV1P subscription (which has
// PlatformSettingsOverride registered) but WITHOUT the opt-in tag on the VMSS.
// This verifies that wireserver returns IsOptedInForRootCerts=false and the provisioning
// script correctly skips certificate download and refresh task registration.
// This test requires RCV1P_TAGS_AUTO_INJECTED to not be true because the platform may auto-inject
// the opt-in tag on the default E2E subscription, making the negative test invalid.
var _ = Register(&Scenario{
	Name:        "RCV1P_Windows_NotOptedIn",
	SkipIf:      skipIfRCV1PNotExplicit,
	Description: "Tests RCV1P cert mode on Windows without VM opt-in tag; expects no cert installation",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster:                         ClusterAzureNetwork,
		VHD:                             config.VHDWindows2022Containerd,
		BootstrapConfigMutatorWithError: rcv1pWindowsBootstrapMutator(false),
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PNotOptedInWindows(ctx, s)
		},
	},
})
