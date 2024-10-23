package service

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-kusto-go/azkustodata"
	"github.com/Azure/azure-kusto-go/azkustodata/kql"
	queryPkg "github.com/Azure/azure-kusto-go/azkustodata/query"
	"github.com/Azure/azure-kusto-go/azkustoingest"
)

func IngestData(ctx context.Context, config *Config) error {
	if config.SourceBranch == "refs/heads/zb/regression2" {
		kcsb := azkustodata.NewConnectionStringBuilder(config.KustoEndpoint).WithUserAssignedIdentityResourceId(config.CommonIdentityId)

		ingestor, err := azkustoingest.New(kcsb, azkustoingest.WithDefaultDatabase(config.KustoDatabase), azkustoingest.WithDefaultTable(config.KustoTable))
		if err != nil {
			return fmt.Errorf("failed to create ingestor: %w", err)
		}
		defer ingestor.Close()

		if _, err = ingestor.FromFile(ctx, config.LocalBuildPerformanceFile, azkustoingest.IngestionMappingRef(config.KustoIngestionMapping, azkustoingest.MultiJSON)); err != nil {
			return fmt.Errorf("failed to ingest build performance data: %w", err)
		}

		log.Printf("Data ingested for %s\n", config.SigImageName)

		return nil
	}

	log.Println("Data not ingested as source branch is not master")

	return nil
}

func QueryData(ctx context.Context, config *Config) (*SKU, error) {
	kcsb := azkustodata.NewConnectionStringBuilder(config.KustoEndpoint).WithUserAssignedIdentityResourceId(config.CommonIdentityId)

	client, err := azkustodata.New(kcsb)
	if err != nil {
		return nil, fmt.Errorf("failed to create kusto query client: %w", err)
	}
	defer client.Close()

	log.Println("Kusto query client created")

	query := kql.New("storedFunction | where primaryKey == SKU")
	params := kql.NewParameters().
		AddString("storedFunction", config.StoredFunctionName).
		AddString("primaryKey", config.PrimaryKey).
		AddString("SKU", config.SigImageName)

	dataset, err := client.Query(ctx, config.KustoDatabase, query, azkustodata.QueryParameters(params))
	if err != nil {
		return nil, fmt.Errorf("failed to query kusto database: %w", err)
	}

	data, err := queryPkg.ToStructs[SKU](dataset)
	if err != nil {
		return nil, fmt.Errorf("failed to persist query data: %w", err)
	}

	numRows := len(dataset.Tables()[0].Rows())
	if numRows != 1 {
		return nil, fmt.Errorf("query returned %d rows", numRows)
	}

	log.Printf("Queried aggregated performance data for %s and received %d row of data", config.SigImageName, numRows)

	return &data[0], nil
}
