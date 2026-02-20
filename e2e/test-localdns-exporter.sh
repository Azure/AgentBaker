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
    /opt/azure/containers/localdns/localdns_exporter.sh
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
    echo "   Fetching metrics from localhost:9353..."
    curl -s http://localhost:9353 || echo "   Failed to fetch metrics"
else
    echo "   Socket is not running. Start it with: systemctl start localdns-exporter.socket"
fi

echo ""
echo "=== Test Complete ==="
echo ""
echo "To enable and start the exporter:"
echo "  sudo systemctl enable --now localdns-exporter.socket"
echo ""
echo "To test the metrics endpoint:"
echo "  curl http://localhost:9353"
echo ""
echo "VMAgent will scrape metrics from port 9353 automatically."
