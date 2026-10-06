#!/bin/bash
# POC: install kubeadm/kubelet/kubectl + CNI from PMC into the edge node image.
#
# This is the bootstrapMode=kubeadm-nocloud counterpart to AKS's CSE/hosted-CP
# install path. It installs to /opt/bin (redirected to /usr/local/bin by the
# lg-redirect sysext) so the location matches AKS and kubelet.service resolves
# the binaries the same way.
set -euo pipefail

K8S_VERSION="${K8S_VERSION:?K8S_VERSION must be set}"
INSTALL_DIR="/opt/bin"

echo "[azurelocal] installing kubeadm/kubelet/kubectl ${K8S_VERSION} from PMC"
mkdir -p "${INSTALL_DIR}"

# Prefer PMC RPMs when available; the exact package names differ across PMC
# channels, so try the versioned names and fall back to unversioned. This is a
# POC seam — wire it to the same package set aksarc-vhd/AzureLocal consumes once
# the shared-build question (analysis §7) is resolved.
if tdnf install -y \
      "kubeadm-${K8S_VERSION}" \
      "kubelet-${K8S_VERSION}" \
      "kubectl-${K8S_VERSION}" \
      kubernetes-cni; then
  echo "[azurelocal] installed versioned k8s packages from PMC"
elif tdnf install -y kubeadm kubelet kubectl kubernetes-cni; then
  echo "[azurelocal] WARNING: installed unversioned k8s packages (POC fallback)"
else
  echo "[azurelocal] ERROR: failed to install k8s packages from PMC" >&2
  exit 1
fi

# Ensure kubelet binaries are discoverable under /opt/bin regardless of where
# the RPM placed them.
for b in kubeadm kubelet kubectl; do
  if [ ! -x "${INSTALL_DIR}/${b}" ]; then
    src="$(command -v "${b}" || true)"
    if [ -n "${src}" ]; then
      ln -sf "${src}" "${INSTALL_DIR}/${b}"
    fi
  fi
done

echo "[azurelocal] k8s install complete:"
"${INSTALL_DIR}/kubeadm" version -o short || true
