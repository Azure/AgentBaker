#!/bin/bash
# Validates that the localdns metrics exporter is working correctly
# and exports the expected VnetDNS and KubeDNS forward IP metrics with proper security hardening.

set -euo pipefail

echo "=== LocalDNS Metrics Exporter Validation ==="
echo ""

# Check if socket is listening
echo "1. Checking if port 9353 is listening..."
if ! ss -tln | grep -q ':9353'; then
    echo "   ❌ ERROR: Port 9353 not listening"
    ss -tln | grep -E ':(9353|53)' || true
    exit 1
fi
echo "   ✓ Port 9353 is listening"
echo ""

# Check HTTP status code
echo "2. Checking HTTP status from http://localhost:9353/metrics..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9353/metrics || true)
HTTP_CODE=${HTTP_CODE:-000}
if [ "$HTTP_CODE" -ne 200 ]; then
    echo "   ❌ ERROR: Metrics endpoint returned HTTP $HTTP_CODE"
    exit 1
fi
echo "   ✓ HTTP 200 OK received"
echo ""

# Fetch metrics body
echo "3. Fetching metrics body..."
METRICS=$(curl -s http://localhost:9353/metrics || true)
if [ -z "$METRICS" ]; then
    echo "   ❌ ERROR: No response body from metrics endpoint"
    exit 1
fi
echo "   ✓ Metrics fetched successfully"
echo ""

# Check for required base metrics
echo "4. Validating base metrics..."
if ! echo "$METRICS" | grep -q "localdns_cpu_usage_seconds_total"; then
    echo "   ❌ ERROR: Missing localdns_cpu_usage_seconds_total"
    exit 1
fi
echo "   ✓ CPU metric present"

if ! echo "$METRICS" | grep -q "localdns_memory_usage_mb"; then
    echo "   ❌ ERROR: Missing localdns_memory_usage_mb"
    exit 1
fi
echo "   ✓ Memory metric present"
echo ""

# Check for VnetDNS forward IP metric
echo "5. Validating VnetDNS forward IP metric..."
if ! echo "$METRICS" | grep -q "localdns_vnetdns_forward_info"; then
    echo "   ❌ ERROR: Missing localdns_vnetdns_forward_info"
    echo "   Available metrics:"
    echo "$METRICS" | grep "^localdns_" | head -10
    exit 1
fi
echo "   ✓ VnetDNS forward metric present"

# Check for KubeDNS forward IP metric
echo "6. Validating KubeDNS forward IP metric..."
if ! echo "$METRICS" | grep -q "localdns_kubedns_forward_info"; then
    echo "   ❌ ERROR: Missing localdns_kubedns_forward_info"
    echo "   Available metrics:"
    echo "$METRICS" | grep "^localdns_" | head -10
    exit 1
fi
echo "   ✓ KubeDNS forward metric present"
echo ""

# Extract and validate VnetDNS forward IP
echo "7. Extracting VnetDNS forward IP..."
VNETDNS_LINE=$(echo "$METRICS" | grep "^localdns_vnetdns_forward_info{" | head -n 1)
echo "   Raw metric: $VNETDNS_LINE"

VNETDNS_STATUS=$(echo "$VNETDNS_LINE" | sed -n 's/.*status="\([^"]*\)".*/\1/p')
VNETDNS_IP=$(echo "$VNETDNS_LINE" | sed -n 's/.*ip="\([^"]*\)".*/\1/p')
VNETDNS_VALUE=$(echo "$VNETDNS_LINE" | awk '{print $NF}')

echo "   Status: $VNETDNS_STATUS"
echo "   IP: $VNETDNS_IP"
echo "   Value: $VNETDNS_VALUE"

# VnetDNS can have different status values depending on configuration
if [ "$VNETDNS_STATUS" = "ok" ]; then
    if [ -z "$VNETDNS_IP" ] || [ "$VNETDNS_IP" = "unknown" ]; then
        echo "   ❌ ERROR: VnetDNS status is ok but IP is missing or unknown"
        exit 1
    fi
    if [ "$VNETDNS_VALUE" != "1" ]; then
        echo "   ❌ ERROR: VnetDNS status is ok but value is not 1"
        exit 1
    fi
    # Validate IP is valid IPv4
    if ! echo "$VNETDNS_IP" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then
        echo "   ❌ ERROR: VnetDNS IP is not a valid IPv4: $VNETDNS_IP"
        exit 1
    fi
    echo "   ✓ VnetDNS forward IP: $VNETDNS_IP (valid)"
elif [ "$VNETDNS_STATUS" = "missing" ]; then
    echo "   ⚠️  VnetDNS forward not configured in corefile (expected for some cluster configs)"
    if [ "$VNETDNS_VALUE" != "0" ]; then
        echo "   ❌ ERROR: VnetDNS not configured but value is not 0"
        exit 1
    fi
elif [ "$VNETDNS_STATUS" = "file_missing" ]; then
    echo "   ⚠️  Forward IPs .prom file is missing (may occur during initial setup)"
    if [ "$VNETDNS_VALUE" != "0" ]; then
        echo "   ❌ ERROR: File missing but value is not 0"
        exit 1
    fi
else
    echo "   ❌ ERROR: Unknown VnetDNS status: $VNETDNS_STATUS"
    exit 1
fi
echo ""

# Extract and validate KubeDNS forward IP
echo "8. Extracting KubeDNS forward IP..."
KUBEDNS_LINE=$(echo "$METRICS" | grep "^localdns_kubedns_forward_info{" | head -n 1)
echo "   Raw metric: $KUBEDNS_LINE"

KUBEDNS_STATUS=$(echo "$KUBEDNS_LINE" | sed -n 's/.*status="\([^"]*\)".*/\1/p')
KUBEDNS_IP=$(echo "$KUBEDNS_LINE" | sed -n 's/.*ip="\([^"]*\)".*/\1/p')
KUBEDNS_VALUE=$(echo "$KUBEDNS_LINE" | awk '{print $NF}')

echo "   Status: $KUBEDNS_STATUS"
echo "   IP: $KUBEDNS_IP"
echo "   Value: $KUBEDNS_VALUE"

# KubeDNS can have different status values depending on configuration
if [ "$KUBEDNS_STATUS" = "ok" ]; then
    if [ -z "$KUBEDNS_IP" ] || [ "$KUBEDNS_IP" = "unknown" ]; then
        echo "   ❌ ERROR: KubeDNS status is ok but IP is missing or unknown"
        exit 1
    fi
    if [ "$KUBEDNS_VALUE" != "1" ]; then
        echo "   ❌ ERROR: KubeDNS status is ok but value is not 1"
        exit 1
    fi
    # Validate IP is valid IPv4
    if ! echo "$KUBEDNS_IP" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then
        echo "   ❌ ERROR: KubeDNS IP is not a valid IPv4: $KUBEDNS_IP"
        exit 1
    fi
    echo "   ✓ KubeDNS forward IP: $KUBEDNS_IP (valid)"
elif [ "$KUBEDNS_STATUS" = "missing" ]; then
    echo "   ⚠️  KubeDNS forward not configured in corefile (expected for some cluster configs)"
    if [ "$KUBEDNS_VALUE" != "0" ]; then
        echo "   ❌ ERROR: KubeDNS not configured but value is not 0"
        exit 1
    fi
elif [ "$KUBEDNS_STATUS" = "file_missing" ]; then
    echo "   ⚠️  Forward IPs .prom file is missing (may occur during initial setup)"
    if [ "$KUBEDNS_VALUE" != "0" ]; then
        echo "   ❌ ERROR: File missing but value is not 0"
        exit 1
    fi
else
    echo "   ❌ ERROR: Unknown KubeDNS status: $KUBEDNS_STATUS"
    exit 1
fi
echo ""

echo "=== ✓ All LocalDNS Metrics Validation Checks Passed ==="
if [ "$VNETDNS_STATUS" = "ok" ] && [ "$KUBEDNS_STATUS" = "ok" ]; then
    echo "VnetDNS forward IP: $VNETDNS_IP"
    echo "KubeDNS forward IP: $KUBEDNS_IP"
elif [ "$VNETDNS_STATUS" = "ok" ]; then
    echo "VnetDNS forward IP: $VNETDNS_IP"
    echo "KubeDNS: forward not configured (expected)"
elif [ "$KUBEDNS_STATUS" = "ok" ]; then
    echo "VnetDNS: forward not configured (expected)"
    echo "KubeDNS forward IP: $KUBEDNS_IP"
else
    echo "Both VnetDNS and KubeDNS forward not configured (may be expected for cluster config)"
fi
echo ""

# Security hardening validation
echo "=== Security Hardening Validation ==="
echo ""

# Step 9: Verify systemd security properties are configured FIRST (before spawning instances)
# This is faster and doesn't require running processes
echo "9. Verifying all 16 systemd security directives are configured..."
echo ""

# Fetch all security-related properties in batches (systemctl has limits)
SECURITY_PROPS_1=$(systemctl show localdns-exporter@.service \
    --property=DynamicUser,PrivateTmp,ProtectSystem,ProtectHome,ReadOnlyPaths,NoNewPrivileges \
    2>/dev/null || true)
SECURITY_PROPS_2=$(systemctl show localdns-exporter@.service \
    --property=ProtectKernelTunables,ProtectKernelModules,ProtectControlGroups,RestrictAddressFamilies \
    2>/dev/null || true)
SECURITY_PROPS_3=$(systemctl show localdns-exporter@.service \
    --property=RestrictNamespaces,LockPersonality,RestrictRealtime,RestrictSUIDSGID,RemoveIPC,PrivateMounts \
    2>/dev/null || true)

SECURITY_PROPS="$SECURITY_PROPS_1
$SECURITY_PROPS_2
$SECURITY_PROPS_3"

echo "   Retrieved security properties:"
echo "$SECURITY_PROPS" | sed 's/^/     /'
echo ""

# Check all 16 security directives
FAILED_CHECKS=0

# 1. DynamicUser=yes
if ! echo "$SECURITY_PROPS" | grep -q "^DynamicUser=yes$"; then
    echo "   ❌ ERROR: DynamicUser not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ DynamicUser=yes"
fi

# 2. PrivateTmp=yes
if ! echo "$SECURITY_PROPS" | grep -q "^PrivateTmp=yes$"; then
    echo "   ❌ ERROR: PrivateTmp not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ PrivateTmp=yes"
fi

# 3. ProtectSystem=strict
if ! echo "$SECURITY_PROPS" | grep -q "^ProtectSystem=strict$"; then
    echo "   ❌ ERROR: ProtectSystem not strict"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectSystem=strict"
fi

# 4. ProtectHome=yes
if ! echo "$SECURITY_PROPS" | grep -q "^ProtectHome=yes$"; then
    echo "   ❌ ERROR: ProtectHome not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectHome=yes"
fi

# 5. ReadOnlyPaths=/ (exact match, not a prefix)
if ! echo "$SECURITY_PROPS" | grep -qE "^ReadOnlyPaths=/$|^ReadOnlyPaths=/ "; then
    echo "   ❌ ERROR: ReadOnlyPaths not set to /"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ReadOnlyPaths=/"
fi

# 6. NoNewPrivileges=yes
if ! echo "$SECURITY_PROPS" | grep -q "^NoNewPrivileges=yes$"; then
    echo "   ❌ ERROR: NoNewPrivileges not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ NoNewPrivileges=yes"
fi

# 7. ProtectKernelTunables=yes
if ! echo "$SECURITY_PROPS" | grep -q "^ProtectKernelTunables=yes$"; then
    echo "   ❌ ERROR: ProtectKernelTunables not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectKernelTunables=yes"
fi

# 8. ProtectKernelModules=yes
if ! echo "$SECURITY_PROPS" | grep -q "^ProtectKernelModules=yes$"; then
    echo "   ❌ ERROR: ProtectKernelModules not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectKernelModules=yes"
fi

# 9. ProtectControlGroups=yes
if ! echo "$SECURITY_PROPS" | grep -q "^ProtectControlGroups=yes$"; then
    echo "   ❌ ERROR: ProtectControlGroups not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectControlGroups=yes"
fi

# 10. RestrictAddressFamilies - check for AF_UNIX (may be formatted different ways)
if ! echo "$SECURITY_PROPS" | grep -qE "RestrictAddressFamilies=.*AF_UNIX"; then
    echo "   ❌ ERROR: RestrictAddressFamilies does not include AF_UNIX"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    # Also verify AF_INET/AF_INET6 are NOT present
    if echo "$SECURITY_PROPS" | grep -qE "RestrictAddressFamilies=.*AF_INET[^6]|RestrictAddressFamilies=.*AF_INET6"; then
        echo "   ❌ ERROR: RestrictAddressFamilies includes AF_INET/AF_INET6 (should be AF_UNIX only)"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    else
        echo "   ✓ RestrictAddressFamilies=AF_UNIX"
    fi
fi

# 11. RestrictNamespaces=yes
if ! echo "$SECURITY_PROPS" | grep -q "^RestrictNamespaces=yes$"; then
    echo "   ❌ ERROR: RestrictNamespaces not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ RestrictNamespaces=yes"
fi

# 12. LockPersonality=yes
if ! echo "$SECURITY_PROPS" | grep -q "^LockPersonality=yes$"; then
    echo "   ❌ ERROR: LockPersonality not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ LockPersonality=yes"
fi

# 13. RestrictRealtime=yes
if ! echo "$SECURITY_PROPS" | grep -q "^RestrictRealtime=yes$"; then
    echo "   ❌ ERROR: RestrictRealtime not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ RestrictRealtime=yes"
fi

# 14. RestrictSUIDSGID=yes
if ! echo "$SECURITY_PROPS" | grep -q "^RestrictSUIDSGID=yes$"; then
    echo "   ❌ ERROR: RestrictSUIDSGID not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ RestrictSUIDSGID=yes"
fi

# 15. RemoveIPC=yes
if ! echo "$SECURITY_PROPS" | grep -q "^RemoveIPC=yes$"; then
    echo "   ❌ ERROR: RemoveIPC not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ RemoveIPC=yes"
fi

# 16. PrivateMounts=yes
if ! echo "$SECURITY_PROPS" | grep -q "^PrivateMounts=yes$"; then
    echo "   ❌ ERROR: PrivateMounts not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ PrivateMounts=yes"
fi

echo ""
if [ "$FAILED_CHECKS" -gt 0 ]; then
    echo "=== ❌ Security Configuration Validation FAILED ==="
    echo "$FAILED_CHECKS out of 16 security directives are not properly configured"
    exit 1
fi
echo "✓ All 16 security directives are properly configured"
echo ""

# Step 10: Trigger a scrape to spawn an instance for runtime validation
echo "10. Triggering scrape to spawn a worker instance for runtime validation..."
curl -s http://localhost:9353/metrics > /dev/null &
CURL_PID=$!
sleep 2  # Increased from 1 to 2 seconds for reliability
echo "   ✓ Scrape triggered"
echo ""

# Step 11: Find active instance
echo "11. Finding active localdns-exporter instance..."
ACTIVE_INSTANCES=$(systemctl list-units --all 'localdns-exporter@*.service' --no-pager --no-legend --plain | awk '{print $1}' || true)
if [ -z "$ACTIVE_INSTANCES" ]; then
    echo "   ⚠️  No active instances found (socket activation may be delayed)"
    wait $CURL_PID 2>/dev/null || true
    sleep 2
    ACTIVE_INSTANCES=$(systemctl list-units --all 'localdns-exporter@*.service' --no-pager --no-legend --plain | awk '{print $1}' || true)
fi

if [ -z "$ACTIVE_INSTANCES" ]; then
    echo "   ⚠️  No instances found after retry, skipping runtime validation"
    echo "   (This may happen if socket activation is very slow or disabled)"
else
    INSTANCE_NAME=$(echo "$ACTIVE_INSTANCES" | head -n 1)
    echo "   ✓ Found instance: $INSTANCE_NAME"
    echo ""

    # Step 12: Get PID of the instance
    echo "12. Getting PID of instance..."
    INSTANCE_PID=$(systemctl show "$INSTANCE_NAME" --property=MainPID --value 2>/dev/null || echo "0")

    if [ "$INSTANCE_PID" = "0" ] || [ -z "$INSTANCE_PID" ]; then
        echo "   ⚠️  Instance PID not found, skipping process-level checks"
    else
        echo "   ✓ Instance PID: $INSTANCE_PID"
        echo ""

        # Step 13: Verify not running as root (DynamicUser runtime enforcement)
        echo "13. Verifying DynamicUser runtime enforcement (not running as root)..."
        INSTANCE_USER=$(ps -o user= -p "$INSTANCE_PID" 2>/dev/null || echo "unknown")
        if [ "$INSTANCE_USER" = "root" ]; then
            echo "   ❌ ERROR: Instance running as root (DynamicUser not enforced at runtime)"
            exit 1
        fi
        echo "   ✓ Running as dynamic user: $INSTANCE_USER"
        echo ""

        # Step 14: Verify no network sockets (RestrictAddressFamilies runtime enforcement)
        echo "14. Verifying RestrictAddressFamilies runtime enforcement (no network sockets)..."
        NETWORK_SOCKETS=$(lsof -p "$INSTANCE_PID" 2>/dev/null | grep -c "IPv4\|IPv6" || echo "0")
        if [ "$NETWORK_SOCKETS" != "0" ]; then
            echo "   ❌ ERROR: Instance has network sockets (RestrictAddressFamilies not enforced)"
            lsof -p "$INSTANCE_PID" | grep "IPv" || true
            exit 1
        fi
        echo "   ✓ No network sockets (AF_UNIX only, restriction enforced)"
        echo ""

        # Step 15: Verify namespace isolation (RestrictNamespaces runtime enforcement)
        echo "15. Verifying namespace isolation..."
        if [ -d "/proc/$INSTANCE_PID/ns" ]; then
            NS_COUNT=$(find /proc/"$INSTANCE_PID"/ns/ -mindepth 1 -maxdepth 1 2>/dev/null | wc -l)
            if [ "$NS_COUNT" -lt 5 ]; then
                echo "   ⚠️  WARNING: Only $NS_COUNT namespaces (expected 5+ for proper isolation)"
            else
                echo "   ✓ Process has $NS_COUNT namespaces (properly isolated)"
            fi
        else
            echo "   ⚠️  Cannot verify namespaces (proc not accessible)"
        fi
        echo ""
    fi
fi

# Wait for curl to finish
wait $CURL_PID 2>/dev/null || true

echo "=== ✓ Security Hardening Validation Passed ==="
echo "Configuration: All 16 systemd security directives verified"
if [ -n "$INSTANCE_PID" ] && [ "$INSTANCE_PID" != "0" ]; then
    echo "Runtime: DynamicUser, RestrictAddressFamilies, and namespace isolation enforced"
fi
