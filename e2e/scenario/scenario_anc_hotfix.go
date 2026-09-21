package scenario

import (
	"context"
	"errors"

	"github.com/Azure/agentbaker/e2e/config"
)

const (
	ancHotfixPointerPath = "/opt/azure/containers/aks-node-controller-hotfix.json"
	ancHotfixBinaryPath  = "/opt/azure/containers/aks-node-controller-hotfix"
	ancHotfixFlowLogPath = "/var/log/azure/anc-hotfix-e2e-flow.log"
	ancLauncherOutput    = "/var/log/azure/aks-node-controller.output"
)

var _ = Register(newANCHotfixFlowScenario(
	"Ubuntu2204_ANCHotfixFlow",
	"Validates the ANC hotfix launcher flow on Ubuntu with controlled check-hotfix/download-hotfix inputs and a PR-built hotfix binary",
	config.VHDUbuntu2204Gen2Containerd,
))

var _ = Register(newANCHotfixFlowScenario(
	"AzureLinuxV3_ANCHotfixFlow",
	"Validates the ANC hotfix launcher flow on Azure Linux with controlled check-hotfix/download-hotfix inputs and a PR-built hotfix binary",
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
			if config.Config.DisableScriptLessCompilation {
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
					ValidateFileHasContent(ctx, s, ancHotfixFlowLogPath, "base check-hotfix"),
					ValidateFileHasContent(ctx, s, ancHotfixFlowLogPath, "base download-hotfix"),
					ValidateFileHasContent(ctx, s, ancHotfixFlowLogPath, "mock staged ANC hotfix 202608.14.1"),
					ValidateNoBaseANCProvision(ctx, s),
					ValidateFileHasContent(ctx, s, ancHotfixPointerPath, `"202608.14":"202608.14.1"`),
					ValidateFileExists(ctx, s, ancHotfixBinaryPath),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "running check-hotfix"),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "Found ANC hotfix config"),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "Using hotfix binary"),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "aks-node-controller completed successfully"),
				)
			},
		},
	}
}

func ValidateNoBaseANCProvision(ctx context.Context, s *Scenario) error {
	_, err := execScriptOnVMForScenarioValidateExitCode(
		ctx,
		s,
		"if sudo grep -q 'unexpected base' /var/log/azure/anc-hotfix-e2e-flow.log; then sudo cat /var/log/azure/anc-hotfix-e2e-flow.log; exit 1; fi",
		0,
		"mock base aks-node-controller unexpectedly handled provision/apply-embedded-hotfix",
	)
	return err
}
