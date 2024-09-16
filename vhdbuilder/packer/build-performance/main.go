package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/agentBaker/vhdbuilder/packer/build-performance/pkg/common"
)

func main() {
	config, err := common.SetupConfig()
	if err != nil {
		log.Fatalf("could not set up config: %v", err)
	}

	maps := common.CreateDataMaps()

	client, err := common.CreateKustoClient(config.KustoEndpoint, config.KustoClientID)
	if err != nil {
		log.Fatalf("kusto ingestion client could not be created: %v.", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fmt.Println("Ingesting data...")
	err = common.IngestData(client, ctx, config.KustoDatabase, config.KustoTable, config.LocalBuildPerformanceFile, config.KustoIngestionMapping)
	if err != nil {
		log.Fatalf("ingestion failed: %v.", err)
	}

	aggregatedSKUData, err := common.QueryData(client, ctx, config.SigImageName, config.KustoDatabase)
	if err != nil {
		log.Fatalf("failed to query build performance data: %v.", err)
	}

	maps.PreparePerformanceDataForEvaluation(config.LocalBuildPerformanceFile, aggregatedSKUData)

	maps.EvaluatePerformance()
}
