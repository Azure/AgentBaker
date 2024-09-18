package main

import (
	"context"
	"fmt"
	"log"
	"time"

	. "github.com/Azure/agentBaker/vhdbuilder/packer/buildperformance/pkg/service"
)

func main() {
	config, err := SetupConfig()
	if err != nil {
		log.Fatalf("could not set up config: %v", err)
	}
	fmt.Println("Program config set")

	maps := CreateDataMaps()

	client, err := CreateKustoClient(config.KustoEndpoint, config.KustoClientId)
	if err != nil {
		panic(err)
	}
	defer client.Close()
	fmt.Println("Kusto client created")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if config.SourceBranch == "refs/heads/zb/ingestBuildPerfData" {
		err = IngestData(client, ctx, config.KustoDatabase, config.KustoTable, config.LocalBuildPerformanceFile, config.KustoIngestionMapping)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Data ingested for %s\n", config.SigImageName)
	}

	queryData, err := QueryData(client, ctx, config.SigImageName, config.KustoDatabase)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Queried aggregated performance data for %s\n", config.SigImageName)

	err = maps.PreparePerformanceDataForEvaluation(config.LocalBuildPerformanceFile, queryData)
	if err != nil {
		panic(err)
	}

	err = maps.EvaluatePerformance()
	if err != nil {
		panic(err)
	}
}
