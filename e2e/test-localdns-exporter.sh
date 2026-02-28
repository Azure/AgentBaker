#!/bin/bash
# Test script for localdns metrics exporter
# This script demonstrates how the exporter works and can be used for local testing

set -euo pipefail

echo "=== Localdns Metrics Exporter Test Script ==="
echo ""

# Check if systemctl is available
if ! command -v systemctl &> /dev/null; then
    echo "ERROR: systemctl is not available. This script requires systemd."
    exit 1
fi

# Check if localdns.service exists
if ! systemctl list-unit-files | grep -q "^localdns.service"; then
    echo "WARNING: localdns.service is not installed on this system."
    echo "The exporter will still work but will return zero values."
fi

echo "1. Testing the exporter script directly:"
echo "   Running: /opt/azure/containers/localdns/localdns_exporter.sh"
echo ""

if [ -f "/opt/azure/containers/localdns/localdns_exporter.sh" ]; then
    echo "GET /metrics HTTP/1.1" | /opt/azure/containers/localdns/localdns_exporter.sh
else
    echo "   ERROR: Exporter script not found at /opt/azure/containers/localdns/localdns_exporter.sh"
    echo "   Run VHD build first to install the files."
    exit 1
fi

echo ""
echo "2. Checking if the exporter socket is enabled:"
systemctl is-enabled localdns-exporter.socket || echo "   Socket is not enabled"

echo ""
echo "3. Checking if the exporter socket is active:"
systemctl is-active localdns-exporter.socket || echo "   Socket is not active"

echo ""
echo "4. Testing metrics via HTTP (if socket is running):"
if systemctl is-active --quiet localdns-exporter.socket; then
    echo "   Fetching metrics from localhost:9353/metrics..."
    curl -s http://localhost:9353/metrics || echo "   Failed to fetch metrics"
else
    echo "   Socket is not running. Start it with: systemctl start localdns-exporter.socket"
fi

echo ""
echo "=== Test Complete ==="
echo ""

# Additional: Security hardening validation
echo "=== Security Hardening Validation ==="
echo ""

if systemctl is-active --quiet localdns-exporter.socket; then
    echo "5. Triggering scrape to spawn worker instance..."
    curl -s http://localhost:9353/metrics > /dev/null &
    CURL_PID=$!
    sleep 1

    echo "6. Checking for active worker instances..."
    INSTANCES=$(systemctl list-units 'localdns-exporter@*.service' --no-pager --no-legend | awk '{print $1}' || true)

    if [ -z "$INSTANCES" ]; then
        echo "   ⚠️  No active instances found (socket activation may be delayed)"
    else
        INSTANCE=$(echo "$INSTANCES" | head -n 1)
        echo "   Found instance: $INSTANCE"

        INSTANCE_PID=$(systemctl show "$INSTANCE" --property=MainPID --value 2>/dev/null || echo "0")

        if [ "$INSTANCE_PID" != "0" ] && [ -n "$INSTANCE_PID" ]; then
            echo ""
            echo "7. Security checks for PID $INSTANCE_PID:"

            # Check user
            INSTANCE_USER=$(ps -o user= -p "$INSTANCE_PID" 2>/dev/null || echo "unknown")
            if [ "$INSTANCE_USER" = "root" ]; then
                echo "   ❌ Running as root (should be DynamicUser)"
            else
                echo "   ✓ Running as dynamic user: $INSTANCE_USER"
            fi

            # Check for network sockets
            NETWORK_SOCKETS=$(lsof -p "$INSTANCE_PID" 2>/dev/null | grep -c "IPv4\|IPv6" || echo "0")
            if [ "$NETWORK_SOCKETS" = "0" ]; then
                echo "   ✓ No network sockets (AF_UNIX only)"
            else
                echo "   ❌ Has network sockets (should be AF_UNIX only)"
            fi
        fi
    fi

    # Check systemd properties
    echo ""
    echo "8. Systemd security properties:"
    systemctl show localdns-exporter@.service \
        --property=DynamicUser,PrivateTmp,ProtectSystem,ProtectHome,RestrictAddressFamilies \
        2>/dev/null | sed 's/^/   /' || echo "   (Could not retrieve properties)"

    wait $CURL_PID 2>/dev/null || true
else
    echo "Socket is not running, skipping security validation"
fi

echo ""
echo "To enable and start the exporter:"
echo "  sudo systemctl enable --now localdns-exporter.socket"
echo ""
echo "To test the metrics endpoint:"
echo "  curl http://localhost:9353/metrics"
echo ""
echo "VMAgent will scrape metrics from port 9353 automatically."

