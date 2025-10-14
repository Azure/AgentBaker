#!/usr/bin/env bash
# This plugin checks for IB link flapping events in the system log. It expects
# to not have any IB link flaps within a given time interval (default is 6
# hours).
#
# The script checks the system log for entries indicating IB link flapping
# events, and compares the timestamps of the last event with the last recorded
# event in a local log file. A local log file is used so that it does not emit
# duplicate alerts for the same event.

set -euo pipefail

readonly OK=0
readonly NONOK=1

readonly IB_FLAPPING_LINK_TEST="IB link flapping detected"
: "${SYS_LOG:=/var/log/syslog}"
: "${TIME_INTERVAL_HOURS:=6}"

# Convert TIME_INTERVAL_HOURS to seconds for timestamp arithmetic operations.
# This is needed because the script compares Unix timestamps (in seconds) to
# determine if multiple IB link flapping events occurred within the specified
# time window. The date command outputs timestamps in seconds since epoch, so we
# need the interval in the same unit for accurate time difference calculations.
((TIME_INTERVAL_SECS = "${TIME_INTERVAL_HOURS}" * 60 * 60))

readonly IB_LINK_EVENTS_LOG="/tmp/npd_ib_link_flapping_events.log"
[ -f "${IB_LINK_EVENTS_LOG}" ] || touch "${IB_LINK_EVENTS_LOG}"

function die() {
    local exit_code=$1
    shift
    echo "$*"
    echo "$(date +'%Y-%m-%d %H:%M:%S') [ERROR] $*" >&2
    exit "${exit_code}"
}

function log() {
    echo "$(date +'%Y-%m-%d %H:%M:%S') [INFO] $1" >>${IB_LINK_EVENTS_LOG}
}

function pass() {
    local exit_code=$1
    shift
    echo "$(date +'%Y-%m-%d %H:%M:%S') [PASS] $*" >>${IB_LINK_EVENTS_LOG}
    exit "${exit_code}"
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

function check_log_entries() {
    local last_entry_in_local_log
    last_entry_in_local_log=$(grep -F "Linkflap event:" ${IB_LINK_EVENTS_LOG} | tail -n 1 || true)
    if [ -n "$last_entry_in_local_log" ]; then
        # For an entry like this, extract Jun 11 23:08:51:
        # 2025-06-11 23:08:52 [INFO] Linkflap event: Jun 11 23:08:51
        echo "${last_entry_in_local_log}" | awk -F 'Linkflap event: ' '{print $2}'
    else
        echo ""
    fi
    return 0
}

function check_ib_link_flapping() {
    local syslog_lost_carrier_line
    local syslog_lost_carrier_array
    local syslog_last_date_str
    local syslog_last_date_sec
    local local_log_last_date_str
    local local_log_last_date_sec
    local diff_secs

    # Find the last entry in syslog that indicates an IB link flapping event
    syslog_lost_carrier_line=$(grep -i "ib.*lost carrier" "${SYS_LOG}" | tail -n 1 || true)

    if [ -z "${syslog_lost_carrier_line}" ]; then
        log "No IB link flapping entry in syslog"
        pass $OK "${FUNCNAME[0]}: No IB link flapping found"
    fi

    # Convert the string into an array to extract the month, day and time. A
    # typical syslog entry looks like this:
    # Jun 11 22:26:32 hostname systemd-networkd[1587]: azv069c8894585: Lost carrier
    IFS=" " read -r -a syslog_lost_carrier_array <<<"${syslog_lost_carrier_line}"

    # Verify we have at least 3 elements (month, day, time) before accessing array indices
    if [ "${#syslog_lost_carrier_array[@]}" -lt 3 ]; then
        log "${FUNCNAME[0]}: Invalid syslog entry format - expected at least 3 fields (month day time), got ${#syslog_lost_carrier_array[@]} fields. Entry: ${syslog_lost_carrier_line}"
        exit $OK
    fi

    # Concatenate the first three elements of the array to form a date string,
    # e.g.: Jun 11 22:26:32
    syslog_last_date_str="${syslog_lost_carrier_array[0]} ${syslog_lost_carrier_array[1]} ${syslog_lost_carrier_array[2]}"

    # Parse the syslog date with intelligent year handling
    if ! syslog_last_date_sec=$(parse_syslog_date_with_year "$syslog_last_date_str"); then
        log "${FUNCNAME[0]}: Failed to parse syslog date: $syslog_last_date_str"
        exit $OK
    fi

    # Extract the last date from the local log file
    local_log_last_date_str=$(check_log_entries)

    # This only runs when we encounter the first ever IB link flapping event, so
    # we will create the first entry in the local log file.
    if [ -z "$local_log_last_date_str" ]; then
        log "No Link flap entry, so will create it with $syslog_last_date_str"
        log "Linkflap event: $syslog_last_date_str"
        exit $OK
    fi

    # Check if the last entry in the local log is different from the current
    # syslog one.
    if [ "$syslog_last_date_str" = "$local_log_last_date_str" ]; then
        log "No new IB link flapping events detected"
        pass $OK "${FUNCNAME[0]}: No new IB link flaps found"
    fi

    # Parse the local log date with intelligent year handling
    if ! local_log_last_date_sec=$(parse_syslog_date_with_year "$local_log_last_date_str"); then
        log "${FUNCNAME[0]}: Failed to parse local log date: $local_log_last_date_str"
        exit $OK
    fi

    ((diff_secs = syslog_last_date_sec - local_log_last_date_sec))

    # If a no new IB link flapping event is detected within the defined time
    # interval, log it and exit without an error.
    if [ $diff_secs -gt $TIME_INTERVAL_SECS ]; then
        log "Time interval > $TIME_INTERVAL_HOURS, No new IB link flapping event detected"
        exit $OK
    fi

    # If a new IB link flapping event is detected within the defined time
    # interval, log it and exit with an error.
    log "Linkflap event: $syslog_last_date_str"
    log "$IB_FLAPPING_LINK_TEST, multiple IB link flapping events within $TIME_INTERVAL_HOURS hours ($local_log_last_date_str, $syslog_last_date_str)"
    die $NONOK "${FUNCNAME[0]}: $IB_FLAPPING_LINK_TEST, multiple IB link flapping events within $TIME_INTERVAL_HOURS hours. FaultCode: NHC2005"
}

check_ib_link_flapping
