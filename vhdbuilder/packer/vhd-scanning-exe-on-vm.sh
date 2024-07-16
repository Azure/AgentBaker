#!/bin/bash
set -eux

OS_SKU=$1
OS_VERSION=$2
SCAN_VM_ADMIN_USERNAME=$3
ARCHITECTURE=$4
TRIVY_REPORT_NAME=$5
TRIVY_TABLE_NAME=$6
SIG_CONTAINER_NAME=$7
STORAGE_ACCOUNT_NAME=$8
ENABLE_TRUSTED_LAUNCH=$9

if [[ "$OS_SKU" == "Ubuntu" ]] && [[ "$OS_VERSION" == "20.04" ]]; then
    sudo apt-get install -y azure-cli
elif [[ "$OS_SKU" == "Ubuntu" ]] && [[ "$OS_VERSION" == "22.04" ]] && [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
    sudo apt update
    sudo apt install -y python3-pip
    pip install azure-cli
    export PATH="/home/$SCAN_VM_ADMIN_USERNAME/.local/bin:$PATH"
    CHECKAZ=$(pip freeze | grep "azure-cli==")
    if [[ -z $CHECKAZ ]]; then
        echo "Azure CLI is not installed properly."
        exit 1
    fi
elif [[ "$OS_SKU" == "Ubuntu" ]] && [[ "$OS_VERSION" == "22.04" ]] && [[ "${ARCHITECTURE,,}" != "arm64" ]]; then
    sudo apt-get install -y ca-certificates curl apt-transport-https lsb-release gnupg
    curl -sL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
    echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/azure-cli.list
    sudo apt-get update -y && sudo apt-get upgrade -y
    sudo apt-get install -y azure-cli
elif [ "$OS_SKU" == "CBLMariner" ] || [ "$OS_SKU" == "AzureLinux" ]; then
    sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc
    sudo sh -c 'echo -e "[azure-cli]\nname=Azure CLI\nbaseurl=https://packages.microsoft.com/yumrepos/azure-cli\nenabled=1\ngpgcheck=1\ngpgkey=https://packages.microsoft.com/keys/microsoft.asc" > /etc/yum.repos.d/azure-cli.repo'
    sudo dnf install -y azure-cli
else
    echo "Unrecognized SKU, Version, and Architecture combination for downloading az: $OS_SKU $OS_VERSION $ARCHITECTURE"
    exit 1
fi

if [[ "${ENABLE_TRUSTED_LAUNCH}" == "True" ]]; then
    az login --identity --allow-no-subscriptions
else
    az login --identity
fi

# trivy scan must have run before this
az storage blob upload --file /opt/azure/containers/trivy-report.json \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${TRIVY_REPORT_NAME} \
    --account-name ${STORAGE_ACCOUNT_NAME} \
    --auth-mode login

az storage blob upload --file /opt/azure/containers/trivy-images-table.txt \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${TRIVY_TABLE_NAME} \
    --account-name ${STORAGE_ACCOUNT_NAME} \
    --auth-mode login