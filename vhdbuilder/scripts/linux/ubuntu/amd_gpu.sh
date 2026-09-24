#!/bin/bash
# Dedicated AMD_GPU Ubuntu VHD support. Sourced only by the AMD image build.

validateAMDGPUImageConfiguration() {
  local generation=${HYPERV_GENERATION:-} enable_fips=${ENABLE_FIPS:-false}
  if [ "${FEATURE_FLAGS:-}" != "AMD_GPU" ] || [ "${OS:-}" != "UBUNTU" ] ||
     [ "${OS_VERSION:-}" != "24.04" ] || [ "${CPU_ARCH:-}" != "amd64" ] ||
     [ "${generation,,}" != "v2" ] || [ "${enable_fips,,}" = "true" ]; then
    echo "AMD_GPU requires the dedicated Ubuntu 24.04 amd64 Gen2 non-FIPS image" >&2
    return 1
  fi
}

isAMDGPUSkippedPackage() {
  case "$1" in
    nvidia-*|dra-driver-nvidia-*|datacenter-gpu-manager-*|dcgm-exporter) return 0 ;;
    *) return 1 ;;
  esac
}

installAMDGPUImage() {
  local components_file="${AMD_COMPONENTS_FILEPATH:-/opt/azure/amd-gpu/components.json}"
  validateAMDGPUImageConfiguration || return 1
  install -Dm0644 /home/packer/amd-gpu-components.json "${components_file}" || return 1
  installAMDGPUDriver || return 1
  capture_benchmark "${SCRIPT_NAME}_build_amd_gpu_kernel_module" || return 1
  installAMDGPUDiagnostics || return 1
  capture_benchmark "${SCRIPT_NAME}_install_amd_gpu_diagnostics" || return 1
  install -Dm0755 /home/packer/amd-gpu-validate.sh /opt/azure/containers/amd-gpu-validate.sh || return 1
}

# Bake on a CPU VM: keep DKMS and its build dependencies for kernel servicing,
# but never load the driver or install the ROCm compute SDK on the host.
# The subshell owns its cleanup trap without replacing the caller's traps.
installAMDGPUDriver() {
  (
    set -o pipefail
    local components_file="${AMD_COMPONENTS_FILEPATH:-/opt/azure/amd-gpu/components.json}"
    local metadata repository key_url fingerprint package_version firmware_version module_version dkms_version
    local kernel_version work_dir marker_dir=/opt/azure/amd-gpu marker_tmp=""
    local generation=${HYPERV_GENERATION:-} enable_fips=${ENABLE_FIPS:-false}
    local actual_fingerprint archive package_name package_arch module_path module_vermagic modprobe_config conf
    local driver_deb="" firmware_deb=""
    local -a apt_options build_packages
    if [ "${OS:-}" != "UBUNTU" ] || [ "${OS_VERSION:-}" != "24.04" ] ||
       [ "${CPU_ARCH:-}" != "amd64" ] || [ "${generation,,}" != "v2" ] ||
       [ "${enable_fips,,}" = "true" ]; then
      echo "AMDGPU requires Ubuntu 24.04 amd64 Gen2 without FIPS" >&2
      exit 1
    fi
    # A failed re-bake must not leave a success marker from an earlier attempt.
    rm -f "${marker_dir}/driver.json" || exit 1
    metadata=$(jq -ce '.AMDGPUDriver | select(type == "object")' "${components_file}") || exit 1
    repository=$(jq -er '.repositoryURL | select(test("^https://repo[.]radeon[.]com/amdgpu/[0-9.]+/ubuntu$"))' <<< "${metadata}") || exit 1
    key_url=$(jq -er '.signingKeyURL | select(test("^https://repo[.]radeon[.]com/[^?[:space:]]+$"))' <<< "${metadata}") || exit 1
    fingerprint=$(jq -er '.signingKeyFingerprint | select(test("^[A-F0-9]{40}$"))' <<< "${metadata}") || exit 1
    jq -e '.distribution == "noble" and .component == "main"' <<< "${metadata}" >/dev/null || exit 1
    package_version=$(jq -er '.packageVersion | strings | select(length > 0)' <<< "${metadata}") || exit 1
    firmware_version=$(jq -er '.firmwarePackageVersion | strings | select(length > 0)' <<< "${metadata}") || exit 1
    module_version=$(jq -er '.moduleVersion | strings | select(length > 0)' <<< "${metadata}") || exit 1
    dkms_version=$(jq -er '.dkmsVersion | strings | select(length > 0)' <<< "${metadata}") || exit 1
    kernel_version=$(uname -r) || exit 1
    work_dir=$(mktemp -d /tmp/amd-gpu.XXXXXX) || exit 1
    trap 'rm -rf "${work_dir}"; if [ -n "${marker_tmp}" ]; then rm -f "${marker_tmp}"; fi' EXIT
    build_packages=(ca-certificates curl gnupg build-essential dkms autoconf automake initramfs-tools
      "linux-headers-${kernel_version}" "linux-modules-extra-${kernel_version}")
    apt_get_update || exit 1
    apt_get_install 20 5 1800 "${build_packages[@]}" || exit 1
    test -f "/lib/modules/${kernel_version}/build/Makefile" || exit 1

    mkdir -p "${work_dir}/gnupg" "${work_dir}/lists/partial" "${work_dir}/packages" || exit 1
    chmod 755 "${work_dir}" || exit 1
    chmod 700 "${work_dir}/gnupg" || exit 1
    retrycmd_if_failure 5 5 60 curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' \
      "${key_url}" -o "${work_dir}/key.asc" || exit 1
    # Compare primary fingerprints only; the legitimate key also has an encryption subkey.
    actual_fingerprint=$(gpg --batch --homedir "${work_dir}/gnupg" --show-keys --with-colons "${work_dir}/key.asc" |
      awk -F: '$1 == "pub" { primary = 1; next } primary && $1 == "fpr" { print $10; primary = 0 }') || exit 1
    if [ "${actual_fingerprint}" != "${fingerprint}" ]; then
      echo "AMDGPU signing key fingerprint mismatch" >&2
      exit 1
    fi
    gpg --batch --homedir "${work_dir}/gnupg" --dearmor --output "${work_dir}/key.gpg" "${work_dir}/key.asc" || exit 1
    chmod 644 "${work_dir}/key.gpg" || exit 1
    printf 'deb [arch=amd64 signed-by=%s/key.gpg] %s noble main\n' "${work_dir}" "${repository}" > "${work_dir}/amdgpu.list" || exit 1
    # AMD's signed metadata is isolated from normal APT state. Only these two
    # exact packages come from AMD; Ubuntu supplies all remaining dependencies.
    apt_options=(-o "Dir::Etc::sourcelist=${work_dir}/amdgpu.list" -o Dir::Etc::sourceparts=-
      -o "Dir::State::lists=${work_dir}/lists" -o APT::Get::AllowUnauthenticated=false
      -o Acquire::AllowInsecureRepositories=false -o Acquire::AllowDowngradeToInsecureRepositories=false)
    retrycmd_if_failure 5 5 300 apt-get "${apt_options[@]}" update || exit 1
    cd "${work_dir}/packages" || exit 1
    retrycmd_if_failure 5 5 300 apt-get "${apt_options[@]}" download \
      "amdgpu-dkms=${package_version}" "amdgpu-dkms-firmware=${firmware_version}" || exit 1
    for archive in "${work_dir}/packages/"*.deb; do
      package_name=$(dpkg-deb -f "${archive}" Package) || exit 1
      package_arch=$(dpkg-deb -f "${archive}" Architecture) || exit 1
      case "${package_arch}" in all|amd64) ;; *) echo "Unexpected AMDGPU package architecture" >&2; exit 1 ;; esac
      case "${package_name}" in
        amdgpu-dkms)
          [ -z "${driver_deb}" ] && [ "$(dpkg-deb -f "${archive}" Version)" = "${package_version}" ] || exit 1
          driver_deb=${archive}
          ;;
        amdgpu-dkms-firmware)
          [ -z "${firmware_deb}" ] && [ "$(dpkg-deb -f "${archive}" Version)" = "${firmware_version}" ] || exit 1
          firmware_deb=${archive}
          ;;
        *) echo "Unexpected AMDGPU package: ${package_name}" >&2; exit 1 ;;
      esac
    done
    [ -n "${driver_deb}" ] && [ -n "${firmware_deb}" ] || exit 1
    apt_get_install 20 5 1800 "${driver_deb}" "${firmware_deb}" || exit 1
    # Keep the compiler, matching headers, firmware and DKMS sources through
    # image cleanup. No vendor repository/key is persisted on the image.
    apt-mark manual "${build_packages[@]}" amdgpu-dkms amdgpu-dkms-firmware || exit 1
    # shellcheck disable=SC2016
    [ "$(dpkg-query -W -f='${Status} ${Version}' amdgpu-dkms)" = "install ok installed ${package_version}" ] || exit 1
    # shellcheck disable=SC2016
    [ "$(dpkg-query -W -f='${Status} ${Version}' amdgpu-dkms-firmware)" = "install ok installed ${firmware_version}" ] || exit 1
    dkms install -m amdgpu -v "${dkms_version}" -k "${kernel_version}" || exit 1
    dkms status -m amdgpu -v "${dkms_version}" -k "${kernel_version}" | grep -Fqx "amdgpu/${dkms_version}, ${kernel_version}, x86_64: installed" || exit 1
    depmod -a "${kernel_version}" || exit 1
    module_path=$(modinfo -k "${kernel_version}" -F filename amdgpu) || exit 1
    case "${module_path}" in
      /lib/modules/"${kernel_version}"/updates/dkms/amdgpu.ko*|/usr/lib/modules/"${kernel_version}"/updates/dkms/amdgpu.ko*) ;;
      *) echo "AMDGPU resolves to an unexpected module: ${module_path}" >&2; exit 1 ;;
    esac
    [ "$(modinfo -k "${kernel_version}" -F version amdgpu)" = "${module_version}" ] || exit 1
    module_vermagic=$(modinfo -k "${kernel_version}" -F vermagic amdgpu) || exit 1
    [ "${module_vermagic%% *}" = "${kernel_version}" ] || exit 1

    # Azure's CPU base image may blacklist the inbox GPU driver. Remove only
    # the AMD entry, retaining unrelated cloud-image module deny rules.
    for conf in /etc/modprobe.d/*.conf /usr/lib/modprobe.d/*.conf; do
      [ -f "${conf}" ] || continue
      sed -i -E '/^[[:space:]]*blacklist[[:space:]]+amdgpu([[:space:]]|$)/d' "${conf}" || exit 1
    done
    modprobe_config=$(modprobe -c) || exit 1
    if grep -Eq '^[[:space:]]*(blacklist|install)[[:space:]]+amdgpu([[:space:]]|$)' <<< "${modprobe_config}"; then
      echo "AMDGPU remains disabled in modprobe configuration" >&2
      exit 1
    fi
    update-initramfs -u -k "${kernel_version}" || exit 1
    mkdir -p "${marker_dir}" || exit 1
    marker_tmp=$(mktemp "${marker_dir}/.driver.json.XXXXXX") || exit 1
    jq -n --arg package_version "${package_version}" --arg firmware_package_version "${firmware_version}" \
      --arg module_version "${module_version}" --arg kernel_version "${kernel_version}" \
      '{schema_version: 1, package_version: $package_version, firmware_package_version: $firmware_package_version,
        module_version: $module_version, kernel_version: $kernel_version}' > "${marker_tmp}" || exit 1
    chmod 644 "${marker_tmp}" || exit 1
    echo "  - amdgpu-dkms version ${package_version}; module ${module_version}; kernel ${kernel_version}" >> "${VHD_LOGS_FILEPATH}" || exit 1
    mv -f "${marker_tmp}" "${marker_dir}/driver.json" || exit 1
  )
}

# Host diagnostics are intentionally separate from the compute SDK. AMD SMI
# uses only its management library and sysdeps package; no pip installation.
installAMDGPUDiagnostics() {
  (
    set -o pipefail
    local components_file="${AMD_COMPONENTS_FILEPATH:-/opt/azure/amd-gpu/components.json}"
    local metadata repository key_url fingerprint actual_fingerprint work_dir
    local amdsmi_package amdsmi_version sysdeps_package sysdeps_version cli_path python_path
    local archive package_name package_arch amdsmi_deb="" sysdeps_deb=""
    local generation=${HYPERV_GENERATION:-} enable_fips=${ENABLE_FIPS:-false}
    local -a apt_options ubuntu_packages
    if [ "${OS:-}" != "UBUNTU" ] || [ "${OS_VERSION:-}" != "24.04" ] ||
       [ "${CPU_ARCH:-}" != "amd64" ] || [ "${generation,,}" != "v2" ] ||
       [ "${enable_fips,,}" = "true" ]; then
      echo "AMD GPU diagnostics require Ubuntu 24.04 amd64 Gen2 without FIPS" >&2
      exit 1
    fi
    metadata=$(jq -ce '.AMDGPUDiagnostics | select(type == "object")' "${components_file}") || exit 1
    repository=$(jq -er '.repositoryURL | select(. == "https://stable.repo.amd.com/rocm/core/packages/ubuntu2404/")' <<< "${metadata}") || exit 1
    key_url=$(jq -er '.signingKeyURL | select(. == "https://stable.repo.amd.com/rocm/gpg/packages.gpg")' <<< "${metadata}") || exit 1
    fingerprint=$(jq -er '.signingKeyFingerprint | select(test("^[A-F0-9]{40}$"))' <<< "${metadata}") || exit 1
    jq -e '.distribution == "stable" and .component == "main"' <<< "${metadata}" >/dev/null || exit 1
    amdsmi_package=$(jq -er '.amdsmiPackage | select(test("^amdrocm-amdsmi[0-9]+[.][0-9]+$"))' <<< "${metadata}") || exit 1
    sysdeps_package=$(jq -er '.sysdepsPackage | select(test("^amdrocm-sysdeps[0-9]+[.][0-9]+$"))' <<< "${metadata}") || exit 1
    amdsmi_version=$(jq -er '.amdsmiVersion | strings | select(length > 0)' <<< "${metadata}") || exit 1
    sysdeps_version=$(jq -er '.sysdepsVersion | strings | select(length > 0)' <<< "${metadata}") || exit 1
    cli_path=$(jq -er '.cliPath | select(test("^/opt/rocm/core-[0-9]+[.][0-9]+/bin/amd-smi$"))' <<< "${metadata}") || exit 1
    python_path="${cli_path%/bin/amd-smi}/share/amd_smi"
    work_dir=$(mktemp -d /tmp/amd-gpu-diagnostics.XXXXXX) || exit 1
    trap 'rm -rf "${work_dir}"' EXIT
    # The vendor package omits the C++ runtime dependency from its metadata.
    ubuntu_packages=(ca-certificates curl gnupg python3 libstdc++6 libgcc-s1 pciutils numactl)
    apt_get_update || exit 1
    apt_get_install 20 5 600 "${ubuntu_packages[@]}" || exit 1
    mkdir -p "${work_dir}/gnupg" "${work_dir}/lists/partial" "${work_dir}/packages" || exit 1
    chmod 755 "${work_dir}" || exit 1
    chmod 700 "${work_dir}/gnupg" || exit 1
    retrycmd_if_failure 5 5 60 curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' \
      "${key_url}" -o "${work_dir}/key.asc" || exit 1
    actual_fingerprint=$(gpg --batch --homedir "${work_dir}/gnupg" --show-keys --with-colons "${work_dir}/key.asc" |
      awk -F: '$1 == "pub" { primary = 1; next } primary && $1 == "fpr" { print $10; primary = 0 }') || exit 1
    if [ "${actual_fingerprint}" != "${fingerprint}" ]; then
      echo "AMD GPU diagnostics signing key fingerprint mismatch" >&2
      exit 1
    fi
    gpg --batch --homedir "${work_dir}/gnupg" --dearmor --output "${work_dir}/key.gpg" "${work_dir}/key.asc" || exit 1
    chmod 644 "${work_dir}/key.gpg" || exit 1
    printf 'deb [arch=amd64 signed-by=%s/key.gpg] %s stable main\n' "${work_dir}" "${repository}" > "${work_dir}/diagnostics.list" || exit 1
    apt_options=(-o "Dir::Etc::sourcelist=${work_dir}/diagnostics.list" -o Dir::Etc::sourceparts=-
      -o "Dir::State::lists=${work_dir}/lists" -o APT::Get::AllowUnauthenticated=false
      -o Acquire::AllowInsecureRepositories=false -o Acquire::AllowDowngradeToInsecureRepositories=false)
    retrycmd_if_failure 5 5 300 apt-get "${apt_options[@]}" update || exit 1
    cd "${work_dir}/packages" || exit 1
    retrycmd_if_failure 5 5 300 apt-get "${apt_options[@]}" download \
      "${amdsmi_package}=${amdsmi_version}" "${sysdeps_package}=${sysdeps_version}" || exit 1
    for archive in "${work_dir}/packages/"*.deb; do
      package_name=$(dpkg-deb -f "${archive}" Package) || exit 1
      package_arch=$(dpkg-deb -f "${archive}" Architecture) || exit 1
      [ "${package_arch}" = amd64 ] || { echo "Unexpected AMD diagnostics package architecture" >&2; exit 1; }
      case "${package_name}" in
        "${amdsmi_package}")
          [ -z "${amdsmi_deb}" ] && [ "$(dpkg-deb -f "${archive}" Version)" = "${amdsmi_version}" ] || exit 1
          amdsmi_deb=${archive}
          ;;
        "${sysdeps_package}")
          [ -z "${sysdeps_deb}" ] && [ "$(dpkg-deb -f "${archive}" Version)" = "${sysdeps_version}" ] || exit 1
          sysdeps_deb=${archive}
          ;;
        *) echo "Unexpected AMD diagnostics package: ${package_name}" >&2; exit 1 ;;
      esac
    done
    [ -n "${amdsmi_deb}" ] && [ -n "${sysdeps_deb}" ] || exit 1
    apt_get_install 20 5 600 "${amdsmi_deb}" "${sysdeps_deb}" || exit 1
    apt-mark manual "${ubuntu_packages[@]}" "${amdsmi_package}" "${sysdeps_package}" || exit 1
    # shellcheck disable=SC2016
    [ "$(dpkg-query -W -f='${Status} ${Version}' "${amdsmi_package}")" = "install ok installed ${amdsmi_version}" ] || exit 1
    # shellcheck disable=SC2016
    [ "$(dpkg-query -W -f='${Status} ${Version}' "${sysdeps_package}")" = "install ok installed ${sysdeps_version}" ] || exit 1
    [ -x "${cli_path}" ] || exit 1
    # Even --help initializes hardware in AMD SMI. Verify the Python bindings
    # and shared-library dependency closure without initialization on CPU bakes.
    python3 -I - "${python_path}" <<'AMDSMI_CHECK' || exit 1
import json
import sys
sys.path.insert(0, sys.argv[1])
import amdsmi
version = amdsmi.amdsmi_get_lib_version()
assert version["major"] > 0, version
print(json.dumps({"amd_smi_library": version, "python_module": amdsmi.__version__}))
AMDSMI_CHECK
    if [ -e /usr/local/bin/amd-smi ] && [ ! -L /usr/local/bin/amd-smi ]; then
      echo "Refusing to replace a non-symlink at /usr/local/bin/amd-smi" >&2
      exit 1
    fi
    mkdir -p /usr/local/bin || exit 1
    ln -sfnT "${cli_path}" /usr/local/bin/amd-smi || exit 1
    echo "  - ${amdsmi_package} version ${amdsmi_version}; ${sysdeps_package} version ${sysdeps_version}" >> "${VHD_LOGS_FILEPATH}" || exit 1
  )
}
