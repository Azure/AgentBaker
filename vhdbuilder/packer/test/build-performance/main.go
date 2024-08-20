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

	// kusto variables
	kustoTable := os.Getenv("KUSTO_TABLE_NAME")
	kustoEndpoint := os.Getenv("KUSTO_ENDPOINT")
	kustoDatabase := os.Getenv("KUSTO_DATABASE_NAME")
	kustoClientID := os.Getenv("KUSTO_CLIENT_ID")
	// build data variables
	sigImageName := os.Getenv("SIG_IMAGE_NAME")
	buildPerformanceDataFile := sigImageName + "-build-performance.json"
	var err error

	// Create Connection String
	kcsb := kusto.NewConnectionStringBuilder(kustoEndpoint).WithUserManagedIdentity(kustoClientID)

	// Create Ingestion Client
	ingestionClient, err := kusto.New(kcsb)
	if err != nil {
		log.Fatalf("Kusto ingestion client could not be created.")
	} else {
		fmt.Printf("Ingestion client successfully created\n")
	}
	defer ingestionClient.Close()

	// Create Ingestor
	ingestor, err := ingest.New(ingestionClient, kustoDatabase, kustoTable)
	if err != nil {
		ingestionClient.Close()
		log.Fatalf("Kusto ingestor could not be created.")
	} else {
		fmt.Printf("Ingestor created successfully\n")
	}
	defer ingestor.Close()

	// Create Context
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	// Ingest Data
	_, err = ingestor.FromFile(ctx, buildPerformanceDataFile, ingest.IngestionMappingRef("oneMapToRuleThemAll", ingest.MultiJSON))
	if err != nil {
		fmt.Printf("Ingestion failed: %v\n", err)
		ingestor.Close()
		ingestionClient.Close()
		cancel()
		log.Fatalf("Igestion command failed to be sent\n")
	} else {
		fmt.Printf("Ingestion started successfully\n")
	}
	defer ingestor.Close()

	fmt.Println("Successfully ingested build performance data.\n")
}
