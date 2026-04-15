#!/bin/bash
set -euo pipefail

# Updates base_image_version in windows_settings.json to the latest available
# version from the Azure Marketplace for each Windows SKU.
#
# This script is a backup. In an ideal world, use the pipeline https://dev.azure.com/msazure/CloudNativeCompute/_build/results?buildId=160464184&view=results
# and only run this script manually if the pipeline fails to update the versions or if you want to check for updates without modifying the settings file (using --dry-run).
#
# Prerequisites:
#   - az CLI installed and logged in
#   - jq installed
#
# Usage:
#   ./update_windows_base_versions.sh [--dry-run]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SETTINGS_FILE="${SCRIPT_DIR}/windows_settings.json"
DEFAULT_PUBLISHER="MicrosoftWindowsServer"
DEFAULT_OFFER="MicrosoftWindowsServer"
DRY_RUN=false

if [ "${1:-}" = "--dry-run" ]; then
    DRY_RUN=true
fi

if [ ! -f "${SETTINGS_FILE}" ]; then
    echo "ERROR: ${SETTINGS_FILE} not found"
    exit 1
fi

for cmd in az jq; do
    if ! command -v "${cmd}" &>/dev/null; then
        echo "ERROR: ${cmd} is required but not installed"
        exit 1
    fi
done

# Build a list of unique (publisher, sku) pairs to query.
# Multiple JSON keys can share the same SKU prefix (e.g. 2025 and 2025-gen2
# have different SKUs but should resolve to the same base_image_version).
# We query each unique SKU individually.
mapfile -t sku_keys < <(jq -r '.WindowsBaseVersions | keys[]' "${SETTINGS_FILE}")

updated=0
errors=0

for key in "${sku_keys[@]}"; do
    sku=$(jq -r ".WindowsBaseVersions.\"${key}\".base_image_sku" "${SETTINGS_FILE}")
    offer=$(jq -r ".WindowsBaseVersions.\"${key}\".base_image_offer // \"${DEFAULT_OFFER}\"" "${SETTINGS_FILE}")
    publisher=$(jq -r ".WindowsBaseVersions.\"${key}\".base_image_publisher // \"${DEFAULT_PUBLISHER}\"" "${SETTINGS_FILE}")
    current_version=$(jq -r ".WindowsBaseVersions.\"${key}\".base_image_version" "${SETTINGS_FILE}")

    echo "Querying latest version for ${key} (publisher=${publisher}, offer=${offer}, sku=${sku})..."

    latest_version=$(
        az vm image list \
            -p "${publisher}" \
            -f "${offer}" \
            -s "${sku}" \
            --all \
            --query "[].version" \
            -o tsv 2>/dev/null |
            sort -uV |
            tail -n 1
    ) || true

    if [ -z "${latest_version}" ]; then
        echo "  WARNING: no versions found for sku=${sku}, skipping"
        errors=$((errors + 1))
        continue
    fi

    if [ "${current_version}" = "${latest_version}" ]; then
        echo "  ${key}: already up to date (${current_version})"
        continue
    fi

    echo "  ${key}: ${current_version} -> ${latest_version}"

    if [ "${DRY_RUN}" = "false" ]; then
        tmp=$(mktemp)
        jq ".WindowsBaseVersions.\"${key}\".base_image_version = \"${latest_version}\"" \
            "${SETTINGS_FILE}" >"${tmp}" &&
            mv "${tmp}" "${SETTINGS_FILE}"
        updated=$((updated + 1))
    else
        updated=$((updated + 1))
    fi
done

echo ""
if [ "${DRY_RUN}" = "true" ]; then
    echo "Dry run complete. ${updated} version(s) would be updated, ${errors} error(s)."
else
    echo "Done. ${updated} version(s) updated, ${errors} error(s)."
fi

if [ "${errors}" -gt 0 ]; then
    exit 1
fi

