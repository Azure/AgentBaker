#!/bin/bash

# shellcheck source=./parts/linux/cloud-init/artifacts/inspektor-gadget/baseline/usr/share/inspektor-gadget/diagnostics.sh
source "$(dirname "${BASH_SOURCE[0]}")/diagnostics.sh" || exit 1

ig_diagnostics_recovery_now() {
    python3 -c 'import time; print(int(time.monotonic()))'
}

ig_diagnostics_recovery_state() {
    local snapshot state changed
    snapshot="$(timeout -k 1s 5s systemctl show -p ActiveState -p StateChangeTimestampMonotonic ig-diagnostics.service)" || {
        echo "[ig-diagnostics] Cannot determine service state; not recovering" >&2
        return 1
    }
    state="$(sed -n 's/^ActiveState=//p' <<< "${snapshot}")"
    changed="$(sed -n 's/^StateChangeTimestampMonotonic=//p' <<< "${snapshot}")"
    case "${changed}" in
        ''|*[!0-9]*|0[0-9]*)
            echo "[ig-diagnostics] Invalid service state timestamp; not recovering" >&2
            return 1
            ;;
    esac
    if [ "${#changed}" -gt 18 ]; then
        echo "[ig-diagnostics] Invalid service state timestamp; not recovering" >&2
        return 1
    fi
    printf '%s %s\n' "${state}" "${changed}"
}

ig_diagnostics_recovery_save_budget() (
    local path="$1" attempts="$2" since="$3" temporary
    temporary="$(mktemp "${path}.XXXXXX")" || return 1
    trap 'rm -f "${temporary}"' EXIT
    if ! printf '%s %s\n' "${attempts}" "${since}" > "${temporary}" ||
       ! mv -f "${temporary}" "${path}"; then
        echo "[ig-diagnostics] Cannot save recovery budget" >&2
        return 1
    fi
)

# Success requires positive failure evidence, not merely an unavailable daemon.
ig_diagnostics_recovery_collectors_failed() {
    local name state result failed=false
    for name in aks-npd-cpu-v1 aks-npd-memory-v1 aks-npd-throttle-v1; do
        if state="$(ig_diagnostics_client show "${name}" 2>&1)"; then
            :
        else
            result=$?
            if [ "${result}" -ne 124 ] && [ "${result}" -ne 137 ] &&
               grep -Fq "instance \"${name}\" not found" <<< "${state}"; then
                failed=true
                continue
            fi
            echo "[ig-diagnostics] Cannot inspect ${name}: ${state}" >&2
            return 2
        fi
        if ! grep -Eq '^ID: [0-9a-f]{32}$' <<< "${state}"; then
            echo "[ig-diagnostics] Unrecognized collector identity for ${name}" >&2
            return 2
        fi
        if grep -Eq '^ *-? *Status: Error$' <<< "${state}"; then
            failed=true
        elif ! grep -Eq '^ *-? *Status: Running$' <<< "${state}"; then
            echo "[ig-diagnostics] Collector ${name} is transitioning or has unknown state" >&2
            return 2
        fi
    done
    [ "${failed}" = true ]
}

ig_diagnostics_recover() (
    set -uo pipefail
    umask 077
    local config="${IG_DIAGNOSTICS_CONFIG_DIR:-/etc/ig.d}"
    local runtime="${IG_DIAGNOSTICS_RUNTIME_DIR:-/run/ig-diagnostics}"
    local directory="${IG_DIAGNOSTICS_RECOVERY_DIR:-/run/ig-diagnostics-recovery}"
    local reader="${IG_DIAGNOSTICS_READER:-/etc/node-problem-detector.d/plugin/ig_report_reader.py}"
    local state current attempts=0 since=0 extra="" now changed result action=start dependency
    if [ ! -f "${config}/skip_vhd_ig" ] ||
       [ ! -f "${config}/diagnostics-version" ] ||
       [ "$(cat "${config}/diagnostics-version")" != 1 ] ||
       [ ! -f "${SYS_DIR:-/sys}/fs/cgroup/cgroup.controllers" ]; then
        echo "[ig-diagnostics] Continuous diagnostics is unsupported; not recovering"
        return 0
    fi
    for dependency in python3 jq ig gadgetctl flock; do
        if ! command -v "${dependency}" >/dev/null; then
            echo "[ig-diagnostics] Recovery requires ${dependency}" >&2
            return 1
        fi
    done
    if [ ! -r "${reader}" ]; then
        echo "[ig-diagnostics] Recovery requires the packaged report reader" >&2
        return 1
    fi
    mkdir -p "${directory}" || return 1
    exec 9>"${directory}/lock" || return 1
    if flock -n 9; then
        :
    else
        result=$?
        if [ "${result}" -eq 1 ]; then
            echo "[ig-diagnostics] Another recovery check is running"
            return 0
        fi
        echo "[ig-diagnostics] Cannot lock recovery budget" >&2
        return 1
    fi
    if [ -e "${directory}/budget" ]; then
        if ! read -r attempts since extra < "${directory}/budget" ||
           ! grep -Eq '^[012]$' <<< "${attempts}" ||
           ! grep -Eq '^(0|[1-9][0-9]{0,9})$' <<< "${since}" ||
           [ -n "${extra}" ]; then
            echo "[ig-diagnostics] Invalid recovery budget; refusing to reset it" >&2
            return 1
        fi
    fi
    state="$(ig_diagnostics_recovery_state)" || return 1
    case "${state%% *}" in
        activating|deactivating|reloading)
            echo "[ig-diagnostics] Service is ${state%% *}; not interrupting it"
            return 0
            ;;
        active)
            if timeout -k 1s 35s python3 "${reader}" --check-startup "${runtime}/collectors.json"; then
                if [ "${since}" -ne 0 ]; then
                    ig_diagnostics_recovery_save_budget "${directory}/budget" "${attempts}" 0 || return 1
                fi
                echo "[ig-diagnostics] All three collectors have fresh reports"
                return 0
            fi
            if ig_diagnostics_recovery_collectors_failed; then
                action=restart
            else
                result=$?
                if [ "${result}" -eq 1 ]; then
                    echo "[ig-diagnostics] Reports unavailable but collectors running; not restarting on staleness alone" >&2
                fi
                return 1
            fi
            ;;
        inactive|failed) ;;
        *)
            echo "[ig-diagnostics] Unknown service state '${state%% *}'; not recovering" >&2
            return 1
            ;;
    esac
    if ! timeout -k 1s 5s systemctl cat ig-diagnostics.service >/dev/null ||
       ! ig_diagnostics_image top_process >/dev/null ||
       ! ig_diagnostics_image top_cpu_throttle >/dev/null; then
        echo "[ig-diagnostics] Recovery prerequisites are incomplete" >&2
        return 1
    fi
    if [ "${attempts}" -ge 2 ]; then
        echo "[ig-diagnostics] Recovery budget exhausted for this boot; operator repair required" >&2
        return 1
    fi
    now="$(ig_diagnostics_recovery_now)" || return 1
    # Wait a full cooldown on the first observed failure, including boot failures.
    if [ "${since}" -eq 0 ]; then
        ig_diagnostics_recovery_save_budget "${directory}/budget" "${attempts}" "${now}" || return 1
        echo "[ig-diagnostics] Recovery deferred for a 15-minute cooldown"
        return 0
    fi
    if [ "${now}" -lt "${since}" ]; then
        echo "[ig-diagnostics] Invalid future recovery timestamp; not recovering" >&2
        return 1
    fi
    if [ "$((now - since))" -lt 900 ]; then
        echo "[ig-diagnostics] Recovery cooldown has not elapsed"
        return 0
    fi
    # An unobserved restart must not let us clear the current ten-minute limit.
    changed="${state#* }"
    if [ "$((now - changed / 1000000))" -lt 600 ]; then
        echo "[ig-diagnostics] Service changed recently; preserving its start-rate limit"
        return 0
    fi
    current="$(ig_diagnostics_recovery_state)" || return 1
    if [ "${current}" != "${state}" ]; then
        echo "[ig-diagnostics] Service state changed; deferring recovery"
        return 0
    fi
    if ! timeout -k 1s 5s systemctl enable ig-diagnostics.service; then
        echo "[ig-diagnostics] Cannot enable service for recovery" >&2
        return 1
    fi
    # Reserve the batch before clearing start limits, even if a command fails.
    attempts=$((attempts + 1))
    ig_diagnostics_recovery_save_budget "${directory}/budget" "${attempts}" "${now}" || return 1
    if ! timeout -k 1s 5s systemctl reset-failed ig-diagnostics.service ||
       ! timeout -k 1s 5s systemctl --no-block --job-mode=fail "${action}" ig-diagnostics.service; then
        echo "[ig-diagnostics] Recovery batch ${attempts}/2 failed" >&2
        return 1
    fi
    echo "[ig-diagnostics] Requested recovery batch ${attempts}/2; awaiting startup report validation"
)

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    ig_diagnostics_recover
fi
