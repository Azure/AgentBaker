package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"

	"github.com/Azure/agentbaker/node-bootstrapper/parser"
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

type App struct {
	// cmdRunner is a function that runs the given command.
	// the goal of this field is to make it easier to test the app by mocking the command runner.
	cmdRunner func(cmd *exec.Cmd) error
}

func cmdRunner(cmd *exec.Cmd) error {
	return cmd.Run()
}

type ProvisionFlags struct {
	ProvisionConfig string
}

func (a *App) Run(ctx context.Context, args []string) int {
	slog.Info("node-bootstrapper started")
	err := a.run(ctx, args)
	exitCode := errToExitCode(err)
	if exitCode == 0 {
		slog.Info("node-bootstrapper finished successfully")
	} else {
		slog.Error("node-bootstrapper failed", "error", err)
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
		err := fs.Parse(args[2:])
		if err != nil {
			return fmt.Errorf("parse args: %w", err)
		}
		if provisionConfig == nil || *provisionConfig == "" {
			return errors.New("--provision-config is required")
		}
		return a.Provision(ctx, ProvisionFlags{ProvisionConfig: *provisionConfig})
	default:
		return fmt.Errorf("unknown command: %s", args[1])
	}
}

func (a *App) Provision(ctx context.Context, flags ProvisionFlags) error {
	inputJSON, err := os.ReadFile(flags.ProvisionConfig)
	if err != nil {
		return fmt.Errorf("open proision file %s: %w", flags.ProvisionConfig, err)
	}

	config := &nbcontractv1.Configuration{}
	err = json.Unmarshal(inputJSON, config)
	if err != nil {
		return fmt.Errorf("unmarshal provision config: %w", err)
	}
	if config.Version != "v0" {
		return fmt.Errorf("unsupported version: %s", config.Version)
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
	// Is it ok to log a single line? Is it too much?
	slog.Info("CSE finished", "exitCode", exitCode, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "error", err)
	return err
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
