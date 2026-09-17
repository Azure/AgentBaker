package scenario

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
)

const (
	ancHotfixPointerPath = "/opt/azure/containers/aks-node-controller-hotfix.json"
	ancHotfixBinaryPath  = "/opt/azure/containers/aks-node-controller-hotfix"
	ancBakedBinaryPath   = "/opt/azure/containers/aks-node-controller"
	ancLogPath           = "/var/log/azure/aks-node-controller.log"
	ancLauncherOutput    = "/var/log/azure/aks-node-controller.output"
)

var _ = Register(newANCHotfixPackageScenario(
	"Ubuntu2204_ANCHotfixPackage",
	"Validates the ANC hotfix package path on Ubuntu: hotfix pointer, download-hotfix, stage, and provision with the hotfixed ANC",
	config.VHDUbuntu2204Gen2Containerd,
))

var _ = Register(newANCHotfixPackageScenario(
	"AzureLinuxV3_ANCHotfixPackage",
	"Validates the ANC hotfix package path on Azure Linux: hotfix pointer, download-hotfix through the rpm package manager, stage, and provision with the hotfixed ANC",
	config.VHDAzureLinuxV3Gen2,
))

func newANCHotfixPackageScenario(name, description string, vhd *config.Image) *Scenario {
	s := &Scenario{
		Name:        name,
		Description: description,
		SkipIf: func(context.Context) string {
			// Two-stage VHD caching would bake the bake VM's downloaded hotfix binary and its
			// aks-node-controller.log into the custom image, so the provision-stage validators
			// could match that cached evidence even if the real node never downloaded anything.
			if config.Config.TestPreProvision {
				return "ANC hotfix package E2E does not run during two-stage VHD caching"
			}
			if config.Config.ANCHotfixE2EVersion == "" {
				return "ANC hotfix package E2E requires --anc-hotfix-e2e-version or ANC_HOTFIX_E2E_VERSION"
			}
			if _, err := ancHotfixBaseVersion(); err != nil {
				return err.Error()
			}
			return ""
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     vhd,
			// ENABLE_PROVISIONING_HOTFIX is deliberately NOT set. It would make the launcher run
			// check-hotfix first, and check-hotfix rewrites ancHotfixPointerPath with whatever the
			// live LPS serves - clobbering the pointer injected below unless the LPS happens to
			// return an empty map. The launcher's download-hotfix branch is gated only on the
			// pointer file existing, so this scenario still covers download/stage/provision
			// deterministically. check-hotfix and the feature gate are covered by shellspec.
			CustomDataWriteFilesWithError: ancHotfixCustomDataWriteFiles,
			Validator: func(ctx context.Context, s *Scenario) error {
				expectedVersion := config.Config.ANCHotfixE2EVersion
				return errors.Join(
					ValidateANCBakedVersionTargetedByHotfix(ctx, s, expectedVersion),
					ValidateFileHasContent(ctx, s, ancHotfixPointerPath, expectedVersion),
					ValidateFileExists(ctx, s, ancHotfixBinaryPath),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "Found ANC hotfix config"),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "Using hotfix binary"),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "aks-node-controller completed successfully"),
					ValidateFileHasContent(ctx, s, ancLogPath, "downloaded ANC hotfix"),
					ValidateANCHotfixBinaryVersion(ctx, s, expectedVersion),
				)
			},
		},
	}
	return s
}

func ancHotfixCustomDataWriteFiles() ([]CustomDataWriteFile, error) {
	content, err := ancHotfixPointerContent()
	if err != nil {
		return nil, err
	}
	return []CustomDataWriteFile{
		{
			Path:        ancHotfixPointerPath,
			Permissions: "0644",
			Owner:       "root",
			Content:     content,
		},
	}, nil
}

func ancHotfixPointerContent() (string, error) {
	base, err := ancHotfixBaseVersion()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("{\"hotfixes\":{\"%s\":\"%s\"}}\n", base, config.Config.ANCHotfixE2EVersion), nil
}

// ancHotfixBaseVersion derives the "YYYYMM.DD" pointer key from the target hotfix version.
// It mirrors aks-node-controller's hotfixBaseFromVersion, which requires three non-empty
// segments, so a version this helper accepts is one ANC can also resolve.
func ancHotfixBaseVersion() (string, error) {
	if config.Config.ANCHotfixE2EBaseVersion != "" {
		return config.Config.ANCHotfixE2EBaseVersion, nil
	}
	version := strings.TrimSpace(config.Config.ANCHotfixE2EVersion)
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("ANC hotfix version %q is not in YYYYMM.DD.PATCH form; set --anc-hotfix-e2e-base-version to override", version)
	}
	return parts[0] + "." + parts[1], nil
}

// ValidateANCBakedVersionTargetedByHotfix fails with an actionable message when the requested
// hotfix cannot apply to the ANC baked into this VHD. ANC resolves the pointer using the BAKED
// binary's own version, and only upgrades within the same base to a strictly higher patch, so a
// mismatched --anc-hotfix-e2e-version otherwise shows up as a confusing "no hotfix binary" failure.
func ValidateANCBakedVersionTargetedByHotfix(ctx context.Context, s *Scenario, expectedVersion string) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(
		ctx,
		s,
		fmt.Sprintf("sudo %s version", ancBakedBinaryPath),
		0,
		"VHD-baked aks-node-controller version command failed",
	)
	if err != nil {
		return err
	}
	baked := strings.TrimSpace(result.stdout)
	bakedParts := strings.SplitN(baked, ".", 3)
	targetParts := strings.SplitN(strings.TrimSpace(expectedVersion), ".", 3)
	if len(bakedParts) < 3 || len(targetParts) < 3 {
		return fmt.Errorf("cannot compare ANC versions: baked %q, target %q (want YYYYMM.DD.PATCH)", baked, expectedVersion)
	}
	bakedBase := bakedParts[0] + "." + bakedParts[1]
	targetBase := targetParts[0] + "." + targetParts[1]
	if bakedBase != targetBase {
		return fmt.Errorf("ANC hotfix %q does not target the VHD-baked ANC %q: base %q != %q; set --anc-hotfix-e2e-version to a %s.PATCH release", expectedVersion, baked, targetBase, bakedBase, bakedBase)
	}
	// Compare patches numerically: "9" >= "10" as strings, which would reject a valid upgrade.
	bakedPatch, err := strconv.Atoi(bakedParts[2])
	if err != nil {
		return fmt.Errorf("VHD-baked ANC version %q has a non-numeric patch: %w", baked, err)
	}
	targetPatch, err := strconv.Atoi(targetParts[2])
	if err != nil {
		return fmt.Errorf("ANC hotfix version %q has a non-numeric patch: %w", expectedVersion, err)
	}
	if bakedPatch >= targetPatch {
		return fmt.Errorf("ANC hotfix %q is not strictly newer than the VHD-baked ANC %q; ANC only upgrades to a higher patch", expectedVersion, baked)
	}
	return nil
}

func ValidateANCHotfixBinaryVersion(ctx context.Context, s *Scenario, expectedVersion string) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(
		ctx,
		s,
		fmt.Sprintf("sudo %s version", ancHotfixBinaryPath),
		0,
		"hotfixed aks-node-controller version command failed",
	)
	if err != nil {
		return err
	}
	actual := strings.TrimSpace(result.stdout)
	if actual != expectedVersion {
		return fmt.Errorf("expected hotfixed aks-node-controller version %q, got %q", expectedVersion, actual)
	}
	return nil
}
