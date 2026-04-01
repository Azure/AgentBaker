#!/bin/bash

if [ "$(grep ^ID= /etc/os-release | cut -c 4-)" = "flatcar" ]; then
    NODE_IP=$(ip -o -4 addr show dev eth0 | awk '{print $4}' | cut -d '/' -f 1)
else
    NODE_IP=$(hostname -I | awk '{print $1}')
fi

TLS_CONFIG_PATH="${NODE_EXPORTER_TLS_CONFIG_PATH:-/etc/node-exporter.d/web-config.yml}"
TLS_CONFIG_ARG=""

# TLS is disabled by default for backward compatibility:
#   - AKS control plane Prometheus scrapes node-exporter via the API server proxy,
#     which connects to backends over plain HTTP. Enabling TLS breaks this path.
#   - The old node-exporter VM extension also defaulted to no TLS.
#
# To enable TLS, set NODE_EXPORTER_TLS_ENABLED=true in /etc/default/node-exporter.
# Optionally set NODE_EXPORTER_TLS_CLIENT_AUTH to control client cert requirements
# (default: NoClientCert). Valid values: NoClientCert, RequestClientCert,
# RequireAnyClientCert, VerifyClientCertIfGiven, RequireAndVerifyClientCert.
if [ "${NODE_EXPORTER_TLS_ENABLED:-false}" = "true" ]; then
    mkdir -p "$(dirname "$TLS_CONFIG_PATH")"

    TLS_CLIENT_AUTH="${NODE_EXPORTER_TLS_CLIENT_AUTH:-NoClientCert}"

    # Validate client auth type against supported values
    case "$TLS_CLIENT_AUTH" in
        NoClientCert|RequestClientCert|RequireAnyClientCert|VerifyClientCertIfGiven|RequireAndVerifyClientCert) ;;
        *)
            echo "WARNING: unsupported NODE_EXPORTER_TLS_CLIENT_AUTH='$TLS_CLIENT_AUTH', defaulting to NoClientCert"
            TLS_CLIENT_AUTH="NoClientCert"
            ;;
    esac

    # Wait for kubelet serving certs to exist (max 5 minutes).
    # Certs are created by kubelet during bootstrap and may not exist at boot time.
    # NODE_EXPORTER_WAIT_TIMEOUT overrides the default for unit testing.
    WAIT_TIMEOUT="${NODE_EXPORTER_WAIT_TIMEOUT:-300}"
    # Use the same overridable paths as the detection block below.
    ROTATION_CERT_WAIT="${NODE_EXPORTER_ROTATION_CERT:-/var/lib/kubelet/pki/kubelet-server-current.pem}"
    STATIC_CERT_CRT_WAIT="${NODE_EXPORTER_STATIC_CERT_CRT:-/etc/kubernetes/certs/kubeletserver.crt}"
    STATIC_CERT_KEY_WAIT="${NODE_EXPORTER_STATIC_CERT_KEY:-/etc/kubernetes/certs/kubeletserver.key}"
    WAIT_INTERVAL=5
    WAIT_ELAPSED=0

    while [ $WAIT_ELAPSED -lt $WAIT_TIMEOUT ]; do
        if [ -f "$ROTATION_CERT_WAIT" ] || \
           { [ -f "$STATIC_CERT_CRT_WAIT" ] && [ -f "$STATIC_CERT_KEY_WAIT" ]; }; then
            break
        fi
        echo "Waiting for kubelet serving certs... (${WAIT_ELAPSED}s/${WAIT_TIMEOUT}s)"
        sleep $WAIT_INTERVAL
        WAIT_ELAPSED=$((WAIT_ELAPSED + WAIT_INTERVAL))
    done

    # Detect TLS cert paths.
    # NODE_EXPORTER_ROTATION_CERT / NODE_EXPORTER_STATIC_CERT_CRT / KEY
    # are overridable for unit testing; production values are the canonical paths.
    ROTATION_CERT="${NODE_EXPORTER_ROTATION_CERT:-/var/lib/kubelet/pki/kubelet-server-current.pem}"
    STATIC_CERT_CRT="${NODE_EXPORTER_STATIC_CERT_CRT:-/etc/kubernetes/certs/kubeletserver.crt}"
    STATIC_CERT_KEY="${NODE_EXPORTER_STATIC_CERT_KEY:-/etc/kubernetes/certs/kubeletserver.key}"

    # Priority: rotation cert > static certs
    CERT_FILE=""
    KEY_FILE=""

    if [ -f "$ROTATION_CERT" ]; then
        CERT_FILE="$ROTATION_CERT"
        KEY_FILE="$ROTATION_CERT"
        echo "Using kubelet serving certificate rotation cert: $CERT_FILE"
    elif [ -f "$STATIC_CERT_CRT" ] && [ -f "$STATIC_CERT_KEY" ]; then
        CERT_FILE="$STATIC_CERT_CRT"
        KEY_FILE="$STATIC_CERT_KEY"
        echo "Using static kubelet serving certs: $CERT_FILE, $KEY_FILE"
    else
        echo "WARNING: TLS enabled but no kubelet serving certs found after ${WAIT_TIMEOUT}s. node-exporter will run without TLS."
    fi

    if [ -n "$CERT_FILE" ] && [ -n "$KEY_FILE" ]; then
        if [ "$TLS_CLIENT_AUTH" != "NoClientCert" ]; then
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
        echo "TLS configured: client_auth_type=$TLS_CLIENT_AUTH, cert=$CERT_FILE"
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

exec "${NODE_EXPORTER_BIN:-/opt/bin/node-exporter}" "${ARGS[@]}"
