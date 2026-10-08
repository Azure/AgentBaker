#!/bin/bash
# POC: stage MULTIPLE k8s versions into the Azure Local edge node image, sourcing
# each piece from the SAME place AKS does wherever an AKS artifact exists:
#
#   kubelet  -> mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext  (AKS source;
#               systemd-sysext image, azlinux3 variant; all target versions present)
#   kubeadm  -> PMC prod/cloud-native RPM   (no AKS equivalent: AKS uses CSE +
#               hosted control plane and ships no kubeadm)
#   kubectl  -> PMC prod/cloud-native RPM
#   cni-plugins / containerd -> pulled in as --alldeps of the above (staged RPMs)
#   system images (apiserver/proxy/coredns/pause) -> already precached from
#               oss/v2/kubernetes/* via components.json (shared with AKS)
#
# IMPORTANT (OSGuard base): /usr is a small hardened partition (~140MB free), so
# we install as FEW tools into the image as possible. Only `oras` is installed
# (needed to pull the kubelet-sysext from MCR). k8s RPMs are DOWNLOADED (not
# installed) to /etc/k8s/<ver>/bin on the roomy rootfs; the node installs the
# selected version's local RPMs + activates its kubelet sysext at bring-up.
set -euo pipefail

: "${K8S_VERSIONS:?K8S_VERSIONS must be set (space- or comma-separated list)}"

KUBELET_SYSEXT_REPO="${KUBELET_SYSEXT_REPO:-mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext}"
SYSEXT_VARIANT="${SYSEXT_VARIANT:-azlinux3-x86-64}"
SYSEXT_DIR="/var/lib/extensions"

# Normalize: accept commas or spaces, strip any leading 'v' (v1.35.8 -> 1.35.8).
read -r -a _raw <<< "${K8S_VERSIONS//,/ }"
versions=()
for v in "${_raw[@]}"; do
  [ -z "$v" ] && continue
  versions+=("${v#v}")
done
echo "[azurelocal] staging k8s versions: ${versions[*]}"

# Keep the /usr footprint minimal: install ONLY oras (needed to pull the
# kubelet-sysext image). Everything else is downloaded to rootfs, not installed.
tdnf clean all || true
tdnf install -y oras
command -v oras >/dev/null || { echo "[azurelocal] ERROR: oras not available" >&2; exit 1; }

mkdir -p "${SYSEXT_DIR}"

for version in "${versions[@]}"; do
  # --- kubeadm + kubectl (+ all runtime deps: cni-plugins, containerd, cri-tools)
  #     DOWNLOADED (not installed) from PMC cloud-native to rootfs -------------
  target_dir="/etc/k8s/${version}/bin"
  echo "[azurelocal] PMC: downloading kubeadm/kubectl ${version} (+deps) -> ${target_dir}"
  mkdir -p "${target_dir}"
  for pkg in kubeadm kubectl; do
    tdnf -v install --downloadonly --alldeps --assumeyes \
        --downloaddir "${target_dir}" "${pkg}-${version}" || {
      echo "[azurelocal] ERROR: failed to download ${pkg}-${version} from PMC" >&2
      exit 1
    }
  done
  echo "[azurelocal] staged $(ls -1 "${target_dir}"/*.rpm 2>/dev/null | wc -l) RPM(s) for ${version}"

  # --- kubelet from AKS's oss/v2 kubelet-sysext image (same source as AKS) -----
  tag="$(oras repo tags "${KUBELET_SYSEXT_REPO}" 2>/dev/null \
          | grep -E "^v${version}-[0-9]+-${SYSEXT_VARIANT}$" \
          | sort -V | tail -1 || true)"
  if [ -z "${tag}" ]; then
    echo "[azurelocal] ERROR: no kubelet-sysext tag for v${version}-*-${SYSEXT_VARIANT} in ${KUBELET_SYSEXT_REPO}" >&2
    exit 1
  fi
  dest="${SYSEXT_DIR}/kubelet-${version}"
  echo "[azurelocal] oss/v2: pulling kubelet sysext ${KUBELET_SYSEXT_REPO}:${tag} -> ${dest}"
  mkdir -p "${dest}"
  oras pull "${KUBELET_SYSEXT_REPO}:${tag}" --output "${dest}" || {
    echo "[azurelocal] ERROR: oras pull failed for ${KUBELET_SYSEXT_REPO}:${tag}" >&2
    exit 1
  }
  raw="$(find "${dest}" -maxdepth 2 -name '*.raw' | head -1 || true)"
  if [ -n "${raw}" ] && [ "${raw}" != "${SYSEXT_DIR}/kubelet-${version}.raw" ]; then
    cp -f "${raw}" "${SYSEXT_DIR}/kubelet-${version}.raw"
  fi
  echo "[azurelocal] staged k8s ${version}: kubeadm/kubectl RPMs + kubelet sysext(${tag})"
done

echo "[azurelocal] staged ${#versions[@]} k8s version(s): ${versions[*]}"
