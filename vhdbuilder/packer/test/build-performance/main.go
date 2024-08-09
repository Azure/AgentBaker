package main

import (
	"context"
	"os"
	"time"

	"github.com/Azure/azure-kusto-go/azkustodata"
	"github.com/Azure/azure-kusto-go/azkustoingest"
)

func main() {

	sourceBranch := os.Getenv("SOURCE_BRANCH")
	kustoEndpoint := os.Getenv("KUSTO_ENDPOINT")
	kustoDatabase := os.Getenv("KUSTO_DATABASE_NAME")
	buildPerformanceDataFile := os.Getenv("VHD_BUILD_PERFORMANCE_DATA_FILE")
	buildPerformanceTable := os.Getenv("KUSTO_TABLE_NAME")

	// Create Connection String
	kustoConnectionString := azkustodata.NewConnectionStringBuilder(kustoEndpoint).WithSystemManagedIdentity()

	if sourceBranch == "master" {

		ingestionClient, err := azkustoingest.New(kustoConnectionString, azkustoingest.WithoutEndpointCorrection())
		if err != nil {
			panic("Kusto ingestion client could not be created.")
		}
		defer ingestionClient.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if _, err := ingestionClient.FromFile(
			ctx,
			buildPerformanceDataFile,
			azkustoingest.IngestionMappingRef("buildPerfMapping", azkustoingest.JSON)); err != nil {
			panic("Failed to ingest build performance data.")
		}

		return
	} else {

	}
	//dataPlaneClient, err := azkustodata.New(kcsb)
	//if err != nil {
	//panic("Could not create Kusto Connection String.")
	//}

	// TODO: If not master, Read JSON file artifact from staging directory, store in var

	// TODO: Query kusto DB for sku specific bell curve for each step

	// TODO: Compare the two, for each step that is not within one standard deviation, log step and difference

	// TODO: Output JSON Array with build performance data and logs for out-of-spec steps

	// TODO: Exit 0
}
