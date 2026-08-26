package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/urfave/cli/v3"
	ctrruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

type App struct {
	stdout io.Writer
	stderr io.Writer
}

func NewApp(stdout, stderr io.Writer) *App {
	return &App{stdout: stdout, stderr: stderr}
}

func (a *App) Run(ctx context.Context, args []string) int {
	if err := config.LoadDotEnv(); err != nil {
		_, _ = fmt.Fprintln(a.stderr, err)
		return exitFailure
	}
	var cmd *cli.Command
	cmd = &cli.Command{
		Name:  "e2e",
		Usage: "Run AgentBaker end-to-end scenarios",
		Commands: []*cli.Command{
			{
				Name:      "run",
				Usage:     "Run selected E2E scenarios",
				ArgsUsage: "[scenario-name ...]",
				Flags:     config.Flags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					opts := runOptionsFromConfig(cmd.Args().Slice())
					return a.run(ctx, opts)
				},
			},
			{
				Name:  "list",
				Usage: "List registered E2E scenario entry points",
				Action: func(context.Context, *cli.Command) error {
					entries := registeredScenarios()
					sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
					for _, entry := range entries {
						_, _ = fmt.Fprintln(a.stdout, entry.name)
					}
					return nil
				},
			},
		},
		Action: func(context.Context, *cli.Command) error {
			return cli.ShowRootCommandHelp(cmd)
		},
		Writer:    a.stdout,
		ErrWriter: a.stderr,
	}

	if err := cmd.Run(ctx, args); err != nil {
		_, _ = fmt.Fprintln(a.stderr, err)
		var usageErr *usageError
		if errors.As(err, &usageErr) {
			return exitUsage
		}
		return exitFailure
	}
	return exitSuccess
}

func (a *App) run(ctx context.Context, opts runOptions) error {
	if opts.parallel < 1 {
		return &usageError{message: "--parallel must be at least 1"}
	}
	if opts.retries < 0 {
		return &usageError{message: "--retries must not be negative"}
	}
	if opts.timeout <= 0 {
		return &usageError{message: "--timeout must be greater than zero"}
	}
	if config.Config.SuiteTimeout <= 0 {
		return &usageError{message: "--suite-timeout must be greater than zero"}
	}
	ctx, cancel := context.WithTimeout(ctx, config.Config.SuiteTimeout)
	defer cancel()

	switch opts.outputMode {
	case "auto", "grouped", "stream":
	default:
		return &usageError{message: "--output must be auto, grouped, or stream"}
	}

	entries := selectEntries(registeredScenarios(), opts.selectors)
	if len(entries) == 0 {
		return &usageError{message: "no scenarios matched the requested names"}
	}
	if err := config.Initialize(); err != nil {
		return fmt.Errorf("initialize E2E configuration: %w", err)
	}
	ctrruntimelog.SetLogger(zap.New())

	if err := os.MkdirAll(opts.logDir, 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	log.Printf("using E2E environment configuration:\n%s\n", config.Config)

	exec := newExecutor(ctx, a.stdout, opts, tagSelectedEntries(entries))
	for _, entry := range entries {
		name := entry.name
		if config.Config.TestPreProvision || entry.scenario.VHDCaching {
			name += "/VHDCreation"
		}
		exec.schedule(name, entry.scenario)
	}
	exec.scenarios.Wait()

	summary := exec.summary()
	if err := exec.writeReports(); err != nil {
		return err
	}
	exec.printSummary()

	if summary.Selected == 0 {
		return &usageError{message: "no scenarios matched the configured filters"}
	}
	if summary.Failed > 0 {
		return fmt.Errorf("%d scenario(s) failed", summary.Failed)
	}
	return nil
}

func selectEntries(entries []scenarioEntry, selectors []string) []scenarioEntry {
	if len(selectors) == 0 {
		return entries
	}
	var selected []scenarioEntry
	for _, entry := range entries {
		for _, selector := range selectors {
			if strings.EqualFold(entry.name, selector) || strings.HasPrefix(strings.ToLower(entry.name), strings.ToLower(selector)+"/") {
				selected = append(selected, entry)
				break
			}
		}
	}
	return selected
}

type usageError struct {
	message string
}

func (e *usageError) Error() string {
	return e.message
}

type runOptions struct {
	parallel   int
	timeout    time.Duration
	retries    int
	logDir     string
	junitFile  string
	outputMode string
	hidePassed bool
	selectors  []string
}

func runOptionsFromConfig(selectors []string) runOptions {
	return runOptions{
		parallel:   config.Config.Parallel,
		timeout:    config.Config.TestTimeout,
		retries:    config.Config.Retries,
		logDir:     config.Config.E2ELoggingDir,
		junitFile:  config.Config.JUnitFile,
		outputMode: config.Config.OutputMode,
		hidePassed: config.Config.HidePassedLogs,
		selectors:  selectors,
	}
}
