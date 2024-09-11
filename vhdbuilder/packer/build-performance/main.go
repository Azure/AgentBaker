package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Azure/azure-kusto-go/kusto"
	kustoErrors "github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/azure-kusto-go/kusto/kql"
)

func main() {

	// Kusto variables
	kustoTable := os.Getenv("BUILD_PERFORMANCE_TABLE_NAME")
	kustoEndpoint := os.Getenv("BUILD_PERFORMANCE_KUSTO_ENDPOINT")
	kustoDatabase := os.Getenv("BUILD_PERFORMANCE_DATABASE_NAME")
	kustoClientID := os.Getenv("BUILD_PERFORMANCE_CLIENT_ID")
	// Build data variables
	sigImageName := os.Getenv("SIG_IMAGE_NAME")
	buildPerformanceDataFile := sigImageName + "-build-performance.json"
	sourceBranch := os.Getenv("GIT_BRANCH")

	var err error

	fmt.Printf("\nRunning build performance program for %s...\n\n", sigImageName)

	// Create Connection String
	kcsb := kusto.NewConnectionStringBuilder(kustoEndpoint).WithUserManagedIdentity(kustoClientID)

	// Create  Client
	client, err := kusto.New(kcsb)
	if err != nil {
		log.Fatalf("Kusto ingestion client could not be created.")
	} else {
		fmt.Printf("Created ingestion client...\n\n")
	}
	defer client.Close()

	// Create Context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if sourceBranch == "refs/heads/zb/ingestBuildPerfData" {

		fmt.Printf("Branch is %s, ingesting data.\n", sourceBranch)

		// Create Ingestor
		ingestor, err := ingest.New(client, kustoDatabase, kustoTable)
		if err != nil {
			client.Close()
			log.Fatalf("Kusto ingestor could not be created.")
		} else {
			fmt.Printf("Created ingestor...\n\n")
		}
		defer ingestor.Close()

		// Ingest Data
		_, err = ingestor.FromFile(ctx, buildPerformanceDataFile, ingest.IngestionMappingRef("buildPerfMap", ingest.MultiJSON))
		if err != nil {
			fmt.Printf("Ingestion failed: %v\n\n", err)
			ingestor.Close()
			client.Close()
			cancel()
			log.Fatalf("Igestion command failed to be sent.\n")
		} else {
			fmt.Printf("Successfully ingested build performance data.\n\n")
		}
		defer ingestor.Close()
	}

	// Create query regardless of the branch
	query := kql.New("Get_Performance_Data | where SIG_IMAGE_NAME == SKU")

	// Declare parameters to be used in the query
	params := kql.NewParameters().AddString("SKU", sigImageName)

	// Define a struct to hold the data
	type SKU struct {
		Name               string `kusto:"SIG_IMAGE_NAME"`
		SKUPerformanceData string `kusto:"BUILD_PERFORMANCE"`
	}

	// Execute the query
	iter, err := client.Query(ctx, kustoDatabase, query, kusto.QueryParameters(params))
	if err != nil {
		fmt.Printf("Failed to query build performance data for %s.\n\n", sigImageName)
	}
	defer iter.Stop()

	// Load query result into struct
	data := SKU{}
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *kustoErrors.Error) error {
			if e != nil {
				return e
			}

			if err := row.ToStruct(&data); err != nil {
				return err
			}

			return nil
		},
	)
	if err != nil {
		fmt.Printf("Failed to load %s performance data into Go struct.\n\n", sigImageName)
	}

	// Declare a variable to hold the JSON object parsed from the SKUPerformanceData string
	var aggPerformanceData map[string]map[string]float64
	var currentBuildPerformanceData map[string]map[string]float64

	// Use json.Unmarshal to parse the string into the map
	err = json.Unmarshal([]byte(data.SKUPerformanceData), &aggPerformanceData)
	if err != nil {
		// Handle the error
		log.Fatal(err)
	}
	// Parse the SKU struct into JSON file to be compared against current pipeline
	// Create dictionary to hold final results
	// Iterate over local performance file, mapping

}
