#!/bin/bash
set -uo pipefail

# shellcheck source=check_dns_common.sh
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
# Attempt to source check_dns_common.sh, exit if it fails.
source "${SCRIPT_DIR}/check_dns_common.sh" || { echo "ERROR: Critical dependency check_dns_common.sh not found or failed to source." >&2; exit 0; }

# --------------------------- Execution starts here ----------------------------
# Check dependencies.
check_dependencies

# Get VNet DNS IPs.
vnet_dns_ips=$(get_vnet_dns_ips)

if [ -z "$vnet_dns_ips" ]; then
    echo "No VNet DNS IPs found to test. Exiting gracefully." >&2
    exit $OK
fi

for ip in $vnet_dns_ips; do
    # DNS check over UDP.
    check_dns_with_retry "$TEST_EXTERNAL_DOMAIN" "$ip" "$UDP_PROTOCOL" "vnetdns"
    udp_result=$?
    if [ $udp_result -ne 0 ]; then
        exit $NOTOK
    fi

    # DNS check over TCP.
    check_dns_with_retry "$TEST_EXTERNAL_DOMAIN" "$ip" "$TCP_PROTOCOL" "vnetdns"
    tcp_result=$?
    if [ $tcp_result -ne 0 ]; then
        exit $NOTOK
    fi
done

# All checks passed.
exit $OK
