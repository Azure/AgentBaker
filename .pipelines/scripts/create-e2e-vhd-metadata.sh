#!/usr/bin/env bash

set -euo pipefail

metadata_file="${1:?metadata output path is required}"

if [ -z "${MANAGED_SIG_ID:-}" ]; then
    echo "##vso[task.logissue type=error]Packer did not return a managed SIG image version ID" >&2
    exit 1
fi

regions="$(jq -nc \
    --arg buildLocation "${PACKER_BUILD_LOCATION:?}" \
    --arg replications "${REPLICATIONS:-}" \
    '([$buildLocation] + ($replications | split(",") | map(split("=")[0] | gsub("^\\s+|\\s+$"; "")) | map(select(length > 0)))) | unique')"

metadata="$(jq -nc \
    --arg imageName "${SIG_IMAGE_NAME:?}" \
    --arg resourceId "${MANAGED_SIG_ID}" \
    --arg version "${CAPTURED_SIG_VERSION:?}" \
    --argjson regions "${regions}" \
    '{($imageName): {resourceId: $resourceId, version: $version, regions: $regions}}')"

case "${OS_SKU,,}:${FEATURE_FLAGS,,}" in
    ubuntu:*cvm*|azurelinux:*cvm*)
        # Preserve the capture entry; query the generalized version in the same gallery.
        gallery_id="${MANAGED_SIG_ID%/images/*}"
        generalized_id="${gallery_id}/images/${SKU_NAME:?}/versions/${CAPTURED_SIG_VERSION}"
        echo "Reading generalized CVM image ${SKU_NAME}/${CAPTURED_SIG_VERSION} for E2E metadata"
        generalized_version="$(
            az sig image-version show --ids "${generalized_id}" --expand ReplicationStatus \
                --output json --only-show-errors
        )" || {
            exit_code=$?
            echo "##vso[task.logissue type=error]Unable to read generalized CVM image ${generalized_id} for E2E metadata" >&2
            exit "${exit_code}"
        }

        if ! generalized_entry="$(jq -ce --arg expectedId "${generalized_id}" --arg version "${CAPTURED_SIG_VERSION}" '
            select((.id | type) == "string") |
            select((.id | ascii_downcase) == ($expectedId | ascii_downcase)) |
            select(.provisioningState == "Succeeded" and .replicationStatus.aggregatedState == "Completed") |
            {
                resourceId: .id,
                version: $version,
                regions: ([.replicationStatus.summary[]? |
                    select(.state == "Completed") | .region |
                    select(type == "string" and length > 0)] | unique)
            } |
            select(.regions | length > 0)
        ' <<<"${generalized_version}")"; then
            echo "##vso[task.logissue type=error]Generalized CVM image ${generalized_id} is not ready with confirmed replica regions; E2E metadata will not be published" >&2
            exit 1
        fi

        metadata="$(jq -c --arg imageName "${SKU_NAME}" --argjson entry "${generalized_entry}" \
            '. + {($imageName): $entry}' <<<"${metadata}")"
        echo "Added image ready for E2Es: ${SKU_NAME}/${CAPTURED_SIG_VERSION}, replicated regions: $(jq -r '.regions | join(", ")' <<<"${generalized_entry}")"
        ;;
esac

mkdir -p "$(dirname "${metadata_file}")"
printf '%s\n' "${metadata}" >"${metadata_file}"
echo "Created E2E VHD metadata: ${metadata_file}"
