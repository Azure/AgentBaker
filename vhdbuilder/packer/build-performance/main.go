package main

import (
	"context"
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
		panic(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if config.SourceBranch == "refs/heads/zb/ingestBuildPerfData" {
		err = common.IngestData(client, ctx, config.KustoDatabase, config.KustoTable, config.LocalBuildPerformanceFile, config.KustoIngestionMapping)
		if err != nil {
			panic(err)
		}
	}

	queryData, err := common.QueryData(client, ctx, config.SigImageName, config.KustoDatabase)
	if err != nil {
		panic(err)
	}

	err = maps.PreparePerformanceDataForEvaluation(config.LocalBuildPerformanceFile, queryData)
	if err != nil {
		panic(err)
	}

	err = maps.EvaluatePerformance()
	if err != nil {
		panic(err)
	}
}
