#!/usr/bin/env bash

set -o nounset
set -e

# Runs the generic node-side component reconciliation loop. Component handlers own
# their payload schema and node mutations; this loop owns annotations, ConfigMap
# validation, dispatch, local component state, and final success reporting.

: "${KUBECONFIG:=/var/lib/kubelet/kubeconfig}"
: "${KUBECTL:=/opt/bin/kubectl --kubeconfig ${KUBECONFIG}}"
: "${KNEAD_COMPONENT_CONFIG_NAMESPACE:=kube-system}"
: "${KNEAD_COMPONENT_CONFIGMAP:=live-patching-config}"
: "${KNEAD_COMPONENT_CONFIG_KEY_JSONPATH:=live-patching-config\.json}"
: "${KNEAD_COMPONENT_GOAL_ANNOTATION:=kubernetes.azure.com/live-patching-config-goal-hash}"
: "${KNEAD_COMPONENT_STATUS_ANNOTATION:=kubernetes.azure.com/live-patching-status}"
: "${KNEAD_COMPONENT_STATE_FILE:=/var/lib/aks/live-patching/current.json}"
: "${KNEAD_KUBECONFIG_WAIT_TIMEOUT_SECONDS:=600}"

KNEAD_COMPONENT_RESULTS='{}'
KNEAD_COMPONENT_RESULTS_VALID=true

# Waits for kubelet credentials so kubectl can read Node and ConfigMap state.
knead_wait_for_kubeconfig() {
    local wait_started_at="${SECONDS}"

    while [ ! -f "${KUBECONFIG}" ]; do
        if [ $((SECONDS - wait_started_at)) -ge "${KNEAD_KUBECONFIG_WAIT_TIMEOUT_SECONDS}" ]; then
            echo "timed out waiting for kubelet kubeconfig after ${KNEAD_KUBECONFIG_WAIT_TIMEOUT_SECONDS}s" >&2
            return 1
        fi
        echo "waiting for kubelet kubeconfig"
        sleep 3
    done
}

# Reads this Node using the lowercase hostname expected from cloud provider registration.
knead_read_node() {
    local node_name

    node_name="$(hostname)"
    if [ -z "${node_name}" ]; then
        echo "cannot get node name" >&2
        return 1
    fi

    node_name="$(printf '%s' "${node_name}" | tr '[:upper:]' '[:lower:]')"

    # shellcheck disable=SC2086
    $KUBECTL get node "${node_name}" -o json
}

# Returns the value of the given annotation.
knead_get_node_annotation() {
    local node_json="$1"
    local annotation="$2"

    printf '%s' "${node_json}" | jq -r --arg annotation "${annotation}" '.metadata.annotations[$annotation] // empty'
}

# Reads and validates the generic ConfigMap contract before any component handler runs.
#
# Knead validates the component envelope. Each handler owns its decoded
# nodeConfig schema. The goal must be the bare sha256 digest of the exact
# ConfigMap value read by this node.
knead_read_configmap() {
    local goal="$1"
    local payload
    local payload_hash

    # shellcheck disable=SC2086
    if ! payload="$($KUBECTL get cm -n "${KNEAD_COMPONENT_CONFIG_NAMESPACE}" "${KNEAD_COMPONENT_CONFIGMAP}" -o "jsonpath={.data.${KNEAD_COMPONENT_CONFIG_KEY_JSONPATH}}")"; then
        echo "failed to read live-patching-config ConfigMap" >&2
        return 1
    fi

    if ! printf '%s' "${payload}" | jq -e '
        (.components | type == "array") and
        (.components | all(
            (.name | type == "string") and
            (.name | length > 0) and
            (.nodeConfig | type == "string")
        )) and
        ([.components[].name] | length) == ([.components[].name] | unique | length)
    ' > /dev/null; then
        echo "live-patching-config payload has invalid envelope" >&2
        return 1
    fi

    if ! printf '%s' "${goal}" | grep -Eq '^[0-9a-f]{64}$'; then
        echo "live-patching goal hash must be a 64-character lowercase sha256 digest" >&2
        return 1
    fi

    payload_hash="$(printf '%s' "${payload}" | sha256sum | awk '{print $1}')"
    if [ "${payload_hash}" != "${goal}" ]; then
        echo "live-patching goal hash does not match ConfigMap payload: goal=${goal}, payload=${payload_hash}" >&2
        return 1
    fi

    printf '%s' "${payload}"
}

# Returns success when this exact component config was applied successfully but
# the overall Node status did not converge, such as when the annotation update or
# a sibling component failed. This local checkpoint prevents repeating successful
# work on the next retry. Missing or malformed state is treated as not current.
knead_component_is_current() {
    local component="$1"
    local component_payload="$2"
    local node_json="$3"
    local component_comparator="$4"
    local current_payload

    if [ ! -f "${KNEAD_COMPONENT_STATE_FILE}" ]; then
        return 1
    fi

    if ! current_payload="$(jq -er --arg component "${component}" \
        '.components | map(select(.name == $component)) | last | .nodeConfig' \
        "${KNEAD_COMPONENT_STATE_FILE}" 2> /dev/null)"; then
        return 1
    fi

    "${component_comparator}" "${component_payload}" "${current_payload}" "${node_json}"
}

# Records one successfully applied component without changing sibling
# state. Missing or malformed state is rebuilt from an empty component list.
knead_write_component_state() {
    local component="$1"
    local component_payload="$2"
    local state='{"components":[]}'
    local state_dir
    local state_tmp
    local updated_at

    state_dir="$(dirname "${KNEAD_COMPONENT_STATE_FILE}")"
    state_tmp="${KNEAD_COMPONENT_STATE_FILE}.tmp"
    updated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

    if ! mkdir -p "${state_dir}"; then
        echo "failed to create knead-component state directory"
        return 1
    fi
    if [ -f "${KNEAD_COMPONENT_STATE_FILE}" ]; then
        if ! state="$(jq -c 'select(.components | type == "array") | {components: .components}' "${KNEAD_COMPONENT_STATE_FILE}" 2> /dev/null)" || [ -z "${state}" ]; then
            state='{"components":[]}'
        fi
    fi
    if ! printf '%s' "${state}" | jq \
        --arg component "${component}" \
        --arg componentPayload "${component_payload}" \
        --arg updatedAt "${updated_at}" \
        '.updatedAt = $updatedAt | .components = ([.components[] | select(.name != $component)] + [{name: $component, nodeConfig: $componentPayload}])' \
        > "${state_tmp}"; then
        echo "failed to render knead-component state"
        return 1
    fi
    if ! mv "${state_tmp}" "${KNEAD_COMPONENT_STATE_FILE}"; then
        echo "failed to write knead-component state"
        return 1
    fi
}

# Adds a component result to the status collection.
knead_set_component_result() {
    local component="$1"
    local code="$2"
    local updated_results

    if ! updated_results="$(printf '%s' "${KNEAD_COMPONENT_RESULTS}" | jq -ce \
        --arg component "${component}" \
        --arg code "${code}" \
        '.[$component] = {code: $code}')" || [ -z "${updated_results}" ]; then
        echo "failed to record component result: ${component}=${code}"
        KNEAD_COMPONENT_RESULTS_VALID=false
        return 1
    fi

    KNEAD_COMPONENT_RESULTS="${updated_results}"
}

# Dispatches every known component and returns failure only after all have been attempted.
#
# This is the main error boundary for node disruption. A broken component should
# not stop unrelated handlers from making progress, so this function records
# failures and continues dispatching. Unknown components are skipped so older
# VHDs can still converge when a newer component appears in the shared config.
#
# Keep component payload parsing behind each handler. The generic loop should know
# only which handler owns a component name, not the shape of that component's JSON.
knead_apply_components() {
    local payload="$1"
    local node_json="$2"
    local failed_components=""
    local component
    local component_count
    local component_comparator
    local component_handler
    local component_index=0
    local component_payload
    local infrastructure_failed=false

    KNEAD_COMPONENT_RESULTS='{}'
    KNEAD_COMPONENT_RESULTS_VALID=true

    if ! component_count="$(printf '%s' "${payload}" | jq -er '.components | length')"; then
        echo "failed to read component count"
        KNEAD_COMPONENT_RESULTS_VALID=false
        return 1
    fi
    while [ "${component_index}" -lt "${component_count}" ]; do
        if ! component="$(printf '%s' "${payload}" | jq -er --argjson index "${component_index}" '.components[$index].name')"; then
            echo "failed to read component name at index: ${component_index}"
            infrastructure_failed=true
            KNEAD_COMPONENT_RESULTS_VALID=false
            component_index=$((component_index + 1))
            continue
        fi
        if ! component_payload="$(printf '%s' "${payload}" | jq -er --argjson index "${component_index}" '.components[$index].nodeConfig')"; then
            echo "failed to read component payload: ${component}"
            failed_components="${failed_components} ${component}"
            infrastructure_failed=true
            KNEAD_COMPONENT_RESULTS_VALID=false
            if ! knead_set_component_result "${component}" "Failed"; then
                infrastructure_failed=true
            fi
            component_index=$((component_index + 1))
            continue
        fi
        echo "applying component: ${component}"

        # Select the handler for this component; unsupported components are ignored.
        case "${component}" in
            securityPatch)
                component_comparator=securityPatchIsCurrent
                component_handler=updateSecurityPatch
                ;;
            localDNS)
                component_comparator=localDNSIsCurrent
                component_handler=updateLocalDNS
                ;;
            *)
                echo "unsupported component: ${component}"
                component_index=$((component_index + 1))
                continue
                ;;
        esac

        # Skip previously applied work; otherwise apply the component and checkpoint success.
        if knead_component_is_current "${component}" "${component_payload}" "${node_json}" "${component_comparator}"; then
            echo "component is already current: ${component}"
            if ! knead_set_component_result "${component}" "Succeeded"; then
                infrastructure_failed=true
            fi
        elif ! "${component_handler}" "${component_payload}" "${node_json}"; then
            failed_components="${failed_components} ${component}"
            echo "component failed: ${component}"
            if ! knead_set_component_result "${component}" "Failed"; then
                infrastructure_failed=true
            fi
        elif ! knead_write_component_state "${component}" "${component_payload}"; then
            failed_components="${failed_components} ${component}"
            echo "failed to persist component state: ${component}"
            if ! knead_set_component_result "${component}" "Failed"; then
                infrastructure_failed=true
            fi
        elif ! knead_set_component_result "${component}" "Succeeded"; then
            infrastructure_failed=true
        fi
        component_index=$((component_index + 1))
    done

    if [ -n "${failed_components}" ]; then
        echo "failed components:${failed_components}"
    fi
    if [ "${infrastructure_failed}" = true ]; then
        echo "component dispatch encountered internal failures"
    fi
    [ -z "${failed_components}" ] && [ "${infrastructure_failed}" = false ]
}

# Records the processed hash and per-component results in the node status annotation.
localDNSIsCurrent() {
    local desired_payload="$1"
    local current_payload="$2"

    [ "${desired_payload}" = "${current_payload}" ]
}

updateLocalDNS() {
    local component_payload="${1:-}"
    local outcome

    if [ ! -x /opt/azure/containers/aks-node-controller ]; then
        echo "aks-node-controller binary is required for localDNS live patching"
        return 1
    fi

    if ! outcome="$(printf '%s' "${component_payload}" | /opt/azure/containers/aks-node-controller apply-localdns-config \
        --config-file - \
        --output /opt/azure/containers/localdns/livepatched.localdns.corefile)"; then
        echo "localDNS config apply failed"
        return 1
    fi
    printf '%s
' "${outcome}"

    case "$(printf '%s
' "${outcome}" | tail -n 1)" in
        applied)
            if ! systemctl restart localdns.service; then
                echo "failed to restart localdns.service"
                return 1
            fi
            echo "localDNS update completed successfully"
            ;;
        alreadyCurrent)
            echo "localDNS is already current"
            ;;
        notFound)
            echo "localDNS LPS config is not available"
            ;;
        noCorefileData)
            echo "localDNS LPS config has no node-applicable payload"
            return 1
            ;;
        *)
            echo "unexpected localDNS apply outcome: ${outcome}"
            return 1
            ;;
    esac
}

knead_write_status() {
    local node_name="$1"
    local goal="$2"
    local status

    if [ "${KNEAD_COMPONENT_RESULTS_VALID}" != true ]; then
        echo "refusing to write incomplete live-patching status"
        return 1
    fi
    if ! status="$(printf '%s' "${KNEAD_COMPONENT_RESULTS}" | jq -c --arg currentHash "${goal}" '{currentHash: $currentHash, components: .}')" || [ -z "${status}" ]; then
        echo "failed to render live-patching status annotation"
        return 1
    fi

    # shellcheck disable=SC2086
    $KUBECTL annotate --overwrite node "${node_name}" "${KNEAD_COMPONENT_STATUS_ANNOTATION}=${status}"
}

knead_main() {
    local node_name
    local node_json
    local goal
    local status
    local payload
    local result=0

    if ! knead_wait_for_kubeconfig; then
        return 1
    fi
    if ! node_json="$(knead_read_node)"; then
        echo "failed to read node"
        return 1
    fi
    if ! node_name="$(printf '%s' "${node_json}" | jq -er '.metadata.name')"; then
        echo "failed to read node name"
        return 1
    fi
    if ! goal="$(knead_get_node_annotation "${node_json}" "${KNEAD_COMPONENT_GOAL_ANNOTATION}")"; then
        echo "failed to read live-patching goal annotation"
        return 1
    fi
    if [ -z "${goal}" ]; then
        echo "live-patching goal is not set, skip knead-component"
        return 0
    fi
    echo "live-patching goal is: ${goal}"

    if ! status="$(knead_get_node_annotation "${node_json}" "${KNEAD_COMPONENT_STATUS_ANNOTATION}")"; then
        echo "failed to read live-patching status annotation"
        return 1
    fi
    # Skip reconciliation only when this goal was processed and every component succeeded.
    if [ -n "${status}" ] && printf '%s' "${status}" | jq -e --arg goal "${goal}" \
        '.currentHash == $goal and (.components | type == "object") and (.components | all(.code == "Succeeded"))' \
        > /dev/null 2>&1; then
        echo "live-patching goal is already converged, nothing to apply"
        return 0
    fi

    # Stop before dispatch when generic inputs are missing or unsafe. Once dispatch
    # starts, knead_apply_components owns the continue-after-component-failure
    # behavior so one broken handler does not prevent other handlers from running.
    if ! payload="$(knead_read_configmap "${goal}")"; then
        result=1
    else
        if ! knead_apply_components "${payload}" "${node_json}"; then
            result=1
        fi

        if ! knead_write_status "${node_name}" "${goal}"; then
            echo "failed to update live-patching status annotation"
            result=1
        fi
    fi

    if [ "${result}" -eq 0 ]; then
        echo "knead-component completed successfully"
    fi
    return "${result}"
}

${__SOURCED__:+return}
# --------------------------------------- Main Execution starts here --------------------------------------------------

# shellcheck disable=SC1091
source /opt/azure/containers/security-update.sh

knead_main "$@"
