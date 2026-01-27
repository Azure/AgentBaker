#!/bin/sh

if [ "$(cat /etc/os-release | grep ^ID= | cut -c 4-)" = "flatcar" ]; then
    NODE_IP=$(ip -o -4 addr show dev eth0 | awk '{print $4}' | cut -d '/' -f 1)
else
    NODE_IP=$(hostname -I | awk '{print $1}')
fi

TLS_CONFIG_PATH="/etc/node-exporter.d/web-config.yml"
TLS_CONFIG_ARG=""
KUBELET_DEFAULTS="/etc/default/kubelet"

# Detect TLS cert paths from kubelet configuration
# Priority: rotation cert > static cert paths from kubelet flags > skip TLS
CERT_FILE=""
KEY_FILE=""

# Check for rotation cert first (used when --rotate-server-certificates=true)
if [ -f "/var/lib/kubelet/pki/kubelet-server-current.pem" ]; then
    CERT_FILE="/var/lib/kubelet/pki/kubelet-server-current.pem"
    KEY_FILE="/var/lib/kubelet/pki/kubelet-server-current.pem"
elif [ -f "$KUBELET_DEFAULTS" ]; then
    # Parse kubelet flags for static cert paths
    KUBELET_FLAGS=$(grep "^KUBELET_FLAGS=" "$KUBELET_DEFAULTS" | cut -d'=' -f2-)
    TLS_CERT=$(echo "$KUBELET_FLAGS" | grep -o '\--tls-cert-file=[^ ]*' | cut -d'=' -f2)
    TLS_KEY=$(echo "$KUBELET_FLAGS" | grep -o '\--tls-private-key-file=[^ ]*' | cut -d'=' -f2)
    
    if [ -n "$TLS_CERT" ] && [ -n "$TLS_KEY" ] && [ -f "$TLS_CERT" ] && [ -f "$TLS_KEY" ]; then
        CERT_FILE="$TLS_CERT"
        KEY_FILE="$TLS_KEY"
    fi
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

exec /opt/bin/node-exporter \
    --web.listen-address=${NODE_IP}:19100 \
    ${TLS_CONFIG_ARG} \
    --no-collector.wifi \
    --no-collector.hwmon \
    --collector.cpu.info \
    --collector.filesystem.mount-points-exclude="^/(dev|proc|sys|run/containerd/.+|var/lib/docker/.+|var/lib/kubelet/.+)($|/)" \
    --collector.netclass.ignored-devices="^(azv.*|veth.*|[a-f0-9]{15})$" \
    --collector.netclass.netlink \
    --collector.netdev.device-exclude="^(azv.*|veth.*|[a-f0-9]{15})$" \
    --no-collector.arp.netlink
