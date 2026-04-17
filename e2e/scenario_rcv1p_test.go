// scenario_rcv1p_test.go contains end-to-end tests for the RCV1P (Root Certificate V1P) cert mode
// on Linux distros. RCV1P is the next-generation mechanism for distributing Azure root CA certificates
// to AKS nodes. Instead of relying on hardcoded certificate bundles, RCV1P queries the Azure wireserver
// at provisioning time to download the latest root certificates and installs them into the OS trust store.
//
// These tests require:
//   - A dedicated subscription (RCV1P_SUBSCRIPTION_ID) with the Microsoft.Compute/PlatformSettingsOverride
//     feature flag registered, which enables the wireserver certificate endpoint.
//   - The VM opt-in tag "platformsettings.host_environment.service.platform_optedin_for_rootcerts=true"
//     on each VMSS, which tells wireserver to serve certificates to this specific VM.
//
// Both conditions must be met: the subscription feature enables the endpoint, and the VM tag grants
// per-VM access. Without the tag, wireserver returns IsOptedInForRootCerts=false.
//
// The positive tests (Test_RCV1P_<Distro>) verify that certificates are downloaded, installed into
// the distro-specific trust store, and a refresh schedule is created. The negative test
// (Test_RCV1P_NotOptedIn) verifies that omitting the VM tag correctly prevents cert installation.
package e2e

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	azruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

// rcv1pOptInTag is the ARM tag that must be set on the VM resource for wireserver to serve
// root certificates. Without this tag, wireserver returns IsOptedInForRootCerts=false even
// if the subscription has the PlatformSettingsOverride feature registered.
const rcv1pOptInTag = "platformsettings.host_environment.service.platform_optedin_for_rootcerts"

// skipIfRCV1PNotConfigured skips the test when the RCV1P subscription is not configured.
// This happens in regular CI runs where the RCV1P variable group is not linked, causing
// Azure DevOps to pass the literal unexpanded string "$(RCV1P_SUBSCRIPTION_ID)".
// It also verifies the Microsoft.Compute/PlatformSettingsOverride feature flag is registered.
func skipIfRCV1PNotConfigured(t *testing.T) {
	t.Helper()
	subID := config.Config.RCV1PSubscriptionID
	if subID == "" || strings.HasPrefix(subID, "$(") {
		t.Skip("RCV1P_SUBSCRIPTION_ID not set or not resolved, skipping RCV1P cert mode test")
	}
	checkPlatformSettingsOverrideFeatureFlag(t, subID)
}

var (
	featureFlagCheckOnce   sync.Once
	featureFlagCheckResult error
)

// checkPlatformSettingsOverrideFeatureFlag verifies the Microsoft.Compute/PlatformSettingsOverride
// feature flag is registered on the given subscription. This is a prerequisite for wireserver to
// serve root certificates. The check runs only once per test run.
func checkPlatformSettingsOverrideFeatureFlag(t *testing.T, subscriptionID string) {
	t.Helper()
	featureFlagCheckOnce.Do(func() {
		featureFlagCheckResult = verifyFeatureFlag(t.Context(), subscriptionID)
	})
	if featureFlagCheckResult != nil {
		t.Fatalf("RCV1P feature flag check failed: %v", featureFlagCheckResult)
	}
}

func verifyFeatureFlag(ctx context.Context, subscriptionID string) error {
	url := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.Features/providers/Microsoft.Compute/features/PlatformSettingsOverride?api-version=2021-07-01",
		subscriptionID,
	)

	req, err := azruntime.NewRequest(ctx, "GET", url)
	if err != nil {
		return fmt.Errorf("failed to create feature flag request: %w", err)
	}

	resp, err := config.RCV1PAzure.Core.Pipeline().Do(req)
	if err != nil {
		return fmt.Errorf("failed to query feature flag: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("feature flag query returned status %d: %s", resp.StatusCode, bodyStr)
	}

	if !strings.Contains(bodyStr, `"Registered"`) {
		return fmt.Errorf("Microsoft.Compute/PlatformSettingsOverride is NOT registered on subscription %s (response: %s); "+
			"wireserver will not serve root certificates without this feature flag", subscriptionID, bodyStr)
	}

	return nil
}

// rcv1pOptInVMConfigMutator sets the platform opt-in tag on the VMSS resource level.
// Note: For wireserver to recognize the tag, it must also be set on the individual VM instance.
// Use VMInstanceTags in the Config to set instance-level tags (applied after VM creation).
func rcv1pOptInVMConfigMutator(vmss *armcompute.VirtualMachineScaleSet) {
	if vmss.Tags == nil {
		vmss.Tags = map[string]*string{}
	}
	vmss.Tags[rcv1pOptInTag] = to.Ptr("true")
}

// rcv1pVMInstanceTags returns the tags that must be set on individual VM instances
// for wireserver to serve root certificates.
func rcv1pVMInstanceTags() map[string]*string {
	return map[string]*string{
		rcv1pOptInTag: to.Ptr("true"),
	}
}

// Test_RCV1P_Ubuntu2204 validates RCV1P cert download and trust store installation on Ubuntu 22.04.
// Ubuntu uses /usr/local/share/ca-certificates/ as the cert drop folder and update-ca-certificates
// to rebuild the trust bundle.
func Test_RCV1P_Ubuntu2204(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on Ubuntu 22.04 with VM opt-in tag",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:         ClusterRCV1PKubenet,
			VHD:             config.VHDUbuntu2204Gen2Containerd,
			VMConfigMutator: rcv1pOptInVMConfigMutator,
			VMInstanceTags:  rcv1pVMInstanceTags(),
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PCertMode(ctx, s)
			},
		},
	})
}

// Test_RCV1P_Ubuntu2404 validates RCV1P cert download and trust store installation on Ubuntu 24.04.
// Covers the newer Ubuntu LTS release to ensure the cert endpoint and trust store integration
// work correctly across Ubuntu versions.
func Test_RCV1P_Ubuntu2404(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on Ubuntu 24.04 with VM opt-in tag",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:         ClusterRCV1PKubenet,
			VHD:             config.VHDUbuntu2404Gen2Containerd,
			VMConfigMutator: rcv1pOptInVMConfigMutator,
			VMInstanceTags:  rcv1pVMInstanceTags(),
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PCertMode(ctx, s)
			},
		},
	})
}

// Test_RCV1P_AzureLinuxV3 validates RCV1P on Azure Linux V3, which uses a different trust store
// layout (/etc/pki/ca-trust/source/anchors/) and update command (update-ca-trust) than Ubuntu.
// This ensures the provisioning script correctly detects the distro and uses the right paths.
func Test_RCV1P_AzureLinuxV3(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on Azure Linux V3 with VM opt-in tag",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:         ClusterRCV1PKubenet,
			VHD:             config.VHDAzureLinuxV3Gen2,
			VMConfigMutator: rcv1pOptInVMConfigMutator,
			VMInstanceTags:  rcv1pVMInstanceTags(),
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PCertMode(ctx, s)
			},
		},
	})
}

// Test_RCV1P_Flatcar validates RCV1P on Flatcar Container Linux, which has a read-only root
// filesystem and requires certificates to be placed in /etc/ssl/certs/ as .pem files.
// This is the most constrained environment for cert installation.
func Test_RCV1P_Flatcar(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on Flatcar with VM opt-in tag",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster: ClusterRCV1PKubenet,
			VHD:     config.VHDFlatcarGen2,
			VMConfigMutator: rcv1pOptInVMConfigMutator,
			VMInstanceTags:  rcv1pVMInstanceTags(),
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PCertMode(ctx, s)
			},
		},
	})
}

// Test_RCV1P_ACL validates RCV1P on Azure Container Linux (ACL), which shares the same
// trust store layout as Azure Linux (/etc/pki/ca-trust/). ACL requires Trusted Launch,
// so the VMConfigMutator combines both the TrustedLaunch and opt-in tag settings.
func Test_RCV1P_ACL(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on ACL with VM opt-in tag",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster: ClusterRCV1PKubenet,
			VHD:     config.VHDACLGen2TL,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
				rcv1pOptInVMConfigMutator(vmss)
			},
			VMInstanceTags: rcv1pVMInstanceTags(),
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PCertMode(ctx, s)
			},
		},
	})
}

// Test_RCV1P_NotOptedIn is a negative test that validates the VM opt-in tag is required
// for cert installation. The VM is created in the RCV1P subscription (which has
// PlatformSettingsOverride registered) but WITHOUT the opt-in tag on the VMSS.
// This verifies that wireserver returns IsOptedInForRootCerts=false and the provisioning
// script correctly skips certificate download and trust store installation.
// This test is critical because it proves the two-layer access control works:
// subscription feature alone is not sufficient — the VM must also be explicitly tagged.
func Test_RCV1P_NotOptedIn(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode without VM opt-in tag; expects no cert installation",
		AzureClient:    config.RCV1PAzure,
		SubscriptionID: config.Config.RCV1PSubscriptionID,
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster: ClusterRCV1PKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PNotOptedIn(ctx, s)
			},
		},
	})
}
