package service

import (
	"context"
	"fmt"

	"github.com/Azure/azure-kusto-go/kusto"
	kustoErrors "github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/azure-kusto-go/kusto/kql"
)

func CreateKustoClient(kustoEndpoint string, kustoClientId string) (*kusto.Client, error) {
	kcsb := kusto.NewConnectionStringBuilder(kustoEndpoint).WithUserManagedIdentity(kustoClientId)
	client, err := kusto.New(kcsb)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func IngestData(client *kusto.Client, ctx context.Context, kustoDatabase string, kustoTable string, buildPerformanceDataFile string, kustoIngestionMap string) error {
	ingestor, err := ingest.New(client, kustoDatabase, kustoTable)
	if err != nil {
		return fmt.Errorf("failed to create ingestor: %w", err)
	}
	defer ingestor.Close()

	_, err = ingestor.FromFile(ctx, buildPerformanceDataFile, ingest.IngestionMappingRef(kustoIngestionMap, ingest.MultiJSON))
	if err != nil {
		return fmt.Errorf("failed to ingest build performance data: %w", err)
	}
	return nil
}

func QueryData(client *kusto.Client, ctx context.Context, sigImageName string, kustoDatabase string) (*SKU, error) {
	query := kql.New("Get_Performance_Data | where SIG_IMAGE_NAME == SKU")
	params := kql.NewParameters().AddString("SKU", sigImageName)

	iter, err := client.Query(ctx, kustoDatabase, query, kusto.QueryParameters(params))
	if err != nil {
		return nil, fmt.Errorf("failed to query kusto database: %w", err)
	}
	defer iter.Stop()

	data := SKU{}
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *kustoErrors.Error) error {
			if e != nil {
				return fmt.Errorf("error while iterating over query table: %w", e)
			}
			if err := row.ToStruct(&data); err != nil {
				return fmt.Errorf("failed to convert query row to struct: %w", err)
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to persist query data: %w", err)
	}
	return &data, nil
}
