// This has been generated using akservice version: v0.0.1.
package starter

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Azure/agentbaker/apiserver"
	"github.com/spf13/cobra"
)

// The log collector routes JSON stdout lines to Kusto tables by "source". init runs before
// the importing main, so its early log calls are JSON too. SetDefault also redirects the
// standard library log package.
//
//nolint:gochecknoinits // logging must be configured before any log call.
func init() {
	slog.SetDefault(apiserver.NewJSONLogger(os.Stdout, apiserver.ContextlessTraceSource))
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(configurators ...apiserver.OptionConfigurator) {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVar(&options.Addr, "addr", ":8080", "the addr to serve the api on")

	for _, configurator := range configurators {
		configurator(options)
	}

	if err := rootCmd.Execute(); err != nil {
		slog.Error("execute agentbaker command", "error", err)
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "agentbaker",
	Short: "Agent baker is responsible for generating all the data necessary to allow Nodes to join an AKS cluster.",
}

var (
	options = &apiserver.Options{
		RequestLogger: apiserver.NewJSONLogger(os.Stdout, apiserver.IncomingRequestTraceSource),
	}
)

// startCmd represents the start command.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the server that hosts agentbaker",
	Run: func(cmd *cobra.Command, args []string) {
		err := startHelper(cmd, args)
		if err != nil {
			slog.Error("start agentbaker", "error", err)
			os.Exit(1)
		}
	},
}

func startHelper(_ *cobra.Command, _ []string) error {
	ctx, shutdown := context.WithCancel(context.Background())
	defer shutdown()

	// setup signal handling to cancel the context
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		<-signals
		slog.Info("received SIGTERM, terminating")
		shutdown()
	}()

	api, err := apiserver.NewAPIServer(options)
	if err != nil {
		slog.Error("create APIServer", "error", err)
		return err
	}

	errorPipeline := make(chan error)

	go func() {
		slog.Info("serving API", "addr", options.Addr)
		errorPipeline <- api.ListenAndServe(ctx)
	}()

	select {
	case <-ctx.Done():
		return nil
	case pipelineErr := <-errorPipeline:
		return pipelineErr
	}
}
