#!/bin/bash
set -euo pipefail

# These are build-time defaults for ordinary Azure Linux 3 Kata, not Kata-CC.
case "${FEATURE_FLAGS:-}" in
  *kata-cc*) exit 0 ;;
  *kata*) ;;
  *) exit 0 ;;
esac
if [ "${OS:-}" != "AZURELINUX" ]; then
  exit 0
fi

kernel_file=${1:-/home/packer/kata-uvm-kernel-version.txt}
notes_file=${2:-/opt/azure/vhd-install.complete}
if [ ! -s "$kernel_file" ]; then
  echo "Missing Kata UVM kernel metadata: $kernel_file" >&2
  exit 1
fi
uvm_kernel=$(cat "$kernel_file")
case "$uvm_kernel" in
  ''|*[!a-zA-Z0-9._+~^-]*)
    echo "Invalid Kata UVM kernel metadata" >&2
    exit 1
    ;;
esac
host_kernel=$(uname -r)
runtime=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}\n' kata-containers)
{
  echo '=== Kata Default Versions Begin'
  echo 'Applies only to the Azure Linux 3 Gen2 Kata VHD (not standard Azure Linux or Kata-CC). These are the defaults shipped with this VHD.'
  echo "Kata host kernel: $host_kernel"
  echo "Kata UVM kernel: $uvm_kernel"
  echo "Kata runtime: $runtime"
  echo '=== Kata Default Versions End'
} >> "$notes_file"
