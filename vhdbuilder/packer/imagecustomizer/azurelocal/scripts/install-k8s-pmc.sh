#!/bin/bash
# POC: install kubeadm/kubelet/kubectl + CNI + containerd from PMC into the edge
# node image.
#
# This is the bootstrapMode=kubeadm-nocloud counterpart to AKS's CSE/hosted-CP
# install path. Azure Linux 3.0 'base' channel package names (verified against
# PMC repodata) differ from upstream:
#   kubernetes-kubeadm -> /usr/bin/kubeadm
#   kubernetes         -> /usr/bin/kubelet
#   kubernetes-client  -> /usr/bin/kubectl
#   cni-plugins        -> CNI plugins
#   containerd 1.7.13  -> container runtime
# Binaries land in /usr/bin; we symlink them into /opt/bin so the location
# matches AKS (lg-redirect sysext maps /usr/local/bin -> /opt/bin).
set -euo pipefail

K8S_VERSION="${K8S_VERSION:?K8S_VERSION must be set}"
INSTALL_DIR="/opt/bin"

echo "[azurelocal] installing kubernetes ${K8S_VERSION} (kubeadm/kubelet/kubectl) + cni-plugins + containerd from PMC base"
mkdir -p "${INSTALL_DIR}"

# Try the exact k8s version first (tdnf picks the highest release for that EVR),
# then fall back to the latest available in the channel. cni-plugins/containerd
# are not k8s-version-pinned.
if tdnf install -y \
      "kubernetes-kubeadm-${K8S_VERSION}" \
      "kubernetes-${K8S_VERSION}" \
      "kubernetes-client-${K8S_VERSION}" \
      cni-plugins \
      containerd; then
  echo "[azurelocal] installed k8s ${K8S_VERSION} packages from PMC base"
elif tdnf install -y \
      kubernetes-kubeadm \
      kubernetes \
      kubernetes-client \
      cni-plugins \
      containerd; then
  echo "[azurelocal] WARNING: pinned version ${K8S_VERSION} unavailable; installed latest k8s from PMC base (POC fallback)"
else
  echo "[azurelocal] ERROR: failed to install k8s packages from PMC base" >&2
  exit 1
fi

# Expose kubeadm/kubelet/kubectl under /opt/bin (RPMs install to /usr/bin).
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
