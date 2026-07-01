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
[ -z "${STORAGE_ACCOUNT_BLOB_URL_STAGING:-}" ] && echo "STORAGE_ACCOUNT_BLOB_URL_STAGING is not set" && exit 1
[ -z "${VHD_NAME:-}" ] && echo "VHD_NAME is not set" && exit 1
[ -z "${IMAGE_BUILDER_IDENTITY_ID:-}" ] && echo "IMAGE_BUILDER_IDENTITY_ID is not set" && exit 1
[ -z "${BUILD_RUN_NUMBER:-}" ] && echo "BUILD_RUN_NUMBER is not set" && exit 1
[ -z "${CAPTURED_SIG_VERSION:-}" ] && echo "CAPTURED_SIG_VERSION is not set" && exit 1
[ -z "${HYPERV_GENERATION:-}" ] && echo "HYPERV_GENERATION is not set" && exit 1
[ -z "${FEATURE_FLAGS:-}" ] && echo "FEATURE_FLAGS is not set" && exit 1
[ -z "${ENABLE_TRUSTED_LAUNCH:-}" ] && echo "ENABLE_TRUSTED_LAUNCH is not set" && exit 1
[ -z "${WORKLOAD_OPTIMIZATION_SCRIPT_URL:-}" ] && echo "WORKLOAD_OPTIMIZATION_SCRIPT_URL is not set" && exit 1

IMAGE_BUILDER_API_VERSION="2025-10-01"
MANAGED_DISK_API_VERSION="2024-03-02"

# image builder rejects new template creations and runs when too many tasks from a single subscription
# are running or waiting (TooManyRequests). This limit clears slowly, so we back off heavily and retry a
# large number of times before giving up.
RATE_LIMIT_MAX_ATTEMPTS=15
RATE_LIMIT_RETRY_DELAY_SECONDS=180

IMAGE_BUILDER_TEMPLATE_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)/../templates/optimize.json"
CAPTURED_SIG_VERSION_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${SIG_GALLERY_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${SIG_IMAGE_NAME}/versions/${CAPTURED_SIG_VERSION}"
IMAGE_BUILDER_RG_NAME="image-builder-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
IMAGE_BUILDER_TEMPLATE_NAME="template-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
IMAGE_BUILDER_TEMPLATE_NAME="${IMAGE_BUILDER_TEMPLATE_NAME:0:64}"
IMAGE_BUILDER_TEMPLATE_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${IMAGE_BUILDER_RG_NAME}/providers/Microsoft.VirtualMachineImages/imageTemplates/${IMAGE_BUILDER_TEMPLATE_NAME}"
VHD_URI="${STORAGE_ACCOUNT_BLOB_URL}/${VHD_NAME}"

# the image builder template distributes the prefetch-optimized image as a VHD without specifying a
# target uri, which causes image builder to publish the VHD to a storage account within its own
# staging resource group. we then copy that staging VHD into the target storage account ourselves,
# which avoids the slow second copy image builder performs when distributing directly to an external
# target storage account. the name of the run output is used to look up the resulting blob
# (artifactUri) after the template run completes.
DISTRIBUTE_RUN_OUTPUT_NAME="VHD"

# the destination vhd container may have an immutability policy, which prevents azcopy from writing the VHD
# into it directly (azcopy creates the page blob and then writes to it, which the policy rejects as a
# modification of an existing blob). To work around this, we azcopy the VHD into the staging container
# (STORAGE_ACCOUNT_BLOB_URL_STAGING, which has no immutability policy), then perform a single server-side
# blob copy from the staging container into the immutable vhd container. The server-side copy creates the
# destination blob in one atomic operation, which the immutability policy allows.
DESTINATION_STORAGE_ACCOUNT_NAME="${STORAGE_ACCOUNT_BLOB_URL#*://}"
DESTINATION_STORAGE_ACCOUNT_NAME="${DESTINATION_STORAGE_ACCOUNT_NAME%%.*}"
DESTINATION_CONTAINER_NAME="${STORAGE_ACCOUNT_BLOB_URL##*/}"
STAGING_VHD_URI="${STORAGE_ACCOUNT_BLOB_URL_STAGING}/${VHD_NAME}"

main() {
    # for idempotency, check whether the target VHD already exists. If a previous run already produced it
    # successfully, exit early; if a server-side copy is still in progress, wait for it to finish. Because
    # the destination container may be immutable, a VHD left in a bad state cannot be deleted and recreated
    # and is treated as fatal.
    check_for_existing_vhd

    # attempt to perform prefetch optimization, distributing the optimized image as a VHD into the
    # image builder staging storage account, then copy that VHD into the target storage account
    ensure_image_builder_rg || exit $?
    run_image_builder_template || exit $?
    copy_optimized_vhd || exit $?
}

check_for_existing_vhd() {
    if [ "$(az storage blob exists --blob-url "${VHD_URI}" --auth-mode login | jq -r '.exists')" = "false" ]; then
        echo "no existing VHD was found at: ${VHD_URI}, will proceed with optimization and VHD creation"
        return 0
    fi
    echo "VHD already exists at: ${VHD_URI}"

    # the VHD in a potentially immutable container is produced by a server-side blob copy, so its copy status
    # tells us whether a previous run already completed. Because the container may have an immutability policy,
    # a VHD in a bad state cannot be deleted and recreated, so any non-success terminal state is fatal.
    copy_status="$(az storage blob show --blob-url "${VHD_URI}" --auth-mode login | jq -r '.properties.copy.status')"
    if [ "${copy_status,,}" = "success" ]; then
        echo "VHD ${VHD_URI} was already produced by a previous prefetch optimization run, nothing to do"
        exit 0
    fi
    if [ "${copy_status,,}" = "pending" ]; then
        echo "VHD ${VHD_URI} is currently being copied by a previous prefetch optimization run, will wait for completion"
        if wait_for_vhd_copy; then
            exit 0
        fi
    fi
    # the VHD exists in a non-success state within the immutable container. It cannot be deleted due to
    # the immutability policy, so there is nothing we can do to recover - fail the script.
    echo "VHD ${VHD_URI} exists in an unrecoverable copy state: '${copy_status}'"
    exit 1
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
            -e "s#<WORKLOAD_OPTIMIZATION_SCRIPT_URL>#${WORKLOAD_OPTIMIZATION_SCRIPT_URL}#g" \
            -e "s#<SOURCE_TYPE>#${SOURCE_TYPE}#g" \
            -e "s#<SOURCE_ID_KEY>#${SOURCE_ID_KEY}#g" \
            -e "s#<SOURCE_ID>#${SOURCE_ID}#g" \
            "${IMAGE_BUILDER_TEMPLATE_PATH}" > input.json || return $?

        if [ ! -f "input.json" ]; then
            echo "unable to create input image template for ${IMAGE_BUILDER_TEMPLATE_NAME}"
            return 1
        fi

        echo "creating image builder template ${IMAGE_BUILDER_TEMPLATE_NAME} in resource group ${IMAGE_BUILDER_RG_NAME}"
        retry_on_rate_limit az resource create -n "${IMAGE_BUILDER_TEMPLATE_NAME}" \
            --properties @input.json \
            --is-full-object \
            --api-version "${IMAGE_BUILDER_API_VERSION}" \
            --resource-type Microsoft.VirtualMachineImages/imageTemplates \
            --resource-group "${IMAGE_BUILDER_RG_NAME}" || return $?

        echo "image builder template ${IMAGE_BUILDER_TEMPLATE_NAME} has been created, starting run..."
        retry_on_rate_limit az image builder run -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}"
    else
        if [ "$(az image builder show -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}" | jq -r '.lastRunStatus')" = "null" ]; then
            echo "template ${IMAGE_BUILDER_TEMPLATE_NAME} has no lastRunStatus, will attempt to run..."
            retry_on_rate_limit az image builder run -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}"
        else
            echo "will attempt to wait for image builder template ${IMAGE_BUILDER_TEMPLATE_NAME} to finish its last run..."
            az image builder wait -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}" --custom "lastRunStatus.runState!='Running'"
        fi
    fi

    template_run_state=$(az image builder show -n "${IMAGE_BUILDER_TEMPLATE_NAME}" -g "${IMAGE_BUILDER_RG_NAME}" | jq -r '.lastRunStatus.runState')
    if [ "${template_run_state,,}" != "succeeded" ]; then
        echo "${IMAGE_BUILDER_TEMPLATE_NAME} failed to run successfully, finished with state: '${template_run_state}'"
        return 1
    fi

    echo "template ${IMAGE_BUILDER_TEMPLATE_NAME} has ran to completion, optimized VHD has been published to image builder staging storage"
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
# since those SKUs must be built on special hardware, and thus must be captured within a Specialized
# SIG image definition after being built with Packer.
# This function performs the following steps to create a suitable source image based on an image version
# coming from a Specialized image definition:
# 1. Create a temporary storage account in the build location which will house the intermediate VHD blob
# 2. Create a managed disk in the build location from the image version produced by Packer, with the corresponding security type (CVM/TL)
# 3. Copy the managed disk to a temporary VHD blob in the temporary storage account
# 4. Create a new managed image in the build location from the temporary VHD blob, which will be used as the source image of the image builder template
convert_specialized_sig_version_to_managed_image() {
    managed_image_name="${CAPTURED_SIG_VERSION}-template-source"
    managed_image_id="$(az image show -g "${IMAGE_BUILDER_RG_NAME}" -n "${managed_image_name}" | jq -r '.id // empty')"
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
    export AZCOPY_AUTO_LOGIN_TYPE="AZCLI"
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
    storage_account_name="${VHD_NAME//./}"
    if ! az storage account show --account-name "${storage_account_name}" >/dev/null 2>&1; then
        echo "creating temporary storage account ${storage_account_name} in resource group ${IMAGE_BUILDER_RG_NAME} in location ${LOCATION}"
        az storage account create -n "${storage_account_name}" -g "${IMAGE_BUILDER_RG_NAME}" --sku "Standard_RAGRS" --allow-shared-key-access false --min-tls-version TLS1_2 --location "${LOCATION}" || return $?
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

# copy_optimized_vhd copies the prefetch-optimized VHD that the image builder template published to its
# staging storage account into the target storage account. Because the template distributes a VHD
# without specifying a target uri, image builder publishes the blob to a storage account within its own
# staging resource group and exposes its location via the run output's artifactUri. The destination vhd
# container may have an immutability policy that blocks azcopy from writing to it directly, so we:
# 1. azcopy the optimized VHD into the staging container (STORAGE_ACCOUNT_BLOB_URL_STAGING)
# 2. perform a single server-side blob copy from the staging container into the immutable vhd container
copy_optimized_vhd() {
    # The staging container should never have an immutability policy, so copying from image builder staging storage is
    # fully retriable. If a previous run already produced a complete staging blob (marked with the
    # prefetched metadata flag) but failed before finishing the server-side copy into the
    # immutable vhd container, reuse that staging blob and skip re-copying. An incomplete staging blob
    # left over from an interrupted copy is deleted and recreated.
    staging_info="$(az storage blob show --blob-url "${STAGING_VHD_URI}" --auth-mode login 2>/dev/null)"
    staging_blob_name="$(jq -r '.name // empty' <<< "${staging_info}" 2>/dev/null)"
    if [ -n "${staging_blob_name}" ] && [ "$(jq -r '.metadata.prefetched' <<< "${staging_info}")" = "true" ]; then
        echo "optimized VHD is already present and complete in staging at ${STAGING_VHD_URI}, skipping copy from image builder staging storage"
    else
        if [ -n "${staging_blob_name}" ]; then
            echo "found an incomplete staging VHD at ${STAGING_VHD_URI} from a previous run, will delete it before recopying"
            delete_staging_vhd || return $?
        fi

        artifact_uri="$(az resource show --ids "${IMAGE_BUILDER_TEMPLATE_ID}/runOutputs/${DISTRIBUTE_RUN_OUTPUT_NAME}" --api-version "${IMAGE_BUILDER_API_VERSION}" --query "properties.artifactUri" -o tsv 2>/dev/null)"
        if [ -z "${artifact_uri}" ] || [ "${artifact_uri,,}" = "null" ] || [ "${artifact_uri,,}" = "none" ]; then
            echo "unable to determine artifactUri for run output ${DISTRIBUTE_RUN_OUTPUT_NAME}, cannot continue"
            return 1
        fi
        echo "setting azcopy environment variables with pool identity: ${IMAGE_BUILDER_IDENTITY_ID}"
        export AZCOPY_AUTO_LOGIN_TYPE="AZCLI"
        export AZCOPY_CONCURRENCY_VALUE="AUTO"
        echo "copying optimized VHD from image builder staging storage to ${STAGING_VHD_URI}"
        azcopy copy "${artifact_uri}" "${STAGING_VHD_URI}" --recursive=true
        azcopy_exit_code=$?
        if [ "${azcopy_exit_code}" -ne 0 ]; then
            echo "failed to copy optimized VHD to staging location ${STAGING_VHD_URI}"
            return "${azcopy_exit_code}"
        fi

        # mark the staging blob complete so a later retry that failed before finishing the server-side
        # copy can detect it and skip re-copying from image builder staging storage.
        az storage blob metadata update --blob-url "${STAGING_VHD_URI}" --auth-mode login --metadata prefetched=true || return $?
    fi

    # server-side copy the VHD from the staging container into the immutable vhd container. This creates
    # the destination blob in a single operation, which is permitted by the container's immutability policy.
    echo "starting server-side copy of ${STAGING_VHD_URI} to ${VHD_URI}"
    az storage blob copy start \
        --account-name "${DESTINATION_STORAGE_ACCOUNT_NAME}" \
        --auth-mode login \
        --destination-container "${DESTINATION_CONTAINER_NAME}" \
        --destination-blob "${VHD_NAME}" \
        --source-uri "${STAGING_VHD_URI}" || return $?

    wait_for_vhd_copy || return $?

    echo "optimized VHD has been published to: ${VHD_URI}"
}

wait_for_vhd_copy() {
    copy_status="$(az storage blob show --blob-url "${VHD_URI}" --auth-mode login | jq -r '.properties.copy.status')"
    while [ "${copy_status,,}" = "pending" ]; do
        echo "server-side copy to ${VHD_URI} is still pending, will wait 30s before checking again"
        sleep 30s
        copy_status="$(az storage blob show --blob-url "${VHD_URI}" --auth-mode login | jq -r '.properties.copy.status')"
    done
    if [ "${copy_status,,}" != "success" ]; then
        echo "server-side copy to ${VHD_URI} finished with unexpected status: ${copy_status}"
        return 1
    fi
    echo "server-side copy to ${VHD_URI} completed successfully"
}

delete_staging_vhd() {
    az storage blob delete --blob-url "${STAGING_VHD_URI}" --auth-mode login || return $?
    while [ -n "$(az storage blob show --blob-url "${STAGING_VHD_URI}" --auth-mode login 2>/dev/null | jq -r '.name // empty')" ]; do
        echo "staging VHD ${STAGING_VHD_URI} has yet to be deleted, will wait 30s before checking again"
        sleep 30s
    done
    echo "${STAGING_VHD_URI} has been deleted"
}

# retry_on_rate_limit runs the provided command, retrying only when it fails because image builder has
# rate limited us (TooManyRequests - image builder rejects new work when too many tasks from a single
# subscription are running or waiting). Because this limit clears slowly, we back off heavily between
# attempts. Any other (non rate limit) failure is returned immediately without retrying.
retry_on_rate_limit() {
    local attempt=1
    local exit_code
    local logfile
    set +x
    logfile="$(mktemp)"
    while true; do
        # stream the command output live via tee while also capturing it so we can inspect it for the
        # rate limit error; PIPESTATUS[0] preserves the wrapped command's exit code (not tee's).
        "$@" 2>&1 | tee "${logfile}"
        exit_code="${PIPESTATUS[0]}"
        if [ "${exit_code}" -eq 0 ]; then
            rm -f "${logfile}"
            set -x
            return 0
        fi
        if ! grep -qi "TooManyRequests" "${logfile}" || [ "${attempt}" -ge "${RATE_LIMIT_MAX_ATTEMPTS}" ]; then
            rm -f "${logfile}"
            set -x
            return "${exit_code}"
        fi
        echo "command was rate limited by image builder (attempt ${attempt}/${RATE_LIMIT_MAX_ATTEMPTS}), waiting ${RATE_LIMIT_RETRY_DELAY_SECONDS}s before retrying..."
        sleep "${RATE_LIMIT_RETRY_DELAY_SECONDS}"
        attempt=$((attempt + 1))
    done
}

main "$@"
