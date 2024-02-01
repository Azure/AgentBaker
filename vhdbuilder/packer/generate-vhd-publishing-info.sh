#!/bin/bash -e

required_env_vars=(
    "CLASSIC_SA_CONNECTION_STRING"
    "STORAGE_ACCT_BLOB_URL"
    "VHD_NAME"
    "OS_NAME"
    "OFFER_NAME"
    "SKU_NAME"
    "HYPERV_GENERATION"
    "IMAGE_VERSION"
)

# Higher the replication_inverse, lower is the usage and number of replicas
set -x
REPLICATION_INVERSE=1
feature_set=("fips" "gpu" "arm64" "cvm" "tl" "kata")
if [ "${OFFER_NAME,,}" != "ubuntu" ]; then
    # Since Ubuntu is our most used SKU as compared to Windows/Mariner/AzLinux, we dont need the same number of replicas for all.
    # Starting off with half replicas.
    REPLICATION_INVERSE=$((REPLICATION_INVERSE * 2))
else
    # 1804 SKUs are not used as much since we defaulted to 22.04 with k8s 1.25 and the lowest supported k8s version supported is 1.25
    # We only support a certain set of functionalities for 2004(CVM, FIPs)
    # Therefore they dont need to be as high in number
    if [[ "${SKU_NAME,,}" != *"2204"* ]]; then
        REPLICATION_INVERSE=$((REPLICATION_INVERSE * 2))
    fi
fi

if [ "${HYPERV_GENERATION,,}" == "v1" ]; then
    # Gen2 SKUs are more used as compared to Gen1 SKUs, therefore Gen1 SKUs do not warrant the same number of replicas
    REPLICATION_INVERSE=$((REPLICATION_INVERSE * 2))
fi

for feature in "${feature_set[@]}"; do
    if [[ "${SKU_NAME,,}" == *"${feature}"* ]]; then
        REPLICATION_INVERSE=$((REPLICATION_INVERSE * 2))
        break
    fi
done
set +x

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        if [ "${OS_NAME,,}" == "linux" ]; then
            if [ "$v" == "IMAGE_VERSION" ]; then
                IMAGE_VERSION=$(date +%Y%m.%d.0)
                echo "$v was not set, set it to ${!v}"
            else
                echo "$v was not set!"
                exit 1
            fi
        else 
            echo "$v was not set for windows!"
            exit 1
        fi
    fi
done

# If building a linux-based VHD, correctly set the intermediate, or "captured" SIG image version resource ID so it can be used by AgentBaker E2E and release scripts.
if [ "${OS_NAME,,}" == "linux" ]; then
    [ -z "$SUBSCRIPTION_ID" ] && echo "SUBSCRIPTION_ID must be set when generating publishing info for linux" && exit 1
    [ -z "$RESOURCE_GROUP_NAME" ] && echo "RESOURCE_GROUP_NAME must be set when generating publishing info for linux" && exit 1
    [ -z "$SIG_IMAGE_NAME" ] && echo "SIG_IMAGE_NAME must be set when generating publishing info for linux" && exit 1
    [ -z "$CAPTURED_SIG_VERSION" ] && echo "CAPTURED_SIG_VERSION must be set when generating publishing info for linux" && exit 1

    INTERMEDIATE_SIG_GALLERY_NAME="PackerSigGalleryEastUS"

    captured_sig_resource_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${INTERMEDIATE_SIG_GALLERY_NAME}/images/${SIG_IMAGE_NAME}/versions/${CAPTURED_SIG_VERSION}"
    echo "captured intermediate SIG image version resource ID: $captured_sig_resource_id"
fi

# SIG image definition for AMD64/ARM64 has subtle difference, otherwise a SIG version cannot be used to create VM/VMSS of corresponding sku.
# 'az sig image-definition create' will have a new property (--architecture Arm64|x64) for this soon. We need this in the publishing-info
# in order that the VHD publish EV2 pipeline can create image-definition with right architecture.
if [[ ${ARCHITECTURE,,} == "arm64" ]]; then
    IMAGE_ARCH="Arm64"
else
    IMAGE_ARCH="x64"
fi

echo "generating traditional SAS token with CLASSIC_SA_CONNECTION_STRING..."
start_date=$(date +"%Y-%m-%dT00:00Z" -d "-1 day")
expiry_date=$(date +"%Y-%m-%dT00:00Z" -d "+1 year")
if [[ "${OS_NAME,,}" != "windows" ]]; then
    [ -z "${OUTPUT_STORAGE_CONTAINER_NAME}" ] && echo "OUTPUT_STORAGE_CONTAINER_NAME should be set..." && exit 1
    echo "storage container name: ${OUTPUT_STORAGE_CONTAINER_NAME}"
    # max of 7 day expiration time when using user delegation SAS
    sas_token=$(az storage container generate-sas --name ${OUTPUT_STORAGE_CONTAINER_NAME} --permissions lr --connection-string ${CLASSIC_SA_CONNECTION_STRING} --start ${start_date} --expiry ${expiry_date} | tr -d '"')
else
    # we still need to use the original connection string when not using a system-assigned identity on 1ES pools
    sas_token=$(az storage container generate-sas --name vhds --permissions lr --connection-string ${CLASSIC_SA_CONNECTION_STRING} --start ${start_date} --expiry ${expiry_date} | tr -d '"')
fi

if [ "$sas_token" == "" ]; then
    echo "sas_token is empty"
    exit 1
fi
vhd_url="${STORAGE_ACCT_BLOB_URL}/${VHD_NAME}?$sas_token"

echo "Testing whether the generated sas token works"
vhd_size=$(curl -sI $vhd_url | grep -i Content-Length | awk '{print $2}')
if [ "$vhd_size" == "" ]; then
    echo "The genrated sas token does not work"
    exit 1
fi
echo "The generated sas token works"

# Do not log sas token
echo "COPY ME ---> ${STORAGE_ACCT_BLOB_URL}/${VHD_NAME}?***"

# Note: The offer_name is the value from OS_SKU (eg. Ubuntu)
if [ "${OS_NAME,,}" == "linux" ]; then
    cat <<EOF > vhd-publishing-info.json
{
    "vhd_url": "$vhd_url",
    "captured_sig_resource_id": "${captured_sig_resource_id}",
    "os_name": "$OS_NAME",
    "sku_name": "$SKU_NAME",
    "offer_name": "$OFFER_NAME",
    "hyperv_generation": "${HYPERV_GENERATION}",
    "image_architecture": "${IMAGE_ARCH}",
    "image_version": "${IMAGE_VERSION}",
    "replication_inverse": "${REPLICATION_INVERSE}"
}
EOF
else
    cat <<EOF > vhd-publishing-info.json
{
    "vhd_url": "$vhd_url",
    "os_name": "$OS_NAME",
    "sku_name": "$SKU_NAME",
    "offer_name": "$OFFER_NAME",
    "hyperv_generation": "${HYPERV_GENERATION}",
    "image_architecture": "${IMAGE_ARCH}",
    "image_version": "${IMAGE_VERSION}",
    "replication_inverse": "${REPLICATION_INVERSE}"
}
EOF
fi

# Do not log sas token
sed 's/?.*\",/?***\",/g' < vhd-publishing-info.json