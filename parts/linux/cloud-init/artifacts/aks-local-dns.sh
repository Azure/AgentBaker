#! /bin/bash
set -euo pipefail

#######################################################################
# AKS Local DNS Service
#######################################################################
# This service runs coredns to act as a caching proxy with serve-stale functionality for both
# pod DNS and local node DNS queries. It also upgrades to TCP for better reliability of
# upstream connections.

. /etc/default/aks-local-dns

# Configuration variables
# These variables can be overridden by specifying them in /etc/default/aks-local-dns
# Setting COREDNS_LOG to "log" will log queries to systemd
COREDNS_LOG="${COREDNS_LOG:-errors}"
# CoreDNS image reference to use to obtain the binary if not present
COREDNS_IMAGE="${COREDNS_IMAGE:-mcr.microsoft.com/oss/kubernetes/coredns:v1.9.4}"
# Delay coredns shutdown to allow connections to finish
COREDNS_SHUTDOWN_DELAY="${COREDNS_SHUTDOWN_DELAY:-5}"
# This must be the DNS service IP for the cluster
DNS_SERVICE_IP="${DNS_SERVICE_IP:-}"
# This is the IP that the local DNS service should bind to for node traffic; usually an APIPA address
LOCAL_NODE_DNS_IP="${LOCAL_NODE_DNS_IP:-169.254.10.10}"
# This is the IP that the local DNS service should bind to for pod traffic; usually an APIPA address
LOCAL_POD_DNS_IP="${LOCAL_POD_DNS_IP:-169.254.10.11}"
# PID file
PID_FILE="${PID_FILE:-/run/aks-local-dns.pid}"

if [[ -z "${DNS_SERVICE_IP}" && ! $* == *--cleanup* ]]; then
     printf "ERROR: DNS_SERVICE_IP is not set.\n"
     exit 1
fi

#######################################################################
# information variables
#######################################################################
SCRIPT_PATH="$(dirname -- "$(readlink -f -- "$0";)";)"
DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status ${DEFAULT_ROUTE_INTERFACE} | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-aks-local-dns.conf"
UPSTREAM_DNS_SERVERS="$(</run/systemd/resolve/resolv.conf awk '/nameserver/ {print $2}' | paste -sd' ')"

#######################################################################
# iptables: build rules
#######################################################################
# These rules skip conntrack for DNS traffic to save conntrack table
# space. OUTPUT rules affect node services and hostNetwork: true pods.
# PREROUTING rules affect traffic from regular pods.
IPTABLES='iptables -w -t raw -m comment --comment "AKS Local DNS: skip conntrack"'
IPTABLES_RULES=()
for CHAIN in OUTPUT PREROUTING; do
for IP in ${LOCAL_NODE_DNS_IP} ${LOCAL_POD_DNS_IP}; do
for PROTO in tcp udp; do
    IPTABLES_RULES+=("${CHAIN} -p ${PROTO} -d ${IP} --dport 53 -j NOTRACK")
done; done; done

#######################################################################
# corefile template
#######################################################################
COREFILE="""
# whoami (used for health check of dns pipeline)
health-check.aks-local-dns.local:53 {
    bind ${LOCAL_NODE_DNS_IP} ${LOCAL_POD_DNS_IP}
    whoami
}

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
    cache 3600s {
        success 9984 5
        denial 9984 5
        serve_stale 3600s verify
    }
    loop
    nsid aks-local-dns
    prometheus ${LOCAL_NODE_DNS_IP}:9253
}

# Pod DNS
.:53 {
    ${COREDNS_LOG}
    bind ${LOCAL_POD_DNS_IP}
    forward . ${DNS_SERVICE_IP} {
        force_tcp
    }
    cache 3600s {
        success 9984 5
        denial 9984 5
        serve_stale 3600s verify
    }
    loop
    nsid aks-local-dns-pod
}
"""

#######################################################################
# cleanup function: will be run on script exit/crash to revert config
#######################################################################
function cleanup {
    # Disable error handling so that we don't get into a recursive loop
    set +e

    # Remove iptables rules to stop forwarding DNS traffic
    for RULE in "${IPTABLES_RULES[@]}"; do
        while eval "${IPTABLES}" -D "${RULE}" 2>/dev/null; do 
            printf "removed iptables rule: $RULE\n"
        done
    done
    
    # Revert the changes made to the DNS configuration if present
    if [ -f ${NETWORK_DROPIN_FILE} ]; then
        printf "reverting dns configuration by removing ${NETWORK_DROPIN_FILE}\n"
        /bin/rm -f ${NETWORK_DROPIN_FILE}
        networkctl reload
    fi

    # Trigger coredns shutdown, if runnin
    if [ ! -z "${COREDNS_PID:-}" ]; then
        if ps ${COREDNS_PID} >/dev/null; then
            if [[ ${COREDNS_SHUTDOWN_DELAY} -gt 0 ]]; then
                # Wait after removing iptables rules and DNS configuration so that we can let connections
                # transition.
                printf "sleeping ${COREDNS_SHUTDOWN_DELAY} seconds to allow connections to terminate\n"
                sleep ${COREDNS_SHUTDOWN_DELAY}
            fi

            printf "sending SIGINT to coredns and waiting for it to terminate\n"

            # Send SIGINT to coredns to trigger shutdown
            kill -SIGINT ${COREDNS_PID}

            # Wait for coredns to shut down
            wait -f ${COREDNS_PID}

            printf "coredns terminated\n"
        fi
    fi

    # Delete the dummy interface if present
    if ip link show dev aks-local-dns >/dev/null 2>&1; then
        printf "removing aks-local-dns dummy interface\n"
        ip link del name aks-local-dns
    fi
}

# If we're invoked with cleanup, run cleanup
if [[ $* == *--cleanup* ]]; then
    cleanup
    exit 0
fi

#######################################################################
# coredns: extract from image
#######################################################################
if [ ! -x ${SCRIPT_PATH}/coredns ]; then
    printf "extracting coredns from docker image: ${COREDNS_IMAGE}\n"
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
    ctr -n k8s.io images mount ${COREDNS_IMAGE} ${CTR_TEMP} >/dev/null

    # Copy coredns to SCRIPT_PATH
    cp ${CTR_TEMP}/coredns ${SCRIPT_PATH}/coredns

    # Umount and clean up the temporary directory
    ctr -n k8s.io images unmount ${CTR_TEMP} >/dev/null
    rm -rf "${CTR_TEMP}"

    # Clear the trap
    trap - EXIT ABRT ERR INT PIPE QUIT TERM
fi

# Enable the cleanup function now that we have a coredns binary
trap "exit 0" QUIT TERM                                    # Exit with code 0 on a successful shutdown
trap "exit 1" ABRT ERR INT PIPE                            # Exit with code 1 on a bad signal
trap "printf 'executing cleanup function\n'; cleanup" EXIT # Always cleanup when you're exiting

#######################################################################
# generate the corefile
#######################################################################
printf "${COREFILE}\n" >"${SCRIPT_PATH}/Corefile"

# Create a dummy interface listening on the link-local IP and the cluster DNS service IP
printf "setting up aks-local-dns dummy interface with IPs ${LOCAL_NODE_DNS_IP} and ${LOCAL_POD_DNS_IP}\n"
ip link add name aks-local-dns type dummy
ip link set up dev aks-local-dns
ip addr add ${LOCAL_NODE_DNS_IP}/32 dev aks-local-dns
ip addr add ${LOCAL_POD_DNS_IP}/32 dev aks-local-dns

# Add IPtables rules that skip conntrack for DNS connections coming from pods
printf "adding iptables rules to skip conntrack for queries to aks-local-dns\n"
for RULE in "${IPTABLES_RULES[@]}"; do
    eval "${IPTABLES}" -A "${RULE}"
done

# Build the coredns command
COREDNS_COMMAND="/opt/azure/aks-local-dns/coredns -conf /opt/azure/aks-local-dns/Corefile -pidfile ${PID_FILE}"
if [[ ! -z "${SYSTEMD_EXEC_PID:-}" ]]; then
    # We're running in systemd, so pass the coredns output via systemd-cat
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

# Disable DNS from DHCP and point the system at aks-local-dns
printf "updating network DNS configuration to point to coredns via ${NETWORK_DROPIN_FILE}\n"
mkdir -p ${NETWORK_DROPIN_DIR}
printf "[Network]\nDNS=${LOCAL_NODE_DNS_IP}\n\n[DHCP]\nUseDNS=false\n" >${NETWORK_DROPIN_FILE}
chmod -R ugo+rX ${NETWORK_DROPIN_DIR}
networkctl reload

printf "startup complete - serving node and pod DNS traffic\n"

#######################################################################
# systemd notify: send ready if service is Type=notify
#######################################################################
if [[ ! -z "${NOTIFY_SOCKET:-}" ]]; then systemd-notify --ready; fi

#######################################################################
# systemd watchdog: send pings so we get restarted if we go unhealthy
#######################################################################
# If the watchdog is defined, we check pod status and pass success to systemd
if [[ ! -z "${NOTIFY_SOCKET:-}" && ! -z "${WATCHDOG_USEC:-}" ]]; then
    # Health check at 40% of WATCHDOG_USEC; this means that we should check
    # twice in every watchdog interval, and thus need to fail two checks to
    # get restarted.
    HEALTH_CHECK_INTERVAL=$((${WATCHDOG_USEC:-5000000} * 40 / 100 / 1000000))
    HEALTH_CHECK_DNS_REQUEST="health-check.aks-local-dns.local @169.254.10.10\nhealth-check.aks-local-dns.local @169.254.10.11"
    printf "starting watchdog loop at ${HEALTH_CHECK_INTERVAL} second intervals\n"
    while [ true ]; do
        if [[ "$(curl -s "http://${LOCAL_NODE_DNS_IP}:8181/ready")" == "OK" ]]; then
            if dig +short +timeout=1 +tries=1 -f<(printf "${HEALTH_CHECK_DNS_REQUEST}"); then
                systemd-notify WATCHDOG=1
            fi
        fi
        sleep ${HEALTH_CHECK_INTERVAL}
    done
else
    wait -f ${COREDNS_PID}
fi

# The cleanup function is called on exit, so it will be run after the
# wait ends (which will be when a signal is sent or coredns crashes) or
# the script receives a terminal signal.
#######################################################################
# end of line
#######################################################################
