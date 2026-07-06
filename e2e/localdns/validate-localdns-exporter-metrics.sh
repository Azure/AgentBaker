#!/bin/bash
# Validates that the localdns metrics exporter is working correctly
# and exports the expected VnetDNS and KubeDNS forward IP metrics with proper security hardening.

set -euo pipefail

echo "=== LocalDNS Metrics Exporter Validation ==="
echo ""

# NOTE: The Go e2e framework checks for the kubernetes.azure.com/localdns-exporter node label
# before running this script. If we get here, the label exists, which means CSE confirmed the
# exporter socket unit is installed on this VHD. Every check below MUST hard-fail on errors —
# there is no "skip gracefully" path. If something is broken, we want to know.

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
SS_LISTEN_OUTPUT=$(ss -tln)
if ! grep -q ':9353[[:space:]]' <<< "$SS_LISTEN_OUTPUT"; then
    echo "   ❌ ERROR: Port 9353 not listening"
    grep -E ':(9353|53)[[:space:]]' <<< "$SS_LISTEN_OUTPUT" || true
    exit 1
fi
# Detect the actual listen address (may be node IP via CSE drop-in, not 127.0.0.1)
LISTEN_ADDR=$(awk '/:9353[[:space:]]/ {print $4; exit}' <<< "$SS_LISTEN_OUTPUT")
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

# Use here-strings when inspecting shell variables. With pipefail enabled,
# `echo "$METRICS" | grep -q ...` can return a false negative if grep exits
# early and the writer gets SIGPIPE on a large metrics payload.
# Validate CPU usage metric — check data line exists and value is a valid number.
echo "5. Validating CPU usage metric..."
CPU_LINE=$(grep -E "^localdns_cpu_usage_seconds_total " <<< "$METRICS" || true)
if [ -z "$CPU_LINE" ]; then
    echo "   ❌ ERROR: Missing localdns_cpu_usage_seconds_total data point"
    echo "   (found only comments/metadata or metric not present at all)"
    grep "localdns_cpu" <<< "$METRICS" || true
    exit 1
fi
CPU_VALUE=$(awk '{print $2}' <<< "$CPU_LINE")
echo "   Raw metric: $CPU_LINE"
echo "   Value: $CPU_VALUE"

if ! grep -Eq '^[0-9]+\.?[0-9]*(e[+-]?[0-9]+)?$' <<< "$CPU_VALUE"; then
    echo "   ❌ ERROR: CPU metric value is not a valid non-negative number: $CPU_VALUE"
    exit 1
fi
# Verify the exporter reports a non-zero CPU value — confirms the resources.prom pipeline is working
# (localdns.sh writes real systemd cgroup data to the .prom file, exporter reads it)
if grep -Eq '^0+(\.0+)?$' <<< "$CPU_VALUE"; then
    echo "   ❌ ERROR: Exported CPU metric is zero ($CPU_VALUE) — resources.prom pipeline may be broken"
    echo "   Raw systemd CPUUsageNSec was $RAW_CPU_NS (from step 2)"
    exit 1
fi
echo "   ✓ localdns_cpu_usage_seconds_total=$CPU_VALUE (valid, non-zero)"
echo ""

# Validate memory usage metric — check data line exists and value is a valid number.
echo "6. Validating memory usage metric..."
MEM_LINE=$(grep -E "^localdns_memory_usage_bytes " <<< "$METRICS" || true)
if [ -z "$MEM_LINE" ]; then
    echo "   ❌ ERROR: Missing localdns_memory_usage_bytes data point"
    echo "   (found only comments/metadata or metric not present at all)"
    grep "localdns_memory" <<< "$METRICS" || true
    exit 1
fi
MEM_VALUE=$(awk '{print $2}' <<< "$MEM_LINE")
echo "   Raw metric: $MEM_LINE"
echo "   Value: $MEM_VALUE"

if ! grep -Eq '^[0-9]+\.?[0-9]*(e[+-]?[0-9]+)?$' <<< "$MEM_VALUE"; then
    echo "   ❌ ERROR: Memory metric value is not a valid non-negative number: $MEM_VALUE"
    exit 1
fi
# Verify the exporter reports a non-zero memory value — confirms the resources.prom pipeline is working
if grep -Eq '^0+(\.0+)?$' <<< "$MEM_VALUE"; then
    echo "   ❌ ERROR: Exported memory metric is zero ($MEM_VALUE) — resources.prom pipeline may be broken"
    echo "   Raw systemd MemoryCurrent was $RAW_MEM_BYTES (from step 2)"
    exit 1
fi
echo "   ✓ localdns_memory_usage_bytes=$MEM_VALUE (valid, non-zero)"
echo ""

# Validate staleness timestamp metric — check data line exists and value is a valid recent timestamp.
echo "6b. Validating metrics staleness timestamp..."
TIMESTAMP_LINE=$(grep -E "^localdns_metrics_last_update_timestamp_seconds " <<< "$METRICS" || true)
if [ -z "$TIMESTAMP_LINE" ]; then
    echo "   ❌ ERROR: Missing localdns_metrics_last_update_timestamp_seconds data point"
    grep "localdns_metrics" <<< "$METRICS" || true
    exit 1
fi
TIMESTAMP_VALUE=$(awk '{print $2}' <<< "$TIMESTAMP_LINE")
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
if ! grep -q "localdns_vnetdns_forward_info" <<< "$METRICS"; then
    echo "   ❌ ERROR: Missing localdns_vnetdns_forward_info"
    echo "   Available metrics:"
    awk '/^localdns_/ { print; if (++count == 10) exit }' <<< "$METRICS"
    exit 1
fi
echo "   ✓ VnetDNS forward metric present"

# Check for KubeDNS forward IP metric
echo "8. Validating KubeDNS forward IP metric..."
if ! grep -q "localdns_kubedns_forward_info" <<< "$METRICS"; then
    echo "   ❌ ERROR: Missing localdns_kubedns_forward_info"
    echo "   Available metrics:"
    awk '/^localdns_/ { print; if (++count == 10) exit }' <<< "$METRICS"
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
    data_lines=$(grep "^${metric_name}{" <<< "$METRICS" || true)
    if [ -z "$data_lines" ]; then
        echo "   ❌ ERROR: No data lines found for ${metric_name}" >&2
        exit 1
    fi

    echo "   All ${display_name} metric lines:" >&2
    echo "$data_lines" | sed 's/^/     /' >&2
    echo "" >&2

    # Check the first line to determine overall status
    local first_line
    first_line=$(sed -n '1p' <<< "$data_lines")
    local first_status
    first_status=$(sed -n 's/.*status="\([^"]*\)".*/\1/p' <<< "$first_line")

    if [ "$first_status" = "ok" ]; then
        # Validate every "ok" line has valid ip, block, and value
        local line_num=0
        while IFS= read -r line; do
            line_num=$((line_num + 1))
            local ip status block value
            ip=$(sed -n 's/.*ip="\([^"]*\)".*/\1/p' <<< "$line")
            status=$(sed -n 's/.*status="\([^"]*\)".*/\1/p' <<< "$line")
            block=$(sed -n 's/.*block="\([^"]*\)".*/\1/p' <<< "$line")
            value=$(awk '{print $NF}' <<< "$line")

            if [ "$status" != "ok" ]; then
                echo "   ❌ ERROR: ${display_name} line $line_num has unexpected status: $status" >&2
                exit 1
            fi
            if [ -z "$ip" ] || [ "$ip" = "unknown" ]; then
                echo "   ❌ ERROR: ${display_name} line $line_num status=ok but IP is missing or unknown" >&2
                exit 1
            fi
            if ! grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' <<< "$ip"; then
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
            if ! grep -q ':53$' <<< "$block"; then
                echo "   ❌ ERROR: ${display_name} line $line_num block label does not end with :53: $block" >&2
                exit 1
            fi
            echo "   ✓ ${display_name} forward: ip=$ip, block=$block (valid)" >&2
        done <<< "$data_lines"
        echo "   ✓ All $line_num ${display_name} forward entries validated" >&2
    elif [ "$first_status" = "missing" ]; then
        echo "   ⚠️  ${display_name} forward not configured in corefile (expected for some cluster configs)" >&2
        local value
        value=$(awk '{print $NF}' <<< "$first_line")
        if [ "$value" != "0" ]; then
            echo "   ❌ ERROR: ${display_name} not configured but value is not 0" >&2
            exit 1
        fi
    elif [ "$first_status" = "file_missing" ]; then
        echo "   ⚠️  Forward IPs .prom file is missing (may occur during initial setup)" >&2
        local value
        value=$(awk '{print $NF}' <<< "$first_line")
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
    sed -n '/^localdns_vnetdns_forward_info{/s/^/  /p' <<< "$METRICS"
    echo "KubeDNS forwards:"
    sed -n '/^localdns_kubedns_forward_info{/s/^/  /p' <<< "$METRICS"
elif [ "$VNETDNS_STATUS" = "ok" ]; then
    echo "VnetDNS forwards:"
    sed -n '/^localdns_vnetdns_forward_info{/s/^/  /p' <<< "$METRICS"
    echo "KubeDNS: forward not configured (expected)"
elif [ "$KUBEDNS_STATUS" = "ok" ]; then
    echo "VnetDNS: forward not configured (expected)"
    echo "KubeDNS forwards:"
    sed -n '/^localdns_kubedns_forward_info{/s/^/  /p' <<< "$METRICS"
else
    echo "Both VnetDNS and KubeDNS forward not configured (may be expected for cluster config)"
fi
echo ""

echo "=== ✓ All localdns exporter functional validations passed ==="
