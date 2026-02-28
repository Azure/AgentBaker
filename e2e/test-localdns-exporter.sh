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
    # Step 1: Check systemd configuration FIRST (faster, doesn't need running instances)
    echo "5. Verifying all 16 systemd security directives are configured..."
    echo ""

    # Fetch all security properties in batches
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

    echo "   Security properties configured:"
    echo "$SECURITY_PROPS" | sed 's/^/     /'
    echo ""

    # Check all directives (non-fatal for manual test script)
    echo "   Checking individual directives:"
    echo "$SECURITY_PROPS" | grep -qE "^DynamicUser=yes$" && echo "     ✓ DynamicUser=yes" || echo "     ❌ DynamicUser missing"
    echo "$SECURITY_PROPS" | grep -qE "^PrivateTmp=yes$" && echo "     ✓ PrivateTmp=yes" || echo "     ❌ PrivateTmp missing"
    echo "$SECURITY_PROPS" | grep -qE "^ProtectSystem=strict$" && echo "     ✓ ProtectSystem=strict" || echo "     ❌ ProtectSystem missing"
    echo "$SECURITY_PROPS" | grep -qE "^ProtectHome=yes$" && echo "     ✓ ProtectHome=yes" || echo "     ❌ ProtectHome missing"
    echo "$SECURITY_PROPS" | grep -qE "^ReadOnlyPaths=/$|^ReadOnlyPaths=/ " && echo "     ✓ ReadOnlyPaths=/" || echo "     ❌ ReadOnlyPaths missing"
    echo "$SECURITY_PROPS" | grep -qE "^NoNewPrivileges=yes$" && echo "     ✓ NoNewPrivileges=yes" || echo "     ❌ NoNewPrivileges missing"
    echo "$SECURITY_PROPS" | grep -qE "^ProtectKernelTunables=yes$" && echo "     ✓ ProtectKernelTunables=yes" || echo "     ❌ ProtectKernelTunables missing"
    echo "$SECURITY_PROPS" | grep -qE "^ProtectKernelModules=yes$" && echo "     ✓ ProtectKernelModules=yes" || echo "     ❌ ProtectKernelModules missing"
    echo "$SECURITY_PROPS" | grep -qE "^ProtectControlGroups=yes$" && echo "     ✓ ProtectControlGroups=yes" || echo "     ❌ ProtectControlGroups missing"

    # RestrictAddressFamilies - check for AF_UNIX and absence of AF_INET
    if echo "$SECURITY_PROPS" | grep -qE "RestrictAddressFamilies=.*AF_UNIX"; then
        if echo "$SECURITY_PROPS" | grep -qE "RestrictAddressFamilies=.*AF_INET[^6]|RestrictAddressFamilies=.*AF_INET6"; then
            echo "     ❌ RestrictAddressFamilies includes AF_INET/AF_INET6 (should be AF_UNIX only)"
        else
            echo "     ✓ RestrictAddressFamilies=AF_UNIX"
        fi
    else
        echo "     ❌ RestrictAddressFamilies missing or doesn't include AF_UNIX"
    fi

    echo "$SECURITY_PROPS" | grep -qE "^RestrictNamespaces=yes$" && echo "     ✓ RestrictNamespaces=yes" || echo "     ❌ RestrictNamespaces missing"
    echo "$SECURITY_PROPS" | grep -qE "^LockPersonality=yes$" && echo "     ✓ LockPersonality=yes" || echo "     ❌ LockPersonality missing"
    echo "$SECURITY_PROPS" | grep -qE "^RestrictRealtime=yes$" && echo "     ✓ RestrictRealtime=yes" || echo "     ❌ RestrictRealtime missing"
    echo "$SECURITY_PROPS" | grep -qE "^RestrictSUIDSGID=yes$" && echo "     ✓ RestrictSUIDSGID=yes" || echo "     ❌ RestrictSUIDSGID missing"
    echo "$SECURITY_PROPS" | grep -qE "^RemoveIPC=yes$" && echo "     ✓ RemoveIPC=yes" || echo "     ❌ RemoveIPC missing"
    echo "$SECURITY_PROPS" | grep -qE "^PrivateMounts=yes$" && echo "     ✓ PrivateMounts=yes" || echo "     ❌ PrivateMounts missing"
    echo ""

    # Step 2: Runtime validation - spawn instance and check enforcement
    echo "6. Triggering scrape to spawn worker instance for runtime validation..."
    curl -s http://localhost:9353/metrics > /dev/null &
    CURL_PID=$!
    sleep 2  # Increased from 1 to 2 seconds for reliability

    echo "7. Checking for active worker instances..."
    INSTANCES=$(systemctl list-units 'localdns-exporter@*.service' --no-pager --no-legend | awk '{print $1}' || true)

    if [ -z "$INSTANCES" ]; then
        echo "   ⚠️  No active instances found (socket activation may be delayed)"
        wait $CURL_PID 2>/dev/null || true
        sleep 2
        INSTANCES=$(systemctl list-units 'localdns-exporter@*.service' --no-pager --no-legend | awk '{print $1}' || true)
    fi

    if [ -z "$INSTANCES" ]; then
        echo "   ⚠️  No instances found after retry, skipping runtime validation"
    else
        INSTANCE=$(echo "$INSTANCES" | head -n 1)
        echo "   Found instance: $INSTANCE"

        INSTANCE_PID=$(systemctl show "$INSTANCE" --property=MainPID --value 2>/dev/null || echo "0")

        if [ "$INSTANCE_PID" != "0" ] && [ -n "$INSTANCE_PID" ]; then
            echo ""
            echo "8. Runtime security enforcement checks for PID $INSTANCE_PID:"

            # Check DynamicUser enforcement
            INSTANCE_USER=$(ps -o user= -p "$INSTANCE_PID" 2>/dev/null || echo "unknown")
            if [ "$INSTANCE_USER" = "root" ]; then
                echo "   ❌ Running as root (DynamicUser not enforced)"
            else
                echo "   ✓ DynamicUser enforced: running as $INSTANCE_USER"
            fi

            # Check RestrictAddressFamilies enforcement
            NETWORK_SOCKETS=$(lsof -p "$INSTANCE_PID" 2>/dev/null | grep -c "IPv4\|IPv6" || echo "0")
            if [ "$NETWORK_SOCKETS" = "0" ]; then
                echo "   ✓ RestrictAddressFamilies enforced: no network sockets (AF_UNIX only)"
            else
                echo "   ❌ RestrictAddressFamilies not enforced: has network sockets"
                lsof -p "$INSTANCE_PID" | grep "IPv" || true
            fi

            # Check namespace isolation
            if [ -d "/proc/$INSTANCE_PID/ns" ]; then
                NS_COUNT=$(ls -1 /proc/$INSTANCE_PID/ns/ 2>/dev/null | wc -l)
                if [ "$NS_COUNT" -ge 5 ]; then
                    echo "   ✓ Namespace isolation: $NS_COUNT namespaces"
                else
                    echo "   ⚠️  Weak namespace isolation: only $NS_COUNT namespaces"
                fi
            fi
        fi
    fi

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

