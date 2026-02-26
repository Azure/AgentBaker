package e2e

import (
	"context"
)

// ValidateLocalDNSExporterMetrics checks if the localdns metrics exporter is working
// and exports the expected VnetDNS and KubeDNS forward IP metrics.
func ValidateLocalDNSExporterMetrics(ctx context.Context, s *Scenario) {
	s.T.Helper()

	script := `set -euo pipefail

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
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9353/metrics)
if [ "$HTTP_CODE" -ne 200 ]; then
    echo "   ❌ ERROR: Metrics endpoint returned HTTP $HTTP_CODE"
    exit 1
fi
echo "   ✓ HTTP 200 OK received"
echo ""

# Fetch metrics body
echo "3. Fetching metrics body..."
METRICS=$(curl -s http://localhost:9353/metrics)
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
`

	execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "localdns exporter metrics validation failed")
}
