#!/usr/bin/env bash

set -euo pipefail

SOURCE_VHD_URI="${1:?source VHD URI or --check-only is required}"

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

if [[ "${SOURCE_VHD_URI}" = "--check-only" ]]; then
    version_info="$(az sig image-version show "${version_args[@]}" --expand ReplicationStatus --output json)"
    if ! jq -e '.provisioningState == "Succeeded" and .replicationStatus.aggregatedState == "Completed"' <<<"${version_info}" >/dev/null; then
        echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} publication is not complete" >&2
        exit 1
    fi
    exit 0
fi

if [[ ! "${SOURCE_VHD_URI}" =~ ^https://([a-z0-9]{3,24})\.blob\.[^/]+/.+ ]]; then
    echo "Source VHD must be an Azure Storage blob URL" >&2
    exit 1
fi
source_storage_account="${BASH_REMATCH[1]}"
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

definition_id="$(az sig image-definition list "${gallery_args[@]}" --query "[?name=='${SKU_NAME}'].id | [0]" --output tsv)"
if [[ -z "${definition_id}" || "${definition_id}" = "None" ]]; then
    definition_id="$(
        az sig image-definition create "${definition_args[@]}" \
            --publisher microsoft-aks --offer "${SIG_GALLERY_NAME}" --sku "${SKU_NAME}" \
            --os-type Linux --os-state Generalized --architecture x64 --hyper-v-generation V2 \
            --location "${PACKER_BUILD_LOCATION:?}" \
            --features "DiskControllerTypes=SCSI,NVMe" "SecurityType=ConfidentialVMSupported" \
            --query id --output tsv
    )"
    az resource update --ids "${definition_id}" --set properties.allowUpdateImage=true --only-show-errors >/dev/null
fi

target_regions=(eastus westeurope)
if [[ "${source_location}" != "eastus" && "${source_location}" != "westeurope" ]]; then
    target_regions+=("${source_location}")
fi

az sig image-version create "${version_args[@]}" \
    --location "${source_location}" \
    --os-vhd-uri "${SOURCE_VHD_URI}" \
    --os-vhd-storage-account "${source_storage_id}" \
    --target-regions "${target_regions[@]}" \
    --tags "buildId=${BUILD_BUILDID:?}" "createdBy=aks-vhd-pipeline" >/dev/null

az sig image-version wait "${version_args[@]}" --expand ReplicationStatus \
    --custom "provisioningState=='Succeeded' && replicationStatus.aggregatedState=='Completed'" \
    --interval 60

echo "Generalized CVM image ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} provisioned successfully"
