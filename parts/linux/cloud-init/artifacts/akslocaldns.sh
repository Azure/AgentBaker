#! /bin/bash
set -euo pipefail

# akslocaldns systemd unit.
# --------------------------------------------------------------------------------------------------------------------
# This systemd unit runs coredns as a caching with serve-stale functionality for both pod DNS and node DNS queries. 
# It also upgrades to TCP for better reliability of upstream connections.

# Verify the required files exists.
# --------------------------------------------------------------------------------------------------------------------
# This should match with 'AKSLOCALDNS_ENV_FILE' defined in parts/linux/cloud-init/artifacts/cse_config.sh.
# This file contains the environment variables used by akslocaldns.
AKSLOCALDNS_ENV_FILE_PATH="/etc/default/akslocaldns/akslocaldns.envfile"
if [ -f "${AKSLOCALDNS_ENV_FILE_PATH}" ]; then
    source "${AKSLOCALDNS_ENV_FILE_PATH}"
else
    printf "Error: akslocaldns envfile does not exist at %s.\n" "${AKSLOCALDNS_ENV_FILE_PATH}"
    exit 1
fi

# Generated Corefile path used by akslocaldns service.
# This should match with 'AKSLOCALDNS_CORE_FILE' defined in parts/linux/cloud-init/artifacts/cse_config.sh.
AKSLOCALDNS_CORE_FILE_PATH="/opt/azure/akslocaldns/corefile"
if [ ! -f "${AKSLOCALDNS_CORE_FILE_PATH}" ] || [ ! -s "${AKSLOCALDNS_CORE_FILE_PATH}" ]; then
    printf "Error: akslocaldns corefile either does not exist or is empty at %s.\n" "${AKSLOCALDNS_CORE_FILE_PATH}"
    exit 1
fi

 # This is slice file for akslocaldns.
AKSLOCALDNS_SLICE_PATH="/etc/systemd/system/akslocaldns.slice"
if [ ! -f "${AKSLOCALDNS_SLICE_PATH}" ]; then
    printf "Error: akslocaldns slice file does not exist at %s.\n" "${AKSLOCALDNS_SLICE_PATH}"
    exit 1
fi

# Load environment variables from envfile.
# --------------------------------------------------------------------------------------------------------------------
# CoreDNS image reference to use to obtain the binary if not present.
: "${AKSLOCALDNS_IMAGE_URL:?AKSLOCALDNS_IMAGE_URL is not set}"
# This is the IP that akslocaldns service should bind to for node traffic; an APIPA address.
: "${AKSLOCALDNS_NODE_LISTENER_IP:?AKSLOCALDNS_NODE_LISTENER_IP is not set}"
# This is the IP that akslocaldns service should bind to for pod traffic; an APIPA address.
: "${AKSLOCALDNS_CLUSTER_LISTENER_IP:?AKSLOCALDNS_CLUSTER_LISTENER_IP is not set}"
# CPU limit for akslocaldns service.
: "${AKSLOCALDNS_CPU_LIMIT:?AKSLOCALDNS_CPU_LIMIT is not set}"
# Memory limit in MB for akslocaldns service.
: "${AKSLOCALDNS_MEMORY_LIMIT:?AKSLOCALDNS_MEMORY_LIMIT is not set}"
# Delay coredns shutdown to allow connections to finish.
: "${AKSLOCALDNS_SHUTDOWN_DELAY:?AKSLOCALDNS_SHUTDOWN_DELAY is not set}"
# PID file.
: "${AKSLOCALDNS_PID_FILE:?AKSLOCALDNS_PID_FILE is not set}"

# Check if coredns binary is cached in VHD.
# --------------------------------------------------------------------------------------------------------------------
# Extract image tag version after `:`
SCRIPT_PATH="/opt/azure/akslocaldns"
COREDNS_VERSION="${AKSLOCALDNS_IMAGE_URL##*:}"

# Coredns binary is extracted from cached coredns image(s) and pre-installed in the VHD -
# /opt/azure/akslocaldns/<version>/coredns.
COREDNS_BINARY_PATH="${SCRIPT_PATH}/${COREDNS_VERSION}/coredns"
if [ ! -x "${COREDNS_BINARY_PATH}" ]; then
    printf "Error: coredns binary not found at %s.\n" "${COREDNS_BINARY_PATH}"
    exit 1
fi

# Configure CPU and Memory limit.
# --------------------------------------------------------------------------------------------------------------------
# Takes a percentage value, suffixed with "%". The percentage specifies how much CPU time the unit shall get at maximum, 
# relative to the total CPU time available on one CPU. Use values > 100% for allotting CPU time on more than one CPU.
CPU_QUOTA="$((AKSLOCALDNS_CPU_LIMIT * 100))%"

CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)
if [ "${CGROUP_VERSION}" = "cgroup2fs" ] || [ "${CGROUP_VERSION}" = "tmpfs" ]; then
    sed -i \
        -e "s/^CPUQuota=[^ ]*/CPUQuota=${CPU_QUOTA}/" \
        -e "s/^MemoryMax=[^ ]*/MemoryMax=${AKSLOCALDNS_MEMORY_LIMIT}M/" \
        "${AKSLOCALDNS_SLICE_PATH}" || { echo "Error: updating akslocaldns slice failed"; exit 1; }
else
    echo "Error: Unsupported cgroup version: ${CGROUP_VERSION}"
    exit 1
fi

# Replace Vnet_Dns_Servers in corefile with VNET DNS Server IPs.
# --------------------------------------------------------------------------------------------------------------------
UPSTREAM_VNET_DNS_SERVERS="$(</run/systemd/resolve/resolv.conf awk '/nameserver/ {print $2}' | paste -sd' ')"
if [ -z "${UPSTREAM_VNET_DNS_SERVERS}" ]; then
    printf "Error: No Upstream VNET DNS servers found in /run/systemd/resolve/resolv.conf.\n"
    exit 1
fi
# Replace all occurrences of Vnet_Dns_Servers with UPSTREAM_VNET_DNS_SERVERS in akslocaldns corefile.
# Based on customer input, corefile was generated with Vnet_Dns_Servers as placeholder in pkg/agent/baker.go.
sed -i "s/Vnet_Dns_Servers/${UPSTREAM_VNET_DNS_SERVERS}/g" \
    "${AKSLOCALDNS_CORE_FILE_PATH}" || { echo "Error: updating akslocaldns corefile failed"; exit 1; }

echo Generated corefile:
cat "${AKSLOCALDNS_CORE_FILE_PATH}"

# Iptables: build rules.
# --------------------------------------------------------------------------------------------------------------------
# These rules skip conntrack for DNS traffic to save conntrack table space. 
# OUTPUT rules affect node services and hostNetwork: true pods.
# PREROUTING rules affect traffic from regular pods.
IPTABLES='iptables -w -t raw -m comment --comment "AKS Local DNS: skip conntrack"'
IPTABLES_RULES=()
for CHAIN in OUTPUT PREROUTING; do
for IP in ${AKSLOCALDNS_NODE_LISTENER_IP} ${AKSLOCALDNS_CLUSTER_LISTENER_IP}; do
for PROTO in tcp udp; do
    IPTABLES_RULES+=("${CHAIN} -p ${PROTO} -d ${IP} --dport 53 -j NOTRACK")
done; done; done

# Information variables.
# --------------------------------------------------------------------------------------------------------------------
DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status "${DEFAULT_ROUTE_INTERFACE}" | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-akslocaldns.conf"

# Cleanup function will be run on script exit/crash to revert config.
# --------------------------------------------------------------------------------------------------------------------
function cleanup {
    # Disable error handling so that we don't get into a recursive loop.
    set +e

    # Remove iptables rules to stop forwarding DNS traffic.
    for RULE in "${IPTABLES_RULES[@]}"; do
        if eval "${IPTABLES}" -C "${RULE}" 2>/dev/null; then
            eval "${IPTABLES}" -D "${RULE}"
            printf "Removed iptables rule: %s.\n" "${RULE}"
        fi
    done

    # Revert the changes made to the DNS configuration if present.
    if [ -f ${NETWORK_DROPIN_FILE} ]; then
        printf "Reverting DNS configuration by removing %s.\n" "${NETWORK_DROPIN_FILE}"
        /bin/rm -f ${NETWORK_DROPIN_FILE}
        networkctl reload
    fi

    # Trigger akslocaldns shutdown, if running.
    if [ ! -z "${COREDNS_PID:-}" ]; then
        if ps ${COREDNS_PID} >/dev/null; then
            if [[ ${AKSLOCALDNS_SHUTDOWN_DELAY} -gt 0 ]]; then
                # Wait after removing iptables rules and DNS configuration so that we can let connections transition.
                printf "sleeping %d seconds to allow connections to terminate.\n" "${AKSLOCALDNS_SHUTDOWN_DELAY}"
                sleep ${AKSLOCALDNS_SHUTDOWN_DELAY}
            fi
            printf "sending SIGINT to akslocaldns and waiting for it to terminate.\n"

            # Send SIGINT to akslocaldns to trigger shutdown.
            kill -SIGINT ${COREDNS_PID}

            # Wait for akslocaldns to shut down.
            wait -f ${COREDNS_PID}
            printf "akslocaldns terminated.\n"
        fi
    fi

    # Delete the dummy interface if present.
    if ip link show dev akslocaldns >/dev/null 2>&1; then
        printf "removing akslocaldns dummy interface.\n"
        ip link del name akslocaldns
    fi
}

# If we're invoked with cleanup, run cleanup.
if [[ $* == *--cleanup* ]]; then
    cleanup
    exit 0
fi

# Enable the cleanup function now that we have a coredns binary.
trap "exit 0" QUIT TERM                                    # Exit with code 0 on a successful shutdown.
trap "exit 1" ABRT ERR INT PIPE                            # Exit with code 1 on a bad signal.
trap "printf 'executing cleanup function\n'; cleanup" EXIT # Always cleanup when you're exiting.

# Configure interface listening on Node listener and cluster listener IPs.
# --------------------------------------------------------------------------------------------------------------------
# Create a dummy interface listening on the link-local IP and the cluster DNS service IP.
printf "setting up akslocaldns dummy interface with IPs %s and %s.\n" "${AKSLOCALDNS_NODE_LISTENER_IP}" "${AKSLOCALDNS_CLUSTER_LISTENER_IP}"
ip link add name akslocaldns type dummy
ip link set up dev akslocaldns
ip addr add ${AKSLOCALDNS_NODE_LISTENER_IP}/32 dev akslocaldns
ip addr add ${AKSLOCALDNS_CLUSTER_LISTENER_IP}/32 dev akslocaldns

# Add IPtables rules that skip conntrack for DNS connections coming from pods.
printf "adding iptables rules to skip conntrack for queries to akslocaldns.\n"
for RULE in "${IPTABLES_RULES[@]}"; do
    eval "${IPTABLES}" -A "${RULE}"
done

# Start akslocaldns.
# --------------------------------------------------------------------------------------------------------------------
COREDNS_COMMAND="${COREDNS_BINARY_PATH} -conf ${AKSLOCALDNS_CORE_FILE_PATH} -pidfile ${AKSLOCALDNS_PID_FILE}"
if [[ ! -z "${SYSTEMD_EXEC_PID:-}" ]]; then
    # We're running in systemd, so pass the coredns output via systemd-cat.
    COREDNS_COMMAND="systemd-cat --identifier=akslocaldns-coredns --stderr-priority=3 -- ${COREDNS_COMMAND}"
fi

printf "starting akslocaldns: %s.\n" "${COREDNS_COMMAND}"
rm -f ${AKSLOCALDNS_PID_FILE}
${COREDNS_COMMAND} &
until [ -f ${AKSLOCALDNS_PID_FILE} ]; do
    sleep 0.1
done
COREDNS_PID="$(cat ${AKSLOCALDNS_PID_FILE})"
printf "akslocaldns PID is %s.\n" "${COREDNS_PID}"

# Wait to direct traffic to akslocaldns until it's ready.
declare -i ATTEMPTS=0
printf "waiting for akslocaldns to start and be able to serve traffic.\n"
until [ "$(curl -s "http://${AKSLOCALDNS_NODE_LISTENER_IP}:8181/ready")" == "OK" ]; do
    if [ $ATTEMPTS -ge 60 ]; then
        printf "ERROR: akslocaldns failed to come online.\n"
        exit 255
    fi
    sleep 1
    ATTEMPTS+=1
done
printf "akslocaldns online and ready to serve node traffic.\n"

# Disable DNS from DHCP and point the system at akslocaldns.
# --------------------------------------------------------------------------------------------------------------------
printf "updating network DNS configuration to point to akslocaldns via %s.\n" "${NETWORK_DROPIN_FILE}"
mkdir -p ${NETWORK_DROPIN_DIR}
printf "[Network]\nDNS=%s\n\n[DHCP]\nUseDNS=false\n" "${AKSLOCALDNS_NODE_LISTENER_IP}" > "${NETWORK_DROPIN_FILE}"
chmod -R ugo+rX ${NETWORK_DROPIN_DIR}
networkctl reload
printf "startup complete - serving node and pod DNS traffic.\n"

# systemd notify: send ready if service is Type=notify.
# --------------------------------------------------------------------------------------------------------------------
if [[ ! -z "${NOTIFY_SOCKET:-}" ]]; then systemd-notify --ready; fi

# systemd watchdog: send pings so we get restarted if we go unhealthy.
# --------------------------------------------------------------------------------------------------------------------
# If the watchdog is defined, we check pod status and pass success to systemd.
if [[ ! -z "${NOTIFY_SOCKET:-}" && ! -z "${WATCHDOG_USEC:-}" ]]; then
    # Health check at 20% of WATCHDOG_USEC; this means that we should check
    # five times in every watchdog interval, and thus need to fail five checks to get restarted.
    HEALTH_CHECK_INTERVAL=$((${WATCHDOG_USEC:-5000000} * 20 / 100 / 1000000))
    HEALTH_CHECK_DNS_REQUEST=$'health-check.akslocaldns.local @'"${AKSLOCALDNS_NODE_LISTENER_IP}"$'\nhealth-check.akslocaldns.local @'"${AKSLOCALDNS_CLUSTER_LISTENER_IP}"
    printf "starting watchdog loop at %d second intervals.\n" "${HEALTH_CHECK_INTERVAL}"
    while 'true'; do
        if [[ "$(curl -s "http://${AKSLOCALDNS_NODE_LISTENER_IP}:8181/ready")" == "OK" ]]; then
            if dig +short +timeout=1 +tries=1 -f <(printf "%s" "${HEALTH_CHECK_DNS_REQUEST}"); then
                systemd-notify WATCHDOG=1
            fi
        fi
        sleep ${HEALTH_CHECK_INTERVAL}
    done
else
    wait -f ${COREDNS_PID}
fi

# The cleanup function is called on exit, so it will be run after the
# wait ends (which will be when a signal is sent or akslocaldns crashes) or the script receives a terminal signal.
# --------------------------------------------------------------------------------------------------------------------
# end of line