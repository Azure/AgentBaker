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
: "${KNEAD_EVENTS_LOGGING_DIR:=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events}"

KNEAD_COMPONENT_RESULTS='{}'
KNEAD_COMPONENT_RESULTS_VALID=true
KNEAD_RESULT_FILE=""
KNEAD_HANDLER_CURRENT=""
KNEAD_HANDLER_MESSAGE=""

# Writes a Guest Agent event using the established logs_to_events schema. This
# remains available to component handlers sourced into this shell process.
knead_emit_event() {
    local task="$1"
    local message="$2"
    local level="${3:-Informational}"
    local events_file_name
    events_file_name="$(date +%s%N)${BASHPID}"
    local timestamp
    timestamp="$(date +"%F %T.%3N")"
    local event_json
    event_json="$(jq -n \
        --arg Timestamp   "${timestamp}" \
        --arg OperationId "${timestamp}" \
        --arg Version "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "${level}" \
        --arg Message     "${message}" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}')"

    mkdir -p "${KNEAD_EVENTS_LOGGING_DIR}"
    printf '%s\n' "${event_json}" > "${KNEAD_EVENTS_LOGGING_DIR%/}/${events_file_name}.json"
}

# Max length of a reported current/message value. The status is a Node annotation,
# so keep each component's result small; values are flattened to one line and cut.
: "${KNEAD_REPORT_MAX_LENGTH:=256}"

# Handlers call knead_report_current and knead_report_message to return values to
# the loop. The loop points KNEAD_RESULT_FILE at a fresh file for each handler run
# and reads it back afterwards. A file (not a variable) is used because handlers
# may run in a subshell. Both are no-ops outside a loop-managed handler run, so
# shared helpers stay safe on other paths. For current the last report wins; for
# message the first report wins, because nested helpers report the specific
# root cause before their callers report a generic wrapper reason.
knead_report_field() {
    local field="$1"
    local value="$2"
    local keep_first="${3:-false}"
    local updated

    if [ -z "${KNEAD_RESULT_FILE:-}" ] || [ ! -f "${KNEAD_RESULT_FILE}" ]; then
        return 0
    fi
    # Compare the printed value rather than relying on "jq -e" exit codes, which
    # differ across jq versions when the input is empty.
    if [ "${keep_first}" = true ] && [ -n "$(jq -r --arg field "${field}" '.[$field] // empty | strings' "${KNEAD_RESULT_FILE}" 2> /dev/null)" ]; then
        return 0
    fi
    value="$(printf '%s' "${value}" | tr '\r\n\t' '   ' | cut -c "1-${KNEAD_REPORT_MAX_LENGTH}")"
    if ! updated="$(jq -c --arg field "${field}" --arg value "${value}" '.[$field] = $value' "${KNEAD_RESULT_FILE}" 2> /dev/null)" || [ -z "${updated}" ]; then
        updated="$(jq -nc --arg field "${field}" --arg value "${value}" '{($field): $value}')" || return 0
    fi
    printf '%s' "${updated}" > "${KNEAD_RESULT_FILE}" || true
    return 0
}

# Reports what this component has applied, recorded as "current" when it succeeds.
knead_report_current() {
    knead_report_field current "$1"
}

# Reports a short reason, recorded as "message" when the component fails. The
# first reported reason is kept.
knead_report_message() {
    knead_report_field message "$1" true
}

# Prints one field from a handler result file, or nothing when it is absent.
knead_read_result_field() {
    local result_file="$1"
    local field="$2"

    jq -r --arg field "${field}" '.[$field] // empty | strings' "${result_file}" 2> /dev/null || true
}

# Component scripts should follow this pattern with a uniquely named
# knead_emit_<component>_event wrapper that delegates transport to knead_emit_event.
# This wrapper emits events for the generic live-patching reconciliation loop.
knead_emit_reconcile_event() {
    local outcome="$1"
    local reason="$2"
    local level="${3:-Informational}"

    knead_emit_event "AKS.LivePatching.reconcile" "${outcome}: ${reason}" "${level}" || true
}

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

    printf '%s' "${node_json}" | jq -r --arg annotation "${annotation}" '(.metadata.annotations // {})[$annotation] // empty'
}

# Reads and validates the generic ConfigMap contract before any component handler runs.
#
# Knead validates the component envelope. Each handler owns its decoded
# nodeConfig schema. The goal must be the bare sha256 digest of the exact
# ConfigMap value read by this node.
knead_read_configmap() {
    local goal="$1"
    local payload
    local payload_with_sentinel
    local payload_hash

    # Preserve trailing newlines until the exact ConfigMap bytes have been hashed.
    # shellcheck disable=SC2086
    if ! payload_with_sentinel="$($KUBECTL get cm -n "${KNEAD_COMPONENT_CONFIG_NAMESPACE}" "${KNEAD_COMPONENT_CONFIGMAP}" -o "jsonpath={.data.${KNEAD_COMPONENT_CONFIG_KEY_JSONPATH}}" && printf '.')"; then
        echo "failed to read live-patching-config ConfigMap" >&2
        return 1
    fi
    payload="${payload_with_sentinel%.}"

    if ! printf '%s' "${payload}" | jq -se '
        length == 1 and (.[0] |
        (type == "object") and
        (.components | type == "array") and
        (.components | all(
            (.name | type == "string") and
            (.name | length > 0) and
            (.name | test("^[A-Za-z][A-Za-z0-9_-]*\\z")) and
            (.nodeConfig | type == "string")
        )) and
        ([.components[].name] | length) == ([.components[].name] | unique | length))
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

# Prints the "current" value saved with a component's checkpoint, or nothing.
knead_component_state_current() {
    local component="$1"

    if [ ! -f "${KNEAD_COMPONENT_STATE_FILE}" ]; then
        return 0
    fi
    jq -r --arg component "${component}" \
        '.components | map(select(.name == $component)) | last | .current // empty | strings' \
        "${KNEAD_COMPONENT_STATE_FILE}" 2> /dev/null || true
}

# Records one successfully applied component without changing sibling
# state, together with the "current" value its handler reported (if any) so a
# later run that skips the handler can still report it. Missing or malformed
# state is rebuilt from an empty component list.
knead_write_component_state() {
    local component="$1"
    local component_payload="$2"
    local component_current="${3:-}"
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
        if ! state="$(jq -c 'select(.components | type == "array") | {components: [.components[] | select(type == "object") | select((.name | type == "string") and (.nodeConfig | type == "string")) | {name, nodeConfig} + (if (.current | type) == "string" then {current} else {} end)]}' "${KNEAD_COMPONENT_STATE_FILE}" 2> /dev/null)" || [ -z "${state}" ]; then
            state='{"components":[]}'
        fi
    fi
    if ! printf '%s' "${state}" | jq \
        --arg component "${component}" \
        --arg componentPayload "${component_payload}" \
        --arg componentCurrent "${component_current}" \
        --arg updatedAt "${updated_at}" \
        '.updatedAt = $updatedAt | .components = ([.components[] | select(.name != $component)] + [{name: $component, nodeConfig: $componentPayload} + (if $componentCurrent == "" then {} else {current: $componentCurrent} end)])' \
        > "${state_tmp}"; then
        echo "failed to render knead-component state"
        return 1
    fi
    if ! mv "${state_tmp}" "${KNEAD_COMPONENT_STATE_FILE}"; then
        echo "failed to write knead-component state"
        return 1
    fi
}

# Adds a component result to the status collection. The optional third argument
# is "current" (recorded only for Succeeded) or "message" (recorded only for
# any other code); an empty value is omitted.
knead_set_component_result() {
    local component="$1"
    local code="$2"
    local detail="${3:-}"
    local updated_results

    if ! updated_results="$(printf '%s' "${KNEAD_COMPONENT_RESULTS}" | jq -ce \
        --arg component "${component}" \
        --arg code "${code}" \
        --arg detail "${detail}" \
        '.[$component] = ({code: $code} + (if $detail == "" then {} elif $code == "Succeeded" then {current: $detail} else {message: $detail} end))')" || [ -z "${updated_results}" ]; then
        echo "failed to record component result: ${component}=${code}"
        KNEAD_COMPONENT_RESULTS_VALID=false
        return 1
    fi

    KNEAD_COMPONENT_RESULTS="${updated_results}"
}

# Runs one component handler with a fresh result file and saves what it reported
# in KNEAD_HANDLER_CURRENT and KNEAD_HANDLER_MESSAGE. Returns the handler status.
# The result file lives only for this call, so a later run can never read it.
knead_run_component_handler() {
    local component_handler="$1"
    local component_payload="$2"
    local node_json="$3"
    local result_file
    local handler_status=0

    KNEAD_HANDLER_CURRENT=""
    KNEAD_HANDLER_MESSAGE=""
    if ! result_file="$(mktemp "${TMPDIR:-/tmp}/knead-result.XXXXXX")" || ! printf '{}' > "${result_file}"; then
        echo "failed to create handler result file; current/message will not be reported"
        [ -z "${result_file}" ] || rm -f "${result_file}"
        result_file=""
    fi

    KNEAD_RESULT_FILE="${result_file}"
    "${component_handler}" "${component_payload}" "${node_json}" || handler_status=$?
    KNEAD_RESULT_FILE=""

    if [ -n "${result_file}" ]; then
        KNEAD_HANDLER_CURRENT="$(knead_read_result_field "${result_file}" current)"
        KNEAD_HANDLER_MESSAGE="$(knead_read_result_field "${result_file}" message)"
        rm -f "${result_file}"
    fi
    return "${handler_status}"
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
            npdConfig)
                component_comparator=npdConfigsIsCurrent
                component_handler=updateNPDConfigs
                ;;
            npdBinary)
                component_comparator=npdBinaryIsCurrent
                component_handler=updateNPDBinary
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
            # The handler is skipped, so replay the "current" saved with its checkpoint.
            if ! knead_set_component_result "${component}" "Succeeded" "$(knead_component_state_current "${component}")"; then
                infrastructure_failed=true
            fi
        elif ! knead_run_component_handler "${component_handler}" "${component_payload}" "${node_json}"; then
            failed_components="${failed_components} ${component}"
            echo "component failed: ${component}"
            if ! knead_set_component_result "${component}" "Failed" "${KNEAD_HANDLER_MESSAGE}"; then
                infrastructure_failed=true
            fi
        elif ! knead_write_component_state "${component}" "${component_payload}" "${KNEAD_HANDLER_CURRENT}"; then
            failed_components="${failed_components} ${component}"
            echo "failed to persist component state: ${component}"
            if ! knead_set_component_result "${component}" "Failed"; then
                infrastructure_failed=true
            fi
        elif ! knead_set_component_result "${component}" "Succeeded" "${KNEAD_HANDLER_CURRENT}"; then
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
        knead_emit_reconcile_event "Failed" "kubeconfig wait failed" "Error"
        return 1
    fi
    if ! node_json="$(knead_read_node)"; then
        echo "failed to read node"
        knead_emit_reconcile_event "Failed" "node read failed" "Error"
        return 1
    fi
    if ! node_name="$(printf '%s' "${node_json}" | jq -er '.metadata.name')"; then
        echo "failed to read node name"
        knead_emit_reconcile_event "Failed" "node name read failed" "Error"
        return 1
    fi
    if ! goal="$(knead_get_node_annotation "${node_json}" "${KNEAD_COMPONENT_GOAL_ANNOTATION}")"; then
        echo "failed to read live-patching goal annotation"
        knead_emit_reconcile_event "Failed" "goal annotation read failed" "Error"
        return 1
    fi
    if [ -z "${goal}" ]; then
        echo "live-patching goal is not set, skip knead-component"
        return 0
    fi
    echo "live-patching goal is: ${goal}"

    if ! status="$(knead_get_node_annotation "${node_json}" "${KNEAD_COMPONENT_STATUS_ANNOTATION}")"; then
        echo "failed to read live-patching status annotation"
        knead_emit_reconcile_event "Failed" "status annotation read failed, goal=${goal}" "Error"
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
        knead_emit_reconcile_event "Succeeded" "goal=${goal}"
    else
        knead_emit_reconcile_event "Failed" "goal=${goal}" "Error"
    fi
    return "${result}"
}

${__SOURCED__:+return}
# --------------------------------------- Main Execution starts here --------------------------------------------------

# shellcheck disable=SC1091
source /opt/azure/containers/security-update.sh
# shellcheck disable=SC1091
source /opt/azure/containers/npd-update.sh

knead_main "$@"
