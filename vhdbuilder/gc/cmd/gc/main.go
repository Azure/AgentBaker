package main

import (
	"context"
	"log"

	"github.com/Azure/agentbaker/vhdbuilder/gc/internal/azure"
	"github.com/Azure/agentbaker/vhdbuilder/gc/internal/gc"
)

func main() {
	ctx := context.Background()

	azureClient, err := azure.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	if err := gc.CollectResourceGroups(ctx, azureClient); err != nil {
		log.Fatal(err)
	}
}
