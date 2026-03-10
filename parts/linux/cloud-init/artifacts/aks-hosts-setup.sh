#!/bin/bash
set -euo pipefail

# aks-hosts-setup.sh
# Resolves A and AAAA records for critical AKS FQDNs and populates /etc/localdns/hosts.
# TARGET_CLOUD is set by CSE (cse_cmd.sh) and persisted via /etc/localdns/cloud-env
# as a systemd EnvironmentFile so it's available on both initial and timer-triggered runs.

HOSTS_FILE="/etc/localdns/hosts"

# Ensure the directory exists
mkdir -p "$(dirname "$HOSTS_FILE")"

# Use TARGET_CLOUD directly. It's available from:
#   1. CSE environment (initial run from enableAKSHostsSetup)
#   2. Systemd EnvironmentFile (timer-triggered runs via aks-hosts-setup.service)
# If TARGET_CLOUD is not set, exit immediately - we must not guess the cloud environment
# as this could cache incorrect DNS entries in the hosts file.
if [ -z "${TARGET_CLOUD:-}" ]; then
    echo "ERROR: TARGET_CLOUD is not set. Cannot determine which FQDNs to resolve."
    echo "This likely means the cloud environment file is missing or CSE did not set TARGET_CLOUD."
    echo "Exiting without modifying hosts file to avoid caching incorrect DNS entries."
    exit 1
fi
local_cloud="${TARGET_CLOUD}"

# Select critical FQDNs based on the cloud environment.
# Each cloud has its own service endpoints for container registry, identity, ARM, and packages.
# This mirrors the cloud detection in GetCloudTargetEnv (pkg/agent/datamodel/sig_config.go).

# FQDNs common to all clouds.
COMMON_FQDNS=(
    "packages.microsoft.com"            # Microsoft packages
)

# Cloud-specific FQDNs.
case "${local_cloud}" in
    AzureChinaCloud)
        CLOUD_FQDNS=(
            "acs-mirror.azureedge.net"          # K8s binaries mirror
            "mcr.azure.cn"                      # Container registry (China)(New)
            "mcr.azk8s.cn"                      # Container registry (China)(Old, migrating from this to mcr.azure.cn)
            "login.partner.microsoftonline.cn"  # Azure AD (China)
            "management.chinacloudapi.cn"       # ARM (China)
        )
        ;;
    AzureUSGovernmentCloud)
        CLOUD_FQDNS=(
            "acs-mirror.azureedge.net"          # K8s binaries mirror
            "mcr.microsoft.com"                 # Container registry
            "login.microsoftonline.us"          # Azure AD (US Gov)
            "management.usgovcloudapi.net"      # ARM (US Gov)
            "packages.aks.azure.com"            # AKS packages
        )
        ;;
    USNatCloud)
        CLOUD_FQDNS=(
            "mcr.microsoft.com"                        # Container registry
            "login.microsoftonline.eaglex.ic.gov"      # Azure AD (USNat)
            "management.azure.eaglex.ic.gov"           # ARM (USNat)
        )
        ;;
    USSecCloud)
        CLOUD_FQDNS=(
            "mcr.microsoft.com"                           # Container registry
            "login.microsoftonline.microsoft.scloud"      # Azure AD (USSec)
            "management.azure.microsoft.scloud"           # ARM (USSec)
        )
        ;;
    AzureStackCloud)
        # Custom cloud / AGC — endpoints are customer-defined.
        CLOUD_FQDNS=(
            "mcr.microsoft.com"                 # Container registry
        )
        ;;
    AzurePublicCloud|AzureGermanCloud|AzureGermanyCloud|AzureBleuCloud)
        # AzurePublicCloud: standard public endpoints.
        # AzureGermanCloud (legacy): retired, uses public endpoints.
        # AzureGermanyCloud (Delos) / AzureBleuCloud: EU sovereign clouds,
        #   use public endpoints for container registry and packages.
        CLOUD_FQDNS=(
            "acs-mirror.azureedge.net"          # K8s binaries mirror
            "mcr.microsoft.com"                 # Container registry
            "login.microsoftonline.com"         # Azure AD / Entra ID
            "management.azure.com"              # ARM
            "packages.aks.azure.com"            # AKS packages
        )
        ;;
    *)
        # Unrecognized cloud environment - exit with error
        echo "Detected cloud environment: ${local_cloud}"
        echo "ERROR: Unrecognized cloud environment: ${local_cloud}"
        echo "Supported clouds: AzureChinaCloud, AzureUSGovernmentCloud, USNatCloud, USSecCloud, AzureStackCloud, AzurePublicCloud, AzureGermanCloud, AzureGermanyCloud, AzureBleuCloud"
        echo "Cannot determine which FQDNs to resolve for hosts file."
        echo "Exiting without modifying hosts file to avoid caching incorrect DNS entries."
        exit 1
        ;;
esac

# Combine common + cloud-specific FQDNs.
CRITICAL_FQDNS=("${COMMON_FQDNS[@]}" "${CLOUD_FQDNS[@]}")

echo "Detected cloud environment: ${local_cloud}"

# Function to resolve IPv4 addresses for a domain
# Filters output to only include valid IPv4 addresses (rejects NXDOMAIN, SERVFAIL, hostnames, etc.)
resolve_ipv4() {
    local domain="$1"
    local output
    output=$(timeout 3 nslookup -type=A "${domain}" 2>/dev/null) || return 0
    # Parse Address lines (skip server address with #), validate IPv4 format with octet range 0-255
    echo "${output}" | awk '/^Address: / && !/^Address: .*#/ {print $2}' | grep -E '^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$' | while IFS='.' read -r a b c d; do
        if [ "$a" -le 255 ] && [ "$b" -le 255 ] && [ "$c" -le 255 ] && [ "$d" -le 255 ]; then
            echo "${a}.${b}.${c}.${d}"
        fi
    done
    return 0
}

# Function to resolve IPv6 addresses for a domain
# Filters output to only include valid IPv6 addresses (rejects NXDOMAIN, SERVFAIL, hostnames, etc.)
resolve_ipv6() {
    local domain="$1"
    local output
    output=$(timeout 3 nslookup -type=AAAA "${domain}" 2>/dev/null) || return 0
    # Parse Address lines (skip server address with #), validate IPv6 format (must contain : and only hex/colons, min 3 chars)
    echo "${output}" | awk '/^Address: / && !/^Address: .*#/ {print $2}' | grep -E '^[0-9a-fA-F:]{3,}$' | grep ':' || return 0
}

echo "Starting AKS critical FQDN hosts resolution at $(date)"

# Track if we resolved at least one address
RESOLVED_ANY=false

# Start building the hosts file content
HOSTS_CONTENT="# AKS critical FQDN addresses resolved at $(date)
# This file is automatically generated by aks-hosts-setup.service
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
if ! echo "${HOSTS_CONTENT}" > "${HOSTS_TMP}"; then
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
    [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "$line" ]] && continue

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
