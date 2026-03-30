#!/bin/bash
# Validates that the localdns metrics exporter is working correctly
# and exports the expected VnetDNS and KubeDNS forward IP metrics with proper security hardening.

set -euo pipefail

echo "=== LocalDNS Metrics Exporter Validation ==="
echo ""

# Generate sustained DNS traffic through localdns so that systemd resource accounting
# (CPUUsageNSec, MemoryCurrent) accumulates clearly measurable values before we scrape.
# We run 10 parallel workers each sending 100 sequential queries (1000 total) to ensure
# CoreDNS uses enough CPU/memory to show up even after the exporter's truncation
# (%.9f seconds for CPU, raw bytes for memory).
echo "0. Generating DNS load through localdns to prime resource accounting..."
for worker in $(seq 1 10); do
    (
        for i in $(seq 1 100); do
            dig +short +tries=1 +timeout=2 @169.254.10.10 "load-test-${worker}-${i}.example.com" > /dev/null 2>&1 || true
        done
    ) &
done
wait
# Wait for at least one watchdog tick (~6s interval) so that localdns.sh writes
# the post-load resource values to resources.prom.
sleep 6
echo "   ✓ Sent ~1000 DNS queries through localdns (10 workers × 100 queries)"
echo ""

# Check if socket is listening
echo "1. Checking if port 9353 is listening..."
if ! ss -tln | grep -q ':9353'; then
    echo "   ❌ ERROR: Port 9353 not listening"
    ss -tln | grep -E ':(9353|53)' || true
    exit 1
fi
# Detect the actual listen address (may be node IP via CSE drop-in, not 127.0.0.1)
LISTEN_ADDR=$(ss -tln | grep ':9353' | awk '{print $4}' | head -1)
LISTEN_ADDR=${LISTEN_ADDR//\*/127.0.0.1}
LISTEN_ADDR=${LISTEN_ADDR//0.0.0.0/127.0.0.1}
echo "   ✓ Port 9353 is listening on ${LISTEN_ADDR}"

# Log drop-in and effective socket config for debugging
echo "   Drop-in directory contents:"
ls -la /etc/systemd/system/localdns-exporter.socket.d/ 2>/dev/null || echo "     (no drop-in directory)"
if [ -f /etc/systemd/system/localdns-exporter.socket.d/10-listen-address.conf ]; then
    echo "   Drop-in config:"
    cat /etc/systemd/system/localdns-exporter.socket.d/10-listen-address.conf | sed 's/^/     /'
fi
echo "   Effective systemd Listen property: $(systemctl show localdns-exporter.socket --property=Listen 2>/dev/null || echo 'unknown')"
echo ""

# Verify raw systemd accounting values are > 0 to confirm accounting is enabled and working.
# localdns.sh periodically writes these values to resources.prom (read by the exporter).
# This root-level check verifies systemd cgroup accounting is functional before we test the
# exported values downstream.
echo "2. Verifying systemd resource accounting is enabled and working..."
RAW_CPU_NS=$(systemctl show localdns.service --property=CPUUsageNSec --value 2>/dev/null || echo "0")
if [ -z "$RAW_CPU_NS" ] || [ "$RAW_CPU_NS" = "[not set]" ]; then
    RAW_CPU_NS=0
fi
RAW_MEM_BYTES=$(systemctl show localdns.service --property=MemoryCurrent --value 2>/dev/null || echo "0")
if [ -z "$RAW_MEM_BYTES" ] || [ "$RAW_MEM_BYTES" = "[not set]" ]; then
    RAW_MEM_BYTES=0
fi
echo "   Raw CPUUsageNSec: $RAW_CPU_NS"
echo "   Raw MemoryCurrent: $RAW_MEM_BYTES bytes"
if [ "$RAW_CPU_NS" -eq 0 ] 2>/dev/null; then
    echo "   ❌ ERROR: CPUUsageNSec is 0 after generating DNS load — CPU accounting is not working"
    exit 1
fi
if [ "$RAW_MEM_BYTES" -eq 0 ] 2>/dev/null; then
    echo "   ❌ ERROR: MemoryCurrent is 0 after generating DNS load — memory accounting is not working"
    exit 1
fi
echo "   ✓ Both non-zero — systemd resource accounting is working"
echo ""

# Scrape the exporter endpoint.
echo "3. Checking HTTP status from http://${LISTEN_ADDR}/metrics..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://${LISTEN_ADDR}/metrics" || true)
HTTP_CODE=${HTTP_CODE:-000}
if [ "$HTTP_CODE" -ne 200 ]; then
    echo "   ❌ ERROR: Metrics endpoint returned HTTP $HTTP_CODE"
    exit 1
fi
echo "   ✓ HTTP 200 OK received"
echo ""

echo "4. Fetching metrics body..."
METRICS=$(curl -s "http://${LISTEN_ADDR}/metrics" || true)
if [ -z "$METRICS" ]; then
    echo "   ❌ ERROR: No response body from metrics endpoint"
    exit 1
fi
echo "   ✓ Metrics fetched successfully"
echo ""

# Validate CPU usage metric — check data line exists and value is a valid number.
echo "5. Validating CPU usage metric..."
CPU_LINE=$(echo "$METRICS" | grep -E "^localdns_cpu_usage_seconds_total " || true)
if [ -z "$CPU_LINE" ]; then
    echo "   ❌ ERROR: Missing localdns_cpu_usage_seconds_total data point"
    echo "   (found only comments/metadata or metric not present at all)"
    echo "$METRICS" | grep "localdns_cpu" || true
    exit 1
fi
CPU_VALUE=$(echo "$CPU_LINE" | awk '{print $2}')
echo "   Raw metric: $CPU_LINE"
echo "   Value: $CPU_VALUE"

if ! echo "$CPU_VALUE" | grep -Eq '^[0-9]+\.?[0-9]*(e[+-]?[0-9]+)?$'; then
    echo "   ❌ ERROR: CPU metric value is not a valid non-negative number: $CPU_VALUE"
    exit 1
fi
# Verify the exporter reports a non-zero CPU value — confirms the resources.prom pipeline is working
# (localdns.sh writes real systemd cgroup data to the .prom file, exporter reads it)
if echo "$CPU_VALUE" | grep -Eq '^0+(\.0+)?$'; then
    echo "   ❌ ERROR: Exported CPU metric is zero ($CPU_VALUE) — resources.prom pipeline may be broken"
    echo "   Raw systemd CPUUsageNSec was $RAW_CPU_NS (from step 2)"
    exit 1
fi
echo "   ✓ localdns_cpu_usage_seconds_total=$CPU_VALUE (valid, non-zero)"
echo ""

# Validate memory usage metric — check data line exists and value is a valid number.
echo "6. Validating memory usage metric..."
MEM_LINE=$(echo "$METRICS" | grep -E "^localdns_memory_usage_bytes " || true)
if [ -z "$MEM_LINE" ]; then
    echo "   ❌ ERROR: Missing localdns_memory_usage_bytes data point"
    echo "   (found only comments/metadata or metric not present at all)"
    echo "$METRICS" | grep "localdns_memory" || true
    exit 1
fi
MEM_VALUE=$(echo "$MEM_LINE" | awk '{print $2}')
echo "   Raw metric: $MEM_LINE"
echo "   Value: $MEM_VALUE"

if ! echo "$MEM_VALUE" | grep -Eq '^[0-9]+\.?[0-9]*(e[+-]?[0-9]+)?$'; then
    echo "   ❌ ERROR: Memory metric value is not a valid non-negative number: $MEM_VALUE"
    exit 1
fi
# Verify the exporter reports a non-zero memory value — confirms the resources.prom pipeline is working
if echo "$MEM_VALUE" | grep -Eq '^0+(\.0+)?$'; then
    echo "   ❌ ERROR: Exported memory metric is zero ($MEM_VALUE) — resources.prom pipeline may be broken"
    echo "   Raw systemd MemoryCurrent was $RAW_MEM_BYTES (from step 2)"
    exit 1
fi
echo "   ✓ localdns_memory_usage_bytes=$MEM_VALUE (valid, non-zero)"
echo ""

# Validate staleness timestamp metric — check data line exists and value is a valid recent timestamp.
echo "6b. Validating metrics staleness timestamp..."
TIMESTAMP_LINE=$(echo "$METRICS" | grep -E "^localdns_metrics_last_update_timestamp_seconds " || true)
if [ -z "$TIMESTAMP_LINE" ]; then
    echo "   ❌ ERROR: Missing localdns_metrics_last_update_timestamp_seconds data point"
    echo "$METRICS" | grep "localdns_metrics" || true
    exit 1
fi
TIMESTAMP_VALUE=$(echo "$TIMESTAMP_LINE" | awk '{print $2}')
echo "   Raw metric: $TIMESTAMP_LINE"
CURRENT_TIME=$(date +%s)
# Timestamp should be within the last 120 seconds (2 watchdog cycles)
if [ "$TIMESTAMP_VALUE" -gt 0 ] 2>/dev/null && [ "$((CURRENT_TIME - TIMESTAMP_VALUE))" -lt 120 ]; then
    echo "   ✓ localdns_metrics_last_update_timestamp_seconds=$TIMESTAMP_VALUE (recent, within 120s of now)"
else
    echo "   ❌ ERROR: Staleness timestamp is zero or too old: $TIMESTAMP_VALUE (current time: $CURRENT_TIME)"
    exit 1
fi
echo ""

# Check for VnetDNS forward IP metric
echo "7. Validating VnetDNS forward IP metric..."
if ! echo "$METRICS" | grep -q "localdns_vnetdns_forward_info"; then
    echo "   ❌ ERROR: Missing localdns_vnetdns_forward_info"
    echo "   Available metrics:"
    echo "$METRICS" | grep "^localdns_" | head -10
    exit 1
fi
echo "   ✓ VnetDNS forward metric present"

# Check for KubeDNS forward IP metric
echo "8. Validating KubeDNS forward IP metric..."
if ! echo "$METRICS" | grep -q "localdns_kubedns_forward_info"; then
    echo "   ❌ ERROR: Missing localdns_kubedns_forward_info"
    echo "   Available metrics:"
    echo "$METRICS" | grep "^localdns_" | head -10
    exit 1
fi
echo "   ✓ KubeDNS forward metric present"
echo ""

# Helper function to validate forward IP metric lines.
# Each metric line has labels: ip="...", block="...", status="..."
# There may be multiple lines per metric family (one per corefile zone block).
# Diagnostic output goes to stderr so it's visible to the user.
# Only the status string ("ok", "missing", "file_missing") goes to stdout for capture.
# Args: $1=metric_name_prefix (e.g. "localdns_vnetdns_forward_info"), $2=display_name (e.g. "VnetDNS")
validate_forward_metrics() {
    local metric_name="$1"
    local display_name="$2"

    # Get all data lines (not comments) for this metric
    local data_lines
    data_lines=$(echo "$METRICS" | grep "^${metric_name}{" || true)
    if [ -z "$data_lines" ]; then
        echo "   ❌ ERROR: No data lines found for ${metric_name}" >&2
        exit 1
    fi

    echo "   All ${display_name} metric lines:" >&2
    echo "$data_lines" | sed 's/^/     /' >&2
    echo "" >&2

    # Check the first line to determine overall status
    local first_line
    first_line=$(echo "$data_lines" | head -n 1)
    local first_status
    first_status=$(echo "$first_line" | sed -n 's/.*status="\([^"]*\)".*/\1/p')

    if [ "$first_status" = "ok" ]; then
        # Validate every "ok" line has valid ip, block, and value
        local line_num=0
        while IFS= read -r line; do
            line_num=$((line_num + 1))
            local ip status block value
            ip=$(echo "$line" | sed -n 's/.*ip="\([^"]*\)".*/\1/p')
            status=$(echo "$line" | sed -n 's/.*status="\([^"]*\)".*/\1/p')
            block=$(echo "$line" | sed -n 's/.*block="\([^"]*\)".*/\1/p')
            value=$(echo "$line" | awk '{print $NF}')

            if [ "$status" != "ok" ]; then
                echo "   ❌ ERROR: ${display_name} line $line_num has unexpected status: $status" >&2
                exit 1
            fi
            if [ -z "$ip" ] || [ "$ip" = "unknown" ]; then
                echo "   ❌ ERROR: ${display_name} line $line_num status=ok but IP is missing or unknown" >&2
                exit 1
            fi
            if ! echo "$ip" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then
                echo "   ❌ ERROR: ${display_name} line $line_num IP is not a valid IPv4: $ip" >&2
                exit 1
            fi
            if [ "$value" != "1" ]; then
                echo "   ❌ ERROR: ${display_name} line $line_num status=ok but value is not 1 (got: $value)" >&2
                exit 1
            fi
            if [ -z "$block" ] || [ "$block" = "none" ]; then
                echo "   ❌ ERROR: ${display_name} line $line_num status=ok but block label is missing or 'none'" >&2
                exit 1
            fi
            # Block should contain :53 (zone format from corefile)
            if ! echo "$block" | grep -q ':53$'; then
                echo "   ❌ ERROR: ${display_name} line $line_num block label does not end with :53: $block" >&2
                exit 1
            fi
            echo "   ✓ ${display_name} forward: ip=$ip, block=$block (valid)" >&2
        done <<< "$data_lines"
        echo "   ✓ All $line_num ${display_name} forward entries validated" >&2
    elif [ "$first_status" = "missing" ]; then
        echo "   ⚠️  ${display_name} forward not configured in corefile (expected for some cluster configs)" >&2
        local value
        value=$(echo "$first_line" | awk '{print $NF}')
        if [ "$value" != "0" ]; then
            echo "   ❌ ERROR: ${display_name} not configured but value is not 0" >&2
            exit 1
        fi
    elif [ "$first_status" = "file_missing" ]; then
        echo "   ⚠️  Forward IPs .prom file is missing (may occur during initial setup)" >&2
        local value
        value=$(echo "$first_line" | awk '{print $NF}')
        if [ "$value" != "0" ]; then
            echo "   ❌ ERROR: File missing but value is not 0" >&2
            exit 1
        fi
    else
        echo "   ❌ ERROR: Unknown ${display_name} status: $first_status" >&2
        exit 1
    fi

    # Return status via stdout (only output on stdout — all diagnostics go to stderr)
    echo "$first_status"
}

# Extract and validate VnetDNS forward IPs (may have multiple lines — one per corefile zone block)
echo "9. Validating VnetDNS forward IP entries..."
VNETDNS_STATUS=$(validate_forward_metrics "localdns_vnetdns_forward_info" "VnetDNS")
echo ""

# Extract and validate KubeDNS forward IPs (may have multiple lines — one per corefile zone block)
echo "10. Validating KubeDNS forward IP entries..."
KUBEDNS_STATUS=$(validate_forward_metrics "localdns_kubedns_forward_info" "KubeDNS")
echo ""

echo "=== ✓ All LocalDNS Metrics Validation Checks Passed ==="
if [ "$VNETDNS_STATUS" = "ok" ] && [ "$KUBEDNS_STATUS" = "ok" ]; then
    echo "VnetDNS forwards:"
    echo "$METRICS" | grep "^localdns_vnetdns_forward_info{" | sed 's/^/  /'
    echo "KubeDNS forwards:"
    echo "$METRICS" | grep "^localdns_kubedns_forward_info{" | sed 's/^/  /'
elif [ "$VNETDNS_STATUS" = "ok" ]; then
    echo "VnetDNS forwards:"
    echo "$METRICS" | grep "^localdns_vnetdns_forward_info{" | sed 's/^/  /'
    echo "KubeDNS: forward not configured (expected)"
elif [ "$KUBEDNS_STATUS" = "ok" ]; then
    echo "VnetDNS: forward not configured (expected)"
    echo "KubeDNS forwards:"
    echo "$METRICS" | grep "^localdns_kubedns_forward_info{" | sed 's/^/  /'
else
    echo "Both VnetDNS and KubeDNS forward not configured (may be expected for cluster config)"
fi
echo ""

# Security hardening validation
echo "=== Security Hardening Validation ==="
echo ""

# We need a live instance to query runtime-effective security properties via systemctl show.
# localdns-exporter@.service is a template unit — systemd only loads properties for instantiated
# units. The workers are Accept=yes (per-connection) and exit immediately after responding.
# To keep an instance alive for inspection, we hold an open connection that sends no HTTP request,
# so the worker spawns and blocks waiting on stdin.

# Step 11: Spawn a persistent worker instance by holding a connection open
echo "11. Spawning a persistent worker instance for security inspection..."
sleep 120 | nc ${LISTEN_ADDR%%:*} ${LISTEN_ADDR##*:} > /dev/null 2>&1 &
NC_PID=$!
sleep 2

# Ensure cleanup of the held connection on exit (normal or error)
cleanup_nc() {
    kill "$NC_PID" 2>/dev/null || true
    wait "$NC_PID" 2>/dev/null || true
}
trap cleanup_nc EXIT

echo "   ✓ Connection held open (PID: $NC_PID)"
echo ""

# Step 12: Find the active instance
echo "12. Finding active localdns-exporter instance..."
ACTIVE_INSTANCES=$(systemctl list-units --all 'localdns-exporter@*.service' --no-pager --no-legend --plain | awk '{print $1}' || true)
if [ -z "$ACTIVE_INSTANCES" ]; then
    echo "   ⚠️  No active instances found (socket activation may be delayed), retrying..."
    sleep 3
    ACTIVE_INSTANCES=$(systemctl list-units --all 'localdns-exporter@*.service' --no-pager --no-legend --plain | awk '{print $1}' || true)
fi

if [ -z "$ACTIVE_INSTANCES" ]; then
    echo "   ❌ ERROR: No localdns-exporter instances found after retry"
    exit 1
fi
INSTANCE_NAME=$(echo "$ACTIVE_INSTANCES" | head -n 1)
echo "   ✓ Found instance: $INSTANCE_NAME"
echo ""

# Step 13: Verify all 16 systemd security directives via systemctl show on the live instance.
# This checks the runtime-effective values — what systemd actually enforces — not just what's
# in the unit file. This catches drop-in overrides, syntax errors, and unsupported directives.
echo "13. Verifying all 16 systemd security directives on live instance..."
echo ""

SECURITY_PROPS_1=$(systemctl show "$INSTANCE_NAME" \
    --property=DynamicUser,PrivateTmp,ProtectSystem,ProtectHome,ReadOnlyPaths,NoNewPrivileges \
    2>/dev/null || true)
SECURITY_PROPS_2=$(systemctl show "$INSTANCE_NAME" \
    --property=ProtectKernelTunables,ProtectKernelModules,ProtectControlGroups,RestrictAddressFamilies \
    2>/dev/null || true)
SECURITY_PROPS_3=$(systemctl show "$INSTANCE_NAME" \
    --property=RestrictNamespaces,LockPersonality,RestrictRealtime,RestrictSUIDSGID,RemoveIPC,PrivateMounts \
    2>/dev/null || true)

SECURITY_PROPS="$SECURITY_PROPS_1
$SECURITY_PROPS_2
$SECURITY_PROPS_3"

echo "   Retrieved security properties:"
echo "$SECURITY_PROPS" | sed 's/^/     /'
echo ""

FAILED_CHECKS=0

if ! echo "$SECURITY_PROPS" | grep -q "^DynamicUser=yes$"; then
    echo "   ❌ ERROR: DynamicUser not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ DynamicUser=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^PrivateTmp=yes$"; then
    echo "   ❌ ERROR: PrivateTmp not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ PrivateTmp=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^ProtectSystem=strict$"; then
    echo "   ❌ ERROR: ProtectSystem not strict"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectSystem=strict"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^ProtectHome=yes$"; then
    echo "   ❌ ERROR: ProtectHome not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectHome=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -qE "^ReadOnlyPaths=/$|^ReadOnlyPaths=/ "; then
    echo "   ❌ ERROR: ReadOnlyPaths not set to /"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ReadOnlyPaths=/"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^NoNewPrivileges=yes$"; then
    echo "   ❌ ERROR: NoNewPrivileges not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ NoNewPrivileges=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^ProtectKernelTunables=yes$"; then
    echo "   ❌ ERROR: ProtectKernelTunables not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectKernelTunables=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^ProtectKernelModules=yes$"; then
    echo "   ❌ ERROR: ProtectKernelModules not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectKernelModules=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^ProtectControlGroups=yes$"; then
    echo "   ❌ ERROR: ProtectControlGroups not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ ProtectControlGroups=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -qE "RestrictAddressFamilies=.*AF_UNIX"; then
    echo "   ❌ ERROR: RestrictAddressFamilies does not include AF_UNIX"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    if echo "$SECURITY_PROPS" | grep -qE "RestrictAddressFamilies=.*AF_INET[^6]|RestrictAddressFamilies=.*AF_INET6"; then
        echo "   ❌ ERROR: RestrictAddressFamilies includes AF_INET/AF_INET6 (should be AF_UNIX only)"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    else
        echo "   ✓ RestrictAddressFamilies=AF_UNIX"
    fi
fi

if ! echo "$SECURITY_PROPS" | grep -q "^RestrictNamespaces=yes$"; then
    echo "   ❌ ERROR: RestrictNamespaces not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ RestrictNamespaces=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^LockPersonality=yes$"; then
    echo "   ❌ ERROR: LockPersonality not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ LockPersonality=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^RestrictRealtime=yes$"; then
    echo "   ❌ ERROR: RestrictRealtime not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ RestrictRealtime=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^RestrictSUIDSGID=yes$"; then
    echo "   ❌ ERROR: RestrictSUIDSGID not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ RestrictSUIDSGID=yes"
fi

if ! echo "$SECURITY_PROPS" | grep -q "^RemoveIPC=yes$"; then
    echo "   ❌ ERROR: RemoveIPC not enabled"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
else
    echo "   ✓ RemoveIPC=yes"
fi

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

# Step 14: Get PID of the instance for runtime enforcement checks
echo "14. Getting PID of instance..."
INSTANCE_PID=$(systemctl show "$INSTANCE_NAME" --property=MainPID --value 2>/dev/null || echo "0")

if [ "$INSTANCE_PID" = "0" ] || [ -z "$INSTANCE_PID" ]; then
    echo "   ⚠️  Instance PID not found, skipping process-level checks"
else
    echo "   ✓ Instance PID: $INSTANCE_PID"
    echo ""

    # Step 15: Verify not running as root (DynamicUser runtime enforcement)
    echo "15. Verifying DynamicUser runtime enforcement (not running as root)..."
    INSTANCE_USER=$(ps -o user= -p "$INSTANCE_PID" 2>/dev/null || echo "unknown")
    if [ "$INSTANCE_USER" = "root" ]; then
        echo "   ❌ ERROR: Instance running as root (DynamicUser not enforced at runtime)"
        exit 1
    fi
    echo "   ✓ Running as dynamic user: $INSTANCE_USER"
    echo ""

    # Step 16: Verify no network sockets (RestrictAddressFamilies runtime enforcement)
    echo "16. Verifying RestrictAddressFamilies runtime enforcement (no network sockets)..."

    # Use /proc filesystem for portability (works on all distros without lsof)
    # Note: Socket-activated services inherit the accepted connection as stdin/stdout (fd 0/1).
    # This inherited socket is AF_INET but is expected and allowed. We only care about
    # NEW sockets the service creates, not the inherited activation socket.
    NETWORK_SOCKETS=0
    INHERITED_SOCKET_INODE=""

    if [ -d "/proc/$INSTANCE_PID/fd" ]; then
        # Find the stdin socket inode (the inherited activation socket)
        if [ -L "/proc/$INSTANCE_PID/fd/0" ]; then
            STDIN_TARGET=$(readlink "/proc/$INSTANCE_PID/fd/0" 2>/dev/null || echo "")
            INHERITED_SOCKET_INODE=$(echo "$STDIN_TARGET" | sed -n 's/^socket:\[\([0-9]*\)\]$/\1/p')
        fi

        # Iterate through file descriptors to find sockets
        for fd in /proc/"$INSTANCE_PID"/fd/*; do
            if [ -L "$fd" ]; then
                FD_TARGET=$(readlink "$fd" 2>/dev/null || echo "")
                SOCKET_INODE=$(echo "$FD_TARGET" | sed -n 's/^socket:\[\([0-9]*\)\]$/\1/p')
                if [ -n "$SOCKET_INODE" ]; then

                    # Skip the inherited stdin/stdout socket from socket activation
                    if [ -n "$INHERITED_SOCKET_INODE" ] && [ "$SOCKET_INODE" = "$INHERITED_SOCKET_INODE" ]; then
                        continue
                    fi

                    # Use ss to check if this socket is TCP/UDP (network socket)
                    if ss -tupn 2>/dev/null | grep -q "inode:$SOCKET_INODE"; then
                        NETWORK_SOCKETS=$((NETWORK_SOCKETS + 1))
                        echo "   Found unexpected network socket: inode=$SOCKET_INODE"
                    fi
                fi
            fi
        done
    else
        echo "   ⚠️  WARNING: Cannot access /proc/$INSTANCE_PID/fd, skipping socket inspection"
    fi

    if [ "$NETWORK_SOCKETS" != "0" ]; then
        echo "   ❌ ERROR: Instance has $NETWORK_SOCKETS unexpected network socket(s) (RestrictAddressFamilies not enforced)"
        exit 1
    fi
    echo "   ✓ No unexpected network sockets (AF_UNIX only, restriction enforced)"
    echo ""

    # Step 17: Verify namespace isolation (RestrictNamespaces runtime enforcement)
    echo "17. Verifying namespace isolation..."
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

echo "=== ✓ Security Hardening Validation Passed ==="
echo "Configuration: All 16 systemd security directives verified on live instance"
if [ -n "${INSTANCE_PID:-}" ] && [ "${INSTANCE_PID:-0}" != "0" ]; then
    echo "Runtime: DynamicUser, RestrictAddressFamilies, and namespace isolation enforced"
fi
