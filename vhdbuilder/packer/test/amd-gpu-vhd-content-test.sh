#!/bin/bash
# Sourced by linux-vhd-content-test.sh only for the dedicated AMD_GPU image.

testAMDGPUImage() {
  local status=0
  if [ ! -x /opt/azure/containers/amd-gpu-validate.sh ]; then
    err testAMDGPUImage "Baked AMD GPU bootstrap validator is missing or not executable"
    status=1
  fi
  testAMDGPUDriver || status=1
  testAMDGPUDiagnostics || status=1
  return "${status}"
}

# Hardware-independent validation: the AMD VHD is baked and content-tested on a
# CPU VM. Loading the module and checking KFD/GPU devices belongs to node CSE/E2E.
# shellcheck disable=SC2016
testAMDGPUDriver() {
  [ "${FEATURE_FLAGS:-}" = "AMD_GPU" ] || return 0
  local components_file="${AMD_COMPONENTS_FILEPATH:-/opt/azure/amd-gpu/components.json}"
  local test=testAMDGPUDriver marker=/opt/azure/amd-gpu/driver.json
  local metadata kernel_version module_path module_vermagic package_version firmware_version module_version dkms_version
  local installed_packages modprobe_config package diagnostics_amdsmi diagnostics_sysdeps
  if [ "${OS_SKU}" != "Ubuntu" ] || [ "${OS_VERSION}" != "24.04" ] ||
     [ "${ENABLE_FIPS,,}" = "true" ] || [ "$(uname -m)" != "x86_64" ]; then
    err "${test}" "Unsupported AMD GPU image configuration"
    return 1
  fi
  if ! metadata=$(jq -ce '.AMDGPUDriver | select(type == "object")' "${components_file}") ||
     ! package_version=$(jq -er '.packageVersion | strings | select(length > 0)' <<< "${metadata}") ||
     ! firmware_version=$(jq -er '.firmwarePackageVersion | strings | select(length > 0)' <<< "${metadata}") ||
     ! module_version=$(jq -er '.moduleVersion | strings | select(length > 0)' <<< "${metadata}") ||
     ! dkms_version=$(jq -er '.dkmsVersion | strings | select(length > 0)' <<< "${metadata}"); then
    err "${test}" "Missing AMDGPU component metadata"
    return 1
  fi
  if ! jq -e --arg package "${package_version}" --arg firmware "${firmware_version}" --arg module_version "${module_version}" \
    '.schema_version == 1 and .package_version == $package and .firmware_package_version == $firmware and
      .module_version == $module_version and (.kernel_version | type == "string" and length > 0)' "${marker}" >/dev/null; then
    err "${test}" "Missing or inconsistent AMDGPU driver marker"
    return 1
  fi
  if [ "$(dpkg-query -W -f='${Status} ${Version}' amdgpu-dkms)" != "install ok installed ${package_version}" ] ||
     [ "$(dpkg-query -W -f='${Status} ${Version}' amdgpu-dkms-firmware)" != "install ok installed ${firmware_version}" ]; then
    err "${test}" "Installed AMDGPU driver/firmware differs from pinned versions"
    return 1
  fi
  kernel_version=$(uname -r)
  if ! dkms status -m amdgpu -v "${dkms_version}" -k "${kernel_version}" |
    grep -Fqx "amdgpu/${dkms_version}, ${kernel_version}, x86_64: installed"; then
    err "${test}" "AMDGPU DKMS is not installed for the running kernel"
    return 1
  fi
  module_path=$(modinfo -k "${kernel_version}" -F filename amdgpu)
  case "${module_path}" in
    /lib/modules/"${kernel_version}"/updates/dkms/amdgpu.ko*|/usr/lib/modules/"${kernel_version}"/updates/dkms/amdgpu.ko*) ;;
    *) err "${test}" "AMDGPU resolves to the inbox or a missing module"; return 1 ;;
  esac
  module_vermagic=$(modinfo -k "${kernel_version}" -F vermagic amdgpu)
  if [ "$(modinfo -k "${kernel_version}" -F version amdgpu)" != "${module_version}" ] ||
     [ "${module_vermagic%% *}" != "${kernel_version}" ]; then
    err "${test}" "AMDGPU module version or vermagic mismatch"
    return 1
  fi
  if [ ! -f "/lib/modules/${kernel_version}/build/Makefile" ] || [ ! -f "/usr/src/amdgpu-${dkms_version}/dkms.conf" ]; then
    err "${test}" "Headers or AMDGPU sources required by future DKMS rebuilds are missing"
    return 1
  fi
  for package in build-essential dkms autoconf automake initramfs-tools "linux-headers-${kernel_version}" "linux-modules-extra-${kernel_version}"; do
    if [ "$(dpkg-query -W -f='${Status}' "${package}")" != 'install ok installed' ]; then
      err "${test}" "Missing retained build dependency ${package}"
      return 1
    fi
  done
  if ! installed_packages=$(dpkg-query -W -f='${db:Status-Abbrev} ${binary:Package}\n'); then
    err "${test}" "Unable to inspect installed packages"
    return 1
  fi
  if ! diagnostics_amdsmi=$(jq -er '.AMDGPUDiagnostics.amdsmiPackage | strings | select(length > 0)' "${components_file}") ||
     ! diagnostics_sysdeps=$(jq -er '.AMDGPUDiagnostics.sysdepsPackage | strings | select(length > 0)' "${components_file}"); then
    err "${test}" "Missing AMDGPU diagnostics package allowlist"
    return 1
  fi
  if awk -v amdsmi="${diagnostics_amdsmi}" -v sysdeps="${diagnostics_sysdeps}" \
    '$1 == "ii" { sub(/:.*/, "", $2); if ($2 != amdsmi && $2 != sysdeps) print $2 }' <<< "${installed_packages}" |
    grep -Eq '^(amdrocm|rocm|hip|hsa-rocr|rocblas|rocfft|rocrand|rocsolver|rocsparse|miopen|migraphx|amdgpu-(core|lib|install|pro)|lib.*-amdgpu-|nvidia-|libnvidia-|cuda-|datacenter-gpu-manager-|dcgm-exporter)'; then
    err "${test}" "Unexpected GPU userspace or NVIDIA package on the AMD driver-and-diagnostics image"
    return 1
  fi
  if ! modprobe_config=$(modprobe -c) ||
    grep -Eq '^[[:space:]]*(blacklist|install)[[:space:]]+amdgpu([[:space:]]|$)' <<< "${modprobe_config}"; then
    err "${test}" "AMDGPU is disabled in modprobe configuration"
    return 1
  fi
  echo "${test}: pinned driver and firmware, DKMS module and rebuild dependencies verified"
}

# AMD SMI initializes GPU drivers even for --help. CPU bake validation checks
# contents and loads the library without amdsmi_init; device discovery runs in GPU E2E.
# shellcheck disable=SC2016
testAMDGPUDiagnostics() {
  [ "${FEATURE_FLAGS:-}" = "AMD_GPU" ] || return 0
  local components_file="${AMD_COMPONENTS_FILEPATH:-/opt/azure/amd-gpu/components.json}"
  local test=testAMDGPUDiagnostics metadata amdsmi_package amdsmi_version sysdeps_package sysdeps_version cli_path library_output
  local package tool tool_output
  if ! metadata=$(jq -ce '.AMDGPUDiagnostics | select(type == "object")' "${components_file}") ||
     ! amdsmi_package=$(jq -er '.amdsmiPackage | strings | select(length > 0)' <<< "${metadata}") ||
     ! amdsmi_version=$(jq -er '.amdsmiVersion | strings | select(length > 0)' <<< "${metadata}") ||
     ! sysdeps_package=$(jq -er '.sysdepsPackage | strings | select(length > 0)' <<< "${metadata}") ||
     ! sysdeps_version=$(jq -er '.sysdepsVersion | strings | select(length > 0)' <<< "${metadata}") ||
     ! cli_path=$(jq -er '.cliPath | strings | select(startswith("/opt/rocm/"))' <<< "${metadata}"); then
    err "${test}" "Missing AMDGPU diagnostics component metadata"
    return 1
  fi
  if [ "$(dpkg-query -W -f='${Status} ${Version}' "${amdsmi_package}")" != "install ok installed ${amdsmi_version}" ] ||
     [ "$(dpkg-query -W -f='${Status} ${Version}' "${sysdeps_package}")" != "install ok installed ${sysdeps_version}" ]; then
    err "${test}" "Installed AMD SMI diagnostics differ from pinned packages"
    return 1
  fi
  for package in pciutils numactl; do
    if [ "$(dpkg-query -W -f='${Status}' "${package}")" != 'install ok installed' ]; then
      err "${test}" "Missing host diagnostics package ${package}"
      return 1
    fi
  done
  for tool in lspci numactl; do
    if ! tool_output=$("${tool}" --version 2>&1); then
      err "${test}" "Host diagnostics command ${tool} failed: ${tool_output}"
      return 1
    fi
  done
  if [ ! -x "${cli_path}" ] || [ ! -L /usr/local/bin/amd-smi ] ||
     [ "$(readlink -f /usr/local/bin/amd-smi)" != "$(readlink -f "${cli_path}")" ]; then
    err "${test}" "AMD SMI executable or command symlink is missing or inconsistent"
    return 1
  fi
  if ! library_output=$(/usr/bin/python3 -I - "${cli_path}" <<'PY'
import json
from pathlib import Path
import sys
# The vendor bin/amd-smi symlink points into libexec; preserve its lexical bin path.
sys.path.insert(0, str(Path(sys.argv[1]).parents[1] / "share/amd_smi"))
import amdsmi
version = amdsmi.amdsmi_get_lib_version()
assert version["major"] > 0, version
print(json.dumps({"python_module": amdsmi.__version__, "library": version}))
PY
  ); then
    err "${test}" "AMD SMI library failed to load without GPU hardware: ${library_output}"
    return 1
  fi
  echo "${test}: pinned AMD SMI packages, command symlink, PCI/NUMA tools and hardware-independent library loading verified"
}
