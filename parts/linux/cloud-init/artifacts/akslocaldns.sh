#! /bin/bash
set -euo pipefail

# akslocaldns systemd unit.
# --------------------------------------------------------------------------------------------------------------------
# This systemd unit runs coredns as a caching with serve-stale functionality for both pod DNS and node DNS queries. 
# It also upgrades to TCP for better reliability of upstream connections.

# Load environment variables from the env file.
# --------------------------------------------------------------------------------------------------------------------
# This should match with 'AKSLOCALDNS_ENV_FILE' defined in parts/linux/cloud-init/artifacts/cse_config.sh.
AKSLOCALDNS_ENV_FILE_PATH="/etc/default/akslocaldns/akslocaldns.envfile"
if [ -f "${AKSLOCALDNS_ENV_FILE_PATH}" ]; then
    source "${AKSLOCALDNS_ENV_FILE_PATH}"
else
    printf "Error: akslocaldns envfile does not exist at %s.\n" "${AKSLOCALDNS_ENV_FILE_PATH}"
    exit 1
fi

# CoreDNS image reference to use to obtain the binary if not present.
: "${AKSLOCALDNS_IMAGE_URL:?Required parameter AKSLOCALDNS_IMAGE_URL is not set}"
# This is the IP that akslocaldns service should bind to for node traffic; an APIPA address.
: "${AKSLOCALDNS_NODE_LISTENER_IP:?Required parameter AKSLOCALDNS_NODE_LISTENER_IP is not set}"
# This is the IP that akslocaldns service should bind to for pod traffic; an APIPA address.
: "${AKSLOCALDNS_CLUSTER_LISTENER_IP:?Required parameter AKSLOCALDNS_CLUSTER_LISTENER_IP is not set}"
# CPU limit for akslocaldns service.
: "${AKSLOCALDNS_CPU_LIMIT:?Required parameter AKSLOCALDNS_CPU_LIMIT is not set}"
# Memory limit in MB for akslocaldns service.
: "${AKSLOCALDNS_MEMORY_LIMIT:?Required parameter AKSLOCALDNS_MEMORY_LIMIT is not set}"
# Delay coredns shutdown to allow connections to finish.
: "${AKSLOCALDNS_SHUTDOWN_DELAY:?Required parameter AKSLOCALDNS_SHUTDOWN_DELAY is not set}"
# PID file.
: "${AKSLOCALDNS_PID_FILE:?Required parameter AKSLOCALDNS_PID_FILE is not set}"

# Information variables.
# --------------------------------------------------------------------------------------------------------------------
SCRIPT_PATH="$(dirname -- "$(readlink -f -- "$0";)";)"
DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status "${DEFAULT_ROUTE_INTERFACE}" | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-akslocaldns.conf"
UPSTREAM_VNET_DNS_SERVERS="$(</run/systemd/resolve/resolv.conf awk '/nameserver/ {print $2}' | paste -sd' ')"

# akslocaldns corefile.
# --------------------------------------------------------------------------------------------------------------------
# Generated Corefile path used by akslocaldns service.
# This should match with 'AKSLOCALDNS_CORE_FILE' defined in parts/linux/cloud-init/artifacts/cse_config.sh.
AKSLOCALDNS_CORE_FILE_PATH="/opt/azure/akslocaldns/Corefile"

if [ ! -f "${AKSLOCALDNS_CORE_FILE_PATH}" ]; then
  printf "Error: akslocaldns corefile does not exist at %s.\n" "${AKSLOCALDNS_CORE_FILE_PATH}"
  exit 1
fi
if [ ! -s "${AKSLOCALDNS_CORE_FILE_PATH}" ]; then
  printf "Error: akslocaldns corefile is empty at %s.\n" "${AKSLOCALDNS_CORE_FILE_PATH}"
  exit 1
fi

# Validate UPSTREAM_VNET_DNS_SERVERS is not empty
if [ -z "${UPSTREAM_VNET_DNS_SERVERS}" ]; then
    printf "Error: No Upstream VNET DNS servers found in /run/systemd/resolve/resolv.conf.\n"
    exit 1
fi
# Replace all occurrences of Vnet_Dns_Servers with UPSTREAM_VNET_DNS_SERVERS in akslocaldns corefile.
# Based on customer input corefile is generated with Vnet_Dns_Servers as placeholder in pkg/agent/baker.go.
sed -i "s/Vnet_Dns_Servers/${UPSTREAM_VNET_DNS_SERVERS}/g" "${AKSLOCALDNS_CORE_FILE_PATH}"

printf "Generated corefile:\n"
cat "${AKSLOCALDNS_CORE_FILE_PATH}"

# Configure CPU and Memory limit.
# --------------------------------------------------------------------------------------------------------------------
AKSLOCALDNS_SLICE_PATH="/etc/systemd/system/akslocaldns.slice"
if [ ! -f "$AKSLOCALDNS_SLICE_PATH" ]; then
    printf "Error: akslocaldns slice file does not exist at %s.\n" "${AKSLOCALDNS_SLICE_PATH}"
    exit 1
fi

CPU_QUOTA="$((AKSLOCALDNS_CPU_LIMIT * 100))%"
CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)
if [ "$CGROUP_VERSION" = "cgroup2fs" ]; then
    sed -i -e "s/^CPUQuota=[^ ]*/CPUQuota=$CPU_QUOTA/" -e "s/^MemoryMax=[^ ]*/MemoryMax=${AKSLOCALDNS_MEMORY_LIMIT}M/" "$AKSLOCALDNS_SLICE_PATH"
elif [ "$CGROUP_VERSION" = "tmpfs" ]; then
    sed -i -e "s/^CPUQuota=[^ ]*/CPUQuota=$CPU_QUOTA/" -e "s/^MemoryMax=[^ ]*/MemoryMax=${AKSLOCALDNS_MEMORY_LIMIT}M/" "$AKSLOCALDNS_SLICE_PATH"
else
    echo "Error: Unsupported cgroup version: $CGROUP_VERSION"
    exit 1
fi

# Iptables: build rules.
# --------------------------------------------------------------------------------------------------------------------
# These rules skip conntrack for DNS traffic to save conntrack table
# space. OUTPUT rules affect node services and hostNetwork: true pods.
# PREROUTING rules affect traffic from regular pods.
IPTABLES='iptables -w -t raw -m comment --comment "AKS Local DNS: skip conntrack"'
IPTABLES_RULES=()
for CHAIN in OUTPUT PREROUTING; do
for IP in ${AKSLOCALDNS_NODE_LISTENER_IP} ${AKSLOCALDNS_CLUSTER_LISTENER_IP}; do
for PROTO in tcp udp; do
    IPTABLES_RULES+=("${CHAIN} -p ${PROTO} -d ${IP} --dport 53 -j NOTRACK")
done; done; done

# Cleanup function: will be run on script exit/crash to revert config.
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

# source /opt/azure/containers/provision_source.sh
# akslocaldns: extract from image.
# --------------------------------------------------------------------------------------------------------------------
if [ ! -x "${SCRIPT_PATH}/coredns" ]; then
    printf "extracting coredns from image: %s \n" "${AKSLOCALDNS_IMAGE_URL}"
    CTR_TEMP="$(mktemp -d)"

    # Set a trap to clean up the temp directory if anything fails.
    function cleanup_coredns_import {
        # Disable error handling so that we don't get into a recursive loop.
        set +e
        printf 'Error extracting coredns.\n'
        ctr -n k8s.io images unmount "${CTR_TEMP}" >/dev/null
        rm -rf "${CTR_TEMP}"
    }
    # Set trap to clean up on failure
    trap cleanup_coredns_import EXIT ABRT ERR INT PIPE QUIT TERM

    # Function to pull the image.
    function retrycmd_pull_image_using_ctr() {
        local retries=$1
        local wait_sleep=$2
        local image="$3"

        for i in $(seq 1 $retries); do
            if timeout 60 ctr -n k8s.io images pull "${image}" >/dev/null; then
                printf "ctr Successfully pulled image: %s \n" "${image}"
                return 0
            fi
            sleep $wait_sleep
        done
        printf "Error: Failed to ctr pull image: %s after %d attempts.\n" "${image}" "${retries}"
        return 1
    }

    if ! ctr -n k8s.io images ls | grep -q "${AKSLOCALDNS_IMAGE_URL}"; then
        printf "Image not found locally, attempting to pull: %s \n" "${AKSLOCALDNS_IMAGE_URL}"
        retrycmd_pull_image_using_ctr 5 10 "${AKSLOCALDNS_IMAGE_URL}" || exit 1
    fi

    # Mount the coredns image to the temporary directory.
    ctr -n k8s.io images mount "${AKSLOCALDNS_IMAGE_URL}" "${CTR_TEMP}" >/dev/null

    # Find the CoreDNS binary in the mounted directory. head -n 1 is used to get the first binary found.
    # For coredns images built using dalec, coredns binary is placed in this path /usr/bin/coredns.
    # reference - https://github.com/Azure/dalec-build-defs/blob/a72a61032c6626dd0f7d66f2508925046a1d6560/specs/coredns/coredns-1.12.0.yml#L65
    # For registry.k8s.io/coredns/coredns:v1.11.3 image, coredns binary is placed in this path /coredns.
    COREDNS_BINARY=$(find "${CTR_TEMP}" -type f -name "coredns" | head -n 1)

    # Check if the binary was found.
    if [[ -z "$COREDNS_BINARY" ]]; then
        printf "Error: coredns binary not found in the image.\n"
        exit 1
    fi    

    # Copy coredns to SCRIPT_PATH.
    cp "$COREDNS_BINARY" "${SCRIPT_PATH}/coredns"

    # Umount and clean up the temporary directory.
    ctr -n k8s.io images unmount "${CTR_TEMP}" >/dev/null
    rm -rf "${CTR_TEMP}"

    # Clear the trap after success.
    trap - EXIT ABRT ERR INT PIPE QUIT TERM
fi

# Enable the cleanup function now that we have a coredns binary.
trap "exit 0" QUIT TERM                                    # Exit with code 0 on a successful shutdown.
trap "exit 1" ABRT ERR INT PIPE                            # Exit with code 1 on a bad signal.
trap "printf 'executing cleanup function\n'; cleanup" EXIT # Always cleanup when you're exiting.

# Node listener and cluster listener.
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

# Build the coredns command.
# --------------------------------------------------------------------------------------------------------------------
COREDNS_COMMAND="/opt/azure/akslocaldns/coredns -conf ${AKSLOCALDNS_CORE_FILE_PATH} -pidfile ${AKSLOCALDNS_PID_FILE}"
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
        printf "\nERROR: akslocaldns failed to come online.\n"
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
# wait ends (which will be when a signal is sent or akslocaldns crashes) or
# the script receives a terminal signal.
# --------------------------------------------------------------------------------------------------------------------
# end of line