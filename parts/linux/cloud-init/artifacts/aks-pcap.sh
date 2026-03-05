#!/bin/bash
set -uxo pipefail

PCAP_DIR="/var/log/azure/aks"
PCAP_FILE="${PCAP_DIR}/aks-node.pcap"
PCAP_ZIP="${PCAP_DIR}/aks-node.pcap.zip"

mkdir -p "${PCAP_DIR}"

# Capture packets on port 443 for up to 300 seconds
timeout 300 tcpdump -i eth0 -s 0 -w "${PCAP_FILE}" 'port 443'

# Compress the pcap into its own zip so it can be picked up by aks-log-collector
if [ -f "${PCAP_FILE}" ]; then
    zip -jm "${PCAP_ZIP}" "${PCAP_FILE}"
fi

/opt/azure/containers/aks-log-collector.sh >/var/log/azure/aks/cse-aks-log-collector.log 2>&1
