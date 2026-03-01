#!/bin/bash

# This file contains shared variables and functions used by:
# - check_dns_to_azuredns.sh
# - check_dns_to_coredns.sh  
# - check_dns_to_localdns.sh

# shellcheck disable=SC2034  
# Variables are used by scripts that source this file.
readonly OK=0
readonly NOTOK=1
readonly UNKNOWN=2

readonly TEST_IN_CLUSTER_DOMAIN="kubernetes.default.svc.cluster.local"
readonly TEST_EXTERNAL_DOMAIN="mcr.microsoft.com"

readonly UDP_PROTOCOL="udp"
readonly TCP_PROTOCOL="tcp"

readonly COMMAND_TIMEOUT_SECONDS=4
readonly DIG_TIMEOUT_SECONDS=3

readonly RETRY_COUNT=2
readonly RETRY_DELAY=1

readonly LOCALDNS_NODE_LISTENER_IP="169.254.10.10"
readonly LOCALDNS_CLUSTER_LISTENER_IP="169.254.10.11"

readonly LOCALDNS_SCRIPT_PATH="/opt/azure/containers/localdns"
readonly LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/updated.localdns.corefile"

readonly KUBELET_DEFAULT_FLAG_FILE="/etc/default/kubelet"

readonly LOCALDNS_STATE_LABEL="kubernetes\.azure\.com\/localdns-state"
readonly SYSTEMD_RESOLV_CONF="/run/systemd/resolve/resolv.conf"

# -----------------------------------------------------------------------------
# Function: check_dependencies
#
# Verifies that required commands are available on the system.
# Note: We exit gracefully if dependencies are missing to ensure we only 
# event when we know there's a problem.
# -----------------------------------------------------------------------------
check_dependencies() {
    local missing_deps=()
    
    # Check for required commands.
    if ! command -v dig >/dev/null 2>&1; then
        missing_deps+=("dig")
    fi
    
    if [ ${#missing_deps[@]} -gt 0 ]; then
        echo "WARNING: Missing required dependencies: ${missing_deps[*]}. Exiting gracefully." >&2
        exit $OK
    fi
}

# -----------------------------------------------------------------------------
# Function: wait_for_kubelet_flags_file
#
# Waits for the kubelet flags file to be available with timeout.
# -----------------------------------------------------------------------------
wait_for_kubelet_flags_file() {
    local timeout=3  # Wait max 3 seconds
    local elapsed=0
    
    while [ ! -f "${KUBELET_DEFAULT_FLAG_FILE}" ] && [ $elapsed -lt $timeout ]; do
        sleep 1
        elapsed=$((elapsed + 1))
    done
}

# -----------------------------------------------------------------------------
# Function: is_localdns_enabled
#
# Checks if LocalDNS is enabled by looking for localdns corefile.
# returns 0 if localdns corefile exists and not empty, 1 if not.
# -----------------------------------------------------------------------------
is_localdns_enabled() {
    if [ -s "$LOCALDNS_CORE_FILE" ]; then
        return 0
    else
        return 1
    fi
}

# -----------------------------------------------------------------------------
# Function: get_kubelet_config_file_path
#
# Extracts the kubelet config file path from systemd environment.
# Returns the config file path if KUBELET_CONFIG_FILE_FLAGS is set, empty otherwise.
# -----------------------------------------------------------------------------
get_kubelet_config_file_path() {
    local env
    local env_kubeletconfig_flag
    local kubeletconfig_path

    # Get kubelet environment with timeout to prevent hanging.
    env=$(timeout "$COMMAND_TIMEOUT_SECONDS" systemctl show kubelet --property=Environment 2>/dev/null | sed 's/^Environment=//')
    if [ -z "$env" ]; then
        return
    fi

    # Extract the entire KUBELET_CONFIG_FILE_FLAGS entry.
    env_kubeletconfig_flag=$(
        echo "$env" \
        | sed 's/[[:space:]]\(["A-Z_][A-Z_]*=\)/\n\1/g' \
        | sed 's/^"\(.*\)"$/\1/' \
        | sed -n 's/^KUBELET_CONFIG_FILE_FLAGS=\(.*\)$/\1/p'
    )
    if [ -z "$env_kubeletconfig_flag" ]; then
        return
    fi

    # Extract only the path following --config
    kubeletconfig_path=$(echo "$env_kubeletconfig_flag" | sed -n 's/.*--config[ =]\([^ ]*\).*/\1/p' | head -1)
    if [ -n "$kubeletconfig_path" ]; then
        echo "$kubeletconfig_path"
    fi
}

# -----------------------------------------------------------------------------
# Function: get_coredns_ip_from_kubelet_json_file
#
# Extracts CoreDNS IP from kubelet JSON configuration file.
# Returns the clusterDNS IP if found, empty otherwise.
# -----------------------------------------------------------------------------
get_coredns_ip_from_kubelet_json_file() {
    local json_file
    local clusterdns_ip
    
    # Get the kubelet json file path.
    # Typically - /etc/default/kubeletconfig.json
    json_file=$(get_kubelet_config_file_path)
    
    if [ -z "$json_file" ]; then
        return
    fi
    
    # Wait for the JSON config file to be available.
    local timeout=3  # Wait max 3 seconds
    local elapsed=0
    
    while [ ! -s "$json_file" ] && [ $elapsed -lt $timeout ]; do
        sleep 1
        elapsed=$((elapsed + 1))
    done
    
    # Extract clusterDNS IP if file exists and is readable.
    if [ -n "$json_file" ] && [ -f "$json_file" ] && [ -r "$json_file" ]; then
        clusterdns_ip=$(
            tr -d '\n' < "$json_file" |
            sed -n 's/.*"clusterDNS"[[:space:]]*:[[:space:]]*\[[[:space:]]*"\([0-9]\{1,3\}\(\.[0-9]\{1,3\}\)\{3\}\)".*/\1/p'
            )
        echo "$clusterdns_ip"
    fi
}

# -----------------------------------------------------------------------------
# Function: check_dns_with_retry
#
# Performs DNS resolution check with retry logic.
# Usage:
#   check_dns_with_retry "domain.name" "server_ip" "protocol" "server"
#
# Parameters:
#   domain: Domain to resolve
#   server_ip: DNS server IP
#   protocol: "tcp" or "udp" (optional, defaults to udp)
#   server: Name of the DNS server
#
# On success, returns 0. On failure after all retries, returns 1.
# -----------------------------------------------------------------------------
check_dns_with_retry() {
    local domain="$1"
    local server_ip="$2"
    local protocol="${3:-udp}"
    local server="$4"
    
    if [ -z "$domain" ] || [ -z "$server_ip" ] || [ -z "$server" ]; then
        echo "Domain, server IP, and server name are required for DNS check"
        return 1
    fi

    local dig_cmd=(dig +short +timeout="$DIG_TIMEOUT_SECONDS" +tries=2 "$domain" @"$server_ip")
    local shell_timeout=$((DIG_TIMEOUT_SECONDS * 2))
    
    # Add TCP flag if specified.
    if [ "$protocol" = "tcp" ]; then
        dig_cmd+=(+tcp)
    fi
    
    # Use shell timeout as safety net (2x dig timeout to allow for retries)
    if timeout "$shell_timeout" "${dig_cmd[@]}" >/dev/null 2>&1; then
        return 0
    else
        echo "dns test to $server:$server_ip over $protocol failed after 2 attempts"
        return 1
    fi
}

# -----------------------------------------------------------------------------
# Function: get_vnet_dns_ips
#
# Extracts VNet DNS IPs from LocalDNS Corefile or system resolv.conf.
# For LocalDNS: looks for root zone with VNet DNS override binding.
# For non-LocalDNS: returns nameservers from systemd's resolv.conf.
# -----------------------------------------------------------------------------
get_vnet_dns_ips() {
    if is_localdns_enabled; then
        # Extract VNet DNS IPs from localdns Corefile with timeout protection.
        # Look for exact root zone sections ".:53 {" with VNet DNS override binding (bind 169.254.10.10)
        # and extract all forward IPs from that specific section.
        # shellcheck disable=SC2016 # awk variables like $1 must not be expanded by bash
        timeout "$COMMAND_TIMEOUT_SECONDS" awk '
        /^\.[ ]*:[ ]*53[ ]*\{/ { 
            in_vnet_root_zone=1
            has_vnet_bind=0
            next
        }
        in_vnet_root_zone && /^[ ]*bind.*169\.254\.10\.10/ {
            has_vnet_bind=1
        }
        in_vnet_root_zone && /^[^#[:space:]].*:[ ]*53[ ]*\{/ { 
            in_vnet_root_zone=0
            has_vnet_bind=0
        }
        in_vnet_root_zone && /^[ ]*\}/ { 
            in_vnet_root_zone=0
            has_vnet_bind=0
        }
        in_vnet_root_zone && has_vnet_bind && /^[ ]*forward[ ]+\.[ ]+/ {
            # Extract all IP addresses from the forward line
            # Handle both single IP and multiple IPs (space or comma separated)
            gsub(/^[ ]*forward[ ]+\.[ ]+/, "")  # Remove everything up to "forward . "
            gsub(/[ ]*\{.*$/, "")              # Remove everything from { onwards
            gsub(/,/, " ")                     # Convert commas to spaces
            # Split the line by spaces and print each IP
            for(i=1; i<=NF; i++) {
                if($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/) {
                    print $i
                }
            }
        }' "$LOCALDNS_CORE_FILE" 2>/dev/null | sort -u
    else
        # If LocalDNS is not enabled, return nameservers from systemd's resolv.conf with timeout protection.
        if [ -s "$SYSTEMD_RESOLV_CONF" ]; then
            timeout "$COMMAND_TIMEOUT_SECONDS" grep -Ei '^nameserver' "$SYSTEMD_RESOLV_CONF" 2>/dev/null | awk '{print $2}' | sort -u
        else
            timeout "$COMMAND_TIMEOUT_SECONDS" grep -Ei '^nameserver' /etc/resolv.conf 2>/dev/null | awk '{print $2}' | sort -u
        fi
    fi
}

# -----------------------------------------------------------------------------
# Function: get_coredns_ip
#
# Discovers CoreDNS IP address from multiple sources with fallbacks.
# Returns the CoreDNS service IP or a default value.
# -----------------------------------------------------------------------------
get_coredns_ip() {
    local coredns_ip=""

    if is_localdns_enabled; then
        # Extract from localdns Corefile.
        coredns_ip=$(
            awk '
            /^cluster\.local:/ { zone=1 }
            zone && /forward \. [0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/ {
                gsub(/^.*forward \. /, "")  # Remove everything up to "forward . "
                gsub(/[ {].*$/, "")         # Remove everything after the first IP (space or brace)
                print $0
                exit
            }' "$LOCALDNS_CORE_FILE"
        )
    else
        # Try kubelet flags file first (most common case).
        wait_for_kubelet_flags_file
        
        if [ -f "${KUBELET_DEFAULT_FLAG_FILE}" ]; then
            coredns_ip=$(sed -n 's/.*--cluster-dns=\([0-9]\{1,3\}\(\.[0-9]\{1,3\}\)\{3\}\).*/\1/p' "$KUBELET_DEFAULT_FLAG_FILE" | head -1)
        fi

        # Fallback to kubelet JSON config file.
        if [ -z "$coredns_ip" ]; then
            coredns_ip=$(get_coredns_ip_from_kubelet_json_file)
        fi
    fi
    
    # Validate IP format and return.
    if [ -n "$coredns_ip" ] && [[ "$coredns_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "$coredns_ip"
    fi
}

# -----------------------------------------------------------------------------
# Function: get_localdns_state_label
#
# Extracts the localdns state label from kubelet node labels.
# Returns localdns state label (enabled/disabled).
# -----------------------------------------------------------------------------
get_localdns_state_label() {
    wait_for_kubelet_flags_file

    if [ ! -f "${KUBELET_DEFAULT_FLAG_FILE}" ]; then
        echo "WARNING: kubelet flags file not found after waiting, exiting gracefully" >&2
        exit $OK
    fi

    # Extract the localdns-state value from KUBELET_NODE_LABELS using the defined variable.
    local localdns_state_label
    if ! timeout "$COMMAND_TIMEOUT_SECONDS" grep -q '^KUBELET_NODE_LABELS=' "$KUBELET_DEFAULT_FLAG_FILE" 2>/dev/null; then
        echo "WARNING: KUBELET_NODE_LABELS not found in kubelet flags file" >&2
        exit $OK
    fi
    
    localdns_state_label=$(timeout "$COMMAND_TIMEOUT_SECONDS" grep -E '^KUBELET_NODE_LABELS=' "$KUBELET_DEFAULT_FLAG_FILE" 2>/dev/null | \
        sed -n "s/.*${LOCALDNS_STATE_LABEL}=\([^,]*\).*/\1/p")
    
    if [ -z "$localdns_state_label" ]; then
        echo "WARNING: cannot extract localdns-state label from kubelet node labels" >&2
        exit $OK
    fi
    
    echo "$localdns_state_label"
}