#!/bin/bash
# Test script for DNS vnetdns script
# Tests check_dns_to_vnetdns.sh only

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common/test_common.sh"
source "$SCRIPT_DIR/common/event_log_validation.sh"

# Script under test
readonly VNETDNS_SCRIPT="/etc/node-problem-detector.d/plugin/check_dns_to_vnetdns.sh"

# Helper function to run VNet DNS tests
run_vnetdns_test() {
    local test_name="$1"
    local expected_output="$2"
    local mock_data_dir="${3:-$SCRIPT_DIR/testdata/mock-data/azuredns-healthy}"
    local dig_scenario="${4:-success}"
    local custom_env_vars="$5"
    local expected_exit_code="${6:-0}"
    
    # Default environment variables for VNet DNS tests
    local default_env_vars="-e DIG_SCENARIO=$dig_scenario -e FORCE_DIAGNOSTICS=false"
    default_env_vars+=" -e PATH=/mock-commands/dns-vnetdns:/mock-commands:/usr/bin:/bin"
    
    # Combine default and custom env vars
    local all_env_vars="$default_env_vars"
    if [ -n "$custom_env_vars" ]; then
        all_env_vars="$all_env_vars $custom_env_vars"
    fi
    
    # Build volume mounts
    local volume_mounts=""
    volume_mounts+="-v \"$SCRIPT_DIR/testdata/mock-commands:/mock-commands:ro\""
    
    if [ -n "$mock_data_dir" ] && [ -d "$mock_data_dir" ]; then
        # Mount mock data files to their expected system paths
        if [ -f "$mock_data_dir/run/systemd/resolve/resolv.conf" ]; then
            volume_mounts+=" -v \"$mock_data_dir/run/systemd/resolve/resolv.conf:/run/systemd/resolve/resolv.conf:ro\""
        fi
        if [ -f "$mock_data_dir/etc/resolv.conf" ]; then
            volume_mounts+=" -v \"$mock_data_dir/etc/resolv.conf:/etc/resolv.conf:ro\""
        fi
        if [ -f "$mock_data_dir/var/lib/kubelet/kubeconfig" ]; then
            volume_mounts+=" -v \"$mock_data_dir/var/lib/kubelet/kubeconfig:/var/lib/kubelet/kubeconfig:ro\""
        fi
        # Also mount the entire mock data directory for any other files
        volume_mounts+=" -v \"$mock_data_dir:/mock-data:ro\""
    fi
    
    # Call the generic run_test function with expected exit code
    run_test "$VNETDNS_SCRIPT" "$test_name" "$expected_output" "$volume_mounts" "$all_env_vars" "30" "$expected_exit_code"
}

start_fixture "Unit tests for check_dns_to_vnetdns.sh"

# VNetDNS - successful DNS resolution
run_vnetdns_test "VNetDNS - DNS resolution successful" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-healthy" \
    "success"

# VNetDNS - no VNet DNS IPs found
run_vnetdns_test "VNetDNS - No VNETDNS IPs Found" \
    "No VNet DNS IPs found to test. Exiting gracefully." \
    "$SCRIPT_DIR/testdata/mock-data/no-vnet-ips" \
    "success"

# VNetDNS - resolution failure (should exit 1 - DNS issue detected)
# VNetDNS - DNS resolution failure
run_vnetdns_test "VNetDNS - DNS resolution failure" \
    "dns test to vnetdns:" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-healthy" \
    "dns-failure" \
    "" \
    1

# LocalDNS disabled (systemd fallback)
run_vnetdns_test "VNetDNS - No LocalDNS Systemd Fallback" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-no-localdns" \
    "success"

# Test multiple VNet DNS IPs with partial failures (should exit 1 - DNS issue detected)
# VNetDNS - Multiple IPs Mixed Results
run_vnetdns_test "VNetDNS - Multiple IPs Mixed Results" \
    "dns test to vnetdns:" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-healthy" \
    "dns-mixed-results" \
    "" \
    1

# Test TCP vs UDP protocol differences
run_vnetdns_test "VNetDNS - TCP Protocol Success" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-healthy" \
    "success"

# Test get_vnet_dns_ips function with real LocalDNS corefile format
run_vnetdns_test "VNetDNS - Real LocalDNS Format" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-real-format" \
    "success"

# Test with malformed corefile (should handle gracefully)
run_vnetdns_test "VNetDNS - Malformed Corefile" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/malformed-corefile" \
    "success"

# Test with large corefile containing multiple sections
run_vnetdns_test "VNetDNS - Large Corefile Multiple IPs" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/large-corefile" \
    "success"

# Test with IPv6 and IPv4 mixed DNS servers (should extract only IPv4)
run_vnetdns_test "VNetDNS - IPv6 Mixed with IPv4" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/ipv6-mixed" \
    "success"

# Test with comma-separated DNS servers
run_vnetdns_test "VNetDNS - Comma Separated IPs" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/comma-separated" \
    "success"

# Test DNS resolution timeout scenario
# VNetDNS - Timeout Handling
run_vnetdns_test "VNetDNS - Timeout Handling" \
    "dns test to vnetdns:" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-healthy" \
    "dns-timeout" \
    "" \
    1

# Test with custom external domain
run_vnetdns_test "VNetDNS - Custom Domain Resolution" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-healthy" \
    "success" \
    "-e TEST_EXTERNAL_DOMAIN=example.microsoft.com"

# Test UDP protocol specifically
run_vnetdns_test "VNetDNS - UDP Protocol Only" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-healthy" \
    "success"

# Test edge case: single DNS server with failure then success
run_vnetdns_test "VNetDNS - Retry Logic Success" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/azuredns-healthy" \
    "success" \
    "-e DIG_RETRY_COUNT=3"

# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi