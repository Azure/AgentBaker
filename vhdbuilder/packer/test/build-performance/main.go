package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
)

func main() {

	kustoTable := os.Getenv("KUSTO_TABLE_NAME")
	kustoEndpoint := os.Getenv("KUSTO_ENDPOINT")
	kustoDatabase := os.Getenv("KUSTO_DATABASE_NAME")
	//sourceBranchName := os.Getenv("SOURCE_BRANCH_NAME")
	sigImageName := os.Getenv("SIG_IMAGE_NAME")
	buildPerformanceDataFile := sigImageName + "-build-performance.json"
	var err error

	// Create Connection String
	kcsb := kusto.NewConnectionStringBuilder(kustoEndpoint).WithSystemManagedIdentity()

	ingestionClient, err := kusto.New(kcsb)
	if err != nil {
		log.Fatalf("Kusto ingestion client could not be created.")
	}
	defer ingestionClient.Close()

	ingestor, err := ingest.New(ingestionClient, kustoDatabase, kustoTable)
	if err != nil {
		ingestionClient.Close()
		log.Fatalf("Kusto ingestor could not be created.")
	}
	defer ingestor.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	_, err = ingestor.FromFile(
		ctx,
		buildPerformanceDataFile,
		ingest.IngestionMappingRef("buildPerfMapping", ingest.JSON))

	if err != nil {
		ingestor.Close()
		ingestionClient.Close()
		cancel()
		log.Fatalf("Failed to ingest build performance data.")
	}
	defer ingestor.Close()

	fmt.Println("Successfully ingested build performance data.")
}
