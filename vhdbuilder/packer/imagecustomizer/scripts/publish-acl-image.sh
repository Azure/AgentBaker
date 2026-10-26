#!/bin/bash
set -euo pipefail

[[ "${OS_SKU:-}" == "AzureContainerLinux" ]]
for name in CAPTURED_SIG_VERSION SIG_IMAGE_NAME SIG_GALLERY_NAME RESOURCE_GROUP_NAME \
    PACKER_BUILD_LOCATION DESTINATION_STORAGE_CONTAINER STORAGE_ACCOUNT_NAME \
    VHD_ARTIFACT_NAME E2E_METADATA_DIR; do
    if [[ -z "${!name:-}" ]]; then
        echo "$name must be set" >&2
        exit 1
    fi
done

# Packer uses a random patch component in [0, 32767]. Reserve a disjoint
# range for its customized successor, without modifying the original SIG.
if [[ ! "$CAPTURED_SIG_VERSION" =~ ^[0-9]+\.[0-9]+\.([0-9]+)$ ]] ||
    (( 10#${BASH_REMATCH[1]:-0} >= 100000 )); then
    echo "Expected a Packer SIG version with patch component below 100000" >&2
    exit 1
fi
customized_version="${CAPTURED_SIG_VERSION%.*}.$((10#${BASH_REMATCH[1]} + 100000))"
vhd_url="${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd"

regions_json="$(jq -nc --arg location "$PACKER_BUILD_LOCATION" --arg replicas "${REPLICATIONS:-}" \
    '([$location] + ($replicas | split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0)))) | unique_by(split("=")[0])')"
mapfile -t regions < <(jq -r '.[]' <<< "$regions_json")
resource_id="$(az sig image-version create \
    --resource-group "$RESOURCE_GROUP_NAME" \
    --gallery-name "$SIG_GALLERY_NAME" \
    --gallery-image-definition "$SIG_IMAGE_NAME" \
    --gallery-image-version "$customized_version" \
    --location "$PACKER_BUILD_LOCATION" \
    --os-vhd-storage-account "$STORAGE_ACCOUNT_NAME" \
    --os-vhd-uri "$vhd_url" \
    --target-regions "${regions[@]}" \
    --replica-count 1 --only-show-errors --query id -o tsv)"
if [[ ! "$resource_id" =~ /versions/"$customized_version"$ ]]; then
    echo "SIG publication did not return the customized image version" >&2
    exit 1
fi

mkdir -p "$E2E_METADATA_DIR"
jq -n --arg name "$SIG_IMAGE_NAME" --arg id "$resource_id" \
    --arg version "$customized_version" --argjson regions "$regions_json" \
    '{($name): {resourceId: $id, version: $version, regions: ($regions | map(split("=")[0]))}}' \
    > "$E2E_METADATA_DIR/${VHD_ARTIFACT_NAME}.json"

generate_publishing_info="${GENERATE_PUBLISHING_INFO:-false}"
if [[ "${generate_publishing_info,,}" == "true" ]]; then
    jq --arg id "$resource_id" '.captured_sig_resource_id = $id' \
        "${ACL_PUBLISHING_INPUT:?}" > vhd-publishing-info.json
fi
