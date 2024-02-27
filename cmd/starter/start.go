// This has been generated using akservice version: v0.0.1.
package starter

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Azure/agentbaker/apiserver"
	agentoverrides "github.com/Azure/agentbaker/pkg/agent/overrides"
	"github.com/spf13/cobra"
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVar(&options.Addr, "addr", ":8080", "the addr to serve the api on")
	startCmd.Flags().StringVar(&serviceOverridesDir, "overrides-dir", "", "the directory containing agentbakersvc override definitions")

	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands.
//
//nolint:gochecknoglobals
var rootCmd = &cobra.Command{
	Use:   "agentbaker",
	Short: "Agent baker is responsible for generating all the data necessary to allow Nodes to join an AKS cluster.",
}

//nolint:gochecknoglobals
var (
	options             = &apiserver.Options{}
	serviceOverridesDir string
)

// startCmd represents the start command.
//
//nolint:gochecknoglobals
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the server that hosts agentbaker",
	Run: func(cmd *cobra.Command, args []string) {
		err := startHelper(cmd, args)
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
	},
}

func startHelper(_ *cobra.Command, _ []string) error {
	ctx, shutdown := context.WithCancel(context.Background())
	defer shutdown()

	serviceOverrides := agentoverrides.NewOverrides()
	if serviceOverridesDir != "" {
		var err error
		serviceOverrides, err = agentoverrides.ReadDir(serviceOverridesDir)
		if err != nil {
			// should we fail hard here or try to continue with startup?
			log.Println(ctx, err.Error())
		}
	}

	// setup signal handling to cancel the context
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		<-signals
		log.Println("received SIGTERM. Terminating...")
		shutdown()
	}()

	api, err := apiserver.NewAPIServer(serviceOverrides, options)
	if err != nil {
		log.Println(ctx, err.Error())
		return err
	}

	errorPipeline := make(chan error)

	go func() {
		log.Printf("Serving API on %s\n", options.Addr)
		errorPipeline <- api.ListenAndServe(ctx)
	}()

	select {
	case <-ctx.Done():
		return nil
	case pipelineErr := <-errorPipeline:
		return pipelineErr
	}
}
