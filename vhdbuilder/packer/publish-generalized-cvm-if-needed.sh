#!/usr/bin/env bash

set -euo pipefail

STAGING_VHD_URI="${1:?staging VHD URI is required}"

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
    local provisioning_state

    echo "Waiting five minutes before checking image version provisioning"
    sleep 300

    while true; do
        if provisioning_state="$(
            az sig image-version show \
                --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
                --gallery-name "${SIG_GALLERY_NAME}" \
                --gallery-image-definition "${SKU_NAME}" \
                --gallery-image-version "${CAPTURED_SIG_VERSION}" \
                --query provisioningState \
                --output tsv \
                --only-show-errors 2>/dev/null
        )"; then
            echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} provisioning state: ${provisioning_state:-unknown}"

            case "${provisioning_state,,}" in
                succeeded)
                    return 0
                    ;;
                failed)
                    return 1
                    ;;
            esac
        else
            echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} does not exist yet"
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

cleanup_managed_image() {
    if az image show \
        --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
        --name "${SKU_NAME}-${CAPTURED_SIG_VERSION}-generalized-${BUILD_BUILDID}" \
        --only-show-errors >/dev/null 2>&1; then
        echo "Deleting temporary managed image"

        az image delete \
            --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
            --name "${SKU_NAME}-${CAPTURED_SIG_VERSION}-generalized-${BUILD_BUILDID}" \
            --only-show-errors
    fi
}

ensure_generalized_image_definition

if provisioning_state="$(
    az sig image-version show \
        --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
        --gallery-name "${SIG_GALLERY_NAME}" \
        --gallery-image-definition "${SKU_NAME}" \
        --gallery-image-version "${CAPTURED_SIG_VERSION}" \
        --query provisioningState \
        --output tsv \
        --only-show-errors 2>/dev/null
)"; then
    case "${provisioning_state,,}" in
        succeeded)
            echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} is already provisioned"
            exit 0
            ;;
        failed)
            delete_failed_image_version
            ;;
        *)
            echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} already exists and is provisioning"

            if wait_for_successful_provisioning; then
                echo "${SKU_NAME}/${CAPTURED_SIG_VERSION} provisioned successfully"
                exit 0
            fi

            delete_failed_image_version
            ;;
    esac
fi

managed_image_id="$(
    az image create \
        --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
        --name "${SKU_NAME}-${CAPTURED_SIG_VERSION}-generalized-${BUILD_BUILDID}" \
        --location "${PACKER_BUILD_LOCATION}" \
        --source "${STAGING_VHD_URI}" \
        --os-type Linux \
        --hyper-v-generation V2 \
        --storage-sku Standard_LRS \
        --tags \
            "buildId=${BUILD_BUILDID}" \
            "createdBy=aks-vhd-pipeline" \
        --query id \
        --output tsv \
        --only-show-errors
)"

trap cleanup_managed_image EXIT

az sig image-version create \
    --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
    --gallery-name "${SIG_GALLERY_NAME}" \
    --gallery-image-definition "${SKU_NAME}" \
    --gallery-image-version "${CAPTURED_SIG_VERSION}" \
    --managed-image "${managed_image_id}" \
    --target-regions eastus westeurope \
    --tags \
        "buildId=${BUILD_BUILDID}" \
        "createdBy=aks-vhd-pipeline" \
    --only-show-errors >/dev/null

if ! wait_for_successful_provisioning; then
    delete_failed_image_version
    echo "Generalized CVM image version failed to provision" >&2
    exit 1
fi

echo "Generalized CVM image ${SIG_GALLERY_NAME}/${SKU_NAME}/${CAPTURED_SIG_VERSION} provisioned successfully"
