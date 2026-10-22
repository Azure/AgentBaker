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
report_publication_failure() {
    local exit_code="${1:-$?}"
    echo "ERROR: Generalized CVM publication failed (exit ${exit_code})" >&2
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

echo "Looking up image definition ${SIG_GALLERY_NAME}/${SKU_NAME}"
definition_id="$(az sig image-definition list "${gallery_args[@]}" --query "[?name=='${SKU_NAME}'].id | [0]" --output tsv)"
if [[ -z "${definition_id}" || "${definition_id}" = "None" ]]; then
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
    echo "Enabling allowUpdateImage on ${definition_id}"
    az resource update --ids "${definition_id}" --set properties.allowUpdateImage=true --only-show-errors >/dev/null
    echo "Enabled allowUpdateImage on ${definition_id}"
else
    echo "Using existing image definition ${definition_id}"
fi

CREATE_TIME="$(date +%s)"
version_tags=(
    "buildDefinitionName=${BUILD_DEFINITION_NAME:?}"
    "buildNumber=${BUILD_NUMBER:?}"
    "buildId=${BUILD_BUILDID:?}"
    "SkipLinuxAzSecPack=true"
    "os=Linux"
    "now=${CREATE_TIME}"
    "createdBy=aks-vhd-pipeline"
    "image_sku=${IMG_SKU:?}"
    "branch=${BRANCH:?}"
)
echo "Creating image version ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} with buildId=${BUILD_BUILDID:?}, now=${CREATE_TIME}, source location ${source_location}, target region ${PACKER_BUILD_LOCATION:?}"
echo "Image version tags: ${version_tags[*]}"
az sig image-version create "${version_args[@]}" \
    --location "${source_location}" \
    --os-vhd-uri "${SOURCE_VHD_URI}" \
    --os-vhd-storage-account "${source_storage_id}" \
    --target-regions "${PACKER_BUILD_LOCATION}" \
    --tags "${version_tags[@]}" >/dev/null
echo "Image version creation completed for ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION}; checking replication"

echo "Waiting for image version ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} replication to ${PACKER_BUILD_LOCATION} to complete"
echo "Verifying image version ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} is ready in ${PACKER_BUILD_LOCATION}"
replication_deadline=$((SECONDS + 900))
while true; do
    image_status="$(
        az sig image-version show "${version_args[@]}" --expand ReplicationStatus \
            --query '{state:provisioningState,replication:replicationStatus}' --output json
    )" || { show_status=$?; [[ "${show_status}" -eq 3 ]] || report_publication_failure "${show_status}"; }
    echo "Image version status: ${image_status:-not found}"
    readiness="$(jq -r --arg region "${PACKER_BUILD_LOCATION}" '
        if .state == "Succeeded" and
            any(.replication.summary[]?; .region == $region and .state == "Completed") then "ready"
        elif .state == "Failed" or .state == "Canceled" or
            any(.replication.summary[]?; .region == $region and .state == "Failed") then "failed"
        else "pending" end
    ' <<<"${image_status:-null}")"
    [[ "${readiness}" != "ready" ]] || break
    [[ "${readiness}" != "failed" ]] || { echo "ERROR: Image provisioning or replication failed" >&2; exit 1; }
    ((SECONDS < replication_deadline)) || { echo "ERROR: Image is not ready in ${PACKER_BUILD_LOCATION} after waiting 900 seconds for replication" >&2; exit 1; }
    echo "Image version is not ready in ${PACKER_BUILD_LOCATION}; retrying in 60 seconds"
    sleep 60
done

echo "Image version ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} replicated successfully to ${PACKER_BUILD_LOCATION}"
echo "Generalized CVM image ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION}: image ready in ${PACKER_BUILD_LOCATION}"
