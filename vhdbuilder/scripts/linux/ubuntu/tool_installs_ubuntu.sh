#!/bin/bash
ERR_UA_TOOLS_INSTALL_TIMEOUT=180 # Timeout waiting for ubuntu-advantage-tools install
ERR_ADD_UA_APT_REPO=181 # Error to add UA apt repository
ERR_UA_ATTACH=182 # Error attaching UA
ERR_UA_DISABLE_LIVEPATCH=183 # Error to disable UA livepatch
ERR_UA_ENABLE_FIPS=184 # Error to enable UA FIPS
ERR_UA_DETACH=185 # Error to detach UA
ERR_LINUX_HEADER_INSTALL_TIMEOUT=186 # Timeout to install linux header
ERR_STRONGSWAN_INSTALL_TIMEOUT=187 # Timeout to install strongswan
ERR_UA_ESM_HOOK_CLEANUP=188 # Error removing the apt ESM hook for Ubuntu Pro
ERR_UA_MASK_UNIT=189 # Error stopping/disabling/masking an Ubuntu Pro background unit
ERR_UA_TOKEN_CLEANUP=190 # Error removing the baked-in Ubuntu Pro machine token state
ERR_NTP_INSTALL_TIMEOUT=10 # Unable to install NTP
ERR_NTP_START_TIMEOUT=11 # Unable to start NTP
ERR_STOP_OR_DISABLE_SYSTEMD_TIMESYNCD_TIMEOUT=12 # Timeout waiting for systemd-timesyncd stop
ERR_STOP_OR_DISABLE_NTP_TIMEOUT=13 # Timeout waiting for ntp stop
ERR_CHRONY_INSTALL_TIMEOUT=14 # Unable to install CHRONY
ERR_CHRONY_START_TIMEOUT=15 # Unable to start CHRONY


echo "Sourcing tool_installs_ubuntu.sh"

# Bake on a CPU VM: keep DKMS and its build dependencies for kernel servicing,
# but never load the driver or install the ROCm compute SDK on the host.
# The subshell owns its cleanup trap without replacing the caller's traps.
installAMDGPUDriver() {
  (
    set -o pipefail
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
    metadata=$(jq -ce '.AMDGPUDriver | select(type == "object")' "${COMPONENTS_FILEPATH}") || exit 1
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
    metadata=$(jq -ce '.AMDGPUDiagnostics | select(type == "object")' "${COMPONENTS_FILEPATH}") || exit 1
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

installBcc() {
    echo "Installing BCC tools..."
    wait_for_apt_locks
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    VERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")
    if [ "${VERSION}" = "22.04" ] || [ "${VERSION}" = "24.04" ]; then
        apt_get_install 120 5 300 build-essential git bison cmake flex libedit-dev libllvm14 llvm-14-dev libclang-14-dev python3 zlib1g-dev libelf-dev libfl-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    else
        apt_get_install 120 5 300 build-essential git bison cmake flex libedit-dev libllvm6.0 llvm-6.0-dev libclang-6.0-dev python zlib1g-dev libelf-dev python3-distutils libfl-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    # Installing it separately here because python3-distutils is not present in the Ubuntu packages for 24.04
    if [ "${VERSION}" = "22.04" ]; then
      apt_get_install 120 5 300 python3-distutils || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    # libPolly.a is needed for the make target that runs later, which is not present in the default patch version of llvm-14 that is downloaded for 24.04
    if [ "${VERSION}" = "24.04" ]; then
      apt_get_install 120 5 300 libpolly-14-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    mkdir -p /tmp/bcc
    pushd /tmp/bcc || exit 1
    git clone https://github.com/iovisor/bcc.git
    mkdir bcc/build; cd bcc/build || exit 1

    git checkout v0.29.0

    cmake -DENABLE_EXAMPLES=off .. || exit 1
    make
    sudo make install || exit 1
    cmake -DPYTHON_CMD=python3 .. || exit 1 # build python3 binding
    pushd src/python/ || exit 1
    make
    sudo make install || exit 1
    popd || exit 1
    popd || exit 1

    # we explicitly do not remove build-essential or python
    # these are standard packages we want to keep, they should usually be in the final build anyway.
    # only ensuring they are installed above.
    if [ "${VERSION}" = "22.04" ] || [ "${VERSION}" = "24.04" ]; then
        apt_get_purge 120 5 300 bison cmake flex libedit-dev libllvm14 llvm-14-dev libclang-14-dev zlib1g-dev libelf-dev libfl-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    else
        apt_get_purge 120 5 300 git bison cmake flex libedit-dev libllvm6.0 llvm-6.0-dev libclang-6.0-dev zlib1g-dev libelf-dev libfl-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    # libPolly.a is needed for the make target that runs later, which is not present in the default patch version of llvm-14 that is downloaded for 24.04
    if [ "${VERSION}" = "24.04" ]; then
      apt_get_purge 120 5 300 libpolly-14-dev || exit $ERR_BCC_INSTALL_TIMEOUT
    fi

    rm -rf /tmp/bcc
}

installBpftrace() {
    local version="v0.9.4"
    local bpftrace_bin="bpftrace"
    local bpftrace_tools="bpftrace-tools.tar"
    local bpftrace_url="https://upstreamartifacts.azureedge.net/$bpftrace_bin/$version"
    local bpftrace_filepath="/usr/local/bin/$bpftrace_bin"
    local tools_filepath="/usr/local/share/$bpftrace_bin"
    if [ "$(isARM64)" -eq 1 ]; then
        # install bpftrace tool using default bpftrace apt package
        # the binary at "$bpftrace_url/$bpftrace_bin" is not for arm64
        if [ ! -f "/usr/sbin/bpftrace" ]; then
            apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
            apt_get_install 120 5 300 bpftrace || exit $ERR_BPFTRACE_TOOLS_INSTALL_TIMEOUT
        fi
        return
    fi

    if [ -f "$bpftrace_filepath" ]; then
        installed_version="$($bpftrace_bin -V | cut -d' ' -f2)"
        if [ "$version" = "$installed_version" ]; then
            return
        fi
        rm "$bpftrace_filepath"
        if [ -d "$tools_filepath" ]; then
            rm -r  "$tools_filepath"
        fi
    fi
    mkdir -p "$tools_filepath"
    install_dir="$BPFTRACE_DOWNLOADS_DIR/$version"
    mkdir -p "$install_dir"
    download_path="$install_dir/$bpftrace_tools"
    retrycmd_if_failure 30 5 60 curl -fSL -o "$bpftrace_filepath" "$bpftrace_url/$bpftrace_bin" || exit $ERR_BPFTRACE_BIN_DOWNLOAD_FAIL
    retrycmd_if_failure 30 5 60 curl -fSL -o "$download_path" "$bpftrace_url/$bpftrace_tools" || exit $ERR_BPFTRACE_TOOLS_DOWNLOAD_FAIL
    tar -xvf "$download_path" -C "$tools_filepath"
    chmod +x "$bpftrace_filepath"
    chmod -R +x "$tools_filepath/tools"
}

disableNtpAndTimesyncdInstallChrony() {
    # Disable systemd-timesyncd if present
    status=$(systemctl show -p SubState --value systemd-timesyncd)
    if [ "$status" = 'dead' ]; then
        echo "systemd-timesyncd is removed, no need to disable"
    else
        systemctl_stop 20 30 120 systemd-timesyncd || exit $ERR_STOP_OR_DISABLE_SYSTEMD_TIMESYNCD_TIMEOUT
        systemctl disable systemd-timesyncd || exit $ERR_STOP_OR_DISABLE_SYSTEMD_TIMESYNCD_TIMEOUT
    fi

    # Disable ntp if present
    status=$(systemctl show -p SubState --value ntp)
    if [ "$status" = 'dead' ]; then
        echo "ntp is removed, no need to disable"
    else
        systemctl_stop 20 30 120 ntp || exit $ERR_STOP_OR_DISABLE_NTP_TIMEOUT
        systemctl disable ntp || exit $ERR_STOP_OR_DISABLE_NTP_TIMEOUT
    fi

    # Install chrony
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    apt_get_install 20 30 120 chrony || exit $ERR_CHRONY_INSTALL_TIMEOUT
    cat > /etc/chrony/chrony.conf <<EOF
# Welcome to the chrony configuration file. See chrony.conf(5) for more
# information about usuable directives.

# This will use (up to):
# - 4 sources from ntp.ubuntu.com which some are ipv6 enabled
# - 2 sources from 2.ubuntu.pool.ntp.org which is ipv6 enabled as well
# - 1 source from [01].ubuntu.pool.ntp.org each (ipv4 only atm)
# This means by default, up to 6 dual-stack and up to 2 additional IPv4-only
# sources will be used.
# At the same time it retains some protection against one of the entries being
# down (compare to just using one of the lines). See (LP: #1754358) for the
# discussion.
#
# About using servers from the NTP Pool Project in general see (LP: #104525).
# Approved by Ubuntu Technical Board on 2011-02-08.
# See http://www.pool.ntp.org/join.html for more information.
#pool ntp.ubuntu.com        iburst maxsources 4
#pool 0.ubuntu.pool.ntp.org iburst maxsources 1
#pool 1.ubuntu.pool.ntp.org iburst maxsources 1
#pool 2.ubuntu.pool.ntp.org iburst maxsources 2

# This directive specify the location of the file containing ID/key pairs for
# NTP authentication.
keyfile /etc/chrony/chrony.keys

# This directive specify the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/chrony.drift

# Uncomment the following line to turn logging on.
#log tracking measurements statistics

# Log files location.
logdir /var/log/chrony

# Stop bad estimates upsetting machine clock.
maxupdateskew 100.0

# This directive enables kernel synchronisation (every 11 minutes) of the
# real-time clock. Note that it can’t be used along with the 'rtcfile' directive.
rtcsync

# Settings come from: https://docs.microsoft.com/en-us/azure/virtual-machines/linux/time-sync
refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0
makestep 1.0 -1
EOF

    systemctlEnableAndStart chrony 30 || exit $ERR_CHRONY_START_TIMEOUT
}

installFIPS() {
    echo "Installing FIPS..."
    wait_for_apt_locks

    # installing fips kernel doesn't remove non-fips kernel now, purge current linux-image-azure
    echo "purging linux-image-azure..."
    linuxImages=$(apt list --installed | grep linux-image- | grep azure | cut -d '/' -f 1)
    for image in $linuxImages; do
        echo "Removing non-fips kernel ${image}..."
        if [ "${image}" != "linux-image-$(uname -r)" ]; then
            apt_get_purge 5 10 120 ${image} || exit 1
        fi
    done

    echo "enabling ua fips-updates..."
    retrycmd_if_failure 5 10 1200 yes | ua enable fips-updates || exit $ERR_UA_ENABLE_FIPS
}

relinkResolvConf() {
    # /run/systemd/resolve/stub-resolv.conf contains local nameserver 127.0.0.53
    # remove this block after toggle disable-1804-systemd-resolved is enabled prod wide
    resolvconf=$(readlink -f /etc/resolv.conf)
    # shellcheck disable=SC3010
    if [[ "${resolvconf}" == */run/systemd/resolve/stub-resolv.conf ]]; then
        unlink /etc/resolv.conf
        ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
    fi
}

listInstalledPackages() {
    apt list --installed
}

# Report setup state separately from the Ubuntu Pro services still needing enablement:
#   unattached
#   ready
#   needs-esm esm-apps esm-infra   (or just the one disabled ESM service)
# Only esm-apps and esm-infra are required; never emit private account/contract/machine data.
ubuntuProESMState() {
    local status_json
    status_json="$(timeout 120 ua status --all --format json 2>/dev/null)" || return 1
    printf '%s' "${status_json}" | jq -ser '
        if length != 1 then error("invalid status") else .[0] end
        | if ._schema_version != "0.1" or .result != "success" or .errors != []
             or (.attached | type) != "boolean"
             or (.services | type) != "array"
             or (.execution_status != "inactive" and .execution_status != "reboot-required")
          then error("invalid status")
          elif .attached == false then "unattached"
          else
            [.services[] | select(.name == "esm-apps" or .name == "esm-infra")]
            | sort_by(.name)
            | if map(.name) != ["esm-apps", "esm-infra"]
                 or any(.[]; .entitled != "yes" or (.status != "enabled" and .status != "disabled"))
              then error("ESM unavailable")
              else
                map(select(.status != "enabled") | .name)
                | if length == 0 then "ready" else "needs-esm " + join(" ") end
              end
          end
    ' 2>/dev/null
}

attachUA() {
    # Keep both the token and captured Pro JSON out of xtrace, even for new callers.
    # A subshell restores the caller's options without exposing private local variables.
    (
        set +x
        local status_summary setup_state pending_esm_services response rc phase recovery recovered=false
        local services_to_enable=()
        if [ -z "${UA_TOKEN:-}" ] || ! command -v jq >/dev/null 2>&1; then
            echo "Ubuntu Pro attachment requires a token and jq" >&2
            exit 1
        fi
        status_summary="$(ubuntuProESMState)" || {
            echo "Unable to determine initial Ubuntu Pro state" >&2
            exit 1
        }
        read -r setup_state pending_esm_services <<< "${status_summary}"
        if [ "${setup_state}" != "unattached" ]; then
            echo "Refusing to change an initially attached Ubuntu Pro machine" >&2
            exit 1
        fi

        while [ "${setup_state}" != "ready" ]; do
            rc=0
            recovery=""
            if [ "${setup_state}" = "unattached" ]; then
                phase=attach
                echo "attaching ua without auto-enabling services..."
                response="$(timeout 1000 ua attach --no-auto-enable --format json "${UA_TOKEN}" 2>/dev/null)" || rc=$?
            elif [ "${setup_state}" = "needs-esm" ]; then
                phase=enable
                # Split only the pending names (e.g. "esm-infra"), not the setup-state marker.
                read -r -a services_to_enable <<< "${pending_esm_services}"
                echo "enabling required Ubuntu Pro ESM services: ${pending_esm_services}..."
                response="$(timeout 1000 ua enable --assume-yes --format json "${services_to_enable[@]}" 2>/dev/null)" || rc=$?
            else
                echo "Invalid Ubuntu Pro setup state" >&2
                exit 1
            fi

            if [ "${rc}" -eq 0 ]; then
                if ! printf '%s' "${response}" | jq -se --arg phase "${phase}" '
                    length == 1 and (.[0] | ._schema_version == "0.1"
                        and .result == "success" and .errors == [] and .failed_services == []
                        and ($phase != "attach" or .processed_services == []))
                ' >/dev/null 2>&1; then
                    echo "Invalid Ubuntu Pro ${phase} success response" >&2
                    exit 1
                fi
            else
                # Pro 31.2/35.1 expose HTTP status via external-api-error.additional_info.code.
                # Generic attach-failure/connectivity-error also cover permanent failures.
                # Share ONE recovery across attachment and ESM setup, not one per command.
                # Service retries require complete results and an explicit cause for every failure.
                if [ "${recovered}" = true ] || [ "${rc}" -ne 1 ] || ! recovery="$(printf '%s' "${response}" | jq -ser --arg phase "${phase}" --arg requested "${pending_esm_services}" '
                    if length != 1 then error("invalid response") else .[0] end
                    | if ._schema_version == "0.1" and .result == "failure"
                        and (.errors | type) == "array" and (.errors | length) > 0
                        and all(.errors[]; .message_code == "external-api-error"
                            and ((.type == "system" and .service == null)
                                 or ($phase == "enable" and .type == "service"
                                     and (.service == "esm-apps" or .service == "esm-infra")))
                            and (.additional_info.code == 500 or .additional_info.code == 502
                                 or .additional_info.code == 503 or .additional_info.code == 504))
                      then
                        if $phase == "enable" and any(.errors[]; .type == "system")
                        then "check-only"
                        elif $phase == "enable" then
                          if (.failed_services | type) == "array" and (.processed_services | type) == "array"
                              and (.processed_services + .failed_services | all(.[]; type == "string"))
                              and (.failed_services | unique) == ([.errors[].service] | unique)
                              and (.processed_services - .failed_services) == .processed_services
                              and (.processed_services + .failed_services | unique) == ($requested | split(" ") | unique)
                          then "retry" else error("incomplete service results") end
                        else "retry" end
                      else error("unclassified failure") end
                ' 2>/dev/null)"; then
                    echo "Ubuntu Pro ${phase} failed (exit ${rc}); no safe recovery remaining" >&2
                    exit 1
                fi
                echo "Transient Ubuntu Pro ${phase} HTTP failure; recovering once after 10 seconds..."
                sleep 10 || exit 1
                recovered=true
            fi

            # Even --no-auto-enable can fail AFTER persisting attachment credentials.
            # Resume only missing ESM services; never reattach that machine or detach it.
            status_summary="$(ubuntuProESMState)" || {
                echo "Unable to determine Ubuntu Pro state after ${phase}" >&2
                exit 1
            }
            read -r setup_state pending_esm_services <<< "${status_summary}"
            # Enable reports accumulated service errors AFTER updating its activity token.
            # A system HTTP failure there can hide permanent service errors: check, never retry.
            if [ "${recovery}" = "check-only" ] && [ "${setup_state}" != "ready" ]; then
                echo "Ubuntu Pro ESM readiness incomplete after an ambiguous enable failure" >&2
                exit 1
            fi
            if { [ "${phase}" = "enable" ] || [ "${rc}" -eq 0 ]; } && [ "${setup_state}" = "unattached" ]; then
                echo "Ubuntu Pro attachment missing after ${phase}" >&2
                exit 1
            fi
            if [ "${phase}" = "enable" ] && [ "${rc}" -eq 0 ] && [ "${setup_state}" != "ready" ]; then
                echo "Required Ubuntu Pro ESM services are not enabled" >&2
                exit 1
            fi
        done
        echo "Ubuntu Pro esm-apps and esm-infra are enabled"
    ) || exit "${ERR_UA_ATTACH}"
}

# disableAndMaskUbuntuProUnit stops, disables and masks a single Ubuntu Pro background
# systemd unit so it can never phone home (esm.ubuntu.com / contracts.canonical.com) on a
# customer node. The set of Pro units differs across releases, so a unit that is not present
# on this image is skipped rather than failing the build. Any other (unexpected) failure DOES
# fail the build: leaving an active Ubuntu Pro unit on a shipped VHD is a security/compliance
# regression and must not be silently swallowed. The helper only operates on known Pro units
# and verifies systemd is responsive before treating a missing unit as "not present", so a
# transient systemctl failure fails the build rather than silently skipping the mask.
disableAndMaskUbuntuProUnit() {
    local unit="$1"

    # Defense in depth: only ever operate on known Ubuntu Pro units so a future caller cannot
    # accidentally stop/disable/mask an unrelated systemd unit through this helper.
    case "${unit}" in
        esm-cache.service|apt-news.service|ua-timer.timer|ua-timer.service) ;;
        *)
            echo "refusing to operate on non ubuntu pro unit ${unit}"
            return 1
            ;;
    esac

    # Confirm systemd is responsive BEFORE interpreting a 'systemctl cat' miss as "unit absent".
    # Otherwise a transient systemctl/DBus failure would be misread as "not present", silently
    # skipping the mask and potentially leaving a live Ubuntu Pro unit on the shipped VHD.
    if ! systemctl list-units --all >/dev/null 2>&1; then
        echo "systemctl is not responsive while handling ${unit}; failing the build"
        return 1
    fi

    # With systemd confirmed healthy, a 'systemctl cat' miss genuinely means the unit is not
    # shipped on this release, so it is safe to skip.
    if ! systemctl cat "${unit}" >/dev/null 2>&1; then
        echo "ubuntu pro unit ${unit} not present on this image, skipping"
        return 0
    fi
    echo "stopping, disabling and masking ${unit} to keep ubuntu pro inert on customer nodes..."
    systemctl stop "${unit}" || return 1
    systemctl disable "${unit}" || return 1
    systemctl mask "${unit}" || return 1
}

detachAndCleanUpUA() {
    echo "disabling ua services individually to preserve FIPS kernel and grub config..."
    retrycmd_if_failure 5 10 300 ua disable esm-apps || exit $ERR_UA_DETACH
    retrycmd_if_failure 5 10 300 ua disable esm-infra || exit $ERR_UA_DETACH

    # The VHD is intentionally NOT 'ua detach'ed: detaching would tear down the installed FIPS
    # kernel/grub configuration. Instead we make Ubuntu Pro inert so the running customer node
    # performs NO phone-home, while leaving the FIPS packages in place. The apt ESM hook removal
    # and esm-cache masking MUST happen before the final apt_get_update below, otherwise that
    # apt update would re-trigger esm-cache and re-establish the esm.ubuntu.com traffic.

    # 1. Remove the apt ESM hook. Without this, every 'apt update' on a customer node (both
    # cloud-init and CSE run apt update during provisioning) restarts esm-cache.service, which
    # fetches ESM metadata from esm.ubuntu.com using its OWN cache independently of
    # /etc/apt/sources.list.d -- so deleting the .list files below is not sufficient on its own.
    rm -f /etc/apt/apt.conf.d/20apt-esm-hook.conf || exit $ERR_UA_ESM_HOOK_CLEANUP

    # 2. Stop, disable and mask the Ubuntu Pro background units. ua-timer drives the periodic
    # contract/metering/MOTD refresh against contracts.canonical.com; esm-cache and apt-news
    # reach out to esm.ubuntu.com. Masking keeps ubuntu-pro-client installed but inert. esm-cache
    # is masked before the final apt update so the hook (even if re-added by a package) cannot
    # start it.
    disableAndMaskUbuntuProUnit esm-cache.service || exit $ERR_UA_MASK_UNIT
    disableAndMaskUbuntuProUnit apt-news.service || exit $ERR_UA_MASK_UNIT
    disableAndMaskUbuntuProUnit ua-timer.timer || exit $ERR_UA_MASK_UNIT
    disableAndMaskUbuntuProUnit ua-timer.service || exit $ERR_UA_MASK_UNIT

    # now that the ESM/FIPS packages are installed, clean up apt settings in the vhd,
    # the VMs created on customers' subscriptions don't have access to UA repo
    rm -f /etc/apt/trusted.gpg.d/ubuntu-advantage-esm-apps.gpg
    rm -f /etc/apt/trusted.gpg.d/ubuntu-advantage-esm-infra-trusty.gpg
    rm -f /etc/apt/trusted.gpg.d/ubuntu-advantage-fips.gpg
    rm -f /etc/apt/sources.list.d/ubuntu-esm-apps.list
    rm -f /etc/apt/sources.list.d/ubuntu-esm-infra.list
    rm -f /etc/apt/sources.list.d/ubuntu-fips-updates.list
    rm -f /etc/apt/sources.list.d/ubuntu-fips-preview.list
    rm -f /etc/apt/auth.conf.d/*ubuntu-advantage

    # 3. Remove the baked-in Ubuntu Pro machine identity/state. The VHD is generalized and cloned
    # onto every customer node, so a leftover machine token would give every node the same Pro
    # machine identity. Remove the private machine-token/access state (security-relevant -> fail
    # the build on error) and the local esm-cache state (best-effort: it is only a cache and the
    # unit is already masked). ubuntu-pro-client stays installed -- removing it risks dependency
    # breakage -- but with no attached identity it stays inert.
    rm -rf /var/lib/ubuntu-advantage/private || exit $ERR_UA_TOKEN_CLEANUP
    rm -rf /var/lib/ubuntu-advantage/messages /var/lib/ubuntu-advantage/esm-cache || true

    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}
