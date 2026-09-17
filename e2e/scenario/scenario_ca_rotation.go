package scenario

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/internal/carefresh"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/parts"
)

func init() {
	for _, image := range []struct {
		name string
		vhd  *config.Image
	}{
		{"Ubuntu2204", config.VHDUbuntu2204Gen2Containerd},
		{"Ubuntu2404", config.VHDUbuntu2404Gen2Containerd},
		{"AzureLinuxV3", config.VHDAzureLinuxV3Gen2},
	} {
		Register(&Scenario{
			Name:        "RCV1P_ContainerdSyntheticCARotation/" + image.name,
			Description: "Isolated synthetic CA addition through the production refresh coordinator, conditional containerd restart and no-op repeat",
			Tags:        Tags{RCV1PCertMode: true},
			SkipIf:      skipIfRCV1PRefreshNotSelected,
			Config: Config{
				Cluster:         ClusterKubenet,
				VHD:             image.vhd,
				VMConfigMutator: rcv1pOptInVMConfigMutator,
				Validator:       validateContainerdCARotation,
			},
		})
	}
}

func validateContainerdCARotation(ctx context.Context, s *Scenario) error {
	ctx, cancel := context.WithTimeout(ctx, carefresh.HealthTimeout)
	defer cancel()
	if s.IsWindows() || !s.Tags.RCV1PCertMode {
		return fmt.Errorf("requires an isolated opted-in Linux RCV1PCertMode scenario")
	}
	survivor, err := rcv1pWorkload(ctx, s)
	if err != nil {
		return err
	}
	before, err := rcv1pSnapshot(ctx, s)
	if err != nil {
		return err
	}
	report, err := validateContainerdCAFixture(ctx, s, false)
	if err != nil {
		return err
	}
	if err := carefresh.ValidateStable(before, report.Before); err != nil {
		return err
	}
	if err := carefresh.ValidateRestart(report.Before, report.After); err != nil {
		return err
	}
	if err := rcv1pNodeReady(ctx, s); err != nil {
		return err
	}
	if err := rcv1pSurvivor(ctx, s, survivor); err != nil {
		return err
	}
	if _, err := rcv1pWorkload(ctx, s); err != nil {
		return err
	}
	if err := rcv1pSurvivor(ctx, s, survivor); err != nil {
		return err
	}
	after, err := rcv1pSnapshot(ctx, s)
	if err != nil {
		return err
	}
	return carefresh.ValidateStable(report.After, after)
}

func validateContainerdCAPull(ctx context.Context, s *Scenario) error {
	_, err := validateContainerdCAFixture(ctx, s, true)
	return err
}

type caRotationScript struct {
	name    string
	content []byte
}

func caRotationScripts() ([]caRotationScript, error) {
	const artifacts = "linux/cloud-init/artifacts/"
	modules, err := fs.Glob(parts.Templates, artifacts+"cse_config*.sh")
	if err != nil {
		return nil, fmt.Errorf("find CA fixture registry modules: %w", err)
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("CA fixture registry modules are missing")
	}
	var scripts []caRotationScript
	for _, name := range append([]string{artifacts + "init-aks-cloud.sh"}, modules...) {
		content, err := parts.Templates.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read CA fixture script %s: %w", name, err)
		}
		scripts = append(scripts, caRotationScript{name: path.Base(name), content: content})
	}
	return scripts, nil
}

func caRotationBuildCommand(ctx context.Context, binary string) (*exec.Cmd, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return nil, err
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/ca-rotation-fixture")
	build.Dir = filepath.Join(repoRoot, "e2e")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	return build, nil
}

func validateContainerdCAFixture(ctx context.Context, s *Scenario, pullOnly bool) (carefresh.FixtureReport, error) {
	timeout := carefresh.FixtureTimeout + 5*time.Minute
	if pullOnly {
		timeout = 8 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	dir, err := os.MkdirTemp("", "ab-ca-fixture-")
	if err != nil {
		return carefresh.FixtureReport{}, err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "fixture")
	// Like the ANC build, compile on the runner, never install build tools on
	// the node. These scenarios use amd64 images.
	build, err := caRotationBuildCommand(ctx, binary)
	if err != nil {
		return carefresh.FixtureReport{}, fmt.Errorf("prepare CA fixture build: %w", err)
	}
	if out, err := build.CombinedOutput(); err != nil {
		return carefresh.FixtureReport{}, fmt.Errorf("build CA fixture: %w: %s", err, out)
	}
	// Use the same test blob transport as ANC binaries. Large SCP writes over
	// the runner's Bastion websocket can close that shared SSH connection.
	f, err := os.Open(binary)
	if err != nil {
		return carefresh.FixtureReport{}, err
	}
	url, uploadErr := config.Azure.UploadAndGetSignedLink(ctx, "ca-rotation/"+filepath.Base(dir), f)
	f.Close()
	if uploadErr != nil {
		return carefresh.FixtureReport{}, fmt.Errorf("upload fixture: %w", uploadErr)
	}
	remoteDir := "/home/azureuser/" + filepath.Base(dir)
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, "mkdir -m 700 "+remoteDir, 0, "create isolated CA fixture directory"); err != nil {
		return carefresh.FixtureReport{}, err
	}
	s.Cleanup(func(ctx context.Context) error {
		_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo rm -rf -- "+remoteDir, 0, "remove CA fixture directory")
		return err
	})
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		fmt.Sprintf("curl --fail --silent --show-error --retry 3 '%s' -o %s/fixture && chmod 0700 %s/fixture", url, remoteDir, remoteDir),
		0, "download CA fixture"); err != nil {
		return carefresh.FixtureReport{}, err
	}
	// Stage the branch scripts with the registry generator's sibling modules,
	// not the released VHD's potentially different configuration implementation.
	var scripts []caRotationScript
	if !pullOnly {
		scripts, err = caRotationScripts()
		if err != nil {
			return carefresh.FixtureReport{}, err
		}
	}
	for _, file := range scripts {
		local := filepath.Join(dir, file.name)
		if err := os.WriteFile(local, file.content, 0600); err != nil {
			return carefresh.FixtureReport{}, fmt.Errorf("stage %s: %w", file.name, err)
		}
		f, err := os.Open(local)
		if err != nil {
			return carefresh.FixtureReport{}, err
		}
		scriptURL, uploadErr := config.Azure.UploadAndGetSignedLink(ctx, "ca-rotation/"+filepath.Base(dir)+"/"+file.name, f)
		f.Close()
		if uploadErr != nil {
			return carefresh.FixtureReport{}, fmt.Errorf("upload %s: %w", file.name, uploadErr)
		}
		if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
			fmt.Sprintf("curl --fail --silent --show-error --retry 3 '%s' -o %s/%s && chmod 0600 %s/%s", scriptURL, remoteDir, file.name, remoteDir, file.name),
			0, "download fixture script"); err != nil {
			return carefresh.FixtureReport{}, err
		}
	}
	cmd := fmt.Sprintf("cd %s && sudo timeout %d ./fixture --node-ip %s --refresh-script %s/init-aks-cloud.sh --registry-script %s/cse_config.sh",
		remoteDir, int((carefresh.FixtureTimeout + time.Minute).Seconds()), s.Runtime.VM.PrivateIP, remoteDir, remoteDir)
	marker := "PASS: changed trust restarted containerd; identical refresh preserved runtime; CRI TLS policies preserved"
	if pullOnly {
		cmd = fmt.Sprintf("cd %s && sudo timeout 300 ./fixture --node-ip %s --pull-only", remoteDir, s.Runtime.VM.PrivateIP)
		marker = "PASS: uncached CRI network pull without OS trust changes"
	}
	before, err := rcv1pSnapshot(ctx, s)
	if err != nil {
		return carefresh.FixtureReport{}, err
	}
	result, err := execScriptOnVMForScenario(ctx, s, cmd)
	if err != nil {
		return carefresh.FixtureReport{}, err
	}
	logging.Logf(ctx, "CA rotation fixture: %s", result)
	if result.exitCode != "0" || !strings.Contains(result.stderr, marker) {
		return carefresh.FixtureReport{}, fmt.Errorf("CA rotation fixture failed: %s", result)
	}
	report, err := carefresh.ParseFixtureReport(result.stdout)
	if err != nil {
		return report, err
	}
	if err := carefresh.ValidateStable(before, report.Before); err != nil {
		return report, err
	}
	after, err := rcv1pSnapshot(ctx, s)
	if err != nil {
		return report, err
	}
	if err := carefresh.ValidateStable(report.After, after); err != nil {
		return report, err
	}
	if before.BundlePath != after.BundlePath || before.BundleSHA256 != after.BundleSHA256 {
		return report, fmt.Errorf("fixture did not preserve/restore original OS trust")
	}
	if pullOnly {
		return report, carefresh.ValidateStable(before, after)
	}
	return report, carefresh.ValidateRestart(before, after)
}
