package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// this needs to get uploaded onto the VM to execute
// storage_sas_token=$(az storage container generate-sas --name ${OUTPUT_STORAGE_CONTAINER_NAME} --permissions acwlr --connection-string ${CLASSIC_SA_CONNECTION_STRING} --start ${start_date} --expiry ${expiry_date} | tr -d '"')
// https://vhdbuildereastustest.blob.core.windows.net/vhd
func uploadFilesFromLocalToBlobStorage() {
	accountName := "<your_account_name>"
	accountKey := "<your_account_key>"
	containerName := "<your_container_name>"
	blobName := "<desired_blob_name>"

	// Create a connection string
	accountConnectionString := fmt.Sprintf("DefaultEndpointsProtocol=https;AccountName=%s;AccountKey=%s", accountName, accountKey)

	// Create a BlobServiceClient
	serviceURL, _ := azblob.NewServiceURL(fmt.Sprintf("https://%s.blob.core.windows.net", accountName), azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))

	// Get a container URL
	containerURL := serviceURL.NewContainerURL(containerName)

	// Get a blob URL
	blobURL := containerURL.NewBlockBlobURL(blobName)

	// Download the blob content
	resp, err := blobURL.Download(context.Background(), 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		fmt.Println("Error downloading blob:", err)
		return
	}
	defer resp.Response().Body.Close()

	// Create the output file
	file, err := os.Create("<local_output_file_path>")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	// Write the blob content to the file
	_, err = file.ReadFrom(resp.Body(azblob.RetryReaderOptions{}))
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}

	fmt.Println("File downloaded successfully!")

}

func downloadFilesFromBlobStorageToTestVM() {

}

func execute_VHD_scans() {
	execute_scans()
}

func main() {
	uploadFilesFromLocalToBlobStorage()
	downloadFilesFromBlobStorageToTestVM()
	execute_VHD_scans()
}
