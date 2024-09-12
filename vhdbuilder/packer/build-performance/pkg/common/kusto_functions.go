package common

import (
	"context"
	"fmt"

	"github.com/Azure/AgentBaker/vhdbuilder/packer/build-performance/pkg/common"
	"github.com/Azure/azure-kusto-go/kusto"
	kustoErrors "github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/azure-kusto-go/kusto/kql"
)

func IngestData(ctx context.Context, client *kusto.Client, kustoDatabase string, kustoTable string, buildPerformanceDataFile string) error {
	// Create Ingestor
	ingestor, err := ingest.New(client, kustoDatabase, kustoTable)
	if err != nil {
		return err
	} else {
		fmt.Printf("Created ingestor...\n\n")
	}
	defer ingestor.Close()

	// Ingest Data
	_, err = ingestor.FromFile(ctx, buildPerformanceDataFile, ingest.IngestionMappingRef("buildPerfMap", ingest.MultiJSON))
	if err != nil {
		ingestor.Close()
		return err
	} else {
		fmt.Printf("Successfully ingested build performance data.\n\n")
	}
	defer ingestor.Close()
	return nil
}

func QueryData(ctx context.Context, client *kusto.Client, sigImageName string, kustoDatabase string, kustoTable string) (*common.SKU, error) {
	query := kql.New("Get_Performance_Data | where SIG_IMAGE_NAME == SKU")
	params := kql.NewParameters().AddString("SKU", sigImageName)

	iter, err := client.Query(ctx, kustoDatabase, query, kusto.QueryParameters(params))
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	data := common.SKU{}
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
