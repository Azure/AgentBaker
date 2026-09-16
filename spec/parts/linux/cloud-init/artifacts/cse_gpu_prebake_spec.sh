#!/bin/bash
# shellcheck disable=SC2218,SC2329 # ShellSpec installs and calls per-example mocks dynamically.

Describe 'NVIDIA prebake registration layout'
    setup_prebake() {
        TEST_ROOT="$(mktemp -d)"
        GPU_DKMS_MARKER_FILE="${TEST_ROOT}/opt/azure/aks-gpu/dkms-marker"
        LIVE="${TEST_ROOT}/var/lib/dkms/nvidia"
        PARKED="${GPU_DKMS_MARKER_FILE%/*}/dkms/nvidia"
        BUILT_MODULE="580.159.04/6.8.0-1065-azure/x86_64/module/nvidia.ko"
        INSTALLED_MODULE="${TEST_ROOT}/lib/modules/6.8.0-1065-azure/updates/dkms/nvidia.ko"
        mkdir -p "${LIVE}/${BUILT_MODULE%/*}" "${GPU_DKMS_MARKER_FILE%/*}" \
            "${INSTALLED_MODULE%/*}" "${TEST_ROOT}/usr/src/nvidia-580.159.04" \
            "${TEST_ROOT}/usr/bin/lib64" "${TEST_ROOT}/etc/ld.so.conf.d"
        printf 'kernel=6.8.0-1065-azure\ndriver_version=580.159.04\ndriver_kind=cuda\narch=x86_64\n' > "${GPU_DKMS_MARKER_FILE}"
        printf 'prebuilt module\n' > "${LIVE}/${BUILT_MODULE}"
        printf 'installed module\n' > "${INSTALLED_MODULE}"
        printf 'driver binary\n' > "${TEST_ROOT}/usr/bin/nvidia-modprobe"
        printf 'driver library\n' > "${TEST_ROOT}/usr/bin/lib64/libnvidia.so"
        ln -s "${TEST_ROOT}/usr/src/nvidia-580.159.04" "${LIVE}/580.159.04/source"
        MODULE_INODE="$(stat -c '%i' "${LIVE}/${BUILT_MODULE}")"

        # Exercise the real functions on an isolated filesystem, including real mv/rm. This avoids
        # adding production path overrides solely for tests or touching the runner's driver files.
        eval "$(sed -n '/^setPrebakedGPUDriverRegistration()/,/^}/p; /^cleanUpPrebakedGPUDriver()/,/^}/p' \
            parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh | sed \
            -e "s|/var/lib/dkms|${TEST_ROOT}/var/lib/dkms|g" \
            -e "s|/lib/modules|${TEST_ROOT}/lib/modules|g" \
            -e "s|/usr/bin|${TEST_ROOT}/usr/bin|g" \
            -e "s|/etc/ld.so.conf.d|${TEST_ROOT}/etc/ld.so.conf.d|g")"
    }

    cleanup_prebake() {
        command rm -rf "${TEST_ROOT}"
    }
    BeforeEach 'setup_prebake'
    AfterEach 'cleanup_prebake'

    lsmod() { :; }
    ldconfig() { :; }
    dkms() {
        [ "$*" = status ] || return 99
        if [ -d "${DKMS_EFFECTIVE_TREE:-${TEST_ROOT}/var/lib/dkms}/nvidia" ]; then
            echo 'nvidia/580.159.04, 6.8.0-1065-azure, x86_64: installed'
        fi
        return 0
    }

    park_prebake() {
        setPrebakedGPUDriverRegistration park >/dev/null
    }

    registration_and_metadata() {
        setPrebakedGPUDriverRegistration "${1}" || return 1
        stat -c '%i' "${2}/${BUILT_MODULE}"
        readlink "${2}/580.159.04/source"
    }

    It 'parks registration while preserving module bytes, inode, source link, and installed files'
        When call registration_and_metadata park "${PARKED}"
        The status should be success
        The output should include 'park completed'
        The directory "${LIVE}" should not be exist
        The contents of file "${PARKED}/${BUILT_MODULE}" should equal 'prebuilt module'
        The contents of file "${INSTALLED_MODULE}" should equal 'installed module'
        The contents of file "${TEST_ROOT}/usr/bin/nvidia-modprobe" should equal 'driver binary'
        The contents of file "${TEST_ROOT}/usr/bin/lib64/libnvidia.so" should equal 'driver library'
        The contents of file "${GPU_DKMS_MARKER_FILE}" should include "dkms_parked_path=${PARKED}"
        The contents of file "${GPU_DKMS_MARKER_FILE}" should include 'driver_kind=cuda'
        The line 2 of output should equal "${MODULE_INODE}"
        The line 3 of output should equal "${TEST_ROOT}/usr/src/nvidia-580.159.04"
    End

    It 'restores the original tree without calling DKMS or depmod'
        park_prebake
        dkms() { echo 'unexpected DKMS invocation' >&2; return 99; }
        depmod() { echo 'unexpected depmod invocation' >&2; return 99; }
        When call registration_and_metadata restore "${LIVE}"
        The status should be success
        The output should include 'restore completed'
        The stderr should equal ''
        The directory "${PARKED}" should not be exist
        The contents of file "${LIVE}/${BUILT_MODULE}" should equal 'prebuilt module'
        The line 2 of output should equal "${MODULE_INODE}"
        The line 3 of output should equal "${TEST_ROOT}/usr/src/nvidia-580.159.04"
    End

    It 'is a no-op when restoration has already completed'
        park_prebake
        setPrebakedGPUDriverRegistration restore >/dev/null
        When call setPrebakedGPUDriverRegistration restore
        The status should be success
        The output should equal ''
        The contents of file "${LIVE}/${BUILT_MODULE}" should equal 'prebuilt module'
    End

    It 'restores an older kernel cache without compiling for the running kernel'
        park_prebake
        uname() { echo 6.8.0-1067-azure; }
        dkms() { echo 'unexpected DKMS invocation' >&2; return 99; }
        When call setPrebakedGPUDriverRegistration restore
        The status should be success
        The output should include 'restore completed'
        The stderr should equal ''
        The contents of file "${LIVE}/${BUILT_MODULE}" should equal 'prebuilt module'
        The directory "${LIVE}/580.159.04/6.8.0-1067-azure" should not be exist
    End

    It 'leaves a legacy live registration unchanged'
        When call setPrebakedGPUDriverRegistration restore
        The status should be success
        The output should equal ''
        The contents of file "${LIVE}/${BUILT_MODULE}" should equal 'prebuilt module'
    End

    It 'is a no-op on an image without a prebake'
        rm -rf "${LIVE}" "${GPU_DKMS_MARKER_FILE}"
        When call setPrebakedGPUDriverRegistration restore
        The status should be success
        The output should equal ''
    End

    It 'rejects a parked-layout image that lost both registration trees'
        park_prebake
        rm -rf "${PARKED}"
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The stderr should include 'Missing NVIDIA DKMS registration'
    End

    It 'rejects restoration when the parked prebake lost its marker'
        park_prebake
        rm "${GPU_DKMS_MARKER_FILE}"
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The stderr should include 'prebake marker or source tree is missing or invalid'
        The directory "${LIVE}" should not be exist
        The directory "${PARKED}" should be exist
    End

    It 'does not merge or overwrite an existing live registration'
        park_prebake
        mkdir -p "${LIVE}"
        printf 'new driver\n' > "${LIVE}/sentinel"
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The stderr should include 'destination already exists'
        The contents of file "${LIVE}/sentinel" should equal 'new driver'
        The contents of file "${PARKED}/${BUILT_MODULE}" should equal 'prebuilt module'
        The directory "${LIVE}/nvidia" should not be exist
    End

    It 'does not overwrite a parked tree during another build'
        mkdir -p "${PARKED}"
        When call setPrebakedGPUDriverRegistration park
        The status should be failure
        The stderr should include 'destination already exists'
        The contents of file "${LIVE}/${BUILT_MODULE}" should equal 'prebuilt module'
    End

    It 'does not park a build that did not produce a marker'
        command rm "${GPU_DKMS_MARKER_FILE}"
        When call setPrebakedGPUDriverRegistration park
        The status should be failure
        The stderr should include 'prebake marker or source tree is missing or invalid'
        The directory "${LIVE}" should be exist
        The directory "${PARKED}" should not be exist
    End

    It 'does not accept a marker without a registration to park'
        command rm -rf "${LIVE}"
        When call setPrebakedGPUDriverRegistration park
        The status should be failure
        The stderr should include 'prebake marker or source tree is missing or invalid'
        The directory "${PARKED}" should not be exist
    End

    It 'rejects a dangling destination symlink'
        park_prebake
        ln -s "${TEST_ROOT}/missing" "${LIVE}"
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The stderr should include 'destination already exists'
        The directory "${PARKED}" should be exist
    End

    It 'rejects a symlink in place of the parked source tree'
        mkdir -p "${PARKED%/*}"
        ln -s "${LIVE}" "${PARKED}"
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The stderr should include 'source tree is missing or invalid'
        The contents of file "${LIVE}/${BUILT_MODULE}" should equal 'prebuilt module'
    End

    It 'rejects cross-filesystem restoration instead of copying the cache'
        park_prebake
        stat() {
            if [ "${3}" = "${PARKED}" ]; then echo 1; else echo 2; fi
        }
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The stderr should include 'across filesystems'
        The directory "${PARKED}" should be exist
        The directory "${LIVE}" should not be exist
    End

    It 'propagates a failed rename and leaves the parked registration intact'
        park_prebake
        mv() { return 1; }
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The directory "${PARKED}" should be exist
        The directory "${LIVE}" should not be exist
    End

    It 'does not move registration when destination creation fails'
        park_prebake
        mkdir() { return 1; }
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The directory "${PARKED}" should be exist
        The directory "${LIVE}" should not be exist
    End

    It 'does not move registration when the filesystem cannot be checked'
        park_prebake
        stat() { return 1; }
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The directory "${PARKED}" should be exist
        The directory "${LIVE}" should not be exist
    End

    It 'detects a no-clobber rename that returned success without moving'
        park_prebake
        mv() {
            command mkdir -p "${LIVE}"
            printf 'concurrent registration\n' > "${LIVE}/sentinel"
            command mv "$@"
        }
        When call setPrebakedGPUDriverRegistration restore
        The status should be failure
        The stderr should include 'Failed to restore'
        The directory "${PARKED}" should be exist
        The contents of file "${LIVE}/sentinel" should equal 'concurrent registration'
        The directory "${LIVE}/nvidia" should not be exist
    End

    It 'rejects an effective DKMS tree redirected to the parked directory'
        DKMS_EFFECTIVE_TREE="${PARKED%/*}"
        When call setPrebakedGPUDriverRegistration park
        The status should be failure
        The stderr should include 'NVIDIA remains registered with DKMS'
    End

    It 'fails the park operation when DKMS status cannot be checked'
        dkms() { return 1; }
        When call setPrebakedGPUDriverRegistration park
        The status should be failure
    End

    It 'cleans the legacy live registration'
        When call cleanUpPrebakedGPUDriver
        The status should be success
        The output should include 'status=cleaned'
        The output should include 'dkms_before=true'
        The output should include 'dkms_after=false parked_after=false'
        The directory "${LIVE}" should not be exist
        The file "${GPU_DKMS_MARKER_FILE}" should not be exist
    End

    It 'cleans the parked registration and installed artifacts'
        park_prebake
        When call cleanUpPrebakedGPUDriver
        The status should be success
        The output should include 'status=cleaned'
        The output should include 'dkms_before=false'
        The output should include 'dkms_after=false parked_after=false'
        The directory "${PARKED}" should not be exist
        The file "${INSTALLED_MODULE}" should not be exist
        The file "${TEST_ROOT}/usr/bin/nvidia-modprobe" should not be exist
        The file "${GPU_DKMS_MARKER_FILE}" should not be exist
    End

    It 'cleans parked artifacts even if the marker was lost'
        park_prebake
        rm "${GPU_DKMS_MARKER_FILE}"
        When call cleanUpPrebakedGPUDriver
        The status should be success
        The output should include 'status=cleaned'
        The directory "${PARKED}" should not be exist
        The file "${INSTALLED_MODULE}" should not be exist
    End

    It 'retains the marker and reports parked residue when cleanup fails'
        park_prebake
        rm() {
            [ "${2:-}" != "${PARKED}" ] || return 1
            command rm "$@"
        }
        When call cleanUpPrebakedGPUDriver
        The status should be success
        The output should include 'status=incomplete'
        The output should include 'dkms_after=false parked_after=true'
        The file "${GPU_DKMS_MARKER_FILE}" should be exist
        The directory "${PARKED}" should be exist
        The directory "${LIVE}" should not be exist
    End

    It 'does not remove an unmarked live driver that may belong to the customer'
        rm "${GPU_DKMS_MARKER_FILE}"
        When call cleanUpPrebakedGPUDriver
        The status should be success
        The output should equal ''
        The contents of file "${LIVE}/${BUILT_MODULE}" should equal 'prebuilt module'
    End

    Describe 'VHD build integration'
        setup_build() {
            eval "$(sed -n '/^buildNVIDIAKernelModule()/,/^}/p' vhdbuilder/packer/install-dependencies.sh | \
                sed "s|/opt/azure/aks-gpu/dkms-marker|${GPU_DKMS_MARKER_FILE}|g")"
            OS=UBUNTU
            UBUNTU_OS_NAME=UBUNTU
            FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE
            NVIDIA_DRIVER_IMAGE=test-image
            NVIDIA_DRIVER_IMAGE_TAG=test-tag
            VHD_LOGS_FILEPATH="${TEST_ROOT}/vhd.log"
        }
        BeforeEach 'setup_build'
        isARM64() { echo 0; }
        apt_get_install() { return 0; }
        retrycmd_if_failure() { echo 'build-only completed'; }

        It 'parks after build-only and before recording a successful prebake'
            When run buildNVIDIAKernelModule
            The status should be success
            The output should include 'build-only completed'
            The output should include 'park completed'
            The directory "${LIVE}" should not be exist
            The directory "${PARKED}" should be exist
            The contents of file "${VHD_LOGS_FILEPATH}" should include 'nvidia-cuda-driver-prebaked='
        End

        It 'fails the VHD build if registration cannot be parked'
            mv() { return 1; }
            When run buildNVIDIAKernelModule
            The status should equal 1
            The output should include 'build-only completed'
            The directory "${LIVE}" should be exist
            The file "${VHD_LOGS_FILEPATH}" should not be exist
        End

        It 'does not park anything when prebaking is disabled'
            FEATURE_FLAGS=None
            When run buildNVIDIAKernelModule
            The status should be success
            The output should equal ''
            The directory "${LIVE}" should be exist
        End

        It 'does not park an ARM64 GPU installation'
            isARM64() { echo 1; }
            When run buildNVIDIAKernelModule
            The status should be success
            The output should equal ''
            The directory "${LIVE}" should be exist
        End
    End
End

Describe 'managed GPU registration dispatch'
    Include './parts/linux/cloud-init/artifacts/cse_config_gpu.sh'
    OS=UBUNTU
    UBUNTU_OS_NAME=UBUNTU
    ERR_GPU_DRIVERS_START_FAIL=84
    isARM64() { echo 0; }
    logs_to_events() { shift; ${@}; }
    setPrebakedGPUDriverRegistration() { echo "registration $*"; }
    cleanUpGridNodeCudaPrebake() { echo 'GRID cleanup'; }
    configGPUDrivers() { echo 'install'; }
    validateGPUDrivers() { echo 'validate'; }
    systemctlEnableAndStart() { :; }
    logGPUDriverPrebakeReadiness() { :; }

    It 'restores before GRID cleanup and installation'
        CONFIG_GPU_DRIVER_IF_NEEDED=true
        When call ensureGPUDrivers
        The status should be success
        The line 1 of output should equal 'registration restore'
        The line 2 of output should equal 'GRID cleanup'
        The line 3 of output should equal 'install'
    End

    It 'restores even when a loadable module only needs validation'
        CONFIG_GPU_DRIVER_IF_NEEDED=false
        When call ensureGPUDrivers
        The status should be success
        The line 1 of output should equal 'registration restore'
        The line 2 of output should equal 'GRID cleanup'
        The line 3 of output should equal 'validate'
    End

    It 'fails provisioning before driver setup if restoration fails'
        setPrebakedGPUDriverRegistration() { return 1; }
        CONFIG_GPU_DRIVER_IF_NEEDED=true
        When run ensureGPUDrivers
        The status should equal 84
        The output should equal ''
    End

    It 'does not restore an Ubuntu prebake on Azure Linux'
        OS=AZURELINUX
        CONFIG_GPU_DRIVER_IF_NEEDED=true
        When call ensureGPUDrivers
        The status should be success
        The output should equal 'install'
    End

    It 'does not restore an x86 prebake on ARM64'
        isARM64() { echo 1; }
        When call ensureGPUDrivers
        The status should be success
        The output should equal ''
    End
End
