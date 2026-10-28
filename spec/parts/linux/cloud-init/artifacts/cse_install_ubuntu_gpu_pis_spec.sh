#!/bin/bash

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
    }

    gb_entries() {
        printf '%s\n' 'nvidia-dkms-580-open=580.159.04-1ubuntu1' \
            'nvidia-driver-580-open=580.159.04-1ubuntu1' \
            'libnvidia-compute-580=580.159.04-1ubuntu1'
    }

    setup_gb_package_mocks() {
        dpkg-query() {
            if [ "$#" -eq 2 ]; then
                [ "${GB_TEST_INSPECTION_ERROR:-false}" = true ] && return 1
                printf '%s\tinstalled\n' nvidia-dkms-580-open nvidia-driver-580-open \
                    libnvidia-compute-580 nvidia-utils-580 nvidia-modprobe
                return
            fi
            case "${@: -1}" in
                nvidia-dkms-580-open|libnvidia-compute-580)
                    printf 'installed\t580.159.04-1ubuntu1\n' ;;
                nvidia-driver-580-open)
                    printf 'installed\t%s\n' "${GB_TEST_DRIVER_VERSION:-580.159.04-1ubuntu1}" ;;
                nvidia-utils-580)
                    printf 'installed\t580.159.04-1ubuntu2\n' ;;
                nvidia-modprobe)
                    printf 'installed\t580.159.04-1ubuntu3\n' ;;
                *)
                    return 1 ;;
            esac
        }
        apt-mark() {
            [ "$1" = showauto ] || return 1
            if [ "${GB_TEST_NONAUTO:-false}" != true ]; then
                printf 'nvidia-utils-580\n'
            fi
            printf 'nvidia-modprobe\n'
        }
        systemctlDisableAndStop() { touch "${fixture}/teardown"; }
        apt_get_purge() { touch "${fixture}/teardown"; }
    }

    stage_gb_configs() {
        cp parts/linux/cloud-init/artifacts/ubuntu/gb/nvidia-peermem.conf "${GB_NVIDIA_PEERMEM_CONFIG_FILE}"
        cp parts/linux/cloud-init/artifacts/ubuntu/gb/modprobe-nvidia-parameters.conf "${GB_NVIDIA_MODPROBE_CONFIG_FILE}"
        printf 'blacklist nouveau\noptions nouveau modeset=0\n' > "${GB_NOUVEAU_MODPROBE_CONFIG_FILE}"
    }

    BeforeEach 'setup_cleanup'
    AfterEach 'rm -rf "$fixture"'

    It 'selects only BOM-matched packages and verified auto-installed dependencies'
        setup_gb_package_mocks
        When call graceBlackwellInstalledDriverPackages "$(gb_entries)"
        The status should be success
        The output should equal "$(printf '%s\n' \
            'nvidia-dkms-580-open=580.159.04-1ubuntu1' \
            'nvidia-driver-580-open=580.159.04-1ubuntu1' \
            'libnvidia-compute-580=580.159.04-1ubuntu1' \
            'nvidia-utils-580=580.159.04-1ubuntu2' \
            'nvidia-modprobe=580.159.04-1ubuntu3')"
    End

    It 'rejects a BOM version mismatch before teardown'
        setup_gb_package_mocks
        GB_TEST_DRIVER_VERSION=580.159.04-1ubuntu9
        When call cleanUpGraceBlackwellGPUDriver "$(gb_entries)" versions-wave2
        The status should be failure
        The stderr should include 'does not match the GB BOM'
        The path "${fixture}/teardown" should not be exist
        The path "${GB_DRIVER_CLEANUP_PENDING_FILE}" should not be exist
    End

    It 'rejects an unverified dependency before teardown'
        setup_gb_package_mocks
        GB_TEST_NONAUTO=true
        When call cleanUpGraceBlackwellGPUDriver "$(gb_entries)" versions-wave2
        The status should be failure
        The stderr should include 'nvidia-utils-580 is not an auto-installed package'
        The path "${fixture}/teardown" should not be exist
        The path "${GB_DRIVER_CLEANUP_PENDING_FILE}" should not be exist
    End

    It 'fails closed when installed package inspection fails'
        setup_gb_package_mocks
        GB_TEST_INSPECTION_ERROR=true
        When call cleanUpGraceBlackwellGPUDriver "$(gb_entries)" versions-wave2
        The status should be failure
        The stderr should include 'unable to inspect installed NVIDIA driver packages'
        The path "${fixture}/teardown" should not be exist
    End

    It 'accepts the checked-in GB configuration templates without weakening exact matching'
        setup_gb_package_mocks
        stage_gb_configs
        systemctlDisableAndStop() { :; }
        systemctl() { return 1; }
        lsmod() { printf 'Module Size Used by\n'; }
        apt_get_purge() { printf '%s\n' "$@" > "${fixture}/purged"; }
        update-initramfs() { :; }
        When call cleanUpGraceBlackwellGPUDriver "$(gb_entries)" versions-wave2
        The status should be success
        The contents of file "${fixture}/purged" should equal "$(printf '%s\n' 10 5 300 \
            nvidia-dkms-580-open nvidia-driver-580-open libnvidia-compute-580 \
            nvidia-utils-580 nvidia-modprobe)"
        The path "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" should not be exist
        The path "${GB_NVIDIA_MODPROBE_CONFIG_FILE}" should not be exist
        The path "${GB_DRIVER_CLEANUP_PENDING_FILE}" should not be exist
    End

    It 'rejects changed GB configuration before teardown'
        setup_gb_package_mocks
        stage_gb_configs
        printf '# customer edit\n' >> "${GB_NVIDIA_PEERMEM_CONFIG_FILE}"
        When call cleanUpGraceBlackwellGPUDriver "$(gb_entries)" versions-wave2
        The status should be failure
        The stderr should include 'was modified; refusing teardown'
        The path "${fixture}/teardown" should not be exist
        The path "${GB_NVIDIA_PEERMEM_CONFIG_FILE}" should be exist
    End

    It 'fails CUDA cleanup when the prebake marker remains'
        touch "${GPU_DKMS_MARKER_FILE}"
        dpkg-query() { :; }
        lsmod() { :; }
        cleanUpGPUDrivers() { :; }
        prebakedGPUDriverArtifactsRemain() { return 1; }
        When call cleanUpGPUDriversForBasePrep
        The status should be failure
        The stderr should include "AKS driver artifacts remain"
        The path "${GPU_DKMS_MARKER_FILE}" should be exist
    End

    It 'refuses GB teardown when installed NVIDIA packages are not BOM-owned'
        touch "${GPU_DKMS_MARKER_FILE}"
        graceBlackwellInstalledDriverPackages() { printf '%s\n' 'nvidia-driver-580-open=580.159.04-1ubuntu1'; }
        dpkg-query() { printf 'nvidia-driver-580-open\tinstalled\nnvidia-driver-535\tinstalled\n'; }
        When call cleanUpGraceBlackwellGPUDriver 'nvidia-driver-580-open=580.159.04-1ubuntu1' versions-wave2
        The status should be failure
        The stderr should include 'NVIDIA package nvidia-driver-535 is not owned by the Grace Blackwell BOM'
        The path "${GPU_DKMS_MARKER_FILE}" should be exist
    End

    It 'retains GB cleanup provenance after initramfs failure and retries'
        cp parts/linux/cloud-init/artifacts/ubuntu/gb/nvidia-peermem.conf "${GB_NVIDIA_PEERMEM_CONFIG_FILE}"
        cp parts/linux/cloud-init/artifacts/ubuntu/gb/modprobe-nvidia-parameters.conf "${GB_NVIDIA_MODPROBE_CONFIG_FILE}"
        printf 'blacklist nouveau\noptions nouveau modeset=0\n' > "${GB_NOUVEAU_MODPROBE_CONFIG_FILE}"
        touch "${GPU_DKMS_MARKER_FILE}"
        graceBlackwellDriverBOMEntries() { printf '%s\n' 'nvidia-driver-580-open=580.159.04-1ubuntu1'; }
        graceBlackwellInstalledDriverPackages() { :; }
        graceBlackwellPackageManagedDriverRemains() { return 1; }
        systemctlDisableAndStop() { :; }
        systemctl() { return 1; }
        lsmod() { printf 'Module Size Used by\n'; }
        update_attempts=0
        update-initramfs() {
            update_attempts=$((update_attempts + 1))
            [ "${update_attempts}" -gt 1 ]
        }
        cleanUpGPUDrivers() { rm -f "${GPU_DKMS_MARKER_FILE}"; }
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
