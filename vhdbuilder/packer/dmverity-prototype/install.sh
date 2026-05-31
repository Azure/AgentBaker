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
# enabled (DMVERITY_RPM_URL in dmverity-prototype.env is empty), so it is
# safe to leave wired into install-dependencies.sh on every build.
#
# Source layout (this dir):
#   dmverity-prototype.env       URLs + sha256 of patched RPMs and CA cert
#   dmverity-keyring-load.sh     boot-time keyctl loader (fallback for
#                                non-osguard kernels)
#   dmverity-keyring.service     systemd unit for ^
#
# What the RPMs themselves land on the VHD when enabled (no out-of-band
# wiring from this script):
#   patched containerd2 RPM (>=2.0.1-3001):
#     /usr/bin/containerd
#     /etc/containerd/config.toml             (erofs snapshotter + dm-verity)
#     /etc/modules-load.d/aks-dmverity.conf   (auto-load erofs + dm_verity)
#     Requires: erofs-utils, veritysetup      (pulled from base repo)
#   patched kernel RPM (6.6.139.1-99.osguard1, optional):
#     /boot/vmlinuz-6.6.139.1-99.osguard1.azl3 with OS Guard CA baked into
#     CONFIG_SYSTEM_TRUSTED_KEYS -> .builtin_trusted_keys on boot. Lets
#     dm-verity verify layer-root-hash PKCS#7 sigs natively without any
#     userspace keyring enrollment.
#
# What this script still lands directly (defense in depth):
#   /etc/aks/dmverity/osguard-signer-ca.pem            (PEM trust anchor;
#                                                       used by userspace
#                                                       verifiers and the
#                                                       keyring fallback)
#   /opt/azure/containers/dmverity-keyring-load.sh     (no-op on osguard
#                                                       kernels; loads CA
#                                                       into .machine on
#                                                       stock kernels)
#   /etc/systemd/system/aks-dmverity-keyring.service   (enabled)
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

if [[ -z "${DMVERITY_RPM_URL:-}" ]]; then
    log "DMVERITY_RPM_URL empty; skipping (stock build)"
    exit 0
fi

# AzureLinux/Mariner only — the patched RPM is .azl3.x86_64.rpm.
if ! command -v tdnf >/dev/null 2>&1; then
    err "tdnf not found; prototype only supports AzureLinux 3 VHD builds"
    exit 1
fi

log "prototype mode enabled on AzureLinux build"

TMP_KRPM=$(mktemp --suffix=.rpm)
TMP_RPM=$(mktemp --suffix=.rpm)
TMP_CERT=$(mktemp --suffix=.pem)
trap 'rm -f "${TMP_KRPM}" "${TMP_RPM}" "${TMP_CERT}"' EXIT

# ----- Install patched kernel RPM (optional, normally delegated) --------
# Bakes the OS Guard CA into .builtin_trusted_keys at compile time so
# dm-verity layer-sig verification works on boot without keyctl enrollment.
#
# *** NORMAL OPERATION: DMVERITY_KERNEL_RPM_URL is empty. ***
# The patched kernel RPM is delivered to the Packer build VM by the
# mariner-aks-pipelines buddy-build channel (`proprietaryRpmBuddyBuildId`
# parameter on the buildAzureLinuxV3gen2 job, set in the ADO UI when the
# build is queued). The pipeline downloads the buddy-build artifact and
# tdnf-installs it by file path *multiple times* during standard
# install-dependencies.sh execution -- BEFORE this install.sh ever runs.
# Re-downloading from kataccstorage here is therefore redundant AND was
# observed to fail mid-build on build 1129685 when the kataccstorage
# container public-access flag flipped off, returning HTTP 409. See
# dmverity-prototype.env for the design write-up.
#
# *** OPT-IN URL PATH (rare): when DMVERITY_KERNEL_RPM_URL is set, we
# install the kernel BY FULL FILE PATH so tdnf does NOT resolve from any
# repo: the spec uses Release: 99.osguard1%{?dist} which outranks any
# realistic upstream patch release at the same Version (1, 2, ..., 9),
# but a hypothetical upstream Version bump (e.g. 6.6.140.1) would
# silently win via repo resolution -- file-path install makes that
# impossible.
#
# We --disablerepo='preview-repo' (NOT '*') so dependency resolution can
# still reach azurelinux-official-base for any new transitive deps the
# kernel needs; preview-repo SAS has been observed to return transient
# 403s mid-build which would abort an otherwise-fine install.
#
# The packer build VM itself stays on the stock kernel for the rest of
# the build (no mid-build reboot); the captured VHD's bootloader picks
# the newest installed kernel on first boot.
if [[ -n "${DMVERITY_KERNEL_RPM_URL:-}" ]]; then
    log "downloading patched kernel RPM from ${DMVERITY_KERNEL_RPM_URL}"
    if ! curl -fsSL --retry 5 --retry-delay 3 -o "${TMP_KRPM}" "${DMVERITY_KERNEL_RPM_URL}"; then
        err "failed to download patched kernel RPM"
        exit 1
    fi
    if [[ -n "${DMVERITY_KERNEL_RPM_SHA256:-}" ]]; then
        actual=$(sha256sum "${TMP_KRPM}" | awk '{print $1}')
        if [[ "${actual}" != "${DMVERITY_KERNEL_RPM_SHA256}" ]]; then
            err "kernel RPM sha256 mismatch: expected ${DMVERITY_KERNEL_RPM_SHA256}, got ${actual}"
            exit 1
        fi
        log "kernel RPM sha256 verified: ${actual}"
    fi
    log "installing patched kernel RPM by file path"
    tdnf install -y --nogpgcheck --disablerepo='preview-repo' "${TMP_KRPM}"
    log "patched kernel staged; will boot on next reboot (VHD capture)"
else
    log "DMVERITY_KERNEL_RPM_URL empty; patched kernel install delegated to buddy-build channel (proprietaryRpmBuddyBuildId)"
fi

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
# present from installStandaloneContainerd). We pass --disablerepo='preview-repo'
# (NOT '*') so dependency resolution can still pull in the containerd RPM's
# Requires: erofs-utils + veritysetup from azurelinux-official-base. The
# preview-repo SAS has been observed to 403 mid-build which would abort the
# install with "Failed to synchronize cache for repo".
log "installing patched containerd RPM by file path"
tdnf install -y --nogpgcheck --disablerepo='preview-repo' "${TMP_RPM}"
log "  mkfs.erofs:   $(mkfs.erofs --version 2>&1 | head -1 || echo 'MISSING')"
log "  veritysetup:  $(veritysetup --version 2>&1 | head -1 || echo 'MISSING')"

INSTALLED_VER=$(containerd --version 2>/dev/null || echo 'unknown')
log "patched containerd installed: ${INSTALLED_VER}"

# Modprobe for the current build VM so the snapshotter can mount layers
# during the rest of install-dependencies.sh (image pre-pull loop). The
# containerd RPM ships /etc/modules-load.d/aks-dmverity.conf which makes
# systemd-modules-load.service handle this on every subsequent boot of
# provisioned AKS nodes.
modprobe erofs     || { err "failed to load erofs kernel module";     exit 1; }
modprobe dm_verity || { err "failed to load dm_verity kernel module"; exit 1; }
log "erofs + dm_verity kernel modules loaded for build VM (RPM-pinned for subsequent boots)"

# ----- Download CA cert (PEM) --------------------------------------------
# Optional with the osguard kernel (CA is in .builtin_trusted_keys), but
# still useful for userspace verifiers (notation verify) and as a fallback
# for non-osguard kernels via aks-dmverity-keyring.service.
if [[ -n "${DMVERITY_CERT_URL:-}" ]]; then
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
else
    log "DMVERITY_CERT_URL empty; skipping CA install (kernel-baked CA only)"
fi

# ----- Stage keyring loader script + systemd unit ------------------------
# Kept as a defensive fallback for VHDs that boot a non-osguard kernel.
# The loader script short-circuits if the running kernel already has the
# OS Guard CA in .builtin_trusted_keys, so on osguard kernels this is a
# true no-op (single keyctl + grep at boot).
install -d -m 0755 /opt/azure/containers
install -m 0755 -o root -g root "${SCRIPT_DIR}/dmverity-keyring-load.sh" \
    /opt/azure/containers/dmverity-keyring-load.sh

install -m 0644 -o root -g root "${SCRIPT_DIR}/dmverity-keyring.service" \
    /etc/systemd/system/aks-dmverity-keyring.service

systemctl enable aks-dmverity-keyring.service

# Restart containerd so the patched binary picks up the RPM-shipped config
# BEFORE install-dependencies.sh kicks off its container-image pre-pull loop.
log "restarting containerd to apply patched binary + RPM-shipped erofs config"
systemctl daemon-reload
systemctl restart containerd
sleep 2
systemctl is-active --quiet containerd || { err "containerd failed to start"; journalctl -u containerd --no-pager -n 50; exit 1; }
log "containerd is active"

log "dmverity prototype installation complete"
log "  - kernel:            $(uname -r) (build VM); installed for next boot: ${DMVERITY_KERNEL_RPM_URL:-<delegated to buddy-build channel>}"
log "  - containerd:        ${INSTALLED_VER}"
log "  - config:            /etc/containerd/config.toml (shipped by RPM)"
log "  - modules-load:      /etc/modules-load.d/aks-dmverity.conf (shipped by RPM)"
log "  - CA cert:           /etc/aks/dmverity/osguard-signer-ca.pem"
log "  - keyring loader:    /opt/azure/containers/dmverity-keyring-load.sh (boot-enabled, no-op on osguard kernel)"
