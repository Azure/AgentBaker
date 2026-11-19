package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/urfave/cli/v3"
)

func (a *App) runInitializeCloudCommand(ctx context.Context, cmd *cli.Command) error {
	hash, err := a.initializeCloud(ctx, ProvisionFlags{
		ProvisionConfig: cmd.String("provision-config"),
		NBCCmd:          cmd.String("nbc-cmd"),
	}, "/opt/azure/containers/init-aks-cloud.sh")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.Root().Writer, hash)
	return err
}

func cloudInitializationSettings(flags ProvisionFlags) (map[string]string, error) {
	// Match provisioning's NBC precedence without executing the command file.
	if flags.NBCCmd != "" {
		info, err := os.Stat(flags.NBCCmd)
		if err != nil {
			return nil, fmt.Errorf("stat NBC cloud settings: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("NBC cloud settings must be a regular file: %s", flags.NBCCmd)
		}
		content, err := os.ReadFile(flags.NBCCmd)
		if err != nil {
			return nil, fmt.Errorf("read NBC cloud settings: %w", err)
		}
		settings := parseEnvVarsFromNBCCmdContent(string(content))
		if strings.TrimSpace(settings["LOCATION"]) == "" {
			return nil, fmt.Errorf("NBC cloud settings must include LOCATION")
		}
		return settings, nil
	}
	if flags.ProvisionConfig == "" {
		return nil, fmt.Errorf("--provision-config or --nbc-cmd is required")
	}
	content, err := os.ReadFile(flags.ProvisionConfig)
	if err != nil {
		return nil, fmt.Errorf("read cloud provision config: %w", err)
	}
	config, err := nodeconfigutils.UnmarshalConfigurationV1(content)
	if err != nil {
		return nil, fmt.Errorf("decode cloud provision config: %w", err)
	}
	if strings.TrimSpace(config.GetClusterConfig().GetLocation()) == "" {
		return nil, fmt.Errorf("cloud settings must include LOCATION")
	}
	return parser.CloudInitializationSettings(config), nil
}

func (a *App) initializeCloud(ctx context.Context, flags ProvisionFlags, scriptPath string) (string, error) {
	settings, err := cloudInitializationSettings(flags)
	if err != nil {
		return "", err
	}
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", fmt.Errorf("read cloud initializer: %w", err)
	}
	cmd := exec.CommandContext(ctx, "/bin/bash", "--", scriptPath)
	cmd.Env = append(os.Environ(),
		"LOCATION="+settings["LOCATION"],
		"REPO_DEPOT_ENDPOINT="+settings["REPO_DEPOT_ENDPOINT"],
		"AKS_CLOUD_INIT_SHA256=",
	)
	// Reserve stdout for the successful initializer's hash, consumed by the launcher.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := a.cmdRun(cmd); err != nil {
		return "", fmt.Errorf("initialize cloud: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(script)), nil
}
