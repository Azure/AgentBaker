#!/bin/bash
set -euo pipefail

# The AzL3 guest is built separately. Validate after installing the host RPMs.
if [[ "${IMG_OFFER:-}" != "azure-linux-3" || "${FEATURE_FLAGS:-}" != *kata* ]]; then
  exit 0
fi

version_file=${1:-/home/packer/kata-containers-version.txt}
if [[ ! -s "$version_file" ]]; then
  echo "Missing Kata UVM version file: $version_file. Rebuild pipeline 417429 with version metadata." >&2
  exit 1
fi
guest_version=$(cat "$version_file")
if [[ ! "$guest_version" =~ ^[a-zA-Z0-9._+~^-]+$ ]]; then
  echo "Invalid Kata UVM version in $version_file" >&2
  exit 1
fi
if ! host_version=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}\n' kata-containers); then
  echo "Cannot determine installed kata-containers host RPM version" >&2
  exit 1
fi

echo "Kata host RPM: $host_version; UVM tools RPM: $guest_version"
if [[ "$host_version" != "$guest_version" ]]; then
  echo "Kata host/UVM version mismatch: host=$host_version, guest=$guest_version. Rebuild the UVM and VHD with matching Kata packages." >&2
  echo "Run the Kata UVM build pipeline: https://dev.azure.com/msazure/CloudNativeCompute/_build?definitionId=417429 . After it succeeds with matching Kata packages, rerun this VHD build." >&2
  exit 1
fi
