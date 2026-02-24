#!/bin/bash
set -uxo pipefail

mkdir -p /var/log/azure/aks

timeout 300 tcpdump -i eth0 -s 0 -w /var/log/azure/aks/aks-node.pcap 'dst port 443' || /opt/azure/containers/aks-log-collector.sh >/var/log/azure/aks/cse-aks-log-collector.log 2>&1
