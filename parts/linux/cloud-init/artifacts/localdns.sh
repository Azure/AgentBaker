#! /bin/bash
set -euo pipefail

# localdns systemd unit.
# This systemd unit runs coredns as a caching with serve-stale functionality for both pod DNS and node DNS queries.
# It also upgrades to TCP for better reliability of upstream connections.

# Also defined in cse_helper file. Not sourcing the entire cse_helper file here.
# These exit codes will be handled in cse_config file.
ERR_LOCALDNS_FAIL=216 # Unable to start localdns systemd unit.
ERR_LOCALDNS_COREFILE_NOTFOUND=217 # Localdns corefile not found.
ERR_LOCALDNS_SLICEFILE_NOTFOUND=218 # Localdns slicefile not found.
ERR_LOCALDNS_BINARY_ERR=219 # Localdns binary not found or not executable.

# Global constants used in this file.
# -------------------------------------------------------------------------------------------------
# Localdns script path.
LOCALDNS_SCRIPT_PATH="/opt/azure/containers/localdns"

# Localdns corefile is created only when localdns profile has state enabled.
# This should match with 'path' defined in parts/linux/cloud-init/nodecustomdata.yml.
LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/localdns.corefile"

# This is the localdns corefile that has updated UpstreamDNSServerIPs and will be used by the localdns systemd unit.
UPDATED_LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/updated.localdns.corefile"

# This is slice file used by localdns systemd unit.
# This should match with 'path' defined in parts/linux/cloud-init/nodecustomdata.yml.
LOCALDNS_SLICE_PATH="/etc/systemd/system"
LOCALDNS_SLICE_FILE="${LOCALDNS_SLICE_PATH}/localdns.slice"

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

# Path to systemd resolv.
RESOLV_CONF="/run/systemd/resolve/resolv.conf"

# Curl check if localdns is running.
# This is used by start_localdns_watchdog and wait_for_localdns_ready.
CURL_COMMAND="curl -s http://${LOCALDNS_NODE_LISTENER_IP}:8181/ready"

# Constant for networkctl reload command.
# This is used by disable_dhcp_use_clusterlistener and cleanup_localdns_configs functions.
NETWORKCTL_RELOAD_CMD="networkctl reload"

# The health check is a DNS request to the localdns service IPs.
HEALTH_CHECK_DNS_REQUEST=$'health-check.localdns.local @'"${LOCALDNS_NODE_LISTENER_IP}"$'\nhealth-check.localdns.local @'"${LOCALDNS_CLUSTER_LISTENER_IP}"

START_LOCALDNS_TIMEOUT=10

# Function definitions used in this file.
# functions defined until "${__SOURCED__:+return}" are sourced and tested in -
# spec/parts/linux/cloud-init/artifacts/localdns_spec.sh.
# -------------------------------------------------------------------------------------------------
# Verify that the localdns corefile exists and is not empty.
verify_localdns_corefile() {
    if [ -z "${LOCALDNS_CORE_FILE:-}" ]; then
        echo "LOCALDNS_CORE_FILE is not set or is empty."
        return 1
    fi

    if [ ! -f "${LOCALDNS_CORE_FILE}" ] || [ ! -s "${LOCALDNS_CORE_FILE}" ]; then
        echo "Localdns corefile either does not exist or is empty at ${LOCALDNS_CORE_FILE}."
        return 1
    fi
    return 0
}

# Verify that the localdns slice file exists and is not empty.
verify_localdns_slicefile() {
    if [ -z "${LOCALDNS_SLICE_FILE:-}" ]; then
        echo "LOCALDNS_SLICE_FILE is not set or is empty."
        return 1
    fi

    if [ ! -f "${LOCALDNS_SLICE_FILE}" ] || [ ! -s "${LOCALDNS_SLICE_FILE}" ]; then
        echo "Localdns slice file does not exist at ${LOCALDNS_SLICE_FILE}."
        return 1
    fi
    return 0
}

# Verify that the localdns binary exists and is executable.
verify_localdns_binary() {
    if [ -z "${COREDNS_BINARY_PATH:-}" ]; then
        echo "COREDNS_BINARY_PATH is not set or is empty."
        return 1
    fi

    if [ ! -f "${COREDNS_BINARY_PATH}" ] || [ ! -x "${COREDNS_BINARY_PATH}" ]; then
        echo "Coredns binary either doesn't exist or isn't executable at ${COREDNS_BINARY_PATH}."
        return 1
    fi

    if ! "${COREDNS_BINARY_PATH}" --version >/dev/null 2>&1; then
        echo "Failed to execute '--version'."
        return 1
    fi
    return 0
}

# Replace AzureDNSIP in corefile with VNET DNS ServerIPs if necessary.
replace_azurednsip_in_corefile() {
    if [ -z "${RESOLV_CONF:-}" ]; then
        echo "RESOLV_CONF is not set or is empty."
        return 1
    fi

    if [ -z "${AZURE_DNS_IP:-}" ]; then
        echo "AZURE_DNS_IP is not set or is empty."
        return 1
    fi

    if [ ! -f "$RESOLV_CONF" ]; then
        echo "$RESOLV_CONF not found."
        return 1
    fi

    # Get the upstream VNET DNS servers from /run/systemd/resolve/resolv.conf.
    UPSTREAM_VNET_DNS_SERVERS=$(awk '/^nameserver/ {print $2}' "$RESOLV_CONF" | paste -sd' ')
    if [ -z "${UPSTREAM_VNET_DNS_SERVERS}" ] || [ "$UPSTREAM_VNET_DNS_SERVERS" = '""' ]; then
        echo "No Upstream VNET DNS servers found in $RESOLV_CONF."
        return 1
    fi
    echo "Found upstream VNET DNS servers: ${UPSTREAM_VNET_DNS_SERVERS}"

    # Based on customer input, corefile was generated in pkg/agent/baker.go.
    # Replace 168.63.129.16 with VNET DNS ServerIPs only if VNET DNS ServerIPs is not equal to 168.63.129.16
    # and also not equal to the localdns node listener IP to avoid creating a circular dependency.
    # Corefile will have 168.63.129.16 when user input has VnetDNS value for forwarddestination.
    # Note - For root domain under VnetDNSOverrides, all DNS traffic should be forwarded to VnetDNS.
    cp "${LOCALDNS_CORE_FILE}" "${UPDATED_LOCALDNS_CORE_FILE}" || {
        echo "Failed to copy ${LOCALDNS_CORE_FILE} to ${UPDATED_LOCALDNS_CORE_FILE}"
        return 1
    }

    if [ "${UPSTREAM_VNET_DNS_SERVERS}" != "${AZURE_DNS_IP}" ] && [ "${UPSTREAM_VNET_DNS_SERVERS}" != "${LOCALDNS_NODE_LISTENER_IP}" ]; then
        echo "Replacing Azure DNS IP ${AZURE_DNS_IP} with upstream VNET DNS servers ${UPSTREAM_VNET_DNS_SERVERS} in corefile ${UPDATED_LOCALDNS_CORE_FILE}"
        sed -i -e "s|${AZURE_DNS_IP}|${UPSTREAM_VNET_DNS_SERVERS}|g" "${UPDATED_LOCALDNS_CORE_FILE}" || {
            echo "Replacing AzureDNSIP in corefile failed."
            return 1
        }
        echo "Successfully updated ${UPDATED_LOCALDNS_CORE_FILE}"
    else
        echo "Skipping DNS IP replacement. Upstream VNET DNS servers (${UPSTREAM_VNET_DNS_SERVERS}) match either Azure DNS IP (${AZURE_DNS_IP}) or localdns node listener IP (${LOCALDNS_NODE_LISTENER_IP})"
    fi

    if [ ! -f "${UPDATED_LOCALDNS_CORE_FILE}" ] || [ ! -s "${UPDATED_LOCALDNS_CORE_FILE}" ]; then
        echo "Updated Localdns corefile either does not exist or is empty at ${UPDATED_LOCALDNS_CORE_FILE}."
        return 1
    fi

    chmod 0644 "${UPDATED_LOCALDNS_CORE_FILE}" || {
        echo "Failed to set permissions on ${UPDATED_LOCALDNS_CORE_FILE}"
        return 1
    }

    return 0
}

# Build iptables rules to skip conntrack for DNS traffic to localdns.
build_localdns_iptable_rules() {
    # These rules skip conntrack for DNS traffic to the local DNS service IPs to save conntrack table space.
    # OUTPUT rules affect node services and hostNetwork: true pods.
    # PREROUTING rules affect traffic from regular pods.
    # Loop over chains, IPs, and protocols to create the rules
    for CHAIN in OUTPUT PREROUTING; do
        for IP in ${LOCALDNS_NODE_LISTENER_IP} ${LOCALDNS_CLUSTER_LISTENER_IP}; do
            for PROTO in tcp udp; do
                # Add rule to IPTABLES_RULES array
                IPTABLES_RULES+=("${CHAIN} -p ${PROTO} -d ${IP} --dport 53 -j NOTRACK")
            done
        done
    done
}

# Verify that the default route interface is set and not empty.
verify_default_route_interface() {
    if [ -z "${DEFAULT_ROUTE_INTERFACE:-}" ]; then
        echo "Unable to determine the default route interface for ${AZURE_DNS_IP}."

        # Extracting default route interface using AzureDNSIP should work in most of the cases.
        # But if user or some process blackholes the route to AzureDNSIP, then -
        # Attempt to determine the default route interface using the upstream VNET DNS servers.
        # This will typically be eth0.
        VNET_DNS_SERVERS=$(awk '/^nameserver/ {print $2}' "$RESOLV_CONF" | paste -sd' ')

        # Extract first DNS server for route determination.
        FIRST_DNS_SERVER=$(echo "${VNET_DNS_SERVERS}" | awk '{print $1}')

        if [ -n "${FIRST_DNS_SERVER}" ] && [ "${FIRST_DNS_SERVER}" != "${LOCALDNS_NODE_LISTENER_IP}" ]; then
            echo "Using upstream VNET DNS server: ${FIRST_DNS_SERVER} to determine default route interface."
            DEFAULT_ROUTE_INTERFACE="$(ip -j route get "${FIRST_DNS_SERVER}" 2>/dev/null | jq -r 'if type == "array" and length > 0 then .[0].dev else empty end')"
        fi

        if [ -z "${DEFAULT_ROUTE_INTERFACE:-}" ]; then
            echo "Unable to determine the default route interface using fallback method with ${FIRST_DNS_SERVER}."
            return 1
        fi
    fi
    return 0
}

# Verify that the network file exists and is not empty.
verify_network_file() {
    if [ ! -f "${NETWORK_FILE:-}" ]; then
        echo "Unable to determine network file for interface."
        return 1
    fi
    return 0
}

# Verify that the network drop-in directory exists and is not empty.
verify_network_dropin_dir() {
    if [ -z "${NETWORK_DROPIN_DIR:-}" ]; then
        echo "NETWORK_DROPIN_DIR is not set or is empty."
        return 1
    fi

    if [ ! -d "${NETWORK_DROPIN_DIR}" ]; then
        echo "Network drop-in directory does not exist."
        return 1
    fi
    return 0
}

# Initialize network variables that may not be set during restarts or cleanup traps.
# This function can be called both during startup and in cleanup functions to ensure variables are available.
initialize_network_variables() {
    # Determine the default route interface if not already set.
    # This will typically be eth0.
    if [ -z "${DEFAULT_ROUTE_INTERFACE:-}" ]; then
        DEFAULT_ROUTE_INTERFACE="$(ip -j route get "${AZURE_DNS_IP}" 2>/dev/null | jq -r 'if type == "array" and length > 0 then .[0].dev else empty end')"
        if ! verify_default_route_interface; then
            echo "Failed to determine default route interface during variable initialization."
            return 1
        fi
    fi

    # Get the network file associated with the default route interface if not already set.
    # This will typically be /run/systemd/network/10-netplan-eth0.network.
    if [ -z "${NETWORK_FILE:-}" ]; then
        NETWORK_FILE="$(networkctl --json=short status "${DEFAULT_ROUTE_INTERFACE}" 2>/dev/null | jq -r '.NetworkFile')"
        if ! verify_network_file; then
            echo "Failed to determine network file during variable initialization."
            return 1
        fi
    fi

    # Get the network drop-in directory.
    # This will typically be /run/systemd/network/10-netplan-eth0.network.d
    if [ -z "${NETWORK_DROPIN_DIR:-}" ]; then
        NETWORK_DROPIN_DIR="/run/systemd/network/${NETWORK_FILE##*/}.d"
    fi

    # Set the network drop-in file path if not already set.
    # 70-localdns.conf file is written later in disable_dhcp_use_clusterlistener function.
    # disable_dhcp_use_clusterlistener function is called after the cleanup function.
    if [ -z "${NETWORK_DROPIN_FILE:-}" ]; then
        NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-localdns.conf"
    fi

    return 0
}

# Start localdns service.
start_localdns() {
    echo "Starting localdns: ${COREDNS_COMMAND}."
    rm -f "${LOCALDNS_PID_FILE}"
    # '&' is needed to put the command in the background.
    # This is because systemd will wait for the process to exit before continuing.
    # This is not what we want, as we want to run in the background.
    # The PID file will be created by coredns when it starts.
    # The PID file is used to track the process ID of coredns.
    # This is needed to send a SIGINT to coredns when we want to stop it.
    ${COREDNS_COMMAND} &

    # Wait until the PID file is created.
    local elapsed=0
    while [ ! -f "${LOCALDNS_PID_FILE}" ]; do
        sleep 1
        elapsed=$((elapsed + 1))
        if [ "$elapsed" -ge "$START_LOCALDNS_TIMEOUT" ]; then
            echo "Timed out waiting for CoreDNS to create PID file at ${LOCALDNS_PID_FILE}."
            return 1
        fi
    done

    COREDNS_PID="$(cat ${LOCALDNS_PID_FILE})"
    echo "Localdns PID is ${COREDNS_PID}."
    return 0
}

# Wait for localdns to be ready to serve traffic.
wait_for_localdns_ready() {
    local maxattempts=$1
    local timeout_duration=$2
    declare -i attempts=0
    local starttime=$(date +%s)

    echo "Waiting for localdns to start and be able to serve traffic."
    until [ "$($CURL_COMMAND)" = "OK" ]; do
        if [ $attempts -ge $maxattempts ]; then
            echo "Localdns failed to come online after $maxattempts attempts."
            return 1
        fi
        # Check for timeout based on elapsed time.
        currenttime=$(date +%s)
        elapsedtime=$((currenttime - starttime))
        if [ $elapsedtime -ge $timeout_duration ]; then
            echo "Localdns failed to come online after $timeout_duration seconds (timeout)."
            return 1
        fi
        sleep 1
        ((attempts++))
    done
    echo "Localdns is online and ready to serve traffic."
    return 0
}

# Add iptables rules to skip conntrack for DNS traffic to localdns.
add_iptable_rules_to_skip_conntrack_from_pods(){
    # Check if the localdns interface already exists and delete it.
    if ip link show localdns >/dev/null 2>&1; then
        echo "Interface localdns already exists, deleting it."
        ip link delete localdns
    fi

    ip link add name localdns type dummy
    ip link set up dev localdns
    ip addr add ${LOCALDNS_NODE_LISTENER_IP}/32 dev localdns
    ip addr add ${LOCALDNS_CLUSTER_LISTENER_IP}/32 dev localdns

    # Add IPtables rules that skip conntrack for DNS connections coming from pods.
    echo "Adding iptables rules to skip conntrack for queries to localdns."
    for RULE in "${IPTABLES_RULES[@]}"; do
        eval "${IPTABLES}" -A "${RULE}"
    done
}

# Disable DNS provided by DHCP and point the system at localdns.
disable_dhcp_use_clusterlistener() {
    mkdir -p "${NETWORK_DROPIN_DIR}"
    # verify that the network drop-in directory was created successfully.
    verify_network_dropin_dir || return 1

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

    eval "$NETWORKCTL_RELOAD_CMD"
    if [ "$?" -ne 0 ]; then
        echo "Failed to reload networkctl."
        return 1
    fi
    return 0
}

# Remove iptables rules and revert DNS configuration.
cleanup_iptables_and_dns() {
    # Ensure network variables are initialized if not already set.
    # This is needed here because this function can be called from cleanup traps or systemd restarts initiated by watchdog.
    if [ -z "${NETWORK_DROPIN_FILE:-}" ] || [ -z "${NETWORK_DROPIN_DIR:-}" ]; then
        echo "Network variables not initialized, attempting to determine them..."
        if ! initialize_network_variables; then
            echo "Failed to initialize network variables during cleanup."
            return 1
        fi
    fi

    # Remove any existing localdns iptables rules by searching for our comment.
    echo "Cleaning up any existing localdns iptables rules..."

    # Get list of existing localdns rules by searching for our comment.
    existing_rules=$(iptables -w -t raw -L --line-numbers -n | grep "localdns: skip conntrack" | awk '{print $1}' | sort -nr)

    if [ -n "$existing_rules" ]; then
        echo "Found existing localdns iptables rules, removing them..."
        failure_occurred=false
        for chain in OUTPUT PREROUTING; do
            # Get rule numbers for this chain and remove them (in reverse order to maintain line numbers)
            chain_rules=$(iptables -w -t raw -L "$chain" --line-numbers -n | grep "localdns: skip conntrack" | awk '{print $1}' | sort -nr)
            for rule_num in $chain_rules; do
                if iptables -w -t raw -D "$chain" "$rule_num" 2>/dev/null; then
                    echo "Successfully removed existing localdns iptables rule from $chain chain (rule $rule_num)."
                else
                    echo "Failed to remove existing localdns iptables rule from $chain chain (rule $rule_num)."
                    failure_occurred=true
                fi
            done
        done
        if [ "$failure_occurred" = true ]; then
            return 1
        fi
    else
        echo "No existing localdns iptables rules found."
    fi

    # Revert DNS configuration and network reload.
    echo "Removing network drop-in file ${NETWORK_DROPIN_FILE}."
    rm -f "$NETWORK_DROPIN_FILE"
    if [ "$?" -ne 0 ]; then
        echo "Failed to remove network drop-in file ${NETWORK_DROPIN_FILE}."
        return 1
    fi
    echo "Successfully removed network drop-in file."

    echo "Attempt to reload network configuration."
    eval "$NETWORKCTL_RELOAD_CMD"
    if [ "$?" -ne 0 ]; then
        echo "Failed to reload network after removing the DNS configuration."
        return 1
    fi
    echo "Reloading network configuration succeeded."

    return 0
}

# Cleanup function to remove localdns related configurations.
cleanup_localdns_configs() {
    # Disable error handling so that we don't get into a recursive loop.
    set +e

    # Remove iptables rules and revert DNS configuration
    cleanup_iptables_and_dns || return 1

    # Trigger localdns shutdown, if running.
    if [ ! -z "${COREDNS_PID:-}" ]; then
        # Check if the process exists by using `ps`.
        if ps -p "${COREDNS_PID}" >/dev/null 2>&1; then
            if [ "${LOCALDNS_SHUTDOWN_DELAY}" -gt 0 ]; then
                # Wait after removing iptables rules and DNS configuration so that we can let connections transition.
                echo "Sleeping ${LOCALDNS_SHUTDOWN_DELAY} seconds to allow connections to terminate."
                sleep "${LOCALDNS_SHUTDOWN_DELAY}"
            fi
            echo "Sending SIGINT to localdns and waiting for it to terminate."

            # Send SIGINT to localdns to trigger a graceful shutdown.
            kill -SIGINT "${COREDNS_PID}"
            kill_status=$?
            if [ "$kill_status" -eq 0 ]; then
                echo "Successfully sent SIGINT to localdns."
            else
                echo "Failed to send SIGINT to localdns. Exit status: $kill_status."
                return 1
            fi

            # Wait for the process to terminate.
            if wait "${COREDNS_PID}"; then
                echo "Localdns terminated successfully."
            else
                echo "Localdns failed to terminate properly."
                return 1
            fi
        fi
    fi

    # Delete the dummy interface if present.
    if ip link show dev localdns >/dev/null 2>&1; then
        echo "Removing localdns dummy interface."
        ip link del name localdns
        if [ "$?" -eq 0 ]; then
            echo "Successfully removed localdns dummy interface."
        else
            echo "Failed to remove localdns dummy interface."
            return 1
        fi
    fi

    # Indicate successful cleanup.
    echo "Successfully cleanup localdns related configurations."
    return 0
}

# Start the localdns watchdog.
# This function is used to check the health of localdns and restart it if necessary.
# It uses systemd-notify to send a watchdog ping to the systemd service manager.
# The watchdog interval is set in the systemd unit file and is passed to the script via the WATCHDOG_USEC environment variable.
# The health check is a DNS request to the localdns service IPs.
# The health check is run at 20% of the WATCHDOG_USEC interval.
# If the health check fails, the script will exit and systemd will restart the service.
start_localdns_watchdog() {
    if [ -n "${NOTIFY_SOCKET:-}" ] && [ -n "${WATCHDOG_USEC:-}" ]; then
        # Health check at 20% of WATCHDOG_USEC; this means that we should check.
        # five times in every watchdog interval, and thus need to fail five checks to get restarted.
        HEALTH_CHECK_INTERVAL=$((${WATCHDOG_USEC:-5000000} * 20 / 100 / 1000000))
        echo "Starting watchdog loop at ${HEALTH_CHECK_INTERVAL} second intervals."
        while true; do
            if [ "$($CURL_COMMAND)" = "OK" ] && dig +short +timeout=1 +tries=1 -f <(printf '%s\n' "$HEALTH_CHECK_DNS_REQUEST"); then
                systemd-notify WATCHDOG=1
            else
                echo "Localdns health check failed - will be restarted."
            fi
            sleep "${HEALTH_CHECK_INTERVAL}"
        done
    else
        wait "${COREDNS_PID}"
    fi
}

${__SOURCED__:+return}

# --------------------------------------- Main Execution starts here --------------------------------------------------

# Verify localdns required files exists.
# ---------------------------------------------------------------------------------------------------------------------
# Verify that generated corefile exists and is not empty.
verify_localdns_corefile || exit $ERR_LOCALDNS_COREFILE_NOTFOUND

# Verify that slice file exists and is not empty.
verify_localdns_slicefile || exit $ERR_LOCALDNS_SLICEFILE_NOTFOUND

# Verify that coredns binary is cached in VHD and is executable.
# Coredns binary is extracted from cached coredns image and pre-installed in the VHD -
# /opt/azure/containers/localdns/binary/coredns.
verify_localdns_binary || exit $ERR_LOCALDNS_BINARY_ERR

# Set required network environment variables.
# ---------------------------------------------------------------------------------------------------------------------
initialize_network_variables || exit $ERR_LOCALDNS_FAIL

# Clean up any existing iptables rules and DNS configuration before starting.
# ---------------------------------------------------------------------------------------------------------------------
cleanup_iptables_and_dns || exit $ERR_LOCALDNS_FAIL

# Replace AzureDNSIP in corefile with VNET DNS ServerIPs.
# ---------------------------------------------------------------------------------------------------------------------
replace_azurednsip_in_corefile || exit $ERR_LOCALDNS_FAIL

# Build IPtable rules.
# ---------------------------------------------------------------------------------------------------------------------
IPTABLES='iptables -w -t raw -m comment --comment "localdns: skip conntrack"'
IPTABLES_RULES=()
build_localdns_iptable_rules

# Setup traps to trigger cleanup_localdns_configs if anything goes wrong.
# ---------------------------------------------------------------------------------------------------------------------
# cleanup_localdns_configs function will be run on script exit/crash to revert config.
# Ensure cleanup runs before exiting on an error.
trap 'echo "Error occurred. Cleaning up..."; cleanup_localdns_configs; exit $ERR_LOCALDNS_FAIL' ABRT ERR INT PIPE

# Always cleanup when exiting.
trap 'echo "Executing cleanup function."; cleanup_localdns_configs || echo "Cleanup failed with error code: $ERR_LOCALDNS_FAIL."' EXIT

# Configure interface listening on Node listener and cluster listener IPs.
# --------------------------------------------------------------------------------------------------------------------
# Create a dummy interface listening on the link-local IP and cluster DNS service IP.
echo "Setting up localdns dummy interface with IPs ${LOCALDNS_NODE_LISTENER_IP} and ${LOCALDNS_CLUSTER_LISTENER_IP}."
add_iptable_rules_to_skip_conntrack_from_pods

# Start localdns service.
# --------------------------------------------------------------------------------------------------------------------
COREDNS_COMMAND="${COREDNS_BINARY_PATH} -conf ${UPDATED_LOCALDNS_CORE_FILE} -pidfile ${LOCALDNS_PID_FILE}"
if [ -n "${SYSTEMD_EXEC_PID:-}" ]; then
    # We're running in systemd, so pass the coredns output via systemd-cat.
    COREDNS_COMMAND="systemd-cat --identifier=localdns-coredns --stderr-priority=3 -- ${COREDNS_COMMAND}"
fi
# Start localdns.
start_localdns || exit $ERR_LOCALDNS_FAIL

# Wait to direct traffic to localdns until it's ready.
wait_for_localdns_ready 60 60 || exit $ERR_LOCALDNS_FAIL

# Disable DNS from DHCP and point the system at localdns.
# --------------------------------------------------------------------------------------------------------------------
echo "Updating network DNS configuration to point to localdns via ${NETWORK_DROPIN_FILE}."
disable_dhcp_use_clusterlistener || exit $ERR_LOCALDNS_FAIL
echo "Startup complete - serving node and pod DNS traffic."

# Systemd notify: send ready if service is Type=notify.
# --------------------------------------------------------------------------------------------------------------------
if [ -n "${NOTIFY_SOCKET:-}" ]; then
   systemd-notify --ready
fi

# Systemd watchdog: send pings so we get restarted if we go unhealthy.
# --------------------------------------------------------------------------------------------------------------------
# If the watchdog is defined, we check status and pass success to systemd.
start_localdns_watchdog

# The cleanup function is called on exit, so it will be run after the
# wait ends (which will be when a signal is sent or localdns crashes) or the script receives a terminal signal.
# --------------------------------------- Main execution ends here ---------------------------------------------------
# end of line
