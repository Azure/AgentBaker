#!/usr/bin/env bash
# localdns-fallback.sh
#
# Kernel-only pod/node DNS fallback for a localdns.service in terminal 'failed'.
# No listener, no CoreDNS binary, no corefile -- just NAT redirects that move
# packets until NPD repairs the node.
#
# WHY NO COREDNS. An earlier revision ran a second CoreDNS against a corefile
# derived from localdns's own. That shares a failure mode with the thing it is
# backing up: if localdns died because its baked binary cannot parse a directive
# the RP sent (failfast_all_unhealthy_upstreams needs >=1.12.1; on v1.11.3 it is
# a hard startup failure), the fallback inherits the same binary and the same
# directive and dies with it. A fallback must not fail for the same reason its
# principal did. iptables rules cannot crash-loop or misparse a config.
#
# WHAT IT COVERS. Pods get 169.254.10.11 (dnsPolicy ClusterFirst) or
# 169.254.10.10 (dnsPolicy Default, hostNetwork) written into /etc/resolv.conf
# at sandbox creation and can never be repointed, so both listeners must keep
# working:
#
#   169.254.10.10:53  -> the VNet DNS from the restored systemd-resolved
#                        upstream file, i.e. external resolution only
#   169.254.10.11:53  -> kube-dns, by jumping into kube-proxy's own service
#                        chain, or by DNAT across the backends the local Cilium
#                        agent reports
#
# WHY NOT DNAT .11 TO THE CLUSTERIP. The nat table is traversed once per
# connection and DNAT is a terminal verdict for it, so a rule that rewrites .11
# to the ClusterIP consumes the traversal that KUBE-SERVICES would have used to
# translate that ClusterIP into a pod IP. On Cilium the service load balancer
# runs before netfilter, so a ClusterIP target is never translated at all. Both
# verified on live nodes. Going one hop further -- straight to the backends --
# also means this script never needs to know the ClusterIP.
#
# NOT COVERED, BY DESIGN. .10 clients lose cluster.local, which is the same
# position they are in on a node with localdns disabled. No cache, no
# serve_stale, no hosts plugin, no VnetDNS overrides. This is a bridge that
# keeps DNS moving for the minutes before NPD repairs the node, not a
# replacement for localdns.
set -euo pipefail

CHAIN=LOCALDNS-FALLBACK
NODE_IP=169.254.10.10
CLUSTER_IP=169.254.10.11
# The upstream file localdns's cleanup path restores; the same one localdns.sh
# reads. Not /etc/resolv.conf, which on a stub-resolver node is 127.0.0.53 and
# would point us back through systemd-resolved at an uplink that may still be
# .10 -- a loop.
RESOLV="${RESOLV:-/run/systemd/resolve/resolv.conf}"
CILIUM_SOCK="${CILIUM_SOCK:-/var/run/cilium/cilium.sock}"
# localdns's ExecStopPost removes the resolv.conf drop-in and runs
# 'networkctl reload', which is ASYNCHRONOUS, and OnFailure= fires as soon as
# ExecStopPost returns. Until the reload lands the file still holds the drop-in
# state -- DNS=169.254.10.10 with UseDNS=false, i.e. .10 and nothing else -- so
# reading it immediately yields no usable upstream. Same race
# wait_for_localdns_removed_from_resolv_conf (localdns.sh) exists for on the
# start path. Bounded, because a node whose resolv.conf never converges has a
# bigger problem than DNS fallback.
RESOLV_WAIT_SECONDS="${RESOLV_WAIT_SECONDS:-10}"

log() { echo "localdns-fallback: $*" >&2; }
die() { log "$*"; exit 1; }

clear_rules() {
    local hook
    for hook in PREROUTING OUTPUT; do
        while iptables -w -t nat -D "${hook}" -j "${CHAIN}" 2>/dev/null; do :; done
    done
    iptables -w -t nat -F "${CHAIN}" 2>/dev/null || true
    iptables -w -t nat -X "${CHAIN}" 2>/dev/null || true
}

# First nameserver in the restored upstream file that is not our own node
# listener. Polls for convergence rather than reading once; see
# RESOLV_WAIT_SECONDS above.
wait_for_vnet_dns() {
    local deadline=$((SECONDS + RESOLV_WAIT_SECONDS))
    local found=""
    while :; do
        found=$(awk -v self="${NODE_IP}" '$1 == "nameserver" && $2 != self { print $2; exit }' \
                "${RESOLV}" 2>/dev/null || true)
        [ -n "${found}" ] && { printf '%s' "${found}"; return 0; }
        [ "${SECONDS}" -ge "${deadline}" ] && return 1
        sleep 0.25
    done
}

# kube-proxy derives the chain name from "kube-system/kube-dns:<port>" plus the
# protocol; read it off KUBE-SERVICES rather than recomputing the hash, so a
# rename upstream surfaces as "not found" instead of a silently wrong jump.
# Empty on Cilium nodes, where KUBE-SERVICES does not exist.
kube_dns_chain() {
    local proto="$1" port="$2"
    iptables -w -t nat -S KUBE-SERVICES 2>/dev/null \
        | awk -v c="kube-system/kube-dns:${port} cluster IP" -v p="-p ${proto}" \
            'index($0, c) && index($0, p) { for (i = 1; i <= NF; i++) if ($i == "-j") print $(i+1) }' \
        | head -1 || true
}

# Active kube-dns backends for one protocol, from the node's own Cilium agent.
cilium_kube_dns_backends() {
    curl -sf --max-time 5 --unix-socket "${CILIUM_SOCK}" http://localhost/v1/service 2>/dev/null \
        | jq -r --arg proto "$1" '
            .[].spec
            | select(.flags.namespace == "kube-system" and .flags.name == "kube-dns")
            | select(.["frontend-address"].port == 53 and .["frontend-address"].protocol == $proto)
            | .["backend-addresses"][]? | select((.state // "active") == "active") | .ip' \
        | sort -u || true
}

# Is a Cilium agent present on this node? Wrapped in a function so tests can
# stand in for it without creating a real unix socket.
cilium_agent_present() {
    [ -S "${CILIUM_SOCK}" ]
}

# With eBPF host routing, pod egress is redirected past netfilter entirely, so a
# nat rule would never see the packet. AKS defaults to legacy host routing.
cilium_legacy_host_routing() {
    curl -sf --max-time 5 --unix-socket "${CILIUM_SOCK}" http://localhost/v1/config 2>/dev/null \
        | jq -e '.status.daemonConfigurationMap.EnableHostLegacyRouting == true' >/dev/null
}

# Even probability across backends, the same 1/(n-i) ladder kube-proxy uses: the
# last rule takes whatever the earlier draws did not.
add_spread() {
    local proto="$1"; shift
    local n=$# i=0 ip
    for ip in "$@"; do
        if [ "${i}" -lt $((n - 1)) ]; then
            iptables -w -t nat -A "${CHAIN}" -d "${CLUSTER_IP}/32" -p "${proto}" --dport 53 \
                -m statistic --mode random \
                --probability "$(awk -v n="${n}" -v i="${i}" 'BEGIN { printf "%.5f", 1 / (n - i) }')" \
                -j DNAT --to-destination "${ip}:53"
        else
            iptables -w -t nat -A "${CHAIN}" -d "${CLUSTER_IP}/32" -p "${proto}" --dport 53 \
                -j DNAT --to-destination "${ip}:53"
        fi
        i=$((i + 1))
    done
}

apply_rules() {
    local state vnet_dns udp_chain tcp_chain mode udp_backends tcp_backends proto

    # Terminal 'failed' only. While localdns is active -- or starting, or between
    # restart attempts -- redirecting would silently take traffic away from a
    # listener that is about to work.
    state=$(systemctl show localdns.service -p ActiveState --value 2>/dev/null || true)
    [ "${state}" = "failed" ] || die "localdns.service is '${state}', not failed; not redirecting"

    vnet_dns=$(wait_for_vnet_dns) \
        || die "no upstream nameserver other than ${NODE_IP} in ${RESOLV} after ${RESOLV_WAIT_SECONDS}s"

    udp_chain=$(kube_dns_chain udp dns)
    tcp_chain=$(kube_dns_chain tcp dns-tcp)
    if [ -n "${udp_chain}" ] && [ -n "${tcp_chain}" ]; then
        mode=kube-proxy
    elif cilium_agent_present; then
        cilium_legacy_host_routing \
            || die "Cilium eBPF host routing: pod traffic bypasses netfilter, cannot redirect ${CLUSTER_IP}"
        udp_backends=$(cilium_kube_dns_backends UDP)
        tcp_backends=$(cilium_kube_dns_backends TCP)
        [ -n "${udp_backends}" ] && [ -n "${tcp_backends}" ] \
            || die "Cilium agent reported no active kube-dns backends"
        mode=cilium
    else
        die "neither kube-proxy kube-dns chains nor a Cilium agent socket found"
    fi

    clear_rules
    iptables -w -t nat -N "${CHAIN}"
    for proto in udp tcp; do
        iptables -w -t nat -A "${CHAIN}" -d "${NODE_IP}/32" -p "${proto}" --dport 53 \
            -j DNAT --to-destination "${vnet_dns}:53"
    done
    if [ "${mode}" = "kube-proxy" ]; then
        iptables -w -t nat -A "${CHAIN}" -d "${CLUSTER_IP}/32" -p udp --dport 53 -j "${udp_chain}"
        iptables -w -t nat -A "${CHAIN}" -d "${CLUSTER_IP}/32" -p tcp --dport 53 -j "${tcp_chain}"
        log "applied (kube-proxy): ${NODE_IP} -> ${vnet_dns}, ${CLUSTER_IP} -> ${udp_chain}/${tcp_chain}"
    else
        # shellcheck disable=SC2086
        add_spread udp ${udp_backends}
        # shellcheck disable=SC2086
        add_spread tcp ${tcp_backends}
        # shellcheck disable=SC2086
        log "applied (cilium): ${NODE_IP} -> ${vnet_dns}, ${CLUSTER_IP} -> $(echo ${udp_backends} | tr '\n' ' ')"
    fi

    # Ahead of KUBE-SERVICES, so the redirect is decided before anything else in
    # nat gets a chance at the packet.
    iptables -w -t nat -I PREROUTING 1 -j "${CHAIN}"
    iptables -w -t nat -I OUTPUT 1 -j "${CHAIN}"
}

# When sourced (e.g. by ShellSpec), stop here so functions can be tested without
# dispatching a mode.
${__SOURCED__:+return}

case "${1:-}" in
    apply) apply_rules ;;
    clear) clear_rules; log "cleared" ;;
    *) echo "usage: $0 apply|clear" >&2; exit 2 ;;
esac
