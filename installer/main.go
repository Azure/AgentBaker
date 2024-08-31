package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func main() {
	slog.Info("Installer started")
	ctx := context.Background()
	if err := run(ctx); err != nil {
		slog.Error("Installer finished with error", "error", err.Error())
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	slog.Info("Installer finished")
}

func run(ctx context.Context) error {
	config := &datamodel.NodeBootstrappingConfiguration{}

	configFile, err := os.Open("config.json")
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(config); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}

	if err := provisionStart(ctx, config); err != nil {
		return fmt.Errorf("provision start: %w", err)
	}
	return nil
}

func provisionStart(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) error {
	slog.Info("Running provision_start.sh")
	defer slog.Info("Finished provision_start.sh")
	cse, err := CSEScript(ctx, config)
	if err != nil {
		return fmt.Errorf("cse script: %w", err)
	}
	slog.Info("Running command", "command", cse)
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", cse)
	cmd.Dir = "/"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func CSEScript(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) (string, error) {
	ab, err := agent.NewAgentBaker()
	if err != nil {
		return "", err
	}
	nodeBootstrapping, err := ab.GetNodeBootstrapping(ctx, config)
	if err != nil {
		return "", err
	}
	return nodeBootstrapping.CSE, nil
}
