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

// skipIfRCV1PNotConfigured skips the test when no subscription with the RCV1P feature flag
// is available. It checks in order:
//  1. Explicit RCV1P_SUBSCRIPTION_ID (dedicated RCV1P subscription)
//  2. E2E_SUBSCRIPTION_ID auto-detection (checks if the feature flag is registered)
//
// When E2E_SUBSCRIPTION_ID has the feature flag registered (e.g., MSFT tenant pipelines),
// the RCV1P tests run automatically without needing a separate variable.
func skipIfRCV1PNotConfigured(t *testing.T) {
	t.Helper()

	subID := strings.TrimSpace(config.Config.RCV1PSubscriptionID)
	if subID != "" && !strings.HasPrefix(subID, "$(") {
		// Explicit RCV1P subscription configured — verify it has the feature flag
		checkPlatformSettingsOverrideFeatureFlag(t, subID, config.RCV1PAzure, true)
		return
	}

	// No explicit RCV1P subscription — try auto-detecting from the E2E subscription
	t.Log("RCV1P_SUBSCRIPTION_ID not set, checking if E2E subscription has PlatformSettingsOverride feature flag...")
	e2eSubID := strings.TrimSpace(config.Config.SubscriptionID)
	if e2eSubID == "" {
		t.Skip("neither RCV1P_SUBSCRIPTION_ID nor E2E_SUBSCRIPTION_ID is set, skipping RCV1P test")
	}

	e2eAzure, err := config.NewAzureClient()
	if err != nil {
		t.Skipf("failed to create E2E Azure client for feature flag auto-detection: %v", err)
	}

	registered, err := queryFeatureFlag(t.Context(), e2eSubID, e2eAzure)
	if err != nil {
		t.Skipf("failed to query feature flag on E2E subscription %s: %v", e2eSubID, err)
	}
	if !registered {
		t.Skipf("E2E subscription %s does not have PlatformSettingsOverride registered, skipping RCV1P test", e2eSubID)
	}

	// E2E subscription is enrolled — configure RCV1P globals so the rest of the test infra works
	t.Logf("auto-detected PlatformSettingsOverride on E2E subscription %s, using it for RCV1P tests", e2eSubID)
	rcv1pAutoDetectOnce.Do(func() {
		config.Config.RCV1PSubscriptionID = e2eSubID
		config.RCV1PAzure = e2eAzure
		rcv1pAutoDetected = true
	})
}

var (
	rcv1pAutoDetectOnce sync.Once
	// rcv1pAutoDetected is true when the RCV1P subscription was auto-detected from the
	// E2E subscription rather than explicitly set via RCV1P_SUBSCRIPTION_ID. On auto-detected
	// (enrolled) subscriptions, the platform auto-injects the opt-in tag on ALL VMSSes,
	// making "not opted in" negative tests impossible.
	rcv1pAutoDetected bool
)

// skipNotOptedInOnAutoDetect skips NotOptedIn negative tests when the RCV1P subscription was
// auto-detected. On enrolled subscriptions, the platform auto-injects the opt-in tag on ALL
// VMSSes, making it impossible to test the "not opted in" scenario.
func skipNotOptedInOnAutoDetect(t *testing.T) {
	t.Helper()
	if rcv1pAutoDetected {
		t.Skip("skipping NotOptedIn test: RCV1P subscription was auto-detected from E2E subscription — " +
			"platform auto-injects opt-in tag on all VMSSes in enrolled subscriptions")
	}
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
func checkPlatformSettingsOverrideFeatureFlag(t *testing.T, subscriptionID string, client *config.AzureClient, failIfMissing bool) {
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
		return
	}

	t.Logf("PlatformSettingsOverride feature flag on subscription %s: registered=%v", subscriptionID, result.registered)
	if failIfMissing && !result.registered {
		t.Fatalf("Microsoft.Compute/PlatformSettingsOverride is NOT registered on subscription %s; "+
			"wireserver will not serve root certificates without this feature flag", subscriptionID)
	}
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

// TODO(rcv1p): remove the branch CSE zip override once the RCV1P code ships in a published
// CSE package on packages.aks.azure.com. Until then, Windows E2E tests would exercise the
// old Get-CACertificates (without -Location, -FailOnError, or IsOptedInForRootCerts) from
// the released aks-windows-cse-scripts-current.zip instead of the PR's version.
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
// TODO(rcv1p): remove this once the RCV1P code ships in a published CSE package.
func rcv1pWindowsCSEMutator(t *testing.T) func(*Cluster, *datamodel.NodeBootstrappingConfiguration) {
	cseURL := getOrBuildBranchCSEPackageURL(t)
	return func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
		nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = cseURL
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
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
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
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
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
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
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
			Cluster:         ClusterRCV1PKubenet,
			VHD:             config.VHDFlatcarGen2,
			VMConfigMutator: rcv1pOptInVMConfigMutator,
			VMInstanceTags:  rcv1pVMInstanceTags(),
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
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
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
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
	skipNotOptedInOnAutoDetect(t)
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
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateRCV1PNotOptedIn(ctx, s)
			},
		},
	})
}
