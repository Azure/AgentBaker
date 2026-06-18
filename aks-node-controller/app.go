package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/fsnotify/fsnotify"
	"github.com/urfave/cli/v3"
)

func isExpectedDiffCSEVar(key string) bool {
	switch key {
	case "CLOUD_INIT_STATUS_SCRIPT",
		"HYPERKUBE_URL",
		"MCR_REPOSITORY_BASE",
		"BLOCK_OUTBOUND_NETWORK",
		"SKIP_WAAGENT_HOLD":
		return true
	}
	return false
}

type App struct {
	// cmdRun is a function that runs the given command.
	// the goal of this field is to make it easier to test the app by mocking the command runner.
	cmdRun      func(cmd *exec.Cmd) error
	eventLogger *helpers.EventLogger

	// hotfixVersionPath overrides the default hotfix version file location for testing.
	hotfixVersionPath string
	// aptSourcesDir overrides the default APT sources directory for testing.
	aptSourcesDir string
	// nodeCustomDataPath overrides the default nodecustomdata path for testing.
	nodeCustomDataPath string

	// httpProbeClient overrides the HTTP client used by the check-lps connectivity
	// probes for testing. When nil, a default client (short timeout, InsecureSkipVerify)
	// is used. The probe only verifies reachability, not certificate trust, since it
	// runs pre-kubelet before any kube credential or CA trust is established.
	httpProbeClient *http.Client

	// probeLogWriter overrides the destination for the secondary check-lps stdout marker
	// for testing. When nil, os.Stdout is used. The primary channel is the slog log file.
	probeLogWriter io.Writer
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
	ProvisionConfig string
	NBCCmd          string
}

type ProvisionStatusFiles struct {
	ProvisionJSONFile     string
	ProvisionCompleteFile string
}

func (a *App) Run(ctx context.Context, args []string) int {
	cmd := &cli.Command{
		Name:    "aks-node-controller",
		Usage:   "Parse contract and run csecmd",
		Version: Version,
		ExitErrHandler: func(context.Context, *cli.Command, error) {
			// Return errors to the caller so exit codes are derived consistently in one place.
		},
		Action: func(context.Context, *cli.Command) error {
			if len(args) > 1 {
				return fmt.Errorf("unknown command: %s", args[1])
			}
			return errors.New("missing command argument")
		},
		Commands: []*cli.Command{
			{
				Name:  "provision",
				Usage: "Run node provisioning",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "provision-config", Usage: "path to the provision config file"},
					&cli.StringFlag{Name: "nbc-cmd", Usage: "path to the NBC command file"},
					&cli.BoolFlag{Name: "dry-run", Usage: "print the command that would be run without executing it"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return a.runProvisionCommand(ctx, ProvisionFlags{
						ProvisionConfig: cmd.String("provision-config"),
						NBCCmd:          cmd.String("nbc-cmd"),
					}, cmd.Bool("dry-run"))
				},
			},
			{
				Name:  "provision-wait",
				Usage: "Wait for provisioning to complete",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					provisionStatusFiles := ProvisionStatusFiles{
						ProvisionJSONFile:     provisionJSONFilePath,
						ProvisionCompleteFile: provisionCompleteFilePath,
					}
					provisionOutput, err := a.runProvisionWaitCommand(ctx, provisionStatusFiles)
					_, _ = fmt.Fprintln(cmd.Root().Writer, provisionOutput)
					return err
				},
			},
			{
				Name:  "version",
				Usage: "Print the version",
				Action: func(_ context.Context, cmd *cli.Command) error {
					_, _ = fmt.Fprintln(cmd.Root().Writer, Version)
					return nil
				},
			},
			{
				Name:  "download-hotfix",
				Usage: "Download the requested hotfix binary",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if len(cmd.Args().Slice()) > 0 {
						return fmt.Errorf("unexpected download-hotfix arguments: %s", strings.Join(cmd.Args().Slice(), " "))
					}
					return a.runDownloadHotfixCommand(ctx)
				},
			},
			{
				Name:  "check-lps",
				Usage: "Probe apiserver connectivity (ClusterIP via kube-proxy + direct FQDN) pre-kubelet and log results",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "provision-config", Usage: "path to the provision config file (used to source the apiserver FQDN)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					// check-lps is a diagnostic probe and must never block provisioning.
					// Swallow any error and always return nil so the exit code is 0 (fail-open).
					a.runCheckLPSCommand(ctx, cmd.String("provision-config"))
					return nil
				},
			},
		},
	}

	err := cmd.Run(ctx, args)
	return errToExitCode(err)
}

func (a *App) runProvisionCommand(ctx context.Context, flags ProvisionFlags, dryRun bool) error {
	slog.Info("aks-node-controller started", "task", "Provision")

	startTime := time.Now()
	a.eventLogger.LogEvent("Provision", "Starting", helpers.EventLevelInformational, startTime, startTime)
	provisionResult, err := a.runProvision(ctx, flags, dryRun)
	a.writeCompleteFileOnError(provisionResult, err)
	endTime := time.Now()
	if err != nil {
		message := fmt.Sprintf("aks-node-controller exited with error %s", err.Error())
		a.eventLogger.LogEvent("Provision", message, helpers.EventLevelError, startTime, endTime)
		slog.Error("aks-node-controller failed", "error", err)
	} else {
		a.eventLogger.LogEvent("Provision", "Completed", helpers.EventLevelInformational, startTime, endTime)
		slog.Info("aks-node-controller finished successfully.")
	}
	return err
}

func (a *App) runProvisionWaitCommand(ctx context.Context, provisionStatusFiles ProvisionStatusFiles) (string, error) {
	slog.Info("aks-node-controller started", "task", "ProvisionWait")

	startTime := time.Now()
	a.eventLogger.LogEvent("ProvisionWait", "Starting", helpers.EventLevelInformational, startTime, startTime)
	provisionOutput, err := a.ProvisionWait(ctx, provisionStatusFiles)
	endTime := time.Now()
	if err != nil {
		message := fmt.Sprintf("aks-node-controller exited with error %s", err.Error())
		a.eventLogger.LogEvent("ProvisionWait", message, helpers.EventLevelError, startTime, endTime)
		slog.Error("aks-node-controller failed", "error", err)
	} else {
		a.eventLogger.LogEvent("ProvisionWait", "Completed", helpers.EventLevelInformational, startTime, endTime)
		slog.Info("aks-node-controller finished successfully.")
	}
	slog.Info("provision-wait finished", "provisionOutput", provisionOutput)
	return provisionOutput, err
}

func (a *App) runDownloadHotfixCommand(ctx context.Context) error {
	slog.Info("aks-node-controller hotfix download started")
	err := a.downloadHotfix(ctx)
	if err != nil {
		slog.Error("aks-node-controller hotfix download failed", "error", err)
		return err
	}
	slog.Info("aks-node-controller hotfix download finished")
	return nil
}

func (a *App) runCheckLPSCommand(ctx context.Context, provisionConfigPath string) {
	slog.Info("aks-node-controller check-lps started")
	// checkLPS never returns an error (it logs everything and is fail-open), but we
	// defensively log here in case that ever changes so it stays a non-blocking probe.
	if err := a.checkLPS(ctx, provisionConfigPath); err != nil {
		slog.Warn("aks-node-controller check-lps encountered an error (ignored)", "error", err)
	}
	slog.Info("aks-node-controller check-lps finished")
}

func buildCmdFromProvisionConfig(ctx context.Context, path string) (*exec.Cmd, error) {
	inputJSON, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open provision file %s: %w", path, err)
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
		return nil, fmt.Errorf("unsupported version: %s", config.Version)
	}

	if config.Version == "v0" {
		slog.Error("v0 version is deprecated, please use v1 instead")
	}

	return parser.BuildCSECmd(ctx, config)
}

func buildCmdFromNBCCmd(ctx context.Context, path string) (*exec.Cmd, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read NBC command file %s: %w", path, err)
	}
	if !fileInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("NBC command file %s must be a regular file", path)
	}

	scriptPath := filepath.Clean(path)
	slog.Info("Using NBC command for scriptless phase 2", "NBCCmdFile", scriptPath)

	// #nosec G204 -- scriptPath is validated as a file path and passed after "--" so bash treats it as a script, not an option or shell input.
	cmd := exec.CommandContext(ctx, "/bin/bash", "--", scriptPath)
	cmd.Env = os.Environ()
	return cmd, nil
}

func (a *App) getNodeCustomDataPath() string {
	if a.nodeCustomDataPath != "" {
		return a.nodeCustomDataPath
	}
	return defaultNodeCustomDataPath
}

// compareEnvs compares the environment variables between the ProvisionConfig and NBCCmd command paths.
// It logs variables that are only in one environment or that have different values between the two.
// A summary of all differences is also emitted as a guest agent event for Kusto querying.
// This function is best-effort: any error is logged and returned from,
// so it never blocks provisioning.
func compareEnvs(ctx context.Context, flags ProvisionFlags, eventLogger *helpers.EventLogger) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("compareEnvs panicked", "panic", r)
		}
	}()

	provisionConfigCmd, err := buildCmdFromProvisionConfig(ctx, flags.ProvisionConfig)
	if err != nil {
		slog.Error("compareEnvs: failed to build cmd from provision config", "error", err)
		return
	}

	// Extract CSE-specific env vars from provision config by filtering out unmodified OS env vars.
	pcEnv := extractCSEEnvVars(provisionConfigCmd.Env)

	// Parse env vars directly from the NBC command file content.
	nbcCmdContent, err := os.ReadFile(flags.NBCCmd)
	if err != nil {
		slog.Error("compareEnvs: failed to read nbc-cmd file", "error", err)
		return
	}
	nbcEnv := parseEnvVarsFromNBCCmdContent(string(nbcCmdContent))

	diffs := diffEnvMaps(pcEnv, nbcEnv)

	now := time.Now()
	if len(diffs) == 0 {
		slog.Info("env compare: no differences found between provision-config and nbc-cmd env vars")
		eventLogger.LogEvent("CompareEnvs", "env vars match between provision-config and nbc-cmd", helpers.EventLevelInformational, now, now)
	} else {
		message := fmt.Sprintf("env var differences (%d): %s", len(diffs), strings.Join(diffs, "; "))
		slog.Info(message)
		eventLogger.LogEvent("CompareEnvs", message, helpers.EventLevelInformational, now, now)
	}
}

// extractCSEEnvVars filters a command's env slice to only CSE-specific variables
// by removing entries that match the current OS environment.
func extractCSEEnvVars(cmdEnv []string) map[string]string {
	osEnv := envSliceToMap(os.Environ())
	allEnv := envSliceToMap(cmdEnv)
	cseEnv := make(map[string]string, len(allEnv))
	for k, v := range allEnv {
		if osVal, inOS := osEnv[k]; !inOS || osVal != v {
			cseEnv[k] = v
		}
	}
	return cseEnv
}

// diffEnvMaps compares two environment variable maps and returns a sorted list of human-readable differences.
func diffEnvMaps(pcEnv, nbcEnv map[string]string) []string {
	allKeys := make(map[string]struct{}, len(pcEnv)+len(nbcEnv))
	for k := range pcEnv {
		allKeys[k] = struct{}{}
	}
	for k := range nbcEnv {
		allKeys[k] = struct{}{}
	}

	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var diffs []string
	for _, key := range sortedKeys {
		pcVal, inPC := pcEnv[key]
		nbcVal, inNBC := nbcEnv[key]
		switch {
		case inPC && !inNBC:
			diffs = append(diffs, fmt.Sprintf("only-in-pc: %s", key))
		case !inPC && inNBC:
			if !isExpectedDiffCSEVar(key) {
				diffs = append(diffs, fmt.Sprintf("only-in-nbc: %s", key))
			}
		case !envValsEqualForKey(key, pcVal, nbcVal):
			if !isExpectedDiffCSEVar(key) {
				diffs = append(diffs, fmt.Sprintf("differs: %s", key))
			}
		}
	}
	return diffs
}

// envValsEqual compares two environment variable values, treating them as equal
// if they differ only in the presence of double quotes around substrings.
// This handles cases like PROXY_VARS where the legacy path strips inner quotes
// due to shell quoting collision while the scriptless path preserves them.
func envValsEqual(a, b string) bool {
	if a == b {
		return true
	}
	return stripDoubleQuotes(a) == stripDoubleQuotes(b)
}

// envValsEqualForKey performs key-specific comparison logic.
// For SYSCTL_CONTENT, it base64-decodes both values and compares the resulting
// key=value pairs as sets (ignoring order and whitespace differences).
func envValsEqualForKey(key, a, b string) bool {
	if key == "SYSCTL_CONTENT" {
		return sysctlContentEqual(a, b)
	}
	return envValsEqual(a, b)
}

// sysctlContentEqual base64-decodes both values and compares the sysctl key=value
// pairs as sets, ignoring line ordering and trailing whitespace.
func sysctlContentEqual(a, b string) bool {
	aDecoded, errA := base64.StdEncoding.DecodeString(a)
	bDecoded, errB := base64.StdEncoding.DecodeString(b)
	if errA != nil || errB != nil {
		// Fall back to literal comparison if decoding fails.
		return envValsEqual(a, b)
	}
	aSet := parseSysctlPairs(string(aDecoded))
	bSet := parseSysctlPairs(string(bDecoded))
	if len(aSet) != len(bSet) {
		return false
	}
	for k, v := range aSet {
		if bSet[k] != v {
			return false
		}
	}
	return true
}

// parseSysctlPairs parses newline-separated "key = value" or "key=value" entries
// into a map, trimming whitespace from both key and value.
func parseSysctlPairs(content string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result
}

func stripDoubleQuotes(s string) string {
	return strings.ReplaceAll(s, "\"", "")
}

// parseEnvVarsFromNBCCmdContent extracts environment variable assignments from an NBC command string.
// The command is a bash one-liner with KEY=VALUE pairs (quoted or unquoted) interspersed with shell commands.
// Only variables with uppercase/underscore names are extracted.
func parseEnvVarsFromNBCCmdContent(content string) map[string]string {
	result := make(map[string]string)
	n := len(content)
	i := 0

	for i < n {
		// Skip whitespace and semicolons.
		for i < n && isDelimiter(content[i]) {
			i++
		}
		if i >= n {
			break
		}

		// Try to read an uppercase variable name.
		keyStart := i
		if !isEnvKeyStart(content[i]) {
			i = skipToken(content, i)
			continue
		}
		for i < n && isEnvKeyChar(content[i]) {
			i++
		}

		// Must be followed by '='.
		if i >= n || content[i] != '=' {
			i = skipToken(content, i)
			continue
		}

		key := content[keyStart:i]
		i++ // skip '='

		var value string
		value, i = parseEnvValue(content, i)
		result[key] = value
	}

	return result
}

// parseEnvValue parses the value portion of a KEY=VALUE assignment starting at position i.
// It handles concatenated quoted (single or double) and unquoted segments. Returns the parsed value and the new position.
func parseEnvValue(content string, i int) (string, int) {
	n := len(content)
	var value strings.Builder
	for i < n {
		switch {
		case content[i] == '"':
			// Double-quoted section: read until closing double quote.
			i++ // skip opening quote
			for i < n && content[i] != '"' {
				value.WriteByte(content[i])
				i++
			}
			if i < n {
				i++ // skip closing quote
			}
		case content[i] == '\'':
			// Single-quoted section: read until closing single quote.
			i++ // skip opening quote
			for i < n && content[i] != '\'' {
				value.WriteByte(content[i])
				i++
			}
			if i < n {
				i++ // skip closing quote
			}
		case isDelimiter(content[i]):
			return value.String(), i
		default:
			// Before consuming, check if this looks like a new KEY= (handles missing spaces between assignments).
			if looksLikeEnvAssignment(content, i) {
				return value.String(), i
			}
			value.WriteByte(content[i])
			i++
		}
	}
	return value.String(), i
}

func isDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == ';'
}

func isEnvKeyStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || c == '_'
}

func isEnvKeyChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// skipToken advances past the current non-whitespace token, respecting quoted sections.
func skipToken(content string, i int) int {
	n := len(content)
	for i < n && content[i] != ' ' && content[i] != '\t' && content[i] != '\n' && content[i] != ';' {
		switch content[i] {
		case '"':
			i++
			for i < n && content[i] != '"' {
				i++
			}
			if i < n {
				i++
			}
		case '\'':
			i++
			for i < n && content[i] != '\'' {
				i++
			}
			if i < n {
				i++
			}
		default:
			i++
		}
	}
	return i
}

// looksLikeEnvAssignment checks if position i starts a KEY= pattern (at least 2-char uppercase key followed by '=').
func looksLikeEnvAssignment(content string, i int) bool {
	n := len(content)
	if i >= n || !isEnvKeyStart(content[i]) {
		return false
	}
	j := i + 1
	for j < n && isEnvKeyChar(content[j]) {
		j++
	}
	return j < n && content[j] == '=' && j-i >= 1
}

// envSliceToMap converts a slice of "KEY=VALUE" strings into a map.
// For duplicate keys the last value wins, matching exec.Cmd behavior.
func envSliceToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, entry := range env {
		k, v, _ := strings.Cut(entry, "=")
		m[k] = v
	}
	return m
}

func (a *App) Provision(ctx context.Context, flags ProvisionFlags) (*ProvisionResult, error) {
	provisionResult := &ProvisionResult{}

	var cmd *exec.Cmd
	if flags.NBCCmd != "" {
		if err := applyNodeCustomData(a.getNodeCustomDataPath()); err != nil {
			provisionResult.ExitCode = strconv.Itoa(240)
			provisionResult.Error = err.Error()
			return provisionResult, err
		}

		var err error
		cmd, err = buildCmdFromNBCCmd(ctx, flags.NBCCmd)
		if err != nil {
			provisionResult.ExitCode = strconv.Itoa(240)
			provisionResult.Error = err.Error()
			return provisionResult, err
		}
	}

	// If NBC command is provided, we prioritize it over the aks node config for provisioning.
	if flags.ProvisionConfig != "" && flags.NBCCmd == "" {
		var err error
		cmd, err = buildCmdFromProvisionConfig(ctx, flags.ProvisionConfig)
		if err != nil {
			provisionResult.ExitCode = strconv.Itoa(240)
			provisionResult.Error = err.Error()
			return provisionResult, err
		}
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	err := a.cmdRun(cmd)
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	// If both flags are provided, compare environments.
	// This is best-effort and should not block provisioning.
	if flags.ProvisionConfig != "" && flags.NBCCmd != "" {
		slog.Info("ProvisionConfig and NBCCmd both provided, comparing envs")
		compareEnvs(ctx, flags, a.eventLogger)
	}

	slog.Info("CSE finished", "exitCode", exitCode, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "error", err)
	provisionResult.ExitCode = strconv.Itoa(exitCode)
	provisionResult.Error = fmt.Sprintf("%v", err)
	provisionResult.Output = strings.Join([]string{stdoutBuf.String(), stderrBuf.String()}, "\n")
	return provisionResult, err
}

// runProvision encapsulates execution for the "provision" subcommand after CLI parsing.
// It returns an error describing any failure; callers should pass that error to
// writeCompleteFileOnError so the sentinel file can be written on fail-fast paths.
func (a *App) runProvision(ctx context.Context, flags ProvisionFlags, dryRun bool) (*ProvisionResult, error) {
	// Handle panics gracefully to ensure we always return a valid result
	provisionResult := &ProvisionResult{}
	var err error

	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in runProvision", "panic", r)
			provisionResult.ExitCode = strconv.Itoa(240)
			provisionResult.Error = fmt.Sprintf("panic during provisioning: %v", r)
			err = fmt.Errorf("panic during provisioning: %v", r)
			a.writeCompleteFileOnError(provisionResult, err)
		}
	}()

	if flags.ProvisionConfig == "" && flags.NBCCmd == "" {
		provisionResult.ExitCode = strconv.Itoa(240)
		provisionResult.Error = "--provision-config or --nbc-cmd is required"
		return provisionResult, errors.New(provisionResult.Error)
	}
	if dryRun {
		a.cmdRun = cmdRunnerDryRun
	}
	return a.Provision(ctx, flags)
}

// writeCompleteFileOnError writes the provision.complete sentinel if err is non-nil,
// allowing provision-wait mode to unblock early on fail-fast validation errors.
func (a *App) writeCompleteFileOnError(provisionResult *ProvisionResult, err error) {
	if err == nil {
		return
	}
	if _, statErr := os.Stat(provisionJSONFilePath); statErr != nil && errors.Is(statErr, os.ErrNotExist) {
		data, err := json.Marshal(provisionResult)
		if err != nil {
			slog.Error("failed to marshal provision result", "error", err)
		}
		baseDir := filepath.Dir(provisionJSONFilePath)
		if writeErr := os.MkdirAll(baseDir, 0755); writeErr != nil {
			slog.Error("failed to create directory for provision.json file", "path", baseDir, "error", writeErr)
		}
		if writeErr := os.WriteFile(provisionJSONFilePath, data, 0600); writeErr != nil {
			slog.Error("failed to write provision.json file", "path", provisionJSONFilePath, "error", writeErr)
		}
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
