//This has been generated using akservice version: v0.0.1
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Azure/agentbaker/apiserver"
	"github.com/spf13/cobra"
)

func main() {
	Execute()
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVar(&options.Addr, "addr", ":8080", "the addr to serve the api on")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "agentbaker",
	Short: "Agent baker is responsible for generating all the data necessary to allow Nodes to join an AKS cluster.",
}

var (
	options = &apiserver.Options{}
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the server that hosts agentbaker",
	Run:   startHelper,
}

func startHelper(cmd *cobra.Command, args []string) {
	ctx, shutdown := context.WithCancel(context.Background())
	defer shutdown()

	// setup signal handling to cancel the context
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		<-signals
		log.Println("received SIGTERM. Terminating...")
		shutdown()
	}()

	api, err := apiserver.NewAPIServer(options)
	if err != nil {
		log.Fatal(ctx, err.Error())
	}

	errorPipeline := make(chan error)

	go func() {
		log.Printf("Serving API on %s\n", options.Addr)
		errorPipeline <- api.ListenAndServe(ctx)
	}()

	select {
	case <-ctx.Done():
		return
	case err := <-errorPipeline:
		log.Fatal(ctx, err.Error())
	}
}
