#! /bin/bash
set -euo pipefail

#######################################################################
# AKS Local DNS Service
#######################################################################
# This service runs coredns to act as a caching proxy with serve-stale functionality for both
# pod DNS and local node DNS queries. It also upgrades to TCP for better reliability of
# upstream connections.

#######################################################################
# validate we're running in our cgroup - needed for iptables rules
#######################################################################
CGROUP_PATH="$(cut -d: -f3 </proc/self/cgroup)"
if [[ "${CGROUP_PATH}" != /aks.slice/aks-local.slice/aks-local-dns.slice/* ]]; then
    printf "ERROR: not running under expected slice path:\n" >&2
    printf "Current cgroup path:  ${CGROUP_PATH}\n" >&2
    printf "Required cgroup path: /aks.slice/aks-local.slice/aks-local-dns.slice/aks-local-dns.service\n\n" >&2
    printf "To run from a command line for testing, use the following command:\n" >&2
    printf "systemd-run -dGt --slice=aks-local-dns ./aks-local-dns.sh\n\n" >&2
    exit 2
fi

# Configuration variables
# These variables can be overridden by specifying them in /etc/default/aks-local-dns
# Setting COREDNS_LOG to "log" will log queries to systemd
COREDNS_LOG="${COREDNS_LOG:-errors}"
# CoreDNS image reference to use to obtain the binary if not present
COREDNS_IMAGE="${COREDNS_IMAGE:-mcr.microsoft.com/oss/kubernetes/coredns:v1.9.4}"
# Delay coredns shutdown to allow connections to finish
COREDNS_SHUTDOWN_DELAY="${COREDNS_SHUTDOWN_DELAY:-5}"
# This must be the DNS service IP for the cluster
DNS_SERVICE_IP="${DNS_SERVICE_IP:-10.0.0.10}"
# Determine if the DNS service IP should be bound (this must be false on IPVS clusters)
BIND_DNS_SERVICE_IP="${BIND_DNS_SERVICE_IP:-true}"
# This is the IP that the local DNS service should bind to for node traffic; usually an APIPA address
LOCAL_NODE_DNS_IP="${LOCAL_NODE_DNS_IP:-169.254.10.10}"
# This is the IP that the local DNS service should bind to for pod traffic; usually an APIPA address
LOCAL_POD_DNS_IP="${LOCAL_POD_DNS_IP:-169.254.10.11}"
# Delay between checking for Kubernetes DNS to be online
KUBERNETES_CHECK_DELAY="${KUBERNETES_CHECK_DELAY:-3}"
# PID file
PID_FILE="${PID_FILE:-/run/aks-local-dns.pid}"

#######################################################################
# information variables
#######################################################################
SCRIPT_PATH="$(dirname -- "$(readlink -f -- "$0";)";)"
DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status ${DEFAULT_ROUTE_INTERFACE} | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-aks-local-dns.conf"
UPSTREAM_DNS_SERVERS="$(</run/systemd/resolve/resolv.conf awk '/nameserver/ {print $2}' | paste -sd' ')"
if [[ "${BIND_DNS_SERVICE_IP}" == "true" ]]; then
  POD_MODE_BIND_IPS="${LOCAL_POD_DNS_IP},${DNS_SERVICE_IP}"
else
  POD_MODE_BIND_IPS="${LOCAL_POD_DNS_IP}"
fi

#######################################################################
# iptables: build rules
#######################################################################
# These rules skip conntrack for DNS traffic; this has two advantages. 
# First, traffic that skips conntrack skips kube-proxy rules, so it hits
# this service. Second, this means that DNS traffic won't require a conntrack
# table entry. The cgroup not-match makes sure that we can still talk to 
# kube-dns from this application.
# OUTPUT rules affect node services and hostNetwork: true pods
# PREROUTING rules affect traffic from regular pods.
IPTABLES='iptables -w -t raw -m comment --comment "AKS Local DNS Cache: skip conntrack"'
IPTABLES_NODE_RULES=() IPTABLES_POD_RULES=()
IPTABLES_NODE_RULES+=("OUTPUT -p tcp -d '${LOCAL_NODE_DNS_IP}' --dport 53 -m cgroup ! --path ${CGROUP_PATH} -j NOTRACK")
IPTABLES_NODE_RULES+=("OUTPUT -p udp -d '${LOCAL_NODE_DNS_IP}' --dport 53 -m cgroup ! --path ${CGROUP_PATH} -j NOTRACK")
IPTABLES_POD_RULES+=("OUTPUT -p tcp -d '${POD_MODE_BIND_IPS}' --dport 53 -m cgroup ! --path ${CGROUP_PATH} -j NOTRACK")
IPTABLES_POD_RULES+=("OUTPUT -p udp -d '${POD_MODE_BIND_IPS}' --dport 53 -m cgroup ! --path ${CGROUP_PATH} -j NOTRACK")
IPTABLES_POD_RULES+=("PREROUTING -p tcp -d '${LOCAL_NODE_DNS_IP},${POD_MODE_BIND_IPS}' --dport 53 -j NOTRACK")
IPTABLES_POD_RULES+=("PREROUTING -p udp -d '${LOCAL_NODE_DNS_IP},${POD_MODE_BIND_IPS}' --dport 53 -j NOTRACK")

#######################################################################
# corefile templates
#######################################################################
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
    nsid aks-local-dns-node
    prometheus :9253
}
"""
# Node and pod DNS configuration
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
    nsid aks-local-dns-node
    prometheus :9253
}

# Pod DNS
.:53 {
    ${COREDNS_LOG}
    bind ${POD_MODE_BIND_IPS//,/ }
    forward . ${DNS_SERVICE_IP} {
      force_tcp
    }
    cache 86400s {
      disable denial
      prefetch 100
      serve_stale 86400s verify
    }
    loop
    nsid aks-local-dns-pod
    prometheus :9253
}
"""

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

#######################################################################
# cleanup function: will be run on script exit/crash to revert config
#######################################################################
function cleanup {
    # Disable error handling so that we don't get into a recursive loop
    set +e
    printf "terminating and cleaning up\n"

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

    # Delete the dummy interface
    printf "removing aks-local-dns dummy interface\n"
    ip link del name aks-local-dns

    # cleaned up
    printf "cleanup complete\n"
}
trap "exit 0" QUIT TERM
trap "exit 1" ABRT ERR INT PIPE
trap "cleanup" EXIT

#######################################################################
# systemd watchdog: send pings so we get restarted if we go unhealthy
#######################################################################
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

#######################################################################
# setup for node DNS (no pod DNS until we know kube-proxy is online)
#######################################################################
# Generate corefile for node DNS only
printf "${COREFILE_NODE_ONLY}\n" >"${SCRIPT_PATH}/Corefile"

# Create a dummy interface listening on the link-local IP and the cluster DNS service IP
printf "setting up aks-local-dns dummy interface\n"
ip link add name aks-local-dns type dummy
ip link set up dev aks-local-dns

printf "adding ${LOCAL_NODE_DNS_IP} to aks-local-dns dummy interface\n";
ip addr add ${LOCAL_NODE_DNS_IP}/32 dev aks-local-dns

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
    fi

    # Let systemd know we're ready and other processes can continue
    systemd-notify --ready
fi
printf "serving node DNS traffic, waiting for cluster coredns to be accessible\n"

#######################################################################
# wait for cluster coredns accessibility
#######################################################################
until dig -4 +tcp +short +tries=1 +timeout=3 kubernetes.default.svc.cluster.local. A @${DNS_SERVICE_IP} >/dev/null 2>/dev/null; do
    printf "kubernetes DNS not yet accessible (is kube-proxy running?), sleeping for ${KUBERNETES_CHECK_DELAY} seconds...\n"
    sleep ${KUBERNETES_CHECK_DELAY}
done
printf "cluster coredns accessible on ${DNS_SERVICE_IP}\n"
# Kubernetes DNS is returning successfully, so kube-proxy is online.

#######################################################################
# setup for pod DNS
#######################################################################
# Add the pod local DNS IP to the aks-local-dns dummy interface
printf "adding ${LOCAL_POD_DNS_IP} to aks-local-dns dummy interface\n"
ip addr add ${LOCAL_POD_DNS_IP}/32 dev aks-local-dns
if [ "${BIND_DNS_SERVICE_IP}" == "true" ]; then
    printf "adding ${DNS_SERVICE_IP} to aks-local-dns dummy interface\n"
    ip addr add ${DNS_SERVICE_IP}/32 dev aks-local-dns
fi

# Regenerate corefile for node and pod DNS
printf "regenerating Corefile to include node and pod configuration\n"
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

#######################################################################
# end of line
#######################################################################

