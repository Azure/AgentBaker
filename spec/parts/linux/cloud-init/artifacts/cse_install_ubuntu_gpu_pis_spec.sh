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

    BeforeEach 'setup_cleanup'
    AfterEach 'rm -rf "$fixture"'

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
        When call cleanUpGraceBlackwellGPUDriver 'nvidia-driver-580-open=580.159.04-1ubuntu1' "${fixture}/bom" versions-wave2
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
