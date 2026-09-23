package scenario

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
)

const (
	ancHotfixPointerPath = "/opt/azure/containers/aks-node-controller-hotfix.json"
	ancHotfixBinaryPath  = "/opt/azure/containers/aks-node-controller-hotfix"
	ancBakedBinaryPath   = "/opt/azure/containers/aks-node-controller"
	ancLogPath           = "/var/log/azure/aks-node-controller.log"
	ancLauncherOutput    = "/var/log/azure/aks-node-controller.output"

	// The launcher and its optional hotfix helper are baked into the VHD by packer
	// (vhdbuilder/packer/packer_source.sh installs them at mode 0755) rather than delivered
	// through custom data, so a node boots with whatever copy the E2E VHD shipped. The fixture
	// overwrites both with the working-tree copies; see ancLauncherOverrideCmd.
	ancLauncherPath     = "/opt/azure/containers/aks-node-controller-launcher.sh"
	ancHotfixHelperPath = "/opt/azure/containers/aks-node-controller-hotfix.sh"

	ancLauncherRepoPath     = "parts/linux/cloud-init/artifacts/aks-node-controller-launcher.sh"
	ancHotfixHelperRepoPath = "parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.sh"

	// ancHotfixFlowTargetVersion is a real published ANC hotfix (git tag
	// aks-node-controller/hotfix/v202608.21.1), so download-hotfix performs a real PMC
	// download and a real package-manager install rather than hitting a synthetic artifact.
	//
	// The PR-built ANC is stamped to ancHotfixFlowBaseVersion so it shares the "202608.21"
	// base and sits at a strictly lower patch, which is what makes the pointer applicable:
	// download-hotfix derives the base from the running binary's version and only upgrades
	// when the base matches and the target patch is higher.
	ancHotfixFlowBaseVersion   = "202608.21.0"
	ancHotfixFlowTargetVersion = "202608.21.1"
)

// The fixture writes the hotfix pointer directly instead of letting check-hotfix fetch it
// from the live-patching service. LPS only serves the ANC hotfix map where the aks-rp
// aks-node-controller-hotfixes toggle matches, and that toggle is scoped to deploy-env
// staging plus a single subscription that is not the AgentBaker E2E subscription, so
// check-hotfix here would always receive an empty config and write no pointer.
//
// Writing the pointer keeps this scenario deterministic and leaves the LPS leg to the
// aks-rp e2ev3 scenario that already covers it end to end (AKSNodeControllerCheckHotfix).
// Everything downstream of the pointer - download, package install, staging, binary
// selection and provisioning - still runs for real against the published hotfix.
//
// The fixture also overwrites the VHD-baked launcher (and the hotfix helper it sources, on
// branches that ship one) with the working-tree copies. Those scripts reach a node through
// packer, not custom data, so without the override the scenario would exercise whichever
// launcher the E2E VHD happened to be built from rather than the code under review.

var _ = Register(newANCHotfixFlowScenario(
	"Ubuntu2204_ANCHotfixFlow",
	"Validates the real ANC hotfix download/stage/select flow on Ubuntu from a seeded hotfix pointer",
	config.VHDUbuntu2204Gen2Containerd,
))

var _ = Register(newANCHotfixFlowScenario(
	"AzureLinuxV3_ANCHotfixFlow",
	"Validates the real ANC hotfix download/stage/select flow on Azure Linux from a seeded hotfix pointer",
	config.VHDAzureLinuxV3Gen2,
))

func newANCHotfixFlowScenario(name, description string, vhd *config.Image) *Scenario {
	return &Scenario{
		Name:        name,
		Description: description,
		SkipIf: func(context.Context) string {
			if config.Config.TestPreProvision {
				return "ANC hotfix flow E2E does not run during two-stage VHD caching"
			}
			if config.Config.DisableScriptless || config.Config.DisableScriptLessCompilation {
				return "ANC hotfix flow E2E requires scriptless ANC compilation"
			}
			return ""
		},
		Config: Config{
			Cluster:              ClusterKubenet,
			VHD:                  vhd,
			ANCHotfixFlowFixture: true,
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateANCLauncherMatchesRepo(ctx, s),
					ValidateANCBakedBinaryVersion(ctx, s, ancHotfixFlowBaseVersion),
					ValidateFileHasContent(ctx, s, ancHotfixPointerPath, fmt.Sprintf(`"%s":"%s"`, hotfixBaseVersion(ancHotfixFlowBaseVersion), ancHotfixFlowTargetVersion)),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "Found ANC hotfix config"),
					ValidateFileHasContent(ctx, s, ancLogPath, "downloading ANC hotfix"),
					ValidateFileHasContent(ctx, s, ancLogPath, "downloaded ANC hotfix"),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "ANC download-hotfix completed"),
					ValidateFileExists(ctx, s, ancHotfixBinaryPath),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "Using hotfix binary"),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "aks-node-controller completed successfully"),
					ValidateANCHotfixBinaryVersion(ctx, s, ancHotfixFlowTargetVersion),
				)
			},
		},
	}
}

func hotfixBaseVersion(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}

// ValidateANCLauncherMatchesRepo proves the fixture's launcher override actually landed.
//
// A failed override is otherwise invisible: the node falls back to the VHD-baked launcher and
// every downstream assertion still passes, silently validating old code instead of the PR's.
func ValidateANCLauncherMatchesRepo(ctx context.Context, s *Scenario) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("locate repo root: %w", err)
	}
	content, err := os.ReadFile(filepath.Join(repoRoot, ancLauncherRepoPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", ancLauncherRepoPath, err)
	}
	sum := sha256.Sum256(content)
	expected := hex.EncodeToString(sum[:])

	result, err := execScriptOnVMForScenarioValidateExitCode(
		ctx,
		s,
		fmt.Sprintf("sudo sha256sum %s | cut -d' ' -f1", ancLauncherPath),
		0,
		"failed to hash the on-node aks-node-controller launcher",
	)
	if err != nil {
		return err
	}
	if actual := strings.TrimSpace(result.stdout); actual != expected {
		return fmt.Errorf(
			"launcher override did not land: %s has sha256 %q, want %q (working-tree copy) - the node ran the VHD-baked launcher",
			ancLauncherPath, actual, expected,
		)
	}
	return nil
}

func ValidateANCBakedBinaryVersion(ctx context.Context, s *Scenario, expectedVersion string) error {
	return validateANCBinaryVersion(ctx, s, ancBakedBinaryPath, expectedVersion, "version-stamped aks-node-controller version command failed")
}

func ValidateANCHotfixBinaryVersion(ctx context.Context, s *Scenario, expectedVersion string) error {
	return validateANCBinaryVersion(ctx, s, ancHotfixBinaryPath, expectedVersion, "hotfixed aks-node-controller version command failed")
}

func validateANCBinaryVersion(ctx context.Context, s *Scenario, path, expectedVersion, failureMessage string) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(
		ctx,
		s,
		fmt.Sprintf("sudo %s version", path),
		0,
		failureMessage,
	)
	if err != nil {
		return err
	}
	actual := strings.TrimSpace(result.stdout)
	if actual != expectedVersion {
		return fmt.Errorf("expected %s version %q, got %q", path, expectedVersion, actual)
	}
	return nil
}
