#!/bin/bash
set -eux
RESOURCE_GROUP_NAME="aksvhdtestbuildrg"
VM_NAME="alison-vhd-test-vm"
TEST_VM_ADMIN_USERNAME="azureuser"
TEST_VM_ADMIN_PASSWORD="TestVM@1715622512"
#VHD_IMAGE_MARINER="/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2/versions/1.1716314854.9962"
VHD_IMAGE="/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2TLGen2/versions/1.1716575690.29480"

#function cleanup() {
#  az vm delete --name $VM_NAME --resource-group $RESOURCE_GROUP_NAME --yes
#}
#trap cleanup EXIT

# use os version to exit if 18.04

# create the VM
#az vm create --resource-group $RESOURCE_GROUP_NAME \
#    --name $VM_NAME \
#    --image $VHD_IMAGE \
#    --admin-username $TEST_VM_ADMIN_USERNAME \
#    --admin-password $TEST_VM_ADMIN_PASSWORD \
#    --os-disk-size-gb 50 \
#    --assign-identity "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/vhd-scanning-UAMI"

#SCRIPT_PATH="/Users/alisonburgess/Documents/development/agentBaker/AgentBaker/vhdbuilder/packer/trivy-scan.sh"
#az vm run-command invoke \
#    --command-id RunShellScript \
#    --name $VM_NAME \
#    --resource-group $RESOURCE_GROUP_NAME \
#    --scripts @$SCRIPT_PATH

#cat /etc/os-release
# use os version / use os_sku for when to use mariner and when extra steps are needed
#isUbuntu=$(az vm run-command invoke \
#    --command-id RunShellScript \
#    --name $VM_NAME \
#    --resource-group $RESOURCE_GROUP_NAME \
#    --scripts 'if grep -q "^NAME=\"Ubuntu\"$" /etc/os-release; then echo "true"; else echo "false"; fi' \
#    --query "value[0].message" -o tsv)

#if [ "$isUbuntu" == "true" ]; then
#    az vm run-command invoke \
#    --command-id RunShellScript \
#    --name $VM_NAME \
#    --resource-group $RESOURCE_GROUP_NAME \
#    --scripts "sudo apt-get install -y ca-certificates curl apt-transport-https lsb-release gnupg &&
#        curl -sL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add - &&
#        echo \"deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ \$(lsb_release -cs) main\" | sudo tee /etc/apt/sources.list.d/azure-cli.list"
    
#    az vm run-command invoke \
#        --command-id RunShellScript \
#        --name $VM_NAME \
#        --resource-group $RESOURCE_GROUP_NAME \
#        --scripts "sudo apt-get update -y && sudo apt-get upgrade -y"

#    az vm run-command invoke \
#        --command-id RunShellScript \
#        --name $VM_NAME \
#        --resource-group $RESOURCE_GROUP_NAME \
#        --scripts "sudo apt-get install -y azure-cli"
#else
    # mariner 
#    az vm run-command invoke \
#        --command-id RunShellScript \
#        --name $VM_NAME \
#        --resource-group $RESOURCE_GROUP_NAME \
#        --scripts "sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc &&
#            sudo sh -c 'echo -e \"[azure-cli]\nname=Azure CLI\nbaseurl=https://packages.microsoft.com/yumrepos/azure-cli\nenabled=1\ngpgcheck=1\ngpgkey=https://packages.microsoft.com/keys/microsoft.asc\" > /etc/yum.repos.d/azure-cli.repo' &&
#            sudo dnf install -y azure-cli"
#fi

#az vm run-command invoke \
#    --command-id RunShellScript \
#    --name $VM_NAME \
#    --resource-group $RESOURCE_GROUP_NAME \
#    --scripts "az login --identity"

# make the name unique

#az vm run-command invoke \
#    --command-id RunShellScript \
#    --name $VM_NAME \
#    --resource-group $RESOURCE_GROUP_NAME \
#    --scripts "az storage blob upload --file /opt/azure/containers/trivy-report.json \
#    --container-name "vhd-scans" \
#    --name "trivy-report-addtimestamp.json" \
#    --account-name "vhdbuildereastustest" \
#    --auth-mode login"

#az vm run-command invoke \
#    --command-id RunShellScript \
#    --name $VM_NAME \
#    --resource-group $RESOURCE_GROUP_NAME \
#    --scripts "az storage blob upload --file /opt/azure/containers/trivy-images-table.txt \
#    --container-name "vhd-scans" \
#    --name "trivy-table-addtimestamp.txt" \
#    --account-name "vhdbuildereastustest" \
#    --auth-mode login"


#az storage blob download --container-name "vhd-scans" --name "trivy-report-test2.json" --file "trivy-report-addtimestamp.json" --account-name "vhdbuildereastustest" --auth-mode login
#az storage blob download --container-name "vhd-scans" --name "trivy-report-test2.json" --file "trivy-table-addtimestamp.txt" --account-name "vhdbuildereastustest" --auth-mode login

#az storage blob delete --container-name "vhd-scans" --name "trivy-report-test2.json" --file "trivy-report-addtimestamp.json" --account-name "vhdbuildereastustest" --auth-mode login
#az storage blob delete --container-name "vhd-scans" --name "trivy-report-test2.json" --file "trivy-table-addtimestamp.txt" --account-name "vhdbuildereastustest" --auth-mode login

###################################################################################################################################################
# az vm delete --name $VM_NAME --resource-group $RESOURCE_GROUP_NAME --yes
R2_GROUP_NAME="alison-testing-rg-TL-trivt"
AZURE_LOCATION="eastus"

az vm delete --name $VM_NAME --resource-group $RESOURCE_GROUP_NAME --yes
az vm delete --name $VM_NAME --resource-group $R2_GROUP_NAME --yes

az group create --name $R2_GROUP_NAME --location ${AZURE_LOCATION} --tags 'source=AgentBaker'
az vm create --resource-group $R2_GROUP_NAME \
    --name $VM_NAME \
    --image $VHD_IMAGE \
    --admin-username $TEST_VM_ADMIN_USERNAME \
    --admin-password $TEST_VM_ADMIN_PASSWORD \
    --os-disk-size-gb 50 \
    --assign-identity "[system]" \
    --scope "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Storage/storageAccounts/vhdbuildereastustest/blobServices/default/containers/vhd-scans" \
    --role "Storage Blob Data Contributor" \
    --size Standard_DS1_v2 --security-type TrustedLaunch --enable-secure-boot true --enable-vtpm true

#az vm identity assign --name $VM_NAME --resource-group $R2_GROUP_NAME
#OBJ_ID=$(az vm identity show --name $VM_NAME --resource-group $R2_GROUP_NAME --query principalId --output tsv)
az role assignment create --assignee $OBJ_ID --role "Storage Blob Data Contributor" --scope "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Storage/storageAccounts/vhdbuildereastustest/blobServices/default/containers/vhd-scans"
az role assignment create --assignee $OBJ_ID --role "Storage Blob Data Owner" --scope "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Storage/storageAccounts/vhdbuildereastustest/blobServices/default/containers/vhd-scans"

#     --assign-identity "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/vhd-scanning-identity" \

SCRIPT_PATH="/Users/alisonburgess/Documents/development/agentBaker/AgentBaker/vhdbuilder/packer/trivy-scan.sh"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME \
    --resource-group $R2_GROUP_NAME \
    --scripts @$SCRIPT_PATH

# has to run after the trivy scan 
OS_SKU="CBLMariner"
OS_VERSION="V2"
# SCRIPT_PATH="/Users/alisonburgess/Documents/development/agentBaker/AgentBaker/vhdbuilder/packer/vhd-scanning-vm-exe.sh"
ARCHITECTURE="V2"
TIMESTAMP=$(date +%s%3N)
TRIVY_REPORT_NAME="trivy-report-1-${TIMESTAMP}.json"
TRIVY_TABLE_NAME="trivy-table-1-${TIMESTAMP}.txt"
# EXE_SCRIPT_PATH="$CDIR/$EXE_SCRIPT_PATH"
EXE_SCRIPT_PATH="/Users/alisonburgess/Documents/development/agentBaker/AgentBaker/vhdbuilder/packer/vhd-scanning-exe-on-vm.sh"
SIG_CONTAINER_NAME="vhd-scans"
STORAGE_ACCOUNT_NAME="vhdbuildereastustest"
ENABLE_TRUSTED_LAUNCH="X86_64"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME \
    --resource-group $R2_GROUP_NAME \
    --scripts @$EXE_SCRIPT_PATH \
    --parameters "OS_SKU=${OS_SKU}" \
        "OS_VERSION=${OS_VERSION}" \
        "TEST_VM_ADMIN_USERNAME=${TEST_VM_ADMIN_USERNAME}" \
        "ARCHITECTURE=${ARCHITECTURE}" \
        "TRIVY_REPORT_NAME=${TRIVY_REPORT_NAME}" \
        "TRIVY_TABLE_NAME=${TRIVY_TABLE_NAME}" \
        "SIG_CONTAINER_NAME"=${SIG_CONTAINER_NAME} \
        "STORAGE_ACCOUNT_NAME"=${STORAGE_ACCOUNT_NAME} \
        ENABLE_TRUSTED_LAUNCH=${ENABLE_TRUSTED_LAUNCH}

az storage blob download --container-name "vhd-scans" --name "trivy-report-test2.json" --file "trivy-report-addtimestamp.json" --account-name "vhdbuildereastustest" --auth-mode login
#az storage blob download --container-name "vhd-scans" --name "trivy-report-test2.json" --file "trivy-table-addtimestamp.txt" --account-name "vhdbuildereastustest" --auth-mode login

#az storage blob delete --container-name "vhd-scans" --name "trivy-report-test2.json" --file "trivy-report-addtimestamp.json" --account-name "vhdbuildereastustest" --auth-mode login
#az storage blob delete --container-name "vhd-scans" --name "trivy-report-test2.json" --file "trivy-table-addtimestamp.txt" --account-name "vhdbuildereastustest" --auth-mode login



# az role assignment create --role "Storage Blob Data Owner" --assignee e83bc3a5-5edf-45e6-bd0d-223995bcab46 --scope "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Storage/storageAccounts/vhdbuildereastustest/blobServices/default/containers/vhd-scans"