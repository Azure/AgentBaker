#!/bin/bash

removeContainerd() {
    apt_get_purge 10 5 300 moby-containerd
}

# Batch install all packages in a single apt_get_install call instead of looping one-by-one.
# On failure, fall back to individual installs for diagnostic clarity. A return code of 2 from
# apt_get_install signals a CSE timeout and is propagated immediately by exiting the script.
aptGetBatchInstallPackagesWithFallback() {
    local -a pkg_list=("$@")

    apt_get_install 30 1 600 "${pkg_list[@]}"
    local batch_rc=$?
    if [ "$batch_rc" -eq 2 ]; then
        exit "$batch_rc"
    elif [ "$batch_rc" -ne 0 ]; then
        echo "Batch install failed, falling back to individual package install"
        local apt_package
        for apt_package in "${pkg_list[@]}"; do
            apt_get_install 30 1 600 "$apt_package"
            local pkg_rc=$?
            if [ "$pkg_rc" -eq 2 ]; then
                exit "$pkg_rc"
            elif [ "$pkg_rc" -ne 0 ]; then
                tail -n 200 /var/log/apt/term.log || true
                tail -n 200 /var/log/dpkg.log || true
                exit $ERR_APT_INSTALL_TIMEOUT
            fi
        done
    fi
}

blobfuseFallbackPackages() {
    local OSVERSION="${1}"
    # blobfuse/blobfuse2 started to be centralized in components.json around April 2026.
    # These legacy fallback versions are only for older VHDs that:
    # - do not have blobfuse/blobfuse2 in components.json yet, and
    # - did not cache blobfuse/blobfuse2 packages in the VHD.
    # This combination is unlikely, so this fallback can be removed
    # 6 months after the April 2026 release.
    local LEGACY_FALLBACK_BLOBFUSE_VERSION="1.4.5"
    local LEGACY_FALLBACK_BLOBFUSE2_VERSION="2.5.4"
    local HAS_BLOBFUSE_COMPONENT="false"
    local HAS_BLOBFUSE2_COMPONENT="false"

    if [ -n "${COMPONENTS_FILEPATH:-}" ] && [ -f "${COMPONENTS_FILEPATH}" ]; then
        if grep -q '"name"[[:space:]]*:[[:space:]]*"blobfuse"' "${COMPONENTS_FILEPATH}"; then
            HAS_BLOBFUSE_COMPONENT="true"
        fi
        if grep -q '"name"[[:space:]]*:[[:space:]]*"blobfuse2"' "${COMPONENTS_FILEPATH}"; then
            HAS_BLOBFUSE2_COMPONENT="true"
        fi
    fi

    # blobfuse2 declares Depends: fuse3 (since 2.3.0), so apt pulls it automatically.
    # blobfuse declares Depends: fuse, so apt pulls it automatically.
    # No need to explicitly install fuse3 or fuse here.
    if [ "${HAS_BLOBFUSE2_COMPONENT}" = "false" ] && ! dpkg -s blobfuse2 >/dev/null 2>&1; then
        echo "blobfuse2=${LEGACY_FALLBACK_BLOBFUSE2_VERSION}"
    fi

    if [ "${OSVERSION}" = "20.04" ]; then
        if [ "${HAS_BLOBFUSE_COMPONENT}" = "false" ] && ! dpkg -s blobfuse >/dev/null 2>&1; then
            echo "blobfuse=${LEGACY_FALLBACK_BLOBFUSE_VERSION}"
        fi
    fi
}

# Installs any required dependencies needed to build the particular Ubuntu minimal image (currently only 26.04)
# These dependencies are needed specifically in order to run various commands required to build the VHD.
installMinimalBuildDeps() {
    local OSVERSION
    OSVERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")

    if [ "${OSVERSION}" = "26.04" ]; then
        installUbuntu2604MinimalBuildDeps
        return 0
    fi

    echo "Unrecognized Ubuntu minimal version ${OSVERSION} - cannot install minimal build dependencies"
    exit 1
}

installUbuntu2604MinimalBuildDeps() {
    wait_for_apt_locks
    retrycmd_silent 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 60 5 10 dpkg -i /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_PKG_ADD_FAIL

    holdWALinuxAgent hold
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT

    local -a pkg_list=(rsyslog gpg)
    aptGetBatchInstallPackagesWithFallback "${pkg_list[@]}"
}

installDeps() {
    wait_for_apt_locks
    retrycmd_silent 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 60 5 10 dpkg -i /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_PKG_ADD_FAIL

    holdWALinuxAgent hold
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT

    local OSVERSION
    OSVERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")

    pkg_list=(apparmor-utils bind9-dnsutils ca-certificates ceph-common cgroup-lite cifs-utils conntrack cracklib-runtime ebtables ethtool glusterfs-client htop init-system-helpers inotify-tools iotop iproute2 ipset iptables nftables jq libpam-pwquality libpwquality-tools mount nfs-common pigz socat sysfsutils sysstat util-linux xz-utils netcat-openbsd zip rng-tools kmod gcc make dkms initramfs-tools linux-headers-$(uname -r))

    if [ "${OSVERSION}" = "26.04" ]; then
        if isMinimalImage; then
            # libc6-dev is needed for GPU driver installation at runtime and is not included on the 26.04 minimal base image
            pkg_list+=(libc6-dev)
            # cron/crontab is needed by init-aks-cloud.sh (RCV1P) since we create a ca-refresh cron job and is not included on the 26.04 minimal base image
            # init-aks-cloud.sh should be refactored to use systemd timers instead to align with AzureLinux
            pkg_list+=(cron)
        fi
    else
        # linux-modules-extra-* isn't bundled into linux-modules-* on Ubuntu releases < 26.04
        pkg_list+=(linux-modules-extra-$(uname -r))
    fi

    while IFS= read -r fallback_pkg; do
        [ -n "${fallback_pkg}" ] && pkg_list+=("${fallback_pkg}")
    done < <(blobfuseFallbackPackages "${OSVERSION}")

    if [ "${OSVERSION}" = "24.04" ] || [ "${OSVERSION}" = "26.04" ]; then
        pkg_list+=(irqbalance)
    fi

    if [ "${OSVERSION}" = "22.04" ] || [ "${OSVERSION}" = "24.04" ] || [ "${OSVERSION}" = "26.04" ]; then
        pkg_list+=("aznfs=3.0.19")
    fi

    aptGetBatchInstallPackagesWithFallback "${pkg_list[@]}"

    if [ "${OSVERSION}" = "22.04" ] || [ "${OSVERSION}" = "24.04" ] || [ "${OSVERSION}" = "26.04" ]; then
        # disable aznfswatchdog since aznfs install and enable aznfswatchdog and aznfswatchdogv4 services at the same time while we only need aznfswatchdogv4
        systemctl disable aznfswatchdog
        systemctl stop aznfswatchdog
    fi
}

updateAptWithMicrosoftPkg() {
    local OSVERSION
    OSVERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")

    retrycmd_silent 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft-prod.list /etc/apt/sources.list.d/ || exit $ERR_MOBY_APT_LIST_TIMEOUT

    echo "deb [arch=amd64,arm64,armhf] https://packages.microsoft.com/ubuntu/${UBUNTU_RELEASE}/prod testing main" > /etc/apt/sources.list.d/microsoft-prod-testing.list

    retrycmd_silent 120 5 25 curl https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > /tmp/microsoft.gpg || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft.gpg /etc/apt/trusted.gpg.d/ || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT

    if [ "${OSVERSION}" = "26.04" ]; then
        # Ubuntu 26.04 (Resolute) PMC repo is signed with Microsoft's newer 2025 gpg key
        retrycmd_silent 120 5 25 curl https://packages.microsoft.com/keys/microsoft-2025.asc | gpg --dearmor > /tmp/microsoft-2025.gpg || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
        retrycmd_if_failure 10 5 10 cp /tmp/microsoft-2025.gpg /etc/apt/trusted.gpg.d/ || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    fi

    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}

updatePMCRepository() {
    packageVersion="${1}"

    # Detect apt source file format: currently custom clouds use DEB822 (.sources), public cloud uses legacy (.list)
    local microsoft_prod_file="/etc/apt/sources.list.d/microsoft-prod.list"
    if [ ! -f "${microsoft_prod_file}" ] && [ -f /etc/apt/sources.list.d/microsoft-prod.sources ]; then
        microsoft_prod_file="/etc/apt/sources.list.d/microsoft-prod.sources"
    fi
    if [ ! -f "${microsoft_prod_file}" ]; then
        echo "ERROR: neither microsoft-prod.list nor microsoft-prod.sources found in /etc/apt/sources.list.d/"
        exit $ERR_APT_UPDATE_TIMEOUT
    fi
    local opts="-o Dir::Etc::sourcelist=${microsoft_prod_file} -o Dir::Etc::sourceparts=-"
    apt_get_update_with_opts "${opts}" || exit $ERR_APT_UPDATE_TIMEOUT

    # if the package version contains a tilde (~), indicating pre-release version, updating test repo
    if echo "$packageVersion" | grep -q '~'; then
        local microsoft_prod_testing_file="/etc/apt/sources.list.d/microsoft-prod-testing.list"
        if [ ! -f "${microsoft_prod_testing_file}" ] && [ -f /etc/apt/sources.list.d/microsoft-prod-testing.sources ]; then
            microsoft_prod_testing_file="/etc/apt/sources.list.d/microsoft-prod-testing.sources"
        fi
        if [ -f "${microsoft_prod_testing_file}" ]; then
            local testing_opts="-o Dir::Etc::sourcelist=${microsoft_prod_testing_file} -o Dir::Etc::sourceparts=-"
            apt_get_update_with_opts "${testing_opts}" || exit $ERR_APT_UPDATE_TIMEOUT
        fi
    fi
}

updateAptWithNvidiaPkg() {
    readonly nvidia_gpg_keyring_path="/etc/apt/keyrings/nvidia.gpg"
    mkdir -p "$(dirname "${nvidia_gpg_keyring_path}")"

    readonly nvidia_sources_list_path="/etc/apt/sources.list.d/nvidia.list"
    local cpu_arch=$(getCPUArch)  # Returns amd64 or arm64
    local repo_arch=""
    local nvidia_ubuntu_release=""

    if [ "$cpu_arch" = "amd64" ]; then
        repo_arch="x86_64"
    elif [ "$cpu_arch" = "arm64" ]; then
        repo_arch="sbsa"
    else
        echo "Unknown CPU architecture: ${cpu_arch}"
        return
    fi

    if [ "${UBUNTU_RELEASE}" = "22.04" ]; then
        nvidia_ubuntu_release="ubuntu2204"
    elif [ "${UBUNTU_RELEASE}" = "24.04" ]; then
        nvidia_ubuntu_release="ubuntu2404"
    elif [ "${UBUNTU_RELEASE}" = "26.04" ]; then
        nvidia_ubuntu_release="ubuntu2604"
    else
        echo "NVIDIA repo setup is not supported on Ubuntu ${UBUNTU_RELEASE}"
        return
    fi

    # Construct URLs based on detected architecture and Ubuntu version
    echo "deb [arch=${cpu_arch} signed-by=${nvidia_gpg_keyring_path}] https://developer.download.nvidia.com/compute/cuda/repos/${nvidia_ubuntu_release}/${repo_arch} /" > ${nvidia_sources_list_path}

    # Add NVIDIA repository
    local nvidia_gpg_key_name="3bf863cc.pub"
    if [ "${UBUNTU_RELEASE}" = "26.04" ]; then
        nvidia_gpg_key_name="60DF8A40.pub"
    fi
    local nvidia_gpg_key_url="https://developer.download.nvidia.com/compute/cuda/repos/${nvidia_ubuntu_release}/${repo_arch}/${nvidia_gpg_key_name}"

    # Download the armored NVIDIA repo key and dearmor it into a binary keyring.
    # apt only accepts a signed-by keyring with a .gpg (binary) or .asc (armored) extension;
    # NVIDIA publishes an ASCII-armored *.pub, so a raw .pub file is rejected as an "unsupported
    # filetype" and the key is ignored (the repo then fails to verify with NO_PUBKEY). Newer apt
    # (e.g. 3.x on Ubuntu 26.04) enforces this strictly, so dearmor to nvidia.gpg.
    local nvidia_gpg_key_tmp="/tmp/${nvidia_gpg_key_name}"
    retrycmd_curl_file 120 5 25 "${nvidia_gpg_key_tmp}" "${nvidia_gpg_key_url}" 300 || exit $ERR_NVIDIA_GPG_KEY_DOWNLOAD_TIMEOUT
    gpg --dearmor < "${nvidia_gpg_key_tmp}" > "${nvidia_gpg_keyring_path}" || exit $ERR_NVIDIA_GPG_KEY_DOWNLOAD_TIMEOUT
    rm -f "${nvidia_gpg_key_tmp}"
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}

isPackageInstalled() {
    local packageName="${1}"
    if dpkg -l "${packageName}" 2>/dev/null | grep -q "^ii"; then
        return 0  # Package is installed
    else
        return 1  # Package is not installed
    fi
}

managedGPUPackageList() {
    local packages=(
        datacenter-gpu-manager-4-core
        datacenter-gpu-manager-4-proprietary
        dcgm-exporter
    )

    if [ "${ENABLE_MANAGED_GPU_EXPERIENCE:-false}" = "true" ]; then
        packages+=(nvidia-device-plugin)
    elif [ "${ENABLE_MANAGED_GPU_EXPERIENCE_DRA:-false}" = "true" ]; then
        packages+=(dra-driver-nvidia-gpu)
    fi

    echo "${packages[@]}"
}

installNvidiaManagedExpPkgFromCache() {
    # Ensure kubelet device-plugins directory exists BEFORE package installation
    mkdir -p /var/lib/kubelet/device-plugins
    mkdir -p /var/lib/kubelet/plugins_registry
    mkdir -p /var/lib/kubelet/plugins

    for packageName in $(managedGPUPackageList); do
        downloadDir="/opt/${packageName}/downloads"
        if isPackageInstalled "${packageName}"; then
            echo "${packageName} is already installed, skipping."
            rm -rf $(dirname ${downloadDir})
            continue
        fi

        debFile=$(find "${downloadDir}" -maxdepth 1 -name "${packageName}*" -print -quit 2>/dev/null) || debFile=""
        if [ -z "${debFile}" ]; then
            echo "Failed to locate ${packageName} deb"
            exit $ERR_MANAGED_NVIDIA_EXP_INSTALL_FAIL
        fi
        logs_to_events "AKS.CSE.install${packageName}.installDebPackageFromFile" "installDebPackageFromFile ${debFile}" || exit $ERR_APT_INSTALL_TIMEOUT
        rm -rf $(dirname ${downloadDir})
    done
}

removeNvidiaRepos() {
    # Remove NVIDIA apt repository configuration
    # to prevent unnecessary network calls during apt-get update
    if [ -f /etc/apt/sources.list.d/nvidia.list ]; then
        rm -f /etc/apt/sources.list.d/nvidia.list
        echo "Removed NVIDIA apt repository"
    fi
    if [ -f /etc/apt/keyrings/nvidia.gpg ]; then
        rm -f /etc/apt/keyrings/nvidia.gpg
        echo "Removed NVIDIA GPG key (nvidia.gpg)"
    fi
    if [ -f /etc/apt/keyrings/nvidia.pub ]; then
        rm -f /etc/apt/keyrings/nvidia.pub
        echo "Removed NVIDIA GPG key (nvidia.pub)"
    fi
}

# loadedNvidiaModuleVersion echoes the version of the currently loaded nvidia kernel module (from
# /sys/module/nvidia/version), or nothing if no nvidia module is loaded. Split into its own function
# so callers and tests can read/stub the loaded-module signal -- the /sys path can't be created on the
# test host.
loadedNvidiaModuleVersion() {
    [ -r /sys/module/nvidia/version ] && cat /sys/module/nvidia/version 2>/dev/null
    return 0
}

# findCustomerDriverVersion echoes a currently-present nvidia driver version that DIFFERS from the
# AKS-baked marker version -- i.e. a customer-owned driver coexisting with (or replacing) AKS's
# pre-bake -- or nothing if every detectable version equals the marker. It inspects EVERY ownership
# signal, not just the first one found: the loaded kernel module, the on-disk .ko for the running
# kernel (covers .run installs with no DKMS entry), and ALL DKMS registrations. Checking every signal
# is what makes the decision order-independent: a customer version is detected even when AKS's own
# marker-version residue is still present alongside it -- e.g. AKS's module auto-loaded at boot while
# the customer's build is DKMS-registered under a different version, or AKS's stale DKMS dir sits next
# to the customer's. Returning the FIRST differing version (any single one is enough to prove a
# customer driver is present and block full teardown) keeps this cheap. $1 = marker version;
# $2 overrides the DKMS dir (test seam).
findCustomerDriverVersion() {
    local marker_version="$1" dkms_dir="${2:-/var/lib/dkms/nvidia}" v="" d
    v="$(loadedNvidiaModuleVersion)"
    if [ -n "${v}" ] && [ "${v}" != "${marker_version}" ]; then printf '%s' "${v}"; return 0; fi
    v="$(modinfo -F version nvidia 2>/dev/null | head -n1)"
    if [ -n "${v}" ] && [ "${v}" != "${marker_version}" ]; then printf '%s' "${v}"; return 0; fi
    for d in "${dkms_dir}"/*/; do
        [ -d "${d}" ] || continue
        d="${d%/}"; d="${d##*/}"
        case "${d}" in
            [0-9]*) if [ "${d}" != "${marker_version}" ]; then printf '%s' "${d}"; return 0; fi ;;
        esac
    done
    return 0
}

# cleanUpPrebakedGPUDriver removes a CUDA driver pre-baked into the shared VHD on any node that does
# NOT install the AKS-managed driver -- the cleanUpGPUDrivers path (GPU_NODE != true OR
# skip_nvidia_driver_install=true): non-GPU VMs, and GPU VMs opted out via --gpu-driver None or the
# skip toggle/tag. There the driver is dead weight (wasted disk; nvidia.ko rebuilt on every kernel
# patch) and, on an opted-out GPU node, unused attack surface.
#
# The prebaked module MAY already be loaded when we run: cuda(-lts) prebakes auto-load nvidia.ko at
# boot (~5s, well before CSE), so on a cuda/cuda-lts SKU opted out via --gpu-driver None the module
# is resident even though ensureGPUDrivers never ran. (grid prebakes do not auto-load, so grid nodes
# arrive here with no module.) Deleting the on-disk .ko then leaves a stale loaded module -- unused
# (refcnt 0, no /dev/nvidia*) but resident until reboot, and a landmine for a subsequent GPU Operator
# install. So we rmmod it first, when idle, before removing the files. No-op unless the marker exists.
# When a customer has replaced the pre-baked driver with their OWN different-version driver (e.g. via
# PreparedImageSpecification or a custom image on a --gpu-driver None pool), we clean only AKS's own
# attributable residue and leave the customer's driver intact -- see the version check below. Pass
# "force_full_teardown" ($1) to skip that check and always run the full teardown: the grid path needs
# the cuda pre-bake gone regardless of version (a driver-KIND collision with the grid driver, not a
# version one), so it must not take the customer-preserve branch. That grid caller lives in
# cse_config.sh, so shellcheck can't see the $1 being passed -- disable its unused-arg warning here.
# shellcheck disable=SC2120
cleanUpPrebakedGPUDriver() {
    local force_full="${1:-}"
    local marker="${GPU_DKMS_MARKER_FILE:-/opt/azure/aks-gpu/dkms-marker}"
    if [ ! -f "${marker}" ]; then
        return 0
    fi

    # Preserve a customer's own driver when they've replaced AKS's pre-bake with a DIFFERENT version.
    # The marker records the version AKS baked; if ANY nvidia driver present on this node -- loaded
    # module, on-disk .ko, or DKMS registration (via findCustomerDriverVersion) -- reports a different
    # version, that driver is the customer's and we must not wipe it. We check every signal, not just
    # the most-authoritative one, so a customer version is detected even when AKS's own marker-version
    # residue lingers alongside it (AKS's module auto-loaded at boot, or a stale AKS DKMS dir next to
    # the customer's) -- otherwise the blanket teardown below would delete the customer's registration
    # and userspace. On a match we clean only AKS-attributable residue -- its version-scoped DKMS dir
    # plus the aks-gpu container's /usr/bin/lib64 staging (a path no standard installer uses) -- and
    # leave the customer's module + userspace binaries untouched. A same-version customer rebuild is
    # indistinguishable from AKS's own bake and is torn down like it (accepted: same version ==
    # functionally equivalent driver). A legacy marker with no driver_version=, no other version on
    # disk, or force_full_teardown falls through to the full teardown below.
    local dkms_dir="${GPU_DKMS_NVIDIA_DIR:-/var/lib/dkms/nvidia}"
    local marker_version="" customer_version=""
    marker_version="$(sed -n 's/^driver_version=//p' "${marker}" | head -n1)"
    if [ "${force_full}" != "force_full_teardown" ] && [ -n "${marker_version}" ]; then
        customer_version="$(findCustomerDriverVersion "${marker_version}" "${dkms_dir}")"
        if [ -n "${customer_version}" ]; then
            # Customer driver coexists. Strip only AKS's own baked residue, every removal gated to the
            # marker version, and preserve the customer's driver:
            #  - AKS's version-scoped DKMS registration (usually already gone -- the customer's
            #    installer deregisters it) and the aks-gpu /usr/bin/lib64 staging (an AKS-only path).
            #  - AKS's kernel module, but ONLY when the module loaded / on disk is AKS's own (its
            #    version == the marker). Leaving AKS's marker-version module resident (it auto-loads at
            #    boot) or on disk next to the customer's userspace recreates the NVML version mismatch
            #    this teardown exists to prevent. The customer's module (a different version) is never
            #    touched; we unload only when idle, since this node installs no driver of its own.
            rm -rf "${dkms_dir:?}/${marker_version}" || true
            rm -rf /usr/bin/lib64 || true
            local loaded_version="" ondisk_version=""
            loaded_version="$(loadedNvidiaModuleVersion)"
            if [ "${loaded_version}" = "${marker_version}" ] && \
               [ "$(cat /sys/module/nvidia/refcnt 2>/dev/null || echo 0)" = "0" ] && \
               ! ls /dev/nvidia* >/dev/null 2>&1; then
                for mod in nvidia_uvm nvidia_drm nvidia_modeset nvidia_peermem nvidia; do
                    rmmod "${mod}" 2>/dev/null || true
                done
            fi
            ondisk_version="$(modinfo -F version nvidia 2>/dev/null | head -n1)"
            if [ "${ondisk_version}" = "${marker_version}" ]; then
                rm -f /lib/modules/"$(uname -r)"/updates/dkms/nvidia*.ko* 2>/dev/null || true
            fi
            echo "AKS_GPU_PREBAKE event=teardown gpu_node=${GPU_NODE:-} status=preserved_customer_driver marker_version=${marker_version} installed_version=${customer_version}"
            return 0
        fi
    fi

    echo "Removing pre-baked NVIDIA driver inherited from shared VHD (node does not install the managed driver)"
    local dkms_before=false module_before=false module_after=false
    [ -d /var/lib/dkms/nvidia ] && dkms_before=true

    # Unload the prebaked nvidia module if it auto-loaded at boot (cuda/cuda-lts SKUs). Only when idle
    # (refcnt 0 and no device nodes) -- this node doesn't install a driver, so nothing should be using
    # it; if something is, leave it and let module_after=true flag an incomplete teardown. Unload the
    # dependent modules first (modeset/uvm/drm) so nvidia's refcnt drops to 0. Best-effort; failures
    # do not abort provisioning.
    if lsmod | grep -q '^nvidia'; then
        module_before=true
        if [ "$(cat /sys/module/nvidia/refcnt 2>/dev/null || echo 0)" = "0" ] && ! ls /dev/nvidia* >/dev/null 2>&1; then
            for mod in nvidia_uvm nvidia_drm nvidia_modeset nvidia_peermem nvidia; do
                rmmod "${mod}" 2>/dev/null || true
            done
        fi
    fi
    lsmod | grep -q '^nvidia' && module_after=true

    # Deregister the nvidia DKMS module by removing its source tree (avoids the slow `dkms remove
    # --all`, ~35s). Any loaded module was unloaded above, so no depmod/initramfs refresh is needed.
    rm -rf /var/lib/dkms/nvidia || true
    rm -f /lib/modules/*/updates/dkms/nvidia*.ko* 2>/dev/null || true
    # The prebake stages libs under the aks-gpu *container's* GPU_DEST=/usr/bin (aks-gpu config.sh),
    # NOT this script's GPU_DEST=/usr/local/nvidia -- so clear /usr/bin.
    rm -rf /usr/bin/lib64 || true
    # Remove the driver binaries too (same /usr/bin) so the node is genuinely driver-free -- else
    # e.g. nvidia-smi stays on PATH but errors once its libs are gone.
    for nvidiaBin in nvidia-smi nvidia-debugdump nvidia-persistenced nvidia-cuda-mps-control \
                     nvidia-cuda-mps-server nvidia-modprobe nvidia-bug-report.sh nvidia-powerd \
                     nvidia-ngx-updater nvidia-sleep.sh; do
        rm -f "/usr/bin/${nvidiaBin}" || true
    done
    rm -f /etc/ld.so.conf.d/nvidia.conf || true
    ldconfig || true

    # Stage-1 observability + retry: assess completeness BEFORE dropping the marker. status=incomplete
    # means the DKMS registration, the setuid nvidia-modprobe binary, or a still-resident nvidia
    # module lingered (a security-coverage alert). On an incomplete teardown we KEEP the marker so the
    # next provision re-runs this cleanup (the marker is the "still needs cleanup" flag); on a clean
    # teardown we drop it. status=cleaned counts toward fleet-wide coverage. Greppable AKS_GPU_PREBAKE.
    local dkms_after=false modprobe_after=false marker_after=true status=cleaned
    [ -d /var/lib/dkms/nvidia ] && dkms_after=true
    [ -e /usr/bin/nvidia-modprobe ] && modprobe_after=true
    if [ "${dkms_after}" = false ] && [ "${modprobe_after}" = false ] && [ "${module_after}" = false ]; then
        rm -f "${marker}" || true
        [ -f "${marker}" ] || marker_after=false
    fi
    if [ "${marker_after}" = true ] || [ "${dkms_after}" = true ] || [ "${modprobe_after}" = true ] || [ "${module_after}" = true ]; then
        status=incomplete
    fi
    echo "AKS_GPU_PREBAKE event=teardown gpu_node=${GPU_NODE:-} status=${status} dkms_before=${dkms_before} module_before=${module_before} module_after=${module_after} marker_after=${marker_after} dkms_after=${dkms_after} modprobe_after=${modprobe_after}"
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu

    for packageName in $(managedGPUPackageList); do
        rm -rf "/opt/${packageName}"
    done

    # A CUDA driver pre-baked into a shared Ubuntu VHD is dead weight on a node that doesn't install
    # the managed driver (non-GPU, or GPU opted out via --gpu-driver None / skip), and while
    # DKMS-registered it forces an nvidia.ko rebuild on every kernel patch. Tear it down here.
    # No-op on VHDs without the aks-gpu prebake marker. Default (no arg) = customer-preserve enabled;
    # the grid caller in cse_config.sh passes force_full_teardown.
    # shellcheck disable=SC2119
    cleanUpPrebakedGPUDriver
}

installCriCtlPackage() {
    version="${1:-}"
    packageName="kubernetes-cri-tools=${version}"
    if [ -z "$version" ]; then
        echo "Error: No version specified for kubernetes-cri-tools package but it is required. Exiting with error."
        exit 1
    fi
    echo "Installing ${packageName} with apt-get"
    apt_get_install 20 30 120 ${packageName} || exit 1
}

installCredentialProviderFromPkg() {
    k8sVersion="${1:-}"
    os=${UBUNTU_OS_NAME}
    if [ -z "$UBUNTU_RELEASE" ]; then
        os=${OS}
        os_version="current"
    else
        os_version="${UBUNTU_RELEASE}"
    fi
    PACKAGE_VERSION=""
    getLatestPkgVersionFromK8sVersion "$k8sVersion" "azure-acr-credential-provider-pmc" "$os" "$os_version" "${OS_VARIANT}"
    packageVersion=$(echo $PACKAGE_VERSION | cut -d "-" -f 1)
    echo "installing azure-acr-credential-provider package version: $packageVersion"
    mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
    chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
    installPkgWithAptGet "azure-acr-credential-provider" "${packageVersion}" "${CREDENTIAL_PROVIDER_BIN_DIR}/acr-credential-provider" || exit "$ERR_CREDENTIAL_PROVIDER_DOWNLOAD_TIMEOUT"
}

installKubeletKubectlFromPkg() {
    local k8sVersion="${1}"

    installPkgWithAptGet "kubelet" "${k8sVersion}" "/opt/bin/kubelet" || exit "$ERR_KUBELET_INSTALL_FAIL"
    installPkgWithAptGet "kubectl" "${k8sVersion}" "/opt/bin/kubectl" || exit "$ERR_KUBECTL_INSTALL_FAIL"
}

installToolFromLocalRepo() {
    local tool_name=$1
    local tool_download_dir=$2

    # Verify the download directory exists and contains repository metadata
    if [ ! -d "${tool_download_dir}" ]; then
        echo "Download directory ${tool_download_dir} does not exist"
        return 1
    fi

    # Check if this is a self-contained local repository (has Packages.gz)
    if [ ! -f "${tool_download_dir}/Packages.gz" ]; then
        echo "Packages.gz not found in ${tool_download_dir}, not a valid local repository"
        return 1
    fi

    echo "Installing ${tool_name} from local repository at ${tool_download_dir}..."
    if ! apt_get_install_from_local_repo "${tool_download_dir}" "${tool_name}"; then
        echo "Failed to install ${tool_name} from local repository"
        return 1
    fi

    echo "${tool_name} installed successfully from local repository"
    return 0
}

installCredentialProviderPackageFromBootstrapProfileRegistry() {
    bootstrapProfileRegistry="$1"
    k8sVersion="${2:-}"

    os=${UBUNTU_OS_NAME}
    if [ -z "$UBUNTU_RELEASE" ]; then
        os=${OS}
        os_version="current"
    else
        os_version="${UBUNTU_RELEASE}"
    fi
    PACKAGE_VERSION=""
    getLatestPkgVersionFromK8sVersion "$k8sVersion" "azure-acr-credential-provider-pmc" "$os" "$os_version" "${OS_VARIANT}"
    packageVersion=$(echo $PACKAGE_VERSION | cut -d "-" -f 1)
    if [ -z "$packageVersion" ]; then
        packageVersion=$(echo "$CREDENTIAL_PROVIDER_DOWNLOAD_URL" | grep -oP 'v\d+(\.\d+)*' | sed 's/^v//' | head -n 1)
        if [ -z "$packageVersion" ]; then
            echo "Failed to determine package version for azure-acr-credential-provider"
            return $ERR_ORAS_PULL_CREDENTIAL_PROVIDER
        fi
    fi
    echo "installing azure-acr-credential-provider package version: $packageVersion"
    mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
    chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
    if ! installToolFromBootstrapProfileRegistry "azure-acr-credential-provider" $bootstrapProfileRegistry "${packageVersion}" "${CREDENTIAL_PROVIDER_BIN_DIR}/acr-credential-provider"; then
        if [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != "true" ] ; then
            # SHOULD_ENFORCE_KUBE_PMC_INSTALL will only be set for e2e tests, which should not fallback to reflect result of package installation behavior
            echo "Fall back to install credential provider from url installation"
            installCredentialProviderFromUrl
        else
            echo "Failed to install credential provider from bootstrap profile registry, and not falling back to package installation"
            exit $ERR_ORAS_PULL_CREDENTIAL_PROVIDER
        fi
    fi
}

extractDebBinaryFromFile() {
    local debFile="${1}"
    local packageName="${2}"
    local targetPath="${3:-/opt/bin/${packageName}}"
    local extractDir

    extractDir=$(mktemp -d) || return 1
    if ! dpkg-deb -x "${debFile}" "${extractDir}"; then
        rm -rf "${extractDir}"
        return 1
    fi

    local sourceBinary="${extractDir}/usr/bin/${packageName}"
    if [ ! -f "${sourceBinary}" ]; then
        echo "Failed to locate usr/bin/${packageName} in ${debFile}"
        rm -rf "${extractDir}"
        return 1
    fi

    mkdir -p "$(dirname "${targetPath}")"

    mv "${sourceBinary}" "${targetPath}"
    chown root:root "${targetPath}"
    chmod 0755 "${targetPath}"

    rm -rf "${extractDir}"
}

installPkgWithAptGet() {
    local packageName="${1:-}"
    local packageVersion="${2}"
    local targetPath="${3:-/opt/bin/${packageName}}"
    local downloadDir="/opt/${packageName}/downloads"
    local debFile=""
    local fullPackageVersion=""

    if fallbackToKubeBinaryInstall "${packageName}" "${packageVersion}" "${targetPath}"; then
        echo "Successfully installed ${packageName} version ${packageVersion} from binary fallback"
        rm -rf "${downloadDir}"
        return 0
    fi

    debFile=$(ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1) || debFile=""
    if [ -z "${debFile}" ]; then

        # update pmc repo to get latest versions
        updatePMCRepository "${packageVersion}"
        # query all package versions and get the latest version for matching k8s version and cpu architecture
        fullPackageVersion=$(apt list "${packageName}" --all-versions | grep -E "${packageVersion}([^0-9]|$)" | grep "$(getCPUArch)" | awk '{print $2}' | sort -V | tail -n 1)
        if [ -z "${fullPackageVersion}" ]; then
            echo "Failed to find valid ${packageName} version for ${packageVersion}"
            return 1
        fi
        echo "Did not find cached deb file, downloading ${packageName} version ${fullPackageVersion}"
        logs_to_events "AKS.CSE.install${packageName}FromPkg.downloadPkgFromVersion" "downloadPkgFromVersion ${packageName} ${fullPackageVersion} ${downloadDir}"

        debFile=$(ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1) || debFile=""
    fi
    if [ -z "${debFile}" ]; then
        echo "Failed to locate ${packageName} deb"
        return 1
    fi

    debFile="${downloadDir}/${debFile}"
    logs_to_events "AKS.CSE.install${packageName}.extractDebBinaryFromFile" "extractDebBinaryFromFile ${debFile} ${packageName} ${targetPath}" || exit "$ERR_APT_INSTALL_TIMEOUT"

    rm -rf "${downloadDir}"
    # Clean up stale cached binaries that were not used
    rm -f /opt/bin/"${packageName}"-* &
}

installPackageFromCache() {
    local packageName="${1:-}"
    local packageVersion="${2}"
    local targetPath="${3:-/opt/bin/${packageName}}"
    local downloadDir="/opt/${packageName}/downloads"
    local debFile=""
    local fullPackageVersion=""
    if fallbackToKubeBinaryInstall "${packageName}" "${packageVersion}" "${targetPath}"; then
        echo "Successfully installed ${packageName} version ${packageVersion} from binary fallback"
        rm -rf "${downloadDir}"
        return 0
    fi

    debFile=$(ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1) || debFile=""
    if [ -z "${debFile}" ]; then
        echo "Failed to find cached deb file for ${packageName} version ${packageVersion}"
        return 1
    fi

    debFile="${downloadDir}/${debFile}"
    logs_to_events "AKS.CSE.install${packageName}.extractDebBinaryFromFile" "extractDebBinaryFromFile ${debFile} ${packageName} ${targetPath}" || exit "$ERR_APT_INSTALL_TIMEOUT"

    rm -rf "${downloadDir}"
    rm -f /opt/bin/"${packageName}"-* &
}

downloadPkgFromVersion() {
    packageName="${1:-}"
    packageVersion="${2:-}"
    downloadDir="${3:-"/opt/${packageName}/downloads"}"
    mkdir -p ${downloadDir}
    apt_get_download 20 30 ${packageName}=${packageVersion} || exit $ERR_APT_INSTALL_TIMEOUT
    # Strip epoch (e.g., 1:4.4.1-1 -> 4.4.1-1)
    version_no_epoch="${packageVersion#*:}"
    cp -al "${APT_CACHE_DIR}/${packageName}_${version_no_epoch}"* "${downloadDir}/" || exit $ERR_APT_INSTALL_TIMEOUT
    echo "Succeeded to download ${packageName} version ${packageVersion}"
}

installContainerd() {
    local packageVersion="${3:-}"
    CONTAINERD_DOWNLOADS_DIR="${1:-$CONTAINERD_DOWNLOADS_DIR}"
    eval containerdOverrideDownloadURL="${2:-}"

    # the user-defined package URL is always picked first, and the other options won't be tried when this one fails
    if [ ! -z "${containerdOverrideDownloadURL}" ]; then
        installContainerdFromOverride ${containerdOverrideDownloadURL} || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        return 0
    fi
    installContainerdWithAptGet "${packageVersion}" "${CONTAINERD_DOWNLOADS_DIR}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
}

installContainerdFromOverride() {
    containerdOverrideDownloadURL=$1
    echo "Installing containerd from user input: ${containerdOverrideDownloadURL}"
    # we'll use a user-defined containerd package to install containerd even though it's the same version as
    # the one already installed on the node considering the source is built by the user for hotfix or test
    logs_to_events "AKS.CSE.installContainerRuntime.removeContainerd" removeContainerd
    logs_to_events "AKS.CSE.installContainerRuntime.downloadContainerdFromURL" downloadContainerdFromURL "${containerdOverrideDownloadURL}"
    logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${CONTAINERD_DEB_FILE}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    echo "Succeeded to install containerd from user input: ${containerdOverrideDownloadURL}"
    return 0
}

installContainerdWithAptGet() {
    # packageVersion is the full version string from components.json, e.g. "2.3.2-ubuntu24.04u2" or "1.7.33-ubuntu22.04u1".
    # The major.minor.patch is extracted for version comparison against the currently installed package.
    local packageVersion="${1}"
    CONTAINERD_DOWNLOADS_DIR="${2:-$CONTAINERD_DOWNLOADS_DIR}"
    local containerdMajorMinorPatchVersion
    containerdMajorMinorPatchVersion="$(echo "$packageVersion" | cut -d- -f1)"

    # Query installed version via dpkg metadata instead of running the containerd
    # binary. `containerd -version` takes ~5.7s to load the full runtime just to
    # print a version string; dpkg-query is instant.
    # dpkg version format: "1.7.31+azure-ubuntu22.04u1" or "1:1.7.31+azure-..."
    # Normalize to pure "major.minor.patch" by stripping epoch, +suffix, -suffix.
    local currentVersion=""
    if dpkg -l moby-containerd 2>/dev/null | grep -q "^ii"; then
        currentVersion=$(dpkg-query -W -f='${Version}' moby-containerd 2>/dev/null | sed 's/^[0-9]*://' | cut -d '+' -f1 | cut -d '-' -f1)
    fi

    if [ -z "$currentVersion" ]; then
        currentVersion="0.0.0"
    fi

    local currentMajorMinor desiredMajorMinor
    currentMajorMinor="$(echo $currentVersion | tr '.' '\n' | head -n 2 | paste -sd.)"
    desiredMajorMinor="$(echo $containerdMajorMinorPatchVersion | tr '.' '\n' | head -n 2 | paste -sd.)"
    semverCompare "$currentVersion" "$containerdMajorMinorPatchVersion"
    local hasGreaterVersion="$?"

    if [ "$hasGreaterVersion" = "0" ] && [ "$currentMajorMinor" = "$desiredMajorMinor" ]; then
        echo "currently installed containerd version ${currentVersion} matches major.minor with higher patch ${containerdMajorMinorPatchVersion}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${packageVersion}"
        logs_to_events "AKS.CSE.installContainerRuntime.removeContainerd" removeContainerd

        # No cached deb found — download from packages.microsoft.com
        logs_to_events "AKS.CSE.installContainerRuntime.downloadContainerdFromVersion" "downloadContainerdFromVersion ${packageVersion}"
        containerdDebFile=$(find "${CONTAINERD_DOWNLOADS_DIR}" -maxdepth 1 -name "moby-containerd_${packageVersion}*" 2>/dev/null | sort -V | tail -n1)
        if [ -z "${containerdDebFile}" ]; then
            echo "Failed to locate cached containerd deb"
            exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
        logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${containerdDebFile}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        return 0
    fi
}

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    # UBUNTU_RELEASE is already set at script load time from cse_install.sh.
    # Read UBUNTU_CODENAME from /etc/os-release instead of lsb_release (avoids Python spawn).
    UBUNTU_CODENAME=$(. /etc/os-release && echo "${VERSION_CODENAME}")
    CONTAINERD_VERSION=$1

    # the user-defined package URL is always picked first, and the other options won't be tried when this one fails
    CONTAINERD_PACKAGE_URL="${CONTAINERD_PACKAGE_URL:=}"
    if [ ! -z "${CONTAINERD_PACKAGE_URL}" ]; then
        installContainerdFromOverride ${CONTAINERD_PACKAGE_URL} || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        return 0
    fi

    echo "Using specified Containerd Version: ${CONTAINERD_VERSION}"
    installContainerdWithAptGet "${CONTAINERD_VERSION}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
}

downloadContainerdFromVersion() {
    # packageVersion is the full version string, e.g. "2.3.2-ubuntu24.04u2" or "1.7.33-ubuntu22.04u1".
    # The major.minor.patch is extracted for the apt glob pattern.
    local packageVersion="$1"
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    # Adding updateAptWithMicrosoftPkg since AB e2e uses an older image version with uncached containerd 1.6 so it needs to download from testing repo.
    # And RP no image pull e2e has apt update restrictions that prevent calls to packages.microsoft.com in CSE
    # This won't be called for new VHDs as they have containerd 1.6 cached
    updateAptWithMicrosoftPkg
    apt_get_download 20 30 moby-containerd=${packageVersion}* || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    cp -al ${APT_CACHE_DIR}moby-containerd_${packageVersion}* $CONTAINERD_DOWNLOADS_DIR/ || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    echo "Succeeded to download containerd version ${packageVersion}"
}

downloadContainerdFromURL() {
    CONTAINERD_DOWNLOAD_URL=$1
    logs_to_events "AKS.CSE.logDownloadURL" "echo $CONTAINERD_DOWNLOAD_URL"
    CONTAINERD_DOWNLOAD_URL=$(update_base_url $CONTAINERD_DOWNLOAD_URL)
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    CONTAINERD_DEB_TMP=${CONTAINERD_DOWNLOAD_URL##*/}
    retrycmd_curl_file 120 5 60 "$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}" ${CONTAINERD_DOWNLOAD_URL} 300 || exit $ERR_CONTAINERD_DOWNLOAD_TIMEOUT
    CONTAINERD_DEB_FILE="$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}"
}

ensureRunc() {
    RUNC_PACKAGE_URL=${2:-""}
    RUNC_DOWNLOADS_DIR=${3:-$RUNC_DOWNLOADS_DIR}
    # the user-defined runc package URL is always picked first, and the other options won't be tried when this one fails
    if [ ! -z "${RUNC_PACKAGE_URL}" ]; then
        echo "Installing runc from user input: ${RUNC_PACKAGE_URL}"
        mkdir -p $RUNC_DOWNLOADS_DIR
        RUNC_DEB_TMP=${RUNC_PACKAGE_URL##*/}
        RUNC_DEB_FILE="$RUNC_DOWNLOADS_DIR/${RUNC_DEB_TMP}"
        retrycmd_curl_file 120 5 60 ${RUNC_DEB_FILE} ${RUNC_PACKAGE_URL} 300 || exit $ERR_RUNC_DOWNLOAD_TIMEOUT
        # we'll use a user-defined containerd package to install containerd even though it's the same version as
        # the one already installed on the node considering the source is built by the user for hotfix or test
        installDebPackageFromFile ${RUNC_DEB_FILE} || exit $ERR_RUNC_INSTALL_TIMEOUT
        echo "Succeeded to install runc from user input: ${RUNC_PACKAGE_URL}"
        return 0
    fi

    TARGET_VERSION=${1:-""}

    if [ "$(isARM64)" -eq 1 ]; then
        if [ "${TARGET_VERSION}" = "1.0.0-rc92" ] || [ "${TARGET_VERSION}" = "1.0.0-rc95" ]; then
            # only moby-runc-1.0.3+azure-1 exists in ARM64 ubuntu repo now, no 1.0.0-rc92 or 1.0.0-rc95
            return
        fi
    fi

    CPU_ARCH=$(getCPUArch)  #amd64 or arm64
    CURRENT_VERSION=""
    if command -v runc &> /dev/null; then
        CURRENT_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
    fi
    CLEANED_TARGET_VERSION=${TARGET_VERSION}

    # after upgrading to 1.1.9, CURRENT_VERSION will also include the patch version (such as 1.1.9-1), so we trim it off
    # since we only care about the major and minor versions when determining if we need to install it
    CURRENT_VERSION=${CURRENT_VERSION%-*} # removes the -1 patch version (or similar)
    CLEANED_TARGET_VERSION=${CLEANED_TARGET_VERSION%-*} # removes the -ubuntu22.04u1 (or similar)

    if [ "${CURRENT_VERSION}" = "${CLEANED_TARGET_VERSION}" ]; then
        echo "target moby-runc version ${CLEANED_TARGET_VERSION} is already installed. skipping installRunc."
        return
    fi
    # if on a vhd-built image, first check if we've cached the deb file
    if [ -f "$VHD_LOGS_FILEPATH" ]; then
        RUNC_DEB_PATTERN="moby-runc_*.deb"
        RUNC_DEB_FILES=()
        RUNC_DEB_FILE=""
        while IFS= read -r file; do
            RUNC_DEB_FILES+=("$file")
        done < <(find "${RUNC_DOWNLOADS_DIR}" -type f -iname "${RUNC_DEB_PATTERN}" 2>/dev/null)
        if [ ${#RUNC_DEB_FILES[@]} -gt 0 ]; then
            RUNC_DEB_FILE=$(printf "%s\n" "${RUNC_DEB_FILES[@]}" | sort -V | tail -n1)
        fi
        if [ -n "${RUNC_DEB_FILE}" ] && [ -f "${RUNC_DEB_FILE}" ]; then
            echo "Found cached runc deb file: ${RUNC_DEB_FILE}"
            installDebPackageFromFile ${RUNC_DEB_FILE} || exit $ERR_RUNC_INSTALL_TIMEOUT
            return 0
        fi
    fi
    echo "No cached runc deb file is found. Using apt-get to install runc."
    apt_get_install 20 30 120 moby-runc=${TARGET_VERSION}* --allow-downgrades || exit $ERR_RUNC_INSTALL_TIMEOUT
}

#EOF
