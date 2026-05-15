#!/bin/bash
#
# dmverity-keyring-load.sh
#
# Loads the OS Guard signer CA into the kernel keyring so dm-verity can
# cryptographically verify the PKCS#7 layer-root-hash signatures that
# `notation sign --dm-verity` attaches to container images. Invoked by
# aks-dmverity-keyring.service at boot, before containerd.service starts.
#
# Reads:  /etc/aks/dmverity/osguard-signer-ca.pem  (PEM-encoded X.509)
#
# Tries to add the cert to (in priority order):
#   1. .machine    — preferred secondary trusted keyring; consulted by the
#                     kernel for dm-verity / kernel module signature checks
#                     on kernels built with CONFIG_INTEGRITY_MACHINE_KEYRING.
#   2. ._evm        — fallback for kernels exposing the EVM keyring.
#   3. @s           — session keyring; works on any kernel but only useful
#                     for userspace verification, NOT for dm-verity-on-mount.
#
# Exits 0 if the cert was added to at least one keyring, 1 otherwise.

set -euo pipefail

readonly CERT=/etc/aks/dmverity/osguard-signer-ca.pem
readonly LOG_TAG='dmverity-keyring'

log() { logger -t "${LOG_TAG}" "$*"; echo "[${LOG_TAG}] $*"; }
err() { logger -t "${LOG_TAG}" -p user.err "$*"; echo "[${LOG_TAG}] ERROR: $*" >&2; }

if [[ ! -f "${CERT}" ]]; then
    log "no CA at ${CERT}; nothing to load (prototype not enabled)"
    exit 0
fi

if ! command -v keyctl >/dev/null 2>&1; then
    err "keyctl not installed; cannot load CA into kernel keyring"
    exit 1
fi

# keyctl padd asymmetric expects DER-encoded X.509 on stdin. Convert PEM->DER
# in-memory so we don't have to ship two files.
DER_TMP=$(mktemp)
trap 'rm -f "${DER_TMP}"' EXIT
if ! openssl x509 -in "${CERT}" -outform DER -out "${DER_TMP}" 2>/dev/null; then
    err "failed to convert ${CERT} from PEM to DER"
    exit 1
fi

# Each entry: human label, keyctl target spec.
TARGETS=(
    'machine:%keyring:.machine'
    'evm:%keyring:._evm'
    'session:@s'
)

for entry in "${TARGETS[@]}"; do
    label="${entry%%:*}"
    target="${entry#*:}"
    if key_id=$(keyctl padd asymmetric '' "${target}" < "${DER_TMP}" 2>/dev/null); then
        log "loaded ${CERT} into ${label} keyring (id ${key_id})"
        exit 0
    fi
done

err "failed to add ${CERT} to any keyring (tried: machine, evm, session)"
err "kernel may have been built without CONFIG_INTEGRITY_MACHINE_KEYRING"
err "or CONFIG_SECONDARY_TRUSTED_KEYRING - dm-verity sig verification will fail"
exit 1
