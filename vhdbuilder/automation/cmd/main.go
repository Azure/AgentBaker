package main

import (
	"context"
	"log"

	"github.com/Azure/agentbaker/vhdbuilder/automation/internal/ado"
	"github.com/Azure/agentbaker/vhdbuilder/automation/internal/env"
)

func main() {
	if env.Variables.ADOPAT == "" {
		log.Fatal("expected to find non-empty ADO_PAT from environment")
	}
	if env.Variables.VHDBuildID == "" {
		log.Fatal("expected to find non-empty VHD_BUILD_ID from environment")
	}

	ctx := context.Background()
	adoClient, err := ado.NewClient(ctx, env.Variables.ADOPAT)
	if err != nil {
		log.Fatalf("constructing ADO client: %s", err)
	}

	log.Println("building EV2 artifacts...")
	build, err := adoClient.BuildEV2Artifacts(ctx, env.Variables.VHDBuildID, nil)
	if err != nil {
		log.Fatalf("building EV2 artifacts: %s", err)
	}

	log.Println("creating SIG release...")
	if err := adoClient.CreateSIGRelease(ctx, build); err != nil {
		log.Fatalf("creating SIG release: %s", err)
	}
}
