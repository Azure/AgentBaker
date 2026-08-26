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
// RCV1P tests run against whichever subscription E2E_SUBSCRIPTION_ID points at; the RCV1P pipeline
// job overrides this to an RCV1P-registered subscription. Positive tests always run and verify
// cert installation. Negative tests are skipped when RCV1P_TAGS_AUTO_INJECTED=true (platform
// auto-injects the opt-in tag, making the "no tag" scenario impossible to reproduce).
package e2e

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// skipIfRCV1PNotConfigured verifies the current E2E subscription has PlatformSettingsOverride
// registered. The RCV1P pipeline job sets E2E_SUBSCRIPTION_ID to an RCV1P-registered subscription;
// on any other subscription the test is skipped.
func skipIfRCV1PNotConfigured(ctx context.Context) string {
	registered, err := getE2ESubscriptionFeatureFlag(ctx)
	if err != nil {
		return fmt.Sprintf("could not verify PlatformSettingsOverride feature flag on E2E subscription: %v", err)
	}
	if !registered {
		return "PlatformSettingsOverride feature flag is not registered on the E2E subscription"
	}
	return ""
}

// skipIfRCV1PNotExplicit skips the test when the platform may auto-inject the RCV1P opt-in tag,
// which invalidates negative tests. The pipeline sets RCV1P_TAGS_AUTO_INJECTED=true on
// subscriptions where the platform injects the opt-in tag automatically.
func skipIfRCV1PNotExplicit(ctx context.Context) string {
	if reason := skipIfRCV1PNotConfigured(ctx); reason != "" {
		return reason
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("RCV1P_TAGS_AUTO_INJECTED")), "true") {
		return "RCV1P_TAGS_AUTO_INJECTED=true; the platform injects the opt-in tag on this subscription"
	}
	return ""
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
// feature flag on the given subscription.
func checkPlatformSettingsOverrideFeatureFlag(ctx context.Context, subscriptionID string, client *config.AzureClient) (bool, error) {
	val, _ := featureFlagChecks.LoadOrStore(subscriptionID, &featureFlagResult{})
	result := val.(*featureFlagResult)
	result.once.Do(func() {
		result.registered, result.err = queryFeatureFlag(ctx, subscriptionID, client)
	})
	return result.registered, result.err
}

// getE2ESubscriptionFeatureFlag returns the PlatformSettingsOverride feature flag status on the
// default E2E subscription.
func getE2ESubscriptionFeatureFlag(ctx context.Context) (bool, error) {
	e2eAzure, err := config.NewAzureClient()
	if err != nil {
		return false, fmt.Errorf("create E2E Azure client for feature flag check: %w", err)
	}
	return checkPlatformSettingsOverrideFeatureFlag(ctx, config.Config.SubscriptionID, e2eAzure)
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

	var result struct {
		Properties struct {
			State string `json:"state"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("failed to parse feature flag response: %w", err)
	}

	return result.Properties.State == "Registered", nil
}

// rcv1pVMConfigMutator returns the VMConfigMutator for RCV1P positive tests. In the single-sub
// model, we always set the opt-in tag explicitly (RCV1P_TAGS_AUTO_INJECTED subscriptions will
// have both our tag and the platform's tag, which is idempotent).
func rcv1pVMConfigMutator() func(*armcompute.VirtualMachineScaleSet) {
	return rcv1pOptInVMConfigMutator
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
func getOrBuildBranchCSEPackageURL() (string, error) {
	branchCSEZipOnce.Do(func() {
		// 5m covers a cold-start storage account create (~30-90s) plus the zip build/upload.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		// Windows scenarios construct this mutator at scenario-struct-init time, which runs
		// BEFORE RunScenario -> CachedCreateVMManagedIdentity (which creates the per-sub blob
		// storage account on first use). On a brand-new region/subscription combo the storage
		// account does not exist yet, so the upload below fails with NXDOMAIN. Ensure storage
		// exists first by piggybacking on the same cached identity-creation path Linux tests use.
		//
		// CachedCreateVMManagedIdentity depends on the per-location resource group already
		// existing (runScenario ensures it later, too late for this init-time path). Ensure
		// the RG first so the identity/storage-account creation succeeds on a fresh sub/region.
		if _, err := CachedEnsureResourceGroup(ctx, config.Config.DefaultLocation); err != nil {
			branchCSEZipErr = fmt.Errorf("ensure shared resource group: %w", err)
			return
		}
		if _, err := CachedCreateVMManagedIdentity(ctx, config.Config.DefaultLocation); err != nil {
			branchCSEZipErr = fmt.Errorf("ensure shared storage account: %w", err)
			return
		}
		branchCSEZipURL, branchCSEZipErr = buildAndUploadCSEZip(ctx)
	})
	if branchCSEZipErr != nil {
		return "", fmt.Errorf("build or upload branch CSE zip: %w", branchCSEZipErr)
	}
	return branchCSEZipURL, nil
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
		_, copyErr := io.Copy(w, f)
		closeErr := f.Close()
		if copyErr != nil {
			return fmt.Errorf("copy %s: %w", path, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", path, closeErr)
		}
		return nil
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
func rcv1pWindowsCSEMutator() (func(*Cluster, *datamodel.NodeBootstrappingConfiguration), error) {
	cseURL, err := getOrBuildBranchCSEPackageURL()
	if err != nil {
		return nil, err
	}
	return func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
		nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = cseURL
	}, nil
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

// RCV1P_Ubuntu2204 validates RCV1P cert download and trust store installation on Ubuntu 22.04.
// Ubuntu uses /usr/local/share/ca-certificates/ as the cert drop folder and update-ca-certificates
// to rebuild the trust bundle.
var _ = Register(&Scenario{
	Name:        "RCV1P_Ubuntu2204",
	SkipIf:      skipIfRCV1PNotConfigured,
	Description: "Tests RCV1P cert mode on Ubuntu 22.04 with VM opt-in tag",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster:         ClusterKubenet,
		VHD:             config.VHDUbuntu2204Gen2Containerd,
		VMConfigMutator: rcv1pVMConfigMutator(),
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PCertMode(ctx, s)
		},
	},
})

// RCV1P_Ubuntu2604Minimal validates RCV1P cert download and trust store installation on Ubuntu 26.04 minimal.
// Covers the newer Ubuntu LTS release to ensure the cert endpoint and trust store integration
// work correctly across Ubuntu versions.
var _ = Register(&Scenario{
	Name:        "RCV1P_Ubuntu2604Minimal",
	SkipIf:      skipIfRCV1PNotConfigured,
	Description: "Tests RCV1P cert mode on Ubuntu 26.04 minimal with VM opt-in tag",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster:         ClusterLatestKubernetesVersionKubenet,
		VHD:             config.VHDUbuntu2604MinimalGen2Containerd,
		VMConfigMutator: rcv1pVMConfigMutator(),
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PCertMode(ctx, s)
		},
	},
})

// RCV1P_Ubuntu2404 validates RCV1P cert download and trust store installation on Ubuntu 24.04.
// Covers the newer Ubuntu LTS release to ensure the cert endpoint and trust store integration
// work correctly across Ubuntu versions.
var _ = Register(&Scenario{
	Name:        "RCV1P_Ubuntu2404",
	SkipIf:      skipIfRCV1PNotConfigured,
	Description: "Tests RCV1P cert mode on Ubuntu 24.04 with VM opt-in tag",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster:         ClusterKubenet,
		VHD:             config.VHDUbuntu2404Gen2Containerd,
		VMConfigMutator: rcv1pVMConfigMutator(),
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PCertMode(ctx, s)
		},
	},
})

// RCV1P_AzureLinuxV3 validates RCV1P on Azure Linux V3, which uses a different trust store
// layout (/etc/pki/ca-trust/source/anchors/) and update command (update-ca-trust) than Ubuntu.
// This ensures the provisioning script correctly detects the distro and uses the right paths.
var _ = Register(&Scenario{
	Name:        "RCV1P_AzureLinuxV3",
	SkipIf:      skipIfRCV1PNotConfigured,
	Description: "Tests RCV1P cert mode on Azure Linux V3 with VM opt-in tag",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster:         ClusterKubenet,
		VHD:             config.VHDAzureLinuxV3Gen2,
		VMConfigMutator: rcv1pVMConfigMutator(),
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PCertMode(ctx, s)
		},
	},
})

// RCV1P_ACL validates RCV1P on Azure Container Linux (ACL), which shares the same
// trust store layout as Azure Linux (/etc/pki/ca-trust/). ACL requires Trusted Launch,
// so the VMConfigMutator combines both the TrustedLaunch and opt-in tag settings.
var _ = Register(&Scenario{
	Name:        "RCV1P_ACL",
	SkipIf:      skipIfRCV1PNotConfigured,
	Description: "Tests RCV1P cert mode on ACL with VM opt-in tag",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster: ClusterKubenet,
		VHD:     config.VHDACLGen2TL,
		VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
			if m := rcv1pVMConfigMutator(); m != nil {
				m(vmss)
			}
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PCertMode(ctx, s)
		},
	},
})

// RCV1P_NotOptedIn is a negative test that validates the VM opt-in tag is required
// for cert installation. The VM is created in the RCV1P subscription (which has
// PlatformSettingsOverride registered) but WITHOUT the opt-in tag on the VMSS.
// This verifies that wireserver returns IsOptedInForRootCerts=false and the provisioning
// script correctly skips certificate download and trust store installation.
// This test requires RCV1P_TAGS_AUTO_INJECTED to not be true because the platform may auto-inject
// the opt-in tag on the current E2E subscription, making the negative test invalid.
var _ = Register(&Scenario{
	Name:        "RCV1P_NotOptedIn",
	SkipIf:      skipIfRCV1PNotExplicit,
	Description: "Tests RCV1P cert mode without VM opt-in tag; expects no cert installation",
	Tags: Tags{
		RCV1PCertMode: true,
	},
	Config: Config{
		Cluster: ClusterKubenet,
		VHD:     config.VHDUbuntu2204Gen2Containerd,
		Validator: func(ctx context.Context, s *Scenario) error {
			return ValidateRCV1PNotOptedIn(ctx, s)
		},
	},
})
