#!/bin/bash

PCI_DEVICES_PATH="${PCI_DEVICES_PATH:-/sys/bus/pci/devices}"
MANA_OBSERVED_FILE="${MANA_OBSERVED_FILE:-/run/node-exporter-mana-observed}"

getNodeExporterHardwareArgs() {
    if [ -f "$MANA_OBSERVED_FILE" ]; then
        printf '%s\n' '--no-collector.infiniband'
        return
    fi
    local device
    for device in "${PCI_DEVICES_PATH}"/*; do
        if [ -d "$device" ] &&
           grep -qi '^0x1414$' "$device/vendor" 2>/dev/null &&
           grep -Eqi '^0x00(b9|ba|c1)$' "$device/device" 2>/dev/null; then
            touch "$MANA_OBSERVED_FILE" || return 1
            printf '%s\n' '--no-collector.infiniband'
            return
        fi
    done
}

nodeExporterMANAAdded() {
    # Record the event before requesting a restart, even if the VF disappears
    # again before ExecStart runs. Never start an inactive/preprovisioned service.
    touch "$MANA_OBSERVED_FILE" || return 1
    # Several VFs may arrive together. Do not restart an exporter that already
    # applied the workaround, but do not use marker existence as proof of that.
    local pid
    pid=$(systemctl show --property=MainPID --value node-exporter.service) || return 1
    if [[ "$pid" =~ ^[1-9][0-9]*$ ]] &&
       grep -zFxq -- '--no-collector.infiniband' "/proc/${pid}/cmdline" 2>/dev/null; then
        return 0
    fi
    systemctl --no-block try-restart node-exporter.service
}

if [ "${NODE_EXPORTER_STARTUP_SOURCE_ONLY:-false}" = "true" ]; then
    return 0
fi

if [ "${1:-}" = "--mana-added" ]; then
    nodeExporterMANAAdded
    exit $?
fi

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

# MANA's RDMA driver publicly supports /sys/class/infiniband, but its rate file
# returns EINVAL with the parser used by node-exporter 1.12.1. node-exporter also
# parses every device before applying either its device include or exclude
# filter, so neither flag can avoid the failure. Detect MANA by its assigned PCI
# IDs (Microsoft 1414; MANA PF 00b9, VF 00ba, PF2 00c1), independently of mana_ib
# registration. Azure servicing can remove/re-add PCI VFs after boot: remember
# MANA in /run across service restarts and use the PCI-add udev rule installed
# by install-node-exporter.sh to re-evaluate on late attachment. /run resets on
# reboot and does not carry build-VM hardware observations into new nodes.
# This suppresses all InfiniBand metrics, including other HCAs on mixed nodes,
# until upstream supports filtering before parsing devices.
# https://github.com/prometheus/node_exporter/issues/3810
# https://learn.microsoft.com/azure/virtual-network/accelerated-networking-mana-linux
# https://github.com/torvalds/linux/blob/master/include/net/mana/gdma.h
HARDWARE_ARG=$(getNodeExporterHardwareArgs) || exit 1
if [ -n "$HARDWARE_ARG" ]; then
    ARGS+=("$HARDWARE_ARG")
fi

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
