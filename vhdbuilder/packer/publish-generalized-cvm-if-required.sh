#!/usr/bin/env bash

set -euo pipefail

SOURCE_VHD_URI="${1:?source VHD URI or --check-only is required}"

cvm_requires_publishing() {
    grep -q "cvm" <<<"${FEATURE_FLAGS,,}" || return 1

    case "${OS_SKU,,}" in
        ubuntu|azurelinux)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

if ! cvm_requires_publishing; then
    echo "Generalized CVM publication is not required for OS_SKU=${OS_SKU:-unset}, FEATURE_FLAGS=${FEATURE_FLAGS:-unset}"
    exit 0
fi

required_variables=(
    AZURE_RESOURCE_GROUP_NAME
    SIG_GALLERY_NAME
    SKU_NAME
    CAPTURED_SIG_VERSION
    PACKER_BUILD_LOCATION
    BUILD_BUILDID
)

for variable in "${required_variables[@]}"; do
    if [[ -z "${!variable:-}" ]]; then
        echo "Required variable ${variable} is not set" >&2
        exit 1
    fi
done

ensure_generalized_image_definition() {
    if az sig image-definition show \
        --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
        --gallery-name "${SIG_GALLERY_NAME}" \
        --gallery-image-definition "${SKU_NAME}" \
        --only-show-errors >/dev/null 2>&1; then
        return
    fi

    az sig image-definition create \
        --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
        --gallery-name "${SIG_GALLERY_NAME}" \
        --gallery-image-definition "${SKU_NAME}" \
        --publisher microsoft-aks \
        --offer "${SIG_GALLERY_NAME}" \
        --sku "${SKU_NAME}" \
        --os-type Linux \
        --os-state Generalized \
        --architecture x64 \
        --hyper-v-generation V2 \
        --location "${PACKER_BUILD_LOCATION}" \
        --features \
            "DiskControllerTypes=SCSI,NVMe" \
            "SecurityType=ConfidentialVMSupported" \
        --only-show-errors >/dev/null

    az resource update \
        --ids "$(
            az sig image-definition show \
                --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
                --gallery-name "${SIG_GALLERY_NAME}" \
                --gallery-image-definition "${SKU_NAME}" \
                --query id \
                --output tsv \
                --only-show-errors
        )" \
        --set properties.allowUpdateImage=true \
        --only-show-errors >/dev/null
}

wait_for_successful_provisioning() {
    local version_info provisioning_state replication_state

    while true; do
        if version_info="$(
            az sig image-version show \
                --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
                --gallery-name "${SIG_GALLERY_NAME}" \
                --gallery-image-definition "${SKU_NAME}" \
                --gallery-image-version "${CAPTURED_SIG_VERSION}" \
                --expand ReplicationStatus \
                --output json \
                --only-show-errors
        )"; then
            provisioning_state="$(jq -r '.provisioningState' <<<"${version_info}")"
            replication_state="$(jq -r '.replicationStatus.aggregatedState' <<<"${version_info}")"
            echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} provisioning: ${provisioning_state}, replication: ${replication_state}"

            case "${provisioning_state,,}/${replication_state,,}" in
                succeeded/completed)
                    return 0
                    ;;
                failed/*|*/failed)
                    return 1
                    ;;
            esac
        else
            echo "Unable to read image version status" >&2
            return 1
        fi

        sleep 60
    done
}

delete_failed_image_version() {
    echo "Deleting failed image version ${SKU_NAME}/${CAPTURED_SIG_VERSION}"

    az sig image-version delete \
        --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
        --gallery-name "${SIG_GALLERY_NAME}" \
        --gallery-image-definition "${SKU_NAME}" \
        --gallery-image-version "${CAPTURED_SIG_VERSION}" \
        --only-show-errors

    az sig image-version wait \
        --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
        --gallery-name "${SIG_GALLERY_NAME}" \
        --gallery-image-definition "${SKU_NAME}" \
        --gallery-image-version "${CAPTURED_SIG_VERSION}" \
        --deleted \
        --only-show-errors
}

if version_info="$(
    az sig image-version show \
        --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
        --gallery-name "${SIG_GALLERY_NAME}" \
        --gallery-image-definition "${SKU_NAME}" \
        --gallery-image-version "${CAPTURED_SIG_VERSION}" \
        --expand ReplicationStatus \
        --output json \
        --only-show-errors 2>&1
)"; then
    provisioning_state="$(jq -r '.provisioningState' <<<"${version_info}")"
    replication_state="$(jq -r '.replicationStatus.aggregatedState' <<<"${version_info}")"
    case "${provisioning_state,,}/${replication_state,,}" in
        succeeded/completed)
            echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} is already provisioned and replicated"
            exit 0
            ;;
    esac
else
    if [[ "${version_info}" != *"(ResourceNotFound)"* && "${version_info}" != *"(ParentResourceNotFound)"* ]]; then
        echo "Unable to read image version: ${version_info}" >&2
        exit 1
    fi
    provisioning_state="missing"
    replication_state="missing"
fi

if [[ "${SOURCE_VHD_URI}" = "--check-only" ]]; then
    echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} publication is not complete"
    exit 1
fi

ensure_generalized_image_definition

case "${provisioning_state,,}/${replication_state,,}" in
    missing/*)
        ;;
    failed/*|*/failed)
        delete_failed_image_version
        ;;
    *)
        echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} already exists and is provisioning"

        if wait_for_successful_provisioning; then
            echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} provisioned successfully"
            exit 0
        fi

        echo "Generalized CVM image version did not complete publication" >&2
        exit 1
        ;;
esac

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

target_regions=(eastus westeurope)
if [[ "${source_location}" != "eastus" && "${source_location}" != "westeurope" ]]; then
    target_regions+=("${source_location}")
fi

az sig image-version create \
    --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
    --gallery-name "${SIG_GALLERY_NAME}" \
    --gallery-image-definition "${SKU_NAME}" \
    --gallery-image-version "${CAPTURED_SIG_VERSION}" \
    --location "${source_location}" \
    --os-vhd-uri "${SOURCE_VHD_URI}" \
    --os-vhd-storage-account "${source_storage_id}" \
    --target-regions "${target_regions[@]}" \
    --tags \
        "buildId=${BUILD_BUILDID}" \
        "createdBy=aks-vhd-pipeline" \
    --only-show-errors >/dev/null

if ! wait_for_successful_provisioning; then
    echo "Generalized CVM image version did not complete publication" >&2
    exit 1
fi

echo "Generalized CVM image ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} provisioned successfully"
