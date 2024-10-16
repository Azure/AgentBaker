package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Azure/agentBaker/vhdbuilder/packer/buildperformance/pkg/service"
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

	maps := service.CreateDataMaps()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := service.IngestData(ctx, config); err != nil {
		panic(err)
	}

	queryData, err := service.QueryData(ctx, config)
	if err != nil {
		panic(err)
	}

	if err = maps.PreparePerformanceDataForEvaluation(config.LocalBuildPerformanceFile, queryData); err != nil {
		panic(err)
	}

	if err = maps.EvaluatePerformance(); err != nil {
		panic(err)
	}
}
