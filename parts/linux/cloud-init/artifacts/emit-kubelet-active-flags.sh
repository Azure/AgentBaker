#!/bin/bash
set -uo pipefail

# This script emits kubelet's effective startup configuration as a structured JSON guest agent
# event so it is queryable per node from Kusto (config/version tracking dashboards).
#
# It reads directly from the kubelet config files on disk:
#   - /etc/default/kubelet (KUBELET_FLAGS string — command-line flags)
#   - /etc/default/kubeletconfig.json (KubeletConfiguration — structured config)
#
# This is instantaneous (no journalctl poll, no retries needed) and produces the same data
# as the active kubelet process uses, since kubelet reads these exact files at startup.

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
KUBELET_DEFAULT_FILE="${KUBELET_DEFAULT_FILE:-/etc/default/kubelet}"
KUBELET_CONFIG_JSON_PATH="${KUBELET_CONFIG_JSON_PATH:-/etc/default/kubeletconfig.json}"

# getKubeletActiveFlagsJSON builds a compact JSON object describing kubelet's effective
# configuration by reading the on-disk config files. The output is a single-line JSON string
# emitted verbatim into GuestAgentGenericLogs.Context1, consumable with parse_json() from Kusto.
getKubeletActiveFlagsJSON() {
    local kubelet_flags="" config_file_json="{}" uses_config_file=false config_path=""

    # Read KUBELET_FLAGS from /etc/default/kubelet
    if [ -f "${KUBELET_DEFAULT_FILE}" ]; then
        kubelet_flags="$(grep '^KUBELET_FLAGS=' "${KUBELET_DEFAULT_FILE}" | sed 's/^KUBELET_FLAGS=//')"
    fi

    # Check if kubelet uses a config file (--config flag in KUBELET_FLAGS or drop-in)
    if [ -f "${KUBELET_CONFIG_JSON_PATH}" ]; then
        uses_config_file=true
        config_path="${KUBELET_CONFIG_JSON_PATH}"
        config_file_json="$(jq -c '.' "${KUBELET_CONFIG_JSON_PATH}" 2>/dev/null || echo '{}')"
    fi

    # Build a { name: value } object from the KUBELET_FLAGS string
    local flags_json="{}"
    if [ -n "${kubelet_flags}" ]; then
        # Parse --key=value pairs from the flags string
        local key value
        for token in ${kubelet_flags}; do
            case "${token}" in
                --*=*)
                    key="${token%%=*}"
                    key="${key#--}"
                    value="${token#*=}"
                    flags_json="$(printf '%s' "${flags_json}" | jq -c --arg k "${key}" --arg v "${value}" '. + {($k): $v}')"
                    ;;
            esac
        done
    fi

    local flag_count
    flag_count="$(printf '%s' "${flags_json}" | jq 'length')"

    # GuestAgentGenericLogs.Context1 is hard-truncated at 3072 bytes (3 KiB) by the Guest Agent /
    # Geneva pipeline. We cap at 3000 to leave ~72 bytes of margin.
    local max_message_bytes=3000
    local payload
    payload="$(jq -cn \
        --argjson uses_config_file "${uses_config_file}" \
        --arg config_path "${config_path}" \
        --argjson flag_count "${flag_count}" \
        --argjson flags "${flags_json}" \
        --argjson config_file "${config_file_json}" \
        '{uses_config_file: $uses_config_file, config_path: $config_path, flag_count: $flag_count, flags: $flags, config_file: $config_file}')"

    local payload_bytes
    payload_bytes="$(printf '%s' "${payload}" | wc -c | tr -d '[:space:]')"
    if [ "${payload_bytes}" -gt "${max_message_bytes}" ]; then
        echo "kubelet active settings payload exceeded ${max_message_bytes} bytes (${payload_bytes}), dropping config_file content" >&2
        payload="$(jq -cn \
            --argjson uses_config_file "${uses_config_file}" \
            --arg config_path "${config_path}" \
            --argjson flag_count "${flag_count}" \
            --argjson flags "${flags_json}" \
            '{uses_config_file: $uses_config_file, config_path: $config_path, flag_count: $flag_count, flags: $flags, config_file: "truncated:exceeded_size_cap"}')"
    fi
    echo "${payload}"
}

# emitKubeletActiveFlagsEvent writes a guest agent event whose Message is the kubelet config JSON,
# so it lands verbatim as pure JSON in GuestAgentGenericLogs.Context1, ready for parse_json().
emitKubeletActiveFlagsEvent() {
    local task="AKS.CSE.ensureKubelet.kubeletActiveFlags"
    local message now eventsFileName
    message="$(getKubeletActiveFlagsJSON)"
    now="$(date +"%F %T.%3N")"
    eventsFileName="$(date +%s%3N)"
    mkdir -p "${EVENTS_LOGGING_DIR}"
    jq -n \
        --arg Timestamp   "${now}" \
        --arg OperationId "${now}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "Informational" \
        --arg Message     "${message}" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}' \
        > "${EVENTS_LOGGING_DIR}${eventsFileName}.json"
}

# Only run when executed directly (not when sourced by tests).
if [ -z "${EMIT_KUBELET_ACTIVE_FLAGS_SOURCE_ONLY:-}" ]; then
    emitKubeletActiveFlagsEvent
fi
