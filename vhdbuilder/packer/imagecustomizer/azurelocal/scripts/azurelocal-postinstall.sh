#!/bin/bash
# POC: postCustomization for the Azure Local edge node-image variant.
#
# Runs INSIDE the Image Customizer chroot (same execution model as
# ../azlosguard/scripts/azlosguard-postinstall.sh). It:
#   1. validates the three gated parameters,
#   2. prepares the /opt/bin <-> /usr/local/bin redirect used during build,
#   3. installs k8s (kubeadm/kubelet/kubectl) from PMC  [bootstrapMode=kubeadm-nocloud],
#   4. pre-caches MCR oss/v2 system images from components.json  [shared with AKS],
#   5. installs the edge agent set                     [agentSet=edge],
#   6. asserts NO AKS CSE bootstrap units are present  [bootstrapMode guard].
set -euo pipefail

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete

required_env_vars=(
    "IMG_SKU"
    "BOOTSTRAP_MODE"
    "AGENT_SET"
    "PUBLISH_TARGET"
    "K8S_VERSIONS"
)
for v in "${required_env_vars[@]}"; do
    if [ -z "${!v:-}" ]; then
        echo "$v was not set!" >&2
        exit 1
    fi
    echo "$v is set to '${!v}'"
done

# --- POC scope guard: this config only implements the edge combination. -------
if [ "${BOOTSTRAP_MODE}" != "kubeadm-nocloud" ]; then
    echo "ERROR: azurelocal POC only supports BOOTSTRAP_MODE=kubeadm-nocloud (got '${BOOTSTRAP_MODE}')" >&2
    exit 1
fi
if [ "${AGENT_SET}" != "edge" ]; then
    echo "ERROR: azurelocal POC only supports AGENT_SET=edge (got '${AGENT_SET}')" >&2
    exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

echo "Starting azurelocal edge build on $(date)" > "${VHD_LOGS_FILEPATH}"
echo "bootstrapMode=${BOOTSTRAP_MODE} agentSet=${AGENT_SET} publishTarget=${PUBLISH_TARGET}" >> "${VHD_LOGS_FILEPATH}"

# --- repart fixups carried over from the OSGuard base image --------------------
sed -i 's/Type=usr/Type=linux-generic/' /etc/repart.d/12-usr-a.conf || true
rm -f /etc/repart.d/15-boot-b.conf /etc/repart.d/16-usr-b.conf /etc/repart.d/17-usr-hash-b.conf || true

# --- /opt/bin <-> /usr/local/bin redirect (matches azlosguard) -----------------
mkdir -p /etc/extensions/lg-redirect-sysext/usr/local/
mkdir -p /opt/bin
ln -sf /opt/bin /etc/extensions/lg-redirect-sysext/usr/local/bin
mount --bind /opt/bin /usr/local/bin
trap "umount /usr/local/bin" EXIT

# --- (3) stage k8s versions from PMC  [bootstrapMode=kubeadm-nocloud] ----------
K8S_VERSIONS="${K8S_VERSIONS}" bash "${SCRIPT_DIR}/install-k8s-pmc.sh"

# --- (4) pre-cache MCR oss/v2 system images from components.json (shared) ------
# Start containerd to allow container precaching, fetch-only to save space.
containerd &
CONTAINERD_PID=$!
# shellcheck disable=2064
trap "kill ${CONTAINERD_PID} 2>/dev/null || true; umount /usr/local/bin 2>/dev/null || true" EXIT
export IMAGE_FETCH_ONLY=true
if [ -x /opt/azure/containers/install-dependencies.sh ]; then
    /opt/azure/containers/install-dependencies.sh || echo "WARNING: install-dependencies.sh returned non-zero (POC)"
fi

# --- (5) edge agent set  [agentSet=edge] ---------------------------------------
# POC placeholder: the edge agents (lbagent, cert-tattoo, fluent-bit, haproxy,
# keepalived) are owned by aksarc-vhd/AzureLocal today. Wire their install here
# when converging pipelines. Intentionally does NOT install the AKS cloud agents
# (acr-credential-provider, secure-tls-bootstrap, node-exporter, aks-log-collector).
echo "[azurelocal] agentSet=edge: edge agents install is a POC placeholder (see README)"

# --- list images for image-bom.json (shared build artifact) --------------------
if [ -x /opt/azure/containers/list-images.sh ]; then
    /opt/azure/containers/list-images.sh || true
fi

# --- (6) assert no AKS CSE bootstrap leaked into the edge image ----------------
for forbidden in \
    /etc/systemd/system/aks-node-controller.service \
    /etc/systemd/system/secure-tls-bootstrap.service \
    /opt/azure/containers/aks-node-controller \
    /opt/azure/containers/provision.sh; do
    if [ -e "${forbidden}" ]; then
        echo "ERROR: AKS CSE bootstrap artifact present in edge image: ${forbidden}" >&2
        exit 1
    fi
done

echo "azurelocal edge postinstall completed on $(date)" >> "${VHD_LOGS_FILEPATH}"
