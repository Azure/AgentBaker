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

	"github.com/Azure/agentbaker/aks-node-controller/parser"
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
		err := a.runProvision(ctx, args[2:])
		// Always notify after provisioning attempt (success is a no-op inside notifier)
		a.writeCompleteFileOnError(err)
		return err
	case "provision-wait":
		provisionStatusFiles := ProvisionStatusFiles{ProvisionJSONFile: provisionJSONFilePath, ProvisionCompleteFile: provisionCompleteFilePath}
		provisionOutput, waitErr := a.ProvisionWait(ctx, provisionStatusFiles)
		//nolint:forbidigo // stdout is part of the interface
		fmt.Println(provisionOutput)
		slog.Info("provision-wait finished", "provisionOutput", provisionOutput)
		return waitErr
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

// runProvision encapsulates argument parsing and execution for the "provision" subcommand.
// It returns an error describing any failure; callers should pass that error to
// writeCompleteFileOnError so the sentinel file can be written on fail-fast paths.
func (a *App) runProvision(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("provision", flag.ContinueOnError)
	provisionConfig := fs.String("provision-config", "", "path to the provision config file")
	dryRun := fs.Bool("dry-run", false, "print the command that would be run without executing it")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse args: %w", err)
	}
	if *provisionConfig == "" {
		return errors.New("--provision-config is required")
	}
	if *dryRun {
		a.cmdRunner = cmdRunnerDryRun
	}
	return a.Provision(ctx, ProvisionFlags{ProvisionConfig: *provisionConfig})
}

// writeCompleteFileOnError writes the provision.complete sentinel if err is non-nil,
// allowing provision-wait mode to unblock early on fail-fast validation errors.
func (a *App) writeCompleteFileOnError(err error) {
	if err == nil {
		return
	}
	if _, statErr := os.Stat(provisionCompleteFilePath); statErr == nil {
		return // already exists
	} else if !errors.Is(statErr, os.ErrNotExist) { // unexpected error
		slog.Error("failed to stat provision.complete file", "path", provisionCompleteFilePath, "error", statErr)
		return
	}
	if writeErr := os.WriteFile(provisionCompleteFilePath, []byte{}, 0600); writeErr != nil {
		slog.Error("failed to write provision.complete file", "path", provisionCompleteFilePath, "error", writeErr)
	}
}

func (a *App) ProvisionWait(ctx context.Context, filepaths ProvisionStatusFiles) (string, error) {
	if _, err := os.Stat(filepaths.ProvisionCompleteFile); err == nil {
		data, err := os.ReadFile(filepaths.ProvisionJSONFile)
		if err != nil {
			return "", fmt.Errorf("failed to read provision.json: %w. One reason could be that AKSNodeConfig is not properly set", err)
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
	if err = os.MkdirAll(dir, 0755); err != nil { // create the directory if it doesn't exist
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
					return "", fmt.Errorf("failed to read provision.json: %w. One reason could be that AKSNodeConfig is not properly set", err)
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
