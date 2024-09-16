package common

import (
	"context"

	"github.com/Azure/azure-kusto-go/kusto"
	kustoErrors "github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/azure-kusto-go/kusto/kql"
)

func CreateKustoClient(kustoEndpoint string, kustoClientID string) (*kusto.Client, error) {
	kcsb := kusto.NewConnectionStringBuilder(kustoEndpoint).WithUserManagedIdentity(kustoClientID)
	client, err := kusto.New(kcsb)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func IngestData(client *kusto.Client, ctx context.Context, kustoDatabase string, kustoTable string, buildPerformanceDataFile string, kustoIngestionMap string) error {

	// Create Ingestor
	ingestor, err := ingest.New(client, kustoDatabase, kustoTable)
	if err != nil {
		return err
	}
	defer ingestor.Close()

	// Ingest Data
	_, err = ingestor.FromFile(ctx, buildPerformanceDataFile, ingest.IngestionMappingRef(kustoIngestionMap, ingest.MultiJSON))
	if err != nil {
		return err
	}
	return nil
}

func QueryData(client *kusto.Client, ctx context.Context, sigImageName string, kustoDatabase string) (*SKU, error) {
	// Build Query
	query := kql.New("Get_Performance_Data | where SIG_IMAGE_NAME == SKU")
	params := kql.NewParameters().AddString("SKU", sigImageName)

	// Execute Query
	iter, err := client.Query(ctx, kustoDatabase, query, kusto.QueryParameters(params))
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	data := SKU{}
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *kustoErrors.Error) error {
			if e != nil {
				return err
			}
			if err := row.ToStruct(&data); err != nil {
				return err
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &data, nil
}
