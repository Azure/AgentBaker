package common

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
)

func IngestData(client *kusto.Client, kustoDatabase string, kustoTable string, ctx context.Context, buildPerformanceDataFile string) {

	// Create Ingestor
	ingestor, err := ingest.New(client, kustoDatabase, kustoTable)
	if err != nil {
		client.Close()
		log.Fatalf("Kusto ingestor could not be created.")
	} else {
		fmt.Printf("Created ingestor...\n\n")
	}
	defer ingestor.Close()

	// Ingest Data
	_, err = ingestor.FromFile(ctx, buildPerformanceDataFile, ingest.IngestionMappingRef("buildPerfMap", ingest.MultiJSON))
	if err != nil {
		fmt.Printf("Ingestion failed: %v\n\n", err)
		ingestor.Close()
		client.Close()
		log.Fatalf("Igestion command failed to be sent.\n")
	} else {
		fmt.Printf("Successfully ingested build performance data.\n\n")
	}
	defer ingestor.Close()
}
