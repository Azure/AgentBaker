package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Azure/azure-kusto-go/azkustodata"
	"github.com/Azure/azure-kusto-go/azkustoingest"
)

func main() {

	kustoTable := os.Getenv("KUSTO_TABLE_NAME")
	kustoEndpoint := os.Getenv("KUSTO_ENDPOINT")
	kustoDatabase := os.Getenv("KUSTO_DATABASE_NAME")
	sigImageName := os.Getenv("SIG_IMAGE_NAME")
	//sourceBranchName := os.Getenv("SOURCE_BRANCH_NAME")
	buildPerformanceDataFile := sigImageName + "build-performance.json"

	// Create Connection String
	kustoConnectionString := azkustodata.NewConnectionStringBuilder(kustoEndpoint).WithAzCli()

	ingestionClient, err := azkustoingest.New(
		kustoConnectionString,
		azkustoingest.WithDefaultTable(kustoTable),
		azkustoingest.WithDefaultDatabase(kustoDatabase))

	if err != nil {
		log.Fatalf("Kusto ingestion client could not be created.")
	}
	defer ingestionClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	_, err = ingestionClient.FromFile(
		ctx,
		buildPerformanceDataFile,
		azkustoingest.IngestionMappingRef("buildPerfMapping", azkustoingest.JSON))

	if err != nil {
		cancel()
		ingestionClient.Close()
		log.Fatalf("Failed to ingest build performance data.")
	}

	fmt.Println("Successfully ingested build performance data.")

	return
}
