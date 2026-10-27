#!/bin/bash
set -euo pipefail

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID must be set" && exit 1

DRY_RUN="${DRY_RUN:-false}"
NO_WAIT="${NO_WAIT:-true}"

main() {
    az account set -s "$SUBSCRIPTION_ID"

    echo "attempting to garbage collect AgentBaker E2E test clusters in subscription: $SUBSCRIPTION_ID"
    garbage_collect || exit $?
}

garbage_collect() {
    groups=$(az group list 2>/dev/null | jq -r '.[] | select(.name | startswith("abe2e")) | .name' | tr -d '\"' || "")
    if [ -z "$groups" ]; then
        echo "no AgentBaker E2E resource groups found in subscription $SUBSCRIPTION_ID, nothing to garbage collect"
        return 0
    fi

    for group in $groups; do
        echo "will garbage collect clusters in resource group: $group"
        if ! garbage_collect_clusters_in_group "$group"; then
            echo "failed to garbage collect clusters in resource group: $group, continuing..."
        fi
    done
}

garbage_collect_clusters_in_group() {
    local group="$1"

    clusters=$(az aks list -g "$group" 2>/dev/null | jq -r '.[] | select(.name | startswith("abe2e")) | .name' | tr -d '\"' || "")
    if [ -z "$clusters" ]; then
        echo "no clusters found to garbage collect in resource group: $group"
        return 0
    fi

    for cluster in $clusters; do
        echo "deleting cluster $cluster..."
        if [ "${DRY_RUN}" = "true" ]; then
            echo "DRY_RUN: az aks delete -g $group -n $cluster --yes --no-wait=$NO_WAIT"
            continue
        fi
        if ! az aks delete -g "$group" -n "$cluster" --yes --no-wait="$NO_WAIT"; then
            echo "failed to delete cluster: $cluster, continuing..."
        fi
    done
}

main "$@"
