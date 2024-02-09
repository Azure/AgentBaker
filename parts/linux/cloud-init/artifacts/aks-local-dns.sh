#! /bin/bash

# Turn on error handling
set -euo pipefail

# AKS Local DNS service
#
# This service runs coredns to act as a caching proxy with serve-stale functionality for both
# pod DNS and local node DNS queries. It also upgrades to TCP for better reliability of
# upstream connections.
#
# TODO: this will not work in IPVS clusters, need to fix this!

# Handle script output
# Prepend timestamps if we're not in systemd, to make running the script manually easier.
# These exec statements read input, prepend each line with a timestamp and descriptor, and then
# print it to the correct output channel.
# Skip if we're running under systemd or set -x
if [[ -z "${SYSTEMD_EXEC_PID:-}" && "${-//[^x]/}" != "x" ]]; then
    SCRIPT_PID=$$
    exec 1> >(trap "" INT TERM USR1 USR2; while read line; do printf "%(%Y-%m-%d %H:%M:%S)T aks-local-dns (${SCRIPT_PID}) INFO:  ${line}\n" >&1; done)
    exec 2> >(trap "" INT TERM USR1 USR2; while read line; do printf "%(%Y-%m-%d %H:%M:%S)T aks-local-dns (${SCRIPT_PID}) ERROR: ${line}\n" >&2; done)
fi

# Configuration variables
# These variables can be overridden by specifying them in /etc/default/aks-local-dns
# Setting COREDNS_LOG to "log" will log queries to systemd
COREDNS_LOG="${COREDNS_LOG:-errors}"
# CoreDNS image reference to use to obtain the binary if not present
COREDNS_IMAGE="${COREDNS_IMAGE:-mcr.microsoft.com/oss/kubernetes/coredns:v1.9.4}"
# Delay coredns shutdown to allow connections to finish
COREDNS_SHUTDOWN_DELAY="${COREDNS_SHUTDOWN_DELAY:-10}"
# This must be the DNS service IP for the cluster
DNS_SERVICE_IP="${DNS_SERVICE_IP:-10.0.0.10}"
# This is the IP that the local DNS service should bind to for pod traffic; usually an APIPA address
LOCAL_POD_DNS_IP="${LOCAL_POD_DNS_IP:-169.254.20.10}"
# This is the IP that the local DNS service should bind to for node traffic; usually an APIPA address
LOCAL_NODE_DNS_IP="${LOCAL_NODE_DNS_IP:-169.254.20.20}"
# Delay between checking for Kubernetes DNS to be online
KUBERNETES_CHECK_DELAY="${KUBERNETES_CHECK_DELAY:-3}"
# PID file
PID_FILE="${PID_FILE:-/run/aks-local-dns.pid}"

# Information variables
SCRIPT_PATH="$(dirname -- "$( readlink -f -- "$0"; )";)"
DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status ${DEFAULT_ROUTE_INTERFACE} | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-aks-local-dns.conf"
UPSTREAM_DNS_SERVERS="$(</run/systemd/resolve/resolv.conf awk '/nameserver/ {print $2}' | paste -sd' ')"

# Build iptables rules
IPTABLES='iptables -w -t raw -m comment --comment "AKS Local DNS Cache: skip conntrack"'
IPTABLES_NODE_RULES=() IPTABLES_POD_RULES=()

## Node DNS rules
IPTABLES_NODE_RULES+=("OUTPUT -m owner --gid-owner $(id -g) -j ACCEPT")
IPTABLES_NODE_RULES+=("OUTPUT -p tcp -d '${LOCAL_NODE_DNS_IP}' --dport 53 -j NOTRACK")
IPTABLES_NODE_RULES+=("OUTPUT -p udp -d '${LOCAL_NODE_DNS_IP}' --dport 53 -j NOTRACK")
IPTABLES_NODE_RULES+=("OUTPUT -p tcp -d '${LOCAL_NODE_DNS_IP}' --dport 53 -j NOTRACK")
IPTABLES_NODE_RULES+=("OUTPUT -p udp -d '${LOCAL_NODE_DNS_IP}' --dport 53 -j NOTRACK")
IPTABLES_NODE_RULES+=("PREROUTING -p tcp -s '${LOCAL_NODE_DNS_IP},${LOCAL_POD_DNS_IP},${DNS_SERVICE_IP}' --dport 53 -j ACCEPT")
IPTABLES_NODE_RULES+=("PREROUTING -p udp -s '${LOCAL_NODE_DNS_IP},${LOCAL_POD_DNS_IP},${DNS_SERVICE_IP}' --dport 53 -j ACCEPT")
IPTABLES_NODE_RULES+=("PREROUTING -p tcp -d '${LOCAL_NODE_DNS_IP}' --dport 53 -j NOTRACK")
IPTABLES_NODE_RULES+=("PREROUTING -p udp -d '${LOCAL_NODE_DNS_IP}' --dport 53 -j NOTRACK")

## Pod DNS rules
IPTABLES_POD_RULES+=("OUTPUT     -p tcp -d '${LOCAL_POD_DNS_IP},${DNS_SERVICE_IP}' --dport 53 -j NOTRACK")
IPTABLES_POD_RULES+=("OUTPUT     -p udp -d '${LOCAL_POD_DNS_IP},${DNS_SERVICE_IP}' --dport 53 -j NOTRACK")
IPTABLES_POD_RULES+=("PREROUTING -p tcp -d '${LOCAL_POD_DNS_IP},${DNS_SERVICE_IP}' --dport 53 -j NOTRACK")
IPTABLES_POD_RULES+=("PREROUTING -p udp -d '${LOCAL_POD_DNS_IP},${DNS_SERVICE_IP}' --dport 53 -j NOTRACK")

# Node-only DNS configuration
COREFILE_NODE_ONLY="""
# Node only DNS (cluster coredns not accessible)
.:53 {
    ${COREDNS_LOG}
    bind ${LOCAL_NODE_DNS_IP}
    forward . ${UPSTREAM_DNS_SERVERS} {
        force_tcp
    }
    ready ${LOCAL_NODE_DNS_IP}:8181
    cache 86400s {
        disable denial
        prefetch 100
        serve_stale 86400s verify
    }
    loop
    nsid aks-local-dns
    prometheus :9253
}
"""

COREFILE_NODE_AND_POD="""
# Node DNS (with cluster.local forward)
.:53 {
    ${COREDNS_LOG}
    bind ${LOCAL_NODE_DNS_IP}
    forward cluster.local ${DNS_SERVICE_IP} {
        force_tcp
    }
    forward . ${UPSTREAM_DNS_SERVERS} {
        force_tcp
    }
    ready ${LOCAL_NODE_DNS_IP}:8181
    cache 86400s {
        disable denial
        prefetch 100
        serve_stale 86400s verify
    }
    loop
    nsid aks-local-dns
    prometheus :9253
}

# Pod DNS
.:53 {
    ${COREDNS_LOG}
    bind ${LOCAL_POD_DNS_IP} ${DNS_SERVICE_IP}
    forward . ${DNS_SERVICE_IP} {
      force_tcp
    }
    cache 86400s {
      disable denial
      prefetch 100
      serve_stale 86400s verify
    }
    loop
    nsid aks-local-dns
    prometheus :9253
}
"""

# Make sure we have coredns
if ! ${SCRIPT_PATH}/coredns -version >/dev/null; then
    printf "coredns not found in ${SCRIPT_PATH}, extracting from docker image\n"
    CTR_TEMP="$(mktemp -d)"

    # Set a trap to clean up the temp directory if anything fails
    function cleanup_coredns_import {
        # Disable error handling so that we don't get into a recursive loop
        set +e
        printf 'Error extracting coredns\n'
        ctr -n k8s.io images unmount ${CTR_TEMP}
        rm -rf ${CTR_TEMP}
    }
    trap cleanup_coredns_import EXIT ABRT ERR INT PIPE QUIT TERM

    # Mount the coredns image to the temporary directory
    ctr -n k8s.io images mount ${COREDNS_IMAGE} ${CTR_TEMP}

    # Copy coredns to SCRIPT_PATH
    cp ${CTR_TEMP}/coredns ${SCRIPT_PATH}/coredns

    # Umount and clean up the temporary directory
    ctr -n k8s.io images unmount ${CTR_TEMP}
    rm -rf "${CTR_TEMP}"

    # Clear the trap
    trap - EXIT ABRT ERR INT PIPE QUIT TERM
fi

# Cleanup function to restore the system to normal on exit or crash
function cleanup {
    # Disable error handling so that we don't get into a recursive loop
    set +e
    printf "terminating and cleaning up\n"

    # Stop the watchdog, if running
    # if [ ! -z "${WATCHDOG_PID:-}" ]; then
    #     kill -SIGTERM ${WATCHDOG_PID}
    # fi

    # Remove iptables rules to stop forwarding DNS traffic
    for RULE in "${IPTABLES_NODE_RULES[@]}" "${IPTABLES_POD_RULES[@]}"; do
        while eval "${IPTABLES}" -D "${RULE}" 2>/dev/null; do 
            printf "removed iptables rule: $RULE\n"
        done
    done
    
    # Revert the changes made to the DNS configuration
    printf "reverting dns configuration by removing ${NETWORK_DROPIN_FILE}\n"
    /bin/rm -f ${NETWORK_DROPIN_FILE}
    networkctl reload

    # Trigger coredns shutdown, if runnin
    if [ ! -z "${COREDNS_PID:-}" ]; then
        if ps ${COREDNS_PID} >/dev/null; then
            # Wait after removing iptables rules and DNS configuration so that we can let connections
            # transition.
            printf "sleeping ${COREDNS_SHUTDOWN_DELAY} seconds to allow connections to gracefully move.\n"
            sleep ${COREDNS_SHUTDOWN_DELAY}

            printf "sending SIGINT to coredns and waiting for it to terminate\n"

            # Send SIGINT to coredns to trigger shutdown
            kill -SIGINT ${COREDNS_PID}

            # Wait for coredns to shut down
            wait -f ${COREDNS_PID}

            printf "coredns terminated\n"
        fi
    fi

    # Delete the dummy interface
    printf "removing aks-local-dns dummy interface\n"
    ip link del name aks-local-dns

    # cleaned up
    printf "cleanup complete\n"
}
trap "exit 0" QUIT TERM
trap "exit 1" ABRT ERR INT PIPE
trap "cleanup" EXIT

# This watchdog function will be called later to let systemd know we're healthy
function watchdog {
    # Health check at 40% of WATCHDOG_USEC; this means that we should check
    # twice in every watchdog interval, and thus need to fail two checks to
    # get restarted.
    HEALTH_CHECK_INTERVAL=$((${WATCHDOG_USEC:-5000000} * 40 / 100 / 1000000))
    printf "starting watchdog loop at ${HEALTH_CHECK_INTERVAL} second intervals\n"
    while [ "$(curl -s "http://${LOCAL_NODE_DNS_IP}:8181/ready")" == "OK" ]; do
        systemd-notify WATCHDOG=1
        sleep ${HEALTH_CHECK_INTERVAL}
    done
}

# Generate corefile for node DNS only
#printf "Setting Corefile:${COREFILE_NODE_ONLY}\n"
printf "${COREFILE_NODE_ONLY}\n" >"${SCRIPT_PATH}/Corefile"

# Create a dummy interface listening on the link-local IP and the cluster DNS service IP
printf "setting up aks-local-dns dummy interface\n"
ip link add name aks-local-dns type dummy
ip link set up dev aks-local-dns
ip addr add ${LOCAL_POD_DNS_IP}/32 dev aks-local-dns
ip addr add ${LOCAL_NODE_DNS_IP}/32 dev aks-local-dns

# Build the coredns command
COREDNS_COMMAND="/opt/azure/aks-local-dns/coredns -conf /opt/azure/aks-local-dns/Corefile -pidfile ${PID_FILE}"
if [[ ! -z "${SYSTEMD_EXEC_PID:-}" ]]; then
    # We're running in systemd, so send the output to journald directly
    COREDNS_COMMAND="systemd-cat --identifier=aks-local-dns-coredns --stderr-priority=3 -- ${COREDNS_COMMAND}"
fi

printf "starting coredns: ${COREDNS_COMMAND}\n"
rm -f ${PID_FILE}
${COREDNS_COMMAND} &
until [ -f ${PID_FILE} ]; do
    sleep 0.1
done
COREDNS_PID="$(cat ${PID_FILE})"
printf "coredns PID is ${COREDNS_PID}\n"

# Wait to direct traffic to coredns until it's ready
declare -i ATTEMPTS=0
printf "waiting for coredns to start and be able to serve traffic\n"
until [ "$(curl -s "http://${LOCAL_NODE_DNS_IP}:8181/ready")" == "OK" ]; do
    if [ $ATTEMPTS -ge 60 ]; then
        printf "\nERROR: coredns failed to come online!\n"
        exit 255
    fi
    sleep 1
    ATTEMPTS+=1
done
printf "coredns online and ready to serve node traffic\n"

# Add IPtables rules that skip conntrack for DNS connections coming from pods
printf "adding iptables rules for node traffic\n"
for RULE in "${IPTABLES_NODE_RULES[@]}"; do
    eval "${IPTABLES}" -A "${RULE}"
done

# Disable DNS from DHCP and point the system at aks-local-dns
printf "updating network DNS configuration to point to coredns via ${NETWORK_DROPIN_FILE}\n"
mkdir -p ${NETWORK_DROPIN_DIR}
printf "[Network]\nDNS=${LOCAL_NODE_DNS_IP}\n\n[DHCP]\nUseDNS=false\n" >${NETWORK_DROPIN_FILE}
chmod -R ugo+rX ${NETWORK_DROPIN_DIR}
networkctl reload

# If we're running in a systemd notify service (see "man systemd.service"), do the required
# extra processing for watchdog and notifying
if [[ ! -z "${NOTIFY_SOCKET:-}" ]]; then
    # Start the watchdog timer in the background if systemd's watchdog is enabled
    if [[ ! -z "${WATCHDOG_USEC:-}" ]]; then
        watchdog &
        WATCHDOG_PID=$!
    fi

    # Let systemd know we're ready and other processes can continue
    systemd-notify --ready
fi
printf "serving node DNS traffic, waiting for cluster coredns to be accessible\n"

until dig -4 +tcp +short +tries=1 +timeout=3 kubernetes.default.svc.cluster.local. A @${DNS_SERVICE_IP} >/dev/null 2>/dev/null; do
    printf "kubernetes DNS not yet accessible (is kube-proxy running?), sleeping for ${KUBERNETES_CHECK_DELAY} seconds...\n"
    sleep ${KUBERNETES_CHECK_DELAY}
done
printf "cluster coredns accessible on ${DNS_SERVICE_IP}\n"
# Kubernetes DNS is returning successfully, so kube-proxy is online. Initialize pod local DNS.

# Add the pod local DNS IP to the aks-local-dns dummy interface
printf "adding ${DNS_SERVICE_IP} to aks-local-dns dummy interface\n"
ip addr add ${DNS_SERVICE_IP}/32 dev aks-local-dns

# Generate corefile for pod DNS and append to existing corefile
printf "regenerating Corefile to include node and pod configuration\n"
#printf "Setting Corefile: ${COREFILE_NODE_AND_POD}\n"
printf "${COREFILE_NODE_AND_POD}" >"${SCRIPT_PATH}/Corefile"

# Send a SIGUSR1 to coredns to trigger reload of the configuration
printf "sending SIGUSR1 to coredns to trigger reload of the new configuration file\n"
kill -SIGUSR1 ${COREDNS_PID}

# Wait for kubernetes DNS to be available via the aks-local-dns pod
printf "waiting for cluster DNS to be accessible via aks-local-dns\n"
until dig -4 +tcp +short +tries=1 +timeout=3 kubernetes.default.svc.cluster.local. A @${LOCAL_POD_DNS_IP} >/dev/null 2>/dev/null; do
    printf "kubernetes DNS not yet accessible via coredns, sleeping for ${KUBERNETES_CHECK_DELAY} seconds...\n"
    sleep ${KUBERNETES_CHECK_DELAY}
done
printf "cluster DNS accessible via aks-local-dns, setting up pod DNS forwarding\n"

# Add IPtables rules that skip conntrack for DNS connections coming from pods
printf "adding iptables rules for pod traffic\n"
for RULE in "${IPTABLES_POD_RULES[@]}"; do
    eval "${IPTABLES}" -A "${RULE}"
done

# Enable cluster DNS from the node by adding cluster.local as a domain on the dummy interface
printf "updating systemd-resolved configuration\n"
resolvectl dns aks-local-dns ${LOCAL_NODE_DNS_IP}
resolvectl domain aks-local-dns cluster.local

printf "startup complete - serving node and pod DNS traffic\n"
wait -f ${COREDNS_PID}

# The cleanup function is called on exit, so it will be run after the wait ends (which will be when a signal is sent or coredns crashes)
