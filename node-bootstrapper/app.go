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
	"strings"
	"time"

	"github.com/Azure/agentbaker/node-bootstrapper/parser"
	"github.com/Azure/agentbaker/node-bootstrapper/utils"
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
	err := a.run(ctx, args)
	exitCode := errToExitCode(err)
	if exitCode == 0 {
		slog.Info("node-bootstrapper finished")
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
	case "monitor":
		return a.Monitor(ctx)
	default:
		return fmt.Errorf("unknown command: %s", args[1])
	}
}

func (a *App) Provision(ctx context.Context, flags ProvisionFlags) error {
	inputJSON, err := os.ReadFile(flags.ProvisionConfig)
	if err != nil {
		return fmt.Errorf("open proision file %s: %w", flags.ProvisionConfig, err)
	}

	cseCmd, err := parser.Parse(inputJSON)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if err := a.provisionStart(ctx, cseCmd); err != nil {
		return fmt.Errorf("provision start: %w", err)
	}
	return nil
}

func (a *App) provisionStart(ctx context.Context, cse utils.SensitiveString) error {
	// CSEScript can't be logged because it contains sensitive information.
	slog.Info("Running CSE script")
	//nolint:gosec // we generate the script, so it's safe to execute
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", cse.UnsafeValue())
	cmd.Dir = "/"
	var stdoutBuf, stderrBuf bytes.Buffer
	// We want to preserve the original stdout and stderr to avoid any issues during migration to the "scriptless" approach
	// RP may rely on stdout and stderr for error handling
	// it's also nice to have a single log file for all the important information, so write to both places
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	err := a.cmdRunner(cmd)
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	// Is it ok to log a single line? Is it too much?
	slog.Info("CSE finished", "exitCode", exitCode, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "error", err)
	return err
}

// usage example:
// node-bootstrapper monitor
func (a *App) Monitor(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			// If the timeout or cancel occurs, exit with a timeout error
			return fmt.Errorf("monitoring timed out: %s still active after 15 minutes", BootstrapService)
		default:
			// Check the active state of the unit
			_, err := a.runSystemctlCommand(ctx, "status", BootstrapService)

			// if service is inactive or failed, error code will be non-zero
			if err == nil {
				// Unit is still active, sleep for 3 seconds before checking again
				time.Sleep(3 * time.Second)
				continue
			}

			provisionJSON, err := a.getProvisionJSON()
			if err != nil {
				return fmt.Errorf("error getting provision.json output, %w", err)
			}
			// print to stdout so it will be returned by custom script extension
			fmt.Println(provisionJSON)
			return nil
		}
	}
}

// runSystemctlCommand is a generic function that runs a systemctl command with specified arguments
func (a *App) runSystemctlCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getProvisionJSON returns the contents of provision.json containing bootstrap status info
func (a *App) getProvisionJSON() (string, error) {
	// Read the file contents
	data, err := os.ReadFile(ProvisionJSONFilePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
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
