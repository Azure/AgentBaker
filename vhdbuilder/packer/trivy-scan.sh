#!/usr/bin/env bash
set -euxo pipefail

TRIVY_REPORT_DIRNAME=/opt/azure/containers
TRIVY_REPORT_ROOTFS_JSON_PATH=${TRIVY_REPORT_DIRNAME}/trivy-report-rootfs.json
TRIVY_REPORT_IMAGE_TABLE_PATH=${TRIVY_REPORT_DIRNAME}/trivy-report-images-table.txt

TRIVY_VERSION="0.53.0"
TRIVY_ARCH=""

MODULE_NAME="vuln-to-kusto-vhd"

OS_SKU=${1}
OS_VERSION=${2}
TEST_VM_ADMIN_USERNAME=${3}
ARCHITECTURE=${4}
SIG_CONTAINER_NAME=${5}
STORAGE_ACCOUNT_NAME=${6}
ENABLE_TRUSTED_LAUNCH=${7}
VHD_NAME=${8}
SKU_NAME=${9}
KUSTO_ENDPOINT=${10}
KUSTO_DATABASE=${11}
KUSTO_TABLE=${12}
TRIVY_UPLOAD_REPORT_NAME=${13}
TRIVY_UPLOAD_TABLE_NAME=${14}
ACCOUNT_NAME=${15}
BLOB_URL=${16}
SEVERITY=${17}
MODULE_VERSION=${18}
UMSI_PRINCIPAL_ID=${19}
UMSI_CLIENT_ID=${20}
BUILD_RUN_NUMBER=${21}
export BUILD_REPOSITORY_NAME=${22}
export BUILD_SOURCEBRANCH=${23}
export BUILD_SOURCEVERSION=${24}
export SYSTEM_COLLECTIONURI=${25}
export SYSTEM_TEAMPROJECT=${26}
export BUILD_BUILDID=${27}

install_azure_cli() {
    OS_SKU=${1}
    OS_VERSION=${2}
    ARCHITECTURE=${3}
    TEST_VM_ADMIN_USERNAME=${4}

    if [[ "$OS_SKU" == "Ubuntu" ]] && [[ "$OS_VERSION" == "20.04" ]]; then
        sudo apt-get install -y azure-cli
    elif [[ "$OS_SKU" == "Ubuntu" ]] && [[ "$OS_VERSION" == "22.04" ]] && [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
        sudo apt update
        sudo apt install -y python3-pip
        pip install azure-cli
        export PATH="/home/$TEST_VM_ADMIN_USERNAME/.local/bin:$PATH"
        CHECKAZ=$(pip freeze | grep "azure-cli==")
        if [[ -z $CHECKAZ ]]; then
            echo "Azure CLI is not installed properly."
            exit 1
        fi
    elif [[ "$OS_SKU" == "Ubuntu" ]] && [[ "$OS_VERSION" == "24.04" ]] && [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
        sudo apt-get install -y ca-certificates curl apt-transport-https lsb-release gnupg
        curl -sL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
        echo "deb [arch=arm64] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/azure-cli.list
        sudo apt-get update -y && sudo apt-get upgrade -y
        sudo apt-get install -y azure-cli
    elif [[ "$OS_SKU" == "Ubuntu" ]] && { [[ "$OS_VERSION" == "22.04" ]] || [[ "$OS_VERSION" == "24.04" ]]; } && [[ "${ARCHITECTURE,,}" != "arm64" ]]; then
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
}

install_azure_cli $OS_SKU $OS_VERSION $ARCHITECTURE $TEST_VM_ADMIN_USERNAME

if [[ "${ENABLE_TRUSTED_LAUNCH}" == "True" ]]; then
    az login --identity --allow-no-subscriptions --username ${UMSI_PRINCIPAL_ID}
else
    az login --identity
fi

arch="$(uname -m)"
if [ "${arch,,}" == "arm64" ] || [ "${arch,,}" == "aarch64" ]; then
    TRIVY_ARCH="Linux-ARM64"
    GO_ARCH="arm64"
elif [ "${arch,,}" == "x86_64" ]; then
    TRIVY_ARCH="Linux-64bit"
    GO_ARCH="amd64"
else
    echo "invalid architecture ${arch,,}"
    exit 1
fi

mkdir -p "$(dirname "${TRIVY_REPORT_DIRNAME}")"

wget "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
tar -xvzf "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
rm "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
chmod a+x trivy 

# pull vuln-to-kusto binary
az storage blob download --auth-mode login --account-name ${ACCOUNT_NAME} -c vuln-to-kusto \
    --name ${MODULE_VERSION}/${MODULE_NAME}_linux_${GO_ARCH} \
    --file ./${MODULE_NAME}
chmod a+x ${MODULE_NAME}

# shellcheck disable=SC2155
export PATH="$(pwd):$PATH"

./trivy --scanners vuln rootfs -f json --skip-dirs /var/lib/containerd --ignore-unfixed --severity ${SEVERITY} -o "${TRIVY_REPORT_ROOTFS_JSON_PATH}" /
if [[ -f ${TRIVY_REPORT_ROOTFS_JSON_PATH} ]]; then
    ./vuln-to-kusto-vhd scan-report \
        --vhd-buildrunnumber=${BUILD_RUN_NUMBER} \
        --vhd-vhdname="${VHD_NAME}" \
        --vhd-ossku="${OS_SKU}" \
        --vhd-osversion="${OS_VERSION}" \
        --vhd-skuname="${SKU_NAME}" \
        --kusto-endpoint=${KUSTO_ENDPOINT} \
        --kusto-database=${KUSTO_DATABASE} \
        --kusto-table=${KUSTO_TABLE} \
        --kusto-managed-identity-client-id=${UMSI_CLIENT_ID} \
        ${TRIVY_REPORT_ROOTFS_JSON_PATH}
fi

IMAGE_LIST=$(ctr -n k8s.io image list -q | grep -v sha256)

echo "This contains the list of images with high and critical level CVEs (if present), that are present in the node. 
Note: images without CVEs are also listed" >> "${TRIVY_REPORT_IMAGE_TABLE_PATH}"

for CONTAINER_IMAGE in $IMAGE_LIST; do
    # append to table
    ./trivy --scanners vuln image --ignore-unfixed --severity ${SEVERITY} -f table ${CONTAINER_IMAGE} >> ${TRIVY_REPORT_IMAGE_TABLE_PATH} || true

    # export to Kusto, one by one
    BASE_CONTAINER_IMAGE=$(basename ${CONTAINER_IMAGE})
    TRIVY_REPORT_IMAGE_JSON_PATH=${TRIVY_REPORT_DIRNAME}/trivy-report-image-${BASE_CONTAINER_IMAGE}.json
    ./trivy --scanners vuln image -f json --ignore-unfixed --severity ${SEVERITY} -o ${TRIVY_REPORT_IMAGE_JSON_PATH} $CONTAINER_IMAGE || true

    if [[ -f ${TRIVY_REPORT_IMAGE_JSON_PATH} ]]; then
        ./vuln-to-kusto-vhd scan-report \
            --vhd-buildrunnumber=${BUILD_RUN_NUMBER} \
            --vhd-vhdname="${VHD_NAME}" \
            --vhd-ossku="${OS_SKU}" \
            --vhd-osversion="${OS_VERSION}" \
            --vhd-skuname="${SKU_NAME}" \
            --kusto-endpoint=${KUSTO_ENDPOINT} \
            --kusto-database=${KUSTO_DATABASE} \
            --kusto-table=${KUSTO_TABLE} \
            --kusto-managed-identity-client-id=${UMSI_CLIENT_ID} \
            ${TRIVY_REPORT_IMAGE_JSON_PATH} || true

        rm ${TRIVY_REPORT_IMAGE_JSON_PATH} || true
    fi
done

rm ./trivy 

chmod a+r "${TRIVY_REPORT_ROOTFS_JSON_PATH}"
chmod a+r "${TRIVY_REPORT_IMAGE_TABLE_PATH}"

az storage blob upload --file ${TRIVY_REPORT_ROOTFS_JSON_PATH} \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${TRIVY_UPLOAD_REPORT_NAME} \
    --account-name ${STORAGE_ACCOUNT_NAME} \
    --auth-mode login

az storage blob upload --file ${TRIVY_REPORT_IMAGE_TABLE_PATH} \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${TRIVY_UPLOAD_TABLE_NAME} \
    --account-name ${STORAGE_ACCOUNT_NAME} \
    --auth-mode login
