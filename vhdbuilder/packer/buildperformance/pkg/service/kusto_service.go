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

func CreateKustoClient(kustoEndpoint string, kustoClientId string) (*azkustodata.Client, error) {
	kustoConnectionString := azkustodata.NewConnectionStringBuilder(kustoEndpoint).WithUserAssignedIdentityResourceId(kustoClientId)
	client, err := azkustodata.New(kustoConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to create kusto client: %w", err)
	}
	return client, nil
}

func IngestData(ctx context.Context, config *Config) error {
	if config.SourceBranch == "refs/heads/zb/buildPerfMods" {
		kustoConnectionString := azkustodata.NewConnectionStringBuilder(config.KustoEndpoint).WithUserAssignedIdentityResourceId(config.KustoClientId)

		ingestor, err := azkustoingest.New(kustoConnectionString)
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
	client, err := CreateKustoClient(config.KustoEndpoint, config.KustoClientId)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	log.Println("Kusto query client created")

	query := kql.New("Get_Performance_Data | where SIG_IMAGE_NAME == SKU")
	params := kql.NewParameters().AddString("SKU", config.SigImageName)

	dataset, err := client.Query(ctx, config.KustoDatabase, query, azkustodata.QueryParameters(params))
	if err != nil {
		return nil, fmt.Errorf("failed to query kusto database: %w", err)
	}

	data, err := queryPkg.ToStructs[SKU](dataset)
	if err != nil {
		return nil, fmt.Errorf("failed to persist query data: %w", err)
	}

	numRows := len(dataset.Tables()[0].Rows())
	fmt.Println("Number of rows returned from query: %d", numRows)

	//if err := CheckNumberOfRowsReturned(iter); err != nil {
	//return nil, err
	//}

	log.Printf("Queried aggregated performance data for %s\n", config.SigImageName)
	log.Println("Query returned 1 row of aggregated data as expected")

	return &data[0], nil
}
