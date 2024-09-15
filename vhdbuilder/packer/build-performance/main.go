package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/agentBaker/vhdbuilder/packer/build-performance/pkg/common"
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
)

func main() {
	var err error

	config, err := common.SetupConfig()
	if err != nil {
		log.Fatalf("failed to configure build-performance program: %v\n\n", err)
	}

	maps := common.CreateDataMaps()

	kcsb := kusto.NewConnectionStringBuilder(config.KustoEndpoint).WithUserManagedIdentity(config.KustoClientID)

	// Create Client
	client, err := kusto.New(kcsb)
	if err != nil {
		log.Fatalf("Kusto ingestion client could not be created.")
	} else {
		fmt.Printf("Created ingestion client...\n\n")
	}
	defer client.Close()

	// Create Ingestor
	ingestor, err := ingest.New(client, config.KustoDatabase, config.KustoTable)
	if err != nil {
		client.Close()
		log.Fatalf("Kusto ingestor could not be created.")
	} else {
		fmt.Printf("Created ingestor...\n\n")
	}
	defer ingestor.Close()

	// Create Context
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	// Ingest Data
	_, err = ingestor.FromFile(ctx, config.LocalBuildPerformanceFile, ingest.IngestionMappingRef(config.KustoIngestionMapping, ingest.MultiJSON))
	if err != nil {
		fmt.Printf("Ingestion failed: %v\n\n", err)
		ingestor.Close()
		client.Close()
		cancel()
		log.Fatalf("Igestion command failed to be sent.\n")
	} else {
		fmt.Printf("Ingestion started successfully.\n\n")
	}

	aggregatedSKUData, err := common.QueryData(client, config.SigImageName, config.KustoDatabase, config.KustoTable)
	if err != nil {
		log.Fatalf("failed to query build performance data for %s.\n\n", config.SigImageName)
	}

	maps.PreparePerformanceDataForEvaluation(config.LocalBuildPerformanceFile, aggregatedSKUData)

	maps.EvaluatePerformance()
}

/*
	if config.SourceBranch == "refs/heads/zb/ingestBuildPerfData" {
		err := common.IngestData(client, config.KustoDatabase, config.KustoTable, config.LocalBuildPerformanceFile, config.KustoIngestionMapping)
		if err != nil {
			log.Fatalf("Ingestion failed: %v\n\n", err)
		}
	}
*/
