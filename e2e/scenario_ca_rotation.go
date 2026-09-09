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
			Name:        "ContainerdCARotation/" + image.name,
			Description: "Real CRI image pulls adopt refreshed OS trust without restarting containerd",
			Config: Config{
				Cluster:   ClusterKubenet,
				VHD:       image.vhd,
				Validator: validateContainerdCARotation,
			},
		})
	}
}

func validateContainerdCARotation(ctx context.Context, s *Scenario) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	dir, err := os.MkdirTemp("", "ab-ca-fixture-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "fixture")
	// Like the ANC build, compile on the runner, never install build tools on
	// the node. These scenarios use amd64 images.
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/ca-rotation-fixture")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build CA fixture: %w: %s", err, out)
	}
	// Use the same test blob transport as ANC binaries. Large SCP writes over
	// the runner's Bastion websocket can close that shared SSH connection.
	f, err := os.Open(binary)
	if err != nil {
		return err
	}
	url, uploadErr := config.Azure.UploadAndGetSignedLink(ctx, "ca-rotation/"+filepath.Base(dir), f)
	f.Close()
	if uploadErr != nil {
		return fmt.Errorf("upload fixture: %w", uploadErr)
	}
	remoteDir := "/home/azureuser/" + filepath.Base(dir)
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, "mkdir -m 700 "+remoteDir, 0, "create isolated CA fixture directory"); err != nil {
		return err
	}
	s.Cleanup(func(ctx context.Context) error {
		_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo rm -rf -- "+remoteDir, 0, "remove CA fixture directory")
		return err
	})
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		fmt.Sprintf("curl --fail --silent --show-error --retry 3 '%s' -o %s/fixture && chmod 0700 %s/fixture", url, remoteDir, remoteDir),
		0, "download CA fixture"); err != nil {
		return err
	}
	// Upload the branch refresh script rather than silently exercising the
	// released VHD's older script. This also tests refresh on an existing node.
	for _, file := range []struct{ local, remote, mode string }{
		{filepath.Join("..", "parts/linux/cloud-init/artifacts/init-aks-cloud.sh"), "init-aks-cloud.sh", "0600"},
		{filepath.Join("..", "parts/linux/cloud-init/artifacts/cse_config.sh"), "cse_config.sh", "0600"},
	} {
		f, err := os.Open(file.local)
		if err != nil {
			return err
		}
		scriptURL, uploadErr := config.Azure.UploadAndGetSignedLink(ctx, "ca-rotation/"+filepath.Base(dir)+"/"+file.remote, f)
		f.Close()
		if uploadErr != nil {
			return fmt.Errorf("upload %s: %w", file.remote, uploadErr)
		}
		if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
			fmt.Sprintf("curl --fail --silent --show-error --retry 3 '%s' -o %s/%s && chmod %s %s/%s", scriptURL, remoteDir, file.remote, file.mode, remoteDir, file.remote),
			0, "download fixture script"); err != nil {
			return err
		}
	}
	cmd := fmt.Sprintf("sudo %s/fixture --node-ip %s --refresh-script %s/init-aks-cloud.sh --registry-script %s/cse_config.sh",
		remoteDir, s.Runtime.VM.PrivateIP, remoteDir, remoteDir)
	result, err := execScriptOnVMForScenario(ctx, s, cmd)
	if err != nil {
		return err
	}
	toolkit.Logf(ctx, "CA rotation fixture: %s", result)
	if result.exitCode != "0" || !strings.Contains(result.stderr, "PASS: refreshed trust used by CRI without restarting containerd") {
		return fmt.Errorf("CA rotation fixture failed: %s", result)
	}
	return nil
}
