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
	clientID := os.Getenv("CLIENT_ID")
	//sourceBranchName := os.Getenv("SOURCE_BRANCH_NAME")
	sigImageName := os.Getenv("SIG_IMAGE_NAME")
	buildPerformanceDataFile := "/go/src/github.com/Azure/AgentBaker/vhdbuilder/packer/test/build-performance/" + sigImageName + "-build-performance.json"
	var err error

	// Create Connection String
	kcsb := kusto.NewConnectionStringBuilder(kustoEndpoint).WithUserManagedIdentity(clientID)

	ingestionClient, err := kusto.New(kcsb)
	if err != nil {
		log.Fatalf("Kusto ingestion client could not be created.")
	} else {
		fmt.Printf("Ingestion client successfully created\n")
	}
	defer ingestionClient.Close()

	ingestor, err := ingest.New(ingestionClient, kustoDatabase, kustoTable)
	if err != nil {
		ingestionClient.Close()
		log.Fatalf("Kusto ingestor could not be created.")
	} else {
		fmt.Printf("Ingestor created successfully\n")
	}
	defer ingestor.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	status, err := ingestor.FromFile(
		ctx,
		buildPerformanceDataFile,
		ingest.IngestionMappingRef("buildPerfMapping1", ingest.JSON),
		ingest.ReportResultToTable())

	if err != nil {
		fmt.Printf("Ingestion failed: %v\n", err)
		ingestor.Close()
		ingestionClient.Close()
		cancel()
		log.Fatalf("Igestion command failed to be sent")
	} else {
		fmt.Printf("Ingestion started successfully\n")
	}
	defer ingestor.Close()

	err = <-status.Wait(ctx)
	if err != nil {
		fmt.Printf("Ingestion failed: %v\n", err)
		statusCode, _ := ingest.GetIngestionStatus(err)
		failureStatus, _ := ingest.GetIngestionFailureStatus(err)
		fmt.Printf("Ingestion status code: %v\n", statusCode)
		fmt.Printf("Ingestion failure status: %v\n", failureStatus)
		ingestor.Close()
		ingestionClient.Close()
		cancel()
		log.Fatalf("Ingestion failed to be completed")
	} else {
		fmt.Printf("Ingestion successful, error == nil\n")
	}

	fmt.Println("Successfully ingested build performance data.")
}
