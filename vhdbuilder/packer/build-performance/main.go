package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/agentBaker/vhdbuilder/packer/build-performance/pkg/common"
	"github.com/Azure/azure-kusto-go/kusto"
)

func main() {
	config, err := common.SetupConfig()
	if err != nil {
		log.Fatalf("failed to configure build-performance program: %v\n\n", err)
	}

	maps := common.CreateDataMaps()

	kcsb := kusto.NewConnectionStringBuilder(config.KustoEndpoint).WithUserManagedIdentity(config.KustoClientID)

	client, err := kusto.New(kcsb)
	if err != nil {
		log.Fatalf("kusto ingestion client could not be created: %v\n\n", err)
	} else {
		fmt.Printf("Created ingestion client...\n\n")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if config.SourceBranch == "refs/heads/zb/ingestBuildPerfData" {
		fmt.Printf("Ingesting data for %s.\n\n", config.SourceBranch)
		err := common.IngestData(ctx, client, config.KustoDatabase, config.KustoTable, config.LocalBuildPerformanceFile)
		if err != nil {
			client.Close()
			cancel()
			log.Fatalf("Ingestion failed: %v\n\n", err)
		}
	}

	maps.DecodeLocalPerformanceData(config.LocalBuildPerformanceFile)

	aggregatedSKUData, err := common.QueryData(ctx, client, config.SigImageName, config.KustoDatabase, config.KustoTable)
	if err != nil {
		log.Fatalf("failed to query build performance data for %s.\n\n", config.SigImageName)
	}

	maps.ParseKustoData(aggregatedSKUData)

	maps.EvaluatePerformance()

	if len(maps.RegressionMap) == 0 {
		fmt.Printf("No regressions found for this pipeline run\n\n")
	} else {
		fmt.Printf("Regressions listed below. Section values represent the amount of time the section exceeded 1 stdev by.\n\n")
		maps.PrintRegressions()
	}

	fmt.Println("Build Performance Evaluation Complete")
}
