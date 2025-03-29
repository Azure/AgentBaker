#! /bin/bash
set -euo pipefail

# localdns systemd unit.
# --------------------------------------------------------------------------------------------------------------------
# This systemd unit runs coredns as a caching with serve-stale functionality for both pod DNS and node DNS queries. 
# It also upgrades to TCP for better reliability of upstream connections.

LOCALDNS_SCRIPT_PATH="/opt/azure/containers/localdns"
AZURE_DNS_IP="168.63.129.16"
LOCALDNS_SHUTDOWN_DELAY=5
LOCALDNS_PID_FILE="/run/localdns.pid"

# Verify the required files exists.
# --------------------------------------------------------------------------------------------------------------------
# This file contains the environment variables used by localdns systemd unit.
# This path should match with 'path' defined in parts/linux/cloud-init/nodecustomdata.yml.
LOCALDNS_ENV_FILE="/etc/default/localdns.envfile"
if [ -f "${LOCALDNS_ENV_FILE}" ]; then
    source "${LOCALDNS_ENV_FILE}"
else
    printf "Error: localdns envfile does not exist at %s.\n" "${LOCALDNS_ENV_FILE}"
    exit 1
fi

# This file contains generated Corefile used by localdns systemd unit.
# This should match with 'path' defined in parts/linux/cloud-init/nodecustomdata.yml.
LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/localdns.corefile"
if [ ! -f "${LOCALDNS_CORE_FILE}" ] || [ ! -s "${LOCALDNS_CORE_FILE}" ]; then
    printf "Error: localdns corefile either does not exist or is empty at %s.\n" "${LOCALDNS_CORE_FILE}"
    exit 1
fi

# This is slice file used by localdns systemd unit.
# This should match with 'path' defined in parts/linux/cloud-init/nodecustomdata.yml.
LOCALDNS_SLICE_PATH="/etc/systemd/system/localdns.slice"
if [ ! -f "${LOCALDNS_SLICE_PATH}" ]; then
    printf "Error: localdns slice file does not exist at %s.\n" "${LOCALDNS_SLICE_PATH}"
    exit 1
fi

# Load environment variables from envfile.
# Each of the below variables must match the field name defined in parts/linux/cloud-init/nodecustomdata.yml.
# --------------------------------------------------------------------------------------------------------------------
# This is the IP that localdns service should bind to for node traffic; an APIPA address.
: "${LOCALDNS_NODE_LISTENER_IP:?LOCALDNS_NODE_LISTENER_IP is not set}"
# This is the IP that localdns service should bind to for pod traffic; an APIPA address.
: "${LOCALDNS_CLUSTER_LISTENER_IP:?LOCALDNS_CLUSTER_LISTENER_IP is not set}"

# Check if coredns binary is cached in VHD.
# --------------------------------------------------------------------------------------------------------------------
# Coredns binary is extracted from cached coredns image and pre-installed in the VHD -
# /opt/azure/containers/localdns/binary/coredns.
COREDNS_BINARY_PATH="${LOCALDNS_SCRIPT_PATH}/binary/coredns"
if [ ! -x "${COREDNS_BINARY_PATH}" ]; then
    printf "Error: coredns binary not found at %s.\n" "${COREDNS_BINARY_PATH}"
    exit 1
fi

# Check if --plugins command runs successfully.
builtInPlugins=$("${COREDNS_BINARY_PATH}" --plugins)
if [ $? -ne 0 ]; then
    printf "Error: Failed to execute '%s --plugins'.\n" "${COREDNS_BINARY_PATH}"
    exit 1
fi

# Replace Vnet_DNS_Server in corefile with VNET DNS Server IPs.
# --------------------------------------------------------------------------------------------------------------------
UPSTREAM_VNET_DNS_SERVERS=$(awk '/nameserver/ {print $2}' /run/systemd/resolve/resolv.conf | paste -sd' ')
# Get the upstream VNET DNS servers from /run/systemd/resolve/resolv.conf.
if [[ -z "${UPSTREAM_VNET_DNS_SERVERS}" ]]; then
    printf "Error: No Upstream VNET DNS servers found in /run/systemd/resolve/resolv.conf.\n"
    exit 1
fi

# Based on customer input, corefile was generated in pkg/agent/baker.go.
# Replace 168.63.129.16 with VNET DNS ServerIPs only if VNET DNS ServerIPs is not equal to 168.63.129.16.
if [[ "${UPSTREAM_VNET_DNS_SERVERS}" != "${AZURE_DNS_IP}" ]]; then
    sed -i -e "s|168.63.129.16|${UPSTREAM_VNET_DNS_SERVERS}|g" "${LOCALDNS_CORE_FILE}" || {
        echo "Error: updating corefile failed"
        exit 1
    }
fi
cat "${LOCALDNS_CORE_FILE}"

# Iptables: build rules.
# --------------------------------------------------------------------------------------------------------------------
# These rules skip conntrack for DNS traffic to the local DNS service IPs to save conntrack table space.
# OUTPUT rules affect node services and hostNetwork: true pods.
# PREROUTING rules affect traffic from regular pods.
IPTABLES='iptables -w -t raw -m comment --comment "Local DNS: skip conntrack"'
IPTABLES_RULES=()
for CHAIN in OUTPUT PREROUTING; do
for IP in ${LOCALDNS_NODE_LISTENER_IP} ${LOCALDNS_CLUSTER_LISTENER_IP}; do
for PROTO in tcp udp; do
    IPTABLES_RULES+=("${CHAIN} -p ${PROTO} -d ${IP} --dport 53 -j NOTRACK")
done; done; done

# Information variables.
# --------------------------------------------------------------------------------------------------------------------
DEFAULT_ROUTE_INTERFACE="$(ip -j route get "${AZURE_DNS_IP}" | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status "${DEFAULT_ROUTE_INTERFACE}" | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-localdns.conf"

# Cleanup function will be run on script exit/crash to revert config.
# --------------------------------------------------------------------------------------------------------------------
function cleanup {
    # Disable error handling so that we don't get into a recursive loop.
    set +e

    # Remove iptables rules to stop forwarding DNS traffic.
    for RULE in "${IPTABLES_RULES[@]}"; do
        if eval "${IPTABLES}" -C "${RULE}" 2>/dev/null; then
            eval "${IPTABLES}" -D "${RULE}"
            if [ $? -eq 0 ]; then
                printf "Successfully removed iptables rule: %s.\n" "${RULE}"
            else
                printf "Failed to remove iptables rule: %s.\n" "${RULE}"
                return 1
            fi
        fi
    done

    # Revert the changes made to the DNS configuration if present.
    if [ -f ${NETWORK_DROPIN_FILE} ]; then
        printf "Reverting DNS configuration by removing %s.\n" "${NETWORK_DROPIN_FILE}"
        /bin/rm -f ${NETWORK_DROPIN_FILE}
        if [ $? -eq 0 ]; then
            networkctl reload
            if [ $? -ne 0 ]; then
                printf "Failed to reload network after removing the DNS configuration.\n"
                return 1
            fi
        else
            printf "Failed to remove %s.\n" "${NETWORK_DROPIN_FILE}"
            return 1
        fi
    fi

    # Trigger localdns shutdown, if running.
    if [ ! -z "${COREDNS_PID:-}" ]; then
        if ps ${COREDNS_PID} >/dev/null; then
            if [[ ${LOCALDNS_SHUTDOWN_DELAY} -gt 0 ]]; then
                # Wait after removing iptables rules and DNS configuration so that we can let connections transition.
                printf "sleeping %d seconds to allow connections to terminate.\n" "${LOCALDNS_SHUTDOWN_DELAY}"
                sleep ${LOCALDNS_SHUTDOWN_DELAY}
            fi
            printf "sending SIGINT to localdns and waiting for it to terminate.\n"

            # Send SIGINT to localdns to trigger shutdown.
            kill -SIGINT ${COREDNS_PID}
            if [ $? -eq 0 ]; then
                printf "Successfully sent SIGINT to localdns.\n"
            else
                printf "Failed to send SIGINT to localdns.\n"
                return 1
            fi

            # Wait for localdns to shut down.
            wait ${COREDNS_PID}
            if [ $? -eq 0 ]; then
                printf "localdns terminated successfully.\n"
            else
                printf "localdns failed to terminate properly.\n"
                return 1
            fi
        fi
    fi

    # Delete the dummy interface if present.
    if ip link show dev localdns >/dev/null 2>&1; then
        printf "removing localdns dummy interface.\n"
        ip link del name localdns
        if [ $? -eq 0 ]; then
            printf "Successfully removed localdns dummy interface.\n"
        else
            printf "Failed to remove localdns dummy interface.\n"
            return 1
        fi
    fi

    # Indicate successful cleanup
    printf "Successfully cleanup localdns related configurations.\n"
    return 0
}

# Enable the cleanup function now that we have a coredns binary.
trap "exit 0" QUIT TERM                                    # Exit with code 0 on a successful shutdown.
trap "exit 1" ABRT ERR INT PIPE                            # Exit with code 1 on a bad signal.
# Always cleanup when exiting.
trap 'printf "executing cleanup function.\n"
cleanup || printf "Cleanup failed with error code: %d.\n" $?' EXIT

# Configure interface listening on Node listener and cluster listener IPs.
# --------------------------------------------------------------------------------------------------------------------
# Create a dummy interface listening on the link-local IP and the cluster DNS service IP.
printf "setting up localdns dummy interface with IPs %s and %s.\n" "${LOCALDNS_NODE_LISTENER_IP}" "${LOCALDNS_CLUSTER_LISTENER_IP}"
ip link add name localdns type dummy
ip link set up dev localdns
ip addr add ${LOCALDNS_NODE_LISTENER_IP}/32 dev localdns
ip addr add ${LOCALDNS_CLUSTER_LISTENER_IP}/32 dev localdns

# Add IPtables rules that skip conntrack for DNS connections coming from pods.
printf "adding iptables rules to skip conntrack for queries to localdns.\n"
for RULE in "${IPTABLES_RULES[@]}"; do
    eval "${IPTABLES}" -A "${RULE}"
done

# Start localdns.
# --------------------------------------------------------------------------------------------------------------------
COREDNS_COMMAND="${COREDNS_BINARY_PATH} -conf ${LOCALDNS_CORE_FILE} -pidfile ${LOCALDNS_PID_FILE}"
if [[ -n "${SYSTEMD_EXEC_PID:-}" ]]; then
    # We're running in systemd, so pass the coredns output via systemd-cat.
    COREDNS_COMMAND="systemd-cat --identifier=localdns-coredns --stderr-priority=3 -- ${COREDNS_COMMAND}"
fi

printf "starting localdns: %s.\n" "${COREDNS_COMMAND}"
rm -f "${LOCALDNS_PID_FILE}"
${COREDNS_COMMAND} &

# Wait until the PID file is created.
until [ -f "${LOCALDNS_PID_FILE}" ]; do
    sleep 0.1
done

COREDNS_PID="$(cat ${LOCALDNS_PID_FILE})"
printf "localdns PID is %s.\n" "${COREDNS_PID}"

# Wait to direct traffic to localdns until it's ready.
declare -i ATTEMPTS=0
MAX_ATTEMPTS=60
TIMEOUT=60
START_TIME=$(date +%s)

printf "waiting for localdns to start and be able to serve traffic.\n"
until [ "$(curl -s "http://${LOCALDNS_NODE_LISTENER_IP}:8181/ready")" == "OK" ]; do
    if [ $ATTEMPTS -ge $MAX_ATTEMPTS ]; then
        printf "ERROR: localdns failed to come online after %d attempts.\n" "$MAX_ATTEMPTS"
        exit 255
    fi
    # Check for timeout based on elapsed time.
    CURRENT_TIME=$(date +%s)
    ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
    if [ $ELAPSED_TIME -ge $TIMEOUT ]; then
        printf "ERROR: localdns failed to come online after %d seconds (timeout).\n" "$TIMEOUT"
        exit 255
    fi
    sleep 1
    ATTEMPTS+=1
done
printf "localdns is online and ready to serve traffic.\n"

# Disable DNS from DHCP and point the system at localdns.
# --------------------------------------------------------------------------------------------------------------------
printf "updating network DNS configuration to point to localdns via %s.\n" "${NETWORK_DROPIN_FILE}"
mkdir -p ${NETWORK_DROPIN_DIR}
printf "[Network]\nDNS=%s\n\n[DHCP]\nUseDNS=false\n" "${LOCALDNS_NODE_LISTENER_IP}" > "${NETWORK_DROPIN_FILE}"
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
    HEALTH_CHECK_DNS_REQUEST=$'health-check.localdns.local @'"${LOCALDNS_NODE_LISTENER_IP}"$'\nhealth-check.localdns.local @'"${LOCALDNS_CLUSTER_LISTENER_IP}"
    printf "starting watchdog loop at %d second intervals.\n" "${HEALTH_CHECK_INTERVAL}"
    while 'true'; do
        if [[ "$(curl -s "http://${LOCALDNS_NODE_LISTENER_IP}:8181/ready")" == "OK" ]]; then
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
# wait ends (which will be when a signal is sent or localdns crashes) or the script receives a terminal signal.
# --------------------------------------------------------------------------------------------------------------------
# end of line