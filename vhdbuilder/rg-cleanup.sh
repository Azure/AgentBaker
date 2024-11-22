#!/bin/bash
set -euxo pipefail

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID must be set" && exit 1
DRY_RUN="${DRY_RUN:-}"

DELETION_DEADLINE=$(( $(date +%s) - 86400 )) # 24 hours ago

function main() {
    az login --identity
    az account set -s $SUBSCRIPTION_ID

    echo "garbage collecting packer resource groups..."
    cleanup_packer_rgs || exit $?

    echo "garbage collecting VHD test resource groups..."
    cleanup_vhd_test_rgs || exit $?

    echo "garbage collecting VHD scanning resource groups..."
    cleanup_vhd_scanning_rgs || exit $?
}

function cleanup_packer_rgs() {
    groups=$(az group list | jq -r --arg dl $DELETION_DEADLINE '.[] | select(.name | test("pkr-Resource-Group*")) | select(.tags.now < $dl).name'  | tr -d '\"' || "")
    for group in $groups; do
        echo "packer resource group $group is more than 24 hours old, will delete..."
        if [ "${DRY_RUN,,}" == "true" ]; then
            echo "DRY_RUN: az group delete -g $group --yes --no-wait"
        else
            az group delete -g $group --yes --no-wait
        fi
    done
}

function cleanup_vhd_test_rgs() {
    groups=$(az group list | jq -r '.[] | select(.name | test("vhd-test*")).name' | tr -d '\"' || "")
    for group in $groups; do
        created_time=$(echo "$group" | cut -d'-' -f3)
        if [ $created_time -lt $DELETION_DEADLINE ]; then
            echo "test resource group $group is more than 24 hours old, will delete..."
            if [ "${DRY_RUN,,}" == "true" ]; then
                echo "DRY_RUN: az group delete -g $group --yes --no-wait"
            else
                az group delete -g $group --yes --no-wait
            fi
        fi
    done
}

function cleanup_vhd_scanning_rgs() {
    groups=$(az group list | jq -r '.[] | select(.name | test("vhd-scanning*")).name' | tr -d '\"' || "")
    for group in $groups; do
        created_time=$(echo "$group" | cut -d'-' -f3)
        if [ $created_time -lt $DELETION_DEADLINE ]; then
            echo "scanning resource group $group is more than 24 hours old, will delete..."
            if [ "${DRY_RUN,,}" == "true" ]; then
                echo "DRY_RUN: az group delete -g $group --yes --no-wait"
            else
                az group delete -g $group --yes --no-wait
            fi
        fi
    done
}

main "$@"