package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/parser"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/fsnotify/fsnotify"
)

type App struct {
	// cmdRunner is a function that runs the given command.
	// the goal of this field is to make it easier to test the app by mocking the command runner.
	cmdRunner func(cmd *exec.Cmd) error
}

func cmdRunner(cmd *exec.Cmd) error {
	return cmd.Run()
}
func cmdRunnerDryRun(cmd *exec.Cmd) error {
	slog.Info("dry-run", "cmd", cmd.String())
	return nil
}

type ProvisionFlags struct {
	ProvisionConfig string
}

type ProvisionStatusFiles struct {
	ProvisionJSONFile     string
	ProvisionCompleteFile string
}

func (a *App) Run(ctx context.Context, args []string) int {
	slog.Info("aks-node-controller started")
	err := a.run(ctx, args)
	exitCode := errToExitCode(err)
	if exitCode == 0 {
		slog.Info("aks-node-controller finished successfully")
	} else {
		slog.Error("aks-node-controller failed", "error", err)
	}
	return exitCode
}

func (a *App) run(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("missing command argument")
	}
	switch args[1] {
	case "provision":
		fs := flag.NewFlagSet("provision", flag.ContinueOnError)
		provisionConfig := fs.String("provision-config", "", "path to the provision config file")
		dryRun := fs.Bool("dry-run", false, "print the command that would be run without executing it")
		err := fs.Parse(args[2:])
		if err != nil {
			return fmt.Errorf("parse args: %w", err)
		}
		if provisionConfig == nil || *provisionConfig == "" {
			return errors.New("--provision-config is required")
		}
		if dryRun != nil && *dryRun {
			a.cmdRunner = cmdRunnerDryRun
		}
		return a.Provision(ctx, ProvisionFlags{ProvisionConfig: *provisionConfig})
	case "provision-wait":
		provisionStatusFiles := ProvisionStatusFiles{ProvisionJSONFile: provisionJSONFilePath, ProvisionCompleteFile: provisionCompleteFilePath}
		provisionOutput, err := a.ProvisionWait(ctx, provisionStatusFiles)
		//nolint:forbidigo // stdout is part of the interface
		fmt.Println(provisionOutput)
		slog.Info("provision-wait finished", "provisionOutput", provisionOutput)
		return err
	default:
		return fmt.Errorf("unknown command: %s", args[1])
	}
}

func (a *App) Provision(ctx context.Context, flags ProvisionFlags) error {
	inputJSON, err := os.ReadFile(flags.ProvisionConfig)
	if err != nil {
		return fmt.Errorf("open provision file %s: %w", flags.ProvisionConfig, err)
	}

	config, err := nodeconfigutils.UnmarshalConfigurationV1(inputJSON)
	if err != nil {
		return fmt.Errorf("unmarshal provision config: %w", err)
	}
	// TODO: "v0" were a mistake. We are not going to have different logic maintaining both v0 and v1
	// Disallow "v0" after some time (allow some time to update consumers)
	if config.Version != "v0" && config.Version != "v1" {
		return fmt.Errorf("unsupported version: %s", config.Version)
	}

	if config.Version == "v0" {
		slog.Error("v0 version is deprecated, please use v1 instead")
	}

	// Check if AksNodeControllerUrl is specified
	if config.AksNodeControllerUrl != "" {
		return a.runExternalNodeController(ctx, config, flags.ProvisionConfig)
	}

	// Use built-in CSE logic
	return a.runBuiltinCSE(ctx, config)
}

// runExternalNodeController downloads and executes the external node controller binary.
func (a *App) runExternalNodeController(ctx context.Context, config *aksnodeconfigv1.Configuration, configPath string) error {
	slog.Info("Using external node controller", "url", config.AksNodeControllerUrl)

	// Define the download path
	binaryPath := "/tmp/aks-node-controller-external"

	// Download the binary
	if err := helpers.DownloadBinary(ctx, config.AksNodeControllerUrl, binaryPath); err != nil {
		return fmt.Errorf("failed to download external node controller: %w", err)
	}

	// Execute the external binary with the same arguments
	cmd := exec.CommandContext(ctx, binaryPath, "provision", "--provision-config", configPath)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	err := a.cmdRunner(cmd)
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	slog.Info("External node controller finished", "exitCode", exitCode, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "error", err)
	return err
}

// runBuiltinCSE runs the built-in CSE logic.
func (a *App) runBuiltinCSE(ctx context.Context, config *aksnodeconfigv1.Configuration) error {
	cmd, err := parser.BuildCSECmd(ctx, config)
	if err != nil {
		return fmt.Errorf("build CSE command: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	err = a.cmdRunner(cmd)
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	slog.Info("CSE finished", "exitCode", exitCode, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "error", err)
	return err
}

func (a *App) ProvisionWait(ctx context.Context, filepaths ProvisionStatusFiles) (string, error) {
	if _, err := os.Stat(filepaths.ProvisionCompleteFile); err == nil {
		data, err := os.ReadFile(filepaths.ProvisionJSONFile)
		if err != nil {
			return "", fmt.Errorf("failed to read provision.json: %w", err)
		}
		return string(data), nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return "", fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Watch the directory containing the provision complete file
	dir := filepath.Dir(filepaths.ProvisionCompleteFile)
	err = os.MkdirAll(dir, 0755) // create the directory if it doesn't exist
	if err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	if err = watcher.Add(dir); err != nil {
		return "", fmt.Errorf("failed to watch directory: %w", err)
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create && event.Name == filepaths.ProvisionCompleteFile {
				data, err := os.ReadFile(filepaths.ProvisionJSONFile)
				if err != nil {
					return "", fmt.Errorf("failed to read provision.json: %w", err)
				}
				return string(data), nil
			}

		case err := <-watcher.Errors:
			return "", fmt.Errorf("error watching file: %w", err)
		case <-ctx.Done():
			return "", fmt.Errorf("context deadline exceeded waiting for provision complete: %w", ctx.Err())
		}
	}
}

var _ ExitCoder = &exec.ExitError{}

type ExitCoder interface {
	error
	ExitCode() int
}

func errToExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr ExitCoder
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
