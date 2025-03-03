#! /bin/bash
set -euo pipefail


AKS_LOCAL_DNS_ENV_FILE_PATH="/etc/default/aks-local-dns/aks-local-dns.envfile"
if [ -f "${AKS_LOCAL_DNS_ENV_FILE_PATH}" ]; then
    source "${AKS_LOCAL_DNS_ENV_FILE_PATH}"
else
    printf "Error: Envfile does not exist at ${AKS_LOCAL_DNS_ENV_FILE_PATH}\n"
    exit 1
fi

: "${AKSLOCALDNS_IMAGE_URL:?Required parameter AKSLOCALDNS_IMAGE_URL is not set}"
: "${AKSLOCALDNS_NODE_LISTENER_IP:?Required parameter AKSLOCALDNS_NODE_LISTENER_IP is not set}"
: "${AKSLOCALDNS_CLUSTER_LISTENER_IP:?Required parameter AKSLOCALDNS_CLUSTER_LISTENER_IP is not set}"
: "${AKSLOCALDNS_CPU_LIMIT:?Required parameter AKSLOCALDNS_CPU_LIMIT is not set}"
: "${AKSLOCALDNS_MEMORY_LIMIT:?Required parameter AKSLOCALDNS_MEMORY_LIMIT is not set}"
: "${AKSLOCALDNS_SHUTDOWN_DELAY:?Required parameter AKSLOCALDNS_SHUTDOWN_DELAY is not set}"
: "${AKSLOCALDNS_PID_FILE:?Required parameter AKSLOCALDNS_PID_FILE is not set}"

readonly COREDNS_IMAGE="${AKSLOCALDNS_IMAGE_URL}"                       
readonly NODE_LISTENER_IP="${AKSLOCALDNS_NODE_LISTENER_IP}"             
readonly CLUSTER_LISTENER_IP="${AKSLOCALDNS_CLUSTER_LISTENER_IP}"       
readonly CPU_LIMIT="${AKSLOCALDNS_CPU_LIMIT}"                           
readonly MEMORY_LIMIT="${AKSLOCALDNS_MEMORY_LIMIT}"                     
readonly COREDNS_SHUTDOWN_DELAY="${AKSLOCALDNS_SHUTDOWN_DELAY}"         
readonly PID_FILE="${AKSLOCALDNS_PID_FILE}"                             

SCRIPT_PATH="$(dirname -- "$(readlink -f -- "$0";)";)"
DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status ${DEFAULT_ROUTE_INTERFACE} | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-aks-local-dns.conf"
UPSTREAM_DNS_SERVERS="$(</run/systemd/resolve/resolv.conf awk '/nameserver/ {print $2}' | paste -sd' ')"

LOCAL_DNS_CORE_FILE_PATH="/opt/azure/aks-local-dns/Corefile"

if [ ! -f "${LOCAL_DNS_CORE_FILE_PATH}" ]; then
  printf "Error: Corefile does not exist at ${LOCAL_DNS_CORE_FILE_PATH}\n"
  exit 1
fi
if [ ! -s "${LOCAL_DNS_CORE_FILE_PATH}" ]; then
  printf "Error: Corefile is empty at ${LOCAL_DNS_CORE_FILE_PATH}\n"
  exit 1
fi

if [ -z "${UPSTREAM_DNS_SERVERS}" ]; then
    echo "Error: No upstream DNS servers found in /run/systemd/resolve/resolv.conf.\n"
    exit 1
fi
sed -i "s/Vnet_Dns_Servers/${UPSTREAM_DNS_SERVERS}/g" "${LOCAL_DNS_CORE_FILE_PATH}"

printf "Generated corefile:\n"
cat "${LOCAL_DNS_CORE_FILE_PATH}"

IPTABLES='iptables -w -t raw -m comment --comment "AKS Local DNS: skip conntrack"'
IPTABLES_RULES=()
for CHAIN in OUTPUT PREROUTING; do
for IP in ${NODE_LISTENER_IP} ${CLUSTER_LISTENER_IP}; do
for PROTO in tcp udp; do
    IPTABLES_RULES+=("${CHAIN} -p ${PROTO} -d ${IP} --dport 53 -j NOTRACK")
done; done; done

function cleanup {
    set +e

    for RULE in "${IPTABLES_RULES[@]}"; do
        while eval "${IPTABLES}" -D "${RULE}" 2>/dev/null; do 
            printf "removed iptables rule: $RULE\n"
        done
    done

    if [ -f ${NETWORK_DROPIN_FILE} ]; then
        printf "Reverting DNS configuration by removing %s\n" "${NETWORK_DROPIN_FILE}"
        /bin/rm -f ${NETWORK_DROPIN_FILE}
        networkctl reload
    fi

    if [ ! -z "${COREDNS_PID:-}" ]; then
        if ps ${COREDNS_PID} >/dev/null; then
            if [[ ${COREDNS_SHUTDOWN_DELAY} -gt 0 ]]; then
                printf "sleeping ${COREDNS_SHUTDOWN_DELAY} seconds to allow connections to terminate\n"
                sleep ${COREDNS_SHUTDOWN_DELAY}
            fi
            printf "sending SIGINT to coredns and waiting for it to terminate\n"

            kill -SIGINT ${COREDNS_PID}

            wait -f ${COREDNS_PID}
            printf "coredns terminated\n"
        fi
    fi

    if ip link show dev aks-local-dns >/dev/null 2>&1; then
        printf "removing aks-local-dns dummy interface\n"
        ip link del name aks-local-dns
    fi
}

if [[ $* == *--cleanup* ]]; then
    cleanup
    exit 0
fi

if [ ! -x "${SCRIPT_PATH}/coredns" ]; then
    printf "extracting coredns from image: ${COREDNS_IMAGE}\n"
    CTR_TEMP="$(mktemp -d)"

    function cleanup_coredns_import {
        set +e
        printf 'Error extracting coredns\n'
        ctr -n k8s.io images unmount "${CTR_TEMP}" >/dev/null
        rm -rf "${CTR_TEMP}"
    }
    trap cleanup_coredns_import EXIT ABRT ERR INT PIPE QUIT TERM

    function retrycmd_pull_image_using_ctr() {
        local retries=$1
        local wait_sleep=$2
        local image="$3"

        for i in $(seq 1 $retries); do
            if timeout 60 ctr -n k8s.io images pull "${image}" >/dev/null; then
                printf "ctr Successfully pulled image: ${image}\n"
                return 0
            fi
            sleep $wait_sleep
        done
        printf "Error: Failed to ctr pull image: ${image} after %d attempts.\n" "$retries"
        return 1
    }

    if ! ctr -n k8s.io images ls | grep -q "${COREDNS_IMAGE}"; then
        printf "Image not found locally, attempting to pull: ${COREDNS_IMAGE}\n"
        retrycmd_pull_image_using_ctr 5 10 "${COREDNS_IMAGE}" || exit 1
    fi

    ctr -n k8s.io images mount "${COREDNS_IMAGE}" "${CTR_TEMP}" >/dev/null

    COREDNS_BINARY=$(find "${CTR_TEMP}" -type f -name "coredns" | head -n 1)

    if [[ -z "$COREDNS_BINARY" ]]; then
        printf "Error: coredns binary not found in the image.\n"
        exit 1
    fi    

    cp "$COREDNS_BINARY" "${SCRIPT_PATH}/coredns"

    ctr -n k8s.io images unmount "${CTR_TEMP}" >/dev/null
    rm -rf "${CTR_TEMP}"

    trap - EXIT ABRT ERR INT PIPE QUIT TERM
fi

trap "exit 0" QUIT TERM                                    
trap "exit 1" ABRT ERR INT PIPE                            
trap "printf 'executing cleanup function\n'; cleanup" EXIT 

printf "setting up aks-local-dns dummy interface with IPs ${NODE_LISTENER_IP} and ${CLUSTER_LISTENER_IP}\n"
ip link add name aks-local-dns type dummy
ip link set up dev aks-local-dns
ip addr add ${NODE_LISTENER_IP}/32 dev aks-local-dns
ip addr add ${CLUSTER_LISTENER_IP}/32 dev aks-local-dns

printf "adding iptables rules to skip conntrack for queries to aks-local-dns\n"
for RULE in "${IPTABLES_RULES[@]}"; do
    eval "${IPTABLES}" -A "${RULE}"
done

COREDNS_COMMAND="/opt/azure/aks-local-dns/coredns -conf ${LOCAL_DNS_CORE_FILE_PATH} -pidfile ${PID_FILE}"
if [[ ! -z "${SYSTEMD_EXEC_PID:-}" ]]; then
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

declare -i ATTEMPTS=0
printf "waiting for coredns to start and be able to serve traffic\n"
until [ "$(curl -s "http://${NODE_LISTENER_IP}:8181/ready")" == "OK" ]; do
    if [ $ATTEMPTS -ge 60 ]; then
        printf "\nERROR: coredns failed to come online!\n"
        exit 255
    fi
    sleep 1
    ATTEMPTS+=1
done
printf "coredns online and ready to serve node traffic\n"

printf "updating network DNS configuration to point to coredns via ${NETWORK_DROPIN_FILE}\n"
mkdir -p ${NETWORK_DROPIN_DIR}
printf "[Network]\nDNS=${NODE_LISTENER_IP}\n\n[DHCP]\nUseDNS=false\n" >${NETWORK_DROPIN_FILE}
chmod -R ugo+rX ${NETWORK_DROPIN_DIR}
networkctl reload
printf "startup complete - serving node and pod DNS traffic\n"

if [[ ! -z "${NOTIFY_SOCKET:-}" ]]; then systemd-notify --ready; fi

if [[ ! -z "${NOTIFY_SOCKET:-}" && ! -z "${WATCHDOG_USEC:-}" ]]; then
    HEALTH_CHECK_INTERVAL=$((${WATCHDOG_USEC:-5000000} * 20 / 100 / 1000000))
    HEALTH_CHECK_DNS_REQUEST=$'health-check.aks-local-dns.local @'"${NODE_LISTENER_IP}"$'\nhealth-check.aks-local-dns.local @'"${CLUSTER_LISTENER_IP}"
    printf "starting watchdog loop at ${HEALTH_CHECK_INTERVAL} second intervals\n"
    while [ true ]; do
        if [[ "$(curl -s "http://${NODE_LISTENER_IP}:8181/ready")" == "OK" ]]; then
            if dig +short +timeout=1 +tries=1 -f<(printf "${HEALTH_CHECK_DNS_REQUEST}"); then
                systemd-notify WATCHDOG=1
            fi
        fi
        sleep ${HEALTH_CHECK_INTERVAL}
    done
else
    wait -f ${COREDNS_PID}
fi
