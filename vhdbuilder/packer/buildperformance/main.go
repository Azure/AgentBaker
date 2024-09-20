package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Azure/agentBaker/vhdbuilder/packer/build-performance/pkg/service"
)

func main() {
	// Recover from panic and exit gracefully in order to prevent failing pipeline step
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic in main:", r)
			os.Exit(0)
		}
	}()

	config, err := service.SetupConfig()
	if err != nil {
		panic(err)
	}
	log.Println("Program config set")

	maps := service.CreateDataMaps()

	client, err := service.CreateKustoClient(config.KustoEndpoint, config.KustoClientId)
	if err != nil {
		panic(err)
	}
	defer client.Close()
	log.Println("Kusto client created")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if config.SourceBranch == "refs/heads/zb/regression2" {
		if err := service.IngestData(
			client,
			ctx,
			config.KustoDatabase,
			config.KustoTable,
			config.LocalBuildPerformanceFile,
			config.KustoIngestionMapping); err != nil {
			panic(err)
		}
		log.Printf("Data ingested for %s\n", config.SigImageName)
	}

	queryData, err := service.QueryData(client, ctx, config.SigImageName, config.KustoDatabase)
	if err != nil {
		panic(err)
	}
	log.Printf("Queried aggregated performance data for %s\n", config.SigImageName)

	if err = maps.PreparePerformanceDataForEvaluation(config.LocalBuildPerformanceFile, queryData); err != nil {
		panic(err)
	}

	if err = maps.EvaluatePerformance(); err != nil {
		panic(err)
	}
}
