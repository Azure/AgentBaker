#!/bin/bash
set -euo pipefail

# Normalize the current signing job's artifact layout without taking an unsigned
# fallback. The CC-only Packer template stages this directory as a unit.
source_dir=${1:?signed artifact directory is required}
destination=${2:-kata-cc-payload}
mkdir -p "$destination"
for name in kata-containers.img kata-containers-igvm.img igvm-measurement.cose reference-info-base64; do
    mapfile -t matches < <(find "$source_dir" -type f -name "$name")
    if [ "${#matches[@]}" -ne 1 ] || [ ! -s "${matches[0]}" ]; then
        echo "Expected one nonempty signed payload file: $name" >&2
        exit 1
    fi
    install -m 0644 "${matches[0]}" "$destination/$name"
done
for name in kata-containers-igvm-debug.img igvm-debug-measurement.cose; do
    mapfile -t matches < <(find "$source_dir" -type f -name "$name")
    if [ "${#matches[@]}" -gt 1 ]; then
        echo "Ambiguous signed payload file: $name" >&2
        exit 1
    fi
    if [ "${#matches[@]}" -eq 1 ]; then
        install -m 0644 "${matches[0]}" "$destination/$name"
    fi
done
# The shared template also uploads this filename for ordinary Kata.
install -m 0644 "$destination/kata-containers.img" kata-containers.img
printf '%s\n' "${KATA_CC_SIGNING_BUILD_ID:?signing build ID is required}" > "$destination/signing-build-id"
