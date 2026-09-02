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
		Name:    "e2e",
		Usage:   "Run AgentBaker end-to-end scenarios",
		Suggest: true,
		Commands: []*cli.Command{
			{
				Name:      "run",
				Usage:     "Run selected E2E scenarios",
				ArgsUsage: "[scenario-name ...]",
				Flags:     config.Flags(),
				Suggest:   true,
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
					scenarios := registeredScenarios()
					sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].Name < scenarios[j].Name })
					for _, scenario := range scenarios {
						_, _ = fmt.Fprintln(a.stdout, scenario.Name)
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

	scenarios := selectScenarios(registeredScenarios(), opts.selectors)
	if len(scenarios) == 0 {
		return &usageError{message: "no scenarios matched the requested names"}
	}
	runnable, filtered, err := partitionScenarios(scenarios, opts.tagFilter)
	if err != nil {
		return err
	}
	if len(runnable) == 0 {
		if err := writeJUnitReport(opts.junitFile, filtered); err != nil {
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
	for _, scenario := range runnable {
		exec.schedule(scenario.Name, scenario)
	}
	waitErr := exec.wait(scenarioCleanupTimeout + 30*time.Second)

	results := exec.snapshotResults(filtered)
	if err := writeJUnitReport(opts.junitFile, results); err != nil {
		return err
	}
	summary := exec.printSummary(results)

	if waitErr != nil {
		return waitErr
	}
	if summary.Failed > 0 {
		return fmt.Errorf("%d scenario(s) failed", summary.Failed)
	}
	return nil
}

func selectScenarios(scenarios []*Scenario, selectors []string) []*Scenario {
	if len(selectors) == 0 {
		return scenarios
	}
	var selected []*Scenario
	for _, scenario := range scenarios {
		for _, selector := range selectors {
			if strings.EqualFold(scenario.Name, selector) || strings.HasPrefix(strings.ToLower(scenario.Name), strings.ToLower(selector)+"/") {
				selected = append(selected, scenario)
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
		tagFilter: tagFilter{
			run:  config.Config.TagsToRun,
			skip: config.Config.TagsToSkip,
		},
		selectors: selectors,
	}
}
