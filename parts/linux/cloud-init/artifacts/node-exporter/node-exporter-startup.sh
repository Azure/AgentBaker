#!/bin/bash

if [ "$(cat /etc/os-release | grep ^ID= | cut -c 4-)" = "flatcar" ]; then
    NODE_IP=$(ip -o -4 addr show dev eth0 | awk '{print $4}' | cut -d '/' -f 1)
else
    NODE_IP=$(hostname -I | awk '{print $1}')
fi

TLS_CONFIG_PATH="/etc/node-exporter.d/web-config.yml"
TLS_CONFIG_ARG=""

# Ensure TLS config directory exists
mkdir -p "$(dirname "$TLS_CONFIG_PATH")"

# Check IMDS tag to determine cert rotation setting (same logic as CSE)
# If aks-disable-kubelet-serving-certificate-rotation=true, use static certs
# Otherwise, use rotation cert
ROTATION_DISABLED="false"
IMDS_CACHE_FILE="/opt/azure/containers/imds_instance_metadata_cache.json"

# Use CSE's cached IMDS response if available, otherwise fetch directly
if [ -f "$IMDS_CACHE_FILE" ]; then
    IMDS_RESPONSE=$(cat "$IMDS_CACHE_FILE")
else
    IMDS_RESPONSE=$(curl -fsSL -H "Metadata: true" --noproxy "*" --retry 20 --retry-delay 2 --retry-connrefused --connect-timeout 5 --max-time 60 "http://169.254.169.254/metadata/instance?api-version=2021-02-01" 2>/dev/null)
fi

if [ -z "$IMDS_RESPONSE" ]; then
    echo "WARNING: Failed to fetch IMDS metadata, assuming cert rotation is enabled"
fi

if [ -n "$IMDS_RESPONSE" ]; then
    ROTATION_DISABLED=$(echo "$IMDS_RESPONSE" | jq -r '.compute.tagsList | map(select(.name | test("aks-disable-kubelet-serving-certificate-rotation"; "i")))[0].value // "false" | test("true"; "i")' 2>/dev/null || echo "false")
fi

# Wait for the appropriate cert to exist (max 5 minutes)
# Certs are created by kubelet during its bootstrap process when it connects
# to the API server, so they may not exist immediately at boot time
WAIT_TIMEOUT=300
WAIT_INTERVAL=5
WAIT_ELAPSED=0

while [ $WAIT_ELAPSED -lt $WAIT_TIMEOUT ]; do
    if [ "$ROTATION_DISABLED" = "true" ]; then
        # Rotation disabled - wait for static certs
        if [ -f "/etc/kubernetes/certs/kubeletserver.crt" ] && [ -f "/etc/kubernetes/certs/kubeletserver.key" ]; then
            break
        fi
    else
        # Rotation enabled - wait for rotation cert
        if [ -f "/var/lib/kubelet/pki/kubelet-server-current.pem" ]; then
            break
        fi
    fi
    # Also check the other cert type in case IMDS tag detection was wrong
    if [ -f "/var/lib/kubelet/pki/kubelet-server-current.pem" ] || \
       { [ -f "/etc/kubernetes/certs/kubeletserver.crt" ] && [ -f "/etc/kubernetes/certs/kubeletserver.key" ]; }; then
        break
    fi
    sleep $WAIT_INTERVAL
    WAIT_ELAPSED=$((WAIT_ELAPSED + WAIT_INTERVAL))
done

# Detect TLS cert paths
# Priority: rotation cert > static certs > skip TLS
CERT_FILE=""
KEY_FILE=""

if [ -f "/var/lib/kubelet/pki/kubelet-server-current.pem" ]; then
    CERT_FILE="/var/lib/kubelet/pki/kubelet-server-current.pem"
    KEY_FILE="/var/lib/kubelet/pki/kubelet-server-current.pem"
    echo "Using kubelet serving certificate rotation cert: $CERT_FILE"
elif [ -f "/etc/kubernetes/certs/kubeletserver.crt" ] && [ -f "/etc/kubernetes/certs/kubeletserver.key" ]; then
    CERT_FILE="/etc/kubernetes/certs/kubeletserver.crt"
    KEY_FILE="/etc/kubernetes/certs/kubeletserver.key"
    echo "Using static kubelet serving certs: $CERT_FILE, $KEY_FILE"
else
    echo "WARNING: No kubelet serving certs found after ${WAIT_TIMEOUT}s, node-exporter will run without TLS. Restart the service after certs are available to enable TLS."
fi

# Configure TLS if we found valid cert paths
if [ -n "$CERT_FILE" ] && [ -n "$KEY_FILE" ]; then
    cat > "$TLS_CONFIG_PATH" <<EOF
tls_server_config:
  cert_file: "$CERT_FILE"
  key_file: "$KEY_FILE"
  client_auth_type: "RequireAndVerifyClientCert"
  client_ca_file: "/etc/kubernetes/certs/ca.crt"
EOF
    TLS_CONFIG_ARG="--web.config.file=${TLS_CONFIG_PATH}"
fi

ARGS=(
    --web.listen-address="${NODE_IP}:19100"
    --no-collector.wifi
    --no-collector.hwmon
    --collector.cpu.info
    --collector.filesystem.mount-points-exclude="^/(dev|proc|sys|run/containerd/.+|var/lib/docker/.+|var/lib/kubelet/.+)($|/)"
    --collector.netclass.ignored-devices="^(azv.*|veth.*|[a-f0-9]{15})$"
    --collector.netclass.netlink
    --collector.netdev.device-exclude="^(azv.*|veth.*|[a-f0-9]{15})$"
    --no-collector.arp.netlink
)

if [ -n "$TLS_CONFIG_ARG" ]; then
    ARGS+=("$TLS_CONFIG_ARG")
fi

exec /opt/bin/node-exporter "${ARGS[@]}"
