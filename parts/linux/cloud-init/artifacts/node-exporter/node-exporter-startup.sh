#!/bin/bash

if [ "$(grep ^ID= /etc/os-release | cut -c 4-)" = "flatcar" ]; then
    NODE_IP=$(ip -o -4 addr show dev eth0 | awk '{print $4}' | cut -d '/' -f 1)
else
    NODE_IP=$(hostname -I | awk '{print $1}')
fi

TLS_CONFIG_PATH="/etc/node-exporter.d/web-config.yml"
TLS_CONFIG_ARG=""

# TLS is disabled by default for backward compatibility:
#   - AKS control plane Prometheus scrapes node-exporter via the API server proxy,
#     which connects to backends over plain HTTP. Enabling TLS breaks this path.
#   - The old node-exporter VM extension also defaulted to no TLS.
#
# To enable TLS, set NODE_EXPORTER_TLS_ENABLED=true in /etc/default/node-exporter.
# Optionally set NODE_EXPORTER_TLS_CLIENT_AUTH to control client cert requirements
# (default: NoClientCert). Valid values: NoClientCert, RequireAndVerifyClientCert,
# RequireAnyClientCert, VerifyClientCertIfGiven.
if [ "${NODE_EXPORTER_TLS_ENABLED:-false}" = "true" ]; then
    mkdir -p "$(dirname "$TLS_CONFIG_PATH")"

    TLS_CLIENT_AUTH="${NODE_EXPORTER_TLS_CLIENT_AUTH:-NoClientCert}"

    # Wait for kubelet serving certs to exist (max 5 minutes).
    # Certs are created by kubelet during bootstrap and may not exist at boot time.
    WAIT_TIMEOUT=300
    WAIT_INTERVAL=5
    WAIT_ELAPSED=0

    while [ $WAIT_ELAPSED -lt $WAIT_TIMEOUT ]; do
        if [ -f "/var/lib/kubelet/pki/kubelet-server-current.pem" ] || \
           { [ -f "/etc/kubernetes/certs/kubeletserver.crt" ] && [ -f "/etc/kubernetes/certs/kubeletserver.key" ]; }; then
            break
        fi
        echo "Waiting for kubelet serving certs... (${WAIT_ELAPSED}s/${WAIT_TIMEOUT}s)"
        sleep $WAIT_INTERVAL
        WAIT_ELAPSED=$((WAIT_ELAPSED + WAIT_INTERVAL))
    done

    # Detect TLS cert paths
    # Priority: rotation cert > static certs
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
        echo "WARNING: TLS enabled but no kubelet serving certs found after ${WAIT_TIMEOUT}s. node-exporter will run without TLS."
    fi

    if [ -n "$CERT_FILE" ] && [ -n "$KEY_FILE" ]; then
        if [ "$TLS_CLIENT_AUTH" = "RequireAndVerifyClientCert" ] || [ "$TLS_CLIENT_AUTH" = "RequireAnyClientCert" ] || [ "$TLS_CLIENT_AUTH" = "VerifyClientCertIfGiven" ]; then
            cat > "$TLS_CONFIG_PATH" <<EOF
tls_server_config:
  cert_file: "$CERT_FILE"
  key_file: "$KEY_FILE"
  client_auth_type: "$TLS_CLIENT_AUTH"
  client_ca_file: "/etc/kubernetes/certs/ca.crt"
EOF
        else
            cat > "$TLS_CONFIG_PATH" <<EOF
tls_server_config:
  cert_file: "$CERT_FILE"
  key_file: "$KEY_FILE"
  client_auth_type: "NoClientCert"
EOF
        fi
        echo "TLS configured with client_auth_type=$TLS_CLIENT_AUTH, cert=$CERT_FILE"
        TLS_CONFIG_ARG="--web.config.file=${TLS_CONFIG_PATH}"
    fi
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

# Append extra args from EnvironmentFile (e.g., /etc/default/node-exporter)
# Example: NODE_EXPORTER_EXTRA_ARGS="--collector.systemd --no-collector.bonding"
if [ -n "${NODE_EXPORTER_EXTRA_ARGS:-}" ]; then
    read -ra EXTRA <<< "$NODE_EXPORTER_EXTRA_ARGS"
    ARGS+=("${EXTRA[@]}")
fi

exec /opt/bin/node-exporter "${ARGS[@]}"
