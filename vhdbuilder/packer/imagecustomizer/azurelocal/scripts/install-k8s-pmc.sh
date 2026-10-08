#!/bin/bash
# POC: stage MULTIPLE k8s versions into the Azure Local edge node image.
#
#   kubeadm / kubelet / kubectl (+ runtime deps: cni-plugins / containerd /
#   cri-tools) are DOWNLOADED (not installed) from PMC prod/cloud-native into
#   /etc/k8s/<ver>/bin on the roomy rootfs. The node installs the selected
#   version's local RPMs at bring-up.
#
#   system images (apiserver/proxy/coredns/pause) -> already precached from
#   oss/v2/kubernetes/* via components.json (shared with AKS).
#
# All three k8s binaries come from PMC (the same source the Aks-Arc-Assembly
# AzureLocal VHD build uses). We intentionally do NOT pull the kubelet-sysext
# image via oras: the build agent routes outbound through a proxy that tdnf
# honours but a direct `oras ... mcr.microsoft.com` call does not, so the tag
# lookup fails. PMC already ships kubelet for every target version.
#
# IMPORTANT (OSGuard base): /usr is a small hardened partition (~140MB free), so
# we install NOTHING into /usr. All k8s packages are downloaded to the rootfs.
set -euo pipefail

: "${K8S_VERSIONS:?K8S_VERSIONS must be set (space- or comma-separated list)}"

# Normalize: accept commas or spaces, strip any leading 'v' (v1.35.8 -> 1.35.8).
read -r -a _raw <<< "${K8S_VERSIONS//,/ }"
versions=()
for v in "${_raw[@]}"; do
  [ -z "$v" ] && continue
  versions+=("${v#v}")
done
echo "[azurelocal] staging k8s versions: ${versions[*]}"

tdnf clean all || true

for version in "${versions[@]}"; do
  # kubeadm/kubelet/kubectl (+ all runtime deps: cni-plugins, containerd,
  # cri-tools) DOWNLOADED (not installed) from PMC cloud-native to rootfs.
  target_dir="/etc/k8s/${version}/bin"
  echo "[azurelocal] PMC: downloading kubeadm/kubelet/kubectl ${version} (+deps) -> ${target_dir}"
  mkdir -p "${target_dir}"
  for pkg in kubeadm kubelet kubectl; do
    tdnf -v install --downloadonly --alldeps --assumeyes \
        --downloaddir "${target_dir}" "${pkg}-${version}" || {
      echo "[azurelocal] ERROR: failed to download ${pkg}-${version} from PMC" >&2
      exit 1
    }
  done
  rpm_count="$(ls -1 "${target_dir}"/*.rpm 2>/dev/null | wc -l)"
  echo "[azurelocal] staged ${rpm_count} RPM(s) for ${version}"

  # Build local repo metadata so the node can install the selected version
  # offline at bring-up. Optional: skip gracefully if createrepo is absent.
  if command -v createrepo >/dev/null 2>&1; then
    createrepo "${target_dir}" >/dev/null 2>&1 \
      || echo "[azurelocal] WARNING: createrepo failed for ${target_dir} (POC)"
  else
    echo "[azurelocal] NOTE: createrepo not present; node will generate repo metadata at bring-up"
  fi
done

echo "[azurelocal] staged ${#versions[@]} k8s version(s): ${versions[*]}"
