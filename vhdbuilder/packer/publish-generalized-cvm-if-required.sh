#!/usr/bin/env bash

set -euo pipefail

SOURCE_VHD_URI="${1:?source VHD URI is required}"

if [[ "${FEATURE_FLAGS,,}" != *cvm* || ! "${OS_SKU,,}" =~ ^(ubuntu|azurelinux)$ ]]; then
    echo "Generalized CVM publication is not required for OS_SKU=${OS_SKU:-unset}, FEATURE_FLAGS=${FEATURE_FLAGS:-unset}"
    exit 0
fi

gallery_args=(
    --resource-group "${AZURE_RESOURCE_GROUP_NAME:?}"
    --gallery-name "${SIG_GALLERY_NAME:?}"
    --only-show-errors
)
definition_args=("${gallery_args[@]}" --gallery-image-definition "${SKU_NAME:?}")
version_args=("${definition_args[@]}" --gallery-image-version "${CAPTURED_SIG_VERSION:?}")

if [[ ! "${SOURCE_VHD_URI}" =~ ^https://([a-z0-9]{3,24})\.blob\.[^/]+/.+ ]]; then
    echo "Source VHD must be an Azure Storage blob URL" >&2
    exit 1
fi
source_storage_account="${BASH_REMATCH[1]}"
publication_stage="resolving source storage account (az storage account list)"
report_publication_failure() {
    local exit_code=$?
    echo "ERROR: Generalized CVM publication failed during ${publication_stage} (exit ${exit_code})" >&2
    exit "${exit_code}"
}
trap report_publication_failure ERR

echo "Publishing generalized CVM image ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} in resource group ${AZURE_RESOURCE_GROUP_NAME}"
echo "Resolving source storage account ${source_storage_account}"
source_storage_info="$(
    az storage account list \
        --query "[?name=='${source_storage_account}'].{id:id,location:location} | [0]" \
        --output json \
        --only-show-errors
)"
source_storage_id="$(jq -r '.id // empty' <<<"${source_storage_info}")"
source_location="$(jq -r '.location // empty' <<<"${source_storage_info}")"
if [[ -z "${source_storage_id}" || -z "${source_location}" ]]; then
    echo "Unable to resolve source storage account ${source_storage_account} and its location" >&2
    exit 1
fi
echo "Resolved source storage account ${source_storage_account} in ${source_location}"

publication_stage="looking up image definition ${SIG_GALLERY_NAME}/${SKU_NAME} (az sig image-definition list)"
echo "Looking up image definition ${SIG_GALLERY_NAME}/${SKU_NAME}"
definition_id="$(az sig image-definition list "${gallery_args[@]}" --query "[?name=='${SKU_NAME}'].id | [0]" --output tsv)"
if [[ -z "${definition_id}" || "${definition_id}" = "None" ]]; then
    publication_stage="creating image definition ${SIG_GALLERY_NAME}/${SKU_NAME} (az sig image-definition create)"
    echo "Image definition not found; creating generalized image definition ${SIG_GALLERY_NAME}/${SKU_NAME}"
    definition_id="$(
        az sig image-definition create "${definition_args[@]}" \
            --publisher microsoft-aks --offer "${SIG_GALLERY_NAME}" --sku "${SKU_NAME}" \
            --os-type Linux --os-state Generalized --architecture x64 --hyper-v-generation V2 \
            --location "${PACKER_BUILD_LOCATION:?}" \
            --features "DiskControllerTypes=SCSI,NVMe" "SecurityType=ConfidentialVMSupported" \
            --query id --output tsv
    )"
    echo "Created image definition ${definition_id}"
    publication_stage="enabling image definition updates (az resource update)"
    echo "Enabling allowUpdateImage on ${definition_id}"
    az resource update --ids "${definition_id}" --set properties.allowUpdateImage=true --only-show-errors >/dev/null
    echo "Enabled allowUpdateImage on ${definition_id}"
else
    echo "Using existing image definition ${definition_id}"
fi

publication_stage="creating image version ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} (az sig image-version create)"
CREATE_TIME="$(date +%s)"
echo "Creating image version ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} with buildId=${BUILD_BUILDID:?}, now=${CREATE_TIME}, source location ${source_location}, target region westus"
az sig image-version create "${version_args[@]}" \
    --location "${source_location}" \
    --os-vhd-uri "${SOURCE_VHD_URI}" \
    --os-vhd-storage-account "${source_storage_id}" \
    --target-regions westus \
    --tags "buildId=${BUILD_BUILDID:?}" "createdBy=aks-vhd-pipeline" "now=${CREATE_TIME}" >/dev/null
echo "Image version create succeeded for ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION}"

publication_stage="waiting for image version provisioning and replication (az sig image-version wait)"
echo "Waiting for image version ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION}: provisioningState=Succeeded and replicationStatus.aggregatedState=Completed"
az sig image-version wait "${version_args[@]}" --expand ReplicationStatus \
    --custom "provisioningState=='Succeeded' && replicationStatus.aggregatedState=='Completed'" \
    --interval 60

echo "Generalized CVM image ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} provisioned successfully"
