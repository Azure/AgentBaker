package scenario

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestScriptlessProvisioningMode(t *testing.T) {
	disabled := config.Config.DisableScriptLessCompilation
	config.Config.DisableScriptLessCompilation = false
	t.Cleanup(func() { config.Config.DisableScriptLessCompilation = disabled })

	tests := []struct {
		name        string
		image       *config.Image
		nbc         *datamodel.NodeBootstrappingConfiguration
		provision   bool
		nbcCommand  bool
		compilation bool
	}{
		{name: "missing image", nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true}},
		{name: "missing NBC", image: config.VHDACLGen2TL},
		{name: "legacy ACL", image: config.VHDACLGen2TL, nbc: &datamodel.NodeBootstrappingConfiguration{}},
		{name: "ACL NBC", image: config.VHDACLGen2TL, nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true}, provision: true, nbcCommand: true},
		{name: "ACL ARM64 NBC", image: config.VHDACLArm64Gen2TL, nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true}, provision: true, nbcCommand: true},
		{name: "ACL CVM NBC", image: config.VHDACLCVMGen2, nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true}, provision: true, nbcCommand: true},
		{name: "ACL CSE delivery", image: config.VHDACLGen2TL, nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true, ScriptlessCSEProvisionMode: true}, provision: true, nbcCommand: true},
		{name: "custom data only", image: config.VHDACLGen2TL, nbc: &datamodel.NodeBootstrappingConfiguration{CustomDataOnlyProvisioning: true}, provision: true, nbcCommand: true},
		{name: "preprovision only", image: config.VHDACLGen2TL, nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true, CustomDataOnlyProvisioning: true, PreProvisionOnly: true}},
		{name: "supported NBC", image: config.VHDAzureLinuxV3Gen2Arm64, nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true}, provision: true, nbcCommand: true, compilation: true},
		{name: "supported phase 3", image: config.VHDAzureLinuxV3Gen2Arm64, nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessAKSNodeConfig: true}, provision: true, compilation: true},
		{name: "phase 3 takes precedence", image: config.VHDAzureLinuxV3Gen2Arm64, nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true, EnableScriptlessAKSNodeConfig: true}, provision: true, compilation: true},
		{name: "custom data only preserves compilation opt-in", image: config.VHDAzureLinuxV3Gen2Arm64, nbc: &datamodel.NodeBootstrappingConfiguration{CustomDataOnlyProvisioning: true}, provision: true, nbcCommand: true},
		{name: "unsupported image cannot compile", image: &config.Image{Distro: datamodel.AKSAzureLinuxV2Gen2}, nbc: &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true}, provision: true, nbcCommand: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario{Config: Config{VHD: tt.image}, Runtime: &ScenarioRuntime{NBC: tt.nbc}}
			require.Equal(t, tt.provision, usesScriptlessProvisioning(s))
			require.Equal(t, tt.nbcCommand, usesScriptlessNBCCSECmd(s))
			require.Equal(t, tt.compilation, enableScriptlessCompilation(s))
		})
	}
	require.False(t, usesScriptlessProvisioning(nil))
	require.False(t, usesScriptlessNBCCSECmd(&Scenario{}))
	require.False(t, config.VHDACLGen2TL.SupportsScriptless())
}

func TestACLNBCValidationDoesNotSkip(t *testing.T) {
	metadata := config.Config.VHDMetadataFile
	config.Config.VHDMetadataFile = "test-metadata.json"
	t.Cleanup(func() { config.Config.VHDMetadataFile = metadata })

	s := &Scenario{
		Config: Config{VHD: config.VHDACLGen2TL},
		Runtime: &ScenarioRuntime{
			NBC:           &datamodel.NodeBootstrappingConfiguration{EnableScriptlessNBCCSECmd: true},
			AKSNodeConfig: &aksnodeconfigv1.Configuration{},
		},
	}
	// A missing VM must fail validation rather than be hidden by an image-support skip.
	require.ErrorContains(t, ValidateScriptlessNBCCSECmd(context.Background(), s), "nil VM")
	require.ErrorContains(t, ValidateScriptlessComparison(context.Background(), s), "nil VM")
	require.NoError(t, ValidateACLLegacyFirstBoot(context.Background(), s))

	s.Runtime.NBC.EnableScriptlessNBCCSECmd = false
	require.NoError(t, ValidateScriptlessNBCCSECmd(context.Background(), s))
	require.NoError(t, ValidateScriptlessComparison(context.Background(), s))
	require.ErrorContains(t, ValidateACLLegacyFirstBoot(context.Background(), s), "nil VM")

	s.Runtime.NBC.CustomDataOnlyProvisioning = true
	require.NoError(t, ValidateACLLegacyFirstBoot(context.Background(), s))
	s.Runtime.NBC = &datamodel.NodeBootstrappingConfiguration{EnableScriptlessAKSNodeConfig: true}
	require.NoError(t, ValidateACLLegacyFirstBoot(context.Background(), s))
	s.Runtime.NBC = nil
	require.ErrorContains(t, ValidateACLLegacyFirstBoot(context.Background(), s), "NBC metadata")
	s.Config.VHD = config.VHDAzureLinuxV3Gen2Arm64
	require.NoError(t, ValidateACLLegacyFirstBoot(context.Background(), s))
}

func TestANCLauncherOutputCommand(t *testing.T) {
	for _, tt := range []struct {
		name    string
		image   *config.Image
		cse     bool
		journal bool
	}{
		{name: "ACL Ignition", image: config.VHDACLGen2TL, journal: true},
		{name: "ACL CSE delivery", image: config.VHDACLGen2TL, cse: true},
		{name: "Azure Linux boothook", image: config.VHDAzureLinuxV3Gen2Arm64},
		{name: "Azure Linux CSE delivery", image: config.VHDAzureLinuxV3Gen2Arm64, cse: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario{
				Config:  Config{VHD: tt.image},
				Runtime: &ScenarioRuntime{NBC: &datamodel.NodeBootstrappingConfiguration{ScriptlessCSEProvisionMode: tt.cse}},
			}
			if tt.journal {
				require.Equal(t, "sudo journalctl -u aks-node-controller.service --no-pager -o cat", ancLauncherOutputCommand(s))
			} else {
				require.Equal(t, "sudo cat /var/log/azure/aks-node-controller.output", ancLauncherOutputCommand(s))
			}
		})
	}
}

func TestScriptlessComparisonOutput(t *testing.T) {
	require.NoError(t, validateScriptlessComparisonOutput("prefix: env compare: no differences found between provision-config and nbc-cmd env vars\n"))
	for _, marker := range []string{"differs", "only-in-pc", "only-in-nbc", "env var differences"} {
		t.Run(marker, func(t *testing.T) {
			err := validateScriptlessComparisonOutput("unrelated log content\n" + marker + ": example\n")
			require.ErrorContains(t, err, marker+": example")
			require.NotContains(t, err.Error(), "unrelated log content")
		})
	}
	require.ErrorContains(t, validateScriptlessComparisonOutput(""), "no successful environment comparison")
	require.ErrorContains(t, validateScriptlessComparisonOutput("unrelated output"), "no successful environment comparison")
}

func TestACLLegacyFirstBootScript(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for ACL guest validation tests")
	}
	for _, tt := range []struct {
		name      string
		unit      string
		kind      string
		wantError string
	}{
		{name: "both links correct"},
		{name: "missing boot-command link", unit: "ignition-bootcmds.service", kind: "missing", wantError: "Expected first-boot symlink"},
		{name: "missing extraction link", unit: "ignition-file-extract.service", kind: "missing", wantError: "Expected first-boot symlink"},
		{name: "wrong boot-command target", unit: "ignition-bootcmds.service", kind: "wrong", wantError: "Unexpected first-boot symlink target"},
		{name: "wrong extraction target", unit: "ignition-file-extract.service", kind: "wrong", wantError: "Unexpected first-boot symlink target"},
		{name: "dangling boot-command link", unit: "ignition-bootcmds.service", kind: "dangling", wantError: "Unable to resolve first-boot symlink"},
		{name: "dangling extraction link", unit: "ignition-file-extract.service", kind: "dangling", wantError: "Unable to resolve first-boot symlink"},
		{name: "regular file is not a link", unit: "ignition-file-extract.service", kind: "regular", wantError: "Expected first-boot symlink"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			wants := filepath.Join(root, "sysinit.target.wants")
			require.NoError(t, os.Mkdir(wants, 0755))
			for _, unit := range []string{"ignition-bootcmds.service", "ignition-file-extract.service"} {
				target := filepath.Join(root, unit)
				require.NoError(t, os.WriteFile(target, []byte("[Service]\nType=oneshot\n"), 0644))
				link := filepath.Join(wants, unit)
				if unit == tt.unit {
					switch tt.kind {
					case "missing":
						continue
					case "regular":
						require.NoError(t, os.WriteFile(link, []byte("not a symlink"), 0644))
						continue
					case "wrong":
						target = filepath.Join(root, "other.service")
						require.NoError(t, os.WriteFile(target, nil, 0644))
					case "dangling":
						require.NoError(t, os.Remove(target))
					}
				}
				require.NoError(t, os.Symlink(target, link))
			}
			script := "sudo() { \"$@\"; }\n" + strings.ReplaceAll(validateACLLegacyFirstBootScript, "/etc/systemd/system", root)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, "bash", "-c", script).CombinedOutput()
			if tt.wantError == "" {
				require.NoError(t, err, "%s", output)
			} else {
				require.Error(t, err)
				require.Contains(t, string(output), tt.wantError)
			}
		})
	}
}
