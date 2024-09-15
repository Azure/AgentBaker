package main

import (
	"fmt"
	"log"

	"github.com/Azure/agentBaker/vhdbuilder/packer/build-performance/pkg/common"
	"github.com/Azure/azure-kusto-go/kusto"
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
		log.Fatalf("Kusto ingestion client could not be created.")
	} else {
		fmt.Printf("Created ingestion client...\n\n")
	}
	defer client.Close()

	if config.SourceBranch == "refs/heads/zb/ingestBuildPerfData" {
		err := common.IngestData(client, config.KustoDatabase, config.KustoTable, config.LocalBuildPerformanceFile, config.KustoIngestionMapping)
		if err != nil {
			log.Fatalf("Ingestion failed: %v\n\n", err)
		}
	}

	aggregatedSKUData, err := common.QueryData(client, config.SigImageName, config.KustoDatabase, config.KustoTable)
	if err != nil {
		log.Fatalf("failed to query build performance data for %s.\n\n", config.SigImageName)
	}

	maps.PreparePerformanceDataForEvaluation(config.LocalBuildPerformanceFile, aggregatedSKUData)

	maps.EvaluatePerformance()
}
