// scenario_rcv1p_test.go contains end-to-end tests for the RCV1P (Root Certificate V1P) cert mode
// on Linux distros. RCV1P is the next-generation mechanism for distributing Azure root CA certificates
// to AKS nodes. Instead of relying on hardcoded certificate bundles, RCV1P queries the Azure wireserver
// at provisioning time to download the latest root certificates and installs them into the OS trust store.
//
// RCV1P requires two conditions on a subscription:
//   - The Microsoft.Compute/PlatformSettingsOverride feature flag must be registered.
//   - The VMSS must have the tag "platformsettings.host_environment.service.platform_optedin_for_rootcerts=true".
//     On subscriptions with the feature flag, the platform may auto-inject this tag on all VMSSes.
//
// RCV1P_SUBSCRIPTION_ID is optional. When set, a dedicated subscription is used and the tests
// explicitly control VMSS tagging (enabling positive and negative test scenarios). When not set,
// the default E2E subscription is used if it has the feature flag registered — in this case the
// platform auto-injects the opt-in tag, so only positive tests can run.
//
// The positive tests (Test_RCV1P_<Distro>) verify that certificates are downloaded, installed into
// the distro-specific trust store, and a refresh schedule is created. The negative tests
// (Test_RCV1P_NotOptedIn) require RCV1P_SUBSCRIPTION_ID to explicitly control tagging.
package e2e

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

// skipIfRCV1PNotConfigured skips the test when RCV1P is not available on the effective subscription.
// It always checks and logs the feature flag status. When RCV1P_SUBSCRIPTION_ID is set, that
// subscription is used with explicit tag control. When not set, the default E2E subscription
// is used if it has the PlatformSettingsOverride feature flag registered.
func skipIfRCV1PNotConfigured(t *testing.T) {
	t.Helper()
	if hasExplicitRCV1PSubscription() {
		// Explicit RCV1P subscription — verify its feature flag and use it
		checkPlatformSettingsOverrideFeatureFlag(t, config.Config.RCV1PSubscriptionID, config.RCV1PAzure, true)
		// Also log the default E2E subscription flag for diagnostics
		logE2ESubscriptionFeatureFlag(t)
		t.Logf("RCV1P mode: explicit subscription %s (we control VMSS tagging)", config.Config.RCV1PSubscriptionID)
		return
	}
	// No explicit RCV1P subscription — check if default E2E subscription supports RCV1P
	registered := logE2ESubscriptionFeatureFlag(t)
	if !registered {
		t.Skip("RCV1P_SUBSCRIPTION_ID not set and E2E subscription does not have PlatformSettingsOverride registered, skipping RCV1P test")
	}
	t.Logf("RCV1P mode: auto-detected on default E2E subscription %s (platform auto-injects opt-in tag, we do NOT set it explicitly)", config.Config.SubscriptionID)
}

// skipIfRCV1PNotExplicit skips the test when RCV1P_SUBSCRIPTION_ID is not explicitly provided.
// Used for negative tests that require explicit control over VMSS tagging (the platform may
// auto-inject the opt-in tag on subscriptions with the feature flag, making negative tests invalid).
func skipIfRCV1PNotExplicit(t *testing.T) {
	t.Helper()
	logE2ESubscriptionFeatureFlag(t)
	if !hasExplicitRCV1PSubscription() {
		t.Skip("RCV1P_SUBSCRIPTION_ID not set, skipping negative RCV1P test (platform may auto-inject opt-in tag on default subscription)")
	}
	checkPlatformSettingsOverrideFeatureFlag(t, config.Config.RCV1PSubscriptionID, config.RCV1PAzure, true)
	t.Logf("RCV1P negative test mode: explicit subscription %s (we control VMSS tagging — opt-in tag intentionally NOT set)", config.Config.RCV1PSubscriptionID)
}

// hasExplicitRCV1PSubscription returns true when RCV1P_SUBSCRIPTION_ID is set to a real value.
func hasExplicitRCV1PSubscription() bool {
	subID := strings.TrimSpace(config.Config.RCV1PSubscriptionID)
	return subID != "" && !strings.HasPrefix(subID, "$(")
}

// rcv1pAzureClient returns the Azure client for RCV1P tests. When RCV1P_SUBSCRIPTION_ID is set,
// returns config.RCV1PAzure. Otherwise returns nil (falls back to default via Scenario.GetAzure).
func rcv1pAzureClient() *config.AzureClient {
	if hasExplicitRCV1PSubscription() {
		return config.RCV1PAzure
	}
	return nil
}

// rcv1pSubscriptionID returns the subscription ID for RCV1P tests. When RCV1P_SUBSCRIPTION_ID
// is set, returns it. Otherwise returns "" (falls back to default via Scenario.GetSubscriptionID).
func rcv1pSubscriptionID() string {
	if hasExplicitRCV1PSubscription() {
		return strings.TrimSpace(config.Config.RCV1PSubscriptionID)
	}
	return ""
}

// rcv1pCluster returns the cluster function for Linux RCV1P tests. When RCV1P_SUBSCRIPTION_ID
// is set, uses a dedicated kubenet cluster in the RCV1P subscription. Otherwise uses the default
// kubenet cluster (Linux tests don't have IP exhaustion issues on kubenet).
func rcv1pCluster() func(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	if hasExplicitRCV1PSubscription() {
		return ClusterRCV1PKubenet
	}
	return ClusterKubenet
}

// rcv1pWindowsCluster returns the cluster function for Windows RCV1P tests. Windows tests must
// use Azure CNI (not kubenet) because baseTemplateWindows() configures the NBC for Azure CNI
// overlay mode — using kubenet causes azure-vnet plugin IP allocation failures. This matches
// the cluster type used by all other non-RCV1P Windows tests (ClusterAzureNetwork).
func rcv1pWindowsCluster() func(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	return ClusterAzureNetwork
}

var (
	featureFlagChecks sync.Map // subscriptionID -> *featureFlagResult
)

type featureFlagResult struct {
	once       sync.Once
	registered bool
	err        error
}

// checkPlatformSettingsOverrideFeatureFlag checks the Microsoft.Compute/PlatformSettingsOverride
// feature flag on the given subscription. When failIfMissing is true (RCV1P tests), the test
// fails if the flag is not registered. When false (diagnostics), it only logs the result.
// Returns true if the flag is registered.
func checkPlatformSettingsOverrideFeatureFlag(t *testing.T, subscriptionID string, client *config.AzureClient, failIfMissing bool) bool {
	t.Helper()
	val, _ := featureFlagChecks.LoadOrStore(subscriptionID, &featureFlagResult{})
	result := val.(*featureFlagResult)
	result.once.Do(func() {
		result.registered, result.err = queryFeatureFlag(t.Context(), subscriptionID, client)
	})

	if result.err != nil {
		t.Logf("PlatformSettingsOverride feature flag check on subscription %s: error: %v", subscriptionID, result.err)
		if failIfMissing {
			t.Fatalf("RCV1P feature flag check failed: %v", result.err)
		}
		return false
	}

	t.Logf("PlatformSettingsOverride feature flag on subscription %s: registered=%v", subscriptionID, result.registered)
	if failIfMissing && !result.registered {
		t.Fatalf("Microsoft.Compute/PlatformSettingsOverride is NOT registered on subscription %s; "+
			"wireserver will not serve root certificates without this feature flag", subscriptionID)
	}
	return result.registered
}

// logE2ESubscriptionFeatureFlag logs the PlatformSettingsOverride feature flag status on the
// default E2E subscription for diagnostic purposes. Returns true if the flag is registered.
func logE2ESubscriptionFeatureFlag(t *testing.T) bool {
	t.Helper()
	e2eAzure, err := config.NewAzureClient()
	if err != nil {
		t.Logf("WARNING: failed to create E2E Azure client for feature flag check: %v", err)
		return false
	}
	registered := checkPlatformSettingsOverrideFeatureFlag(t, config.Config.SubscriptionID, e2eAzure, false)
	return registered
}

func queryFeatureFlag(ctx context.Context, subscriptionID string, client *config.AzureClient) (bool, error) {
	url := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.Features/providers/Microsoft.Compute/features/PlatformSettingsOverride?api-version=2021-07-01",
		subscriptionID,
	)

	req, err := azruntime.NewRequest(ctx, "GET", url)
	if err != nil {
		return false, fmt.Errorf("failed to create feature flag request: %w", err)
	}

	resp, err := client.Core.Pipeline().Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to query feature flag: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if resp.StatusCode != 200 {
		return false, fmt.Errorf("feature flag query returned status %d: %s", resp.StatusCode, bodyStr)
	}

	return strings.Contains(bodyStr, `"Registered"`), nil
}

// rcv1pVMConfigMutator returns the VMConfigMutator for RCV1P positive tests.
// When RCV1P_SUBSCRIPTION_ID is set, we explicitly set the opt-in tag to control tagging.
// When using auto-detection, the platform auto-injects the tag, so no mutator is needed.
func rcv1pVMConfigMutator() func(*armcompute.VirtualMachineScaleSet) {
	if hasExplicitRCV1PSubscription() {
		return rcv1pOptInVMConfigMutator
	}
	return nil
}

// REVERT ME: build and upload a CSE zip from the branch's staging/cse/windows/ so that
// Windows RCV1P E2E tests exercise the actual RCV1P code instead of the published v0.0.52 package.
var (
	branchCSEZipURL  string
	branchCSEZipErr  error
	branchCSEZipOnce sync.Once
)

// getOrBuildBranchCSEPackageURL builds a CSE zip from staging/cse/windows/ (matching the
// pipeline packaging in .pipelines/scripts/windows_package_cse.sh) and uploads it to the
// E2E blob storage. Returns a SAS-signed URL. Uses sync.Once so the zip is built and
// uploaded exactly once across all parallel tests.
func getOrBuildBranchCSEPackageURL(t *testing.T) string {
	t.Helper()
	branchCSEZipOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		branchCSEZipURL, branchCSEZipErr = buildAndUploadCSEZip(ctx)
	})
	if branchCSEZipErr != nil {
		t.Fatalf("failed to build/upload branch CSE zip: %v", branchCSEZipErr)
	}
	t.Logf("using branch CSE package URL: %s", branchCSEZipURL)
	return branchCSEZipURL
}

func buildAndUploadCSEZip(ctx context.Context) (string, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return "", fmt.Errorf("find repo root: %w", err)
	}
	cseDir := filepath.Join(repoRoot, "staging", "cse", "windows")

	tmpFile, err := os.CreateTemp("", "aks-windows-cse-scripts-branch-*.zip")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)
	err = filepath.Walk(cseDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(cseDir, path)
		if err != nil {
			return err
		}
		// normalize to forward slashes for zip
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		// skip test files and debug helper (matches windows_package_cse.sh)
		if strings.HasSuffix(rel, ".tests.ps1") || strings.Contains(rel, ".tests.suites") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "README" || rel == "debug/update-scripts.ps1" {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		w, err := zw.Create(rel)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", rel, err)
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("build zip: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("close zip writer: %w", err)
	}

	// seek to start for upload
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek temp file: %w", err)
	}

	blobName := fmt.Sprintf("cse-packages/aks-windows-cse-scripts-branch-%s.zip",
		time.Now().UTC().Format("20060102-150405"))
	url, err := config.Azure.UploadAndGetSignedLink(ctx, blobName, tmpFile)
	if err != nil {
		return "", fmt.Errorf("upload CSE zip: %w", err)
	}
	return url, nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// e2e/ has its own go.mod, go up one more
			if filepath.Base(dir) == "e2e" {
				dir = filepath.Dir(dir)
				continue
			}
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repo root (go.mod) from %s", dir)
		}
		dir = parent
	}
}

// rcv1pWindowsCSEMutator returns a BootstrapConfigMutator that overrides CseScriptsPackageURL
// to use the branch-built CSE zip containing the RCV1P code.
func rcv1pWindowsCSEMutator(t *testing.T) func(*datamodel.NodeBootstrappingConfiguration) {
	cseURL := getOrBuildBranchCSEPackageURL(t)
	return func(nbc *datamodel.NodeBootstrappingConfiguration) {
		nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = cseURL
	}
}

// rcv1pOptInVMConfigMutator sets the platform opt-in tag on the VMSS resource level.
// VMSS resource-level tags are automatically inherited by VM instances at creation time,
// which allows wireserver to recognize the tag and serve root certificates.
func rcv1pOptInVMConfigMutator(vmss *armcompute.VirtualMachineScaleSet) {
	if vmss.Tags == nil {
		vmss.Tags = map[string]*string{}
	}
	vmss.Tags[rcv1pOptInTag] = to.Ptr("true")
}

// Test_RCV1P_Ubuntu2204 validates RCV1P cert download and trust store installation on Ubuntu 22.04.
// Ubuntu uses /usr/local/share/ca-certificates/ as the cert drop folder and update-ca-certificates
// to rebuild the trust bundle.
func Test_RCV1P_Ubuntu2204(t *testing.T) {
	skipIfRCV1PNotConfigured(t)
	RunScenario(t, &Scenario{
		Description:    "Tests RCV1P cert mode on Ubuntu 22.04 with VM opt-in tag",
		AzureClient:    rcv1pAzureClient(),
		SubscriptionID: rcv1pSubscriptionID(),
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:         rcv1pCluster(),
			VHD:             config.VHDUbuntu2204Gen2Containerd,
			VMConfigMutator: rcv1pVMConfigMutator(),
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
		AzureClient:    rcv1pAzureClient(),
		SubscriptionID: rcv1pSubscriptionID(),
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:         rcv1pCluster(),
			VHD:             config.VHDUbuntu2404Gen2Containerd,
			VMConfigMutator: rcv1pVMConfigMutator(),
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
		AzureClient:    rcv1pAzureClient(),
		SubscriptionID: rcv1pSubscriptionID(),
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster:         rcv1pCluster(),
			VHD:             config.VHDAzureLinuxV3Gen2,
			VMConfigMutator: rcv1pVMConfigMutator(),
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
		AzureClient:    rcv1pAzureClient(),
		SubscriptionID: rcv1pSubscriptionID(),
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster: rcv1pCluster(),
			VHD:     config.VHDFlatcarGen2,
			VMConfigMutator: rcv1pVMConfigMutator(),
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
		AzureClient:    rcv1pAzureClient(),
		SubscriptionID: rcv1pSubscriptionID(),
		Tags: Tags{
			RCV1PCertMode: true,
		},
		Config: Config{
			Cluster: rcv1pCluster(),
			VHD:     config.VHDACLGen2TL,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
				if m := rcv1pVMConfigMutator(); m != nil {
					m(vmss)
				}
			},
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
// This test requires RCV1P_SUBSCRIPTION_ID because the platform may auto-inject the opt-in
// tag on the default E2E subscription, making the negative test invalid.
func Test_RCV1P_NotOptedIn(t *testing.T) {
	skipIfRCV1PNotExplicit(t)
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
