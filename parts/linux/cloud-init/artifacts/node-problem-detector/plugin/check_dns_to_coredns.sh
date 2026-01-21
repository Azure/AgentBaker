#!/bin/bash

set -uo pipefail

# shellcheck source=check_dns_common.sh
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
# Attempt to source check_dns_common.sh, exit if it fails.
source "${SCRIPT_DIR}/check_dns_common.sh" || { echo "ERROR: Critical dependency check_dns_common.sh not found or failed to source." >&2; exit 0; }

# --------------------------- Execution starts here ----------------------------
# Check dependencies.
check_dependencies

# Get the CoreDNS serviceIP.
coredns_service_ip=$(get_coredns_ip)

if [ -z "$coredns_service_ip" ]; then
    echo "No coredns service IP found to test. Exiting gracefully." >&2
    exit $OK
fi

# DNS check over UDP.
check_dns_with_retry "$TEST_IN_CLUSTER_DOMAIN" "$coredns_service_ip" "$UDP_PROTOCOL" "coredns"
udp_result=$?
if [ $udp_result -ne 0 ]; then
    exit $NOTOK
fi

# DNS check over TCP.
check_dns_with_retry "$TEST_IN_CLUSTER_DOMAIN" "$coredns_service_ip" "$TCP_PROTOCOL" "coredns"
tcp_result=$?
if [ $tcp_result -ne 0 ]; then
    exit $NOTOK
fi

# All checks passed.
exit $OK