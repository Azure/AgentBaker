package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
)

func (a *App) runCloudPrerequisitesCommand(ctx context.Context, flags ProvisionFlags) error {
	result, err := a.initializeCloudPrerequisites(ctx, flags)
	if err != nil {
		a.reportProvisionStatus(ctx, result, err)
		a.writeCompleteFileOnError(result, err)
		now := time.Now()
		a.eventLogger.LogEvent("Provision", result.Error, helpers.EventLevelError, now, now)
	}
	return err
}

func cloudPrerequisiteSettings(flags ProvisionFlags) (map[string]string, error) {
	// Match Provision: NBC wins when both inputs are present. Never execute it to read settings.
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
	if err := nodeconfigutils.Validate(config); err != nil {
		return nil, fmt.Errorf("validate cloud provision config: %w", err)
	}
	return map[string]string{
		"LOCATION":            config.GetClusterConfig().GetLocation(),
		"REPO_DEPOT_ENDPOINT": parser.GetRepoDepotEndpoint(config),
	}, nil
}

func (a *App) initializeCloudPrerequisites(ctx context.Context, flags ProvisionFlags) (*ProvisionResult, error) {
	result := &ProvisionResult{ExitCode: "240"}
	settings, err := cloudPrerequisiteSettings(flags)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	path := a.initAKSCloudPath
	if path == "" {
		path = "/opt/azure/containers/init-aks-cloud.sh"
	}
	cmd := exec.CommandContext(ctx, "/bin/bash", path, "prerequisites")
	cmd.Env = append(os.Environ(),
		"LOCATION="+settings["LOCATION"],
		"REPO_DEPOT_ENDPOINT="+settings["REPO_DEPOT_ENDPOINT"],
		"AKS_CLOUD_PREREQUISITES_DONE=false",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	err = a.cmdRun(cmd)
	result.Output = strings.Join([]string{stdout.String(), stderr.String()}, "\n")
	if err != nil {
		if cmd.ProcessState != nil {
			result.ExitCode = strconv.Itoa(cmd.ProcessState.ExitCode())
		}
		err = fmt.Errorf("initialize cloud prerequisites: %w", err)
		result.Error = err.Error()
		return result, err
	}
	result.ExitCode = "0"
	return result, nil
}
