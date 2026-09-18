#!/bin/bash
# shellcheck disable=SC2218,SC2329 # ShellSpec installs per-example mocks dynamically.

Describe 'whole NVIDIA prebake payload'
    setup_payload() {
        TEST_ROOT=$(mktemp -d)
        GPU_DKMS_MARKER_FILE="${TEST_ROOT}/opt/azure/aks-gpu/dkms-marker"
        PAYLOAD="${GPU_DKMS_MARKER_FILE%/*}/prebake"
        LIVE="${TEST_ROOT}/var/lib/dkms/nvidia"
        KERNEL=6.8.0-1065-azure
        RUNNING_KERNEL=${KERNEL}
        NVIDIA_DRIVER_IMAGE_TAG=580.159.04-test
        MODULE="${TEST_ROOT}/lib/modules/${KERNEL}/updates/dkms/nvidia.ko"
        BINARY="${TEST_ROOT}/usr/bin/nvidia-modprobe"
        LIBRARY="${TEST_ROOT}/usr/bin/lib64/lib64/libnvidia-ml.so.580.159.04"
        mkdir -p "${LIVE}/580.159.04/${KERNEL}/x86_64/module" "${MODULE%/*}" \
            "${GPU_DKMS_MARKER_FILE%/*}" "${LIBRARY%/*}" "${TEST_ROOT}/etc/ld.so.conf.d" \
            "${TEST_ROOT}/var/lib/nvidia" "${TEST_ROOT}/usr/src/nvidia-580.159.04" \
            "${TEST_ROOT}/usr/lib/x86_64-linux-gnu" "${TEST_ROOT}/usr/share/glvnd/egl_vendor.d" \
            "${TEST_ROOT}/boot"
        printf 'kernel=%s\ndriver_version=580.159.04\ndriver_kind=cuda\narch=x86_64\n' "${KERNEL}" > "${GPU_DKMS_MARKER_FILE}"
        printf 'prebuilt module\n' > "${LIVE}/580.159.04/${KERNEL}/x86_64/module/nvidia.ko"
        printf 'signed module\n' > "${MODULE}"
        printf 'driver binary\n' > "${BINARY}"
        chmod 4755 "${BINARY}"
        printf 'driver library\n' > "${LIBRARY}"
        printf 'OS library\n' > "${TEST_ROOT}/usr/lib/x86_64-linux-gnu/libnvidia-ml.so.580.159.04"
        printf 'EGL config\n' > "${TEST_ROOT}/usr/share/glvnd/egl_vendor.d/10_nvidia.json"
        printf 'unrelated module\n' > "${MODULE%/*}/other.ko"
        printf '/usr/bin/lib64\n' > "${TEST_ROOT}/etc/ld.so.conf.d/nvidia.conf"
        ln -s "${TEST_ROOT}/usr/src/nvidia-580.159.04" "${LIVE}/580.159.04/source"
        ln -s nvidia-modprobe "${TEST_ROOT}/usr/bin/nvidia-smi"
        # Actual NVIDIA backup-log record types, including an overlay backup that must NOT
        # cause the host's original library to be moved or treated as prebake-owned.
        printf '580.159.04\nNVIDIA Driver\n1: %s\n123\n0: %s\nnvidia-modprobe\n1: %s\n456\n100: %s\n123 644 0 0\n1: %s\n789\n' \
            "${BINARY}" "${TEST_ROOT}/usr/bin/nvidia-smi" \
            "${TEST_ROOT}/usr/lib/x86_64-linux-gnu/libnvidia-ml.so.580.159.04" \
            "${TEST_ROOT}/usr/lib/x86_64-linux-gnu/libnvidia-ml.so.580.159.04" \
            "${TEST_ROOT}/usr/share/glvnd/egl_vendor.d/10_nvidia.json" > "${TEST_ROOT}/var/lib/nvidia/log"
        BINARY_ID=$(stat -c '%i:%f' "${BINARY}")
        CALLS="${TEST_ROOT}/calls"
        : > "${CALLS}"
        # Real filesystem operations; relocate only literal host roots in the extracted helpers.
        eval "$(sed -n '/^prebakedGPUDriverFiles()/,/^}/p; /^setPrebakedGPUDriverState()/,/^}/p; /^setPrebakedGPUDriverRegistration()/,/^}/p; /^cleanUpPrebakedGPUDriver()/,/^}/p' \
            parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh | \
            sed -E "s#/(usr|etc|var|lib|boot)/#${TEST_ROOT}/\1/#g")"
    }
    BeforeEach 'setup_payload'
    cleanup_payload() { command rm -rf "${TEST_ROOT}"; }
    AfterEach 'cleanup_payload'

    uname() { if [ "${1}" = -r ]; then echo "${RUNNING_KERNEL}"; else echo x86_64; fi; }
    lsmod() { :; }
    depmod() { printf 'depmod %s\n' "$*" >> "${CALLS}"; }
    ldconfig() { echo ldconfig >> "${CALLS}"; }
    lsinitramfs() { :; }
    update-initramfs() { printf 'initramfs %s\n' "$*" >> "${CALLS}"; }
    dkms() {
        [ "$*" = status ] || return 99
        if [ -d "${DKMS_EFFECTIVE_TREE:-${TEST_ROOT}/var/lib/dkms}/nvidia" ]; then
            echo 'nvidia/580.159.04, 6.8.0-1065-azure, x86_64: installed'
        fi
    }
    park_payload() { setPrebakedGPUDriverState park >/dev/null; }

    It 'parks the full inventory without moving source, OS libraries or other modules'
        When call setPrebakedGPUDriverState park
        The status should be success
        The output should include 'action=park status=completed'
        The directory "${LIVE}" should not be exist
        The file "${MODULE}" should not be exist
        The file "${BINARY}" should not be exist
        The file "${LIBRARY}" should not be exist
        The directory "${TEST_ROOT}/var/lib/nvidia" should not be exist
        The file "${TEST_ROOT}/etc/ld.so.conf.d/nvidia.conf" should not be exist
        The file "${TEST_ROOT}/usr/share/glvnd/egl_vendor.d/10_nvidia.json" should not be exist
        The contents of file "${PAYLOAD}/files${MODULE}" should equal 'signed module'
        The contents of file "${PAYLOAD}/files${LIBRARY}" should equal 'driver library'
        The contents of file "${TEST_ROOT}/usr/lib/x86_64-linux-gnu/libnvidia-ml.so.580.159.04" should equal 'OS library'
        The contents of file "${MODULE%/*}/other.ko" should equal 'unrelated module'
        The directory "${TEST_ROOT}/usr/src/nvidia-580.159.04" should be exist
        The contents of file "${GPU_DKMS_MARKER_FILE}" should include 'prebake_layout=whole-v1'
        The contents of file "${CALLS}" should include "depmod -a ${KERNEL}"
        The contents of file "${CALLS}" should include 'ldconfig'
    End

    It 'keeps privileged binaries behind a root-only cache directory'
        park_payload
        When call stat -c '%u:%a' "${PAYLOAD}"
        The status should be success
        The output should equal '0:700'
    End

    It 'restores byte-identical files, permissions, inode and source links without DKMS install'
        park_payload
        dkms() { echo 'unexpected DKMS call' >&2; return 99; }
        restore_and_inspect() {
            setPrebakedGPUDriverState restore || return 1
            stat -c '%i:%f' "${BINARY}"
            readlink "${LIVE}/580.159.04/source"
        }
        When call restore_and_inspect
        The status should be success
        The stderr should equal ''
        The line 2 of output should equal "${BINARY_ID}"
        The line 3 of output should equal "${TEST_ROOT}/usr/src/nvidia-580.159.04"
        The contents of file "${MODULE}" should equal 'signed module'
        The contents of file "${LIBRARY}" should equal 'driver library'
        The contents of file "${GPU_DKMS_MARKER_FILE}" should not include 'prebake_layout='
        The directory "${PAYLOAD}" should not be exist
    End

    It 'is a no-op after the normal installer has changed restored files'
        park_payload
        setPrebakedGPUDriverState restore >/dev/null
        printf 'installer updated binary\n' > "${BINARY}"
        depmod() { echo 'unexpected depmod' >&2; return 99; }
        When call setPrebakedGPUDriverState restore
        The status should be success
        The output should equal ''
        The contents of file "${BINARY}" should equal 'installer updated binary'
    End

    Describe 'cache mismatch'
        Parameters
            kernel
            driver_version
        End
        It 'requests the normal installer without activating an incompatible cache'
            park_payload
            if [ "$1" = kernel ]; then RUNNING_KERNEL=6.8.0-1067-azure; else NVIDIA_DRIVER_IMAGE_TAG=590.1.2-test; fi
            When call setPrebakedGPUDriverState restore
            The status should equal 2
            The output should include "event=cache_miss reason=$1"
            The directory "${LIVE}" should not be exist
            The file "${MODULE}" should not be exist
            The file "${BINARY}" should not be exist
            The directory "${PAYLOAD}" should not be exist
            The file "${GPU_DKMS_MARKER_FILE}" should not be exist
        End
    End

    It 'rejects missing payload data before activating any file'
        park_payload
        rm "${PAYLOAD}/files${BINARY}"
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'Missing NVIDIA prebake file'
        The directory "${LIVE}" should not be exist
        The file "${MODULE}" should not be exist
    End

    It 'rejects an existing destination without overwriting customer data'
        park_payload
        printf 'customer driver\n' > "${BINARY}"
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'destination already exists'
        The contents of file "${BINARY}" should equal 'customer driver'
        The directory "${LIVE}" should not be exist
    End

    It 'resumes after interruption between renames, activating DKMS last'
        park_payload
        mv() {
            [ "${3}" != "${PAYLOAD}/files${BINARY}" ] || return 1
            command mv "$@"
        }
        setPrebakedGPUDriverState restore >/dev/null || :
        unset -f mv
        When call setPrebakedGPUDriverState restore
        The status should be success
        The output should include 'action=restore status=completed'
        The contents of file "${BINARY}" should equal 'driver binary'
        The directory "${LIVE}" should be exist
        The directory "${PAYLOAD}" should not be exist
    End

    It 'leaves DKMS inactive if restoration stops before the binaries are moved'
        park_payload
        mv() {
            [ "${3}" != "${PAYLOAD}/files${BINARY}" ] || return 1
            command mv "$@"
        }
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The directory "${LIVE}" should not be exist
        The file "${PAYLOAD}/restore.started" should be exist
    End

    It 'does not accept a replaced destination on a partial restore retry'
        park_payload
        mv() {
            [ "${3}" != "${PAYLOAD}/files${BINARY}" ] || return 1
            command mv "$@"
        }
        setPrebakedGPUDriverState restore >/dev/null || :
        unset -f mv
        rm "${MODULE}"
        printf 'foreign module\n' > "${MODULE}"
        chmod 600 "${MODULE}"
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'Missing NVIDIA prebake file'
        The directory "${LIVE}" should not be exist
    End

    It 'does not discard active files after a kernel change during a partial restore'
        park_payload
        touch "${PAYLOAD}/restore.started"
        RUNNING_KERNEL=6.8.0-1067-azure
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'during a partial restore'
        The directory "${PAYLOAD}" should be exist
    End

    It 'retries a metadata failure after all files have moved'
        park_payload
        depmod() { return 1; }
        setPrebakedGPUDriverState restore >/dev/null || :
        depmod() { :; }
        When call setPrebakedGPUDriverState restore
        The status should be success
        The output should include 'action=restore status=completed'
        The directory "${LIVE}" should be exist
        The directory "${PAYLOAD}" should not be exist
    End

    It 'rejects cross-filesystem restoration before any move'
        park_payload
        stat() {
            if [ "$2" = '%d' ] && [ "$3" = "${BINARY%/*}" ]; then echo 999; else command stat "$@"; fi
        }
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'across filesystems'
        The directory "${LIVE}" should not be exist
        The file "${MODULE}" should not be exist
    End

    It 'detects a no-clobber move that silently did not move the file'
        park_payload
        mv() { return 0; }
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'Failed to restore NVIDIA prebake file'
        The directory "${LIVE}" should not be exist
    End

    It 'rejects a payload whose directory allows non-root access'
        park_payload
        chmod 755 "${PAYLOAD}"
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'permissions'
    End

    It 'rejects an uncommitted VHD park'
        park_payload
        rm "${PAYLOAD}/park.complete"
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'parking did not complete'
    End

    It 'rejects a lost full payload when its marker remains'
        park_payload
        rm -rf "${PAYLOAD}"
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'Missing whole NVIDIA prebake payload'
    End

    It 'can restore using the private metadata if the public marker was lost'
        park_payload
        rm "${GPU_DKMS_MARKER_FILE}"
        When call setPrebakedGPUDriverState restore
        The status should be success
        The output should include 'action=restore status=completed'
        The file "${GPU_DKMS_MARKER_FILE}" should be exist
        The directory "${LIVE}" should be exist
    End

    It 'regenerates indexes for the baked kernel rather than the builder kernel'
        RUNNING_KERNEL=6.8.0-1060-azure
        When call setPrebakedGPUDriverState park
        The status should be success
        The output should include 'action=park status=completed'
        The contents of file "${CALLS}" should include "depmod -a ${KERNEL}"
        The contents of file "${CALLS}" should not include "depmod -a ${RUNNING_KERNEL}"
    End

    It 'rebuilds an initramfs containing the removed NVIDIA module'
        touch "${TEST_ROOT}/boot/initrd.img-${KERNEL}"
        lsinitramfs() {
            if ! grep -q initramfs "${CALLS}"; then echo "usr/lib/modules/${KERNEL}/updates/dkms/nvidia.ko.zst"; fi
        }
        When call setPrebakedGPUDriverState park
        The status should be success
        The output should include 'action=park status=completed'
        The contents of file "${CALLS}" should include "initramfs -u -k ${KERNEL}"
    End

    It 'fails a VHD park if the initramfs still contains NVIDIA'
        touch "${TEST_ROOT}/boot/initrd.img-${KERNEL}"
        lsinitramfs() { echo "usr/lib/modules/${KERNEL}/updates/dkms/nvidia.ko"; }
        When call setPrebakedGPUDriverState park
        The status should be failure
        The stderr should include 'NVIDIA module remains'
        The file "${PAYLOAD}/park.complete" should not be exist
    End

    It 'fails a VHD park if the effective DKMS tree points into the cache'
        DKMS_EFFECTIVE_TREE="${PAYLOAD}/files${TEST_ROOT}/var/lib/dkms"
        When call setPrebakedGPUDriverState park
        The status should be failure
        The stderr should include 'NVIDIA remains registered'
        The file "${PAYLOAD}/park.complete" should not be exist
    End

    It 'fails before parking if the installer inventory is unavailable'
        rm "${TEST_ROOT}/var/lib/nvidia/log"
        When call setPrebakedGPUDriverState park
        The status should be failure
        The stderr should include 'log'
        The directory "${LIVE}" should be exist
        The file "${BINARY}" should be exist
    End

    It 'rejects replacement of host files outside the library overlay'
        printf '100: %s\n123 644 0 0\n' "${TEST_ROOT}/usr/bin/system-tool" >> "${TEST_ROOT}/var/lib/nvidia/log"
        When call setPrebakedGPUDriverState park
        The status should be failure
        The stderr should include 'replacement of host file'
        The directory "${LIVE}" should be exist
    End

    It 'rejects unrecognized installer inventory records'
        printf '3: %s\n123\n' "${BINARY}" >> "${TEST_ROOT}/var/lib/nvidia/log"
        When call setPrebakedGPUDriverState park
        The status should be failure
        The stderr should include 'Cannot read NVIDIA installer inventory'
        The directory "${LIVE}" should be exist
    End

    It 'does not confuse the in-tree nvidiafb module with the CUDA prebake'
        touch "${TEST_ROOT}/boot/initrd.img-${KERNEL}"
        lsinitramfs() { echo "usr/lib/modules/${KERNEL}/kernel/drivers/video/fbdev/nvidia/nvidiafb.ko"; }
        When call setPrebakedGPUDriverState park
        The status should be success
        The output should include 'action=park status=completed'
        The contents of file "${CALLS}" should not include 'initramfs'
    End

    It 'audits an older initramfs even if that kernel has no NVIDIA files left on disk'
        touch "${TEST_ROOT}/boot/initrd.img-6.8.0-1060-azure"
        lsinitramfs() {
            if ! grep -q initramfs "${CALLS}"; then echo 'usr/lib/modules/6.8.0-1060-azure/updates/dkms/nvidia.ko'; fi
        }
        When call setPrebakedGPUDriverState park
        The status should be success
        The output should include 'action=park status=completed'
        The contents of file "${CALLS}" should include 'initramfs -u -k 6.8.0-1060-azure'
    End

    It 'requests a normal install when the cached architecture differs'
        park_payload
        sed -i 's/arch=x86_64/arch=aarch64/' "${PAYLOAD}/marker"
        When call setPrebakedGPUDriverState restore
        The status should equal 2
        The output should include 'reason=architecture'
        The directory "${LIVE}" should not be exist
    End

    It 'rejects a symlink replacing the private payload directory'
        park_payload
        mv "${PAYLOAD}" "${PAYLOAD}.other"
        ln -s "${PAYLOAD}.other" "${PAYLOAD}"
        When call setPrebakedGPUDriverState restore
        The status should be failure
        The stderr should include 'Invalid NVIDIA prebake payload'
        The directory "${LIVE}" should not be exist
    End

    It 'parks the whole payload through the actual VHD build entry point'
        eval "$(sed -n '/^buildNVIDIAKernelModule()/,/^}/p' vhdbuilder/packer/install-dependencies.sh | \
            sed "s|/opt/azure/aks-gpu/dkms-marker|${GPU_DKMS_MARKER_FILE}|g")"
        OS=UBUNTU
        UBUNTU_OS_NAME=UBUNTU
        FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE
        NVIDIA_DRIVER_IMAGE=test-image
        VHD_LOGS_FILEPATH="${TEST_ROOT}/vhd.log"
        isARM64() { echo 0; }
        apt_get_install() { :; }
        retrycmd_if_failure() { echo 'build-only completed'; }
        When run buildNVIDIAKernelModule
        The status should be success
        The output should include 'build-only completed'
        The output should include 'action=park status=completed'
        The file "${MODULE}" should not be exist
        The file "${BINARY}" should not be exist
        The directory "${LIVE}" should not be exist
        The file "${PAYLOAD}/park.complete" should be exist
        The contents of file "${VHD_LOGS_FILEPATH}" should include 'nvidia-cuda-driver-prebaked='
    End

    Describe 'CPU, opt-out and GRID cache cleanup'
        It 'removes only the cache even if the public marker was lost'
            park_payload
            rm "${GPU_DKMS_MARKER_FILE}"
            printf 'customer binary\n' > "${BINARY}"
            ldconfig() { echo 'unexpected ldconfig' >&2; return 99; }
            lsmod() { echo 'unexpected lsmod' >&2; return 99; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include 'layout=whole-v1 status=cleaned'
            The stderr should equal ''
            The contents of file "${BINARY}" should equal 'customer binary'
            The directory "${PAYLOAD}" should not be exist
        End

        It 'reports failed cache deletion without activating a driver'
            park_payload
            rm() { return 1; }
            When call cleanUpPrebakedGPUDriver
            The status should be failure
            The output should include 'status=incomplete'
            The directory "${LIVE}" should not be exist
            The file "${BINARY}" should not be exist
        End

        It 'keeps CPU and opted-out node provisioning nonfatal when cache deletion fails'
            park_payload
            eval "$(sed -n '/^cleanUpGPUDrivers()/,/^}/p' parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh)"
            GPU_DEST="${TEST_ROOT}/unused"
            managedGPUPackageList() { :; }
            rm() { return 1; }
            When call cleanUpGPUDrivers
            The status should be success
            The output should include 'status=incomplete'
            The directory "${LIVE}" should not be exist
        End

        It 'does not treat a partially restored payload as an inactive cache'
            park_payload
            touch "${PAYLOAD}/restore.started"
            When call cleanUpPrebakedGPUDriver
            The status should be failure
            The output should include 'status=incomplete'
            The directory "${PAYLOAD}" should be exist
        End

        Include './parts/linux/cloud-init/artifacts/cse_config_gpu.sh'
        OS=UBUNTU
        UBUNTU_OS_NAME=UBUNTU
        NVIDIA_GPU_DRIVER_TYPE=grid-v20

        It 'discards the whole CUDA cache before GRID setup without restoring it'
            park_payload
            When call cleanUpGridNodeCudaPrebake
            The status should be success
            The output should include 'layout=whole-v1 status=cleaned'
            The directory "${LIVE}" should not be exist
            The file "${BINARY}" should not be exist
            The directory "${PAYLOAD}" should not be exist
        End

        It 'preserves an installed GRID driver and its marker while discarding the cache'
            park_payload
            printf 'driver_kind=grid\n' > "${GPU_DKMS_MARKER_FILE}"
            printf 'GRID binary\n' > "${BINARY}"
            When call cleanUpGridNodeCudaPrebake
            The status should be success
            The output should include 'layout=whole-v1 status=cleaned'
            The contents of file "${BINARY}" should equal 'GRID binary'
            The contents of file "${GPU_DKMS_MARKER_FILE}" should equal 'driver_kind=grid'
            The directory "${PAYLOAD}" should not be exist
        End
    End
End
