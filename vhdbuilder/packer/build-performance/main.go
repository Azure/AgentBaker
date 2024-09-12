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
		log.Fatalf(err)
	}

	dataMaps := common.CreateDataMaps()

	kcsb := kusto.NewConnectionStringBuilder(config.KustoEndpoint).WithUserManagedIdentity(config.KustoClientID)

	client, err := kusto.New(kcsb)
	if err != nil {
		log.Fatalf("Kusto ingestion client could not be created.")
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

	common.DecodeVHDPerformanceData(config.LocalBuildPerformanceFile, dataMaps.HoldingMap)

	common.ConvertTimestampsToSeconds(dataMaps.HoldingMap, dataMaps.LocalPerformanceDataMap)

	aggregatedSKUData, err := common.QueryData(ctx, client, config.SigImageName, config.KustoDatabase, config.KustoTable, config.LocalBuildPerformanceFile)
	if err != nil {
		log.Fatalf("Failed to query build performance data for %s.\n\n", config.SigImageName)
	}

	common.ParseKustoData(aggregatedSKUData, dataMaps.QueriedPerformanceData)

	common.EvaluatePerformance(dataMaps.LocalPerformanceDataMap, dataMaps.QueriedPerformanceData, dataMaps.RegressionMap)

	if len(dataMaps.RegressionMap) == 0 {
		fmt.Printf("No regressions found for this pipeline run\n\n")
	} else {
		fmt.Printf("Regressions found for this pipline run. Listed is the amount of time that the identified section exceeded one standard deviation by:\n\n")
		common.PrintRegressions(dataMaps.RegressionMap)
	}

	fmt.Println("Build Performance Evaluation Complete")
}
