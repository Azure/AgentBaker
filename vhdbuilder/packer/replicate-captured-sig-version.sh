#!/bin/bash
set -euo pipefail

STORAGE_ACCOUNT_TYPE="Premium_LRS"
PACKER_GALLERY_NAME="PackerSigGalleryEastUS"

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID must be set when replicating packer SIG image version" && exit 1
[ -z "${RESOURCE_GROUP_NAME:-}" ] && echo "RESOURCE_GROUP_NAME must be set when replicating packer SIG image version" && exit 1
[ -z "${SIG_IMAGE_NAME:-}" ] && echo "SIG_IMAGE_NAME must be set when replicating packer SIG image version" && exit 1
[ -z "${CAPTURED_SIG_VERSION:-}" ] && echo "CAPTURED_SIG_VERSION must be set when replicating packer SIG image version" && exit 1

replicate_captured_sig_image_version() {
    if [ -z "${REPLICATIONS:-}" ]; then
        echo "no replications targets have been specified, exiting without replicating"
        exit 0
    fi

    IFS=',' read -ra replication_targets <<< "${REPLICATIONS}"
    local replication_string

    for replication_target in "${replication_targets[@]}"; do
        # shellcheck disable=SC3010
        if [[ ! "${replication_target}" =~ ^[^=]+=[0-9]+$ ]]; then
            echo "warning: invalid replication target format: '${replication_target}', expected format: <region>=<replicas>"
            continue
        fi

        IFS='=' read -r -a target <<< "${replication_target}"
        region=${target[0]}
        replicas=${target[1]}

        echo "will add replication target: ${region}=${replicas}"
        replication_string+=" --add publishingProfile.targetRegions name=${region} regionalReplicaCount=${replicas} storageAccountType=${STORAGE_ACCOUNT_TYPE}"
    done

    # once we migrate to HCL2 packer templates, this extra step will no longer be needed: https://developer.hashicorp.com/nomad/docs/reference/hcl2/functions/string/split
    command_string="az sig image-version update --subscription ${SUBSCRIPTION_ID} -g ${RESOURCE_GROUP_NAME} -r ${PACKER_GALLERY_NAME} -i ${SIG_IMAGE_NAME} -e ${CAPTURED_SIG_VERSION} $replication_string"
    echo "will replicate with command: ${command_string}"

    if [ "${DRY_RUN,,}" = "true" ]; then
        echo "DRY_RUN: exiting without running replication command"
        return 0
    fi

    if ! eval "$command_string"; then
        echo "failed to update SIG image version ${PACKER_GALLERY_NAME}/${SIG_IMAGE_NAME}/${CAPTURED_SIG_VERSION} with specified replication targets"
        exit 1
    fi
}

replicate_captured_sig_image_version
