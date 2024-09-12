package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/AgentBaker/vhdbuilder/packer/build-performance/pkg/common"
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/agentBaker/vhdbuilder/packer/build-performance/pkg/common"
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

		fmt.Printf("Branch is %s, ingesting data.\n", config.SourceBranch)
		common.IngestData(client, config.KustoDatabase, config.KustoTable, ctx, config.LocalBuildPerformanceFile)
	}

	aggregateSKUData := common.QueryData(config.SigImageName, client, config.KustoDatabase, config.KustoTable, ctx, config.LocalBuildPerformanceFile)

	common.DecodeVHDPerformanceData(config.LocalBuildPerformanceFile, dataMaps.HoldingMap)

	common.ConvertTimestampsToSeconds(dataMaps.HoldingMap, dataMaps.LocalPerformanceDataMap)

	common.ParseKustoData(aggregateSKUData, dataMaps.QueriedPerformanceData)

	common.EvaluatePerformance(dataMaps.LocalPerformanceDataMap, dataMaps.QueriedPerformanceData, dataMaps.RegressionMap)

	common.PrintRegressions(dataMaps.RegressionMap)
}
