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
	runStarted := false
	cmd := &cli.Command{
		Name:  "e2e",
		Usage: "Run AgentBaker end-to-end scenarios",
		Commands: []*cli.Command{
			{
				Name:      "run",
				Usage:     "Run selected E2E scenarios",
				ArgsUsage: "[scenario-name ...]",
				Flags:     config.Flags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					runStarted = true
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
		Action: func(_ context.Context, cmd *cli.Command) error {
			return cli.ShowRootCommandHelp(cmd)
		},
		Writer:    a.stdout,
		ErrWriter: a.stderr,
	}

	if err := cmd.Run(ctx, args); err != nil {
		_, _ = fmt.Fprintln(a.stderr, err)
		if !runStarted {
			return exitUsage
		}
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
	if config.Config.TestTimeout <= 0 {
		return &usageError{message: "--timeout must be greater than zero"}
	}
	if config.Config.SuiteTimeout <= 0 {
		return &usageError{message: "--suite-timeout must be greater than zero"}
	}
	if config.Config.TestTimeoutCluster <= 0 {
		return &usageError{message: "--cluster-timeout must be greater than zero"}
	}
	if config.Config.TestTimeoutVMSS <= 0 {
		return &usageError{message: "--vmss-timeout must be greater than zero"}
	}
	if config.Config.DefaultPollInterval <= 0 {
		return &usageError{message: "--poll-interval must be greater than zero"}
	}
	switch opts.outputMode {
	case "auto", "grouped", "stream":
	default:
		return &usageError{message: "--output must be auto, grouped, or stream"}
	}

	entries := selectEntries(registeredScenarios(), opts.selectors)
	if len(entries) == 0 {
		return &usageError{message: "no scenarios matched the requested names"}
	}
	for i := range entries {
		if config.Config.TestPreProvision || entries[i].scenario.VHDCaching {
			entries[i].name += "/VHDCreation"
		}
	}
	runnable, filtered, err := partitionEntries(entries, opts.tagFilter)
	if err != nil {
		return err
	}
	if len(runnable) == 0 {
		if err := newExecutor(ctx, a.stdout, opts, 0).writeReports(filtered); err != nil {
			return err
		}
		return &usageError{message: "no scenarios matched the configured filters"}
	}

	ctx, cancel := context.WithTimeout(ctx, config.Config.SuiteTimeout)
	defer cancel()

	if err := config.Initialize(); err != nil {
		return fmt.Errorf("initialize E2E configuration: %w", err)
	}
	ctrruntimelog.SetLogger(zap.New())

	if err := os.MkdirAll(opts.logDir, 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	log.Printf("using E2E environment configuration:\n%s\n", config.Config)

	exec := newExecutor(ctx, a.stdout, opts, len(runnable))
	for _, entry := range runnable {
		exec.schedule(entry.name, entry.scenario)
	}
	waitErr := exec.wait(scenarioCleanupTimeout + 30*time.Second)

	if err := exec.writeReports(filtered); err != nil {
		return err
	}
	summary := exec.printSummary()

	if waitErr != nil {
		return waitErr
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
	retries    int
	logDir     string
	junitFile  string
	outputMode string
	hidePassed bool
	tagFilter  tagFilter
	selectors  []string
}

func runOptionsFromConfig(selectors []string) runOptions {
	return runOptions{
		parallel:   config.Config.Parallel,
		retries:    config.Config.Retries,
		logDir:     config.Config.E2ELoggingDir,
		junitFile:  config.Config.JUnitFile,
		outputMode: config.Config.OutputMode,
		hidePassed: config.Config.HidePassedLogs,
		tagFilter: tagFilter{
			run:  config.Config.TagsToRun,
			skip: config.Config.TagsToSkip,
		},
		selectors: selectors,
	}
}
