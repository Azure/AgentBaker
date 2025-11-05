#!/bin/bash
# Test script for DNS CoreDNS script
# Tests check_dns_to_coredns.sh only

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common/test_common.sh"
source "$SCRIPT_DIR/common/event_log_validation.sh"

# Script under test
readonly COREDNS_SCRIPT="/etc/node-problem-detector.d/plugin/check_dns_to_coredns.sh"

# Helper function to run get_vnet_dns_ips tests
run_get_vnet_dns_ips_test() {
    local test_name="$1"
    local expected_output="$2"
    local mock_data_dir="$3"
    local expected_exit_code="${4:-0}"
    
    # Use the test wrapper script for get_vnet_dns_ips
    local test_script="/mock-commands/dns-coredns/test_get_vnet_dns_ips"
    
    # Default environment variables
    local default_env_vars="-e FORCE_DIAGNOSTICS=false"
    default_env_vars+=" -e PATH=/mock-commands/dns-coredns:/mock-commands:/usr/bin:/bin"
    
    # Build volume mounts
    local volume_mounts=""
    volume_mounts+="-v \"$SCRIPT_DIR/testdata/mock-commands:/mock-commands:ro\""
    
    if [ -n "$mock_data_dir" ] && [ -d "$mock_data_dir" ]; then
        # Mount mock data files to their expected system paths
        if [ -f "$mock_data_dir/opt/azure/containers/localdns/updated.localdns.corefile" ]; then
            volume_mounts+=" -v \"$mock_data_dir/opt/azure/containers/localdns/updated.localdns.corefile:/opt/azure/containers/localdns/updated.localdns.corefile:ro\""
        fi
        if [ -f "$mock_data_dir/run/systemd/resolve/resolv.conf" ]; then
            volume_mounts+=" -v \"$mock_data_dir/run/systemd/resolve/resolv.conf:/run/systemd/resolve/resolv.conf:ro\""
        fi
        if [ -f "$mock_data_dir/etc/resolv.conf" ]; then
            volume_mounts+=" -v \"$mock_data_dir/etc/resolv.conf:/etc/resolv.conf:ro\""
        fi
        # Also mount the entire mock data directory for any other files
        volume_mounts+=" -v \"$mock_data_dir:/mock-data:ro\""
    fi
    
    # Call the generic run_test function with expected exit code
    run_test "$test_script" "$test_name" "$expected_output" "$volume_mounts" "$default_env_vars" "30" "$expected_exit_code"
}

# Helper function to run get_coredns_ip tests
run_get_coredns_ip_test() {
    local test_name="$1"
    local expected_ip="$2"
    local corefile_root="$3"
    local iptables_scenario="$4"
    local expected_exit_code="$5"

    printf "Testing: %s... " "$test_name"

    # Setup test environment
    local old_path="$PATH"
    local temp_corefile=""
    
    # For corefile tests, copy to the expected location
    if [ -n "$corefile_root" ] && [ -f "$corefile_root/opt/azure/containers/localdns/updated.localdns.corefile" ]; then
        mkdir -p "$SCRIPT_DIR/opt/azure/containers/localdns"
        temp_corefile="$SCRIPT_DIR/opt/azure/containers/localdns/updated.localdns.corefile"
        cp "$corefile_root/opt/azure/containers/localdns/updated.localdns.corefile" "$temp_corefile"
    fi
    
    if [ -n "$iptables_scenario" ]; then
        export IPTABLES_SCENARIO="$iptables_scenario"
        export PATH="$SCRIPT_DIR/testdata/mock-commands/dns-coredns:$PATH"
    else
        # Default scenario for iptables tests
        export IPTABLES_SCENARIO="success"
        export PATH="$SCRIPT_DIR/testdata/mock-commands/dns-coredns:$PATH"
    fi

    # Run the test using the wrapper script that handles kubelet file mocking
    local actual_output
    actual_output=$(cd "$SCRIPT_DIR" && PATH="$PATH" IPTABLES_SCENARIO="$IPTABLES_SCENARIO" COREFILE_ROOT="$corefile_root" "$SCRIPT_DIR/testdata/mock-commands/dns-coredns/test_get_coredns_ip_wrapper" 2>&1)
    local exit_code=$?
    
    # Cleanup
    [ -n "$temp_corefile" ] && rm -f "$temp_corefile"
    [ -d "$SCRIPT_DIR/opt" ] && rm -rf "$SCRIPT_DIR/opt"
    unset IPTABLES_SCENARIO
    export PATH="$old_path"
    
    # Check results
    if [ $exit_code -eq $expected_exit_code ]; then
        if echo "$actual_output" | grep -q "$expected_ip"; then
            echo "PASSED"
            return 0
        else
            echo "FAILED"
            echo "  Exit code: $exit_code (expected: $expected_exit_code) ✓"
            echo "  Expected to find: '$expected_ip'"
            echo "  Output excerpt:"
            echo "$actual_output" | head -3 | sed 's/^/    /'
            return 1
        fi
    else
        echo "FAILED"
        echo "  Exit code: $exit_code (expected: $expected_exit_code)"
        echo "  Output excerpt:"
        echo "$actual_output" | head -3 | sed 's/^/    /'
        return 1
    fi
}

# Helper function to run CoreDNS tests
run_coredns_test() {
    local test_name="$1"
    local expected_output="$2"
    local mock_data_dir="${3:-$SCRIPT_DIR/testdata/mock-data/coredns-healthy}"
    local dig_scenario="${4:-success}"
    local iptables_scenario="${5:-success}"
    local custom_env_vars="$6"
    local expected_exit_code="${7:-0}"
    
    # Default environment variables for CoreDNS tests
    local default_env_vars="-e DIG_SCENARIO=$dig_scenario -e IPTABLES_SCENARIO=$iptables_scenario -e FORCE_DIAGNOSTICS=false"
    default_env_vars+=" -e PATH=/mock-commands/dns-coredns:/mock-commands:/usr/bin:/bin"
    
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
        if [ -f "$mock_data_dir/etc/default/kubelet" ]; then
            volume_mounts+=" -v \"$mock_data_dir/etc/default/kubelet:/etc/default/kubelet:ro\""
        fi
        if [ -f "$mock_data_dir/opt/azure/containers/localdns/updated.localdns.corefile" ]; then
            volume_mounts+=" -v \"$mock_data_dir/opt/azure/containers/localdns/updated.localdns.corefile:/opt/azure/containers/localdns/updated.localdns.corefile:ro\""
        fi
        # Also mount the entire mock data directory for any other files
        volume_mounts+=" -v \"$mock_data_dir:/mock-data:ro\""
    fi
    
    # Call the generic run_test function with expected exit code
    run_test "$COREDNS_SCRIPT" "$test_name" "$expected_output" "$volume_mounts" "$all_env_vars" "30" "$expected_exit_code"
}

start_fixture "Unit tests for check_dns_to_coredns.sh"

# CoreDNS - successful DNS resolution
run_coredns_test "CoreDNS - DNS resolution successful" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-healthy" \
    "success"

# CoreDNS - no LocalDNS (iptables discovery)
run_coredns_test "CoreDNS - No LocalDNS Iptables Discovery" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-no-localdns" \
    "success" \
    "success"

# CoreDNS - iptables discovery fallback
run_coredns_test "CoreDNS - Iptables Fallback Discovery" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-no-localdns" \
    "success" \
    "alternative"

# CoreDNS - TCP protocol success
run_coredns_test "CoreDNS - TCP Protocol Success" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-healthy" \
    "success"

# CoreDNS - UDP protocol success
run_coredns_test "CoreDNS - UDP Protocol Success" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-healthy" \
    "success"

# CoreDNS - kubelet flags discovery
run_coredns_test "CoreDNS - Kubelet Flags Discovery" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-kubelet-only" \
    "success" \
    "no-coredns"

# CoreDNS - corefile variant format
run_coredns_test "CoreDNS - Corefile Variant Format" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-corefile-variants" \
    "success"

# CoreDNS - multiple discovery sources
run_coredns_test "CoreDNS - Multiple Discovery Sources" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-multiple-sources" \
    "success" \
    "success"

# CoreDNS - fallback to default IP
run_coredns_test "CoreDNS - Fallback Default IP" \
    "WARNING: Could not discover CoreDNS IP from iptables, falling back to default: 10.0.0.10" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-fallback" \
    "success" \
    "no-coredns"

# CoreDNS - custom in-cluster domain
run_coredns_test "CoreDNS - Custom In-Cluster Domain" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-healthy" \
    "success" \
    "success" \
    "-e TEST_IN_CLUSTER_DOMAIN=test.default.svc.cluster.local"

# CoreDNS - iptables discovery failure
run_coredns_test "CoreDNS - Iptables Discovery Failure" \
    "WARNING: Could not discover CoreDNS IP from iptables, falling back to default: 10.0.0.10" \
    "$SCRIPT_DIR/testdata/mock-data/no-coredns-ip" \
    "success" \
    "no-coredns"

# Test 1: LocalDNS enabled - Basic VNet DNS extraction
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS Basic VNet DNS" \
    "168.63.129.16" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-basic" \
    0

# Test 2: LocalDNS enabled - Multiple VNet DNS IPs
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS Multiple VNet DNS IPs" \
    "168.63.129.16
8.8.8.8" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-multiple" \
    0

# Test 3: LocalDNS enabled - Comma separated VNet DNS IPs
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS Comma Separated IPs" \
    "1.1.1.1
168.63.129.16
8.8.8.8" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-comma" \
    0

# Test 4: LocalDNS enabled - Complex corefile with multiple zones
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS Complex Corefile" \
    "168.63.129.16
8.8.4.4" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-complex" \
    0

# Test 5: LocalDNS enabled - No VNet DNS section (empty result)
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS No VNet Section" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-no-vnet" \
    0

# Test 6: LocalDNS enabled - Malformed corefile
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS Malformed Corefile" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-malformed" \
    0

# Test 7: LocalDNS enabled - VNet section without proper bind
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS No VNet Bind" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-no-vnet-bind" \
    0

# Test 8: LocalDNS enabled - Different spacing and formatting
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS Different Formatting" \
    "168.63.129.16
8.8.8.8" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-formatting" \
    0

# Test 9: LocalDNS disabled - systemd resolv.conf
run_get_vnet_dns_ips_test "get_vnet_dns_ips - No LocalDNS Systemd Resolv" \
    "168.63.129.16
8.8.8.8" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-no-localdns-systemd" \
    0

# Test 10: LocalDNS disabled - fallback to /etc/resolv.conf
run_get_vnet_dns_ips_test "get_vnet_dns_ips - No LocalDNS Fallback Resolv" \
    "1.1.1.1
8.8.4.4" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-no-localdns-fallback" \
    0

# Test 11: LocalDNS disabled - empty resolv.conf files
run_get_vnet_dns_ips_test "get_vnet_dns_ips - No LocalDNS Empty Resolv" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-empty-resolv" \
    0

# Test 12: LocalDNS enabled - IPv6 addresses filtered out
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS IPv6 Filtering" \
    "168.63.129.16" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-ipv6" \
    0

# Test 13: LocalDNS enabled - Real AKS corefile format
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS Real AKS Format" \
    "168.63.129.16" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-real-aks" \
    0

# Test 14: LocalDNS enabled - Multiple root zones (only VNet should match)
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS Multiple Root Zones" \
    "168.63.129.16" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-multiple-roots" \
    0

# Test 15: LocalDNS enabled - Deduplication test
run_get_vnet_dns_ips_test "get_vnet_dns_ips - LocalDNS Deduplication" \
    "168.63.129.16
8.8.8.8" \
    "$SCRIPT_DIR/testdata/mock-data/vnet-dns-ips-duplicates" \
    0

# Test 1: Primary iptables discovery - dns-tcp cluster IP
run_get_coredns_ip_test "get_coredns_ip - Iptables DNS-TCP Discovery" \
    "10.0.0.10" \
    "" \
    "success" \
    0

# Test 2: Real AKS iptables format
run_get_coredns_ip_test "get_coredns_ip - Real AKS Iptables Format" \
    "10.0.0.10" \
    "" \
    "real-aks" \
    0

# Test 3: Edge case IP formats (single digits, large numbers)
run_get_coredns_ip_test "get_coredns_ip - Edge Case IP Formats" \
    "1.2.3.4" \
    "" \
    "edge-cases" \
    0

# Test 4: Fallback pattern - any kube-dns cluster IP
run_get_coredns_ip_test "get_coredns_ip - Fallback Pattern Discovery" \
    "10.0.0.25" \
    "" \
    "fallback-pattern" \
    0

# Test 5: Alternative iptables format (broader kube-dns pattern)
run_get_coredns_ip_test "get_coredns_ip - Alternative Iptables Format" \
    "10.0.0.15" \
    "" \
    "alternative" \
    0

# Test 6: All discovery methods fail - default IP with warning
run_get_coredns_ip_test "get_coredns_ip - Default IP Fallback" \
    "WARNING: Could not discover CoreDNS IP from iptables, falling back to default: 10.0.0.10" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-fallback" \
    "no-coredns" \
    0

# Test 7: Multiple discovery sources - iptables takes precedence
run_get_coredns_ip_test "get_coredns_ip - Iptables Precedence" \
    "10.0.0.10" \
    "$SCRIPT_DIR/testdata/mock-data/coredns-multiple-sources" \
    "success" \
    0

# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi