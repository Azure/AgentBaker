#!/bin/bash

# Build-time helpers for the dedicated Azure Linux 3 Kata L1VH preview image.
validate_l1vh_kata_preview() {
    case "${ENABLE_L1VH:-False}" in
        False|false|'') return 0 ;;
        True) ;;
        *) echo "ENABLE_L1VH must be True or False." >&2; return 1 ;;
    esac

    local fips="${ENABLE_FIPS:-}" trusted_launch="${ENABLE_TRUSTED_LAUNCH:-}" tl_supported="${TRUSTED_LAUNCH_SUPPORTED:-}"

    if [ "${MODE:-}" != "linuxVhdMode" ] || [ "${OS_SKU:-}" != "AzureLinux" ] ||
        [ "${OS_VERSION:-}" != "V3kata" ] || [ "${ARCHITECTURE:-}" != "X86_64" ] ||
        [ "${HYPERV_GENERATION:-}" != "V2" ] || [ "${FEATURE_FLAGS:-}" != "kata" ] ||
        [ "${fips,,}" != "false" ] || [ "${trusted_launch,,}" != "false" ] ||
        [ "${tl_supported,,}" != "false" ]; then
        echo "L1VH preview requires the non-FIPS, non-TL Azure Linux 3 x64 Gen2 Kata build." >&2
        return 1
    fi

    local source_pattern='^/subscriptions/[[:xdigit:]-]+/resourceGroups/[^/[:space:]]+/providers/Microsoft\.Compute/galleries/[[:alnum:]_.-]+/images/[[:alnum:]_.-]+/versions/[0-9]+\.[0-9]+\.[0-9]+$'
    # The repository lint pass also checks Bash helpers in POSIX mode.
    # shellcheck disable=SC3010
    if [[ ! ${L1VH_SOURCE_IMAGE_VERSION_ID:-} =~ $source_pattern ]]; then
        echo "L1VH_SOURCE_IMAGE_VERSION_ID must be a full private gallery image version ARM ID with a pinned numeric version (not latest)." >&2
        return 1
    fi

    case "${L1VH_SCHEDULER_TYPE:-}" in
        AzureManaged|GuestManaged) ;;
        *)
            echo "Set L1VH_SCHEDULER_TYPE to AzureManaged or GuestManaged after confirming the value with Compute/Azure Linux." >&2
            return 1
            ;;
    esac

    if [ "${SKU_NAME:-}" != "V3katagen2l1vhpreview" ] ||
        { [ -n "${SIG_IMAGE_NAME:-}" ] && [ "$SIG_IMAGE_NAME" != "AzureLinuxV3katagen2l1vhpreview" ]; }; then
        echo "L1VH preview must use its dedicated V3katagen2l1vhpreview SKU and AzureLinuxV3katagen2l1vhpreview build definition." >&2
        return 1
    fi
}

l1vh_image_features() {
    jq -nc --arg scheduler "$L1VH_SCHEDULER_TYPE" '[
        {name: "DiskControllerTypes", value: "SCSI,NVMe"},
        {name: "VirtualizationType", value: "Direct"},
        {name: "DirectVirtualizationSchedulerType", value: $scheduler}
    ]'
}

validate_l1vh_image_definition() {
    local definition="$1"
    jq -e --arg scheduler "$L1VH_SCHEDULER_TYPE" '
        .properties as $p |
        $p.osType == "Linux" and $p.osState == "Generalized" and
        $p.hyperVGeneration == "V2" and $p.architecture == "x64" and
        ([ $p.features[]? | select(.name == "VirtualizationType") ] ==
            [{name: "VirtualizationType", value: "Direct"}]) and
        ([ $p.features[]? | select(.name == "DirectVirtualizationSchedulerType") ] ==
            [{name: "DirectVirtualizationSchedulerType", value: $scheduler}]) and
        any($p.features[]?; .name == "DiskControllerTypes" and .value == "SCSI,NVMe") and
        all($p.features[]?; .name != "SecurityType")
    ' <<< "$definition" >/dev/null
}

ensure_l1vh_image_definition() {
    validate_l1vh_kata_preview || return 1
    local definitions existing_id definition body state attempt
    local url="https://management.azure.com/subscriptions/${GALLERY_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${SIG_IMAGE_NAME}?api-version=2025-12-03"

    # A failed read (including permission errors) must not be mistaken for an absent definition.
    definitions=$(az sig image-definition list --subscription "$GALLERY_SUBSCRIPTION_ID" \
        --resource-group "$AZURE_RESOURCE_GROUP_NAME" --gallery-name "$SIG_GALLERY_NAME" -o json) || return 1
    existing_id=$(jq -r --arg name "$SIG_IMAGE_NAME" '.[] | select(.name == $name) | .id' <<< "$definitions") || return 1
    if [ -z "$existing_id" ]; then
        body=$(jq -nc --arg location "$AZURE_LOCATION" --arg gallery "$SIG_GALLERY_NAME" \
            --arg name "$SIG_IMAGE_NAME" --argjson features "$(l1vh_image_features)" '{
                location: $location,
                properties: {
                    osType: "Linux", osState: "Generalized", hyperVGeneration: "V2", architecture: "x64",
                    identifier: {publisher: "microsoft-aks", offer: $gallery, sku: $name},
                    features: $features
                }
            }') || return 1
        az rest --method put --url "$url" --body "$body" --output none || return 1
    fi

    # Use the new API for read-back as well as creation; older SDKs may drop feature fields.
    for ((attempt = 0; attempt < 60; attempt++)); do
        definition=$(az rest --method get --url "$url" -o json) || return 1
        state=$(jq -r '.properties.provisioningState' <<< "$definition") || return 1
        case "$state" in
            Succeeded)
                if ! validate_l1vh_image_definition "$definition"; then
                    echo "L1VH preview definition has incompatible properties/features; refusing to modify an existing image family." >&2
                    return 1
                fi
                echo "Validated L1VH preview image definition ${SIG_IMAGE_NAME}"
                return 0
                ;;
            Creating|Updating) sleep 5 ;;
            *) echo "L1VH preview definition provisioning failed: $state" >&2; return 1 ;;
        esac
    done
    echo "Timed out waiting for L1VH preview image definition." >&2
    return 1
}

render_l1vh_packer_template() {
    local base_template="$1"
    # Keep IMG_OFFER/IMG_SKU available to OS-specific provisioning and artifact selection.
    # Only replace the actual Packer source fields; gallery source and destination are independent.
    jq --arg source "$L1VH_SOURCE_IMAGE_VERSION_ID" '
        del(.builders[0].image_publisher, .builders[0].image_offer,
            .builders[0].image_sku, .builders[0].image_version) |
        .builders[0].shared_image_gallery = {id: $source}
    ' "$base_template"
}

add_l1vh_publishing_info() {
    local info="$1"
    jq --arg source "$L1VH_SOURCE_IMAGE_VERSION_ID" --argjson features "$(l1vh_image_features)" '
        del(.publisher_base_image_version, .publisher_base_image_sku) |
        .source_image_version_id = $source |
        .gallery_image_features = $features
    ' <<< "$info"
}
