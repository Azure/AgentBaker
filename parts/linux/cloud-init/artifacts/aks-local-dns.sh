#! /bin/bash -e

# AKS Local DNS service
#
# This service runs coredns to act as a caching proxy with serve-stale functionality for both
# pod DNS and local node DNS queries. It also upgrades to TCP for better reliability of
# upstream connections.
#
# TODO: this will not work in IPVS clusters, need to fix this!

# Configuration variables
# These variables can be overridden by specifying them in /etc/default/aks-local-dns
# Setting COREDNS_LOG to "log" will log queries to systemd
COREDNS_LOG="${COREDNS_LOG:-errors}"
# This must be the DNS service IP for the cluster
DNS_SERVICE_IP="${DNS_SERVICE_IP:-10.0.0.10}"
# This is the IP that the local DNS service should bind to for pod traffic; usually an APIPA address
LOCAL_POD_DNS_IP="${LOCAL_POD_DNS_IP:-169.254.20.10}"
# This is the IP that the local DNS service should bind to for node traffic; usually an APIPA address
LOCAL_NODE_DNS_IP="${LOCAL_NODE_DNS_IP:-169.254.20.20}"
# Delay between checking for Kubernetes DNS to be online
KUBERNETES_CHECK_DELAY="${KUBERNETES_CHECK_DELAY:-3}"

# Utility variable
SCRIPT_PATH="$(dirname -- "$( readlink -f -- "$0"; )";)"

# Get paths for overriding the default DNS behavior
DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status ${DEFAULT_ROUTE_INTERFACE} | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-aks-local-dns.conf"
UPSTREAM_DNS_SERVERS="$(</run/systemd/resolve/resolv.conf awk '/nameserver/ {print $2}' | paste -sd' ')"

SED_COMMAND="s/__PILLAR__KUBE__DNS__SERVICE__/${DNS_SERVICE_IP}/g; \
             s/__PILLAR__UPSTREAM__DNS__SERVERS__/${UPSTREAM_DNS_SERVERS}/g; \
             s/__PILLAR__LOCAL__POD__DNS__IP__/${LOCAL_POD_DNS_IP}/g; \
             s/__PILLAR__LOCAL__NODE__DNS__IP__/${LOCAL_NODE_DNS_IP}/g; \
             s/__COREDNS__LOG__/${COREDNS_LOG}/g"

# Use systemd-notify if we're running as a systemd service; if not, log the output we would have sent
if [[ ! -z "$NOTIFY_SOCKET" ]]; then
    function notify {
        printf "$*\n";
        systemd-notify --status "$@";
    }
else
    function notify { printf "$*\n"; }
fi

# Cleanup function to restore the system to normal on exit or crash
function cleanup {
    # Disable quit on error so that we clean up as best we can
    set +e

    # Remove iptables rules on shutdown
    while iptables -t raw -D PREROUTING -d ${DNS_SERVICE_IP}/32 -p tcp -m comment --comment "aks-local-dns: skip conntrack for pod DNS queries" -j NOTRACK 2>/dev/null; do
        notify "removed tcp iptables rule"
    done

    while iptables -t raw -D PREROUTING -d ${DNS_SERVICE_IP}/32 -p udp -m comment --comment "aks-local-dns: skip conntrack for pod DNS queries" -j NOTRACK 2>/dev/null; do
        notify "removed udp iptables rule"
    done

    # Revert the changes made to the DNS configuration
    notify "reverting dns configuration by removing ${NETWORK_DROPIN_FILE}"
    /bin/rm -f ${NETWORK_DROPIN_FILE}
    networkctl reload

    # Kill any background jobs like CoreDNS
    notify "sending SIGTERM to child coredns processes"
    pkill -SIGTERM -eP $$ coredns

    # Delete the dummy interface
    notify "removing aks-local-dns dummy network link"
    ip link del name aks-local-dns

    # cleaned up
    notify "cleanup complete"
}
trap "exit 0" ABRT INT PIPE QUIT TERM
trap "cleanup" EXIT

# This watchdog function will be called later to let systemd know we're healthy
function watchdog {
    # See if we're running in a systemd service with a watchdog
    if [[ ! -z "$WATCHDOG_USEC" ]]; then
        # Health check at 40% of WATCHDOG_USEC; this means that we should check
        # twice in every watchdog interval, and thus need to fail two checks to
        # get restarted.
        HEALTH_CHECK_INTERVAL=$((${WATCHDOG_USEC:-5000000} * 40 / 100 / 1000000))
        printf "starting watchdog loop at ${HEALTH_CHECK_INTERVAL} second intervals\n"
        while [ "$(curl -s "http://${LOCAL_NODE_DNS_IP}:8181/ready")" == "OK" ]; do
            systemd-notify WATCHDOG=1
            sleep ${HEALTH_CHECK_INTERVAL}
        done
    fi
}

# Generate corefile for node DNS only
sed -e "$SED_COMMAND" <"${SCRIPT_PATH}/Corefile.node" >"${SCRIPT_PATH}/Corefile"

# Create a dummy interface listening on the link-local IP and the cluster DNS service IP
notify "setting up network link"
ip link add name aks-local-dns type dummy
ip link set up dev aks-local-dns
ip addr add ${LOCAL_POD_DNS_IP}/32 dev aks-local-dns
ip addr add ${LOCAL_NODE_DNS_IP}/32 dev aks-local-dns

notify "starting coredns"
if [[ ! -z "$NOTIFY_SOCKET" ]]; then
    # Start coredns in the background and send the output to systemd
    systemd-cat --identifier=aks-local-dns-coredns --stderr-priority=3 -- \
        /opt/azure/aks-local-dns/coredns -conf /opt/azure/aks-local-dns/Corefile &
else
    /opt/azure/aks-local-dns/coredns -conf /opt/azure/aks-local-dns/Corefile &
fi
COREDNS_PID=$!
echo "CoreDNS PID is ${COREDNS_PID}"

# Wait to direct traffic to coredns until it can serve traffic from the API server
declare -i ATTEMPTS=0
notify "waiting for coredns to start and be able to serve traffic"
until [ "$(curl -s "http://${LOCAL_NODE_DNS_IP}:8181/ready")" == "OK" ]; do
    if [ $ATTEMPTS -ge 60 ]; then
        printf "\nERROR: coredns failed to come online!\n"
        exit 255
    fi
    sleep 1
    ATTEMPTS+=1
done
notify "coredns online and ready to serve node traffic"

# Disable DNS from DHCP and point the system at aks-local-dns
notify "updating network DNS configuration to point to coredns via ${NETWORK_DROPIN_FILE}"
mkdir -p ${NETWORK_DROPIN_DIR}
printf "[Network]\nDNS=${LOCAL_NODE_DNS_IP}\n\n[DHCP]\nUseDNS=false\n" >${NETWORK_DROPIN_FILE}
chmod -R ugo+rX ${NETWORK_DROPIN_DIR}
networkctl reload

# Start the watchdog timer in the background
watchdog &

# Let systemd know we're ready and other processes can continue
if [[ ! -z "$NOTIFY_SOCKET" ]]; then
    systemd-notify --ready
fi
notify "serving node DNS traffic, waiting for cluster coredns to be accessible"

until dig -4 +tcp +short +tries=1 +timeout=3 kubernetes.default.svc.cluster.local. A @${DNS_SERVICE_IP} >/dev/null 2>/dev/null; do
    printf "kubernetes DNS not yet accessible (is kube-proxy running?), sleeping for ${KUBERNETES_CHECK_DELAY} seconds...\n"
    sleep ${KUBERNETES_CHECK_DELAY}
done
notify "cluster coredns accessible on ${DNS_SERVICE_IP}"
# Kubernetes DNS is returning successfully, so kube-proxy is online. Initialize pod local DNS

# Add the pod local DNS IP to the aks-local-dns dummy interface
notify "adding ${DNS_SERVICE_IP} to aks-local-dns dummy interface"
ip addr add ${DNS_SERVICE_IP}/32 dev aks-local-dns

# Generate corefile for pod DNS and append to existing corefile
notify "regenerating Corefile to include node and pod configuration"
sed -e "$SED_COMMAND" <"${SCRIPT_PATH}/Corefile.pod" >"${SCRIPT_PATH}/Corefile"

# Send a SIGUSR1 to coredns to trigger reload of the configuration
notify "sending SIGHUP to coredns to trigger reload of the new configuration file"
pkill -SIGUSR1 -eP $$ coredns

# Wait for kubernetes DNS to be available via the aks-local-dns pod
notify "waiting for cluster DNS to be accessible via aks-local-dns"
until dig -4 +tcp +short +tries=1 +timeout=3 kubernetes.default.svc.cluster.local. A @${LOCAL_POD_DNS_IP} >/dev/null 2>/dev/null; do
    printf "kubernetes DNS not yet accessible via coredns, sleeping for ${KUBERNETES_CHECK_DELAY} seconds...\n"
    sleep ${KUBERNETES_CHECK_DELAY}
done
notify "cluster DNS accessible via aks-local-dns, setting up pod DNS forwarding"

# Add IPtables rules that skip conntrack for DNS connections coming from pods
notify "adding iptables rules"
iptables -t raw -A PREROUTING -d ${DNS_SERVICE_IP}/32 -p tcp -m comment --comment "aks-local-dns: skip conntrack for pod DNS queries" -j NOTRACK
iptables -t raw -A PREROUTING -d ${DNS_SERVICE_IP}/32 -p udp -m comment --comment "aks-local-dns: skip conntrack for pod DNS queries" -j NOTRACK

# Enable cluster DNS from the node by adding cluster.local as a domain on the dummy interface
notify "updating systemd-resolved configuration"
resolvectl dns aks-local-dns ${LOCAL_NODE_DNS_IP}
resolvectl domain aks-local-dns cluster.local

notify "startup complete - serving node and pod DNS traffic"
wait