#!/bin/bash
set -euo pipefail

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID must be set" && exit 1

DRY_RUN="${DRY_RUN:-false}"
NO_WAIT="${NO_WAIT:-false}"

main() {
    az account set -s "$SUBSCRIPTION_ID"

    echo "attempting to garbage collect AgentBaker E2E resource groups in subscription: $SUBSCRIPTION_ID"
    garbage_collect || exit $?
}

garbage_collect() {
    groups=$(az group list 2>/dev/null | jq -r '.[] | select(.name | startswith("abe2e")) | .name' | tr -d '\"' || "")
    if [ -z "$groups" ]; then
        echo "no AgentBaker E2E resource groups found in subscription $SUBSCRIPTION_ID, nothing to garbage collect"
        return 0
    fi

     local -a no_wait_args=()
     if [ "${NO_WAIT,,}" = "true" ]; then
         no_wait_args+=(--no-wait)
     fi

    for group in $groups; do
        echo "will delete resource group: $group"
        if [ "${DRY_RUN,,}" = "true" ]; then
            echo "DRY_RUN: az group delete -g $group --yes ${no_wait_args[*]}"
            continue
        fi
        if ! az group delete -g "$group" --yes "${no_wait_args[@]}"; then
            echo "failed to delete resource group: $group, continuing..."
        fi
    done
}

main "$@"
