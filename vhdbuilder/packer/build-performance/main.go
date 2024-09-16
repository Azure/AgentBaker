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
	client, err := kusto.New(kcsb)
	if err != nil {
		log.Fatalf("failed to create client: %v\n\n", err)
	}
	fmt.Printf("Created client...\n\n")
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ingestor, err := ingest.New(client, config.KustoDatabase, config.KustoTable)
	if err != nil {
		log.Fatalf("Kusto ingestor could not be created.")
	} else {
		fmt.Printf("Created ingestor...\n\n")
	}
	defer ingestor.Close()

	// Ingest Data
	_, err = ingestor.FromFile(ctx, config.LocalBuildPerformanceFile, ingest.IngestionMappingRef(config.KustoIngestionMapping, ingest.MultiJSON))
	if err != nil {
		log.Fatalf("Ingestion failed: %v\n\n", err)
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
		log.Fatalf("ingestion failed: %v\n\n", err)
	}
}
*/
