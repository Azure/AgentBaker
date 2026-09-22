package scenario

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
)

const (
	ancHotfixPointerPath = "/opt/azure/containers/aks-node-controller-hotfix.json"
	ancHotfixBinaryPath  = "/opt/azure/containers/aks-node-controller-hotfix"
	ancBakedBinaryPath   = "/opt/azure/containers/aks-node-controller"
	ancLogPath           = "/var/log/azure/aks-node-controller.log"
	ancLauncherOutput    = "/var/log/azure/aks-node-controller.output"

	ancHotfixFlowBaseVersion   = "202608.14.0"
	ancHotfixFlowTargetVersion = "202608.14.1"
)

var _ = Register(newANCHotfixFlowScenario(
	"Ubuntu2204_ANCHotfixFlow",
	"Validates the ANC hotfix launcher flow on Ubuntu with a PR-built ANC stamped to a known hotfix base version",
	config.VHDUbuntu2204Gen2Containerd,
))

var _ = Register(newANCHotfixFlowScenario(
	"AzureLinuxV3_ANCHotfixFlow",
	"Validates the ANC hotfix launcher flow on Azure Linux with a PR-built ANC stamped to a known hotfix base version",
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
					ValidateANCBakedBinaryVersion(ctx, s, ancHotfixFlowBaseVersion),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "running check-hotfix"),
					ValidateFileHasContent(ctx, s, ancLogPath, "check-hotfix completed"),
					ValidateFileHasContent(ctx, s, ancHotfixPointerPath, fmt.Sprintf(`"%s":"%s"`, hotfixBaseVersion(ancHotfixFlowBaseVersion), ancHotfixFlowTargetVersion)),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "Found ANC hotfix config"),
					ValidateFileHasContent(ctx, s, ancLogPath, "downloaded ANC hotfix"),
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
