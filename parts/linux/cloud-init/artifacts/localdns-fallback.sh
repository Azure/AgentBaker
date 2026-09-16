#!/usr/bin/env bash
# localdns-fallback.sh
#
# Minimal pod-DNS fallback for LocalDNS. Started by localdns.service's
# OnFailure= and by localdns-fallback-probe.timer. Pods on the node have
# 169.254.10.11 baked into their /etc/resolv.conf and cannot be repointed, so
# without something answering on .11 their DNS black-holes until localdns
# recovers.
#
# NOTE on the OnFailure= trigger: systemd fires OnFailure= on EVERY failed start
# attempt, not only on entry to the terminal 'failed' state. Measured on a live
# node (systemd 255) with Restart=on-failure, 'systemctl is-failed' reported
# 'activating' throughout while the journal logged 'Triggering OnFailure=
# dependencies' once per restart cycle. Binding .11 on each of those would make
# the fallback flap (localdns's ExecStartPre tears it down ~2s later, on the way
# into the next doomed attempt), leaving .11 dark for most of the storm. See
# localdns_is_mid_restart_cycle() below, which makes those early invocations
# no-ops so only a settled localdns hands over.
#
# This binds 169.254.10.11:53 and forwards to the real kube-dns Service
# ClusterIP (COREDNS_SERVICE_IP, persisted to /etc/localdns/environment at
# bootstrap). Forwarding to the ClusterIP relies on kube-proxy DNAT on the node,
# which is healthy in the localdns-crash case this targets.
#
# Scope: pod DNS (.11) only. It intentionally does NOT touch the node listener
# (169.254.10.10) or the node resolver — node-level DNS restoration is handled
# by localdns.service's ExecStopPost. It does not attempt to recover or
# reset-failed localdns; the failed unit is left for NPD to observe.
set -euo pipefail

LOCALDNS_SCRIPT_PATH="/opt/azure/containers/localdns"
COREDNS_BINARY_PATH="${LOCALDNS_SCRIPT_PATH}/binary/coredns"
FALLBACK_DIR="${LOCALDNS_SCRIPT_PATH}/fallback"
FALLBACK_COREFILE="${FALLBACK_DIR}/fallback.corefile"
FALLBACK_PID_FILE="${FALLBACK_DIR}/fallback.pid"

# LocalDNS link-local listener IPs.
LOCALDNS_CLUSTER_LISTENER_IP="169.254.10.11"

# Default kube-dns ClusterIP, used only if the environment file did not provide
# one (mirrors the datamodel default). COREDNS_SERVICE_IP is expected to be set
# via EnvironmentFile=/etc/localdns/environment.
DEFAULT_COREDNS_SERVICE_IP="10.0.0.10"

log() { echo "localdns-fallback: $*" >&2; }

# True while localdns.service is between failing start attempts, i.e. systemd has
# already scheduled (or is about to schedule) another restart. In that window the
# unit is NOT settled: binding .11 here only to have localdns's ExecStartPre stop
# us ~RestartSec later produces flapping rather than coverage.
#
# systemd reports this as ActiveState=activating with SubState=auto-restart (255
# also uses auto-restart-queued). A unit that has genuinely given up is
# ActiveState=failed/SubState=failed; a cleanly stopped one is inactive/dead; a
# running-but-wedged one is active/running. All three of those are legitimate
# reasons to take over .11, so only the auto-restart window is excluded.
localdns_is_mid_restart_cycle() {
    local state
    state="$(systemctl show localdns.service -p SubState --value 2>/dev/null || true)"
    case "${state}" in
        auto-restart|auto-restart-queued) return 0 ;;
        *) return 1 ;;
    esac
}

resolve_upstream() {
    local ip="${COREDNS_SERVICE_IP:-}"
    if [ -z "${ip}" ]; then
        log "COREDNS_SERVICE_IP not set in environment; using default ${DEFAULT_COREDNS_SERVICE_IP}"
        ip="${DEFAULT_COREDNS_SERVICE_IP}"
    fi
    printf '%s' "${ip}"
}

# Idempotently ensure the dummy interface carrying .11 exists and is up. The
# interface's lifetime differs by localdns exit path: the trap path deletes it,
# the ExecStopPost path leaves it assigned. We must handle both — create it if
# missing, and add .11 only if not already present.
ensure_cluster_listener_interface() {
    if ! ip link show localdns >/dev/null 2>&1; then
        log "dummy interface 'localdns' absent; creating it."
        ip link add name localdns type dummy
    fi
    ip link set up dev localdns

    if ip addr show dev localdns | grep -qw "${LOCALDNS_CLUSTER_LISTENER_IP}"; then
        log "cluster listener ${LOCALDNS_CLUSTER_LISTENER_IP} already present on localdns."
    else
        log "assigning cluster listener ${LOCALDNS_CLUSTER_LISTENER_IP} to localdns."
        ip addr add "${LOCALDNS_CLUSTER_LISTENER_IP}/32" dev localdns
    fi
}

generate_fallback_corefile() {
    local upstream
    upstream="$(resolve_upstream)"
    mkdir -p "${FALLBACK_DIR}"
    # A deliberately minimal Corefile: bind only .11 (never .10), forward
    # everything to the kube-dns ClusterIP, with a small cache so a brief
    # upstream blip does not immediately surface to pods.
    cat > "${FALLBACK_COREFILE}" <<EOF
.:53 {
    bind ${LOCALDNS_CLUSTER_LISTENER_IP}
    forward . ${upstream} {
        policy sequential
    }
    cache 30
    loop
    log
    errors
}
EOF
    log "generated fallback corefile forwarding ${LOCALDNS_CLUSTER_LISTENER_IP} -> ${upstream}"
}

verify_coredns_binary() {
    if [ ! -x "${COREDNS_BINARY_PATH}" ]; then
        log "coredns binary missing or not executable at ${COREDNS_BINARY_PATH}."
        return 1
    fi
}

start_fallback() {
    # Exit 0, not non-zero: a refused start is an expected outcome of the
    # per-attempt OnFailure= trigger, not a fault. Failing here would burn the
    # unit's own restart budget and could latch it into 'failed'.
    if localdns_is_mid_restart_cycle; then
        log "localdns.service is between restart attempts; not taking over ${LOCALDNS_CLUSTER_LISTENER_IP} yet."
        return 0
    fi

    verify_coredns_binary
    ensure_cluster_listener_interface
    generate_fallback_corefile

    local cmd="${COREDNS_BINARY_PATH} -conf ${FALLBACK_COREFILE} -pidfile ${FALLBACK_PID_FILE}"
    if [ -n "${SYSTEMD_EXEC_PID:-}" ]; then
        cmd="systemd-cat --identifier=localdns-fallback-coredns --stderr-priority=3 -- ${cmd}"
    fi
    log "starting fallback coredns bound to ${LOCALDNS_CLUSTER_LISTENER_IP}."
    exec ${cmd}
}

# cleanup: remove the .11 address the fallback added, but ONLY if localdns is not
# active (localdns owns .11 when it is running). This runs from ExecStopPost so
# that when localdns recovers and its ExecStartPre stops us, we don't yank the
# address out from under a starting localdns.
cleanup_fallback() {
    if systemctl is-active --quiet localdns.service; then
        log "localdns.service is active; leaving ${LOCALDNS_CLUSTER_LISTENER_IP} in place for it."
        return 0
    fi
    # localdns is not active. Leave the interface/address as-is: localdns's own
    # start path (add_iptable_rules_to_skip_conntrack_from_pods) deletes and
    # recreates the interface, so we do not need to remove it here, and removing
    # it could black-hole pods during the gap before localdns rebinds.
    log "cleanup complete (no address removal; localdns start path re-creates the interface)."
    return 0
}

# When sourced (e.g. by ShellSpec), stop here so functions can be tested without
# dispatching a mode.
${__SOURCED__:+return}

case "${1:-start}" in
    start)   start_fallback ;;
    cleanup) cleanup_fallback ;;
    *) log "unknown mode '${1:-}'; expected start|cleanup"; exit 1 ;;
esac
