#!/bin/bash
set -uxo pipefail

# TODO(cameissner): migrate to vhdbuilder go binary once VHD build scripts are hosted internally
# TODO(cameissner/hebeberm): add support for images built by imagecustomizer (OSGuard) - it seems the cleanup script can't run due to a permissions issue

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID is not set" && exit 1
[ -z "${LOCATION:-}" ] && echo "LOCATION is not set" && exit 1
[ -z "${SIG_GALLERY_RESOURCE_GROUP_NAME:-}" ] && echo "SIG_GALLERY_RESOURCE_GROUP_NAME is not set" && exit 1
[ -z "${SIG_GALLERY_NAME:-}" ] && echo "SIG_GALLERY_NAME is not set" && exit 1
[ -z "${SIG_IMAGE_NAME:-}" ] && echo "SIG_IMAGE_NAME is not set" && exit 1
[ -z "${SKU_NAME:-}" ] && echo "SKU_NAME is not set" && exit 1
[ -z "${STORAGE_ACCOUNT_BLOB_URL:-}" ] && echo "STORAGE_ACCOUNT_BLOB_URL is not set" && exit 1
[ -z "${VHD_NAME:-}" ] && echo "VHD_NAME is not set" && exit 1
[ -z "${IMAGE_BUILDER_IDENTITY_ID:-}" ] && echo "IMAGE_BUILDER_IDENTITY_ID is not set" && exit 1
[ -z "${BUILD_RUN_NUMBER:-}" ] && echo "BUILD_RUN_NUMBER is not set" && exit 1
[ -z "${CAPTURED_SIG_VERSION:-}" ] && echo "CAPTURED_SIG_VERSION is not set" && exit 1
[ -z "${HYPERV_GENERATION:-}" ] && echo "HYPERV_GENERATION is not set" && exit 1
[ -z "${FEATURE_FLAGS:-}" ] && echo "FEATURE_FLAGS is not set" && exit 1
[ -z "${ENABLE_TRUSTED_LAUNCH:-}" ] && echo "ENABLE_TRUSTED_LAUNCH is not set" && exit 1

IMAGE_BUILDER_API_VERSION="2024-02-01"
MANAGED_DISK_API_VERSION="2024-03-02"

IMAGE_BUILDER_TEMPLATE_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)/../templates/optimize.json"
CAPTURED_SIG_VERSION_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${SIG_GALLERY_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${SIG_IMAGE_NAME}/versions/${CAPTURED_SIG_VERSION}"
IMAGE_BUILDER_RG_NAME="image-builder-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
IMAGE_BUILDER_TEMPLATE_NAME="template-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
VHD_URI="${STORAGE_ACCOUNT_BLOB_URL}/${VHD_NAME}"

main() {
    # for idempotency, check to see if the VHD we're trying to create already exists.
    # if it's already in the expected state, this will cause the script to exit early.
    # otherwise, we delete any existing VHD in an unexpected state and retry the whole
    # optimization + conversion flow
    check_for_existing_vhd

    # attempt to perform prefetch optimization and VHD conversion
    ensure_image_builder_rg || exit $?
    run_image_builder_template || exit $?
}

check_for_existing_vhd() {
    vhd_info="$(az storage blob show --blob-url "${VHD_URI}" --auth-mode login)"
    if [ -n "${vhd_info}" ]; then
        echo "VHD already exists at: ${VHD_URI}"
        image_builder_source="$(jq -r '.metadata.VMImageBuilderSource' <<< "${vhd_info}")"
        if [ -n "${image_builder_source}" ] && [ "${image_builder_source}" != "null" ]; then
            echo "VHD ${VHD_URI} has already been produced by a previous image builder template run"
            copy_status="$(jq -r '.properties.copy.status' <<< "${vhd_info}")"
            if [ "${copy_status,,}" = "success" ]; then
                echo "VHD ${VHD_URI} has been successfully copied from image builder storage, nothing to do"
                exit 0
            fi
            if [ "${copy_status,,}" = "pending" ]; then
                echo "echo VHD ${VHD_URI} is currently being copied from image builder storage, will wait for copy completion"
                wait_for_vhd_copy || exit $?
                exit 0
            fi
            echo "VHD ${VHD_URI} has a bad copy state: ${copy_status}, will delete it and recreate"
            delete_vhd || exit $?
        else
            echo "VHD ${VHD_URI} exists but was not produced by an image builder template run, will delete before proceeding"
            delete_vhd || exit $?
        fi
    else
        echo "no existing VHD was found at: ${VHD_URI}, will proceed with optimization and VHD creation"
    fi
}

ensure_image_builder_rg() {
    if [ "$(az group exists -g "${IMAGE_BUILDER_RG_NAME}")" = "true" ]; then
        echo "image builder resource group ${IMAGE_BUILDER_RG_NAME} already exists"
        return 0
    fi
    echo "creating resource group ${IMAGE_BUILDER_RG_NAME}"
    az group create -g "${IMAGE_BUILDER_RG_NAME}" -l "${LOCATION}" --tags "createdBy=aks-vhd-pipeline" "buildNumber=${BUILD_RUN_NUMBER}" "now=$(date +%s)" "image_sku=${SKU_NAME}" || return $?
}

run_image_builder_template() {
    if need_new_template; then
        prepare_source || return $?
        sed -e "s#<LOCATION>#${LOCATION}#g" \
            -e "s#<IMAGE_BUILDER_IDENTITY_ID>#${IMAGE_BUILDER_IDENTITY_ID}#g" \
            -e "s#<SOURCE_TYPE>#${SOURCE_TYPE}#g" \
            -e "s#<SOURCE_ID_KEY>#${SOURCE_ID_KEY}#g" \
            -e "s#<SOURCE_ID>#${SOURCE_ID}#g" \
            -e "s#<VHD_URI>#${VHD_URI}#g" \
            "${IMAGE_BUILDER_TEMPLATE_PATH}" > input.json || return $?

        if [ ! -f "input.json" ]; then
            echo "unable to create input image template for ${IMAGE_BUILDER_TEMPLATE_NAME}"
            return 1
        fi

        echo "creating image builder template ${IMAGE_BUILDER_TEMPLATE_NAME} in resource group ${IMAGE_BUILDER_RG_NAME}"
        az resource create -n "${IMAGE_BUILDER_TEMPLATE_NAME}" \
            --properties @input.json \
            --is-full-object \
            --api-version "${IMAGE_BUILDER_API_VERSION}" \
            --resource-type Microsoft.VirtualMachineImages/imageTemplates \
            --resource-group "${IMAGE_BUILDER_RG_NAME}" || return $?

        echo "image builder template ${IMAGE_BUILDER_TEMPLATE_NAME} has been created, starting run..."
        az image builder run -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}"
    else
        echo "will attempt to wait for image builder template ${IMAGE_BUILDER_TEMPLATE_NAME} to finish its last run..."
        az image builder wait -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}" --custom "lastRunStatus.runState!='Running'"
    fi

    template_run_state=$(az image builder show -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}" | jq -r '.lastRunStatus.runState')
    if [ "${template_run_state,,}" != "succeeded" ]; then
        echo "${IMAGE_BUILDER_TEMPLATE_NAME} failed to run successfully, finished with state: '${template_run_state}'"
        return 1
    fi

    echo "template ${IMAGE_BUILDER_TEMPLATE_NAME} has ran to completion, VHD has been published to: ${VHD_URI}"
}

need_new_template() {
    template_info=$(az image builder show -g "${IMAGE_BUILDER_RG_NAME}" -n "${IMAGE_BUILDER_TEMPLATE_NAME}")
    if [ -z "$(jq -r '.id' <<< "${template_info}")" ]; then
        return 0
    fi
    template_provisioning_state=$(jq -r '.provisioningState' <<< "${template_info}")
    if [ "${template_provisioning_state,,}" = "failed" ]; then
        echo "provisioning state of template ${IMAGE_BUILDER_TEMPLATE_NAME} is: '${template_provisioning_state}', will delete and re-create template"
        az image builder delete -g "${IMAGE_BUILDER_RG_NAME}" -n "${IMAGE_BUILDER_TEMPLATE_NAME}"
        return 0
    fi
    last_run_state=$(jq -r '.lastRunStatus.runState' <<< "${template_info}")
    if [ "${last_run_state,,}" = "failed" ]; then
        echo "last run state of template ${IMAGE_BUILDER_TEMPLATE_NAME} is: '${last_run_state}', will delete and re-create template"
        az image builder delete -g "${IMAGE_BUILDER_RG_NAME}" -n "${IMAGE_BUILDER_TEMPLATE_NAME}"
        return 0
    fi
    return 1
}

prepare_source() {
    if [ "${ENABLE_TRUSTED_LAUNCH,,}" = "true" ] || grep -q "cvm" <<< "$FEATURE_FLAGS"; then
        echo "image ${SKU_NAME} is a TL/CVM flavor, will create managed image source"
        convert_specialized_sig_version_to_managed_image || return $?
        SOURCE_TYPE="ManagedImage"
        SOURCE_ID_KEY="imageId"
        SOURCE_ID="${SOURCE_MANAGED_IMAGE_ID}"
        return 0
    fi
    echo "image ${SKU_NAME} is NOT a TL/CVM flavor, will source from existing gallery image version: ${CAPTURED_SIG_VERSION_ID}"
    SOURCE_TYPE="SharedImageVersion"
    SOURCE_ID_KEY="imageVersionId"
    SOURCE_ID="${CAPTURED_SIG_VERSION_ID}"
}

# This function is needed to convert SIG image versions within a specialized image defintion
# To a managed image which can be used as a source image for the image builder template.
# This is needed since image builder templates do not support SIG image version sources that
# have a "Specialized" OS state. As of writing, this only applies to TrustedLaunch and CVM SKUs,
# sonce those SKUs must be built on special hardware, and thus must be captured within a Specialized
# SIG image definition after being built with Packer.
# This function performs the following steps to create a suitable source image based on an image version
# coming from a Specialized image definition:
# 1. Create a temporary storage account in the build location which will house the intermediate VHD blob
# 2. Create a managed disk in the build location from the image version produced by Packer, with the corresponding security type (CVM/TL)
# 3. Copy the managed disk to a temporary VHD blob in the temporary storage account
# 4. Create a new managed image in the build location from the temporary VHD blob, which will be used as the source image of the image builder template
convert_specialized_sig_version_to_managed_image() {
    managed_image_name="${CAPTURED_SIG_VERSION}-template-source"
    managed_image_id="$(az image show -g "${IMAGE_BUILDER_RG_NAME}" -n "${managed_image_name}" | jq -r '.id')"
    if [ -n "${managed_image_id}" ]; then
        echo "managed image source already exists: ${managed_image_id}"
        SOURCE_MANAGED_IMAGE_ID="${managed_image_id}"
        return 0
    fi

    create_temp_storage || return $?

    if [ "$(az storage blob exists --blob-url "${TEMP_VHD_URI}" --auth-mode login | jq -r '.exists')" = "true" ]; then
        echo "creating managed image from already-existing VHD: ${TEMP_VHD_URI}"
        managed_image_id=$(az image create -g "${IMAGE_BUILDER_RG_NAME}" -n "${managed_image_name}" \
            --os-type Linux \
            --hyper-v-generation "${HYPERV_GENERATION}" \
            --source "${TEMP_VHD_URI}" | jq -r '.id')
        if [ -z "${managed_image_id}" ]; then
            echo "unable to create managed image using source: ${TEMP_VHD_URI}"
            return 1
        fi
        echo "created managed image source: ${managed_image_id}"
        SOURCE_MANAGED_IMAGE_ID="${managed_image_id}"
        return 0
    fi

    disk_resource_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${IMAGE_BUILDER_RG_NAME}/providers/Microsoft.Compute/disks/${CAPTURED_SIG_VERSION}"
    if [ -z "$(az disk show --ids "${disk_resource_id}" | jq -r '.id')" ]; then
        security_type="ConfidentialVM_VMGuestStateOnlyEncryptedWithPlatformKey"
        if [ "${ENABLE_TRUSTED_LAUNCH,,}" = "true" ]; then
            security_type="TrustedLaunch"
        fi
        disk_resource_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${IMAGE_BUILDER_RG_NAME}/providers/Microsoft.Compute/disks/${CAPTURED_SIG_VERSION}"
        echo "converting $CAPTURED_SIG_VERSION_ID to ${disk_resource_id}"
        echo "will use security type: ${security_type}"

        az resource create --id "${disk_resource_id}" --api-version "${MANAGED_DISK_API_VERSION}" --is-full-object --location "$LOCATION" --properties "{\"location\": \"$LOCATION\", \
            \"properties\": { \
                \"osType\": \"Linux\", \
                \"securityProfile\": { \
                    \"securityType\": \"${security_type}\" \
                }, \
                \"creationData\": { \
                    \"createOption\": \"FromImage\", \
                    \"galleryImageReference\": { \
                        \"id\": \"${CAPTURED_SIG_VERSION_ID}\" \
                    } \
                } \
            } \
        }" || return $?

        echo "converted $CAPTURED_SIG_VERSION_ID to ${disk_resource_id}"
    fi

    set +x
    disk_sas_url=$(az disk grant-access --ids "${disk_resource_id}" --duration-in-seconds 1800 | jq -r '.accessSAS')
    if [ -z "${disk_sas_url}" ] || [ "${disk_sas_url,,}" = "null" ] || [ "${disk_sas_url,,}" = "none" ]; then
        echo "generated SAS URL for managed disk is empty, cannot continue"
        return 1
    fi
    echo "setting azcopy environment variables with pool identity: ${IMAGE_BUILDER_IDENTITY_ID}"
    export AZCOPY_AUTO_LOGIN_TYPE="MSI"
    export AZCOPY_MSI_RESOURCE_STRING="${IMAGE_BUILDER_IDENTITY_ID}"
    export AZCOPY_CONCURRENCY_VALUE="AUTO"
    echo "uploading $disk_resource_id to ${TEMP_VHD_URI}"
    azcopy copy "${disk_sas_url}" "${TEMP_VHD_URI}" --recursive=true || return $?
    set -x

    echo "creating managed image from ${TEMP_VHD_URI}"
    managed_image_id=$(az image create -g "${IMAGE_BUILDER_RG_NAME}" -n "${managed_image_name}" \
        --os-type Linux \
        --hyper-v-generation "${HYPERV_GENERATION}" \
        --source "${TEMP_VHD_URI}" | jq -r '.id')
    if [ -z "${managed_image_id}" ]; then
        echo "unable to create managed image using source: ${TEMP_VHD_URI}"
        return 1
    fi

    # specialized SIG image version -> specialized managed disk -> VHD blob in build location -> managed image
    echo "created managed image source: ${managed_image_id}"
    SOURCE_MANAGED_IMAGE_ID="${managed_image_id}"
}

create_temp_storage() {
    storage_account_name="tmp${VHD_NAME//./}"
    if [ -z "$(az storage account show --account-name "${storage_account_name}" | jq -r '.name')" ]; then
        echo "creating temporary storage account ${storage_account_name} in resource group ${IMAGE_BUILDER_RG_NAME} in location ${LOCATION}"
        az storage account create -n "${storage_account_name}" -g "${IMAGE_BUILDER_RG_NAME}" --sku "Standard_RAGRS" --allow-shared-key-access false --location "${LOCATION}" || return $?
    fi
    storage_container_name="vhd"
    if [ "$(az storage container exists -n "${storage_container_name}" --account-name "${storage_account_name}" --auth-mode login | jq -r '.exists')" = "false" ]; then
        echo "creating container \"${storage_container_name}\" within temporary storage account ${storage_account_name}"
        az storage container create --name "${storage_container_name}" --account-name "${storage_account_name}" --auth-mode login || return $?
    fi

    temp_vhd_uri="https://${storage_account_name}.blob.core.windows.net/${storage_container_name}/${VHD_NAME}"
    echo "temp VHD URI is ${temp_vhd_uri}"
    TEMP_VHD_URI="${temp_vhd_uri}"
}

wait_for_vhd_copy() {
    while [ "$(az storage blob show --blob-url "${VHD_URI}" --auth-mode login | jq -r '.properties.copy.status')" = "pending" ]; do
        echo "VHD ${VHD_URI} is still undergoing a pending copy operation"
        sleep 30s
    done
    echo "pending copy operation over ${VHD_URI} has completed"
}

delete_vhd() {
    az storage blob delete --blob-url "${VHD_URI}" --auth-mode login || return $?
    while [ -n "$(az storage blob show --blob-url "${VHD_URI}" --auth-mode login | jq -r '.name')" ]; do
        echo "VHD ${VHD_URI} has yet to be deleted, will wait 30s before checking again"
        sleep 30s
    done
    echo "${VHD_URI} has been deleted"
}

main "$@"
