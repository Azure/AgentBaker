#!/usr/bin/env bash
# localdns-kubelet-dns.sh
#
# Repoints kubelet's --cluster-dns between the LocalDNS cluster listener
# (169.254.10.11) and the real kube-dns Service ClusterIP.
#
# Why this exists. A pod's /etc/resolv.conf is written once, by kubelet, when its
# sandbox is created, and is never revisited. So the two pod generations on a node
# whose localdns has failed need different remedies:
#
#   already-running pods -> cannot be repointed at all; something must keep
#                           answering on .11 (localdns-fallback.service)
#   pods created later   -> can be pointed straight at CoreDNS, so they do not
#                           depend on the fallback existing or working
#
# This script is the second half. It is deliberately surgical: it rewrites only
# the --cluster-dns= value inside the existing KUBELET_FLAGS= line and restarts
# kubelet. It does NOT re-run ensureKubelet(), which would be destructive at
# runtime: with TLS_BOOTSTRAP_TOKEN and API_SERVER_NAME absent from a post-
# provisioning environment, ensureKubelet truncates KUBELET_FLAGS to empty
# (cse_config.sh, 'echo "KUBELET_FLAGS=${KUBELET_FLAGS}" > ...') and overwrites the
# live /var/lib/kubelet/kubeconfig with one pointing at https://:443 and a client
# certificate a TLS-bootstrapped node does not use.
#
# Both modes are idempotent and only restart kubelet when they actually changed
# the file, so a flapping localdns cannot flap kubelet.
set -euo pipefail

KUBELET_DEFAULT_FILE="/etc/default/kubelet"
LOCALDNS_CLUSTER_LISTENER_IP="169.254.10.11"
DEFAULT_COREDNS_SERVICE_IP="10.0.0.10"
# Records what --cluster-dns was before we first touched it, so 'restore' puts
# back exactly that rather than assuming the constant.
ORIGINAL_STATE_FILE="/etc/localdns/kubelet-cluster-dns.orig"

log() { echo "localdns-kubelet-dns: $*" >&2; }

current_cluster_dns() {
    [ -r "${KUBELET_DEFAULT_FILE}" ] || return 1
    grep -oE '\-\-cluster-dns=[^" ]+' "${KUBELET_DEFAULT_FILE}" 2>/dev/null | head -1 | cut -d= -f2-
}

resolve_coredns_service_ip() {
    local ip="${COREDNS_SERVICE_IP:-}"
    if [ -z "${ip}" ]; then
        log "COREDNS_SERVICE_IP not set; using default ${DEFAULT_COREDNS_SERVICE_IP}"
        ip="${DEFAULT_COREDNS_SERVICE_IP}"
    fi
    printf '%s' "${ip}"
}

# Rewrite the --cluster-dns value in place, preserving every other flag. Staged
# through a temp file because redirecting into the file we are reading truncates
# it first.
set_cluster_dns() {
    local want="$1" tmp
    tmp="$(mktemp)"
    sed -E "s|--cluster-dns=[^\" ]+|--cluster-dns=${want}|g" "${KUBELET_DEFAULT_FILE}" > "${tmp}"
    if ! grep -q -- "--cluster-dns=${want}" "${tmp}"; then
        log "failed to rewrite --cluster-dns to ${want}; leaving ${KUBELET_DEFAULT_FILE} untouched."
        rm -f "${tmp}"
        return 1
    fi
    cat "${tmp}" > "${KUBELET_DEFAULT_FILE}"
    rm -f "${tmp}"
    chmod 0600 "${KUBELET_DEFAULT_FILE}"
}

restart_kubelet() {
    log "restarting kubelet so the new --cluster-dns applies to pods created from now on."
    # --no-block: we are called from the fallback's start path and must not gate
    # DNS recovery for existing pods on kubelet finishing its restart.
    systemctl restart --no-block kubelet.service || {
        log "kubelet restart request failed."
        return 1
    }
}

point_to_coredns() {
    local cur want
    cur="$(current_cluster_dns || true)"
    if [ -z "${cur}" ]; then
        log "no --cluster-dns found in ${KUBELET_DEFAULT_FILE}; nothing to repoint."
        return 0
    fi
    want="$(resolve_coredns_service_ip)"
    if [ "${cur}" = "${want}" ]; then
        log "kubelet --cluster-dns is already ${want}; no change, not restarting kubelet."
        return 0
    fi
    if [ "${cur}" != "${LOCALDNS_CLUSTER_LISTENER_IP}" ]; then
        log "kubelet --cluster-dns is ${cur}, not the localdns cluster listener; leaving it alone."
        return 0
    fi
    mkdir -p "$(dirname "${ORIGINAL_STATE_FILE}")"
    printf '%s' "${cur}" > "${ORIGINAL_STATE_FILE}"
    set_cluster_dns "${want}" || return 1
    log "kubelet --cluster-dns ${cur} -> ${want} (new pods will bypass ${LOCALDNS_CLUSTER_LISTENER_IP})."
    restart_kubelet
}

restore() {
    local cur want coredns_ip
    cur="$(current_cluster_dns || true)"
    if [ -z "${cur}" ]; then
        log "no --cluster-dns found in ${KUBELET_DEFAULT_FILE}; nothing to restore."
        return 0
    fi

    # Only ever undo a change WE made. This runs from localdns.service's
    # ExecStartPost, i.e. on every localdns start including every boot, so it must
    # not treat "the current value is not the localdns listener" as licence to
    # rewrite it: on a node whose --cluster-dns legitimately differs, that would
    # silently overwrite RP-provided configuration and bounce kubelet every time
    # localdns starts. Evidence that it was us is either the recorded original, or
    # a current value that is exactly the CoreDNS ClusterIP we would have set.
    coredns_ip="${COREDNS_SERVICE_IP:-}"
    if [ -s "${ORIGINAL_STATE_FILE}" ]; then
        want="$(cat "${ORIGINAL_STATE_FILE}")"
    elif [ -n "${coredns_ip}" ] && [ "${cur}" = "${coredns_ip}" ]; then
        # Recorded original lost (e.g. /etc/localdns wiped), but the value on disk
        # is the one we would have written. Safe to hand it back.
        want="${LOCALDNS_CLUSTER_LISTENER_IP}"
        log "no recorded original, but --cluster-dns is ${cur} (the CoreDNS ClusterIP); restoring to ${want}."
    else
        # Not a value we set. Leave it alone, silently: the overwhelmingly common
        # case is a normal start where nothing was ever repointed.
        return 0
    fi

    if [ "${cur}" = "${want}" ]; then
        rm -f "${ORIGINAL_STATE_FILE}"
        return 0
    fi
    set_cluster_dns "${want}" || return 1
    rm -f "${ORIGINAL_STATE_FILE}"
    log "kubelet --cluster-dns ${cur} -> ${want} (localdns is serving again)."
    restart_kubelet
}

# When sourced (e.g. by ShellSpec), stop here so functions can be tested without
# dispatching a mode.
${__SOURCED__:+return}

case "${1:-}" in
    point-to-coredns) point_to_coredns ;;
    restore)          restore ;;
    *) log "unknown mode '${1:-}'; expected point-to-coredns|restore"; exit 1 ;;
esac
