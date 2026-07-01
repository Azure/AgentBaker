// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

// Command cosi-upload streams a staged COSI artifact to PMC's Azure Front Door
// upload endpoint using the Azure Blob SDK's chunked block-blob upload. Because
// the SDK splits the file into blocks (Put Block + Put Block List), it is not
// subject to the single Put Blob size limit that a plain PUT hits on large COSI
// images. Authentication is Microsoft Entra ID via the Azure CLI login supplied
// by the pipeline's AzureCLI@2 task; the AFD endpoint transparently forwards the
// bearer token to the blob origin.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

func main() {
	endpoint := flag.String("endpoint", "", "AFD upload endpoint base URL, e.g. https://<host> (required)")
	container := flag.String("container", "", "destination container name (required)")
	blob := flag.String("blob", "", "destination blob name (required)")
	file := flag.String("file", "", "path to the local COSI file to upload (required)")
	flag.Parse()

	if *endpoint == "" || *container == "" || *blob == "" || *file == "" {
		log.Fatal("--endpoint, --container, --blob and --file are all required")
	}

	if err := run(context.Background(), *endpoint, *container, *blob, *file); err != nil {
		log.Fatalf("upload COSI: %v", err)
	}
}

func run(ctx context.Context, endpoint, container, blob, filePath string) error {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("create Azure CLI credential: %w", err)
	}

	client, err := azblob.NewClient(endpoint, cred, nil)
	if err != nil {
		return fmt.Errorf("create blob client for %s: %w", endpoint, err)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	log.Printf("Uploading %s to %s/%s/%s", filePath, endpoint, container, blob)
	if _, err := client.UploadFile(ctx, container, blob, f, &azblob.UploadFileOptions{
		BlockSize:   16 * 1024 * 1024, // 16 MiB blocks
		Concurrency: 8,
	}); err != nil {
		return fmt.Errorf("upload %s -> %s/%s/%s: %w", filePath, endpoint, container, blob, err)
	}

	log.Printf("Successfully uploaded %s/%s/%s", endpoint, container, blob)
	return nil
}
