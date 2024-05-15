#!/bin/bash

# uploadFilesFromLocalToBlobStorage
storage_account_name="${OUTPUT_STORAGE_ACCOUNT_NAME}"
container_name="vhd-scans"

# az login should have already occured

storage_connection_string=$(az storage account show-connection-string --name $storage_account_name --query connectionString --output tsv)
az storage blob upload --connection-string $storage_connection_string --container-name $container_name --file execute-vhd-scans.sh --name "execute-vhd-scans.sh"
az storage blob upload --connection-string $storage_connection_string --container-name $container_name --file trivy-scans.sh --name "trivy-scans.sh"
az storage blob upload --connection-string $storage_connection_string --container-name $container_name --file storage-scans.sh --name "storage-scans.sh"

# downloadFilesFromBlobStorageToTestVM



# execute_VHD_scans
./execute-vhd-scans.sh


