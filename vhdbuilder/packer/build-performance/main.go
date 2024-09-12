package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Azure/AgentBaker/vhdbuilder/packer/build-performance/pkg/common"
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/agentBaker/vhdbuilder/packer/build-performance/pkg/common"
)

func main() {

	kustoTable := os.Getenv("BUILD_PERFORMANCE_TABLE_NAME")
	kustoEndpoint := os.Getenv("BUILD_PERFORMANCE_KUSTO_ENDPOINT")
	kustoDatabase := os.Getenv("BUILD_PERFORMANCE_DATABASE_NAME")
	kustoClientID := os.Getenv("BUILD_PERFORMANCE_CLIENT_ID")

	sigImageName := os.Getenv("SIG_IMAGE_NAME")
	buildPerformanceDataFile := sigImageName + "-build-performance.json"
	sourceBranch := os.Getenv("GIT_BRANCH")

	queriedPerformanceData := map[string]map[string][]float64{}
	currentBuildPerformanceData := map[string]map[string]float64{}
	holdingMap := map[string]map[string]string{}
	regressions := map[string]map[string]float64{}

	var err error

	fmt.Printf("\nRunning build performance program for %s...\n\n", sigImageName)

	kcsb := kusto.NewConnectionStringBuilder(kustoEndpoint).WithUserManagedIdentity(kustoClientID)

	client, err := kusto.New(kcsb)
	if err != nil {
		log.Fatalf("Kusto ingestion client could not be created.")
	} else {
		fmt.Printf("Created ingestion client...\n\n")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if sourceBranch == "refs/heads/zb/ingestBuildPerfData" {

		fmt.Printf("Branch is %s, ingesting data.\n", sourceBranch)
		common.IngestData(client, kustoDatabase, kustoTable, ctx, buildPerformanceDataFile)

	}

	data := common.QueryData(sigImageName, client, kustoDatabase, kustoTable, ctx, buildPerformanceDataFile)
	common.DecodeVHDPerformanceData("/home/zbailey/go/go_proj/go_test/json.json", holdingMap)
	common.ConvertTimestampsToSeconds(holdingMap, currentBuildPerformanceData)
	common.ParseKustoData(data, queriedPerformanceData)
	common.EvaluatePerformance(currentBuildPerformanceData, queriedPerformanceData, regressions)
	common.PrintRegressions(regressions)
}
