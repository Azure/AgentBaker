#!/bin/bash
# POC: stage MULTIPLE k8s versions into the Azure Local edge node image as
# per-version offline RPM repos, mirroring the production aksarc-vhd/AzureLocal
# download-k8s-bin.sh model (one VHD carries many k8s versions; the node installs
# the selected version at bring-up).
#
# Package sources verified against live PMC repodata:
#   kubeadm / kubelet / kubectl  (bare names)  -> prod/cloud-native (1.33/1.34/1.35...)
#   cni-plugins (1.4.0) , containerd (1.7.13)  -> prod/base
#
# For each version it downloads kubeadm/kubelet/kubectl + ALL deps into
# /etc/k8s/<version>/bin, runs createrepo, and registers a local file:// repo at
# /etc/yum.repos.d/k8s-<version>.repo. Nothing is installed into /usr/bin at
# build time (bootstrapMode=kubeadm-nocloud installs the chosen version later).
set -euo pipefail

: "${K8S_VERSIONS:?K8S_VERSIONS must be set (space- or comma-separated list)}"

# Normalize: accept commas or spaces, strip any leading 'v' (v1.33.12 -> 1.33.12).
read -r -a _raw <<< "${K8S_VERSIONS//,/ }"
versions=()
for v in "${_raw[@]}"; do
  [ -z "$v" ] && continue
  versions+=("${v#v}")
done
echo "[azurelocal] staging k8s versions: ${versions[*]}"

# One-time build deps: createrepo for offline metadata, plus the shared runtime
# bits (cni-plugins + containerd) that are not k8s-version-specific.
tdnf install -y createrepo_c cni-plugins containerd
CREATEREPO="$(command -v createrepo_c || command -v createrepo)"
if [ -z "${CREATEREPO}" ]; then
  echo "[azurelocal] ERROR: createrepo not available" >&2
  exit 1
fi

for version in "${versions[@]}"; do
  target_dir="/etc/k8s/${version}/bin"
  echo "[azurelocal] downloading k8s ${version} (kubeadm/kubelet/kubectl + deps) -> ${target_dir}"
  mkdir -p "${target_dir}"
  for pkg in kubeadm kubelet kubectl; do
    tdnf -v install --downloadonly --alldeps --assumeyes \
        --downloaddir "${target_dir}" "${pkg}-${version}" || {
      echo "[azurelocal] ERROR: failed to download ${pkg}-${version}" >&2
      exit 1
    }
  done
  echo "[azurelocal] building offline repo metadata in ${target_dir}"
  "${CREATEREPO}" "${target_dir}"
  cat > "/etc/yum.repos.d/k8s-${version}.repo" <<REPO
[k8s-${version}]
name=Kubernetes Local Repo ${version}
baseurl=file://${target_dir}
enabled=1
gpgcheck=0
REPO
  echo "[azurelocal] staged k8s ${version}:"; ls -1 "${target_dir}" | head
done

# Rebuild tdnf cache once after registering all local repos.
tdnf clean all || true
tdnf makecache || true
echo "[azurelocal] staged ${#versions[@]} k8s version(s): ${versions[*]}"
