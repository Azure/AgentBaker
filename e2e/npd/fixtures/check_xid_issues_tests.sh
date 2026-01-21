#!/bin/bash
# Test script for NPD GPU XID error checks

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common/test_common.sh"

# Script under test
readonly SCRIPT_UNDER_TEST="/etc/node-problem-detector.d/plugin/check_xid_error.sh"

# Helper function to run XID error check
run_xid_test() {
    local test_name="$1"
    local expected_output="$2"
    local mock_data_dir="${3:-$SCRIPT_DIR/testdata/mock-data/check-xid-errors}"
    local custom_env_vars="$4"
    local timeout_seconds="${5:-30}"
    local expected_exit_code="${6:-0}"

    # Build volume mounts
    local volume_mounts=""

    # Mount scenario-specific mock commands
    if [ -d "$mock_data_dir/mock-commands" ]; then
        volume_mounts+="-v \"$mock_data_dir/mock-commands:/mock-commands:ro\""
    fi

    # Mount scenario-specific configuration files to non-conflicting paths
    if [ -d "$mock_data_dir" ]; then
        [ -d "$mock_data_dir/var" ] && volume_mounts+=" -v \"$mock_data_dir/var:/mock-var:ro\""
        [ -d "$mock_data_dir/tmp" ] && volume_mounts+=" -v \"$mock_data_dir/tmp:/mock-tmp:rw\""
    fi

    # NPD-specific diagnostic keywords for better error reporting
    local npd_keywords=""

    # Call the generic run_test function with timeout, expected exit code, and diagnostic keywords
    run_test "$SCRIPT_UNDER_TEST" "$test_name" "$expected_output" "$volume_mounts" "$custom_env_vars" "$timeout_seconds" "$expected_exit_code" "$npd_keywords"
}

start_fixture "check_xid_error.sh Tests"

add_section "XID Error Check without Errors"
run_xid_test "XID Error Check - No Errors" \
    "No recent GPU XID errors found in system logs" \
    "$SCRIPT_DIR/testdata/mock-data/check-xid-no-errors" "" 30 0

add_section "XID Error Check with Error"

run_xid_test "XID Error Check - Single XID Error" \
    "XID error-codes found: 48. FaultCode: NHC2001" \
    "$SCRIPT_DIR/testdata/mock-data/check-xid-errors-48" "" 30 1

run_xid_test "XID Error Check - Multiple XID Error" \
    "XID error-codes found: 48, 56. FaultCode: NHC2001" \
    "$SCRIPT_DIR/testdata/mock-data/check-xid-errors-48-56" "" 30 1

# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi
