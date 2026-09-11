#!/bin/bash
# Generates the temporary Packer template used by CVM Stage 2 (final,
# CVM_BUILD_STAGE=final) builds.
#
# Stage 2 must run the *unmodified* provisioner sequence in
# vhd-image-builder-cvm.json (the same one the existing single-stage 22.04/
# 24.04 CVM builds use) against a real ConfidentialVM, but sourced from the
# exact Stage 1 (bootstrap) Shared Image Gallery version instead of a
# Marketplace image -- Ubuntu 26.04 has no ready-made "cvm" Marketplace SKU
# yet, which is the entire reason Stage 1 exists.
#
# This script copies vhd-image-builder-cvm.json byte-for-byte except for the
# builder's image source: the Marketplace source keys (image_publisher,
# image_offer, image_sku, image_version) are entirely removed (not set to
# empty strings -- Packer's azure-arm builder treats an empty string as "set"
# for its "exactly one source" validation) and replaced with a
# "shared_image_gallery" block pointing at the Stage 1 image. Every other
# builder property (security_type, secure_boot_enabled, vtpm_enabled,
# shared_image_gallery_destination, azure_tags, vm_size, etc.), all
# provisioners, and the error-cleanup-provisioner are left untouched.
#
# The generated file is a build artifact, not a source file: it is written
# fresh on every Stage 2 run and removed by the caller (see
# packer.mk:run-packer-cvm-final) once the packer build finishes, regardless
# of outcome.
#
# Required environment variables (set as pipeline variables by Stage 1 -- see
# "Build VHD - CVM Bootstrap (Stage 1)" in .builder-release-template.yaml):
#   CVM_BOOTSTRAP_SUBSCRIPTION_ID
#   CVM_BOOTSTRAP_RESOURCE_GROUP_NAME
#   CVM_BOOTSTRAP_SIG_GALLERY_NAME
#   CVM_BOOTSTRAP_SIG_IMAGE_NAME
#   CVM_BOOTSTRAP_SIG_IMAGE_VERSION
#
# Optional environment variables:
#   CVM_BASE_TEMPLATE   input template path (default vhdbuilder/packer/vhd-image-builder-cvm.json)
#   CVM_FINAL_TEMPLATE  output template path (default vhdbuilder/packer/vhd-image-builder-cvm-final.generated.json)

set -euo pipefail

CVM_BASE_TEMPLATE="${CVM_BASE_TEMPLATE:-vhdbuilder/packer/vhd-image-builder-cvm.json}"
CVM_FINAL_TEMPLATE="${CVM_FINAL_TEMPLATE:-vhdbuilder/packer/vhd-image-builder-cvm-final.generated.json}"

# assertRequiredCvmBootstrapVarsSet fails with a clear message if any of the
# exact Stage 1 image identity variables are missing.
assertRequiredCvmBootstrapVarsSet() {
    local required_var
    local missing=0
    for required_var in \
        CVM_BOOTSTRAP_SUBSCRIPTION_ID \
        CVM_BOOTSTRAP_RESOURCE_GROUP_NAME \
        CVM_BOOTSTRAP_SIG_GALLERY_NAME \
        CVM_BOOTSTRAP_SIG_IMAGE_NAME \
        CVM_BOOTSTRAP_SIG_IMAGE_VERSION; do
        if [ -z "${!required_var:-}" ]; then
            echo "ERROR: ${required_var} must be set (by Stage 1) to generate the CVM final-stage packer template" >&2
            missing=1
        fi
    done
    return "${missing}"
}

# generateCvmFinalTemplate writes a CVM final-stage packer template to
# $2, derived from the base template at $1, with the Marketplace source keys
# on builders[0] replaced by a shared_image_gallery block built from the
# CVM_BOOTSTRAP_* environment variables. Returns non-zero without modifying
# $2 if $1 is missing or the result is not valid JSON.
generateCvmFinalTemplate() {
    local base_template="$1"
    local final_template="$2"
    local tmp_template="${final_template}.tmp"

    if [ ! -f "${base_template}" ]; then
        echo "ERROR: base CVM template ${base_template} not found" >&2
        return 1
    fi

    jq \
        --arg subscription "${CVM_BOOTSTRAP_SUBSCRIPTION_ID}" \
        --arg resource_group "${CVM_BOOTSTRAP_RESOURCE_GROUP_NAME}" \
        --arg gallery_name "${CVM_BOOTSTRAP_SIG_GALLERY_NAME}" \
        --arg image_name "${CVM_BOOTSTRAP_SIG_IMAGE_NAME}" \
        --arg image_version "${CVM_BOOTSTRAP_SIG_IMAGE_VERSION}" \
        '
        .builders = [
          .builders[0]
          | del(.image_publisher, .image_offer, .image_sku, .image_version)
          | .shared_image_gallery = {
              subscription: $subscription,
              resource_group: $resource_group,
              gallery_name: $gallery_name,
              image_name: $image_name,
              image_version: $image_version
            }
        ]
        | .variables |= del(.img_publisher, .img_offer, .img_version)
        ' "${base_template}" > "${tmp_template}"

    if ! jq empty "${tmp_template}" >/dev/null 2>&1; then
        echo "ERROR: generated CVM final-stage template is not valid JSON" >&2
        rm -f "${tmp_template}"
        return 1
    fi

    mv "${tmp_template}" "${final_template}"
}

main() {
    if ! assertRequiredCvmBootstrapVarsSet; then
        exit 1
    fi

    echo "Generating CVM final-stage packer template ${CVM_FINAL_TEMPLATE} from ${CVM_BASE_TEMPLATE}"
    echo "Source (Stage 1 bootstrap image): subscription=${CVM_BOOTSTRAP_SUBSCRIPTION_ID} resource_group=${CVM_BOOTSTRAP_RESOURCE_GROUP_NAME} gallery=${CVM_BOOTSTRAP_SIG_GALLERY_NAME} image=${CVM_BOOTSTRAP_SIG_IMAGE_NAME} version=${CVM_BOOTSTRAP_SIG_IMAGE_VERSION}"

    if ! generateCvmFinalTemplate "${CVM_BASE_TEMPLATE}" "${CVM_FINAL_TEMPLATE}"; then
        exit 1
    fi

    echo "Generated CVM final-stage packer template: ${CVM_FINAL_TEMPLATE}"
}

# Allow the pure functions above to be sourced (e.g. by ShellSpec) without
# executing main.
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi
