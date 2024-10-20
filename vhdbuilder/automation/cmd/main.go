package main

import (
	"context"
	"log"
	"os"

	"github.com/Azure/agentbaker/vhdbuilder/automation/internal/ado"
)

const (
	envKeyADOPAT     = "ADO_PAT"
	envKeyVHDBuildID = "VHD_BUILD_ID"
)

func main() {
	pat := os.Getenv(envKeyADOPAT)
	if pat == "" {
		log.Fatal("expected to find non-empty ADO_PAT from environment")
	}
	vhdBuildID := os.Getenv(envKeyVHDBuildID)
	if vhdBuildID == "" {
		log.Fatal("expected to find non-empty VHD_BUILD_ID from environment")
	}

	ctx := context.Background()
	adoClient, err := ado.NewClient(ctx, pat)
	if err != nil {
		log.Fatalf("constructing ADO client: %s", err)
	}

	log.Println("building EV2 artifacts...")
	build, err := adoClient.BuildEV2Artifacts(ctx, vhdBuildID)
	if err != nil {
		log.Fatalf("building EV2 artifacts: %s", err)
	}

	log.Println("creating SIG release...")
	if err := adoClient.CreateSIGRelease(ctx, build); err != nil {
		log.Fatalf("creating SIG release: %s", err)
	}
}
