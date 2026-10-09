package scenario

import (
	"context"
	"errors"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	// enabledFeaturesPath is the on-node KEY=VALUE feature-flag file. AgentBaker writes it from
	// NodeBootstrappingConfiguration.EnabledFeatures, and the aks-node-controller wrapper parses
	// and exports every valid entry before launching the binary.
	enabledFeaturesPath = "/opt/azure/containers/enabled_features.sh"

	// aksNodeConfigPath is the AKSNodeConfig delivered alongside the NBC command. (ancNBCCmdPath
	// for the other source already exists in scenario_anc_hotfix.go.)
	aksNodeConfigPath = "/opt/azure/containers/aks-node-controller-config.json"

	// useAKSNodeConfigFeature selects AKSNodeConfig as the provisioning source when an NBC
	// command is also present (scriptless phase 3). Setting it here stands in for the aks-rp
	// toggle that will eventually populate EnabledFeatures - nothing in RP sets it yet, so the
	// E2E has to opt in itself to exercise the path at all.
	useAKSNodeConfigFeature = "USE_AKS_NODE_CONFIG"

	// ancProvisionSourceConfig / ancProvisionSourceNBCCmd are fragments of the structured log
	// aks-node-controller emits naming the source it actually provisioned from. Asserting it is
	// what makes this scenario meaningful: both sources are delivered and both produce a working
	// node, so node readiness alone cannot distinguish which one ran. The binary logs through a
	// JSON handler (main.go), so these match the serialized attribute, not logfmt.
	ancProvisionSourceConfig  = `"source":"provision-config"`
	ancProvisionSourceNBCCmd  = `"source":"nbc-cmd"`
	ancCompareEnvsLogFragment = "ProvisionConfig and NBCCmd both provided, comparing envs"
)

// This scenario pins the phase 3 source switch. It deliberately keeps BOTH provisioning sources
// on the node - the toggle changes which one executes, not what is delivered - so it also proves
// the reversed comparison still runs: the config provisions while nbc-cmd serves as the baseline.
//
// Supplying both a BootstrapConfigMutator and an AKSNodeConfigMutator without the Scriptless tag
// is what puts the node in the two-source state (see prepareAKSNode in provision.go), which is
// the only state where the toggle has any effect.
var _ = Register(newUseAKSNodeConfigScenario(
	"Ubuntu2204_UseAKSNodeConfig",
	"Validates that USE_AKS_NODE_CONFIG makes aks-node-controller provision from the AKSNodeConfig on Ubuntu while the NBC command remains on the node for comparison",
	config.VHDUbuntu2204Gen2Containerd,
))

var _ = Register(newUseAKSNodeConfigScenario(
	"AzureLinuxV3_UseAKSNodeConfig",
	"Validates that USE_AKS_NODE_CONFIG makes aks-node-controller provision from the AKSNodeConfig on Azure Linux while the NBC command remains on the node for comparison",
	config.VHDAzureLinuxV3Gen2,
))

func newUseAKSNodeConfigScenario(name, description string, vhd *config.Image) *Scenario {
	return &Scenario{
		Name:        name,
		Description: description,
		SkipIf: func(context.Context) string {
			if config.Config.TestPreProvision {
				return "USE_AKS_NODE_CONFIG E2E does not run during two-stage VHD caching"
			}
			if config.Config.DisableScriptless {
				return "USE_AKS_NODE_CONFIG E2E requires scriptless provisioning"
			}
			return ""
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     vhd,
			// The mutator pair below is load-bearing, not decoration: it is what makes
			// prepareAKSNode deliver both aks-node-controller-nbc-cmd.sh and
			// aks-node-controller-config.json. The feature toggle rides along on the NBC and
			// AgentBaker renders it into enabled_features.sh with no allowlist, so no
			// producer-side change is needed to run this.
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				if nbc.EnabledFeatures == nil {
					nbc.EnabledFeatures = map[string]string{}
				}
				nbc.EnabledFeatures[useAKSNodeConfigFeature] = "true"
			},
			AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
				if cfg.EnabledFeatures == nil {
					cfg.EnabledFeatures = map[string]string{}
				}
				cfg.EnabledFeatures[useAKSNodeConfigFeature] = "true"
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					// The toggle actually reached the node. Without this a later assertion
					// could pass for the wrong reason - e.g. if nbc-cmd were simply absent.
					ValidateFileHasContent(ctx, s, enabledFeaturesPath, useAKSNodeConfigFeature+"=true"),
					validateLauncherAnnouncedToggle(ctx, s),

					// Both sources are still delivered: this scenario is the reverse-compare
					// step, not the retirement of nbc-cmd.
					ValidateFileExists(ctx, s, ancNBCCmdPath),
					ValidateFileExists(ctx, s, aksNodeConfigPath),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "Launching aks-node-controller with nbc cmd"),

					// The actual behavior under test, and its negation. Asserting only the
					// positive would still pass if the binary logged both.
					ValidateFileHasContent(ctx, s, ancLogPath, ancProvisionSourceConfig),
					ValidateFileExcludesContent(ctx, s, ancLogPath, ancProvisionSourceNBCCmd),

					// The whole point of keeping nbc-cmd on the command line: the comparison
					// survives the switch instead of going dark exactly when it is needed.
					ValidateFileHasContent(ctx, s, ancLogPath, ancCompareEnvsLogFragment),

					ValidateFileHasContent(ctx, s, ancLauncherOutput, "aks-node-controller completed successfully"),
				)
			},
		},
	}
}

// validateLauncherAnnouncedToggle asserts the launcher logged the toggle it acted on.
//
// That log line lives in aks-node-controller-launcher.sh, which packer bakes into the image
// (vhdbuilder/packer/packer_source.sh) rather than delivering through CustomData, so a lane that
// resolved a main-built image runs a launcher predating the toggle and cannot emit it. The binary
// is not subject to the same lag - the hotfix path compiles it from this branch and ships it to
// every lane - so the assertions on the executed source stay ungated and keep this scenario
// meaningful even where the launcher is old. That split is also why the line is only ever a log:
// the binary reads USE_AKS_NODE_CONFIG itself, and the launcher merely narrates the decision.
func validateLauncherAnnouncedToggle(ctx context.Context, s *Scenario) error {
	if laneResolvedMainBuiltImage() {
		logging.Logf(ctx, "SKIP: this lane resolved a main-built image (%s=%s), which predates the "+
			"USE_AKS_NODE_CONFIG line in aks-node-controller-launcher.sh; run against the PR's VHD build to exercise it",
			config.Config.SIGVersionTagName, config.Config.SIGVersionTagValue)
		return nil
	}
	return ValidateFileHasContent(ctx, s, ancLauncherOutput, "USE_AKS_NODE_CONFIG is enabled")
}
