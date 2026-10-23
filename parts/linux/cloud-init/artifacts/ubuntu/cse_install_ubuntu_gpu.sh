#!/bin/bash

prebakedGPUDriverArtifactsRemain() {
    local binary modules
    [ -d /var/lib/dkms/nvidia ] || [ -e /usr/bin/lib64 ] || [ -e /etc/ld.so.conf.d/nvidia.conf ] && return 0
    for binary in nvidia-smi nvidia-debugdump nvidia-persistenced nvidia-cuda-mps-control \
                  nvidia-cuda-mps-server nvidia-modprobe nvidia-bug-report.sh nvidia-powerd \
                  nvidia-ngx-updater nvidia-sleep.sh; do
        [ -e "/usr/bin/${binary}" ] && return 0
    done
    compgen -G '/lib/modules/*/updates/dkms/nvidia*.ko*' >/dev/null && return 0
    modules=$(lsmod) || return 0
    grep -q '^nvidia' <<< "${modules}"
}

graceBlackwellDriverBOMEntries() {
    # Packer stages this BOM only on NVIDIA_GB images; package versions must still match it before removal.
    local bom="${1:-${GB_MAI_BOM_FILE:-/opt/azure/containers/gb-mai-bom.json}}"
    local wave="${2:-versions-wave2}"
    [ -s "${bom}" ] || return 1

    jq -er --arg wave "${wave}" '
        .[$wave] as $packages |
        select(($packages | type) == "object") |
        select(any($packages | keys[]; test("^nvidia-dkms-[0-9]+-open$"))) |
        select(any($packages | keys[]; test("^nvidia-driver-[0-9]+-open$"))) |
        select(all($packages | keys[]; test("^(libxnvctrl0|libnvidia-[A-Za-z0-9-]+|nvidia-[A-Za-z0-9-]+|xserver-xorg-video-nvidia-[0-9]+)$"))) |
        select(all($packages | values[]; type == "string" and length > 0)) |
        $packages | to_entries[] | "\(.key)=\(.value)"
    ' "${bom}"
}

graceBlackwellPackageManagedDriverRemains() {
    local allowed="${1:-}" package_states
    package_states=$(dpkg-query -W -f='${binary:Package}\t${db:Status-Status}\n') || {
        echo "unable to inspect installed NVIDIA driver packages"
        return 0
    }
    awk -F '\t' -v allowed="${allowed}" '
        $2 == "installed" {
            name = $1
            sub(/:.*/, "", name)
            if (name !~ /^(nvidia-(dkms|driver|utils|compute-utils|firmware|kernel-)|nvidia-(modprobe|persistenced)$|xserver-xorg-video-nvidia-|libxnvctrl0$|libnvidia-)/ || name ~ /^libnvidia-container/) next
            count = split(allowed, owned, "\n")
            for (i = 1; i <= count; i++) {
                sub(/=.*/, "", owned[i])
                if (owned[i] == name) next
            }
            print $1
            found = 1
            exit
        }
        END { if (!found) exit 1 }
    ' <<< "${package_states}"
}

graceBlackwellInstalledDriverPackages() {
    local entries="${1}" package expected state version series="" auto_packages bom_package
    local -a installed=()

    while IFS='=' read -r package expected; do
        [ -n "${package}" ] || continue
        case "${package}" in
            nvidia-driver-[0-9]*-open)
                series="${package#nvidia-driver-}"
                series="${series%-open}"
                ;;
        esac
        state=$(dpkg-query -W -f='${db:Status-Status}\t${Version}' "${package}" 2>/dev/null) || state=""
        [ "${state%%$'\t'*}" = installed ] || continue
        version="${state#*$'\t'}"
        if [ "${version}" != "${expected}" ]; then
            echo "Grace Blackwell package ${package} version ${version} does not match the GB BOM ${expected}" >&2
            return 1
        fi
        installed+=("${package}=${version}")
    done <<< "${entries}"

    [ -n "${series}" ] || return 1
    auto_packages=$(apt-mark showauto) || return 1
    # BOMs do not always list apt dependencies; use each dependency's exact dpkg version, never the driver's version.
    for package in "nvidia-utils-${series}" "nvidia-compute-utils-${series}" nvidia-modprobe nvidia-persistenced; do
        while IFS='=' read -r bom_package expected; do
            [ "${bom_package}" = "${package}" ] && continue 2
        done <<< "${entries}"
        state=$(dpkg-query -W -f='${db:Status-Status}\t${Version}' "${package}" 2>/dev/null) || state=""
        [ "${state%%$'\t'*}" = installed ] || continue
        version="${state#*$'\t'}"
        if [ -z "${version}" ] || ! grep -Eq "^${package}(:arm64)?$" <<< "${auto_packages}"; then
            echo "Grace Blackwell dependency ${package} is not an auto-installed package with a verifiable version" >&2
            return 1
        fi
        installed+=("${package}=${version}")
    done

    printf '%s\n' "${installed[@]}"
}

graceBlackwellConfigIsUnmodified() {
    local config="${1}" expected
    shift
    [ -e "${config}" ] || return 0

    expected=$(printf '%s\n' "$@")
    if ! cmp -s "${config}" <(printf '%s\n' "$@") &&
        ! cmp -s "${config}" <(printf '%s' "${expected}"); then
        echo "Grace Blackwell module configuration ${config} was modified; refusing teardown" >&2
        return 1
    fi
}

cleanUpGraceBlackwellGPUDriver() {
    local entries="${1}" bom="${2}" wave="${3}" pending="${4:-false}"
    local packages package version modules service module remaining_package pending_file
    pending_file="${GB_DRIVER_CLEANUP_PENDING_FILE:-/opt/azure/containers/gb-driver-cleanup.pending}"
    local nouveau_config="${GB_NOUVEAU_MODPROBE_CONFIG_FILE:-/etc/modprobe.d/blacklist-nouveau.conf}"
    local peermem_config="${GB_NVIDIA_PEERMEM_CONFIG_FILE:-/etc/modules-load.d/nvidia-peermem.conf}"
    local nvidia_config="${GB_NVIDIA_MODPROBE_CONFIG_FILE:-/etc/modprobe.d/nvidia.conf}"
    packages=$(graceBlackwellInstalledDriverPackages "${entries}") || return 1
    if remaining_package=$(graceBlackwellPackageManagedDriverRemains "${packages}"); then
        echo "NVIDIA package ${remaining_package} is not owned by the Grace Blackwell BOM" >&2
        return 1
    fi
    if [ "${pending}" != true ] && [ ! -f "${nvidia_config}" ]; then
        modules=$(lsmod) || return 1
        # BasePrep may retry after teardown succeeded but a later step failed.
        if [ -z "${packages}" ] && ! grep -q '^nvidia' <<< "${modules}" &&
            ! prebakedGPUDriverArtifactsRemain &&
            [ ! -e "${nvidia_config}" ] && [ ! -L "${nvidia_config}" ] &&
            [ ! -e "${peermem_config}" ] && [ ! -L "${peermem_config}" ] &&
            [ ! -e "${nouveau_config}" ] && [ ! -L "${nouveau_config}" ]; then
            return 0
        fi
        echo "Grace Blackwell NVIDIA module configuration ${nvidia_config} is unavailable; refusing teardown" >&2
        return 1
    fi
    [ "${pending}" = true ] || [ -f "${nvidia_config}" ] || {
        echo "Grace Blackwell NVIDIA module configuration ${nvidia_config} is unavailable; refusing teardown" >&2
        return 1
    }
    graceBlackwellConfigIsUnmodified "${peermem_config}" \
        '# Explicitly load nvidia_peermem at boot for GPUDirect RDMA.' \
        "# We cannot rely on a \`softdep nvidia post: nvidia_peermem\` in modprobe.d because" \
        "# \`nvidia\` is autoloaded by the kernel via PCI modalias before the OFED/RDMA" \
        '# stack is up, so the post-dep fails silently. systemd-modules-load.service' \
        '# runs later in boot, by which point modprobe can resolve the full dep chain' \
        '# (ib_core, mlx5_ib, nvidia, ...).' \
        'nvidia_peermem' || return 1
    graceBlackwellConfigIsUnmodified "${nvidia_config}" \
        'options nvidia NVreg_RestrictProfilingToAdminUsers=0' \
        'options nvidia NVreg_CreateImexChannel0=1' \
        'options nvidia NVreg_CoherentGPUMemoryMode=driver' \
        'options nvidia NVreg_RegistryDwords="RMBug5172204War=4"' || return 1
    graceBlackwellConfigIsUnmodified "${nouveau_config}" \
        'blacklist nouveau' 'options nouveau modeset=0' || return 1

    local -a purge_packages=()
    while IFS='=' read -r package version; do
        [ -n "${package}" ] && purge_packages+=("${package}")
    done <<< "${packages}"

    for service in nvidia-device-plugin nvidia-dcgm-exporter nvidia-dcgm nvidia-imex nvidia-persistenced; do
        systemctlDisableAndStop "${service}" || return 1
        if systemctl is-active --quiet "${service}" || systemctl is-enabled --quiet "${service}"; then
            echo "Grace Blackwell service ${service} remains active or enabled" >&2
            return 1
        fi
    done

    modules=$(lsmod) || return 1
    if grep -q '^nvidia' <<< "${modules}" && systemctl cat openibd &>/dev/null; then
        systemctl_stop 20 5 25 openibd || {
            echo "Grace Blackwell service openibd could not be stopped" >&2
            return 1
        }
        if systemctl is-active --quiet openibd; then
            echo "Grace Blackwell service openibd remains active" >&2
            return 1
        fi
        modules=$(lsmod) || return 1
    fi
    for module in nvidia_peermem nvidia_uvm nvidia_drm nvidia_modeset nvidia; do
        grep -q "^${module}[[:space:]]" <<< "${modules}" || continue
        rmmod "${module}" || { echo "Failed to unload Grace Blackwell module ${module}" >&2; return 1; }
        modules=$(lsmod) || return 1
        grep -q "^${module}[[:space:]]" <<< "${modules}" && {
            echo "Grace Blackwell module ${module} remains loaded after removal" >&2
            return 1
        }
    done
    grep -q '^nvidia' <<< "${modules}" && { echo "NVIDIA module remains loaded after Grace Blackwell teardown" >&2; return 1; }

    if [ "${#purge_packages[@]}" -gt 0 ]; then
        apt_get_purge 10 5 300 "${purge_packages[@]}" || {
            echo "Grace Blackwell driver package purge failed" >&2
            return 1
        }
    fi

    if [ "${pending}" != true ]; then
        printf '%s\n' "${wave}" > "${pending_file}" || {
            rm -f "${pending_file}"
            echo "Unable to preserve Grace Blackwell teardown provenance" >&2
            return 1
        }
    fi
    rm -f "${peermem_config}" "${nvidia_config}" "${nouveau_config}" || return 1
    for package in "${peermem_config}" "${nvidia_config}" "${nouveau_config}"; do
        if [ -e "${package}" ] || [ -L "${package}" ]; then
            echo "Grace Blackwell module configuration ${package} remains after removal" >&2
            return 1
        fi
    done
    update-initramfs -u || return 1
    rm -f "${pending_file}" || return 1
    [ ! -e "${pending_file}" ] || return 1
}

cleanUpGPUDriversForBasePrep() {
    local packageName marker="${GPU_DKMS_MARKER_FILE:-/opt/azure/aks-gpu/dkms-marker}" remaining_package
    local peermem_config="${GB_NVIDIA_PEERMEM_CONFIG_FILE:-/etc/modules-load.d/nvidia-peermem.conf}"
    local bom="${GB_MAI_BOM_FILE:-/opt/azure/containers/gb-mai-bom.json}"
    local legacy_bom="${GB200_MAI_BOM_FILE:-/opt/azure/containers/gb200-mai-bom.json}"
    local pending_file="${GB_DRIVER_CLEANUP_PENDING_FILE:-/opt/azure/containers/gb-driver-cleanup.pending}"
    local wave=versions-wave2 entries gb_layout=false pending_teardown=false pending_wave

    if [ -e "${pending_file}" ]; then
        IFS= read -r pending_wave < "${pending_file}" || {
            echo "GPU basePrep cleanup incomplete: invalid Grace Blackwell pending marker" >&2
            return 1
        }
        case "${pending_wave}" in
            versions-wave2) ;;
            versions-wave1)
                bom="${legacy_bom}"
                wave=versions-wave1
                ;;
            *)
            echo "GPU basePrep cleanup incomplete: invalid Grace Blackwell pending marker" >&2
            return 1
            ;;
        esac
        gb_layout=true
        pending_teardown=true
    elif [ -e "${peermem_config}" ]; then
        gb_layout=true
    elif [ -e "${legacy_bom}" ] && [ ! -e "${bom}" ]; then
        bom="${legacy_bom}"
        wave=versions-wave1
        gb_layout=true
    fi

    entries=$(graceBlackwellDriverBOMEntries "${bom}" "${wave}" 2>/dev/null) || entries=""
    if [ "${gb_layout}" = true ]; then
        if [ -z "${entries}" ]; then
            echo "GPU basePrep cleanup incomplete: Grace Blackwell BOM is unavailable" >&2
            return 1
        fi
        cleanUpGraceBlackwellGPUDriver "${entries}" "${bom}" "${wave}" "${pending_teardown}" || {
            echo "GPU basePrep cleanup incomplete: Grace Blackwell driver teardown failed" >&2
            return 1
        }
    elif remaining_package=$(graceBlackwellPackageManagedDriverRemains); then
        echo "GPU basePrep cleanup incomplete: NVIDIA package ${remaining_package} has no verifiable GB BOM ownership" >&2
        return 1
    fi

    if remaining_package=$(graceBlackwellPackageManagedDriverRemains); then
        echo "GPU basePrep cleanup incomplete: NVIDIA package ${remaining_package} remains after Grace Blackwell teardown" >&2
        return 1
    fi

    cleanUpGPUDrivers
    if prebakedGPUDriverArtifactsRemain; then
        echo "GPU basePrep cleanup incomplete: pre-baked GPU driver artifacts remain" >&2
        return 1
    fi
    if [ -e "${GPU_DEST}" ] || [ -e /opt/gpu ] || [ -e "${marker}" ]; then
        echo "GPU basePrep cleanup incomplete: AKS driver artifacts remain" >&2
        return 1
    fi
    for packageName in $(managedGPUPackageList); do
        if [ -e "/opt/${packageName}" ]; then
            echo "GPU basePrep cleanup incomplete: ${packageName} cache remains" >&2
            return 1
        fi
    done
}
