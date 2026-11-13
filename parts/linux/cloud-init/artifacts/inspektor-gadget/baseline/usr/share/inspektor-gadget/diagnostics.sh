#!/bin/bash

ig_diagnostics_client() {
    timeout -k 1s 5s gadgetctl --remote-address \
        "unix://${IG_DIAGNOSTICS_RUNTIME_DIR:-/run/ig-diagnostics}/ig.socket" "$@"
}

ig_diagnostics_image() {
    local gadget="$1" image
    image="$(grep -E "/${gadget}:v?[0-9]+\.[0-9]+\.[0-9]+$" \
        "${IG_DIAGNOSTICS_TRACKING_FILE:-/var/lib/ig/imported-gadgets.txt}")" || {
        echo "[ig-diagnostics] No imported image for ${gadget}" >&2
        return 1
    }
    if [ "$(printf '%s\n' "${image}" | wc -l)" -ne 1 ]; then
        echo "[ig-diagnostics] Ambiguous imported image for ${gadget}" >&2
        return 1
    fi
    printf '%s' "${image}"
}

ig_diagnostics_collector() {
    local name="$1" image="$2" sort="$3" state id attempt
    if state="$(ig_diagnostics_client show "${name}" 2>&1)"; then
        ig_diagnostics_client delete "${name}" >&2 || return 1
    elif ! grep -Fq "instance \"${name}\" not found" <<< "${state}"; then
        echo "[ig-diagnostics] Cannot inspect ${name}: ${state}" >&2
        return 1
    fi

    # Recreate only our named instances so persisted definitions cannot override
    # the current image/parameters after a daemon restart or package change.
    ig_diagnostics_client run "${image}" --detach --name "${name}" \
        --pull never --interval 5s --count 0 --sort "${sort}" --max-entries 15 >&2 || return 1
    for attempt in 1 2 3 4 5 6; do
        state="$(ig_diagnostics_client show "${name}")" || return 1
        id="$(sed -n 's/^ID: \([0-9a-f]\{32\}\)$/\1/p' <<< "${state}")"
        if [ -n "${id}" ] && grep -Eq '^ *-? *Status: Running$' <<< "${state}"; then
            printf '%s' "${id}"
            return 0
        fi
        echo "[ig-diagnostics] Waiting for ${name} (attempt ${attempt}/6)" >&2
        sleep 1
    done
    echo "[ig-diagnostics] Collector ${name} is not running: ${state}" >&2
    return 1
}

ig_diagnostics_start() (
    set -uo pipefail
    umask 077
    local runtime_dir="${IG_DIAGNOSTICS_RUNTIME_DIR:-/run/ig-diagnostics}"
    local ready=false attempt process_image throttle_image cpu memory throttle started boot_id
    rm -f "${runtime_dir}/collectors.json" || return 1
    for attempt in 1 2 3 4 5 6; do
        if ig_diagnostics_client list >/dev/null; then
            ready=true
            break
        fi
        echo "[ig-diagnostics] Waiting for daemon (attempt ${attempt}/6)" >&2
        sleep 1
    done
    if [ "${ready}" != true ]; then
        echo "[ig-diagnostics] Daemon did not become ready" >&2
        return 1
    fi

    process_image="$(ig_diagnostics_image top_process)" || return 1
    throttle_image="$(ig_diagnostics_image top_cpu_throttle)" || return 1
    started="$(date +%s)" || return 1
    boot_id="$(cat /proc/sys/kernel/random/boot_id)" || return 1
    cpu="$(ig_diagnostics_collector aks-npd-cpu-v1 "${process_image}" -cpuRelative)" || return 1
    memory="$(ig_diagnostics_collector aks-npd-memory-v1 "${process_image}" -memRelative)" || return 1
    throttle="$(ig_diagnostics_collector aks-npd-throttle-v1 "${throttle_image}" -throttleRatio)" || return 1
    local temporary
    temporary="$(mktemp "${runtime_dir}/collectors.XXXXXX")" || return 1
    trap 'rm -f "${temporary}"' EXIT
    jq -n --arg cpu "${cpu}" --arg memory "${memory}" --arg throttle "${throttle}" \
        --arg boot "${boot_id}" --argjson started "${started}" \
        '{version:1, bootId:$boot, startedAt:$started,
          collectors:{cpu:$cpu, memory:$memory, throttle:$throttle}}' > "${temporary}" || return 1
    # Running precedes asynchronous gadget initialization. Require usable data
    # against the private candidate manifest before NPD can see readiness.
    if ! timeout -k 1s 35s python3 /etc/node-problem-detector.d/plugin/ig_report_reader.py \
        --check-startup "${temporary}"; then
        echo "[ig-diagnostics] Collectors did not produce usable startup reports" >&2
        return 1
    fi
    mv -f "${temporary}" "${runtime_dir}/collectors.json" || return 1
    echo "[ig-diagnostics] Continuous collectors started"
)

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    ig_diagnostics_start
fi
