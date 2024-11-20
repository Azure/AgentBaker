#! /bin/bash
set -euo pipefail


COREDNS_IMAGE="$1"                         
NODE_LISTENER_IP="$2"                      
CLUSTER_LISTENER_IP="$3"                   

COREDNS_SHUTDOWN_DELAY="3"                 
PID_FILE="/run/aks-local-dns.pid"          


SCRIPT_PATH="$(dirname -- "$(readlink -f -- "$0";)";)"
DEFAULT_ROUTE_INTERFACE="$(ip -j route get 168.63.129.16 | jq -r '.[0].dev')"
NETWORK_FILE="$(networkctl --json=short status ${DEFAULT_ROUTE_INTERFACE} | jq -r '.NetworkFile')"
NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-aks-local-dns.conf"


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










if [ ! -x ${SCRIPT_PATH}/coredns ]; then
    printf "extracting coredns from docker image: ${COREDNS_IMAGE}\n"
    CTR_TEMP="$(mktemp -d)"

    function cleanup_coredns_import {
        set +e
        printf 'Error extracting coredns\n'
        ctr -n k8s.io images unmount ${CTR_TEMP}
        rm -rf ${CTR_TEMP}
    }
    trap cleanup_coredns_import EXIT ABRT ERR INT PIPE QUIT TERM

    if ! ctr -n k8s.io images ls | grep -q "${COREDNS_IMAGE}"; then
        printf "Image not found locally, pulling: ${COREDNS_IMAGE}\n"
        if ! ctr -n k8s.io images pull "${COREDNS_IMAGE}"; then
            printf "Error: Failed to pull the image: ${COREDNS_IMAGE}\n"
            exit 1
        fi
    fi

    if ! ctr -n k8s.io images mount "${COREDNS_IMAGE}" "${CTR_TEMP}" >/dev/null; then
        printf "Error: Failed to mount the image: ${COREDNS_IMAGE}\n"
        exit 1
    fi

    if [ ! -f "${CTR_TEMP}/coredns" ]; then
        printf "Error: coredns binary not found in the image\n"
        exit 1
    fi

    cp "${CTR_TEMP}/coredns" "${SCRIPT_PATH}/coredns" || {
        printf "Error: Failed to copy coredns binary to ${SCRIPT_PATH}\n"
        exit 1
    }

    ctr -n k8s.io images unmount ${CTR_TEMP} >/dev/null
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

LOCAL_DNS_CORE_FILE_PATH="/opt/azure/aks-local-dns/Corefile"
if [ ! -f "${LOCAL_DNS_CORE_FILE_PATH}" ]; then
  echo "Error: Corefile does not exist."
  exit 1
fi
if [ ! -s "${LOCAL_DNS_CORE_FILE_PATH}" ]; then
  echo "Error: Corefile is empty."
  exit 1
fi
cat "${LOCAL_DNS_CORE_FILE_PATH}"

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

