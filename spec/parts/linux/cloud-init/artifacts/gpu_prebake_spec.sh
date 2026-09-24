#!/bin/bash

Describe 'unregistered CUDA prebakes'
    Include ./parts/linux/cloud-init/artifacts/cse_config_gpu.sh

    setup() {
        TEST_DIR="$(mktemp -d)"
        GPU_DKMS_MARKER_FILE="${TEST_DIR}/dkms-marker"
        printf 'driver_kind=cuda\n' > "${GPU_DKMS_MARKER_FILE}"
    }
    cleanup() { command rm -rf "${TEST_DIR}"; }
    BeforeEach setup
    AfterEach cleanup

    OS=UBUNTU
    UBUNTU_OS_NAME=UBUNTU
    GPU_NODE=true
    skip_nvidia_driver_install=false
    NVIDIA_GPU_DRIVER_TYPE=cuda-lts
    CONFIG_GPU_DRIVER_IF_NEEDED=true
    NVIDIA_DRIVER_IMAGE=example/aks-gpu
    NVIDIA_DRIVER_IMAGE_TAG=test
    CTR_GPU_INSTALL_CMD='ctr run'
    ERR_GPU_DRIVERS_START_FAIL=84

    isARM64() { echo 0; }
    logs_to_events() { shift; eval "$*"; }
    systemctlEnableAndStart() { :; }
    retrycmd_if_failure() {
        shift 3
        echo "$*"
        DKMS_STATUS="nvidia/580.126.09, 6.8.0-test, x86_64: installed"
    }
    uname() { case "$1" in -r) echo 6.8.0-test ;; -m) echo x86_64 ;; esac; }
    modinfo() { echo 580.126.09; }
    dkms() { echo "${DKMS_STATUS:-}"; }
    configGPUDrivers() { installGPUDriverImage; }
    validateGPUDrivers() { echo 'validation only'; }
    cleanUpPrebakedGPUDriver() { echo 'CUDA cleanup'; }

    Describe 'GPU dispatch'
        Parameters
            true
            false
        End
        It 'uses normal install for unregistered CUDA in both installation and validation-only mode'
            CONFIG_GPU_DRIVER_IF_NEEDED="$1"
            When call ensureGPUDrivers
            The status should be success
            The output should include '/entrypoint.sh install'
            The output should not include 'install-skip-build'
            The output should not include 'validation only'
        End
    End

    Describe 'unchanged paths'
        It 'rejects an installer that returns success without completing DKMS installation'
            retrycmd_if_failure() { :; }
            When run ensureGPUDrivers
            The status should equal 84
            The output should include 'NVIDIA DKMS installation is incomplete'
        End

        It 'propagates container installation failure'
            retrycmd_if_failure() { return 1; }
            When call installGPUDriverImage
            The status should be failure
            The output should equal ''
        End

        It 'propagates installation failure even with an older installed DKMS record'
            DKMS_STATUS='nvidia/580.126.09, 6.8.0-test, x86_64: installed'
            retrycmd_if_failure() { return 1; }
            When run ensureGPUDrivers
            The status should equal 84
            The output should equal ''
        End

        It 'does not reinstall a complete DKMS installation in validation-only mode'
            DKMS_STATUS='nvidia/580.126.09, 6.8.0-test, x86_64: installed'
            CONFIG_GPU_DRIVER_IF_NEEDED=false
            When call ensureGPUDrivers
            The status should be success
            The output should include 'validation only'
            The output should not include '/entrypoint.sh'
        End

        It 'keeps ordinary install on images without a prebake'
            command rm "${GPU_DKMS_MARKER_FILE}"
            When call ensureGPUDrivers
            The status should be success
            The output should include '/entrypoint.sh install'
            The output should not include 'install-skip-build'
        End

        It 'keeps validation-only behavior without a prebake'
            command rm "${GPU_DKMS_MARKER_FILE}"
            CONFIG_GPU_DRIVER_IF_NEEDED=false
            When call ensureGPUDrivers
            The status should be success
            The output should include 'validation only'
            The output should not include '/entrypoint.sh'
        End

        It 'cleans CUDA before ordinary GRID installation'
            NVIDIA_GPU_DRIVER_TYPE=grid-v20
            When call ensureGPUDrivers
            The status should be success
            The output should include 'CUDA cleanup'
            The output should include '/entrypoint.sh install'
            The output should not include 'install-skip-build'
        End

        It 'does not change non-Ubuntu validation'
            OS=AZURELINUX
            CONFIG_GPU_DRIVER_IF_NEEDED=false
            When call ensureGPUDrivers
            The status should be success
            The output should equal 'validation only'
        End
    End

    Describe 'current-kernel DKMS installation check'
        Describe 'incomplete or mismatched installation'
            Parameters
                ''
                'nvidia/580.126.09: added'
                'nvidia/580.126.09, 6.8.0-test, x86_64: built'
                'nvidia/580.126.09, 6.8.0-old, x86_64: installed'
                'nvidia/580.126.09, 6.8.0-test, aarch64: installed'
                'nvidia/570.0.0, 6.8.0-test, x86_64: installed'
                'nvidia/580.126.09: broken'
                'nvidia/580.126.09, 6.8.0-test, x86_64: installed (WARNING! Diff between built and installed module!)'
            End
            It 'does not accept the record as installed'
                DKMS_STATUS="$1"
                When call isNvidiaDKMSInstalledForCurrentKernel
                The status should be failure
                The output should equal ''
            End

            It 'repairs the record through normal installation on a validation-only node'
                DKMS_STATUS="$1"
                CONFIG_GPU_DRIVER_IF_NEEDED=false
                When call ensureGPUDrivers
                The status should be success
                The output should include '/entrypoint.sh install'
                The output should not include 'install-skip-build'
            End
        End

        Describe 'complete installation'
            Parameters
                'nvidia/580.126.09, 6.8.0-test, x86_64: installed'
                'nvidia/580.126.09, 6.8.0-test, x86_64: installed (original_module exists)'
                'nvidia, 580.126.09, 6.8.0-test, x86_64: installed'
            End
            It 'accepts a current-kernel installed record including a retained prebake backup'
                DKMS_STATUS="$1"
                When call isNvidiaDKMSInstalledForCurrentKernel
                The status should be success
                The output should equal ''
            End
        End

        It 'queries the exact driver version, running kernel, and architecture'
            dkms() {
                echo "$*" > "${TEST_DIR}/dkms-call"
                echo 'nvidia/580.126.09, 6.8.0-test, x86_64: installed'
            }
            When call isNvidiaDKMSInstalledForCurrentKernel
            The status should be success
            The contents of file "${TEST_DIR}/dkms-call" should equal 'status -m nvidia -v 580.126.09 -k 6.8.0-test -a x86_64'
        End

        It 'rejects an unsuccessful DKMS query'
            dkms() { return 1; }
            When call isNvidiaDKMSInstalledForCurrentKernel
            The status should be failure
            The output should equal ''
        End

        It 'rejects an unreadable module'
            modinfo() { return 1; }
            When call isNvidiaDKMSInstalledForCurrentKernel
            The status should be failure
            The output should equal ''
        End
    End

    It 'keeps the existing ARM64 early return'
        isARM64() { echo 1; }
        When call ensureGPUDrivers
        The status should be success
        The output should equal ''
    End

    Describe 'PIS stage gates'
        eval "$(sed -n '/^function nodePrep {/,/^}/p' ./parts/linux/cloud-init/artifacts/cse_main.sh)"
        should_skip_nvidia_drivers() { echo "${LIVE_SKIP}"; }
        basePrep() { echo 'unexpected basePrep'; return 1; }
        reconcileVulnerableKernelModuleMitigation() { :; }
        isAmdAmaEnabledNode() { return 1; }
        checkServiceHealth() { :; }
        systemctl() { return 1; }
        retrycmd_if_failure() {
            shift 3
            case "$*" in
                *'/entrypoint.sh'*)
                    echo "$*"
                    DKMS_STATUS='nvidia/580.126.09, 6.8.0-test, x86_64: installed'
                    ;;
            esac
        }
        logs_to_events() {
            case "$1" in
                AKS.CSE.ensureGPUDrivers) ensureGPUDrivers; exit "$?" ;;
                AKS.CSE.ensureGPUDrivers.*) shift; eval "$*" ;;
                AKS.CSE.cleanUpGPUDrivers) echo 'cleanup without registration'; exit 0 ;;
            esac
        }
        provision_cached_image() {
            touch "${TEST_DIR}/base_prep.complete"
            eval "$(sed -n '/^if \[ ! -f \/opt\/azure\/containers\/base_prep.complete \]/,/^echo "Custom script finished."/p' \
                ./parts/linux/cloud-init/artifacts/cse_main.sh |
                sed "s|/opt/azure/containers/base_prep.complete|${TEST_DIR}/base_prep.complete|g")"
        }
        API_SERVER_NAME=127.0.0.1
        PRE_PROVISION_ONLY=false
        LIVE_SKIP=false

        It 'uses the live opt-out decision and installs with DKMS when basePrep is skipped'
            skip_nvidia_driver_install=true
            CONFIG_GPU_DRIVER_IF_NEEDED=false
            When run provision_cached_image
            The status should be success
            The output should include 'Skipping basePrep'
            The output should include '/entrypoint.sh install'
            The output should not include 'install-skip-build'
            The output should not include 'unexpected basePrep'
        End

        It 'keeps validation when a PIS node already has a complete DKMS installation'
            DKMS_STATUS='nvidia/580.126.09, 6.8.0-test, x86_64: installed'
            CONFIG_GPU_DRIVER_IF_NEEDED=false
            When run provision_cached_image
            The status should be success
            The output should include 'Skipping basePrep'
            The output should include 'validation only'
            The output should not include '/entrypoint.sh'
        End

        It 'does not register during pre-provision-only execution'
            PRE_PROVISION_ONLY=true
            When run provision_cached_image
            The status should be success
            The output should include 'Skipping nodePrep'
            The output should not include '/entrypoint.sh'
        End

        Describe 'PIS CPU and opt-out'
            Parameters
                false false
                true true
            End
            It 'takes cleanup without calling ensureGPUDrivers'
                GPU_NODE="$1"
                LIVE_SKIP="$2"
                ensureGPUDrivers() { echo 'unexpected ensureGPUDrivers'; return 1; }
                When run provision_cached_image
                The status should be success
                The output should include 'cleanup without registration'
                The output should not include 'unexpected ensureGPUDrivers'
                The output should not include '/entrypoint.sh'
            End
        End
    End

    Describe 'VHD build contract'
        FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE
        NVIDIA_DRIVER_IMAGE=example/aks-gpu
        NVIDIA_DRIVER_IMAGE_TAG=test
        apt_get_install() { :; }
        retrycmd_if_failure() { touch "${TEST_DIR}/dkms-marker"; }
        dkms() { echo "${DKMS_STATUS:-}"; return "${DKMS_EXIT:-0}"; }
        run_prebake() {
            VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
            eval "$(sed -n '/^buildNVIDIAKernelModule() {/,/^}/p' ./vhdbuilder/packer/install-dependencies.sh |
                sed "s|/opt/azure/aks-gpu/dkms-marker|${TEST_DIR}/dkms-marker|g; s|/var/lib/dkms/nvidia|${TEST_DIR}/nvidia|g")"
            buildNVIDIAKernelModule
        }

        It 'accepts an unregistered prebake'
            When run run_prebake
            The status should be success
            The output should include 'Pre-building NVIDIA CUDA'
        End

        It 'rejects an old container that leaves a registration'
            DKMS_STATUS='nvidia/580.126.09, added'
            When run run_prebake
            The status should be failure
            The output should include 'must not register NVIDIA'
        End

        It 'rejects a dangling registration even when dkms status is empty'
            ln -s "${TEST_DIR}/missing" "${TEST_DIR}/nvidia"
            When run run_prebake
            The status should be failure
            The output should include 'must not register NVIDIA'
        End

        It 'rejects an unsuccessful dkms status check'
            DKMS_EXIT=1
            When run run_prebake
            The status should be failure
            The output should include 'Pre-building NVIDIA CUDA'
        End
    End

    Describe 'final VHD content validation'
        FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE
        OS_SKU=Ubuntu
        dkms() { echo "${DKMS_STATUS:-}"; return "${DKMS_EXIT:-0}"; }
        modinfo() { return "${MODINFO_EXIT:-0}"; }
        err() { echo "$*" >&2; }
        validate_image() {
            eval "$(sed -n '/^testNvidiaPrebakeUnregistered() {/,/^}/p' ./vhdbuilder/packer/test/linux-vhd-content-test.sh |
                sed "s|/opt/azure/aks-gpu/dkms-marker|${TEST_DIR}/dkms-marker|g; s|/var/lib/dkms/nvidia|${TEST_DIR}/nvidia|g")"
            testNvidiaPrebakeUnregistered
        }

        It 'accepts compiled modules with no registration'
            When call validate_image
            The status should be success
            The output should equal ''
        End

        It 'rejects registration introduced after the build-only step'
            mkdir -p "${TEST_DIR}/nvidia"
            When call validate_image
            The status should be failure
            The stderr should include 'active NVIDIA DKMS registration'
        End

        It 'rejects a missing compiled module'
            MODINFO_EXIT=1
            When call validate_image
            The status should be failure
            The stderr should include 'compiled module is missing'
        End

        It 'does not require a prebake on other image variants'
            FEATURE_FLAGS=None
            command rm "${GPU_DKMS_MARKER_FILE}"
            When call validate_image
            The status should be success
            The output should equal ''
        End
    End
End
