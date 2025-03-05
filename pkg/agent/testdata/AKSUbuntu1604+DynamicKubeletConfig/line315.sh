#! /bin/bash
set -euo pipefail


AKSLOCALDNS_ENV_FILE_PATH="/etc/default/akslocaldns/akslocaldns.envfile"
if [ -f "${AKSLOCALDNS_ENV_FILE_PATH}" ]; then
    source "${AKSLOCALDNS_ENV_FILE_PATH}"
else
    printf "Error: akslocaldns envfile does not exist at %s.\n" "${AKSLOCALDNS_ENV_FILE_PATH}"
    exit 1
fi

AKSLOCALDNS_CORE_FILE_PATH="/opt/azure/akslocaldns/Corefile"
if [ ! -f "${AKSLOCALDNS_CORE_FILE_PATH}" ] || [ ! -s "${AKSLOCALDNS_CORE_FILE_PATH}" ]; then
    printf "Error: akslocaldns corefile either does not exist or is empty at %s.\n" "${AKSLOCALDNS_CORE_FILE_PATH}"
    exit 1
fi

AKSLOCALDNS_SLICE_PATH="/etc/systemd/system/akslocaldns.slice"
if [ ! -f "${AKSLOCALDNS_SLICE_PATH}" ]; then
    printf "Error: akslocaldns slice file does not exist at %s.\n" "${AKSLOCALDNS_SLICE_PATH}"
    exit 1
fi

: "${AKSLOCALDNS_IMAGE_URL:?AKSLOCALDNS_IMAGE_URL is not set}"
: "${AKSLOCALDNS_NODE_LISTENER_IP:?AKSLOCALDNS_NODE_LISTENER_IP is not set}"
: "${AKSLOCALDNS_CLUSTER_LISTENER_IP:?AKSLOCALDNS_CLUSTER_LISTENER_IP is not set}"
: "${AKSLOCALDNS_CPU_LIMIT:?AKSLOCALDNS_CPU_LIMIT is not set}"
: "${AKSLOCALDNS_MEMORY_LIMIT:?AKSLOCALDNS_MEMORY_LIMIT is not set}"
: "${AKSLOCALDNS_SHUTDOWN_DELAY:?AKSLOCALDNS_SHUTDOWN_DELAY is not set}"
: "${AKSLOCALDNS_PID_FILE:?AKSLOCALDNS_PID_FILE is not set}"

COREDNS_VERSION="${AKSLOCALDNS_IMAGE_URL##*:}"
SCRIPT_PATH="/opt/azure/akslocaldns"
if [ ! -x "${SCRIPT_PATH}/${COREDNS_VERSION}/coredns" ]; then
    printf "coredns binary not found at %s. \n" "${AKSLOCALDNS_IMAGE_URL}"
    exit 1
fi

CPU_QUOTA="$((AKSLOCALDNS_CPU_LIMIT * 100))%"
CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)
if [ "${CGROUP_VERSION}" = "cgroup2fs" ]; then
    sed -i -e "s/^CPUQuota=[^ ]*/CPUQuota=${CPU_QUOTA}/" -e "s/^MemoryMax=[^ ]*/MemoryMax=${AKSLOCALDNS_MEMORY_LIMIT}M/" "${AKSLOCALDNS_SLICE_PATH}" || { echo "Error: sed command failed"; exit 1; }
else
    echo "Error: Unsupported cgroup version: ${CGROUP_VERSION}"
    exit 1
fi

UPSTREAM_VNET_DNS_SERVERS="$(</run/systemd/resolve/resolv.conf awk '/nameserver/ {print $2}' | paste -sd' ')"
if [ -z "${UPSTREAM_VNET_DNS_SERVERS}" ]; then
    printf "Error: No Upstream VNET DNS servers found in /run/systemd/resolve/resolv.conf.\n"
    exit 1
fi
sed -i "s/Vnet_Dns_Servers/${UPSTREAM_VNET_DNS_SERVERS}/g" "${AKSLOCALDNS_CORE_FILE_PATH}" || { echo "Error: sed command failed"; exit 1; }

IPTABLES='iptables -w -t raw -m comment --comment "AKS Local DNS: skip conntrack"'
IPTABLES_RULES=()
for CHAIN in OUTPUT PREROUTING; do
for IP in ${AKSLOCALDNS_NODE_LISTENER_IP} ${AKSLOCALDNS_CLUSTER_LISTENER_IP}; do
for PROTO in tcp udp; do
    IPTABLES_RULES+=("${CHAIN} -p ${PROTO} -d ${IP} --dport 53 -j NOTRACK")
done; done; done

DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status "${DEFAULT_ROUTE_INTERFACE}" | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-akslocaldns.conf"

function cleanup {
    set +e

    for RULE in "${IPTABLES_RULES[@]}"; do
        if eval "${IPTABLES}" -C "${RULE}" 2>/dev/null; then
            eval "${IPTABLES}" -D "${RULE}"
            printf "Removed iptables rule: %s.\n" "${RULE}"
        fi
    done

    if [ -f ${NETWORK_DROPIN_FILE} ]; then
        printf "Reverting DNS configuration by removing %s.\n" "${NETWORK_DROPIN_FILE}"
        /bin/rm -f ${NETWORK_DROPIN_FILE}
        networkctl reload
    fi

    if [ ! -z "${COREDNS_PID:-}" ]; then
        if ps ${COREDNS_PID} >/dev/null; then
            if [[ ${AKSLOCALDNS_SHUTDOWN_DELAY} -gt 0 ]]; then
                printf "sleeping %d seconds to allow connections to terminate.\n" "${AKSLOCALDNS_SHUTDOWN_DELAY}"
                sleep ${AKSLOCALDNS_SHUTDOWN_DELAY}
            fi
            printf "sending SIGINT to akslocaldns and waiting for it to terminate.\n"

            kill -SIGINT ${COREDNS_PID}

            wait -f ${COREDNS_PID}
            printf "akslocaldns terminated.\n"
        fi
    fi

    if ip link show dev akslocaldns >/dev/null 2>&1; then
        printf "removing akslocaldns dummy interface.\n"
        ip link del name akslocaldns
    fi
}

if [[ $* == *--cleanup* ]]; then
    cleanup
    exit 0
fi

trap "exit 0" QUIT TERM                                    
trap "exit 1" ABRT ERR INT PIPE                            
trap "printf 'executing cleanup function\n'; cleanup" EXIT 

printf "setting up akslocaldns dummy interface with IPs %s and %s.\n" "${AKSLOCALDNS_NODE_LISTENER_IP}" "${AKSLOCALDNS_CLUSTER_LISTENER_IP}"
ip link add name akslocaldns type dummy
ip link set up dev akslocaldns
ip addr add ${AKSLOCALDNS_NODE_LISTENER_IP}/32 dev akslocaldns
ip addr add ${AKSLOCALDNS_CLUSTER_LISTENER_IP}/32 dev akslocaldns

printf "adding iptables rules to skip conntrack for queries to akslocaldns.\n"
for RULE in "${IPTABLES_RULES[@]}"; do
    eval "${IPTABLES}" -A "${RULE}"
done

COREDNS_COMMAND="${SCRIPT_PATH}/${COREDNS_VERSION}/coredns -conf ${AKSLOCALDNS_CORE_FILE_PATH} -pidfile ${AKSLOCALDNS_PID_FILE}"
if [[ ! -z "${SYSTEMD_EXEC_PID:-}" ]]; then
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

printf "updating network DNS configuration to point to akslocaldns via %s.\n" "${NETWORK_DROPIN_FILE}"
mkdir -p ${NETWORK_DROPIN_DIR}
printf "[Network]\nDNS=%s\n\n[DHCP]\nUseDNS=false\n" "${AKSLOCALDNS_NODE_LISTENER_IP}" > "${NETWORK_DROPIN_FILE}"
chmod -R ugo+rX ${NETWORK_DROPIN_DIR}
networkctl reload
printf "startup complete - serving node and pod DNS traffic.\n"

if [[ ! -z "${NOTIFY_SOCKET:-}" ]]; then systemd-notify --ready; fi

if [[ ! -z "${NOTIFY_SOCKET:-}" && ! -z "${WATCHDOG_USEC:-}" ]]; then
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
