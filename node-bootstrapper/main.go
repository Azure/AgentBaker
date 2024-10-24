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
	"github.com/Azure/agentbaker/node-bootstrapper/parser"
	"github.com/Azure/agentbaker/node-bootstrapper/utils"
)

// Some options are intentionally non-configurable to avoid customization by users
// it will help us to avoid introducing any breaking changes in the future.
const (
	LogFile          = "/var/log/azure/node-bootstrapper.log"
	BootstrapService = "bootstrap.service"
)

func main() {
	logFile, err := os.OpenFile(LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		//nolint:forbidigo // there is no other way to communicate the error
		fmt.Printf("failed to open log file: %s\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(logFile, nil))
	slog.SetDefault(logger)
	slog.Info("node-bootstrapper started")

	ctx := context.Background()
	err = Run(ctx)
	exitCode := utils.ErrToExitCode(err)

	if exitCode == 0 {
		slog.Info("node-bootstrapper finished successfully")
	} else {
		slog.Error("node-bootstrapper finished with error", "error", err.Error())
	}

	_ = logFile.Close()
	os.Exit(exitCode)
}

func Run(ctx context.Context) error {
	const minNumberArgs = 2
	if len(os.Args) < minNumberArgs {
		return errors.New("missing command argument")
	}
	switch os.Args[1] {
	case "provision":
		return Provision(ctx)
	case "monitor":
		return Monitor(ctx)
	default:
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
}

// usage example:
// node-bootstrapper monitor
func Monitor(ctx context.Context) error {
	for {
		// Check the active state of the unit
		unitStatus, err := runSystemctlCommand(ctx, "is-active", BootstrapService)
		if err != nil {
			return fmt.Errorf("systemctl is-active %s failed with %w", BootstrapService, err)
		}

		// Check if the unit has completed
		if unitStatus == "inactive" || unitStatus == "failed" || unitStatus == "active" {
			exitStatus, err := runSystemctlCommand(ctx, "show", BootstrapService, "-p", "ExecMainStatus", "--value")
			if err != nil {
				return fmt.Errorf("systemctl show %s -p ExecMainStatus --value failed with %w", BootstrapService, err)
			}

			statusOutput, err := runSystemctlCommand(ctx, "status", BootstrapService)
			if err != nil {
				return fmt.Errorf("systemctl status %s failed with %w", BootstrapService, err)
			}

			// Convert exitStatus to an integer for exit code
			exitCode := -1
			fmt.Sscanf(exitStatus, "%d", &exitCode)
			err = &utils.CustomExitError{
				Code: exitCode,
				Msg:  statusOutput,
			}
			return err
		}

		// Sleep for 3 seconds before checking again
		time.Sleep(3 * time.Second)
	}
}

// usage example:
// node-bootstrapper provision --provision-config=config.json .
func Provision(ctx context.Context) error {
	fs := flag.NewFlagSet("provision", flag.ContinueOnError)
	provisionConfig := fs.String("provision-config", "", "path to the provision config file")
	err := fs.Parse(os.Args[2:])
	if err != nil {
		return fmt.Errorf("parse args: %w", err)
	}
	if provisionConfig == nil || *provisionConfig == "" {
		return errors.New("--provision-config is required")
	}

	inputJSON, err := os.ReadFile(*provisionConfig)
	if err != nil {
		return fmt.Errorf("open provision file %s: %w", *provisionConfig, err)
	}

	cseCmd, err := parser.Parse(inputJSON)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if err := provisionStart(ctx, cseCmd); err != nil {
		return fmt.Errorf("provision start: %w", err)
	}
	return nil
}

func provisionStart(ctx context.Context, cse utils.SensitiveString) error {
	// CSEScript can't be logged because it contains sensitive information.
	slog.Info("Running CSE script")
	// TODO: add Windows support
	//nolint:gosec // we generate the script, so it's safe to execute
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", cse.UnsafeValue())
	cmd.Dir = "/"
	var stdoutBuf, stderrBuf bytes.Buffer
	// We want to preserve the original stdout and stderr to avoid any issues during migration to the "scriptless" approach
	// RP may rely on stdout and stderr for error handling
	// it's also nice to have a single log file for all the important information, so write to both places
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	err := cmd.Run()
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	// Is it ok to log a single line? Is it too much?
	slog.Info("CSE finished", "exitCode", exitCode, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "error", err)
	return err
}

// runSystemctlCommand is a generic function that runs a systemctl command with specified arguments
func runSystemctlCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
