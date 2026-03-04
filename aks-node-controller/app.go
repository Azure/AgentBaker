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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/fsnotify/fsnotify"
)

type App struct {
	// cmdRun is a function that runs the given command.
	// the goal of this field is to make it easier to test the app by mocking the command runner.
	cmdRun      func(cmd *exec.Cmd) error
	eventLogger *helpers.EventLogger
}

// commandMetadata holds all metadata for a command in one place.
type commandMetadata struct {
	taskName string
	handler  func(*App, context.Context, []string) (func(), error)
}

// getCommandRegistry returns the command registry mapping command names to their metadata.
// Adding a new command only requires adding one entry here.
func getCommandRegistry() map[string]commandMetadata {
	return map[string]commandMetadata{
		"provision": {
			taskName: "Provision",
			handler: func(a *App, ctx context.Context, args []string) (func(), error) {
				flags, cleanup, err := parseProvisionFlags(args)
				if err != nil {
					return cleanup, err
				}
				if a.eventLogger == nil {
					a.eventLogger = helpers.NewEventLogger(flags.EventsDir)
				}
				if flags.DryRun {
					a.cmdRun = cmdRunnerDryRun
				}
				provisionResult, err := a.Provision(ctx, flags)
				// Always notify after provisioning attempt (success is a no-op inside notifier)
				a.writeCompleteFileOnError(flags.ProvisionStatusFiles, provisionResult, err)
				return cleanup, err
			},
		},
		"provision-wait": {
			taskName: "ProvisionWait",
			handler: func(a *App, ctx context.Context, args []string) (func(), error) {
				flags, cleanup, err := parseProvisionWaitFlags(args)
				if err != nil {
					return cleanup, err
				}
				if a.eventLogger == nil {
					a.eventLogger = helpers.NewEventLogger(flags.EventsDir)
				}
				provisionOutput, err := a.ProvisionWait(ctx, flags.ProvisionStatusFiles)
				//nolint:forbidigo // stdout is part of the interface
				fmt.Println(provisionOutput)
				slog.Info("provision-wait finished", "provisionOutput", provisionOutput)
				return cleanup, err
			},
		},
	}
}

// provision.json values are emitted as strings by the shell jq invocation.
// We only care about ExitCode + Error + Output (snippet) for failure detection.
type ProvisionResult struct {
	ExitCode string `json:"ExitCode"`
	Error    string `json:"Error"`
	Output   string `json:"Output"`
}

func cmdRunner(cmd *exec.Cmd) error {
	return cmd.Run()
}
func cmdRunnerDryRun(cmd *exec.Cmd) error {
	slog.Info("dry-run", "cmd", cmd.String())
	return nil
}

type ProvisionFlags struct {
	ProvisionConfig      string
	DryRun               bool
	EventsDir            string
	ProvisionStatusFiles ProvisionStatusFiles
}

type ProvisionWaitFlags struct {
	EventsDir            string
	ProvisionStatusFiles ProvisionStatusFiles
}

type ProvisionStatusFiles struct {
	ProvisionJSONFile     string
	ProvisionCompleteFile string
}

func parseProvisionFlags(args []string) (ProvisionFlags, func(), error) {
	fs := flag.NewFlagSet("provision", flag.ContinueOnError)
	provisionConfig := fs.String("provision-config", "", "path to the provision config file")
	dryRun := fs.Bool("dry-run", false, "print the command that would be run without executing it")
	logPath := fs.String("log-path", defaultLogPath, "path to the log file")
	eventsDir := fs.String("events-dir", defaultEventsDir, "path to the events directory")
	provisionJSONFile := fs.String("provision-json-file", defaultProvisionJSONFilePath, "path to the provision.json status file")
	provisionCompleteFile := fs.String("provision-complete-file", defaultProvisionCompleteFilePath, "path to the provision.complete sentinel file")

	noop := func() {}
	if err := fs.Parse(args); err != nil {
		return ProvisionFlags{}, noop, fmt.Errorf("parse args: %w", err)
	}
	if *provisionConfig == "" {
		return ProvisionFlags{}, noop, errors.New("--provision-config is required")
	}

	cleanup := configureLogging(*logPath)

	return ProvisionFlags{
		ProvisionConfig: *provisionConfig,
		DryRun:          *dryRun,
		EventsDir:       *eventsDir,
		ProvisionStatusFiles: ProvisionStatusFiles{
			ProvisionJSONFile:     *provisionJSONFile,
			ProvisionCompleteFile: *provisionCompleteFile,
		},
	}, cleanup, nil
}

func parseProvisionWaitFlags(args []string) (ProvisionWaitFlags, func(), error) {
	fs := flag.NewFlagSet("provision-wait", flag.ContinueOnError)
	logPath := fs.String("log-path", defaultLogPath, "path to the log file")
	eventsDir := fs.String("events-dir", defaultEventsDir, "path to the events directory")
	provisionJSONFile := fs.String("provision-json-file", defaultProvisionJSONFilePath, "path to the provision.json status file")
	provisionCompleteFile := fs.String("provision-complete-file", defaultProvisionCompleteFilePath, "path to the provision.complete sentinel file")

	noop := func() {}
	if err := fs.Parse(args); err != nil {
		return ProvisionWaitFlags{}, noop, fmt.Errorf("parse args: %w", err)
	}

	cleanup := configureLogging(*logPath)

	return ProvisionWaitFlags{
		EventsDir: *eventsDir,
		ProvisionStatusFiles: ProvisionStatusFiles{
			ProvisionJSONFile:     *provisionJSONFile,
			ProvisionCompleteFile: *provisionCompleteFile,
		},
	}, cleanup, nil
}

func (a *App) Run(ctx context.Context, args []string) int {
	slog.Info("aks-node-controller started", "args", args)
	err := a.run(ctx, args)
	exitCode := errToExitCode(err)
	if exitCode == 0 {
		slog.Info("aks-node-controller finished successfully.")
	} else {
		slog.Error("aks-node-controller failed", "error", err)
	}
	return exitCode
}

func (a *App) run(ctx context.Context, args []string) error {
	command := ""
	if len(args) >= 2 {
		command = args[1]
	}
	if command == "" {
		return errors.New("missing command argument")
	}

	cmd, ok := getCommandRegistry()[command]
	if !ok {
		return fmt.Errorf("unknown command: %s", command)
	}

	startTime := time.Now()

	cleanup, err := func() (func(), error) {
		return cmd.handler(a, ctx, args[2:])
	}()
	if cleanup != nil {
		defer cleanup()
	}

	endTime := time.Now()
	if a.eventLogger != nil {
		a.eventLogger.LogEvent(cmd.taskName, "Starting", helpers.EventLevelInformational, startTime, startTime)
		if err != nil {
			message := fmt.Sprintf("aks-node-controller exited with error %s", err.Error())
			a.eventLogger.LogEvent(cmd.taskName, message, helpers.EventLevelError, startTime, endTime)
		} else {
			a.eventLogger.LogEvent(cmd.taskName, "Completed", helpers.EventLevelInformational, startTime, endTime)
		}
	}
	return err
}

func (a *App) Provision(ctx context.Context, flags ProvisionFlags) (*ProvisionResult, error) {
	provisionResult := &ProvisionResult{}
	inputJSON, err := os.ReadFile(flags.ProvisionConfig)
	if err != nil {
		provisionResult.ExitCode = strconv.Itoa(240)
		provisionResult.Error = fmt.Sprintf("open provision file %s: %v", flags.ProvisionConfig, err)
		return provisionResult, errors.New(provisionResult.Error)
	}

	config, err := nodeconfigutils.UnmarshalConfigurationV1(inputJSON)
	if err != nil {
		// We try our best to continue unmarshal even if there are unexpected situations such as unknown fields.
		// It usually happens when a newer version of aksNodeConfig is being parsed by an older version of aks-node-controller.
		// This allows older versions of aks-node-controller to read configurations that may have fields added in newer versions.
		// Log the error and continue processing.
		// Feature owner should be aware that any unrecognized fields will be ignored in older versions of VHD image.

		slog.Info("Unmarshalling aksNodeConfigv1 encounters error but the process will continue."+
			"This may be due to version mismatch. "+
			"Usually it is newer aksNodeConfig being parsed by older aks-node-controller. "+
			"Continuing with partial configuration, but unrecognized fields will be ignored.",
			"error", err)
	}
	// TODO: "v0" were a mistake. We are not going to have different logic maintaining both v0 and v1
	// Disallow "v0" after some time (allow some time to update consumers)
	if config.Version != "v0" && config.Version != "v1" {
		provisionResult.ExitCode = strconv.Itoa(240)
		provisionResult.Error = fmt.Sprintf("unsupported version: %s", config.Version)
		return provisionResult, errors.New(provisionResult.Error)
	}

	if config.Version == "v0" {
		slog.Error("v0 version is deprecated, please use v1 instead")
	}

	cmd, err := parser.BuildCSECmd(ctx, config)
	if err != nil {
		provisionResult.ExitCode = strconv.Itoa(240)
		provisionResult.Error = fmt.Sprintf("build CSE command: %v", err)
		return provisionResult, errors.New(provisionResult.Error)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	err = a.cmdRun(cmd)
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	slog.Info("CSE finished", "exitCode", exitCode, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "error", err)
	provisionResult.ExitCode = strconv.Itoa(exitCode)
	provisionResult.Error = fmt.Sprintf("%v", err)
	provisionResult.Output = strings.Join([]string{stdoutBuf.String(), stderrBuf.String()}, "\n")
	return provisionResult, err
}

// writeCompleteFileOnError writes the provision.complete sentinel if err is non-nil,
// allowing provision-wait mode to unblock early on fail-fast validation errors.
func (a *App) writeCompleteFileOnError(filepaths ProvisionStatusFiles, provisionResult *ProvisionResult, err error) {
	if err == nil {
		return
	}
	if _, statErr := os.Stat(filepaths.ProvisionJSONFile); statErr != nil && errors.Is(statErr, os.ErrNotExist) {
		data, err := json.Marshal(provisionResult)
		if err != nil {
			slog.Error("failed to marshal provision result", "error", err)
		}
		baseDir := filepath.Dir(filepaths.ProvisionJSONFile)
		if writeErr := os.MkdirAll(baseDir, 0755); writeErr != nil {
			slog.Error("failed to create directory for provision.json file", "path", baseDir, "error", writeErr)
		}
		if writeErr := os.WriteFile(filepaths.ProvisionJSONFile, data, 0600); writeErr != nil {
			slog.Error("failed to write provision.json file", "path", filepaths.ProvisionJSONFile, "error", writeErr)
		}
	}
	if _, statErr := os.Stat(filepaths.ProvisionCompleteFile); statErr == nil {
		return // already exists
	} else if !errors.Is(statErr, os.ErrNotExist) { // unexpected error
		slog.Error("failed to stat provision.complete file", "path", filepaths.ProvisionCompleteFile, "error", statErr)
		return
	}
	if writeErr := os.WriteFile(filepaths.ProvisionCompleteFile, []byte{}, 0600); writeErr != nil {
		slog.Error("failed to write provision.complete file", "path", filepaths.ProvisionCompleteFile, "error", writeErr)
	}
}

func (a *App) ProvisionWait(ctx context.Context, filepaths ProvisionStatusFiles) (string, error) {
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

	if _, statErr := os.Stat(filepaths.ProvisionCompleteFile); statErr == nil {
		// Fast path: provision.complete already exists when we enter. Avoid watcher overhead.
		// We read and evaluate once and return immediately. Only this branch executes in this scenario.
		return readAndEvaluateProvision(filepaths.ProvisionJSONFile)
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create && event.Name == filepaths.ProvisionCompleteFile {
				// Event path: provision.complete was created after we started watching. Read and evaluate now.
				// This is mutually exclusive with the fast path above; only one of these calls runs per invocation.
				return readAndEvaluateProvision(filepaths.ProvisionJSONFile)
			}

		case err := <-watcher.Errors:
			return "", fmt.Errorf("error watching file: %w", err)
		case <-ctx.Done():
			return "", fmt.Errorf("context deadline exceeded waiting for provision complete: %w", ctx.Err())
		}
	}
}

// evaluateProvisionStatus inspects the serialized CSEStatus (provision.json contents).
// If ExitCode is non-zero we return an error so that provision-wait exits with a failure code.
// We still surface the full JSON on stdout (handled by caller) for diagnostics.
func evaluateProvisionStatus(data []byte) error {
	var result ProvisionResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse provision.json: %w", err)
	}
	if result.ExitCode == "" {
		return fmt.Errorf("missing ExitCode in provision.json")
	}
	code, err := strconv.Atoi(result.ExitCode)
	if err != nil {
		return fmt.Errorf("invalid ExitCode in provision.json: %s", result.ExitCode)
	}
	if code != 0 {
		outSnippet := result.Output
		return fmt.Errorf("provision failed: exitCode=%d error=%s output=%q", code, result.Error, outSnippet)
	}
	return nil
}

// readAndEvaluateProvision reads provision.json content from the given path and evaluates its status.
// It returns the raw JSON string plus any error derived from its parsed ExitCode.
func readAndEvaluateProvision(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read provision.json: %w. One reason could be that AKSNodeConfig is not properly set", err)
	}
	return string(data), evaluateProvisionStatus(data)
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
