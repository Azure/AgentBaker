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
		signal.Reset(os.Interrupt, syscall.SIGTERM)
	}()

	os.Exit(e2e.NewApp(os.Stdout, os.Stderr).Run(ctx, os.Args))
}
