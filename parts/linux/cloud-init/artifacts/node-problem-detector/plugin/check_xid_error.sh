#!/bin/bash

# This plugin checks for GPU XID errors in the system logs.
# XID errors indicate critical GPU hardware/driver issues that need attention.
set -euo pipefail

readonly OK=0
readonly NONOK=1

# XID error codes that indicate serious GPU issues
# Check this Nvidia documentation for more details:
# https://docs.nvidia.com/deploy/xid-errors/index.html
readonly XID_ERROR_CODES="48 56 57 58 62 63 64 65 68 69 73 74 79 80 81 92 119 120"

# Time threshold in seconds to consider XID errors recent (2 hours = 7200 seconds)
readonly TIME_THRESHOLD_SECONDS=7200
readonly XID_LAST_SEEN_TIME_SEC="/tmp/npd_xid_last_seen.cache"

function verify_gpu_support() {
    # Check if nvidia-smi exists (this should only run on GPU nodes)
    if ! command -v nvidia-smi >/dev/null 2>&1; then
        echo "nvidia-smi not found. Skipping XID error check."
        exit "${OK}"
    fi
}

function parse_syslog_date_with_year() {
    local syslog_date_str="$1"
    local current_year
    local current_date_sec
    local parsed_date_sec

    current_year=$(date +%Y)
    current_date_sec=$(date +%s)

    # Try parsing with current year first
    if parsed_date_sec=$(date --date "$syslog_date_str $current_year" +%s 2>/dev/null); then
        # If the parsed date is more than 30 days in the future, it's likely from last year
        local diff_secs=$((parsed_date_sec - current_date_sec))
        if [ $diff_secs -lt $((30 * 24 * 60 * 60)) ]; then
            echo "$parsed_date_sec"
            return 0
        fi
    fi

    # Fallback: try previous year if current year parsing failed and the
    # difference is larger than 30 days.
    local prev_year=$((current_year - 1))
    if parsed_date_sec=$(date --date "$syslog_date_str $prev_year" +%s 2>/dev/null); then
        echo "$parsed_date_sec"
        return 0
    fi

    # If all parsing attempts fail, return error
    return 1
}

function get_last_seen_timestamp() {
    cat "${XID_LAST_SEEN_TIME_SEC}" 2>/dev/null || echo "0"
}

# Check system logs for XID errors
# On AKS Ubuntu nodes, we check /var/log/syslog for kernel messages
check_xid_errors() {
    local xid_error_codes_found=""

    # Check current syslog and recent rotated logs
    for syslog_file in /var/log/syslog /var/log/syslog.1; do
        # Check if syslog file exists and is readable
        if [ ! -f "$syslog_file" ] || [ ! -r "$syslog_file" ]; then
            continue
        fi

        # Get all XID errors in one pass for better performance
        local all_xid_lines
        all_xid_lines=$(grep "Xid.*:" "$syslog_file" 2>/dev/null || true)

        # Check if any XID errors were found
        if [ -z "$all_xid_lines" ]; then
            continue
        fi

        local last_seen_timestamp_seconds
        last_seen_timestamp_seconds=$(get_last_seen_timestamp)

        local log_timestamp_seconds
        # Check each critical XID error code
        for xid_code in $XID_ERROR_CODES; do
            local xid_line
            local log_timestamp
            local current_ts
            local time_diff_seconds

            # Look for the most recent occurrence of this XID error
            xid_line=$(echo "$all_xid_lines" | grep ": $xid_code," | tail -n 1 || true)

            if [ -z "$xid_line" ]; then
                continue
            fi

            # Extract timestamp and check if error is recent
            # Syslog format: "Jan 15 10:30:45 hostname kernel: ..."
            # Extract timestamp from syslog format (first 3 fields: Month Day Time)
            log_timestamp=$(echo "$xid_line" | awk '{print $1, $2, $3}')
            log_timestamp_seconds=$(parse_syslog_date_with_year "$log_timestamp")

            # Skip this syslog line, if this line's timestamp is less
            # than or equal to the last seen timestamp, this means we've
            # already seen this error line.
            if [ "$log_timestamp_seconds" -le "$last_seen_timestamp_seconds" ]; then
                continue
            fi

            current_ts=$(date +"%s")
            time_diff_seconds=$((current_ts - log_timestamp_seconds))

            # Check if the log line is within our time threshold of 2 hours.
            if [ "$time_diff_seconds" -gt $TIME_THRESHOLD_SECONDS ]; then
                continue
            fi

            xid_error_codes_found="$xid_error_codes_found$xid_code, "

            # Save this log's timestamp as the last seen if it is bigger than the last seen timestamp
            if [ "$log_timestamp_seconds" -gt "$last_seen_timestamp_seconds" ]; then
                echo "$log_timestamp_seconds" >"${XID_LAST_SEEN_TIME_SEC}"
            fi
        done

    done

    # Report results
    if [ -z "$xid_error_codes_found" ]; then
        # Remove the trailing ", " from the string
        echo "No recent GPU XID errors found in system logs"
        exit "${OK}"
    fi

    xid_error_codes_found=${xid_error_codes_found%, }
    echo "XID error-codes found: ${xid_error_codes_found}. FaultCode: NHC2001"
    exit "${NONOK}"
}

verify_gpu_support
# Run the XID error check
check_xid_errors
