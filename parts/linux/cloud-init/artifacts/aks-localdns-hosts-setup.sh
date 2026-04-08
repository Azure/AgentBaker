#!/bin/bash
set -euo pipefail

# aks-localdns-hosts-setup.sh
# Resolves A and AAAA records for critical AKS FQDNs and populates /etc/localdns/hosts.
# LOCALDNS_CRITICAL_FQDNS is set by CSE (cse_cmd.sh) and persisted via /etc/localdns/environment
# as a systemd EnvironmentFile so it's available on both initial and timer-triggered runs.
#
# Logging: All output goes to journald via the aks-localdns-hosts-setup.service unit.
#   View logs:   journalctl -u aks-localdns-hosts-setup --no-pager
#   Collected by aks-log-collector.sh into the guest agent log archive.

HOSTS_FILE="/etc/localdns/hosts"

# Ensure the directory exists
mkdir -p "$(dirname "$HOSTS_FILE")"

# Use LOCALDNS_CRITICAL_FQDNS directly. It's available from:
#   1. CSE environment (initial run from enableAKSLocalDNSHostsSetup)
#   2. Systemd EnvironmentFile (timer-triggered runs via aks-localdns-hosts-setup.service)
# If LOCALDNS_CRITICAL_FQDNS is not set or empty, exit gracefully — this means
# the RP didn't pass FQDNs (old RP). The corefile falls back to the no-hosts variant.
if [ -z "${LOCALDNS_CRITICAL_FQDNS:-}" ]; then
    echo "LOCALDNS_CRITICAL_FQDNS is not set or empty. RP did not pass critical FQDNs."
    echo "Exiting without modifying hosts file. Corefile will use no-hosts variant."
    exit 0
fi

# Parse the comma-separated FQDN list into an array.
IFS=',' read -ra CRITICAL_FQDNS <<< "${LOCALDNS_CRITICAL_FQDNS}"

echo "Received ${#CRITICAL_FQDNS[@]} critical FQDNs from RP"

# Determine upstream DNS server(s) for dig queries.
# Once localdns is running, /etc/resolv.conf points to 169.254.10.10 (localdns itself).
# If the hosts plugin is active, dig without @server would get answers from
# /etc/localdns/hosts — creating a self-referential loop where stale IPs can never refresh.
# To break this loop, we resolve against the upstream DNS server(s) that localdns forwards to,
# persisted by localdns.sh (replace_azurednsip_in_corefile) to /etc/localdns/upstream-dns.
# If the file doesn't exist yet (first run during CSE, before localdns starts), we don't pin
# any upstream — dig uses the system resolver, which still points to the real upstream at that point.
UPSTREAM_DNS_FILE="/etc/localdns/upstream-dns"
UPSTREAM_DNS_SERVERS=""
if [ -f "${UPSTREAM_DNS_FILE}" ]; then
    # File contains space-separated DNS server IPs (e.g., "10.0.0.4 10.0.0.5" or "168.63.129.16").
    UPSTREAM_DNS_SERVERS=$(cat "${UPSTREAM_DNS_FILE}" 2>/dev/null | tr '\n' ' ')
fi
if [ -n "${UPSTREAM_DNS_SERVERS}" ]; then
    echo "Using upstream DNS servers: ${UPSTREAM_DNS_SERVERS}"
else
    echo "No upstream DNS file found, using system resolver"
fi

# Function to resolve IPv4 addresses for a domain
# Filters output to only include valid IPv4 addresses (rejects NXDOMAIN, SERVFAIL, hostnames, etc.)
# Tries each upstream DNS server until one succeeds. If no upstream servers are configured,
# uses the system resolver (appropriate for first run before localdns starts).
resolve_ipv4() {
    local domain="$1"
    local output=""
    if [ -n "${UPSTREAM_DNS_SERVERS}" ]; then
        for server in ${UPSTREAM_DNS_SERVERS}; do
            output=$(timeout 3 dig +short -t A "${domain}" @"${server}" 2>/dev/null) && break
        done
    else
        output=$(timeout 3 dig +short -t A "${domain}" 2>/dev/null) || return 0
    fi
    # Validate IPv4 format with octet range 0-255.
    # Uses here-string (<<<) instead of a grep|while pipeline to avoid
    # pipefail complications when grep matches nothing.
    while IFS= read -r line; do
        if [[ "$line" =~ ^([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})$ ]]; then
            [[ ${BASH_REMATCH[1]} -le 255 && ${BASH_REMATCH[2]} -le 255 && \
               ${BASH_REMATCH[3]} -le 255 && ${BASH_REMATCH[4]} -le 255 ]] && echo "$line"
        fi
    done <<< "${output}"
}

# Function to resolve IPv6 addresses for a domain
# Filters output to only include valid IPv6 addresses (rejects NXDOMAIN, SERVFAIL, hostnames, etc.)
# Tries each upstream DNS server until one succeeds. If no upstream servers are configured,
# uses the system resolver (appropriate for first run before localdns starts).
resolve_ipv6() {
    local domain="$1"
    local output=""
    if [ -n "${UPSTREAM_DNS_SERVERS}" ]; then
        for server in ${UPSTREAM_DNS_SERVERS}; do
            output=$(timeout 3 dig +short -t AAAA "${domain}" @"${server}" 2>/dev/null) && break
        done
    else
        output=$(timeout 3 dig +short -t AAAA "${domain}" 2>/dev/null) || return 0
    fi
    # Validate IPv6 format using here-string to avoid pipefail complications.
    # Three-stage check rejects malformed dig output:
    #   1. Only hex digits and colons, minimum 3 chars (rejects hostnames, error strings)
    #   2. At least two colons (rejects "1:2", ":ff" — too short to be valid IPv6)
    #   3. At least one hex digit (rejects all-colon strings like ":::::::")
    while IFS= read -r line; do
        if [[ "$line" =~ ^[0-9a-fA-F:]{3,}$ ]] && \
           [[ "$line" == *:*:* ]] && \
           [[ "$line" =~ [0-9a-fA-F] ]]; then
            echo "$line"
        fi
    done <<< "${output}"
}

echo "Starting AKS critical FQDN hosts resolution at $(date)"

# Track if we resolved at least one address
RESOLVED_ANY=false

# Start building the hosts file content
HOSTS_CONTENT="# AKS critical FQDN addresses resolved at $(date)
# This file is automatically generated by aks-localdns-hosts-setup.service
"

# Resolve each FQDN
for DOMAIN in "${CRITICAL_FQDNS[@]}"; do
    echo "Resolving addresses for ${DOMAIN}..."

    # Get IPv4 and IPv6 addresses using helper functions
    IPV4_ADDRS=$(resolve_ipv4 "${DOMAIN}")
    IPV6_ADDRS=$(resolve_ipv6 "${DOMAIN}")

    # Check if we got any results for this domain
    if [ -z "${IPV4_ADDRS}" ] && [ -z "${IPV6_ADDRS}" ]; then
        echo "  WARNING: No IP addresses resolved for ${DOMAIN}"
        continue
    fi

    RESOLVED_ANY=true
    HOSTS_CONTENT+="
# ${DOMAIN}"

    if [ -n "${IPV4_ADDRS}" ]; then
        for addr in ${IPV4_ADDRS}; do
            HOSTS_CONTENT+="
${addr} ${DOMAIN}"
        done
    fi

    if [ -n "${IPV6_ADDRS}" ]; then
        for addr in ${IPV6_ADDRS}; do
            HOSTS_CONTENT+="
${addr} ${DOMAIN}"
        done
    fi
done

# Check if we resolved at least one domain
if [ "${RESOLVED_ANY}" != "true" ]; then
    echo "WARNING: No IP addresses resolved for any domain at $(date)"
    echo "This is likely a temporary DNS issue. Timer will retry later."
    # Keep existing hosts file intact and exit successfully so systemd doesn't mark unit as failed
    exit 0
fi

# Write the hosts file atomically: write to a temp file in the same directory,
# validate it, then rename it over the target. rename(2) on the same filesystem
# is atomic, so CoreDNS (or any other reader) never sees invalid or truncated data.
echo "Writing addresses to ${HOSTS_FILE}..."
HOSTS_TMP="${HOSTS_FILE}.tmp.$$"

# Write content to temp file with explicit error checking
if ! printf '%s\n' "${HOSTS_CONTENT}" > "${HOSTS_TMP}"; then
    echo "ERROR: Failed to write to temporary file ${HOSTS_TMP}"
    rm -f "${HOSTS_TMP}"  # Clean up temp file
    exit 1
fi

# Set permissions with explicit error checking
if ! chmod 0644 "${HOSTS_TMP}"; then
    echo "ERROR: Failed to chmod temporary file ${HOSTS_TMP}"
    rm -f "${HOSTS_TMP}"  # Clean up temp file
    exit 1
fi

# Validate temp file BEFORE moving into place to ensure we never publish invalid data
# Verify the file was written and has content
if [ ! -s "${HOSTS_TMP}" ]; then
    echo "ERROR: Temporary hosts file ${HOSTS_TMP} is empty or does not exist after write"
    rm -f "${HOSTS_TMP}"
    exit 1
fi

# Verify that every non-comment, non-empty line has the format: <IP> <FQDN>
# This ensures we don't have any lines with FQDN but missing IP address
echo "Validating hosts file entries format..."
INVALID_LINES=()
VALID_ENTRIES=0
while IFS= read -r line; do
    # Skip comments and empty lines
    if [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "$line" ]]; then
        continue
    fi

    # Check if line has at least two fields (IP and FQDN)
    ip=$(echo "$line" | awk '{print $1}')
    fqdn=$(echo "$line" | awk '{print $2}')

    # Critical check: ensure we have both IP and FQDN (no empty IP mappings)
    if [ -z "$ip" ] || [ -z "$fqdn" ]; then
        echo "ERROR: Invalid entry found - missing IP or FQDN: '$line'"
        INVALID_LINES+=("$line")
        continue
    fi

    # Validate IP format (IPv4 or IPv6)
    if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        # Valid IPv4
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    elif [[ "$ip" =~ : ]]; then
        # Valid IPv6 (contains colon)
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    else
        echo "ERROR: Invalid IP format: '$ip' in line: '$line'"
        INVALID_LINES+=("$line")
    fi
done < "${HOSTS_TMP}"

if [ ${#INVALID_LINES[@]} -gt 0 ]; then
    echo "ERROR: Found ${#INVALID_LINES[@]} invalid entries in temporary hosts file"
    echo "Invalid entries:"
    printf '%s\n' "${INVALID_LINES[@]}"
    echo "This indicates FQDN to empty IP mappings or malformed entries"
    rm -f "${HOSTS_TMP}"
    exit 1
fi

if [ $VALID_ENTRIES -eq 0 ]; then
    echo "ERROR: No valid IP address mappings found in temporary hosts file"
    echo "File content:"
    cat "${HOSTS_TMP}"
    rm -f "${HOSTS_TMP}"
    exit 1
fi

echo "✓ All entries in temporary hosts file are valid (IP FQDN format)"
echo "Found ${VALID_ENTRIES} valid IP address mappings"

# Atomic rename with explicit error checking - only done after validation passes
if ! mv "${HOSTS_TMP}" "${HOSTS_FILE}"; then
    echo "ERROR: Failed to move temporary file to ${HOSTS_FILE}"
    rm -f "${HOSTS_TMP}"  # Clean up temp file
    exit 1
fi

echo "AKS critical FQDN hosts resolution completed at $(date)"
