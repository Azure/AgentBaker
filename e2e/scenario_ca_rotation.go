package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/internal/carefresh"
	"github.com/Azure/agentbaker/e2e/toolkit"
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

func validateContainerdCAFixture(ctx context.Context, s *Scenario, pullOnly bool) (carefresh.FixtureReport, error) {
	timeout := carefresh.FixtureTimeout + 5*time.Minute
	if pullOnly {
		timeout = 8 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	dir, err := os.MkdirTemp(".", "ab-ca-fixture-")
	if err != nil {
		return carefresh.FixtureReport{}, err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "fixture")
	// Like the ANC build, compile on the runner, never install build tools on
	// the node. These scenarios use amd64 images.
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/ca-rotation-fixture")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
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
	// Upload the branch refresh script rather than silently exercising the
	// released VHD's older script. This also tests refresh on an existing node.
	scripts := []struct{ local, remote, mode string }{
		{filepath.Join("..", "parts/linux/cloud-init/artifacts/init-aks-cloud.sh"), "init-aks-cloud.sh", "0600"},
		{filepath.Join("..", "parts/linux/cloud-init/artifacts/cse_config.sh"), "cse_config.sh", "0600"},
	}
	if pullOnly {
		scripts = nil
	}
	for _, file := range scripts {
		f, err := os.Open(file.local)
		if err != nil {
			return carefresh.FixtureReport{}, err
		}
		scriptURL, uploadErr := config.Azure.UploadAndGetSignedLink(ctx, "ca-rotation/"+filepath.Base(dir)+"/"+file.remote, f)
		f.Close()
		if uploadErr != nil {
			return carefresh.FixtureReport{}, fmt.Errorf("upload %s: %w", file.remote, uploadErr)
		}
		if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
			fmt.Sprintf("curl --fail --silent --show-error --retry 3 '%s' -o %s/%s && chmod %s %s/%s", scriptURL, remoteDir, file.remote, file.mode, remoteDir, file.remote),
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
	toolkit.Logf(ctx, "CA rotation fixture: %s", result)
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
