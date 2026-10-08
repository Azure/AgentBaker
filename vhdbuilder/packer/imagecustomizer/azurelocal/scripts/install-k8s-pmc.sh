#!/bin/bash
# POC: stage MULTIPLE k8s versions into the Azure Local edge node image, sourcing
# each piece from the SAME place AKS does wherever an AKS artifact exists:
#
#   kubelet  -> mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext  (AKS source;
#               systemd-sysext image, azlinux3 variant; all target versions present)
#   kubeadm  -> PMC prod/cloud-native RPM   (no AKS equivalent: AKS uses CSE +
#               hosted control plane and ships no kubeadm)
#   kubectl  -> PMC prod/cloud-native RPM   (bundled with the kubeadm staging)
#   system images (apiserver/proxy/coredns/pause) -> already precached from
#               oss/v2/kubernetes/* via components.json (shared with AKS)
#
# Delivery (mirrors production's multi-version-per-VHD model):
#   - kubeadm/kubectl (+deps) are downloaded per version into /etc/k8s/<ver>/bin
#     and turned into an offline tdnf repo (k8s-<ver>.repo); the node installs
#     the selected version at bring-up.
#   - kubelet is staged per version as a systemd-sysext image under
#     /var/lib/extensions/kubelet-<ver>/ ; the node activates the selected
#     version's sysext at bring-up (same mechanism AKS uses).
set -euo pipefail

: "${K8S_VERSIONS:?K8S_VERSIONS must be set (space- or comma-separated list)}"

KUBELET_SYSEXT_REPO="${KUBELET_SYSEXT_REPO:-mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext}"
# Azure Linux 3 / x86_64 sysext variant tag suffix (verified present on MCR).
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

# One-time build deps: createrepo (offline metadata), oras (pull kubelet-sysext
# from MCR), plus the shared runtime bits (cni-plugins + containerd) from base.
tdnf install -y createrepo_c oras cni-plugins containerd
CREATEREPO="$(command -v createrepo_c || command -v createrepo)"
[ -n "${CREATEREPO}" ] || { echo "[azurelocal] ERROR: createrepo not available" >&2; exit 1; }
command -v oras >/dev/null || { echo "[azurelocal] ERROR: oras not available" >&2; exit 1; }

mkdir -p "${SYSEXT_DIR}"

for version in "${versions[@]}"; do
  # --- kubeadm + kubectl (+deps) from PMC cloud-native -> per-version offline repo
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
  "${CREATEREPO}" "${target_dir}"
  cat > "/etc/yum.repos.d/k8s-${version}.repo" <<REPO
[k8s-${version}]
name=Kubernetes Local Repo ${version}
baseurl=file://${target_dir}
enabled=1
gpgcheck=0
REPO

  # --- kubelet from AKS's oss/v2 kubelet-sysext image (same source as AKS) ------
  # Resolve the newest build of this version's azlinux3 x86_64 sysext tag.
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
  # Normalize the sysext raw image name so the node can activate kubelet-<ver>.
  raw="$(find "${dest}" -maxdepth 2 -name '*.raw' | head -1 || true)"
  if [ -n "${raw}" ] && [ "${raw}" != "${SYSEXT_DIR}/kubelet-${version}.raw" ]; then
    cp -f "${raw}" "${SYSEXT_DIR}/kubelet-${version}.raw"
  fi
  echo "[azurelocal] staged k8s ${version}: kubeadm/kubectl(PMC repo) + kubelet sysext(${tag})"
done

tdnf clean all || true
tdnf makecache || true
echo "[azurelocal] staged ${#versions[@]} k8s version(s): ${versions[*]}"
