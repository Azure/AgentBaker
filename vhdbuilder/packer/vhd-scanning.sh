#!/bin/bash
set -eux

RESOURCE_GROUP_NAME="$AZURE_RESOURCE_GROUP_NAME"
VM_NAME_SCANNING="${VM_NAME}-scanning"
VHD_IMAGE="$IMG_DEF"
SIG_CONTAINER_NAME="vhd-scans"

if [ "$OS_VERSION" == "18.04" ]; then
    echo "Skipping scanning for 18.04"
    exit 0
fi

function cleanup() {
  az vm delete --name $VM_NAME_SCANNING --resource-group $RESOURCE_GROUP_NAME --yes
}
trap cleanup EXIT

#fix identity string
az vm create --resource-group $RESOURCE_GROUP_NAME \
    --name $VM_NAME_SCANNING \
    --image $VHD_IMAGE \
    --admin-username $TEST_VM_ADMIN_USERNAME \
    --admin-password $TEST_VM_ADMIN_PASSWORD \
    --os-disk-size-gb 50 \
    --assign-identity "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/vhd-scanning-UAMI"

TRIVY_PATH="$(dirname "$FULL_PATH")/trivy-scan.sh"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME_SCANNING \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$TRIVY_PATH

if [ "$OS_SKU" = "ubuntu" ]; then
    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME_SCANNING \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo apt-get install -y azure-cli"

elif [ "$OS_SKU" = "azure linux" ]; then
    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME_SCANNING \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo apt-get install -y ca-certificates curl apt-transport-https lsb-release gnupg &&
            curl -sL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add - &&
            echo \"deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ \$(lsb_release -cs) main\" | sudo tee /etc/apt/sources.list.d/azure-cli.list"
    
    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME_SCANNING \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo apt-get update -y && sudo apt-get upgrade -y"

    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME_SCANNING \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo apt-get install -y azure-cli"

elif [ "$OS_SKU" = "CBL mariner" ]; then
    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME_SCANNING \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc &&
            sudo sh -c 'echo -e \"[azure-cli]\nname=Azure CLI\nbaseurl=https://packages.microsoft.com/yumrepos/azure-cli\nenabled=1\ngpgcheck=1\ngpgkey=https://packages.microsoft.com/keys/microsoft.asc\" > /etc/yum.repos.d/azure-cli.repo' &&
            sudo dnf install -y azure-cli"
else
    echo "I don't know what you are, exiting shell script."
    exit 1
fi

az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME_SCANNING \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts "az login --identity"

TIMESTAMP=$(date +%s%3N)
TRIVY_REPORT_NAME="trivy-report-${BUILD_ID}-${TIMESTAMP}.json"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME_SCANNING \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts "az storage blob upload --file /opt/azure/containers/trivy-report.json \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${TRIVY_REPORT_NAME} \
    --account-name ${SIG_GALLERY_NAME} \
    --auth-mode login"

TRIVY_TABLE_NAME="trivy-table-${BUILD_ID}-${TIMESTAMP}.txt"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME_SCANNING \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts "az storage blob upload --file /opt/azure/containers/trivy-images-table.txt \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${TRIVY_TABLE_NAME} \
    --account-name ${SIG_GALLERY_NAME} \
    --auth-mode login"


az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_REPORT_NAME} --file trivy-report.json --account-name ${SIG_GALLERY_NAME} --auth-mode login
az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_TABLE_NAME} --file  trivy-images-table.txt --account-name ${SIG_GALLERY_NAME} --auth-mode login

az storage blob delete --account-name ${SIG_GALLERY_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_REPORT_NAME} --auth-mode login
az storage blob delete --account-name ${SIG_GALLERY_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_REPORT_NAME} --auth-mode login