package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Azure/azure-kusto-go/azkustodata"
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/azkustoingest"
	kustoErrors "github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/azure-kusto-go/kusto/kql"
)

func CreateKustoClient(kustoEndpoint string, kustoClientId string) (*kusto.Client, error) {
	kustoConnectionString := azkustodata.NewConnectionStringBuilder(kustoEndpoint).WithUserManagedIdentity(kustoClientId)
	client, err := azkustodata.New(kustoConnectionString)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func IngestData(ctx context.Context, config *Config) error {
	if config.SourceBranch == "refs/heads/zb/regression2" {
		kustoConnectionString := azkustodata.NewConnectionStringBuilder(config.KustoEndpoint).WithUserManagedIdentity(config.KustoClientId)

		ingestor, err := azkustoingest.New(kustoConnectionString, config.KustoDatabase, config.KustoTable)
		if err != nil {
			return fmt.Errorf("failed to create ingestor: %w", err)
		}
		defer ingestor.Close()

		if _, err = ingestor.FromFile(ctx, config.LocalBuildPerformanceFile, ingest.IngestionMappingRef(config.KustoIngestionMapping, ingest.MultiJSON)); err != nil {
			return fmt.Errorf("failed to ingest build performance data: %w", err)
		}
		log.Printf("Data ingested for %s\n", config.SigImageName)
		return nil
	}
	log.Println("Data not ingested as source branch is not master")
	return nil
}

func QueryData(ctx context.Context, client *kusto.Client, sigImageName string, kustoDatabase string) (*SKU, error) {
	query := kql.New("Get_Performance_Data | where SIG_IMAGE_NAME == SKU")
	params := kql.NewParameters().AddString("SKU", sigImageName)

	iter, err := client.Query(ctx, kustoDatabase, query, kusto.QueryParameters(params))
	if err != nil {
		return nil, fmt.Errorf("failed to query kusto database: %w", err)
	}
	defer iter.Stop()

	data := SKU{}
	if err = iter.DoOnRowOrError(
		func(row *table.Row, e *kustoErrors.Error) error {
			if e != nil {
				return fmt.Errorf("error while iterating over query table: %w", e)
			}
			if err := row.ToStruct(&data); err != nil {
				return fmt.Errorf("failed to convert query row to struct: %w", err)
			}
			return nil
		},
	); err != nil {
		return nil, fmt.Errorf("failed to persist query data: %w", err)
	}

	if err := CheckNumberOfRowsReturned(iter); err != nil {
		return nil, err
	}
	log.Println("Query returned 1 row of aggregated data as expected")

	return &data, nil
}

func CheckNumberOfRowsReturned(iter *kusto.RowIterator) error {
	// GetQueryCompletionInformation returns a datatable with information about the query
	infoTable, err := iter.GetQueryCompletionInformation()
	if err != nil {
		return fmt.Errorf("unable to get query completion information: %v", err)
	}

	if len(infoTable.KustoRows) == 0 {
		return fmt.Errorf("query completion information is empty")
	}

	// Custom struct to unmarshal the query completion information
	QueryInformation := QueryCompletionInfo{}

	// The number of rows returned by the query is stored in the last element of a slice in the last row in the datatable returned by GetQueryCompletionInformation
	row := infoTable.KustoRows[len(infoTable.KustoRows)-1]
	payload := row[len(row)-1].String()

	if err = json.Unmarshal([]byte(payload), &QueryInformation); err != nil {
		return fmt.Errorf("could not unmarshal query completion information: %v", err)
	}

	numRows := QueryInformation.Payload[0]["table_row_count"]

	if numRows != 1 {
		return fmt.Errorf("unexpected number of rows returned from query: %v", numRows)
	}

	log.Println("Query returned 1 row of aggregated data as expected")

	return nil
}
