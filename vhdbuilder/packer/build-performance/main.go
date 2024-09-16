package main

import (
	"fmt"
	"log"

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

	fmt.Println("Ingesting data...")
	err = common.IngestData(client, config.KustoDatabase, config.KustoTable, config.LocalBuildPerformanceFile, config.KustoIngestionMapping)
	if err != nil {
		log.Fatalf("ingestion failed: %v.", err)
	}

	aggregatedSKUData, err := common.QueryData(client, config.SigImageName, config.KustoDatabase)
	if err != nil {
		log.Fatalf("failed to query build performance data: %v.", err)
	}

	maps.PreparePerformanceDataForEvaluation(config.LocalBuildPerformanceFile, aggregatedSKUData)

	maps.EvaluatePerformance()
}
