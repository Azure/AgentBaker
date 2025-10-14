#!/bin/bash

set -uo pipefail

# shellcheck source=check_dns_common.sh
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
# Attempt to source check_dns_common.sh, exit if it fails.
source "${SCRIPT_DIR}/check_dns_common.sh" || { echo "ERROR: Critical dependency check_dns_common.sh not found or failed to source." >&2; exit 0; }

# --------------------------- Execution starts here ----------------------------
if is_localdns_enabled; then
  check_dependencies

  # Check localdns service status.
  service_check_output=$(timeout "$COMMAND_TIMEOUT_SECONDS" systemctl is-active localdns.service 2>&1)
  localdns_service_exit_code=$?
    
  if [ $localdns_service_exit_code -ne 0 ]; then
    if [ $localdns_service_exit_code -eq 124 ]; then
      echo "systemctl command to check localdns service timed out after $COMMAND_TIMEOUT_SECONDS seconds."
      exit $UNKNOWN
    else
      echo "localdns service is not running. State: $service_check_output, exit code: $localdns_service_exit_code"
      exit $NOTOK
    fi
  fi

  # DNS check against the cluster listener IP.
  check_dns_with_retry "$TEST_IN_CLUSTER_DOMAIN" "$LOCALDNS_CLUSTER_LISTENER_IP" "$UDP_PROTOCOL" "localdns"
  clusterlistener_result=$?
  if [ $clusterlistener_result -ne 0 ]; then
      exit $NOTOK
  fi

  # DNS check against the node listener IP.
  check_dns_with_retry "$TEST_IN_CLUSTER_DOMAIN" "$LOCALDNS_NODE_LISTENER_IP" "$UDP_PROTOCOL" "localdns"
  nodelistener_result=$?
  if [ $nodelistener_result -ne 0 ]; then
      exit $NOTOK
  fi
else
  # Check if LocalDNS is enabled via label.
  localdns_state=$(get_localdns_state_label)
  if echo "$localdns_state" | grep -q "enabled"; then
    echo "localdns corefile not found and localdns state is enabled"
    exit $NOTOK
  fi
fi

# All checks passed.
exit $OK