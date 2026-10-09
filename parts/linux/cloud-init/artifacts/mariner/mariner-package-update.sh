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
: "${KNEAD_EVENTS_LOGGING_DIR:=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events}"

LIVE_PATCHING_COMPONENT_RESULTS='{}'
LIVE_PATCHING_COMPONENT_RESULTS_VALID=true

# Same Guest Agent transport and schema as the Ubuntu reconciler.
knead_emit_event() {
    local task="$1"
    local message="$2"
    local level="${3:-Informational}"
    local events_file_name
    events_file_name="$(date +%s%3N)"
    local timestamp
    timestamp="$(date +"%F %T.%3N")"
    local event_json
    event_json="$(jq -n \
        --arg Timestamp "${timestamp}" --arg OperationId "${timestamp}" \
        --arg Version "1.23" --arg TaskName "${task}" --arg EventLevel "${level}" \
        --arg Message "${message}" --arg EventPid "0" --arg EventTid "0" \
        '{Timestamp:$Timestamp,OperationId:$OperationId,Version:$Version,TaskName:$TaskName,EventLevel:$EventLevel,Message:$Message,EventPid:$EventPid,EventTid:$EventTid}')"
    mkdir -p "${KNEAD_EVENTS_LOGGING_DIR}"
    printf '%s\n' "${event_json}" > "${KNEAD_EVENTS_LOGGING_DIR%/}/${events_file_name}.json"
}

knead_emit_reconcile_event() {
    local outcome="$1"
    local reason="$2"
    local level="${3:-Informational}"
    knead_emit_event "AKS.LivePatching.reconcile" "${outcome}: ${reason}" "${level}" || true
}

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
        (.components | all((.name | type == "string") and (.name | length > 0) and
            (.name | test("^[A-Za-z][A-Za-z0-9_-]*\\z")) and
            (.nodeConfig | type == "string"))) and
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
    current_payload=$(jq -er --arg component "${component}" '.components | map(select(.name == $component)) | last | .nodeConfig' "${LIVE_PATCHING_STATE_FILE}" 2> /dev/null) || return 1
    "${comparator}" "${component_payload}" "${current_payload}" "${node_json}"
}

write_component_checkpoint() {
    local component="$1"
    local component_payload="$2"
    local state_tmp="${LIVE_PATCHING_STATE_FILE}.tmp"
    local state='{"components":[]}'
    local updated_at
    updated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

    mkdir -p "$(dirname "${LIVE_PATCHING_STATE_FILE}")" || return 1
    if [ -f "${LIVE_PATCHING_STATE_FILE}" ]; then
        state=$(jq -c 'select(.components | type == "array") | {components:[.components[] | select(type == "object") | select((.name | type == "string") and (.nodeConfig | type == "string"))]}' "${LIVE_PATCHING_STATE_FILE}" 2> /dev/null) || state='{"components":[]}'
        [ -n "${state}" ] || state='{"components":[]}'
    fi
    printf '%s' "${state}" | jq --arg component "${component}" --arg nodeConfig "${component_payload}" --arg updatedAt "${updated_at}" \
        '.updatedAt = $updatedAt | .components = ([.components[] | select(.name != $component)] + [{name:$component,nodeConfig:$nodeConfig}])' > "${state_tmp}" || return 1
    mv "${state_tmp}" "${LIVE_PATCHING_STATE_FILE}"
}

set_component_result() {
    local component="$1"
    local code="$2"
    local results

    if ! results=$(printf '%s' "${LIVE_PATCHING_COMPONENT_RESULTS}" | jq -ce --arg component "${component}" --arg code "${code}" '.[$component]={code:$code}') || [ -z "${results}" ]; then
        echo "failed to record component result: ${component}=${code}"
        LIVE_PATCHING_COMPONENT_RESULTS_VALID=false
        return 1
    fi
    LIVE_PATCHING_COMPONENT_RESULTS="${results}"
}

write_generic_status() {
    local node_name="$1"
    local goal="$2"
    local status

    if [ "${LIVE_PATCHING_COMPONENT_RESULTS_VALID}" != true ]; then
        echo "refusing to write incomplete live-patching status"
        return 1
    fi
    status=$(printf '%s' "${LIVE_PATCHING_COMPONENT_RESULTS}" | jq -ce --arg currentHash "${goal}" '{currentHash:$currentHash,components:.}') || return 1
    [ -n "${status}" ] || return 1
    # shellcheck disable=SC2086
    $KUBECTL annotate --overwrite node "${node_name}" "${LIVE_PATCHING_STATUS_ANNOTATION}=${status}"
}

apply_components() {
    local payload="$1"
    local node_json="$2"
    local component_count component_name component_payload
    local component_handler component_comparator
    local component_index=0
    local failed=false
    LIVE_PATCHING_COMPONENT_RESULTS='{}'
    LIVE_PATCHING_COMPONENT_RESULTS_VALID=true
    if ! component_count=$(printf '%s' "${payload}" | jq -er '.components | length'); then
        echo "failed to read component count"
        LIVE_PATCHING_COMPONENT_RESULTS_VALID=false
        return 1
    fi

    while [ "${component_index}" -lt "${component_count}" ]; do
        if ! component_name=$(printf '%s' "${payload}" | jq -er --argjson index "${component_index}" '.components[$index].name'); then
            echo "failed to read component name at index: ${component_index}"
            LIVE_PATCHING_COMPONENT_RESULTS_VALID=false
            failed=true
            component_index=$((component_index + 1))
            continue
        fi
        if ! component_payload=$(printf '%s' "${payload}" | jq -er --argjson index "${component_index}" '.components[$index].nodeConfig'); then
            echo "failed to read component payload: ${component_name}"
            LIVE_PATCHING_COMPONENT_RESULTS_VALID=false
            failed=true
            set_component_result "${component_name}" Failed || true
            component_index=$((component_index + 1))
            continue
        fi
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
            if ! set_component_result "${component_name}" Succeeded; then
                LIVE_PATCHING_COMPONENT_RESULTS_VALID=false
                failed=true
            fi
        elif ! "${component_handler}" "${component_payload}" "${node_json}"; then
            echo "component failed: ${component_name}"
            set_component_result "${component_name}" Failed || LIVE_PATCHING_COMPONENT_RESULTS_VALID=false
            failed=true
        elif ! write_component_checkpoint "${component_name}" "${component_payload}"; then
            echo "failed to persist component state: ${component_name}"
            knead_emit_reconcile_event Failed "checkpoint write failed: ${component_name}" Error
            set_component_result "${component_name}" Failed || LIVE_PATCHING_COMPONENT_RESULTS_VALID=false
            failed=true
        elif ! set_component_result "${component_name}" Succeeded; then
            LIVE_PATCHING_COMPONENT_RESULTS_VALID=false
            failed=true
        fi
        component_index=$((component_index + 1))
    done

    [ "${failed}" = false ]
}

generic_main() {
    local node_json="$1"
    local goal="$2"
    local node_name status payload
    local result=0

    node_name=$(printf '%s' "${node_json}" | jq -er '.metadata.name // empty') || {
        knead_emit_reconcile_event Failed "node name read failed" Error
        return 1
    }
    if ! printf '%s' "${goal}" | grep -Eq '^[0-9a-f]{64}$'; then
        echo "live-patching goal hash must be a 64-character lowercase sha256 digest" >&2
        knead_emit_reconcile_event Failed "invalid goal hash" Error
        return 1
    fi
    status=$(get_node_annotation "${node_json}" "${LIVE_PATCHING_STATUS_ANNOTATION}") || {
        knead_emit_reconcile_event Failed "status annotation read failed" Error
        return 1
    }
    if [ -n "${status}" ] && printf '%s' "${status}" | jq -e --arg goal "${goal}" \
        '.currentHash == $goal and (.components | type == "object") and (.components | all(.code == "Succeeded"))' > /dev/null 2>&1; then
        echo "live-patching goal is already converged, nothing to apply"
        return 0
    fi
    if ! payload=$(read_generic_config "${goal}"); then
        result=1
    else
        apply_components "${payload}" "${node_json}" || result=1
        if ! write_generic_status "${node_name}" "${goal}"; then
            echo "failed to update live-patching status annotation"
            result=1
        fi
    fi
    if [ "${result}" -eq 0 ]; then
        echo "generic live-patching completed successfully"
        knead_emit_reconcile_event Succeeded "goal=${goal}"
    else
        knead_emit_reconcile_event Failed "goal=${goal}" Error
    fi
    return "${result}"
}

main() {
    local version_id node_name node_json goal

    if ! wait_for_kubeconfig; then
        knead_emit_reconcile_event Failed "kubeconfig wait failed" Error
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
        knead_emit_reconcile_event Failed "node name read failed" Error
        return 1
    fi
    node_name=$(printf '%s' "${node_name}" | tr '[:upper:]' '[:lower:]')
    # shellcheck disable=SC2086
    node_json=$($KUBECTL get node "${node_name}" -o json) || {
        knead_emit_reconcile_event Failed "node read failed" Error
        return 1
    }
    goal=$(get_node_annotation "${node_json}" "${LIVE_PATCHING_GOAL_ANNOTATION}") || {
        knead_emit_reconcile_event Failed "goal annotation read failed" Error
        return 1
    }
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
