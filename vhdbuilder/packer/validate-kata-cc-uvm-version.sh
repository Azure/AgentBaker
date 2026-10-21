#!/bin/bash
set -euo pipefail

# The kata-cc UVM is built from kata-containers-cc-tools; it must match the host kata-containers-cc RPM.
case ",${FEATURE_FLAGS:-}," in
  *,kata-cc,*) ;;
  *) exit 0 ;;
esac

version_file=${1:-/home/packer/kata-cc/kata-containers-cc-version.txt}
if [ ! -s "$version_file" ]; then
  echo "Missing Kata-CC UVM version file: $version_file" >&2
  exit 1
fi
guest_version=$(cat "$version_file")
case "$guest_version" in
  ''|*[!a-zA-Z0-9._+~^-]*)
    echo "Invalid Kata-CC UVM version in $version_file" >&2
    exit 1
    ;;
esac
if ! host_version=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}\n' kata-containers-cc); then
  echo "Cannot determine installed kata-containers-cc host RPM version" >&2
  exit 1
fi

echo "Kata-CC host RPM: $host_version; UVM tools RPM: $guest_version"
if [ "$host_version" != "$guest_version" ]; then
  echo "Kata-CC host/UVM version mismatch: host=$host_version, guest=$guest_version. Rebuild the UVM and VHD with matching kata-containers-cc packages." >&2
  exit 1
fi
