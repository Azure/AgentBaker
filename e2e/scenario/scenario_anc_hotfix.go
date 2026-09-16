package scenario

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	ancHotfixPointerPath = "/opt/azure/containers/aks-node-controller-hotfix.json"
	ancHotfixBinaryPath  = "/opt/azure/containers/aks-node-controller-hotfix"
	ancEnabledFeatures   = "/opt/azure/containers/enabled_features.sh"
	ancLogPath           = "/var/log/azure/aks-node-controller.log"
	ancLauncherOutput    = "/var/log/azure/aks-node-controller.output"
)

var _ = Register(newANCHotfixPackageScenario(
	"Ubuntu2204_ANCHotfixPackage",
	"Validates the ANC hotfix package path on Ubuntu: launcher feature gate, check-hotfix, PMC download, stage, and provision with the hotfixed ANC",
	config.VHDUbuntu2204Gen2Containerd,
))

var _ = Register(newANCHotfixPackageScenario(
	"AzureLinuxV3_ANCHotfixPackage",
	"Validates the ANC hotfix package path on Azure Linux: launcher feature gate, check-hotfix, PMC download through dnf/tdnf fallback, stage, and provision with the hotfixed ANC",
	config.VHDAzureLinuxV3Gen2,
))

func newANCHotfixPackageScenario(name, description string, vhd *config.Image) *Scenario {
	s := &Scenario{
		Name:        name,
		Description: description,
		SkipIf: func(context.Context) string {
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
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				if nbc.EnabledFeatures == nil {
					nbc.EnabledFeatures = map[string]string{}
				}
				nbc.EnabledFeatures["ENABLE_PROVISIONING_HOTFIX"] = "true"
			},
			AKSNodeConfigMutator: func(_ *Cluster, nodeConfig *aksnodeconfigv1.Configuration) {
				if nodeConfig.EnabledFeatures == nil {
					nodeConfig.EnabledFeatures = map[string]string{}
				}
				nodeConfig.EnabledFeatures["ENABLE_PROVISIONING_HOTFIX"] = "true"
			},
			CustomDataWriteFilesWithError: ancHotfixCustomDataWriteFiles,
			Validator: func(ctx context.Context, s *Scenario) error {
				expectedVersion := config.Config.ANCHotfixE2EVersion
				return errors.Join(
					ValidateFileHasContent(ctx, s, ancEnabledFeatures, "ENABLE_PROVISIONING_HOTFIX=true"),
					ValidateFileHasContent(ctx, s, ancHotfixPointerPath, expectedVersion),
					ValidateFileExists(ctx, s, ancHotfixBinaryPath),
					ValidateFileHasContent(ctx, s, ancLauncherOutput, "ENABLE_PROVISIONING_HOTFIX=true; running check-hotfix"),
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
	if _, err := ancHotfixBaseVersion(); err != nil {
		return nil, err
	}
	return []CustomDataWriteFile{
		{
			Path:        ancHotfixPointerPath,
			Permissions: "0644",
			Owner:       "root",
			Content:     ancHotfixPointerContent(),
		},
		{
			Path:        ancEnabledFeatures,
			Permissions: "0644",
			Owner:       "root",
			Content:     "ENABLE_PROVISIONING_HOTFIX=true\n",
		},
	}, nil
}

func ancHotfixPointerContent() string {
	base, err := ancHotfixBaseVersion()
	if err != nil {
		return "{}\n"
	}
	return fmt.Sprintf("{\"hotfixes\":{\"%s\":\"%s\"}}\n", base, config.Config.ANCHotfixE2EVersion)
}

func ancHotfixBaseVersion() (string, error) {
	if config.Config.ANCHotfixE2EBaseVersion != "" {
		return config.Config.ANCHotfixE2EBaseVersion, nil
	}
	version := config.Config.ANCHotfixE2EVersion
	lastDot := strings.LastIndex(version, ".")
	if lastDot == -1 {
		return "", fmt.Errorf("ANC hotfix version %q must include a patch suffix or --anc-hotfix-e2e-base-version must be set", version)
	}
	base := version[:lastDot]
	if base == "" {
		return "", fmt.Errorf("ANC hotfix version %q produced an empty base version", version)
	}
	return base, nil
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
