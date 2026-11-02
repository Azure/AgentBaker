#!/bin/bash
# ShellSpec invokes mocks indirectly and evaluates quoted hooks.
# shellcheck disable=SC2016,SC2317

Describe 'Ubuntu PIS GPU basePrep cleanup'
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu_gpu.sh"
    fixture="${PWD}/.shellspec-gpu-cleanup-$$"
    GPU_DEST="${fixture}/gpu-dest"
    managedGPUPackageList() { :; }

    setup_cleanup() {
        mkdir -p "${fixture}"
        GPU_DKMS_MARKER_FILE="${fixture}/marker"
        GB_NVIDIA_PEERMEM_CONFIG_FILE="${fixture}/peermem.conf"
        GB_NVIDIA_MODPROBE_CONFIG_FILE="${fixture}/nvidia.conf"
        GB_NOUVEAU_MODPROBE_CONFIG_FILE="${fixture}/nouveau.conf"
        GB_DRIVER_CLEANUP_PENDING_FILE="${fixture}/cleanup.pending"
        GB_MAI_BOM_FILE="${fixture}/bom.json"
        export GB200_MAI_BOM_FILE="${fixture}/legacy-bom.json"
        cp vhdbuilder/packer/gb-mai-bom.json "${GB_MAI_BOM_FILE}"
        mkdir -p "${GPU_DEST}"
        touch "${GPU_DEST}/customer-file"
        lsmod() { printf 'Module Size Used by\n'; }
        cleanUpGPUDrivers() { rm -rf "${GPU_DEST}"; rm -f "${GPU_DKMS_MARKER_FILE}"; }
    }

    gb_entries() {
        printf '%s\n' 'nvidia-dkms-580-open=580.159.04-1ubuntu1' \
            'nvidia-driver-580-open=580.159.04-1ubuntu1' \
            'libnvidia-compute-580=580.159.04-1ubuntu1'
    }

    gb_dependency_fixture() {
        awk -v package="${1}" '
            /^Package: / { selected = ($2 == package) }
            selected && /^(Package|Version|Depends): / { print }
        ' spec/fixtures/nvidia-gb-cleanup.packages
    }

    setup_gb_package_mocks() {
        dpkg-query() {
            [ "${GB_TEST_INSPECTION_ERROR:-false}" = true ] && return 1
            local package version
            if [ "${GB_TEST_PURGED:-false}" != true ]; then
                for package in nvidia-dkms-580-open nvidia-driver-580-open libnvidia-compute-580 \
                    nvidia-utils-580 nvidia-modprobe; do
                    case "${package}" in
                        nvidia-driver-580-open) version="${GB_TEST_DRIVER_VERSION:-580.159.04-1ubuntu1}" ;;
                        nvidia-utils-580) version=580.159.04-1ubuntu2 ;;
                        nvidia-modprobe) version=580.159.04-1ubuntu3 ;;
                        *) version=580.159.04-1ubuntu1 ;;
                    esac
                    printf '%s\tinstalled\t%s\n' "${package}" "${version}"
                done
            fi
            if [ "${GB_TEST_EGL:-false}" = true ]; then
                if [ "${GB_TEST_PURGED:-false}" != true ]; then
                    printf 'libnvidia-gl-580\tinstalled\t580.159.04-1ubuntu1\n'
                fi
                printf 'libnvidia-egl-gbm1\tinstalled\t1.1.2.1-0ubuntu1\n'
                printf 'libnvidia-egl-wayland1\tinstalled\t1:1.1.18-0ubuntu1\n'
                printf '%s\tinstalled\t1.0.0-0ubuntu1\n' libnvidia-egl-xcb1 libnvidia-egl-xlib1
            fi
            if [ "${GB_TEST_UNKNOWN:-false}" = true ]; then
                printf 'nvidia-driver-535\tinstalled\t535.1\n'
            fi
            if [ "${GB_TEST_PURGED:-false}" != true ]; then
                for package in ${GB_TEST_DEPENDENTS:-}; do
                    version=$(gb_dependency_fixture "${package}" | awk '/^Version: / { print $2 }')
                    [ "${GB_TEST_MISSING_VERSION:-false}" != true ] || version=""
                    printf '%s\tinstalled\t%s\n' "${package}" "${GB_TEST_DEPENDENT_VERSION:-${version}}"
                done
                [ "${GB_TEST_SETTINGS:-false}" != true ] || printf 'libxnvctrl0\tinstalled\t580.159.04-1ubuntu1\n'
            fi
            return 0
        }
        apt-mark() {
            [ "${GB_TEST_APT_MARK_ERROR:-false}" = true ] && return 1
            [ "$1" = showauto ] || return 1
            if [ "${GB_TEST_NONAUTO:-false}" != true ]; then
                printf 'nvidia-utils-580\n'
            fi
            printf 'nvidia-modprobe\n'
            [ "${GB_TEST_SETTINGS_AUTO:-false}" != true ] || printf 'nvidia-settings\n'
            return 0
        }
        systemctlDisableAndStop() { touch "${fixture}/teardown"; }
        systemctl() { return 1; }
        apt-get() {
            [ "${GB_TEST_APT_ERROR:-false}" = true ] && return 1
            [ "${LC_ALL}" = C ] || return 1
            [ "$1" = -s ] && [ "$2" = purge ] && [ "$3" = -- ] || return 1
            shift 3
            local package version dependency dependencies candidate
            for package in "$@"; do
                version=$(dpkg-query | awk -F '\t' -v package="${package}" '$1 == package { print $3 }')
                printf 'Purg %s [%s]\n' "${package}" "${version}"
            done
            # Model reverse removals from the pinned package Depends, not just the purge arguments.
            for package in ${GB_TEST_DEPENDENTS:-}; do
                case " $* " in
                    *" ${package} "*) continue ;;
                esac
                dependencies=$(gb_dependency_fixture "${package}" | sed -n 's/^Depends: //p' | tr ',' '\n')
                while read -r dependency _; do
                    for candidate in "$@"; do
                        [ "${dependency}" = "${candidate}" ] || continue
                        version=$(gb_dependency_fixture "${package}" | awk '/^Version: / { print $2 }')
                        printf 'Remv %s%s [%s]\n' "${package}" "${GB_TEST_PLAN_ARCH:-}" \
                            "${GB_TEST_PLAN_VERSION:-${GB_TEST_DEPENDENT_VERSION:-${version}}}"
                        break 2
                    done
                done <<< "${dependencies}"
            done
            if [ "${GB_TEST_REVERSE_DEPENDENCY:-false}" = true ]; then
                printf 'Remv customer-package [1.0]\n'
            fi
            [ -z "${GB_TEST_APT_ACTION:-}" ] || printf '%s customer-package (1.0)\n' "${GB_TEST_APT_ACTION}"
            return 0
        }
        apt_get_purge() {
            printf '%s\n' "$@" > "${fixture}/purged"
            GB_TEST_PURGED=true
        }
        jq() {
            if [ "${GB_TEST_PURGE_BOM_ERROR:-false}" = true ] && [ "${1:-}" = -r ]; then
                return 2
            fi
            command jq "$@"
        }
        update-initramfs() { :; }
    }

    stage_gb_configs() {
        cp parts/linux/cloud-init/artifacts/ubuntu/gb/nvidia-peermem.conf "${GB_NVIDIA_PEERMEM_CONFIG_FILE}"
        cp parts/linux/cloud-init/artifacts/ubuntu/gb/modprobe-nvidia-parameters.conf "${GB_NVIDIA_MODPROBE_CONFIG_FILE}"
        printf 'blacklist nouveau\noptions nouveau modeset=0\n' > "${GB_NOUVEAU_MODPROBE_CONFIG_FILE}"
    }

    setup_all_cleanup() {
        setup_cleanup
        setup_gb_package_mocks
        stage_gb_configs
    }

    BeforeEach 'setup_all_cleanup'
    AfterEach 'rm -rf "$fixture"'

    It 'selects only BOM-matched packages and verified auto-installed dependencies'
        When call graceBlackwellInstalledDriverPackages "$(gb_entries)"
        The status should be success
        The output should equal "$(printf '%s\n' \
            'nvidia-dkms-580-open=580.159.04-1ubuntu1' \
            'nvidia-driver-580-open=580.159.04-1ubuntu1' \
            'libnvidia-compute-580=580.159.04-1ubuntu1' \
            'nvidia-utils-580=580.159.04-1ubuntu2' \
            'nvidia-modprobe=580.159.04-1ubuntu3')"
    End

    It 'accepts the checked-in GB configuration templates without weakening exact matching'
        When call cleanUpGraceBlackwellGPUDriver "$(gb_entries)" versions-wave2
        The status should be success
        The contents of file "${fixture}/purged" should equal "$(printf '%s\n' 10 5 300 \
            nvidia-dkms-580-open nvidia-driver-580-open libnvidia-compute-580 \
            nvidia-utils-580 nvidia-modprobe)"
        The path "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" should not be exist
        The path "${GB_NVIDIA_MODPROBE_CONFIG_FILE}" should not be exist
        The path "${GB_DRIVER_CLEANUP_PENDING_FILE}" should not be exist
    End

    It 'fails CUDA cleanup when the prebake marker remains'
        touch "${GPU_DKMS_MARKER_FILE}"
        dpkg-query() { :; }
        lsmod() { :; }
        cleanUpGPUDrivers() { :; }
        prebakedGPUDriverArtifactsRemain() { return 1; }
        rm -f "${GB_MAI_BOM_FILE}" "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" "${GB_NVIDIA_MODPROBE_CONFIG_FILE}" "${GB_NOUVEAU_MODPROBE_CONFIG_FILE}"
        When call cleanUpGPUDriversForBasePrep
        The status should be failure
        The stderr should include "AKS driver artifacts remain"
        The path "${GPU_DKMS_MARKER_FILE}" should be exist
    End

    It 'retains all four EGL support libraries while cleaning the known driver'
        GB_TEST_EGL=true
        When call cleanUpGPUDriversForBasePrep
        The status should be success
        The stderr should include 'retaining NVIDIA EGL support package libnvidia-egl-gbm1'
        The stderr should include 'retaining NVIDIA EGL support package libnvidia-egl-wayland1'
        The stderr should include 'retaining NVIDIA EGL support package libnvidia-egl-xcb1'
        The stderr should include 'retaining NVIDIA EGL support package libnvidia-egl-xlib1'
        The contents of file "${fixture}/purged" should equal "$(printf '%s\n' 10 5 300 \
            libnvidia-gl-580 nvidia-dkms-580-open libnvidia-compute-580 nvidia-driver-580-open \
            nvidia-utils-580 nvidia-modprobe)"
        The path "${GB_NVIDIA_MODPROBE_CONFIG_FILE}" should not be exist
        The path "${GPU_DEST}" should not be exist
    End

    It 'selects and cleans the exact auto-installed nvidia-settings dependency'
        GB_TEST_DEPENDENTS=nvidia-settings
        GB_TEST_SETTINGS=true
        GB_TEST_SETTINGS_AUTO=true
        When call cleanUpGPUDriversForBasePrep
        The status should be success
        The stderr should equal ''
        The contents of file "${fixture}/purged" should equal "$(printf '%s\n' 10 5 300 \
            libxnvctrl0 nvidia-dkms-580-open libnvidia-compute-580 nvidia-driver-580-open \
            nvidia-utils-580 nvidia-modprobe nvidia-settings)"
        The path "${GPU_DEST}" should not be exist
    End

    Describe 'intact preflight retention'
        prepare_retention_case() {
            touch "${GPU_DKMS_MARKER_FILE}"
            case "${1}" in
                driver-version) GB_TEST_DRIVER_VERSION=580.159.04-1ubuntu9 ;;
                manual-dependency) GB_TEST_NONAUTO=true ;;
                modified-config) printf '# customer edit\n' >> "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" ;;
                unowned-driver) GB_TEST_UNKNOWN=true ;;
                unowned-reverse-dependency) GB_TEST_REVERSE_DEPENDENCY=true ;;
                imex-version)
                    GB_TEST_DEPENDENTS=nvidia-imex
                    GB_TEST_DEPENDENT_VERSION=580.159.04-1ubuntu9 ;;
                imex-without-BOM-entry)
                    GB_TEST_DEPENDENTS=nvidia-imex
                    command jq 'del(."versions-wave3"."nvidia-imex")' "${GB_MAI_BOM_FILE}" > "${fixture}/changed-bom.json"
                    mv "${fixture}/changed-bom.json" "${GB_MAI_BOM_FILE}" ;;
                manual-nvidia-settings)
                    GB_TEST_SETTINGS=true
                    GB_TEST_DEPENDENTS=nvidia-settings ;;
                apt-install) GB_TEST_APT_ACTION=Inst ;;
                apt-configure) GB_TEST_APT_ACTION=Conf ;;
                plan-version)
                    GB_TEST_DEPENDENTS=nvidia-imex
                    GB_TEST_PLAN_VERSION=580.159.04-1ubuntu9 ;;
                missing-BOM) rm -f "${GB_MAI_BOM_FILE}" ;;
                unfamiliar-BOM) printf '{"versions-wave2":{"customer-driver":"1.0"}}\n' > "${GB_MAI_BOM_FILE}" ;;
                unfamiliar-BOM-package)
                    printf '%s\n' '{"versions-wave2":{"nvidia-dkms-580-open":"580.159.04-1ubuntu1","nvidia-driver-580-open":"580.159.04-1ubuntu1","libnvidia-customer-library":"1.0"}}' > "${GB_MAI_BOM_FILE}" ;;
            esac
        }
        Parameters
            'driver-version' 'does not match the GB BOM'
            'manual-dependency' 'nvidia-utils-580 is not an auto-installed package'
            'modified-config' 'was modified'
            'unowned-driver' 'NVIDIA package nvidia-driver-535 is not owned by the Grace Blackwell BOM'
            'unowned-reverse-dependency' 'purge would remove unowned package customer-package'
            'imex-version' 'purge would remove unowned package nvidia-imex'
            'imex-without-BOM-entry' 'purge would remove unowned package nvidia-imex'
            'manual-nvidia-settings' 'nvidia-settings is not an auto-installed package'
            'apt-install' 'non-removal action Inst'
            'apt-configure' 'non-removal action Conf'
            'plan-version' 'purge version does not match installed package nvidia-imex'
            'missing-BOM' 'Grace Blackwell BOM ownership is unavailable'
            'unfamiliar-BOM' 'Grace Blackwell BOM ownership is unavailable'
            'unfamiliar-BOM-package' 'Grace Blackwell BOM ownership is unavailable'
        End
        It "retains $1 without starting teardown"
            prepare_retention_case "$1"
            When call cleanUpGPUDriversForBasePrep
            The status should be success
            The stderr should include "$2"
            The stderr should include 'WARNING'
            The path "${fixture}/teardown" should not be exist
            The path "${fixture}/purged" should not be exist
            The path "${GB_DRIVER_CLEANUP_PENDING_FILE}" should not be exist
            The path "${GPU_DEST}/customer-file" should be exist
            The path "${GPU_DKMS_MARKER_FILE}" should be exist
            The path "${GB_NVIDIA_MODPROBE_CONFIG_FILE}" should be exist
            The path "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" should be exist
        End
    End

    Describe 'owned IMEX dependency closure'
        Parameters
            'current-wave2' ':arm64'
            'legacy-wave1' ''
        End
        It "cleans the pinned dependency using $1"
            GB_TEST_DEPENDENTS=nvidia-imex
            GB_TEST_PLAN_ARCH="$2"
            if [ "$1" = legacy-wave1 ]; then
                command jq '."versions-wave1" = ."versions-wave2" | del(."versions-wave2")' \
                    "${GB_MAI_BOM_FILE}" > "${GB200_MAI_BOM_FILE}"
                rm -f "${GB_MAI_BOM_FILE}"
            fi
            When call cleanUpGPUDriversForBasePrep
            The status should be success
            The stderr should equal ''
            The path "${fixture}/purged" should be exist
            The path "${GB_NVIDIA_MODPROBE_CONFIG_FILE}" should not be exist
            The path "${GPU_DEST}" should not be exist
        End
    End

    Describe 'preflight inspection errors'
        prepare_inspection_error() {
            case "${1}" in
                dpkg) GB_TEST_INSPECTION_ERROR=true ;;
                apt) GB_TEST_APT_ERROR=true ;;
                apt-mark) GB_TEST_APT_MARK_ERROR=true ;;
                config) cmp() { return 2; } ;;
                invalid-BOM) printf 'not JSON\n' > "${GB_MAI_BOM_FILE}" ;;
                missing-imex-version)
                    GB_TEST_DEPENDENTS=nvidia-imex
                    GB_TEST_MISSING_VERSION=true ;;
                purge-BOM)
                    GB_TEST_DEPENDENTS=nvidia-imex
                    GB_TEST_PURGE_BOM_ERROR=true ;;
            esac
        }
        Parameters
            'dpkg' 'unable to inspect installed NVIDIA driver packages'
            'apt' 'unable to inspect Grace Blackwell purge'
            'apt-mark' 'unable to inspect auto-installed NVIDIA packages'
            'config' 'unable to inspect Grace Blackwell module configuration'
            'invalid-BOM' 'unable to inspect Grace Blackwell BOM'
            'missing-imex-version' 'unable to inspect installed version of purge package nvidia-imex'
            'purge-BOM' 'unable to inspect Grace Blackwell purge ownership'
        End
        It "fails closed on $1 without starting teardown"
            prepare_inspection_error "$1"
            When call cleanUpGPUDriversForBasePrep
            The status should be failure
            The stderr should include "$2"
            The path "${fixture}/teardown" should not be exist
            The path "${GPU_DEST}/customer-file" should be exist
        End
    End

    Describe 'post-start removal errors'
        prepare_teardown_error() {
            case "${1}" in
                purge) apt_get_purge() { return 1; } ;;
                module)
                    lsmod() { printf 'nvidia 123 0\n'; }
                    rmmod() { return 1; } ;;
                artifacts) cleanUpGPUDrivers() { return 1; } ;;
                initramfs) update-initramfs() { return 1; } ;;
                package-residue) apt_get_purge() { touch "${fixture}/purged"; } ;;
            esac
        }
        Parameters
            'purge' 'driver package purge failed'
            'module' 'Failed to unload Grace Blackwell module nvidia'
            'artifacts' 'AKS driver artifact removal failed'
            'initramfs' 'Grace Blackwell driver teardown failed'
            'package-residue' 'remains after Grace Blackwell teardown'
        End
        It "fails on $1 after teardown starts"
            prepare_teardown_error "$1"
            When call cleanUpGPUDriversForBasePrep
            The status should be failure
            The stderr should include "$2"
            The path "${fixture}/teardown" should be exist
            if [ "$1" = initramfs ] || [ "$1" = package-residue ]; then
                The path "${fixture}/purged" should be exist
            fi
            if [ "$1" = initramfs ]; then
                The path "${GB_DRIVER_CLEANUP_PENDING_FILE}" should be exist
            fi
        End
    End

    It 'fails closed when loaded modules cannot be inspected'
        dpkg-query() { :; }
        lsmod() { return 1; }
        rm -f "${GB_MAI_BOM_FILE}" "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" "${GB_NVIDIA_MODPROBE_CONFIG_FILE}" "${GB_NOUVEAU_MODPROBE_CONFIG_FILE}"
        When call cleanUpGPUDriversForBasePrep
        The status should be failure
        The stderr should include 'unable to inspect loaded NVIDIA modules'
        The path "${GPU_DEST}/customer-file" should be exist
    End

    It 'retains unmarked CUDA artifacts without invoking generic cleanup'
        dpkg-query() { :; }
        rm -f "${GB_MAI_BOM_FILE}" "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" "${GB_NVIDIA_MODPROBE_CONFIG_FILE}" "${GB_NOUVEAU_MODPROBE_CONFIG_FILE}"
        prebakedGPUDriverArtifactsRemain() { return 0; }
        cleanUpGPUDrivers() { touch "${fixture}/teardown"; }
        When call cleanUpGPUDriversForBasePrep
        The status should be success
        The stderr should include 'WARNING'
        The stderr should include 'CUDA prebake ownership is unavailable'
        The path "${fixture}/teardown" should not be exist
        The path "${GPU_DEST}/customer-file" should be exist
    End

    It 'retains customer-owned config symlinks without deleting their targets'
        mv "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" "${fixture}/customer-config"
        ln -s "${fixture}/customer-config" "${GB_NVIDIA_PEERMEM_CONFIG_FILE}"
        When call cleanUpGPUDriversForBasePrep
        The status should be success
        The stderr should include 'WARNING'
        The path "${fixture}/teardown" should not be exist
        The path "${fixture}/customer-config" should be exist
        The path "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" should be symlink
    End

    It 'does not convert unfamiliar state into success on a pending teardown'
        printf 'versions-wave2\n' > "${GB_DRIVER_CLEANUP_PENDING_FILE}"
        GB_TEST_UNKNOWN=true
        When call cleanUpGPUDriversForBasePrep
        The status should be failure
        The stderr should include 'Grace Blackwell driver teardown failed'
        The path "${GB_DRIVER_CLEANUP_PENDING_FILE}" should be exist
        The path "${GPU_DEST}/customer-file" should be exist
    End

    It 'retains GB cleanup provenance after initramfs failure and retries'
        touch "${GPU_DKMS_MARKER_FILE}"
        graceBlackwellDriverBOMEntries() { printf '%s\n' 'nvidia-driver-580-open=580.159.04-1ubuntu1'; }
        graceBlackwellInstalledDriverPackages() { :; }
        graceBlackwellPackageManagedDriverRemains() { return 1; }
        dpkg-query() { :; }
        systemctlDisableAndStop() { :; }
        systemctl() { return 1; }
        lsmod() { printf 'Module Size Used by\n'; }
        update_attempts=0
        update-initramfs() {
            update_attempts=$((update_attempts + 1))
            [ "${update_attempts}" -gt 1 ]
        }
        cleanUpGPUDrivers() { rm -f "${GPU_DKMS_MARKER_FILE}"; rm -rf "${GPU_DEST}"; }
        prebakedGPUDriverArtifactsRemain() { return 1; }
        retry_cleanup() {
            local status=0 pending_wave
            cleanUpGPUDriversForBasePrep > "${fixture}/first-attempt.log" 2>&1 && status=1
            grep -q 'GPU basePrep cleanup incomplete: Grace Blackwell driver teardown failed' "${fixture}/first-attempt.log" || status=1
            IFS= read -r pending_wave < "${GB_DRIVER_CLEANUP_PENDING_FILE}" || status=1
            [ "${pending_wave}" = versions-wave2 ] || status=1
            cleanUpGPUDriversForBasePrep || status=1
            [ ! -e "${GB_DRIVER_CLEANUP_PENDING_FILE}" ] || status=1
            [ ! -e "${GPU_DKMS_MARKER_FILE}" ] || status=1
            return "${status}"
        }
        When call retry_cleanup
        The status should be success
    End
End
