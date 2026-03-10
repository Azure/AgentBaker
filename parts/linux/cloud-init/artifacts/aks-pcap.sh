#!/bin/bash
set -uxo pipefail

PCAP_DIR="/var/log/azure/aks"
PCAP_FILE="${PCAP_DIR}/aks-node.pcap"
PCAP_ZIP="${PCAP_DIR}/aks-node.pcap.zip"

mkdir -p "${PCAP_DIR}"

# Capture packets on port 443 for up to 300 seconds, limiting output to 95MB
timeout 300 tcpdump -i eth0 -s 0 -C 95 -W 1 -w "${PCAP_FILE}" 'port 443'
# -C/-W appends a numeric suffix to the filename, rename it back
if [ -f "${PCAP_FILE}0" ]; then
    mv "${PCAP_FILE}0" "${PCAP_FILE}"
fi

# Compress the pcap into its own zip so it can be picked up by aks-log-collector
if [ -f "${PCAP_FILE}" ]; then
    zip -jm "${PCAP_ZIP}" "${PCAP_FILE}"
fi

/opt/azure/containers/aks-log-collector.sh >/var/log/azure/aks/cse-aks-log-collector.log 2>&1
