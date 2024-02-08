#! /bin/bash -e

# AKS Local DNS service
#
# This service runs coredns to act as a caching proxy with serve-stale functionality for both
# pod DNS and local node DNS queries. It also upgrades to TCP for better reliability of
# upstream connections.

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

# Utility variable
SCRIPT_PATH="$(dirname -- "$( readlink -f -- "$0"; )";)"

# Get paths for overriding the default DNS behavior
DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status ${DEFAULT_ROUTE_INTERFACE} | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-aks-local-dns.conf"

# Use systemd-notify if we're running as a systemd service; if not, log the output we would have sent
if [[ ! -z "$NOTIFY_SOCKET" ]]; then
    function notify { systemd-notify "$@"; }
else
    function notify { printf "systemd-notify $*\n"; }
fi

# Cleanup function to restore the system to normal on exit or crash
function cleanup {
    # Remove iptables rules on shutdown
    notify --status "removing iptables rules"
    while iptables -t raw -D PREROUTING -d ${DNS_SERVICE_IP}/32 -p tcp -m comment --comment "aks-local-dns: skip conntrack for pod DNS queries" -j NOTRACK 2>/dev/null; do
        true # this is a noop to delete all the rules
    done

    while iptables -t raw -D PREROUTING -d ${DNS_SERVICE_IP}/32 -p udp -m comment --comment "aks-local-dns: skip conntrack for pod DNS queries" -j NOTRACK 2>/dev/null; do
        true # this is a noop to delete all the rules
    done

    # Revert the changes made to the DNS configuration
    notify --status "reverting dns configuration"
    /bin/rm -f ${NETWORK_DROPIN_FILE}
    networkctl reload

    # Delete the dummy interface
    notify --status "removing network link"
    ip link del name aks-local-dns

    # cleaned up
    notify --status "cleanup complete"
}
trap "exit 0" ABRT INT PIPE QUIT TERM
trap "cleanup" EXIT

# Generate corefile
sed -e "s/__PILLAR__KUBE__DNS__SERVICE__/${DNS_SERVICE_IP}/g; \
        s/__PILLAR__LOCAL__POD__DNS__IP__/${LOCAL_POD_DNS_IP}/g; \
        s/__PILLAR__LOCAL__NODE__DNS__IP__/${LOCAL_NODE_DNS_IP}/g; \
        s/__COREDNS__LOG__/${COREDNS_LOG}/g \
        " \
    <"${SCRIPT_PATH}/Corefile.base" >"${SCRIPT_PATH}/Corefile"

# Make sure systemd-resolved is being used
ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf

# Create a dummy interface listening on the link-local IP and the cluster DNS service IP
notify --status "setting up network link"
ip link add name aks-local-dns type dummy
ip link set up dev aks-local-dns
ip addr add ${LOCAL_POD_DNS_IP}/32 dev aks-local-dns
ip addr add ${LOCAL_NODE_DNS_IP}/32 dev aks-local-dns
ip addr add ${DNS_SERVICE_IP}/32 dev aks-local-dns

# Start coredns in the background and send the output to systemd
notify --status "starting coredns"
/opt/azure/aks-local-dns/coredns -conf /opt/azure/aks-local-dns/Corefile \
    | systemd-cat --identifier=aks-local-dns-coredns --stderr-priority=3 &

# Wait to direct traffic to coredns until it can serve traffic from the API server
notify --status "waiting for coredns to be ready"
declare -i ATTEMPTS=0
printf "waiting for coredns to start and be able to serve traffic"
until [ "$(curl -s "http://${LOCAL_NODE_DNS_IP}:8181/ready")" == "OK" ]; do
    if [ $ATTEMPTS -ge 60 ]; then
        printf "\nERROR: coredns failed to come online!\n"
        exit 255
    fi
    sleep 1
    printf "."
    ATTEMPTS+=1
done
printf "done.\n"

# Add IPtables rules that skip conntrack for DNS connections coming from pods
notify --status "adding iptables rules"
iptables -t raw -A PREROUTING -d ${DNS_SERVICE_IP}/32 -p tcp -m comment --comment "aks-local-dns: skip conntrack for pod DNS queries" -j NOTRACK
iptables -t raw -A PREROUTING -d ${DNS_SERVICE_IP}/32 -p udp -m comment --comment "aks-local-dns: skip conntrack for pod DNS queries" -j NOTRACK

# Disable DNS from DHCP and point the system at aks-local-dns
notify --status "updating network DNS configuration"
mkdir -p ${NETWORK_DROPIN_DIR}
printf "[Network]\nDNS=${LOCAL_NODE_DNS_IP}\n\n[DHCP]\nUseDNS=false\n" >${NETWORK_DROPIN_FILE}
chmod -R ugo+rX ${NETWORK_DROPIN_DIR}
networkctl reload

# Enable cluster DNS from the node by adding cluster.local as a domain on the dummy interface
notify --status "updating systemd-resolved configuration"
resolvectl dns aks-local-dns ${LOCAL_NODE_DNS_IP}
resolvectl domain aks-local-dns cluster.local

# Let systemd know we're ready and other processes can continue
notify --ready --status="serving DNS traffic"

# See if we're running in a systemd service with a watchdog
if [[ ! -z "$WATCHDOG_USEC" ]]; then
    # Health check at 80% of WATCHDOG_USEC
    HEALTH_CHECK_INTERVAL=$((${WATCHDOG_USEC:-5000000} * 80 / 100 / 1000000))
    printf "starting watchdog loop at ${HEALTH_CHECK_INTERVAL} second intervals\n"
    while [ "$(curl -s "http://${LOCAL_NODE_DNS_IP}:8181/ready")" == "OK" ]; do
        notify WATCHDOG=1
        sleep ${HEALTH_CHECK_INTERVAL}
    done
else
    # Not running in a watchdog service; just wait for coredns
    wait
fi