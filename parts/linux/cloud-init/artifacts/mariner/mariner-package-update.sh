#!/usr/bin/env bash

set -o nounset
set -e

# Generic LPC reconciliation. Component handlers own payload schemas and node
# mutations; this entrypoint owns the envelope, dispatch, checkpoints and status.
OS_RELEASE_FILE="/etc/os-release"
KUBECONFIG="/var/lib/kubelet/kubeconfig"
KUBECTL="/opt/bin/kubectl --kubeconfig ${KUBECONFIG}"
: "${LIVE_PATCHING_CONFIG_NAMESPACE:=kube-system}"
: "${LIVE_PATCHING_CONFIGMAP:=live-patching-config}"
: "${LIVE_PATCHING_CONFIG_KEY_JSONPATH:=live-patching-config\.json}"
: "${LIVE_PATCHING_GOAL_ANNOTATION:=kubernetes.azure.com/live-patching-config-goal-hash}"
: "${LIVE_PATCHING_STATUS_ANNOTATION:=kubernetes.azure.com/live-patching-status}"
: "${LIVE_PATCHING_STATE_FILE:=/var/lib/aks/live-patching/current.json}"

LIVE_PATCHING_COMPONENT_RESULTS='{}'

wait_for_kubeconfig() {
    local n=0
    while [ ! -f "${KUBECONFIG}" ]; do
        echo 'Waiting for TLS bootstrapping'
        if [ "$n" -lt 100 ]; then
            n=$((n+1))
            sleep 3
        else
            echo "timeout waiting for kubeconfig to be present"
            return 1
        fi
    done
}

get_node_annotation() {
    local node_json="$1"
    local annotation="$2"

    printf '%s' "${node_json}" | jq -r --arg annotation "${annotation}" '(.metadata.annotations // {})[$annotation] // empty'
}

read_generic_config() {
    local goal="$1"
    local payload payload_with_sentinel payload_hash

    # shellcheck disable=SC2086
    if ! payload_with_sentinel="$($KUBECTL get cm -n "${LIVE_PATCHING_CONFIG_NAMESPACE}" "${LIVE_PATCHING_CONFIGMAP}" -o "jsonpath={.data.${LIVE_PATCHING_CONFIG_KEY_JSONPATH}}" && printf '.')"; then
        echo "failed to read live-patching-config ConfigMap" >&2
        return 1
    fi
    payload="${payload_with_sentinel%.}"
    if ! printf '%s' "${payload}" | jq -se '
        length == 1 and (.[0] |
        (type == "object") and
        (.components | type == "array") and
        (.components | all((.name | type == "string") and (.name | length > 0) and (.nodeConfig | type == "string"))) and
        ([.components[].name] | length) == ([.components[].name] | unique | length))
    ' > /dev/null; then
        echo "live-patching-config payload has invalid envelope" >&2
        return 1
    fi
    if ! printf '%s' "${goal}" | grep -Eq '^[0-9a-f]{64}$'; then
        echo "live-patching goal hash must be a 64-character lowercase sha256 digest" >&2
        return 1
    fi
    payload_hash=$(printf '%s' "${payload}" | sha256sum | awk '{print $1}')
    if [ "${payload_hash}" != "${goal}" ]; then
        echo "live-patching goal hash does not match ConfigMap payload: goal=${goal}, payload=${payload_hash}" >&2
        return 1
    fi
    printf '%s' "${payload}"
}

component_is_current() {
    local component="$1"
    local component_payload="$2"
    local node_json="$3"
    local comparator="$4"
    local current_payload

    [ -f "${LIVE_PATCHING_STATE_FILE}" ] || return 1
    current_payload=$(jq -er --arg component "${component}" '.components[$component].nodeConfig' "${LIVE_PATCHING_STATE_FILE}" 2> /dev/null) || return 1
    "${comparator}" "${component_payload}" "${current_payload}" "${node_json}"
}

write_component_checkpoint() {
    local component="$1"
    local component_payload="$2"
    local state_tmp="${LIVE_PATCHING_STATE_FILE}.tmp"
    local state='{"components":{}}'

    mkdir -p "$(dirname "${LIVE_PATCHING_STATE_FILE}")" || return 1
    if [ -f "${LIVE_PATCHING_STATE_FILE}" ]; then
        state=$(jq -c 'select(.components | type == "object") | {components:.components}' "${LIVE_PATCHING_STATE_FILE}" 2> /dev/null) || state='{"components":{}}'
        [ -n "${state}" ] || state='{"components":{}}'
    fi
    printf '%s' "${state}" | jq --arg component "${component}" --arg nodeConfig "${component_payload}" \
        '.components[$component] = {nodeConfig:$nodeConfig}' > "${state_tmp}" || return 1
    mv "${state_tmp}" "${LIVE_PATCHING_STATE_FILE}"
}

set_component_result() {
    local component="$1"
    local code="$2"
    local results

    results=$(printf '%s' "${LIVE_PATCHING_COMPONENT_RESULTS}" | jq -ce --arg component "${component}" --arg code "${code}" '.[$component]={code:$code}') || return 1
    [ -n "${results}" ] || return 1
    LIVE_PATCHING_COMPONENT_RESULTS="${results}"
}

write_generic_status() {
    local node_name="$1"
    local goal="$2"
    local status

    status=$(printf '%s' "${LIVE_PATCHING_COMPONENT_RESULTS}" | jq -ce --arg currentHash "${goal}" '{currentHash:$currentHash,components:.}') || return 1
    [ -n "${status}" ] || return 1
    # shellcheck disable=SC2086
    $KUBECTL annotate --overwrite node "${node_name}" "${LIVE_PATCHING_STATUS_ANNOTATION}=${status}"
}

generic_main() {
    local node_json="$1"
    local goal="$2"
    local node_name status payload component_count component_name component_payload
    local component_handler component_comparator
    local component_index=0
    local failed=false

    node_name=$(printf '%s' "${node_json}" | jq -er '.metadata.name // empty') || return 1
    if ! printf '%s' "${goal}" | grep -Eq '^[0-9a-f]{64}$'; then
        echo "live-patching goal hash must be a 64-character lowercase sha256 digest" >&2
        return 1
    fi
    status=$(get_node_annotation "${node_json}" "${LIVE_PATCHING_STATUS_ANNOTATION}") || return 1
    if [ -n "${status}" ] && printf '%s' "${status}" | jq -e --arg goal "${goal}" \
        '.currentHash == $goal and (.components | type == "object") and (.components | all(.code == "Succeeded"))' > /dev/null 2>&1; then
        echo "live-patching goal is already converged, nothing to apply"
        return 0
    fi
    payload=$(read_generic_config "${goal}") || return 1
    component_count=$(printf '%s' "${payload}" | jq -r '.components | length') || return 1
    LIVE_PATCHING_COMPONENT_RESULTS='{}'

    while [ "${component_index}" -lt "${component_count}" ]; do
        component_name=$(printf '%s' "${payload}" | jq -r --argjson index "${component_index}" '.components[$index].name') || return 1
        component_payload=$(printf '%s' "${payload}" | jq -r --argjson index "${component_index}" '.components[$index].nodeConfig') || return 1
        case "${component_name}" in
            securityPatch)
                component_handler=updateSecurityPatch
                component_comparator=securityPatchIsCurrent
                ;;
            *)
                echo "unsupported component: ${component_name}"
                component_index=$((component_index + 1))
                continue
                ;;
        esac
        echo "applying component: ${component_name}"
        if component_is_current "${component_name}" "${component_payload}" "${node_json}" "${component_comparator}"; then
            echo "component is already current: ${component_name}"
            set_component_result "${component_name}" Succeeded || return 1
        elif "${component_handler}" "${component_payload}" "${node_json}" && write_component_checkpoint "${component_name}" "${component_payload}"; then
            set_component_result "${component_name}" Succeeded || return 1
        else
            echo "component failed: ${component_name}"
            set_component_result "${component_name}" Failed || return 1
            failed=true
        fi
        component_index=$((component_index + 1))
    done

    if ! write_generic_status "${node_name}" "${goal}"; then
        echo "failed to update live-patching status annotation"
        return 1
    fi
    if [ "${failed}" = true ]; then
        return 1
    fi
    echo "generic live-patching completed successfully"
}

main() {
    local version_id node_name node_json goal

    if ! wait_for_kubeconfig; then
        return 1
    fi
    version_id=$(grep '^VERSION_ID=' "${OS_RELEASE_FILE}" | cut -d'=' -f2 | tr -d '"')
    if [ "${version_id}" != "3.0" ]; then
        legacy_main
        return
    fi
    node_name=$(hostname)
    if [ -z "${node_name}" ]; then
        echo "cannot get node name"
        return 1
    fi
    node_name=$(printf '%s' "${node_name}" | tr '[:upper:]' '[:lower:]')
    # shellcheck disable=SC2086
    node_json=$($KUBECTL get node "${node_name}" -o json) || return 1
    goal=$(get_node_annotation "${node_json}" "${LIVE_PATCHING_GOAL_ANNOTATION}") || return 1
    if [ -z "${goal}" ]; then
        legacy_main
        return
    fi
    echo "live-patching goal is: ${goal}"
    generic_main "${node_json}" "${goal}"
}

${__SOURCED__:+return}
# shellcheck disable=SC1091
source /opt/azure/containers/security-update.sh
main "$@"
