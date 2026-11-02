#!/bin/bash

prebakedGPUDriverArtifactsRemain() {
    local binary modules
    modules=$(lsmod) || {
        echo "unable to inspect loaded NVIDIA modules" >&2
        return 2
    }
    [ -d /var/lib/dkms/nvidia ] || [ -e /usr/bin/lib64 ] || [ -e /etc/ld.so.conf.d/nvidia.conf ] && return 0
    for binary in nvidia-smi nvidia-debugdump nvidia-persistenced nvidia-cuda-mps-control \
                  nvidia-cuda-mps-server nvidia-modprobe nvidia-bug-report.sh nvidia-powerd \
                  nvidia-ngx-updater nvidia-sleep.sh; do
        [ -e "/usr/bin/${binary}" ] && return 0
    done
    compgen -G '/lib/modules/*/updates/dkms/nvidia*.ko*' >/dev/null && return 0
    grep -q '^nvidia' <<< "${modules}"
}

graceBlackwellDriverBOMEntries() {
    # Packer stages this BOM only on NVIDIA_GB images; package versions must still match it before removal.
    local bom="${1:-${GB_MAI_BOM_FILE:-/opt/azure/containers/gb-mai-bom.json}}"
    local wave="${2:-versions-wave2}"
    [ -e "${bom}" ] || return 2
    [ -f "${bom}" ] && [ ! -L "${bom}" ] || return 2
    if [ ! -r "${bom}" ] || ! jq empty "${bom}" >/dev/null; then
        echo "unable to inspect Grace Blackwell BOM ${bom}" >&2
        return 1
    fi

    local result
    jq -er --arg wave "${wave}" '
        select(type == "object") |
        .[$wave] as $packages |
        select(($packages | type) == "object") |
        select(any($packages | keys[]; test("^nvidia-dkms-[0-9]+-open$"))) |
        select(any($packages | keys[]; test("^nvidia-driver-[0-9]+-open$"))) |
        select(all($packages | keys[]; test("^(libxnvctrl0|libnvidia-(common|cfg1|gpucomp|gl|compute|extra|decode|encode|fbc1)-[0-9]+|nvidia-(firmware|kernel-common)-[0-9]+|nvidia-(dkms|driver|kernel-source)-[0-9]+-open|xserver-xorg-video-nvidia-[0-9]+)$"))) |
        select(all($packages | values[]; type == "string" and length > 0)) |
        $packages | to_entries[] | "\(.key)=\(.value)"
    ' "${bom}"
    result=$?
    case "${result}" in
        0) return 0 ;;
        4) return 2 ;;
        *)
            echo "unable to inspect Grace Blackwell BOM ${bom}" >&2
            return 1
            ;;
    esac
}

# Return 0 with the first unowned driver package, 1 when none remain, or 2 on
# inspection failure. EGL platform adapters are retained: they contain no kernel
# driver and their packages do not depend on the driver packages being purged.
graceBlackwellPackageManagedDriverRemains() {
    local allowed="${1:-}" package_states="${2-}" result
    if [ "$#" -lt 2 ]; then
        package_states=$(dpkg-query -W -f='${binary:Package}\t${db:Status-Status}\n') || {
            echo "unable to inspect installed NVIDIA driver packages"
            return 2
        }
    fi
    GPU_CLEANUP_ALLOWED="${allowed}" awk -F '\t' '
        BEGIN { allowed = ENVIRON["GPU_CLEANUP_ALLOWED"] }
        $2 == "installed" {
            name = $1
            sub(/:.*/, "", name)
            if (name !~ /^(nvidia-(dkms|driver|utils|compute-utils|firmware|kernel-)|nvidia-(modprobe|persistenced)$|xserver-xorg-video-nvidia-|libxnvctrl0$|libnvidia-)/ || name ~ /^libnvidia-container/) next
            if (name ~ /^libnvidia-egl-(gbm1|wayland1|xcb1|xlib1)$/) {
                print "WARNING: retaining NVIDIA EGL support package " name > "/dev/stderr"
                next
            }
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
    result=$?
    if [ "${result}" -gt 1 ]; then
        echo "unable to inspect installed NVIDIA driver packages"
        return 2
    fi
    return "${result}"
}

graceBlackwellInstalledPackageState() {
    awk -F '\t' -v package="${1}" '
        $1 == package || $1 == package ":arm64" { print $2 "\t" $3; exit }
    ' <<< "${2}"
}

graceBlackwellInstalledDriverPackages() {
    local entries="${1}" package expected state version series="" auto_packages bom_package package_states="${2-}"
    local -a installed=()
    if [ "$#" -lt 2 ]; then
        package_states=$(dpkg-query -W -f='${binary:Package}\t${db:Status-Status}\t${Version}\n') || {
            echo "unable to inspect installed NVIDIA driver packages" >&2
            return 1
        }
    fi

    while IFS='=' read -r package expected; do
        [ -n "${package}" ] || continue
        case "${package}" in
            nvidia-driver-[0-9]*-open)
                series="${package#nvidia-driver-}"
                series="${series%-open}"
                ;;
        esac
        state=$(graceBlackwellInstalledPackageState "${package}" "${package_states}") || return 1
        [ "${state%%$'\t'*}" = installed ] || continue
        version="${state#*$'\t'}"
        [ -n "${version}" ] || {
            echo "unable to inspect installed version of NVIDIA package ${package}" >&2
            return 1
        }
        if [ "${version}" != "${expected}" ]; then
            echo "Grace Blackwell package ${package} version ${version} does not match the GB BOM ${expected}" >&2
            return 2
        fi
        installed+=("${package}=${version}")
    done <<< "${entries}"

    [ -n "${series}" ] || return 2
    auto_packages=$(apt-mark showauto) || {
        echo "unable to inspect auto-installed NVIDIA packages" >&2
        return 1
    }
    # BOMs do not always list apt dependencies; use each dependency's exact dpkg version, never the driver's version.
    for package in "nvidia-utils-${series}" "nvidia-compute-utils-${series}" nvidia-modprobe nvidia-persistenced nvidia-settings; do
        while IFS='=' read -r bom_package expected; do
            [ "${bom_package}" = "${package}" ] && continue 2
        done <<< "${entries}"
        state=$(graceBlackwellInstalledPackageState "${package}" "${package_states}") || return 1
        [ "${state%%$'\t'*}" = installed ] || continue
        version="${state#*$'\t'}"
        [ -n "${version}" ] || {
            echo "unable to inspect installed version of NVIDIA package ${package}" >&2
            return 1
        }
        if ! grep -Eq "^${package}(:arm64)?$" <<< "${auto_packages}"; then
            echo "Grace Blackwell dependency ${package} is not an auto-installed package with a verifiable version" >&2
            return 2
        fi
        installed+=("${package}=${version}")
    done

    printf '%s\n' "${installed[@]}"
}

graceBlackwellConfigIsUnmodified() {
    local config="${1}" expected result
    shift
    if [ -L "${config}" ] || { [ -e "${config}" ] && [ ! -f "${config}" ]; }; then
        echo "Grace Blackwell module configuration ${config} is not an owned regular file" >&2
        return 2
    fi
    [ -e "${config}" ] || return 0

    expected=$(printf '%s\n' "$@")
    cmp -s "${config}" <(printf '%s\n' "$@") && return 0
    result=$?
    if [ "${result}" -gt 1 ]; then
        echo "unable to inspect Grace Blackwell module configuration ${config}" >&2
        return 1
    fi
    cmp -s "${config}" <(printf '%s' "${expected}") && return 0
    result=$?
    if [ "${result}" -gt 1 ]; then
        echo "unable to inspect Grace Blackwell module configuration ${config}" >&2
        return 1
    fi
    echo "Grace Blackwell module configuration ${config} was modified; retaining driver" >&2
    return 2
}

# Return 2 only when read-only preflight cannot establish safe ownership. The
# caller retains that intact driver. Inspection errors and every failure after
# service/module/package teardown starts return 1, including pending retries.
cleanUpGraceBlackwellGPUDriver() {
    local entries="${1}" wave="${2}" pending="${3:-false}"
    local packages package version modules service module remaining_package pending_file result
    local simulation action candidate owned remaining_module package_states plan_version expected_versions bom_entries state
    local bom="${4:-${GB_MAI_BOM_FILE:-/opt/azure/containers/gb-mai-bom.json}}"
    pending_file="${GB_DRIVER_CLEANUP_PENDING_FILE:-/opt/azure/containers/gb-driver-cleanup.pending}"
    local nouveau_config="${GB_NOUVEAU_MODPROBE_CONFIG_FILE:-/etc/modprobe.d/blacklist-nouveau.conf}"
    local peermem_config="${GB_NVIDIA_PEERMEM_CONFIG_FILE:-/etc/modules-load.d/nvidia-peermem.conf}"
    local nvidia_config="${GB_NVIDIA_MODPROBE_CONFIG_FILE:-/etc/modprobe.d/nvidia.conf}"
    package_states=$(dpkg-query -W -f='${binary:Package}\t${db:Status-Status}\t${Version}\n') || {
        echo "unable to inspect installed NVIDIA driver packages" >&2
        return 1
    }
    packages=$(graceBlackwellInstalledDriverPackages "${entries}" "${package_states}") || return $?
    remaining_package=$(graceBlackwellPackageManagedDriverRemains "${packages}" "${package_states}")
    result=$?
    if [ "${result}" -eq 2 ]; then
        echo "${remaining_package}" >&2
        return 1
    elif [ "${result}" -eq 0 ]; then
        echo "NVIDIA package ${remaining_package} is not owned by the Grace Blackwell BOM" >&2
        return 2
    fi
    modules=$(lsmod) || { echo "unable to inspect loaded NVIDIA modules" >&2; return 1; }
    remaining_module=$(awk '$1 ~ /^nvidia/ && $1 !~ /^(nvidia|nvidia_peermem|nvidia_uvm|nvidia_drm|nvidia_modeset)$/ { print $1; exit }' <<< "${modules}") || return 1
    if [ -n "${remaining_module}" ]; then
        echo "NVIDIA module ${remaining_module} has no verified teardown order; retaining driver" >&2
        return 2
    fi
    if [ "${pending}" != true ] && [ ! -f "${nvidia_config}" ]; then
        # BasePrep may retry after teardown succeeded but a later step failed.
        if [ -z "${packages}" ] && ! grep -q '^nvidia' <<< "${modules}" &&
            [ ! -e "${nvidia_config}" ] && [ ! -L "${nvidia_config}" ] &&
            [ ! -e "${peermem_config}" ] && [ ! -L "${peermem_config}" ] &&
            [ ! -e "${nouveau_config}" ] && [ ! -L "${nouveau_config}" ]; then
            prebakedGPUDriverArtifactsRemain
            result=$?
            [ "${result}" -ne 2 ] || return 1
            [ "${result}" -ne 1 ] || return 0
        fi
        echo "Grace Blackwell NVIDIA module configuration ${nvidia_config} is unavailable; refusing teardown" >&2
        return 2
    fi
    graceBlackwellConfigIsUnmodified "${peermem_config}" \
        '# Explicitly load nvidia_peermem at boot for GPUDirect RDMA.' \
        "# We cannot rely on a \`softdep nvidia post: nvidia_peermem\` in modprobe.d because" \
        "# \`nvidia\` is autoloaded by the kernel via PCI modalias before the OFED/RDMA" \
        '# stack is up, so the post-dep fails silently. systemd-modules-load.service' \
        '# runs later in boot, by which point modprobe can resolve the full dep chain' \
        '# (ib_core, mlx5_ib, nvidia, ...).' \
        'nvidia_peermem' || return $?
    graceBlackwellConfigIsUnmodified "${nvidia_config}" \
        'options nvidia NVreg_RestrictProfilingToAdminUsers=0' \
        'options nvidia NVreg_CreateImexChannel0=1' \
        'options nvidia NVreg_CoherentGPUMemoryMode=driver' \
        'options nvidia NVreg_RegistryDwords="RMBug5172204War=4"' || return $?
    graceBlackwellConfigIsUnmodified "${nouveau_config}" \
        'blacklist nouveau' 'options nouveau modeset=0' || return $?

    local -a purge_packages=()
    while IFS='=' read -r package version; do
        [ -n "${package}" ] && purge_packages+=("${package}")
    done <<< "${packages}"

    if [ "${#purge_packages[@]}" -gt 0 ]; then
        simulation=$(LC_ALL=C apt-get -s purge -- "${purge_packages[@]}") || {
            echo "unable to inspect Grace Blackwell purge" >&2
            return 1
        }
        # Dependency removals may span the driver and IMEX installation waves.
        bom_entries=$(jq -r '
            to_entries[] | select(.key | test("^versions-wave[1-3]$")) |
            .value | objects | to_entries[] |
            select(.value | type == "string" and length > 0) | "\(.key)=\(.value)"
        ' "${bom}") || {
            echo "unable to inspect Grace Blackwell purge ownership" >&2
            return 1
        }
        while read -r action package plan_version _; do
            case "${action}" in
                Inst|Conf)
                    echo "Grace Blackwell purge has non-removal action ${action} for ${package}; retaining driver" >&2
                    return 2
                    ;;
                Remv|Purg)
                    owned=false
                    for candidate in "${purge_packages[@]}"; do
                        [ "${package%:arm64}" != "${candidate}" ] || owned=true
                    done
                    if [ "${owned}" != true ]; then
                        # Apt can also remove wave3 IMEX (Depends: nvidia-modprobe).
                        # Prove the closure from this image's BOM, never from apt's proposal alone.
                        expected_versions=$(awk -F '=' -v package="${package%:arm64}" '
                            $1 == package { print substr($0, index($0, "=") + 1) }
                        ' <<< "${bom_entries}") || return 1
                        if [ -z "${expected_versions}" ]; then
                            echo "Grace Blackwell purge would remove unowned package ${package}; retaining driver" >&2
                            return 2
                        fi
                    fi
                    state=$(graceBlackwellInstalledPackageState "${package%:arm64}" "${package_states}") || return 1
                    version="${state#*$'\t'}"
                    if [ "${state%%$'\t'*}" != installed ] || [ -z "${version}" ]; then
                        echo "unable to inspect installed version of purge package ${package}" >&2
                        return 1
                    fi
                    if [ "${owned}" != true ] && ! grep -Fxq -- "${version}" <<< "${expected_versions}"; then
                        echo "Grace Blackwell purge would remove unowned package ${package}; retaining driver" >&2
                        return 2
                    fi
                    if [ "${plan_version}" != "[${version}]" ]; then
                        echo "Grace Blackwell purge version does not match installed package ${package}; retaining driver" >&2
                        return 2
                    fi
                    ;;
            esac
        done <<< "${simulation}"
        for candidate in "${purge_packages[@]}"; do
            grep -Eq "^(Remv|Purg) ${candidate}(:arm64)?([[:space:]]|$)" <<< "${simulation}" || {
                echo "unable to verify Grace Blackwell purge of ${candidate}" >&2
                return 1
            }
        done
    fi

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

# Sourced during Ubuntu PIS basePrep after the CSE helpers and distro installer:
# GPU_DEST, managedGPUPackageList, cleanUpGPUDrivers, apt_get_purge,
# systemctlDisableAndStop, and systemctl_stop must be available.
# Unfamiliar intact drivers bypass all cleanup. Once teardown starts, any remaining
# driver artifact or inspection failure blocks the basePrep marker.
cleanUpGPUDriversForBasePrep() {
    local packageName marker="${GPU_DKMS_MARKER_FILE:-/opt/azure/aks-gpu/dkms-marker}" remaining_package
    local peermem_config="${GB_NVIDIA_PEERMEM_CONFIG_FILE:-/etc/modules-load.d/nvidia-peermem.conf}"
    local bom="${GB_MAI_BOM_FILE:-/opt/azure/containers/gb-mai-bom.json}"
    local legacy_bom="${GB200_MAI_BOM_FILE:-/opt/azure/containers/gb200-mai-bom.json}"
    local pending_file="${GB_DRIVER_CLEANUP_PENDING_FILE:-/opt/azure/containers/gb-driver-cleanup.pending}"
    local wave=versions-wave2 entries gb_layout=false pending_teardown=false pending_wave result

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
    elif [ -e "${legacy_bom}" ] && [ ! -e "${bom}" ]; then
        bom="${legacy_bom}"
        wave=versions-wave1
        gb_layout=true
    elif [ -e "${peermem_config}" ] || [ -L "${peermem_config}" ] ||
        [ -e "${bom}" ] || [ -L "${bom}" ]; then
        gb_layout=true
    fi

    if [ "${gb_layout}" = true ]; then
        entries=$(graceBlackwellDriverBOMEntries "${bom}" "${wave}")
        result=$?
        if [ "${result}" -eq 2 ] && [ "${pending_teardown}" != true ]; then
            echo "WARNING: retaining GPU driver and artifacts: Grace Blackwell BOM ownership is unavailable" >&2
            return 0
        elif [ "${result}" -ne 0 ]; then
            echo "GPU basePrep cleanup incomplete: Grace Blackwell BOM is unavailable" >&2
            return 1
        fi
        cleanUpGraceBlackwellGPUDriver "${entries}" "${wave}" "${pending_teardown}" "${bom}"
        result=$?
        if [ "${result}" -eq 2 ] && [ "${pending_teardown}" != true ]; then
            echo "WARNING: retaining GPU driver and artifacts: Grace Blackwell teardown ownership or safety is unverified" >&2
            return 0
        elif [ "${result}" -ne 0 ]; then
            echo "GPU basePrep cleanup incomplete: Grace Blackwell driver teardown failed" >&2
            return 1
        fi
        remaining_package=$(graceBlackwellPackageManagedDriverRemains)
        result=$?
        if [ "${result}" -eq 2 ]; then
            echo "${remaining_package}" >&2
            return 1
        elif [ "${result}" -eq 0 ]; then
            echo "GPU basePrep cleanup incomplete: NVIDIA package ${remaining_package} remains after Grace Blackwell teardown" >&2
            return 1
        fi
    else
        remaining_package=$(graceBlackwellPackageManagedDriverRemains)
        result=$?
        if [ "${result}" -eq 2 ]; then
            echo "${remaining_package}" >&2
            return 1
        elif [ "${result}" -eq 0 ]; then
            echo "WARNING: retaining GPU driver and artifacts: NVIDIA package ${remaining_package} has no verifiable GB BOM ownership" >&2
            return 0
        fi
        if [ -L "${marker}" ] || { [ -e "${marker}" ] && [ ! -f "${marker}" ]; }; then
            echo "WARNING: retaining GPU driver and artifacts: CUDA prebake marker is not an owned regular file" >&2
            return 0
        fi
        if [ ! -f "${marker}" ]; then
            prebakedGPUDriverArtifactsRemain
            result=$?
            [ "${result}" -ne 2 ] || return 1
            if [ "${result}" -eq 0 ]; then
                echo "WARNING: retaining GPU driver and artifacts: CUDA prebake ownership is unavailable" >&2
                return 0
            fi
        fi
    fi

    cleanUpGPUDrivers || {
        echo "GPU basePrep cleanup incomplete: AKS driver artifact removal failed" >&2
        return 1
    }
    prebakedGPUDriverArtifactsRemain
    result=$?
    [ "${result}" -ne 2 ] || return 1
    if [ "${result}" -eq 0 ]; then
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
