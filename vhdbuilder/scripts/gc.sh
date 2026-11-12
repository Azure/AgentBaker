#!/bin/bash
set -euxo pipefail

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID must be set" && exit 1

SKIP_TAG_NAME="gc.skip"
SKIP_TAG_VALUE="true"

DRY_RUN="${DRY_RUN:-}"

STANDARD_RETENTION_SECONDS="${STANDARD_RETENTION_SECONDS:-14400}"
SKIP_RETENTION_SECONDS="${SKIP_RETENTION_SECONDS:-604800}"

for retention_seconds in "$STANDARD_RETENTION_SECONDS" "$SKIP_RETENTION_SECONDS"; do
    case "$retention_seconds" in
        ''|0*|*[!0-9]*)
            echo "STANDARD_RETENTION_SECONDS and SKIP_RETENTION_SECONDS must be positive integers in seconds" >&2
            exit 1
            ;;
    esac
done

STANDARD_DEADLINE=$(( $(date +%s) - STANDARD_RETENTION_SECONDS ))
SKIP_DEADLINE=$(( $(date +%s) - SKIP_RETENTION_SECONDS ))

function main() {
    az account set -s $SUBSCRIPTION_ID

    echo "garbage collecting ephemeral resource groups..."
    cleanup_rgs || exit $?

    # TODO(cameissner): migrate linux VHD build back-fill deletion logic to this script
}

function cleanup_rgs() {
    groups=$(az group list | jq -r --arg dl $STANDARD_DEADLINE '.[] | select(.name | test("vhd-test*|vhd-scanning*|pkr-Resource-Group*")) | select(.tags.now < $dl).name'  | tr -d '\"' || "")
    if [ -z "$groups" ]; then
        echo "no resource groups found for garbage collection"
        return 0
    fi

    for group in $groups; do
        echo "resource group $group is in-scope for garbage collection"
        group_object=$(az group show -g $group)
        tag_value=$(echo "$group_object" | jq -r --arg skipTagName $SKIP_TAG_NAME '.tags."\($skipTagName)"')

        if [ "${tag_value,,}" = "$SKIP_TAG_VALUE" ]; then
            now=$(echo "$group_object" | jq -r '.tags.now')
            if [ "$now" != "null" ] && [ "$now" -lt "$SKIP_DEADLINE" ]; then
                echo "resource group $group is tagged with $SKIP_TAG_NAME=$SKIP_TAG_VALUE but is more than $SKIP_RETENTION_SECONDS seconds old, will attempt to delete..."
                delete_group $group || return $?
            fi
            continue
        fi

        echo "will attempt to delete resource group $group"
        delete_group $group || return $?
    done
}

function delete_group() {
    local group=$1

    if [ "${DRY_RUN,,}" = "true" ]; then
        echo "DRY_RUN: az group delete -g $group --yes --no-wait"
        return 0
    fi

    if ! az group delete -g $group --yes --no-wait; then
        echo "failed to delete resource group: ${group}, continuing..."
    fi
}

main "$@"
