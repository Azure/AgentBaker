#!/usr/bin/env bash
set -euxo pipefail

TRIVY_REPORT_DIRNAME=/opt/azure/containers
TRIVY_REPORT_ROOTFS_JSON_PATH=${TRIVY_REPORT_DIRNAME}/trivy-report-rootfs.json
TRIVY_REPORT_IMAGE_TABLE_PATH=${TRIVY_REPORT_DIRNAME}/trivy-report-images-table.txt
CVE_DIFF_QUERY_OUTPUT_PATH=${TRIVY_REPORT_DIRNAME}/cve-diff.txt
CVE_LIST_QUERY_OUTPUT_PATH=${TRIVY_REPORT_DIRNAME}/cve-list.txt
TRIVY_DB_REPOSITORIES="mcr.microsoft.com/mirror/ghcr/aquasecurity/trivy-db:2,ghcr.io/aquasecurity/trivy-db:2,public.ecr.aws/aquasecurity/trivy-db"

TRIVY_VERSION="0.57.0"
TRIVY_ARCH=""

MODULE_NAME="vuln-to-kusto-vhd"

OS_SKU=${1}
OS_VERSION=${2}
TEST_VM_ADMIN_USERNAME=${3}
ARCHITECTURE=${4}
SIG_CONTAINER_NAME=${5}
STORAGE_ACCOUNT_NAME=${6}
ENABLE_TRUSTED_LAUNCH=${7}
VHD_ARTIFACT_NAME=${8}
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
AZURE_MSI_RESOURCE_STRING=${21}
BUILD_RUN_NUMBER=${22}
export BUILD_REPOSITORY_NAME=${23}
export BUILD_SOURCEBRANCH=${24}
export BUILD_SOURCEVERSION=${25}
export SYSTEM_COLLECTIONURI=${26}
export SYSTEM_TEAMPROJECT=${27}
export BUILD_BUILDID=${28}
export IMAGE_VERSION=${29}
CVE_DIFF_UPLOAD_REPORT_NAME=${30}
CVE_LIST_UPLOAD_REPORT_NAME=${31}
SCAN_RESOURCE_PREFIX=${32}

source /opt/azure/containers/provision_source_distro.sh

retrycmd_if_failure() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        timeout $timeout "${@}" && break || \
        if [ $i -eq $retries ]; then
            echo Executed \"$@\" $i times;
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed \"$@\" $i times;
}

install_azure_cli() {
    OS_SKU=${1}
    OS_VERSION=${2}
    ARCHITECTURE=${3}
    TEST_VM_ADMIN_USERNAME=${4}

    if [ "$OS_SKU" = "Ubuntu" ] && [ "$OS_VERSION" = "22.04" ] && [ "${ARCHITECTURE,,}" = "arm64" ]; then
        apt_get_update
        apt_get_install 5 1 60 python3-pip
        pip install azure-cli
        export PATH="/home/$TEST_VM_ADMIN_USERNAME/.local/bin:$PATH"
        CHECKAZ=$(pip freeze | grep "azure-cli==")
        if [ -z $CHECKAZ ]; then
            echo "Azure CLI is not installed properly."
            exit 1
        fi
    elif [ "$OS_SKU" = "Ubuntu" ] && [ "$OS_VERSION" = "24.04" ] && [ "${ARCHITECTURE,,}" = "arm64" ]; then
        apt_get_install 5 1 60 ca-certificates curl apt-transport-https lsb-release gnupg
        curl -sL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
        echo "deb [arch=arm64] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/azure-cli.list
        apt_get_update
        apt_get_install 5 1 60 azure-cli
    elif [ "$OS_SKU" = "Ubuntu" ] && { [ "$OS_VERSION" = "20.04" ] || [ "$OS_VERSION" = "22.04" ] || [ "$OS_VERSION" = "24.04" ]; } && [ "${ARCHITECTURE,,}" != "arm64" ]; then
        apt_get_install 5 1 60 ca-certificates curl apt-transport-https lsb-release gnupg
        curl -sL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
        echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/azure-cli.list
        apt_get_update
        apt_get_install 5 1 60 azure-cli
    elif [ "$OS_SKU" = "CBLMariner" ] || [ "$OS_SKU" = "AzureLinux" ]; then
        sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc
        sudo sh -c 'echo -e "[azure-cli]\nname=Azure CLI\nbaseurl=https://packages.microsoft.com/yumrepos/azure-cli\nenabled=1\ngpgcheck=1\ngpgkey=https://packages.microsoft.com/keys/microsoft.asc" > /etc/yum.repos.d/azure-cli.repo'
        sudo dnf install -y azure-cli
    elif [ "$OS_SKU" = "Flatcar" ] || [ "$OS_SKU" = "AzureLinuxOSGuard" ]; then
        python3 -m venv "/home/$TEST_VM_ADMIN_USERNAME/venv"
        export PATH="/home/$TEST_VM_ADMIN_USERNAME/venv/bin:$PATH"
        pip install azure-cli
        CHECKAZ=$(pip freeze | grep "azure-cli==")
        if [ -z $CHECKAZ ]; then
            echo "Azure CLI is not installed properly."
            exit 1
        fi
    else
        echo "Unrecognized SKU, Version, and Architecture combination for downloading az: $OS_SKU $OS_VERSION $ARCHITECTURE"
        exit 1
    fi
}

login_with_user_assigned_managed_identity() {
    local TYPE_FLAG="$1"
    local ID=$2

    LOGIN_FLAGS="--identity $TYPE_FLAG $ID"
    if [ "${ENABLE_TRUSTED_LAUNCH,,}" = "true" ]; then
        LOGIN_FLAGS="$LOGIN_FLAGS --allow-no-subscriptions"
    fi

   echo "logging into azure with flags: $LOGIN_FLAGS"
   az login $LOGIN_FLAGS
}
login_with_umsi_object_id() {
    login_with_user_assigned_managed_identity "--object-id" "$1"
}
login_with_umsi_resource_id() {
    login_with_user_assigned_managed_identity "--resource-id" "$1"
}

install_azure_cli $OS_SKU $OS_VERSION $ARCHITECTURE $TEST_VM_ADMIN_USERNAME

login_with_umsi_object_id ${UMSI_PRINCIPAL_ID}

arch="$(uname -m)"
if [ "${arch,,}" = "arm64" ] || [ "${arch,,}" = "aarch64" ]; then
    TRIVY_ARCH="Linux-ARM64"
    GO_ARCH="arm64"
elif [ "${arch,,}" = "x86_64" ]; then
    TRIVY_ARCH="Linux-64bit"
    GO_ARCH="amd64"
else
    echo "invalid architecture ${arch,,}"
    exit 1
fi

mkdir -p "$(dirname "${TRIVY_REPORT_DIRNAME}")"

curl -fL -o "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz" "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
tar -xvzf "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz" --no-same-owner
rm "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
chmod a+x trivy 

# pull vuln-to-kusto binary
az storage blob download --auth-mode login --account-name ${ACCOUNT_NAME} -c vuln-to-kusto \
    --name ${MODULE_VERSION}/${MODULE_NAME}_linux_${GO_ARCH} \
    --file ./${MODULE_NAME}
chmod a+x ${MODULE_NAME}

# shellcheck disable=SC2155
export PATH="$(pwd):$PATH"

# we do a delayed retry here since it's possible we'll get rate-limited by ghcr.io, which hosts the vulnerability DB
retrycmd_if_failure 10 30 600 ./trivy --scanners vuln rootfs -f json --db-repository ${TRIVY_DB_REPOSITORIES} --skip-dirs /var/lib/containerd --ignore-unfixed --severity ${SEVERITY} -o "${TRIVY_REPORT_ROOTFS_JSON_PATH}" /

if [ -f ${TRIVY_REPORT_ROOTFS_JSON_PATH} ]; then
    ./vuln-to-kusto-vhd scan-report \
        --vhd-buildrunnumber=${BUILD_RUN_NUMBER} \
        --vhd-vhdname="${VHD_ARTIFACT_NAME}" \
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
    ./trivy --scanners vuln image --ignore-unfixed --severity ${SEVERITY} --db-repository ${TRIVY_DB_REPOSITORIES} --skip-db-update -f table ${CONTAINER_IMAGE} >> ${TRIVY_REPORT_IMAGE_TABLE_PATH} || true

    # export to Kusto, one by one
    BASE_CONTAINER_IMAGE=$(basename ${CONTAINER_IMAGE})
    TRIVY_REPORT_IMAGE_JSON_PATH=${TRIVY_REPORT_DIRNAME}/trivy-report-image-${BASE_CONTAINER_IMAGE}.json
    ./trivy --scanners vuln image -f json --ignore-unfixed --severity ${SEVERITY} --db-repository ${TRIVY_DB_REPOSITORIES} --skip-db-update -o ${TRIVY_REPORT_IMAGE_JSON_PATH} $CONTAINER_IMAGE || true

    if [ -f ${TRIVY_REPORT_IMAGE_JSON_PATH} ]; then
        ./vuln-to-kusto-vhd scan-report \
            --vhd-buildrunnumber=${BUILD_RUN_NUMBER} \
            --vhd-vhdname="${VHD_ARTIFACT_NAME}" \
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

./vuln-to-kusto-vhd query-report query-diff 24h \
    --vhd-vhdname=${VHD_ARTIFACT_NAME} \
    --vhd-nodeimageversion=${IMAGE_VERSION} \
    --severity="HIGH" \
    --scan-resource-prefix=${SCAN_RESOURCE_PREFIX} \
    --kusto-endpoint=${KUSTO_ENDPOINT} \
    --kusto-database=${KUSTO_DATABASE} \
    --kusto-table=${KUSTO_TABLE} \
    --kusto-managed-identity-client-id=${UMSI_CLIENT_ID} >> ${CVE_DIFF_QUERY_OUTPUT_PATH}

./vuln-to-kusto-vhd query-report query-list 24h \
    --vhd-vhdname=${VHD_ARTIFACT_NAME} \
    --vhd-nodeimageversion=${IMAGE_VERSION} \
    --severity="HIGH" \
    --scan-resource-prefix=${SCAN_RESOURCE_PREFIX} \
    --kusto-endpoint=${KUSTO_ENDPOINT} \
    --kusto-database=${KUSTO_DATABASE} \
    --kusto-table=${KUSTO_TABLE} \
    --kusto-managed-identity-client-id=${UMSI_CLIENT_ID} >> ${CVE_LIST_QUERY_OUTPUT_PATH}

rm ./trivy

chmod a+r "${CVE_DIFF_QUERY_OUTPUT_PATH}"
chmod a+r "${TRIVY_REPORT_ROOTFS_JSON_PATH}"
chmod a+r "${TRIVY_REPORT_IMAGE_TABLE_PATH}"
chmod a+r "${CVE_LIST_QUERY_OUTPUT_PATH}"

login_with_umsi_resource_id ${AZURE_MSI_RESOURCE_STRING}

az storage blob upload --file ${CVE_DIFF_QUERY_OUTPUT_PATH} \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${CVE_DIFF_UPLOAD_REPORT_NAME} \
    --account-name ${STORAGE_ACCOUNT_NAME} \
    --auth-mode login

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

az storage blob upload --file ${CVE_LIST_QUERY_OUTPUT_PATH} \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${CVE_LIST_UPLOAD_REPORT_NAME} \
    --account-name ${STORAGE_ACCOUNT_NAME} \
    --auth-mode login
