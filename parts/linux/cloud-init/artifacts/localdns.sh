#! /bin/bash
set -euo pipefail

# localdns systemd unit.
# --------------------------------------------------------------------------------------------------------------------
# This systemd unit runs coredns as a caching with serve-stale functionality for both pod DNS and node DNS queries.
# It also upgrades to TCP for better reliability of upstream connections.

# Localdns script path.
LOCALDNS_SCRIPT_PATH="/opt/azure/containers/localdns"
# Localdns corefile is created only when localdns profile has state enabled.
# This should match with 'path' defined in parts/linux/cloud-init/nodecustomdata.yml.
LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/localdns.corefile"
# This is slice file used by localdns systemd unit.
# This should match with 'path' defined in parts/linux/cloud-init/nodecustomdata.yml.
LOCALDNS_SLICE_PATH="/etc/systemd/system/localdns.slice"
# Azure DNS IP.
AZURE_DNS_IP="168.63.129.16"
# Localdns node listener IP.
LOCALDNS_NODE_LISTENER_IP="169.254.10.10"
# Localdns cluster listener IP.
LOCALDNS_CLUSTER_LISTENER_IP="169.254.10.11"
# Localdns shutdown delay.
LOCALDNS_SHUTDOWN_DELAY=5
# Localdns pid file.
LOCALDNS_PID_FILE="/run/localdns.pid"
# Path of coredns binary used by localdns.
COREDNS_BINARY_PATH="${LOCALDNS_SCRIPT_PATH}/binary/coredns"

# Also defined in cse_helper file. Not sourcing the entire cse_helper file here.
# These exit codes will be handled in cse_config file.
ERR_LOCALDNS_FAIL=216 # Unable to start localdns systemd unit.
ERR_LOCALDNS_COREFILE_NOTFOUND=217 # Localdns corefile not found.
ERR_LOCALDNS_SLICEFILE_NOTFOUND=218 # Localdns slicefile not found.
ERR_LOCALDNS_BINARY_NOTFOUND=219 # Localdns binary not found.

# Verify the required files exists.
# --------------------------------------------------------------------------------------------------------------------
# This file contains generated corefile used by localdns systemd unit.
if [ ! -f "${LOCALDNS_CORE_FILE}" ] || [ ! -s "${LOCALDNS_CORE_FILE}" ]; then
    printf "Localdns corefile either does not exist or is empty at %s.\n" "${LOCALDNS_CORE_FILE}"
    exit $ERR_LOCALDNS_COREFILE_NOTFOUND
fi

# This is slice file used by localdns systemd unit.
if [ ! -f "${LOCALDNS_SLICE_PATH}" ]; then
    printf "Localdns slice file does not exist at %s.\n" "${LOCALDNS_SLICE_PATH}"
    exit $ERR_LOCALDNS_SLICEFILE_NOTFOUND
fi

# Check if coredns binary is cached in VHD.
# --------------------------------------------------------------------------------------------------------------------
# Coredns binary is extracted from cached coredns image and pre-installed in the VHD -
# /opt/azure/containers/localdns/binary/coredns.
if [[ ! -f "${COREDNS_BINARY_PATH}" || ! -x "${COREDNS_BINARY_PATH}" ]]; then
    printf "Coredns binary either doesn't exist or isn't executable %s.\n" "${COREDNS_BINARY_PATH}"
    exit $ERR_LOCALDNS_BINARY_NOTFOUND
fi

# Check if --plugins command runs successfully.
builtInPlugins=$("${COREDNS_BINARY_PATH}" --plugins)
if [ $? -ne 0 ]; then
    printf "Failed to execute '%s --plugins'.\n" "${COREDNS_BINARY_PATH}"
    exit $ERR_LOCALDNS_FAIL
fi

# Replace Vnet_DNS_Server in corefile with VNET DNS Server IPs.
# --------------------------------------------------------------------------------------------------------------------
UPSTREAM_VNET_DNS_SERVERS=$(awk '/nameserver/ {print $2}' /run/systemd/resolve/resolv.conf | paste -sd' ')
# Get the upstream VNET DNS servers from /run/systemd/resolve/resolv.conf.
if [[ -z "${UPSTREAM_VNET_DNS_SERVERS}" ]]; then
    printf "No Upstream VNET DNS servers found in /run/systemd/resolve/resolv.conf.\n"
    exit $ERR_LOCALDNS_FAIL
fi

# Based on customer input, corefile was generated in pkg/agent/baker.go.
# Replace 168.63.129.16 with VNET DNS ServerIPs only if VNET DNS ServerIPs is not equal to 168.63.129.16.
# Corefile will have 168.63.129.16 when user input has VnetDNS value for forwarddestination. 
# Note - For root domain under VnetDNSOverrides, all DNS traffic should be forwarded to VnetDNS.
if [[ "${UPSTREAM_VNET_DNS_SERVERS}" != "${AZURE_DNS_IP}" ]]; then
    sed -i -e "s|${AZURE_DNS_IP}|${UPSTREAM_VNET_DNS_SERVERS}|g" "${LOCALDNS_CORE_FILE}" || {
        printf "Updating corefile failed"
        exit $ERR_LOCALDNS_FAIL
    }
fi
cat "${LOCALDNS_CORE_FILE}"

# Iptables: build rules.
# --------------------------------------------------------------------------------------------------------------------
# These rules skip conntrack for DNS traffic to the local DNS service IPs to save conntrack table space.
# OUTPUT rules affect node services and hostNetwork: true pods.
# PREROUTING rules affect traffic from regular pods.
IPTABLES='iptables -w -t raw -m comment --comment "localdns: skip conntrack"'
IPTABLES_RULES=()

# Loop over chains, IPs, and protocols to create the rules
for CHAIN in OUTPUT PREROUTING; do
    for IP in ${LOCALDNS_NODE_LISTENER_IP} ${LOCALDNS_CLUSTER_LISTENER_IP}; do
        for PROTO in tcp udp; do
            # Add rule to IPTABLES_RULES array
            IPTABLES_RULES+=("${CHAIN} -p ${PROTO} -d ${IP} --dport 53 -j NOTRACK")
        done
    done
done

# Information variables.
# --------------------------------------------------------------------------------------------------------------------
# Get default route interface for the given AZURE_DNS_IP.
DEFAULT_ROUTE_INTERFACE="$(ip -j route get "${AZURE_DNS_IP}" 2>/dev/null | jq -r 'if type == "array" and length > 0 then .[0].dev else empty end')"
if [[ -z "${DEFAULT_ROUTE_INTERFACE}" ]]; then
    echo "Unable to determine the default route interface for ${AZURE_DNS_IP}."
    exit $ERR_LOCALDNS_FAIL
fi

# Get the network file associated with the default route interface.
NETWORK_FILE="$(networkctl --json=short status "${DEFAULT_ROUTE_INTERFACE}" 2>/dev/null | jq -r '.NetworkFile')"
if [[ -z "${NETWORK_FILE}" ]]; then
    echo "Unable to determine network file for interface ${DEFAULT_ROUTE_INTERFACE}."
    exit $ERR_LOCALDNS_FAIL
fi

# Check and create the drop-in directory if it does not exist.
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
                return $ERR_LOCALDNS_FAIL
            fi
        fi
    done

    # Revert the changes made to the DNS configuration if present.
    if [ -f "${NETWORK_DROPIN_FILE}" ]; then
        printf "Reverting DNS configuration by removing %s.\n" "${NETWORK_DROPIN_FILE}"
        if /bin/rm -f "${NETWORK_DROPIN_FILE}"; then
            networkctl reload || {
                printf "Failed to reload network after removing the DNS configuration.\n"
                return $ERR_LOCALDNS_FAIL
            }
        else
            printf "Failed to remove %s.\n" "${NETWORK_DROPIN_FILE}"
            return $ERR_LOCALDNS_FAIL
        fi
    fi

    # Trigger localdns shutdown, if running.
    # COREDNS_PID should be non-empty and a valid PID number.
    if [[ -n "${COREDNS_PID}" ]] && [[ "${COREDNS_PID}" =~ ^[0-9]+$ ]]; then
        # The kill -0 command checks if the process exists. If it doesn't exist, this condition will fail,
        # and the script will exit without trying to kill the process.
        if kill -0 "${COREDNS_PID}" 2>/dev/null; then
            if [[ "${LOCALDNS_SHUTDOWN_DELAY}" -gt 0 ]]; then
                # Wait after removing iptables rules and DNS configuration so that we can let connections transition.
                printf "Sleeping %d seconds to allow connections to terminate.\n" "${LOCALDNS_SHUTDOWN_DELAY}"
                sleep "${LOCALDNS_SHUTDOWN_DELAY}"
            fi
            printf "Sending SIGINT to localdns and waiting for it to terminate.\n"

            # Send SIGINT to localdns to trigger a graceful shutdown.
            kill -SIGINT "${COREDNS_PID}"
            kill_status=$?
            if [ $kill_status -eq 0 ]; then
                printf "Successfully sent SIGINT to localdns.\n"
            else
                printf "Failed to send SIGINT to localdns. Exit status: %s.\n" "$kill_status"
                return $ERR_LOCALDNS_FAIL
            fi

            # Wait for the process to terminate.
            if wait "${COREDNS_PID}"; then
                printf "Localdns terminated successfully.\n"
            else
                printf "Localdns failed to terminate properly.\n"
                return "$ERR_LOCALDNS_FAIL"
            fi
        fi
    fi

    # Delete the dummy interface if present.
    if ip link show dev localdns >/dev/null 2>&1; then
        printf "Removing localdns dummy interface.\n"
        ip link del name localdns
        if [ $? -eq 0 ]; then
            printf "Successfully removed localdns dummy interface.\n"
        else
            printf "Failed to remove localdns dummy interface.\n"
            return $ERR_LOCALDNS_FAIL
        fi
    fi

    # Indicate successful cleanup.
    printf "Successfully cleanup localdns related configurations.\n"
    return 0
}

# Ensure cleanup runs before exiting on an error.
trap 'printf "Error occurred. Cleaning up...\n"; cleanup; exit $ERR_LOCALDNS_FAIL' ABRT ERR INT PIPE

# Always cleanup when exiting.
trap 'printf "Executing cleanup function.\n"; cleanup || printf "Cleanup failed with error code: %d.\n" $?' EXIT


# Configure interface listening on Node listener and cluster listener IPs.
# --------------------------------------------------------------------------------------------------------------------
# Create a dummy interface listening on the link-local IP and the cluster DNS service IP.
printf "Setting up localdns dummy interface with IPs %s and %s.\n" "${LOCALDNS_NODE_LISTENER_IP}" "${LOCALDNS_CLUSTER_LISTENER_IP}"
ip link add name localdns type dummy
ip link set up dev localdns
ip addr add ${LOCALDNS_NODE_LISTENER_IP}/32 dev localdns
ip addr add ${LOCALDNS_CLUSTER_LISTENER_IP}/32 dev localdns

# Add IPtables rules that skip conntrack for DNS connections coming from pods.
printf "Adding iptables rules to skip conntrack for queries to localdns.\n"
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

printf "Starting localdns: %s.\n" "${COREDNS_COMMAND}"
rm -f "${LOCALDNS_PID_FILE}"
${COREDNS_COMMAND} &

# Wait until the PID file is created.
until [ -f "${LOCALDNS_PID_FILE}" ]; do
    sleep 0.1
done

COREDNS_PID="$(cat ${LOCALDNS_PID_FILE})"
printf "Localdns PID is %s.\n" "${COREDNS_PID}"

# Wait to direct traffic to localdns until it's ready.
declare -i ATTEMPTS=0
MAX_ATTEMPTS=60
TIMEOUT=60
START_TIME=$(date +%s)

printf "Waiting for localdns to start and be able to serve traffic.\n"
until [ "$(curl -s "http://${LOCALDNS_NODE_LISTENER_IP}:8181/ready")" == "OK" ]; do
    if [ $ATTEMPTS -ge $MAX_ATTEMPTS ]; then
        printf "Localdns failed to come online after %d attempts.\n" "$MAX_ATTEMPTS"
        exit $ERR_LOCALDNS_FAIL
    fi
    # Check for timeout based on elapsed time.
    CURRENT_TIME=$(date +%s)
    ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
    if [ $ELAPSED_TIME -ge $TIMEOUT ]; then
        printf "Localdns failed to come online after %d seconds (timeout).\n" "$TIMEOUT"
        exit $ERR_LOCALDNS_FAIL
    fi
    sleep 1
    ((ATTEMPTS++))
done
printf "Localdns is online and ready to serve traffic.\n"

# Disable DNS from DHCP and point the system at localdns.
# --------------------------------------------------------------------------------------------------------------------
printf "Updating network DNS configuration to point to localdns via %s.\n" "${NETWORK_DROPIN_FILE}"
mkdir -p "${NETWORK_DROPIN_DIR}"

cat > "${NETWORK_DROPIN_FILE}" <<EOF
# Set DNS server to localdns cluster listernerIP.
[Network]
DNS=${LOCALDNS_NODE_LISTENER_IP}

# Disable DNS provided by DHCP to ensure local DNS is used.
[DHCP]
UseDNS=false
EOF

# Set permissions on the drop-in directory and file.
chmod -R ugo+rX "${NETWORK_DROPIN_DIR}"

networkctl reload
if [[ $? -ne 0 ]]; then
    echo "Error: Failed to reload networkctl."
    exit $ERR_LOCALDNS_FAIL
fi

printf "Startup complete - serving node and pod DNS traffic.\n"

# systemd notify: send ready if service is Type=notify.
# --------------------------------------------------------------------------------------------------------------------
if [[ -n "${NOTIFY_SOCKET:-}" ]]; then 
   systemd-notify --ready 
fi

# systemd watchdog: send pings so we get restarted if we go unhealthy.
# --------------------------------------------------------------------------------------------------------------------
# If the watchdog is defined, we check pod status and pass success to systemd.
if [[ -n "${NOTIFY_SOCKET:-}" && -n "${WATCHDOG_USEC:-}" ]]; then
    # Health check at 20% of WATCHDOG_USEC; this means that we should check
    # five times in every watchdog interval, and thus need to fail five checks to get restarted.
    HEALTH_CHECK_INTERVAL=$((${WATCHDOG_USEC:-5000000} * 20 / 100 / 1000000))
    HEALTH_CHECK_DNS_REQUEST=$'health-check.localdns.local @'"${LOCALDNS_NODE_LISTENER_IP}"$'\nhealth-check.localdns.local @'"${LOCALDNS_CLUSTER_LISTENER_IP}"
    printf "Starting watchdog loop at %d second intervals.\n" "${HEALTH_CHECK_INTERVAL}"
    while true; do
        if [[ "$(curl -s "http://${LOCALDNS_NODE_LISTENER_IP}:8181/ready")" == "OK" ]]; then
            if echo -e "${HEALTH_CHECK_DNS_REQUEST}" | dig +short +timeout=1 +tries=1; then
                systemd-notify WATCHDOG=1
            fi
        fi
        sleep ${HEALTH_CHECK_INTERVAL}
    done
else
    wait ${COREDNS_PID}
fi

# The cleanup function is called on exit, so it will be run after the
# wait ends (which will be when a signal is sent or localdns crashes) or the script receives a terminal signal.
# --------------------------------------------------------------------------------------------------------------------
# end of line