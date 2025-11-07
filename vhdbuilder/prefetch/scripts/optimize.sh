#!/bin/bash
set -uxo pipefail

# TODO(cameissner): migrate to vhdbuilder go binary once VHD build scripts are hosted internally

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID is not set" && exit 1
[ -z "${LOCATION:-}" ] && echo "LOCATION is not set" && exit 1
[ -z "${SIG_GALLERY_RESOURCE_GROUP_NAME:-}" ] && echo "SIG_GALLERY_RESOURCE_GROUP_NAME is not set" && exit 1
[ -z "${SIG_GALLERY_NAME:-}" ] && echo "SIG_GALLERY_NAME is not set" && exit 1
[ -z "${SKU_NAME:-}" ] && echo "SKU_NAME is not set" && exit 1
[ -z "${STORAGE_ACCOUNT_BLOB_URL:-}" ] && echo "STORAGE_ACCOUNT_BLOB_URL is not set" && exit 1
[ -z "${VHD_STORAGE_ACCOUNT_NAME:-}" ] && echo "VHD_STORAGE_ACCOUNT_NAME is not set" && exit 1
[ -z "${VHD_STORAGE_CONTAINER_NAME:-}" ] && echo "VHD_STORAGE_CONTAINER_NAME is not set" && exit 1
[ -z "${IMAGE_BUILDER_IDENTITY_ID:-}" ] && echo "IMAGE_BUILDER_IDENTITY_ID is not set" && exit 1
[ -z "${BUILD_RUN_NUMBER:-}" ] && echo "BUILD_RUN_NUMBER is not set" && exit 1
[ -z "${CAPTURED_SIG_VERSION:-}" ] && echo "CAPTURED_SIG_VERSION is not set" && exit 1

API_VERSION="2024-02-01"
CAPTURED_SIG_VERSION_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${SIG_GALLERY_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${SKU_NAME}/versions/${CAPTURED_SIG_VERSION}"
IMAGE_BUILDER_RG_NAME="image-builder-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
IMAGE_BUILDER_TEMPLATE_NAME="template-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
OPTIMIZED_VHD_BLOB_NAME="${CAPTURED_SIG_VERSION}.vhd"
OPTIMIZED_VHD_URI="${STORAGE_ACCOUNT_BLOB_URL}/${OPTIMIZED_VHD_BLOB_NAME}"

main() {
    ensure_image_builder_resource_group || exit $?
    run_image_builder_template || exit $?
    # copy_optimized_vhd || exit $?
}

run_image_builder_template() {
    if need_new_template; then
        sed -e "s#<LOCATION>#${LOCATION}#g" \
            -e "s#<IMAGE_BUILDER_IDENTITY_ID>#${IMAGE_BUILDER_IDENTITY_ID}#g" \
            -e "s#<CAPTURED_SIG_VERSION_ID>#${CAPTURED_SIG_VERSION_ID}#g" \
            -e "s#<OPTIMIZED_VHD_URI>#${OPTIMIZED_VHD_URI}#g" \
            ../templates/optimize.json > input.json

        echo "creating image builder template ${IMAGE_BUILDER_TEMPLATE_NAME} in resource group ${IMAGE_BUILDER_RG_NAME}"
        az resource create -n" ${IMAGE_BUILDER_TEMPLATE_NAME}" \
            --properties @input.json \
            --is-full-object \
            --api-version "${API_VERSION}" \
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
}

copy_optimized_vhd() {
    staging_rg_name=$(az resource show -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}" \
        --resource-type Microsoft.VirtualMachineImages/imageTemplates \
        --api-version "${API_VERSION}" | jq -r '.properties.exactStagingResourceGroup')
    staging_rg_name=${staging_rg_name##*/}

    copy_info=$(az storage blob show \
        --name "${OPTIMIZED_VHD_BLOB_NAME}" \
        --container-name "${VHD_STORAGE_CONTAINER_NAME}" \
        --account-name "${VHD_STORAGE_ACCOUNT_NAME}" \
        --subscription "${SUBSCRIPTION_ID}" 2>/dev/null | jq '.properties.copy')
    copy_source=$(jq -r '.source' <<< "${copy_info}")
    if [ "${copy_source}" != "null" ]; then
        # this blob has previously been copied to from somewhere else
        set_storage_details_from_vhd_blob_url "${copy_source}" || return $?
        source_storage_account_name=${STORAGE_ACCOUNT_NAME}
        # attempt to show the storage account under the assumption it's within the template's staging resource group
        source_storage_account_info=$(az storage account show -g "${staging_rg_name}" -n "${source_storage_account_name}" --subscription "${SUBSCRIPTION_ID}")
        if [ -n "${source_storage_account_info}" ]; then
            # double-check the tags on the storage account to guarantee it contains the optimized blob
            source_storage_account_created_by=$(jq -r '.tags.createdby' <<< "${source_storage_account_info}")
            if [ "${source_storage_account_created_by,,}" = "azurevmimagebuilder" ]; then
                copy_status=$(jq -r '.status' <<< "${copy_info}")
                if [ "${copy_status,,}" = "success" ] || [ "${copy_status,,}" = "pending" ]; then
                    # if the copy is already done or is currently in-progress, exit early
                    echo "blob ${CAPTURED_SIG_VERSION}.vhd has been copied or is in an active copy operation from ${copy_source} (status = ${copy_status})"
                    return 0
                fi
            fi
        fi
    fi

    set_storage_details_from_vhd_blob_url "$(az image builder show-runs -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}" | jq -r '.[-1].artifactUri')" || return $?
    set_optimized_vhd_sas_url "${staging_rg_name}" "${STORAGE_ACCOUNT_NAME}" "${STORAGE_CONTAINER_NAME}" "${VHD_BLOB_NAME}" || return $?

    echo "beginning copy of ${CAPTURED_SIG_VERSION}.vhd to ${VHD_STORAGE_ACCOUNT_NAME}/${VHD_STORAGE_CONTAINER_NAME}/${CAPTURED_SIG_VERSION}.vhd"
    az storage blob copy start \
        --destination-blob "${OPTIMIZED_VHD_BLOB_NAME}" \
        --destination-container" ${VHD_STORAGE_CONTAINER_NAME}" \
        --account-name "${VHD_STORAGE_ACCOUNT_NAME}" \
        --subscription "${SUBSCRIPTION_ID}" \
        --source-uri "${OPTIMIZED_VHD_SAS_URL}" || return $?

    while [ "$(az storage blob show \
      --name "${OPTIMIZED_VHD_BLOB_NAME}" \
      --container-name "${VHD_STORAGE_CONTAINER_NAME}" \
      --account-name "${VHD_STORAGE_ACCOUNT_NAME}" \
      --subscription "${SUBSCRIPTION_ID}" 2>/dev/null | jq -r .properties.copy.status)" != "success" ]; do
      echo "waiting for copy to storage account: ${VHD_STORAGE_ACCOUNT_NAME}, container: ${VHD_STORAGE_CONTAINER_NAME}, blob: ${CAPTURED_SIG_VERSION}.vhd"
      sleep 60s
    done
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

set_optimized_vhd_sas_url() {
    local rg_name=$1
    local storage_account_name=$2
    local storage_container_name=$3
    local blob_name=$4

    set +x
    connection_string=$(az storage account show-connection-string --resource-group "${rg_name}" --name "${storage_account_name}" | jq -r '.connectionString')
    [ -z "${connection_string}" ] && echo "an error occured when generating connection string for storage account: ${rg_name}/${storage_account_name}" && return 1
    # set the SAS to expire after 180 minutes
    expiry=$(date -u -d "180 minutes" '+%Y-%m-%dT%H:%MZ')
    sas_token=$(az storage container generate-sas --connection-string "${connection_string}" --name "${storage_container_name}" --permissions lr --expiry "${expiry}" | tr -d '"')
    [ -z "${sas_token}" ] && echo "an error occured when generating SAS token for ${rg_name}/${storage_account_name}/${storage_container_name}/${blob_name}" && return 1
    OPTIMIZED_VHD_SAS_URL="https://${storage_account_name}.blob.core.windows.net/${storage_container_name}/${blob_name}?${sas_token}"
    set -x

    echo "generated SAS url for blob: ${rg_name}/${storage_account_name}/${storage_container_name}/${blob_name}"
}

set_storage_details_from_vhd_blob_url() {
    local blob_url=$1

    echo "attempting to extract storage account and container name from blob url: ${blob_url}"
    # shellcheck disable=SC3010
    if [[ ! "${blob_url%%\?*}" =~ https:\/\/(.*)?.blob.core.windows.net(:443)?\/(.*)?\/(.*)? ]]; then
      echo "unable to extract unique vhd version from blob url: ${blob_url}"
      return 1
    fi

    STORAGE_ACCOUNT_NAME="${BASH_REMATCH[1]}"
    STORAGE_CONTAINER_NAME="${BASH_REMATCH[3]}"
    VHD_BLOB_NAME="${BASH_REMATCH[4]}"
}

ensure_image_builder_resource_group() {
  echo "ensuring resource group: $IMAGE_BUILDER_RG_NAME"

  if [ -z "$(az group show --name "${IMAGE_BUILDER_RG_NAME}" | jq '.id' )" ]; then
    echo "creating resource group ${IMAGE_BUILDER_RG_NAME}"
    az group create --name "${IMAGE_BUILDER_RG_NAME}" --location "${LOCATION}" || return $?
  fi
}

main "$@"
