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
KUBELET_DNS_SCRIPT="${LOCALDNS_SCRIPT_PATH}/localdns-kubelet-dns.sh"

# The corefile localdns itself runs. UPDATED_ is the one actually passed to
# coredns (post AzureDNS-IP rewrite); the other is the pre-rewrite source.
UPDATED_LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/updated.localdns.corefile"
LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/localdns.corefile"

# LocalDNS link-local listener IPs.
LOCALDNS_NODE_LISTENER_IP="169.254.10.10"
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
    mkdir -p "${FALLBACK_DIR}"
    if derive_fallback_corefile_from_localdns; then
        return 0
    fi
    generate_minimal_fallback_corefile
}

# Preferred path: reuse the .11 half of the Corefile localdns was already running.
#
# Those server blocks already forward to the real kube-dns ClusterIP (see the
# KubeDNS-overrides branch in pkg/agent/baker.go, 'forward . {{$.CoreDNSServiceIP}}'),
# so deriving from them preserves behaviour a hand-written minimal Corefile cannot:
# the hosts plugin for critical AKS FQDNs, the long cache with serve_stale, the
# internal.cloudapp.net / reddog.microsoft.com NXDOMAIN templates, and the ready and
# prometheus endpoints the localdns exporter scrapes. It also keeps the upstream
# correct on clusters where COREDNS_SERVICE_IP was never populated, because the
# generator that produced this file already resolved it.
#
# Blocks are kept only if they bind .11, and the bind line is rewritten to .11 alone:
# the health-check block binds BOTH listeners (baker.go: 'bind {{$.NodeListenerIP}}
# {{$.ClusterListenerIP}}') and this unit must never take over the node listener.
derive_fallback_corefile_from_localdns() {
    local src=""
    local candidate
    for candidate in "${UPDATED_LOCALDNS_CORE_FILE}" "${LOCALDNS_CORE_FILE}"; do
        if [ -s "${candidate}" ]; then src="${candidate}"; break; fi
    done
    if [ -z "${src}" ]; then
        log "no localdns corefile to derive from; falling back to the minimal corefile."
        return 1
    fi

    awk -v want="${LOCALDNS_CLUSTER_LISTENER_IP}" '
        function flush_block(   i, l, indent) {
            if (keep) {
                for (i = 0; i < n; i++) {
                    l = buf[i]
                    if (l ~ /^[[:space:]]*bind[[:space:]]/) {
                        match(l, /^[[:space:]]*/)
                        indent = substr(l, 1, RLENGTH)
                        print indent "bind " want
                    } else {
                        print l
                    }
                }
            }
            n = 0; keep = 0
        }
        BEGIN { depth = 0; n = 0; keep = 0 }
        {
            line = $0
            if (depth == 0) {
                # Server blocks start at column 0 and open a brace on the same line.
                if (line ~ /^[^[:space:]#].*\{[[:space:]]*$/) { buf[n++] = line; depth = 1 }
                next
            }
            buf[n++] = line
            tmp = line
            opens = gsub(/\{/, "{", tmp)
            tmp = line
            closes = gsub(/\}/, "}", tmp)
            depth += opens - closes
            if (line ~ /^[[:space:]]*bind[[:space:]]/ && index(line, want) > 0) keep = 1
            if (depth <= 0) { flush_block(); depth = 0 }
        }
        END { if (n > 0) flush_block() }
    ' "${src}" > "${FALLBACK_COREFILE}.derived" 2>/dev/null

    if [ ! -s "${FALLBACK_COREFILE}.derived" ] || ! grep -q "bind ${LOCALDNS_CLUSTER_LISTENER_IP}" "${FALLBACK_COREFILE}.derived"; then
        log "derived corefile from ${src} has no ${LOCALDNS_CLUSTER_LISTENER_IP} server block; falling back to the minimal corefile."
        rm -f "${FALLBACK_COREFILE}.derived"
        return 1
    fi
    # Never let a derived block seize the node listener.
    if grep -q "${LOCALDNS_NODE_LISTENER_IP}" "${FALLBACK_COREFILE}.derived"; then
        log "derived corefile still references ${LOCALDNS_NODE_LISTENER_IP}; falling back to the minimal corefile."
        rm -f "${FALLBACK_COREFILE}.derived"
        return 1
    fi

    mv "${FALLBACK_COREFILE}.derived" "${FALLBACK_COREFILE}"
    log "derived fallback corefile from ${src} ($(grep -c "^[^[:space:]#].*{" "${FALLBACK_COREFILE}") server block(s) bound to ${LOCALDNS_CLUSTER_LISTENER_IP})"
    return 0
}

# Floor: a deliberately minimal Corefile, used when the localdns corefile is
# missing or unusable (including when a malformed corefile is what killed
# localdns in the first place, in which case inheriting it would be pointless).
# Binds only .11, never .10, and forwards everything to the kube-dns ClusterIP.
generate_minimal_fallback_corefile() {
    local upstream
    upstream="$(resolve_upstream)"
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
    log "generated minimal fallback corefile forwarding ${LOCALDNS_CLUSTER_LISTENER_IP} -> ${upstream}"
}

verify_coredns_binary() {
    if [ ! -x "${COREDNS_BINARY_PATH}" ]; then
        log "coredns binary missing or not executable at ${COREDNS_BINARY_PATH}."
        return 1
    fi
}

# True when something in localdns.service's cgroup already holds .11:53. Any
# spurious trigger (for example a probe that cannot run dig and therefore reads
# .11 as permanently dark) must not make us fight a healthy localdns for the
# socket: coredns would exit 'address already in use' on every attempt, and with
# an unbounded restart budget that is a hot loop on a healthy node.
cluster_listener_owned_by_localdns() {
    local pid
    pid="$(ss -lunpH 2>/dev/null | grep "${LOCALDNS_CLUSTER_LISTENER_IP}:53" \
           | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2)"
    [ -z "${pid}" ] && return 1
    grep -q 'localdns\.service' "/proc/${pid}/cgroup" 2>/dev/null
}

start_fallback() {
    # Every foreseeable "not our turn" below returns 0 rather than failing. The
    # unit has an unbounded restart budget so that it is never permanently refused
    # a restart; that is only safe if a predictable condition can never make it
    # exit non-zero, which would turn Restart=on-failure into a spin.
    if localdns_is_mid_restart_cycle; then
        log "localdns.service is between restart attempts; not taking over ${LOCALDNS_CLUSTER_LISTENER_IP} yet."
        return 0
    fi
    if cluster_listener_owned_by_localdns; then
        log "localdns.service already owns ${LOCALDNS_CLUSTER_LISTENER_IP}:53; nothing to do."
        return 0
    fi
    if ! verify_coredns_binary; then
        log "cannot serve ${LOCALDNS_CLUSTER_LISTENER_IP} without the coredns binary; giving up quietly."
        return 0
    fi

    ensure_cluster_listener_interface
    generate_fallback_corefile

    # Repoint kubelet before handing off to coredns. Pods created from here on get
    # the real CoreDNS ClusterIP and never depend on this unit at all; only pods
    # that already exist rely on .11, because their resolv.conf cannot be rewritten.
    "${KUBELET_DNS_SCRIPT}" point-to-coredns || log "kubelet repoint failed; continuing to serve ${LOCALDNS_CLUSTER_LISTENER_IP} for existing pods."

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
