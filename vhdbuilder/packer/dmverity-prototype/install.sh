#!/bin/bash
#
# install.sh
#
# dm-verity prototype installer for AzureLinux 3 VHD builds. Runs inside
# the AgentBaker Packer build VM (NOT a chroot) right after
# installStandaloneContainerd, so that the patched containerd handles all
# subsequent image pulls during VHD build (the build itself becomes the
# end-to-end test of the notary `--dm-verity` referrer flow).
#
# This script is idempotent and is a hard no-op when the prototype is not
# enabled (URLs in dmverity-prototype.env are empty), so it is safe to
# leave wired into install-dependencies.sh on every build.
#
# Source layout (this dir):
#   dmverity-prototype.env       URLs + sha256 of patched containerd RPM and CA
#   dmverity-keyring-load.sh     boot-time keyctl loader
#   dmverity-keyring.service     systemd unit for ^
#   containerd-dmverity.toml     full /etc/containerd/config.toml replacement
#                                (RPM ships no config; built-in defaults use
#                                overlayfs which would skip the erofs differ)
#
# What it lands on the VHD when enabled:
#   patched containerd RPM        installed via tdnf -> /usr/bin/containerd
#   /etc/aks/dmverity/osguard-signer-ca.pem   (PKCS#7 trust anchor, PEM)
#   /etc/containerd/config.toml               (erofs snapshotter + dm-verity)
#   /opt/azure/containers/dmverity-keyring-load.sh
#   /etc/systemd/system/aks-dmverity-keyring.service  (enabled)
#
# See README.md in this directory for the full prototype workflow.

set -euo pipefail

# Resolve our script directory so we can find the sibling files
# regardless of how install-dependencies.sh invokes us.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
readonly ENV_FILE="${SCRIPT_DIR}/dmverity-prototype.env"
readonly LOG_TAG='dmverity-prototype'

log() { echo "[${LOG_TAG}] $*"; }
err() { echo "[${LOG_TAG}] ERROR: $*" >&2; }

if [[ ! -f "${ENV_FILE}" ]]; then
    log "no env file at ${ENV_FILE}; skipping (stock build)"
    exit 0
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"

if [[ -z "${DMVERITY_RPM_URL:-}" || -z "${DMVERITY_CERT_URL:-}" ]]; then
    log "DMVERITY_RPM_URL/DMVERITY_CERT_URL empty; skipping (stock build)"
    exit 0
fi

# AzureLinux/Mariner only — the patched RPM is .azl3.x86_64.rpm.
if ! command -v tdnf >/dev/null 2>&1; then
    err "tdnf not found; prototype only supports AzureLinux 3 VHD builds"
    exit 1
fi

log "prototype mode enabled on AzureLinux build"

TMP_RPM=$(mktemp --suffix=.rpm)
TMP_CERT=$(mktemp --suffix=.pem)
trap 'rm -f "${TMP_RPM}" "${TMP_CERT}"' EXIT

# ----- Download patched containerd RPM -----------------------------------
log "downloading patched containerd RPM from ${DMVERITY_RPM_URL}"
if ! curl -fsSL --retry 5 --retry-delay 3 -o "${TMP_RPM}" "${DMVERITY_RPM_URL}"; then
    err "failed to download patched containerd RPM"
    exit 1
fi

if [[ -n "${DMVERITY_RPM_SHA256:-}" ]]; then
    actual=$(sha256sum "${TMP_RPM}" | awk '{print $1}')
    if [[ "${actual}" != "${DMVERITY_RPM_SHA256}" ]]; then
        err "RPM sha256 mismatch: expected ${DMVERITY_RPM_SHA256}, got ${actual}"
        exit 1
    fi
    log "RPM sha256 verified: ${actual}"
fi

# Force-install (replaces stock containerd2 if a different build is already
# present from installStandaloneContainerd). We pass --disablerepo='*' so
# tdnf does NOT refresh any configured repo metadata as a side-effect of
# installing this local RPM. This prototype install runs mid-build on a VM
# that has the build's Preview Repo SAS configured, and that SAS has been
# observed to return transient 403s; a refresh failure there would abort
# the prototype install with `Error: Failed to synchronize cache for repo`.
# Dependency resolution still works because tdnf consults the installed
# rpmdb before reaching for repos, and the patched RPM's deps were just
# satisfied by the stock containerd2 install above.
log "installing patched containerd RPM"
tdnf install -y --nogpgcheck --disablerepo='*' "${TMP_RPM}"

INSTALLED_VER=$(containerd --version 2>/dev/null || echo 'unknown')
log "patched containerd installed: ${INSTALLED_VER}"

# ----- Download CA cert (PEM) --------------------------------------------
log "downloading CA cert from ${DMVERITY_CERT_URL}"
if ! curl -fsSL --retry 5 --retry-delay 3 -o "${TMP_CERT}" "${DMVERITY_CERT_URL}"; then
    err "failed to download CA cert"
    exit 1
fi

if [[ -n "${DMVERITY_CERT_SHA256:-}" ]]; then
    actual=$(sha256sum "${TMP_CERT}" | awk '{print $1}')
    if [[ "${actual}" != "${DMVERITY_CERT_SHA256}" ]]; then
        err "cert sha256 mismatch: expected ${DMVERITY_CERT_SHA256}, got ${actual}"
        exit 1
    fi
    log "cert sha256 verified: ${actual}"
fi

# Sanity-check PEM. keyctl padd asymmetric accepts PEM directly via stdin.
if ! head -c 32 "${TMP_CERT}" | grep -q -- '-----BEGIN CERTIFICATE-----'; then
    err "downloaded cert is not PEM-encoded"
    exit 1
fi

install -d -m 0755 /etc/aks/dmverity
install -m 0644 -o root -g root "${TMP_CERT}" /etc/aks/dmverity/osguard-signer-ca.pem
log "installed CA at /etc/aks/dmverity/osguard-signer-ca.pem"

# ----- Stage scripts, systemd unit, containerd config --------------------
install -d -m 0755 /opt/azure/containers
install -m 0755 -o root -g root "${SCRIPT_DIR}/dmverity-keyring-load.sh" \
    /opt/azure/containers/dmverity-keyring-load.sh

install -m 0644 -o root -g root "${SCRIPT_DIR}/dmverity-keyring.service" \
    /etc/systemd/system/aks-dmverity-keyring.service

install -d -m 0755 /etc/containerd
install -m 0644 -o root -g root "${SCRIPT_DIR}/containerd-dmverity.toml" \
    /etc/containerd/config.toml
log "installed containerd config at /etc/containerd/config.toml"

systemctl enable aks-dmverity-keyring.service

# Restart containerd so the patched binary picks up the new config BEFORE
# install-dependencies.sh kicks off its container-image pre-pull loop.
log "restarting containerd to apply patched binary + erofs config"
systemctl daemon-reload
systemctl restart containerd
sleep 2
systemctl is-active --quiet containerd || { err "containerd failed to start"; journalctl -u containerd --no-pager -n 50; exit 1; }
log "containerd is active"

log "dmverity prototype installation complete"
log "  - containerd:        ${INSTALLED_VER}"
log "  - config:            /etc/containerd/config.toml"
log "  - CA cert:           /etc/aks/dmverity/osguard-signer-ca.pem"
log "  - keyring loader:    /opt/azure/containers/dmverity-keyring-load.sh (boot-enabled)"
