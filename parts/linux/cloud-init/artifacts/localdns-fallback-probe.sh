#!/usr/bin/env bash
# localdns-fallback-probe.sh
#
# Safety-net trigger for the LocalDNS pod-DNS fallback. localdns.service's
# OnFailure= only fires on entry to the terminal 'failed' state, so it does not
# cover cases where 169.254.10.11 stops answering while the unit is NOT 'failed':
#   - localdns cleanly stopped and staying stopped (inactive)
#   - localdns 'active' but its .11 listener is wedged / socket dead
#
# This probe runs periodically (via localdns-fallback-probe.timer). It checks
# whether .11 answers and, only after a debounce of N consecutive failures,
# starts the fallback. On recovery it resets and lets localdns's own
# ExecStartPre reclaim .11 (it does not stop the fallback out from under a
# handoff; localdns.service does that).
#
# Debounce exists to absorb routine localdns restarts (RestartSec=2), during
# which .11 is briefly dark but self-heals well within the debounce window.
set -euo pipefail

LOCALDNS_CLUSTER_LISTENER_IP="169.254.10.11"
HEALTH_CHECK_NAME="health-check.localdns.local"
DIG_TIMEOUT=1
DIG_TRIES=1

# Debounce: FAIL_THRESHOLD consecutive failures at the timer interval (~5s)
# before we switch over. 3 x ~5s ≈ 15s, above a routine RestartSec=2 restart.
FAIL_THRESHOLD="${LOCALDNS_FALLBACK_PROBE_FAIL_THRESHOLD:-3}"

STATE_DIR="/run/localdns-fallback"
FAIL_COUNTER_FILE="${STATE_DIR}/consecutive_fails"

FALLBACK_SERVICE="localdns-fallback.service"

log() { echo "localdns-fallback-probe: $*"; }

read_counter() {
    if [ -r "${FAIL_COUNTER_FILE}" ]; then
        local v
        v="$(cat "${FAIL_COUNTER_FILE}" 2>/dev/null || echo 0)"
        case "${v}" in
            ''|*[!0-9]*) echo 0 ;;
            *) echo "${v}" ;;
        esac
    else
        echo 0
    fi
}

write_counter() {
    mkdir -p "${STATE_DIR}"
    echo "$1" > "${FAIL_COUNTER_FILE}"
}

cluster_listener_answers() {
    # Exit 0 for ANY DNS response (including NXDOMAIN) => a listener is alive on
    # .11, whether that is localdns or the fallback. Non-zero only on
    # timeout/connection failure => nothing is serving .11.
    dig +short +timeout="${DIG_TIMEOUT}" +tries="${DIG_TRIES}" \
        "${HEALTH_CHECK_NAME}" "@${LOCALDNS_CLUSTER_LISTENER_IP}" >/dev/null 2>&1
}

fallback_is_active() {
    systemctl is-active --quiet "${FALLBACK_SERVICE}"
}

main() {
    # Fail OPEN, not closed. dig is not installed by the VHD build on every image
    # (localdns itself never needs it - it health-checks with curl against the
    # ready endpoint on :8181), and a missing dig makes cluster_listener_answers
    # return 127, which reads as ".11 is dark" on every single tick. That would
    # start the fallback against a perfectly healthy localdns forever. Doing
    # nothing is the correct response to "cannot measure".
    if ! command -v dig >/dev/null 2>&1; then
        log "dig is not available on this image; cannot evaluate ${LOCALDNS_CLUSTER_LISTENER_IP}, taking no action."
        return 0
    fi

    if cluster_listener_answers; then
        # .11 is being served. Reset the debounce counter. Do NOT stop the
        # fallback here: if the fallback is what is answering, stopping it would
        # black-hole pods. localdns.service's ExecStartPre handles the handoff
        # when localdns itself comes back and reclaims .11.
        local prev
        prev="$(read_counter)"
        if [ "${prev}" -ne 0 ]; then
            log ".11 answering again; resetting debounce counter (was ${prev})."
            write_counter 0
        fi
        return 0
    fi

    # .11 is dark. Advance the debounce counter.
    local fails
    fails="$(( $(read_counter) + 1 ))"
    write_counter "${fails}"
    log ".11 not answering (${fails}/${FAIL_THRESHOLD})."

    if [ "${fails}" -lt "${FAIL_THRESHOLD}" ]; then
        # Within the debounce window; likely a routine restart. Wait.
        return 0
    fi

    if fallback_is_active; then
        # Debounce tripped but fallback already up and yet .11 is still dark:
        # this is not a localdns-only problem (e.g. kube-proxy / interface).
        # Nothing more this probe can do; leave it for NPD.
        log "fallback already active but .11 still dark; leaving for NPD."
        return 0
    fi

    log ".11 dark for ${fails} consecutive probes; starting ${FALLBACK_SERVICE}."
    # --no-block so the probe oneshot returns promptly; systemd starts it.
    systemctl start --no-block "${FALLBACK_SERVICE}" || {
        log "failed to start ${FALLBACK_SERVICE}."
        return 1
    }
}

# When sourced (e.g. by ShellSpec), stop here so functions can be tested without
# executing the probe.
${__SOURCED__:+return}

main "$@"
