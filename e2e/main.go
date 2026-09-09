package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Azure/agentbaker/e2e"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		// The first signal starts graceful cleanup; a second signal uses the OS
		// default handler to terminate immediately.
		signal.Reset(os.Interrupt, syscall.SIGTERM)
	}()

	os.Exit(e2e.NewApp(os.Stdout, os.Stderr).Run(ctx, os.Args))
}
