#!/bin/bash
set -euo pipefail

PACKER_GALLERY_NAME="PackerSigGalleryEastUS"

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID must be set when replicating captured SIG image version" && exit 1
[ -z "${RESOURCE_GROUP_NAME:-}" ] && echo "RESOURCE_GROUP_NAME must be set when replicating captured SIG image version" && exit 1
[ -z "${SIG_IMAGE_NAME:-}" ] && echo "SIG_IMAGE_NAME must be set when replicating captured SIG image version" && exit 1
[ -z "${CAPTURED_SIG_VERSION:-}" ] && echo "CAPTURED_SIG_VERSION must be set when replicating captured SIG image version" && exit 1
[ -z "${PACKER_BUILD_LOCATION:-}" ] && echo "PACKER_BUILD_LOCATION must be set when replicating captured SIG image version" && exit 1

replicate_captured_sig_image_version() {
    if [ -z "${REPLICATIONS:-}" ]; then
        echo "no replications targets have been specified, exiting without replicating"
        return 0
    fi

    IFS=',' read -ra replication_targets <<< "${REPLICATIONS}"
    local target_regions
    local updated_packer_build_location_replicas

    for replication_target in "${replication_targets[@]}"; do
        # shellcheck disable=SC3010
        if [[ ! "${replication_target}" =~ ^[^=]+=[0-9]+$ ]]; then
            echo "warning: invalid replication target format: '${replication_target}', expected format: <region>=<replicas>"
            continue
        fi
        
        IFS='=' read -r -a target <<< "${replication_target}"
        region=${target[0]}
        if [ "${region,,}" = "${PACKER_BUILD_LOCATION,,}" ]; then
            updated_packer_build_location_replicas="true"
        fi

        target_regions+=" $replication_target"
    done

    if [ "${updated_packer_build_location_replicas:-}" != "true" ]; then
        # this is needed since SIG API requires specifying a complete set of replication targets
        # packer, by default, will only create a single replica when publishing the freshly-built image version
        target_regions+=" ${PACKER_BUILD_LOCATION}"
    fi

    # once we migrate to HCL2 packer templates, this extra step will no longer be needed: https://developer.hashicorp.com/nomad/docs/reference/hcl2/functions/string/split
    command_string="az sig image-version update --subscription ${SUBSCRIPTION_ID} -g ${RESOURCE_GROUP_NAME} -r ${PACKER_GALLERY_NAME} -i ${SIG_IMAGE_NAME} -e ${CAPTURED_SIG_VERSION} --target-regions${target_regions}"
    echo "will replicate with command: ${command_string}"

    if [ "${DRY_RUN,,}" = "true" ]; then
        echo "DRY_RUN: exiting without running replication command"
        return 0
    fi

    if ! eval "$command_string"; then
        echo "failed to update SIG image version ${SUBSCRIPTION_ID}/${RESOURCE_GROUP_NAME}/${PACKER_GALLERY_NAME}/${SIG_IMAGE_NAME}/${CAPTURED_SIG_VERSION} with specified replication targets"
        return 1
    fi
}

replicate_captured_sig_image_version || exit $?
