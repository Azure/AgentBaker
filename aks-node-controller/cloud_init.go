package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
)

const (
	initCloudScriptPath = "/opt/azure/containers/init-aks-cloud.sh"
	initCloudMarkerPath = "/run/aks/init-aks-cloud.done"
	// Older NBC commands run the initializer unconditionally and cannot consume the handoff.
	nbcCloudInitGuard = `if [ -f "${INIT_AKS_CLOUD_FILEPATH}" ] && [ ! -f "/run/aks/init-aks-cloud.done" ]; then`
)

func cloudInitializationSettings(flags ProvisionFlags) (location, endpoint string, supported bool, err error) {
	if flags.NBCCmd != "" {
		info, err := os.Stat(flags.NBCCmd)
		if err != nil {
			return "", "", false, fmt.Errorf("stat NBC command: %w", err)
		}
		if !info.Mode().IsRegular() {
			return "", "", false, fmt.Errorf("NBC command must be a regular file: %s", flags.NBCCmd)
		}
		content, err := os.ReadFile(flags.NBCCmd)
		if err != nil {
			return "", "", false, fmt.Errorf("read NBC command: %w", err)
		}
		if !strings.Contains(string(content), nbcCloudInitGuard) {
			return "", "", false, nil
		}
		settings := parseEnvVarsFromNBCCmdContent(string(content))
		location, hasLocation := settings["LOCATION"]
		endpoint, hasEndpoint := settings["REPO_DEPOT_ENDPOINT"]
		if !hasLocation || !hasEndpoint {
			return "", "", false, fmt.Errorf("NBC command is missing cloud initialization settings")
		}
		return location, endpoint, true, nil
	}

	if flags.ProvisionConfig == "" {
		return "", "", false, fmt.Errorf("--provision-config or --nbc-cmd is required")
	}
	content, err := os.ReadFile(flags.ProvisionConfig)
	if err != nil {
		return "", "", false, fmt.Errorf("read provision config: %w", err)
	}
	config, err := nodeconfigutils.UnmarshalConfigurationV1(content)
	if err != nil {
		return "", "", false, fmt.Errorf("decode provision config: %w", err)
	}
	location, endpoint = parser.CloudInitializationSettings(config)
	return location, endpoint, true, nil
}

func (a *App) initializeCloud(ctx context.Context, flags ProvisionFlags, scriptPath, markerPath string) error {
	location, endpoint, supported, err := cloudInitializationSettings(flags)
	if err != nil {
		return err
	}
	if !supported {
		slog.Info("NBC command does not support the cloud initialization handoff; retaining existing hotfix ordering")
		return nil
	}

	// The path is fixed by the caller and the cloud settings are passed as environment values.
	cmd := exec.CommandContext(ctx, scriptPath) // #nosec G204
	cmd.Env = append(os.Environ(), "LOCATION="+location, "REPO_DEPOT_ENDPOINT="+endpoint)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := a.cmdRun(cmd); err != nil {
		return fmt.Errorf("initialize cloud: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(markerPath), 0755); err != nil {
		slog.Warn("failed to create cloud initialization marker; provisioning will initialize again", "error", err)
	} else if err := os.WriteFile(markerPath, nil, 0644); err != nil {
		slog.Warn("failed to create cloud initialization marker; provisioning will initialize again", "error", err)
	}
	return nil
}
