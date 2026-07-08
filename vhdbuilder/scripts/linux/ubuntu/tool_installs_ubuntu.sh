#!/bin/bash
{{/* FIPS-related error codes */}}
ERR_UA_TOOLS_INSTALL_TIMEOUT=180 {{/* Timeout waiting for ubuntu-advantage-tools install */}}
ERR_ADD_UA_APT_REPO=181 {{/* Error to add UA apt repository */}}
ERR_UA_ATTACH=182 {{/* Error attaching UA */}}
ERR_UA_DISABLE_LIVEPATCH=183 {{/* Error to disable UA livepatch */}}
ERR_UA_ENABLE_FIPS=184 {{/* Error to enable UA FIPS */}}
ERR_UA_DETACH=185 {{/* Error to detach UA */}}
ERR_LINUX_HEADER_INSTALL_TIMEOUT=186 {{/* Timeout to install linux header */}}
ERR_STRONGSWAN_INSTALL_TIMEOUT=187 {{/* Timeout to install strongswan */}}
ERR_UA_ESM_HOOK_CLEANUP=188 {{/* Error removing the apt ESM hook for Ubuntu Pro */}}
ERR_UA_MASK_UNIT=189 {{/* Error stopping/disabling/masking an Ubuntu Pro background unit */}}
ERR_UA_TOKEN_CLEANUP=190 {{/* Error removing the baked-in Ubuntu Pro machine token state */}}
ERR_AMD_ROCM_UNSUPPORTED_OS=191 {{/* AMD ROCm prebake is only supported on Ubuntu 24.04 amd64 */}}
ERR_AMD_ROCM_GPG_KEY_DOWNLOAD_TIMEOUT=192 {{/* Timeout waiting for AMD ROCm GPG key download */}}
ERR_AMD_ROCM_INSTALL_TIMEOUT=193 {{/* Timeout waiting for AMD ROCm package install */}}
ERR_AMD_ROCM_VALIDATE_FAIL=194 {{/* AMD ROCm prebake validation failed */}}
ERR_AMD_ROCM_REPO_CONFIG_INVALID=195 {{/* AMD ROCm apt source or key configuration is invalid */}}

ERR_NTP_INSTALL_TIMEOUT=10 {{/*Unable to install NTP */}}
ERR_NTP_START_TIMEOUT=11 {{/* Unable to start NTP */}}
ERR_STOP_OR_DISABLE_SYSTEMD_TIMESYNCD_TIMEOUT=12 {{/* Timeout waiting for systemd-timesyncd stop */}}
ERR_STOP_OR_DISABLE_NTP_TIMEOUT=13 {{/* Timeout waiting for ntp stop */}}
ERR_CHRONY_INSTALL_TIMEOUT=14 {{/*Unable to install CHRONY */}}
ERR_CHRONY_START_TIMEOUT=15 {{/* Unable to start CHRONY */}}


echo "Sourcing tool_installs_ubuntu.sh"

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
    pushd /tmp/bcc
    git clone https://github.com/iovisor/bcc.git
    mkdir bcc/build; cd bcc/build

    git checkout v0.29.0

    cmake -DENABLE_EXAMPLES=off .. || exit 1
    make
    sudo make install || exit 1
    cmake -DPYTHON_CMD=python3 .. || exit 1 # build python3 binding
    pushd src/python/
    make
    sudo make install || exit 1
    popd
    popd
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

amdRocmRepoBaseUrl() {
    local repo_base_url="${AMD_ROCM_REPO_BASE_URL:-https://repo.radeon.com}"
    repo_base_url="${repo_base_url%/}"
    case "${repo_base_url}" in
        https://*)
            echo "${repo_base_url}"
            return 0
            ;;
    esac
    echo "AMD_ROCM_REPO_BASE_URL must be an https URL, got '${repo_base_url}'"
    return $ERR_AMD_ROCM_REPO_CONFIG_INVALID
}

amdRocmGpgKeyUrl() {
    local repo_base_url="${1}"
    local gpg_key_url="${AMD_ROCM_GPG_KEY_URL:-${repo_base_url}/rocm/rocm.gpg.key}"
    case "${gpg_key_url}" in
        https://*)
            echo "${gpg_key_url}"
            return 0
            ;;
    esac
    echo "AMD_ROCM_GPG_KEY_URL must be an https URL, got '${gpg_key_url}'"
    return $ERR_AMD_ROCM_REPO_CONFIG_INVALID
}

amdRocmAptPinOrigin() {
    local repo_base_url="${1}"
    if [ -n "${AMD_ROCM_APT_PIN_ORIGIN:-}" ]; then
        echo "${AMD_ROCM_APT_PIN_ORIGIN}"
        return 0
    fi
    if [ "${repo_base_url}" = "https://repo.radeon.com" ]; then
        echo "repo.radeon.com"
        return 0
    fi
    return 1
}

validateAmdRocmGpgKey() {
    local gpg_key_path="${1}"
    local expected_fingerprint="${AMD_ROCM_GPG_KEY_FINGERPRINT:-CA8BB4727A47B4D09B4EE8969386B48A1A693C5C}"
    local actual_fingerprint

    if [ -z "${expected_fingerprint}" ]; then
        echo "AMD ROCm GPG key fingerprint validation is disabled"
        return 0
    fi

    expected_fingerprint="$(echo "${expected_fingerprint}" | tr -d '[:space:]' | tr '[:lower:]' '[:upper:]')"
    actual_fingerprint="$(gpg --show-keys --with-colons "${gpg_key_path}" 2>/dev/null | awk -F: '$1 == "fpr" { print $10; exit }' || true)"
    actual_fingerprint="$(echo "${actual_fingerprint}" | tr -d '[:space:]' | tr '[:lower:]' '[:upper:]')"
    if [ -z "${actual_fingerprint}" ] || [ "${actual_fingerprint}" != "${expected_fingerprint}" ]; then
        echo "AMD ROCm GPG key fingerprint mismatch. expected=${expected_fingerprint} actual=${actual_fingerprint}"
        return $ERR_AMD_ROCM_REPO_CONFIG_INVALID
    fi
}

setupAmdRocmAptRepos() {
    local rocm_version="${1}"
    local amdgpu_repo_version="${2}"
    local rocm_gpg_keyring_path="/etc/apt/keyrings/rocm.gpg"
    local rocm_gpg_key_download_path="/tmp/rocm.gpg.key"
    local repo_base_url
    local gpg_key_url
    local apt_pin_origin
    local ubuntu_codename="${UBUNTU_CODENAME:-noble}"
    repo_base_url="$(amdRocmRepoBaseUrl)" || return $ERR_AMD_ROCM_REPO_CONFIG_INVALID
    gpg_key_url="$(amdRocmGpgKeyUrl "${repo_base_url}")" || return $ERR_AMD_ROCM_REPO_CONFIG_INVALID

    if ! command -v gpg >/dev/null 2>&1; then
        apt_get_install 30 1 300 gnupg || return $ERR_AMD_ROCM_INSTALL_TIMEOUT
    fi

    mkdir -p "$(dirname "${rocm_gpg_keyring_path}")"
    retrycmd_curl_file 120 5 25 "${rocm_gpg_key_download_path}" "${gpg_key_url}" 300 || return $ERR_AMD_ROCM_GPG_KEY_DOWNLOAD_TIMEOUT
    validateAmdRocmGpgKey "${rocm_gpg_key_download_path}" || return $ERR_AMD_ROCM_REPO_CONFIG_INVALID
    gpg --dearmor --yes -o "${rocm_gpg_keyring_path}" "${rocm_gpg_key_download_path}" || return $ERR_AMD_ROCM_GPG_KEY_DOWNLOAD_TIMEOUT
    rm -f "${rocm_gpg_key_download_path}"

    cat > /etc/apt/sources.list.d/rocm.list <<EOF
deb [arch=amd64 signed-by=${rocm_gpg_keyring_path}] ${repo_base_url}/rocm/apt/${rocm_version} ${ubuntu_codename} main
deb [arch=amd64,i386 signed-by=${rocm_gpg_keyring_path}] ${repo_base_url}/graphics/${rocm_version}/ubuntu ${ubuntu_codename} main
EOF

    cat > /etc/apt/sources.list.d/amdgpu.list <<EOF
deb [arch=amd64,i386 signed-by=${rocm_gpg_keyring_path}] ${repo_base_url}/amdgpu/${amdgpu_repo_version}/ubuntu ${ubuntu_codename} main
EOF

    if apt_pin_origin="$(amdRocmAptPinOrigin "${repo_base_url}")"; then
        cat > /etc/apt/preferences.d/repo-radeon-pin-600 <<EOF
Package: *
Pin: release o=${apt_pin_origin}
Pin-Priority: 600
EOF
    else
        rm -f /etc/apt/preferences.d/repo-radeon-pin-600
    fi

    apt_get_update || return $ERR_APT_UPDATE_TIMEOUT
}

removeAmdRocmAptRepos() {
    rm -f /etc/apt/sources.list.d/rocm.list
    rm -f /etc/apt/sources.list.d/amdgpu.list
    rm -f /etc/apt/sources.list.d/amdgpu-proprietary.list
    rm -f /etc/apt/preferences.d/repo-radeon-pin-600
    rm -f /etc/apt/keyrings/rocm.gpg
    rm -f /var/lib/apt/lists/*repo.radeon.com*
    rm -f /var/lib/apt/lists/*packages.microsoft.com*
    rm -f /var/lib/apt/lists/*packages.aks.azure.com*
}

amdRocmBinaryPath() {
    local binary_name="${1}"
    if command -v "${binary_name}" >/dev/null 2>&1; then
        command -v "${binary_name}"
        return 0
    fi
    if [ -x "/opt/rocm/bin/${binary_name}" ]; then
        echo "/opt/rocm/bin/${binary_name}"
        return 0
    fi
    return 1
}

ensureAmdRocmModuleAutoload() {
    mkdir -p /etc/modules-load.d
    for modprobe_conf in /etc/modprobe.d/*.conf; do
        [ -f "${modprobe_conf}" ] || continue
        sed -i '/^[[:space:]]*blacklist[[:space:]]\+amdgpu\([[:space:]]\|$\)/d' "${modprobe_conf}"
        sed -i '/^[[:space:]]*install[[:space:]]\+amdgpu[[:space:]]\+\/bin\/false\([[:space:]]\|$\)/d' "${modprobe_conf}"
    done
    printf '%s\n' amdgpu > /etc/modules-load.d/amdgpu.conf
}

validateAmdRocmPrebake() {
    local marker_path="/opt/azure/amd-rocm/version"
    local kernel_version
    kernel_version="$(uname -r)"

    for package_name in amdgpu-dkms libdrm-amdgpu-dev rocm-core rocminfo rocm-smi-lib; do
        dpkg-query -W "${package_name}" >/dev/null || return 1
    done

    dkms status amdgpu | grep -q "${kernel_version}.*installed" || return 1
    modinfo amdgpu >/dev/null || return 1
    ! grep -qsE '^[[:space:]]*(blacklist[[:space:]]+amdgpu|install[[:space:]]+amdgpu[[:space:]]+/bin/false)([[:space:]]|$)' /etc/modprobe.d/*.conf 2>/dev/null || return 1
    grep -qx amdgpu /etc/modules-load.d/amdgpu.conf || return 1
    amdRocmBinaryPath rocminfo >/dev/null || return 1
    amdRocmBinaryPath rocm-smi >/dev/null || return 1
    [ -f "${marker_path}" ] || return 1
}

installAmdRocmPrebake() {
    local rocm_version="${AMD_ROCM_VERSION:-7.2.4}"
    local amdgpu_repo_version="${AMD_ROCM_AMDGPU_REPO_VERSION:-30.30.4}"
    local amdgpu_dkms_version="${AMD_ROCM_AMDGPU_DKMS_VERSION:-1:6.16.13.30300400-2341068.24.04}"
    local libdrm_amdgpu_dev_version="${AMD_ROCM_LIBDRM_AMDGPU_DEV_VERSION:-1:2.4.125.07020400-2341098.24.04}"
    local rocm_package_version="${AMD_ROCM_PACKAGE_VERSION:-7.2.4.70204-93~24.04}"
    local rocminfo_package_version="${AMD_ROCM_ROCMINFO_VERSION:-1.0.0.70204-93~24.04}"
    local rocm_smi_lib_package_version="${AMD_ROCM_SMI_LIB_VERSION:-7.8.0.70204-93~24.04}"
    local kernel_version
    local err
    kernel_version="$(uname -r)"

    if [ "${UBUNTU_RELEASE}" != "24.04" ] || [ "$(isARM64)" -eq 1 ]; then
        echo "AMD ROCm prebake is only supported on Ubuntu 24.04 amd64. Found Ubuntu ${UBUNTU_RELEASE}, CPU_ARCH=$(getCPUArch)."
        exit $ERR_AMD_ROCM_UNSUPPORTED_OS
    fi

    ensureAmdRocmModuleAutoload
    setupAmdRocmAptRepos "${rocm_version}" "${amdgpu_repo_version}" || { err=$?; removeAmdRocmAptRepos; exit $err; }

    apt_get_install 30 1 600 "linux-headers-${kernel_version}" "linux-modules-extra-${kernel_version}" || { removeAmdRocmAptRepos; exit $ERR_AMD_ROCM_INSTALL_TIMEOUT; }
    apt_get_install 30 1 1800 \
        "amdgpu-dkms=${amdgpu_dkms_version}" \
        "libdrm-amdgpu-dev=${libdrm_amdgpu_dev_version}" \
        "rocm-core=${rocm_package_version}" \
        "rocminfo=${rocminfo_package_version}" \
        "rocm-smi-lib=${rocm_smi_lib_package_version}" || { removeAmdRocmAptRepos; exit $ERR_AMD_ROCM_INSTALL_TIMEOUT; }

    mkdir -p /opt/azure/amd-rocm
    cat > /opt/azure/amd-rocm/version <<EOF
rocm_version=${rocm_version}
amdgpu_repo_version=${amdgpu_repo_version}
amdgpu_dkms_version=${amdgpu_dkms_version}
libdrm_amdgpu_dev_version=${libdrm_amdgpu_dev_version}
package_set=minimal-host
kernel=${kernel_version}
built_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF
    chmod 644 /opt/azure/amd-rocm/version

    validateAmdRocmPrebake || { removeAmdRocmAptRepos; exit $ERR_AMD_ROCM_VALIDATE_FAIL; }
    removeAmdRocmAptRepos
}

attachUA() {
    echo "attaching ua..."
    retrycmd_silent 5 10 1000 ua attach $UA_TOKEN || exit $ERR_UA_ATTACH

    echo "disabling ua livepatch..."
    retrycmd_if_failure 5 10 300 ua disable livepatch || exit $ERR_UA_DISABLE_LIVEPATCH
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
