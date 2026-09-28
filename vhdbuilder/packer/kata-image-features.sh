#!/bin/bash

is_azurelinux3_kata_image() {
    # Match the existing release SKU, not every image with a Kata-related feature flag.
    [ "${OS_SKU:-}" = "AzureLinux" ] && [ "${SKU_NAME:-}" = "V3katagen2" ]
}

kata_image_features() {
    jq -nc '[
        {name: "VirtualizationType", value: "Direct"},
        {name: "DirectVirtualizationSchedulerType", value: "GuestManaged"}
    ]'
}

wait_for_kata_image_definition() {
    local url="$1" definition state attempt
    for ((attempt = 0; attempt < 60; attempt++)); do
        definition=$(az rest --method get --url "$url" -o json) || return 1
        state=$(jq -r '.properties.provisioningState' <<< "$definition") || return 1
        case "$state" in
            Succeeded) printf '%s\n' "$definition"; return 0 ;;
            Creating|Updating) sleep 5 ;;
            *) echo "Kata image definition provisioning failed: $state" >&2; return 1 ;;
        esac
    done
    echo "Timed out waiting for Kata image definition." >&2
    return 1
}

ensure_kata_image_features() {
    if ! is_azurelinux3_kata_image; then
        return 0
    fi

    local definition features body
    local url="/subscriptions/${GALLERY_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${SIG_IMAGE_NAME}?api-version=2025-12-03"
    definition=$(wait_for_kata_image_definition "$url") || return 1

    # Merge only the two virtualization features, preserving NVMe, security, and future features.
    features=$(jq -c --argjson required "$(kata_image_features)" '
        [ .properties.features[]? |
          select(.name != "VirtualizationType" and .name != "DirectVirtualizationSchedulerType") ] + $required
    ' <<< "$definition") || return 1
    if jq -e --argjson features "$features" '
        ((.properties.features // []) | sort_by(.name)) == ($features | sort_by(.name))
    ' <<< "$definition" >/dev/null; then
        echo "Kata image definition ${SIG_IMAGE_NAME} already has direct virtualization features"
        return 0
    fi

    # PATCH retains resource tags and unrelated properties. Include the API's required fields.
    body=$(jq -c --argjson features "$features" '{properties: {
        osType: .properties.osType, osState: .properties.osState,
        identifier: .properties.identifier, hyperVGeneration: .properties.hyperVGeneration,
        allowUpdateImage: true, features: $features
    }}' <<< "$definition") || return 1
    az rest --method patch --url "$url" --body "$body" --output none || return 1
    definition=$(wait_for_kata_image_definition "$url") || return 1
    if ! jq -e --argjson features "$features" '
        ((.properties.features // []) | sort_by(.name)) == ($features | sort_by(.name))
    ' <<< "$definition" >/dev/null; then
        echo "Kata image definition features did not match after update." >&2
        return 1
    fi
    echo "Updated Kata image definition ${SIG_IMAGE_NAME} with direct virtualization features"
}
