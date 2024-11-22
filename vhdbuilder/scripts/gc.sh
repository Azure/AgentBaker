#!/bin/bash
set -euxo pipefail

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID must be set" && exit 1

SKIP_TAG_NAME="${SKIP_TAG_NAME:-gc.skip}"
SKIP_TAG_VALUE="${SKIP_TAG_VALUE:-true}"

DRY_RUN="${DRY_RUN:-}"

DAY_AGO=$(( $(date +%s) - 86400 )) # 24 hours ago
WEEK_AGO=$(( $(date +%s) - 604800 )) # 7 days ago


function main() {
    az login --identity # relies on an appropriately permissioned identity being attached to the build agent
    az account set -s $SUBSCRIPTION_ID

    echo "garbage collecting ephemeral resource groups..."
    cleanup_rgs || exit $?
}

function cleanup_rgs() {
    groups=$(az group list | jq -r --arg dl $DAY_AGO '.[] | select(.name | test("vhd-test*|vhd-scanning*|pkr-Resource-Group*")) | select(.tags.now < $dl).name'  | tr -d '\"' || "")
    for group in $groups; do
        echo "resource group $group is more than 24 hours old"
        group_object=$(az group show -g $group)
        tag_value=$(echo "$group_object" | jq -r --arg skipTagName $SKIP_TAG_NAME '.tags."\($skipTagName)"')

        if [ "${tag_value,,}" == "$SKIP_TAG_VALUE" ]; then
            now=$(echo "$group_object" | jq -r '.tags.now')
            if [ "$now" != "null" ] && [ $now -lt $WEEK_AGO ]; then
                echo "resource group $group is tagged with $SKIP_TAG_NAME=$SKIP_TAG_VALUE but is more than 7 days old, will attempt to delete..."
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

    if [ "${DRY_RUN,,}" == "true" ]; then
        echo "DRY_RUN: az group delete -g $group --yes --no-wait"
        return 0
    fi

    az group delete -g $group --yes --no-wait || return $?
}

main "$@"