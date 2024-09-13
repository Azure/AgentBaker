package common

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-kusto-go/kusto"
	kustoErrors "github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/azure-kusto-go/kusto/kql"
)

func IngestData(client *kusto.Client, kustoDatabase string, kustoTable string, buildPerformanceDataFile string, kustoIngestionMap string) error {
	// Create Ingestor
	ingestor, err := ingest.New(client, kustoDatabase, kustoTable)
	if err != nil {
		return err
	} else {
		fmt.Printf("Created ingestor...\n\n")
	}

	// Create ingestion context
	ingestionCtx, cancelIngestion := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelIngestion()

	// Ingest Data
	_, err = ingestor.FromFile(ingestionCtx, buildPerformanceDataFile, ingest.IngestionMappingRef(kustoIngestionMap, ingest.MultiJSON))
	if err != nil {
		ingestor.Close()
		return err
	} else {
		fmt.Printf("Successfully ingested build performance data.\n\n")
	}
	defer ingestor.Close()

	return nil
}

func QueryData(client *kusto.Client, sigImageName string, kustoDatabase string, kustoTable string) (*SKU, error) {
	// Build Query
	query := kql.New("Get_Performance_Data | where SIG_IMAGE_NAME == SKU")
	params := kql.NewParameters().AddString("SKU", sigImageName)

	// Create query context
	queryCtx, cancelQuery := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelQuery()

	// Execute Query
	iter, err := client.Query(queryCtx, kustoDatabase, query, kusto.QueryParameters(params))
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
