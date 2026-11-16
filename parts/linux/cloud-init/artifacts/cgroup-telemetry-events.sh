#!/bin/bash

# Publish a cgroup observation, splitting the named object only when necessary.
# Each fragment is checked before any file is published.
publishCgroupTelemetry() {
    local events_dir="$1" event_id="$2" start_time="$3" end_time="$4"
    local task_name="$5" event_level="$6" message="$7" field="$8" label="$9"
    local events event_index event target temporary

    if ! events=$(jq -cn \
        --arg Timestamp "${start_time}" \
        --arg OperationId "${end_time}" \
        --arg Version "1.23" \
        --arg TaskName "${task_name}" \
        --arg EventLevel "${event_level}" \
        --arg Message "${message}" \
        --arg EventPid "0" \
        --arg EventTid "0" \
        --arg id "${event_id}" \
        --arg field "${field}" '
    # These byte limits are multiples of 3, so their Base64 length bounds are exact on jq 1.5.
    def fits: (.Message | @base64 | length) <= 4096 and (tojson | @base64 | length) <= 8192;
    {Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName,
     EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid} as $event |
    if ($event | fits) then [$event] else
        ($Message | fromjson) as $payload |
        ($payload[$field] | to_entries) as $entries |
        ((($entries | length) + 3) / 4 | floor) as $count |
        [range(0; $count) as $index |
            $payload |
            .[$field] = ($entries[$index * 4:($index + 1) * 4] | from_entries) |
            .TelemetryChunk = {id: $id, index: $index, count: $count, encoding: "json"} |
            tojson as $chunk | $event | .Message = $chunk]
    end |
    if length > 0 and all(.[]; fits) then .[]
    else error("cgroup event exceeds guest agent transport limits") end
'); then
        echo "${label} telemetry: failed to bound event payload" >&2
        return 1
    fi

    if [ -z "${events}" ]; then
        echo "${label} telemetry: no events generated" >&2
        return 1
    fi

    mkdir -p "${events_dir}" || return 1
    event_index=0
    while IFS= read -r event; do
        printf -v target '%s/%s%06d.json' "${events_dir%/}" "${event_id}" "${event_index}"
        # WALinuxAgent deletes all entries in events/, including hidden staging files.
        temporary=$(mktemp "${events_dir%/}/../.${label}-event.XXXXXXXX") || return 1
        if ! printf '%s\n' "${event}" > "${temporary}" ||
            ! chmod 0644 "${temporary}" || ! ln "${temporary}" "${target}"; then
            echo "${label} telemetry: failed to publish ${target}" >&2
            rm -f "${temporary}"
            return 1
        fi
        rm -f "${temporary}" || return 1
        event_index=$((event_index + 1))
    done <<< "${events}"
}
