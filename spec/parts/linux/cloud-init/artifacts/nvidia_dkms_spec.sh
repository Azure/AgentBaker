#!/bin/bash

Describe 'parked NVIDIA DKMS registration'
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"

    setup() {
        TEST_DIR="${SHELLSPEC_WORKDIR}/nvidia-dkms"
        GPU_DKMS_ACTIVE_DIR="${TEST_DIR}/var/lib/dkms/nvidia"
        GPU_DKMS_PARKED_DIR="${TEST_DIR}/opt/azure/aks-gpu/dkms/nvidia"
        GPU_DKMS_MARKER_FILE="${TEST_DIR}/opt/azure/aks-gpu/dkms-marker"
        DRIVER_VERSION="580.126.09"
        KERNEL="6.8.0-test-azure"
        OS="UBUNTU"
        UBUNTU_OS_NAME="UBUNTU"
        GPU_NODE=true
        skip_nvidia_driver_install=false
        NVIDIA_GPU_DRIVER_TYPE="cuda-lts"
        CONFIG_GPU_DRIVER_IF_NEEDED=true
        ERR_GPU_DRIVERS_START_FAIL=84
        mkdir -p "$(dirname "${GPU_DKMS_MARKER_FILE}")" "${TEST_DIR}/usr/src/nvidia-${DRIVER_VERSION}"
        printf 'PACKAGE_NAME="nvidia"\n' > "${TEST_DIR}/usr/src/nvidia-${DRIVER_VERSION}/dkms.conf"
        printf 'installed module\n' > "${TEST_DIR}/nvidia.ko"
        printf 'userspace\n' > "${TEST_DIR}/nvidia-smi"
        write_marker
    }

    cleanup() {
        command rm -rf "${TEST_DIR}"
    }

    write_marker() {
        printf 'driver_version=%s\nkernel=%s\narch=x86_64\ndriver_kind=cuda\n' \
            "${DRIVER_VERSION}" "${KERNEL}" > "${GPU_DKMS_MARKER_FILE}"
    }

    registration() {
        mkdir -p "${1}/${DRIVER_VERSION}/${KERNEL}/x86_64/module"
        ln -s "${TEST_DIR}/usr/src/nvidia-${DRIVER_VERSION}" "${1}/${DRIVER_VERSION}/source"
        ln -s "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/${KERNEL}/x86_64" "${1}/${DRIVER_VERSION}/build"
        printf 'cached module\n' > "${1}/${DRIVER_VERSION}/${KERNEL}/x86_64/module/nvidia.ko"
    }

    parked_registration() {
        registration "${GPU_DKMS_PARKED_DIR}"
        printf 'dkms_registration=parked-v1\n' >> "${GPU_DKMS_MARKER_FILE}"
    }

    # Use the real GNU move on macOS too; Linux images use coreutils mv.
    mv() {
        if [ "$1" = -T ]; then
            [ "${FAIL_MOVE:-false}" = true ] && return 1
            [ "${SKIP_MOVE:-false}" = true ] && return 0
            if [ "${RACE_MOVE:-false}" = true ]; then
                command mkdir -p "$5"
                printf 'independent registration\n' > "$5/independent"
            fi
        else
            [ "${FAIL_MARKER_UPDATE:-false}" = true ] && return 1
        fi
        if command -v gmv >/dev/null 2>&1; then
            command gmv "$@"
        else
            command mv "$@"
        fi
    }

    # Cleanup still removes installed modules/userspace in production. Only allow this fixture's
    # real paths through, so testing its existing best-effort removal never touches the host.
    rm() {
        local arg
        for arg in "$@"; do
            case "${arg}" in
                -*) ;;
                "${TEST_DIR}"/*)
                    if [ "${FAIL_REMOVE_PARKED:-false}" = true ] && [ "${arg}" = "${GPU_DKMS_PARKED_DIR}" ]; then
                        return 1
                    fi
                    ;;
                *) return 0 ;;
            esac
        done
        command rm "$@"
    }
    lsmod() { :; }
    ldconfig() { :; }
    isARM64() { echo "${ARM64:-0}"; }
    uname() { echo "${KERNEL}"; }
    logs_to_events() { shift; eval "$*"; }
    systemctlEnableAndStart() { :; }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'image parking'
        It 'moves registration and compiled caches, preserving the source symlink and installed driver'
            registration "${GPU_DKMS_ACTIVE_DIR}"
            When call parkPrebakedNvidiaDKMS
            The status should be success
            The output should include "event=dkms_parked"
            The path "${GPU_DKMS_ACTIVE_DIR}" should not be exist
            The path "${GPU_DKMS_PARKED_DIR}/${DRIVER_VERSION}/source" should be symlink
            The path "${GPU_DKMS_PARKED_DIR}/${DRIVER_VERSION}/build" should be symlink
            The path "${GPU_DKMS_PARKED_DIR}/${DRIVER_VERSION}/source/dkms.conf" should be file
            The contents of file "${GPU_DKMS_PARKED_DIR}/${DRIVER_VERSION}/${KERNEL}/x86_64/module/nvidia.ko" should equal "cached module"
            The contents of file "${TEST_DIR}/nvidia.ko" should equal "installed module"
            The contents of file "${TEST_DIR}/nvidia-smi" should equal "userspace"
            The contents of file "${GPU_DKMS_MARKER_FILE}" should include "driver_version=${DRIVER_VERSION}"
            The contents of file "${GPU_DKMS_MARKER_FILE}" should include "dkms_registration=parked-v1"
        End

        It 'is idempotent after a completed park'
            parked_registration
            When call parkPrebakedNvidiaDKMS
            The status should be success
            The output should include "event=dkms_already_parked"
            The path "${GPU_DKMS_PARKED_DIR}/nvidia" should not be exist
        End

        It 'rejects an existing destination without merging or replacing either registration'
            parked_registration
            registration "${GPU_DKMS_ACTIVE_DIR}"
            When call parkPrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=park_destination_exists"
            The path "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/source/dkms.conf" should be file
            The path "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/build/module/nvidia.ko" should be file
            The path "${GPU_DKMS_PARKED_DIR}/${DRIVER_VERSION}/source/dkms.conf" should be file
            The path "${GPU_DKMS_PARKED_DIR}/nvidia" should not be exist
        End

        It 'fails when the expected registration was not built'
            When call parkPrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=missing_or_invalid_registration"
        End

        It 'does not park when the marker cannot be published'
            registration "${GPU_DKMS_ACTIVE_DIR}"
            FAIL_MARKER_UPDATE=true
            When call parkPrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=marker_update_failed"
            The contents of file "${GPU_DKMS_MARKER_FILE}" should not include "dkms_registration"
            The path "${GPU_DKMS_ACTIVE_DIR}" should be directory
            The path "${GPU_DKMS_PARKED_DIR}" should not be exist
        End

        It 'rejects a dangling destination symlink'
            registration "${GPU_DKMS_ACTIVE_DIR}"
            mkdir -p "$(dirname "${GPU_DKMS_PARKED_DIR}")"
            ln -s "${TEST_DIR}/absent" "${GPU_DKMS_PARKED_DIR}"
            When call parkPrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=park_destination_exists"
            The path "${GPU_DKMS_ACTIVE_DIR}" should be directory
        End

        Context 'move failures'
            Parameters
                FAIL_MOVE
                SKIP_MOVE
            End
            It 'fails if move fails or reports success without moving'
                registration "${GPU_DKMS_ACTIVE_DIR}"
                export "$1=true"
                When call parkPrebakedNvidiaDKMS
                The status should be failure
                The output should include "reason=park_failed"
                The path "${GPU_DKMS_ACTIVE_DIR}" should be directory
            End
        End
    End

    Describe 'restoration'
        It 'restores the original tree before ordinary installation'
            parked_registration
            When call restorePrebakedNvidiaDKMS
            The status should be success
            The output should include "event=dkms_restored"
            The path "${GPU_DKMS_PARKED_DIR}" should not be exist
            The path "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/source/dkms.conf" should be file
            The path "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/build/module/nvidia.ko" should be file
        End

        It 'accepts a retry after activation without nesting directories'
            registration "${GPU_DKMS_ACTIVE_DIR}"
            printf 'dkms_registration=parked-v1\n' >> "${GPU_DKMS_MARKER_FILE}"
            When call restorePrebakedNvidiaDKMS
            The status should be success
            The output should include "event=dkms_already_active"
            The path "${GPU_DKMS_ACTIVE_DIR}/nvidia" should not be exist
        End

        It 'does not change legacy active registrations'
            registration "${GPU_DKMS_ACTIVE_DIR}"
            When call restorePrebakedNvidiaDKMS
            The status should be success
            The output should equal ""
            The path "${GPU_DKMS_ACTIVE_DIR}" should be directory
        End

        It 'does not require a marker or registration on old non-prebaked images'
            rm "${GPU_DKMS_MARKER_FILE}"
            When call restorePrebakedNvidiaDKMS
            The status should be success
            The output should equal ""
        End

        It 'fails when a parked-image marker has neither registration tree'
            printf 'dkms_registration=parked-v1\n' >> "${GPU_DKMS_MARKER_FILE}"
            When call restorePrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=missing_or_invalid_registration"
        End

        It 'does not treat an empty layout field and missing cache as a legacy image'
            printf 'dkms_registration=\n' >> "${GPU_DKMS_MARKER_FILE}"
            When call restorePrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=unsupported_registration_layout"
        End

        It 'does not overwrite a destination created concurrently with restoration'
            parked_registration
            RACE_MOVE=true
            When call restorePrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=restore_failed"
            The contents of file "${GPU_DKMS_ACTIVE_DIR}/independent" should equal "independent registration"
            The path "${GPU_DKMS_PARKED_DIR}" should be directory
            The path "${GPU_DKMS_ACTIVE_DIR}/nvidia" should not be exist
        End

        It 'fails without modifying an independently active registration'
            parked_registration
            mkdir -p "${GPU_DKMS_ACTIVE_DIR}/other-driver-version"
            When call restorePrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=active_and_parked_registration"
            The path "${GPU_DKMS_ACTIVE_DIR}/other-driver-version" should be directory
            The path "${GPU_DKMS_PARKED_DIR}" should be directory
        End

        It 'rejects a parked tree with missing metadata'
            parked_registration
            rm "${GPU_DKMS_MARKER_FILE}"
            When call restorePrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=unsupported_registration_layout"
            The path "${GPU_DKMS_PARKED_DIR}" should be directory
        End

        It 'rejects a missing source tree instead of accepting an empty DKMS cache'
            parked_registration
            rm -rf "${TEST_DIR}/usr/src"
            When call restorePrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=missing_or_invalid_registration"
            The path "${GPU_DKMS_PARKED_DIR}" should be directory
        End

        It 'rejects an empty DKMS configuration'
            parked_registration
            : > "${TEST_DIR}/usr/src/nvidia-${DRIVER_VERSION}/dkms.conf"
            When call restorePrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=missing_or_invalid_registration"
        End

        It 'reports failure if the active parent directory cannot be created'
            parked_registration
            mkdir() { return 1; }
            When call restorePrebakedNvidiaDKMS
            The status should be failure
            The output should include "reason=restore_failed"
            The path "${GPU_DKMS_PARKED_DIR}" should be directory
        End

        Context 'invalid marker fields'
            Parameters
                'driver_version=../other'
                'driver_kind=grid'
                'dkms_registration=unknown'
            End
            It 'rejects duplicate or unsupported metadata without trusting paths'
                parked_registration
                printf '%s\n' "$1" >> "${GPU_DKMS_MARKER_FILE}"
                When call restorePrebakedNvidiaDKMS
                The status should be failure
                The output should include "event=dkms_error"
                The path "${GPU_DKMS_PARKED_DIR}" should be directory
            End
        End

        It 'restores registration but does not manufacture a cache for a different kernel'
            parked_registration
            KERNEL="6.8.0-new-azure"
            When call restorePrebakedNvidiaDKMS
            The status should be success
            The output should include "event=dkms_restored"
            The path "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/${KERNEL}" should not be exist
        End

        It 'can retry after an unsuccessful restoration move'
            parked_registration
            retry_restore() {
                FAIL_MOVE=true
                restorePrebakedNvidiaDKMS && return 1
                FAIL_MOVE=false
                restorePrebakedNvidiaDKMS
            }
            When call retry_restore
            The status should be success
            The output should include "reason=restore_failed"
            The output should include "event=dkms_restored"
            The path "${GPU_DKMS_PARKED_DIR}" should not be exist
        End
    End

    Describe 'managed provisioning'
        configGPUDrivers() {
            [ -f "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/source/dkms.conf" ] || return 99
            echo "ordinary install ${GPU_DV:-}"
        }
        validateGPUDrivers() {
            [ -f "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/source/dkms.conf" ] || return 99
            echo "validate registered driver"
        }

        Context 'consumer dispatch'
            Parameters
                true 'ordinary install'
                false 'validate registered driver'
            End
            It 'restores before the selected consumer'
                parked_registration
                CONFIG_GPU_DRIVER_IF_NEEDED="$1"
                When call ensureGPUDrivers
                The status should be success
                The line 1 of output should equal "AKS_GPU_PREBAKE event=dkms_restored"
                The line 2 of output should include "$2"
            End
        End

        It 'keeps ordinary installation when the requested driver version differs'
            parked_registration
            GPU_DV="595.1.2"
            When call ensureGPUDrivers
            The status should be success
            The output should include "ordinary install 595.1.2"
            The output should not include "skip-build"
        End

        Context 'missing cache dispatch'
            Parameters
                true
                false
            End
            It 'fails provisioning before either consumer when the expected cache is missing'
                CONFIG_GPU_DRIVER_IF_NEEDED="$1"
                printf 'dkms_registration=parked-v1\n' >> "${GPU_DKMS_MARKER_FILE}"
                When run ensureGPUDrivers
                The status should equal 84
                The output should include "reason=missing_or_invalid_registration"
                The output should not include "ordinary install"
                The output should not include "validate registered driver"
            End
        End

        Context 'unmanaged nodes'
            Parameters
                false false
                true true
            End
            It 'never activates on CPU or driver-opt-out nodes'
                parked_registration
                GPU_NODE="$1"
                skip_nvidia_driver_install="$2"
                When call ensureGPUDrivers
                The status should be success
                The output should equal ""
                The path "${GPU_DKMS_PARKED_DIR}" should be directory
                The path "${GPU_DKMS_ACTIVE_DIR}" should not be exist
            End
        End

        It 'retains the ARM64 exclusion'
            parked_registration
            ARM64=1
            When call ensureGPUDrivers
            The status should be success
            The output should equal ""
            The path "${GPU_DKMS_PARKED_DIR}" should be directory
        End

        It 'retains non-Ubuntu provisioning without invoking Ubuntu-only helpers'
            OS="AZURELINUX"
            configGPUDrivers() { echo "AzureLinux driver installation"; }
            When call ensureGPUDrivers
            The status should be success
            The output should equal "AzureLinux driver installation"
        End

        It 'cleans a GRID node before installation without activating its CUDA prebake'
            parked_registration
            NVIDIA_GPU_DRIVER_TYPE=grid
            configGPUDrivers() {
                [ ! -e "${GPU_DKMS_ACTIVE_DIR}" ] && [ ! -e "${GPU_DKMS_PARKED_DIR}" ] || return 99
                echo "install GRID"
            }
            When call ensureGPUDrivers
            The status should be success
            The output should include "event=grid_cuda_prebake_teardown"
            The output should include "parked_after=false"
            The output should include "install GRID"
            The output should not include "event=dkms_restored"
        End

        It 'does not let a GRID validation shortcut bypass failed parked-cache cleanup'
            parked_registration
            NVIDIA_GPU_DRIVER_TYPE=grid-v20
            CONFIG_GPU_DRIVER_IF_NEEDED=false
            FAIL_REMOVE_PARKED=true
            When run ensureGPUDrivers
            The status should equal 84
            The output should include "reason=grid_parked_cleanup_failed"
            The output should not include "validate registered driver"
        End

        It 'does not remove an independent active registration before installing GRID'
            parked_registration
            mkdir -p "${GPU_DKMS_ACTIVE_DIR}/independent"
            NVIDIA_GPU_DRIVER_TYPE=grid
            When run ensureGPUDrivers
            The status should equal 84
            The output should include "reason=active_and_parked_registration"
            The output should include "reason=grid_parked_cleanup_failed"
            The output should not include "ordinary install"
            The path "${GPU_DKMS_ACTIVE_DIR}/independent" should be directory
            The path "${GPU_DKMS_PARKED_DIR}" should be directory
        End
    End

    Describe 'cleanup'
        Context 'supported image layouts'
            Parameters
                active
                parked
            End
            It 'removes legacy or parked state using fixed paths, ignoring marker-provided paths'
                if [ "$1" = active ]; then
                    registration "${GPU_DKMS_ACTIVE_DIR}"
                else
                    parked_registration
                fi
                printf 'dkms_path=%s\n' "${TEST_DIR}/nvidia-smi" >> "${GPU_DKMS_MARKER_FILE}"
                When call cleanUpPrebakedGPUDriver
                The status should be success
                The output should include "status=cleaned"
                The output should include "parked_after=false"
                The path "${GPU_DKMS_ACTIVE_DIR}" should not be exist
                The path "${GPU_DKMS_PARKED_DIR}" should not be exist
                The path "${GPU_DKMS_MARKER_FILE}" should not be exist
                The path "${TEST_DIR}/nvidia-smi" should be file
            End
        End

        It 'reports incomplete cleanup without deleting an independently active registration'
            parked_registration
            registration "${GPU_DKMS_ACTIVE_DIR}"
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "status=incomplete"
            The output should include "reason=active_and_parked_registration"
            The path "${GPU_DKMS_ACTIVE_DIR}" should be directory
            The path "${GPU_DKMS_PARKED_DIR}" should be directory
            The path "${GPU_DKMS_MARKER_FILE}" should be file
        End

        It 'cleans an orphaned parked cache even when its marker is missing'
            parked_registration
            rm "${GPU_DKMS_MARKER_FILE}"
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "status=cleaned"
            The path "${GPU_DKMS_PARKED_DIR}" should not be exist
        End

        It 'keeps the marker for retry when removal fails'
            parked_registration
            FAIL_REMOVE_PARKED=true
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "status=incomplete"
            The output should include "parked_after=true"
            The path "${GPU_DKMS_MARKER_FILE}" should be file
        End
    End

    Describe 'VHD build integration'
        # Load the real function without executing the packer script's installation mainline.
        eval "$(sed -n '/^buildNVIDIAKernelModule() {/,/^}/p' ./vhdbuilder/packer/install-dependencies.sh)"
        FEATURE_FLAGS="NVIDIA_CUDA_PREBAKE,NVIDIA_DKMS_PARK"
        NVIDIA_DRIVER_IMAGE="example/aks-gpu-cuda-lts"
        NVIDIA_DRIVER_IMAGE_TAG="580.126.09-test"
        apt_get_install() { echo "build prerequisites"; }
        retrycmd_if_failure() {
            echo "build-only invoked"
            [ "${FAIL_BUILD:-false}" = true ] && return 1
            if [ ! -d "${GPU_DKMS_ACTIVE_DIR}" ]; then registration "${GPU_DKMS_ACTIVE_DIR}"; fi
            if [ "${SKIP_BUILD_MARKER:-false}" != true ]; then write_marker; fi
        }

        build_image() {
            VHD_LOGS_FILEPATH="${TEST_DIR}/vhd-install.complete"
            buildNVIDIAKernelModule
        }

        It 'declares success only after removing the active registration'
            When call build_image
            The status should be success
            The output should include "build-only invoked"
            The output should include "event=dkms_parked"
            The path "${GPU_DKMS_ACTIVE_DIR}" should not be exist
            The path "${GPU_DKMS_PARKED_DIR}/${DRIVER_VERSION}/source/dkms.conf" should be file
            The contents of file "${TEST_DIR}/vhd-install.complete" should include "nvidia-cuda-driver-prebaked"
        End

        It 'restores a previous parked build before rebuilding and parks it again'
            parked_registration
            When call build_image
            The status should be success
            The output should include "event=dkms_restored"
            The output should include "event=dkms_parked"
            The path "${GPU_DKMS_ACTIVE_DIR}" should not be exist
            The path "${GPU_DKMS_PARKED_DIR}/nvidia" should not be exist
        End

        It 'fails the image build when parking fails and never writes success'
            FAIL_MOVE=true
            When run build_image
            The status should equal 1
            The output should include "reason=park_failed"
            The path "${TEST_DIR}/vhd-install.complete" should not be exist
        End

        It 'preserves a build failure rather than claiming prebake success'
            FAIL_BUILD=true
            When run build_image
            The status should equal 1
            The output should include "build-only invoked"
            The output should not include "event=dkms_parked"
            The path "${TEST_DIR}/vhd-install.complete" should not be exist
        End

        It 'rejects two registrations before invoking the build'
            parked_registration
            registration "${GPU_DKMS_ACTIVE_DIR}"
            When run build_image
            The status should equal 1
            The output should include "reason=active_and_parked_registration"
            The output should not include "build-only invoked"
        End

        It 'preserves existing registered prebaking when parking is not enabled'
            FEATURE_FLAGS="NVIDIA_CUDA_PREBAKE"
            rm "${GPU_DKMS_MARKER_FILE}"
            When call build_image
            The status should be success
            The output should include "build-only invoked"
            The output should not include "event=dkms_parked"
            The path "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/source/dkms.conf" should be file
            The path "${GPU_DKMS_PARKED_DIR}" should not be exist
            The contents of file "${TEST_DIR}/vhd-install.complete" should include "nvidia-cuda-driver-prebaked"
        End

        It 'preserves repeated legacy prebakes without the parking opt-in'
            FEATURE_FLAGS="NVIDIA_CUDA_PREBAKE"
            registration "${GPU_DKMS_ACTIVE_DIR}"
            When call build_image
            The status should be success
            The output should include "build-only invoked"
            The output should not include "event=dkms_parked"
            The path "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/build/module/nvidia.ko" should be file
            The path "${GPU_DKMS_PARKED_DIR}" should not be exist
        End

        It 'retains the legacy marker requirement before declaring success'
            FEATURE_FLAGS="NVIDIA_CUDA_PREBAKE"
            SKIP_BUILD_MARKER=true
            rm "${GPU_DKMS_MARKER_FILE}"
            When run build_image
            The status should equal 1
            The output should include "did not produce"
            The path "${TEST_DIR}/vhd-install.complete" should not be exist
        End

        Context 'parking opt-in removed from a previously parked build'
            Parameters
                parked
                restored
            End
            It 'refuses to silently downgrade an existing parked layout'
                FEATURE_FLAGS="NVIDIA_CUDA_PREBAKE"
                parked_registration
                if [ "$1" = restored ]; then
                    mkdir -p "$(dirname "${GPU_DKMS_ACTIVE_DIR}")"
                    mv -T -n -- "${GPU_DKMS_PARKED_DIR}" "${GPU_DKMS_ACTIVE_DIR}"
                fi
                When run build_image
                The status should equal 1
                The output should include "parked NVIDIA prebake requires"
                The output should not include "build-only invoked"
            End
        End

        Context 'existing image gates'
            Parameters
                UBUNTU 0 None
                UBUNTU 1 'NVIDIA_CUDA_PREBAKE,NVIDIA_DKMS_PARK'
                AZURELINUX 0 'NVIDIA_CUDA_PREBAKE,NVIDIA_DKMS_PARK'
            End
            It 'does not prebuild outside supported distro, architecture, and feature gates'
                OS="$1"
                ARM64="$2"
                FEATURE_FLAGS="$3"
                When call build_image
                The status should be success
                The output should equal ""
                The path "${GPU_DKMS_ACTIVE_DIR}" should not be exist
                The path "${GPU_DKMS_PARKED_DIR}" should not be exist
            End
        End
    End

    Describe 'Packer feature-flag transport'
        FEATURE_FLAGS="NVIDIA_CUDA_PREBAKE,NVIDIA_DKMS_PARK"
        rendered_flags_are_preserved() {
            local commands command word flags_seen executable_seen count=0
            local words=()
            commands="$(jq -r --arg flags "${FEATURE_FLAGS}" '
                .provisioners[].inline[]? |
                select(startswith("sudo ") and contains("FEATURE_FLAGS={{user `feature_flags`}}")) |
                gsub("\\{\\{user `feature_flags`\\}\\}"; $flags) |
                gsub("\\{\\{user `[^`]+`\\}\\}"; "fixture")
            ' "$1")" || return 1
            # Values are deliberately whitespace-free; tokenize without executing build commands.
            while IFS= read -r command; do
                read -r -a words <<< "${command}"
                [ "${words[0]}" = sudo ] || return 1
                flags_seen=false
                executable_seen=false
                for word in "${words[@]:1}"; do
                    case "${word}" in
                        FEATURE_FLAGS=*)
                            if [ "${word#FEATURE_FLAGS=}" != "${FEATURE_FLAGS}" ]; then
                                echo "feature flags split across command arguments"
                                return 1
                            fi
                            flags_seen=true
                            ;;
                        *=*) ;;
                        *)
                            [ "${word}" = /bin/bash ] || return 1
                            executable_seen=true
                            break
                            ;;
                    esac
                done
                [ "${flags_seen}" = true ] && [ "${executable_seen}" = true ] || return 1
                count=$((count + 1))
            done <<< "${commands}"
            [ "${count}" -eq 3 ] || return 1
            echo "3 provisioner commands preserve FEATURE_FLAGS"
        }

        Context 'supported Ubuntu templates'
            Parameters
                ./vhdbuilder/packer/vhd-image-builder-base.json
                ./vhdbuilder/packer/vhd-image-builder-cvm.json
            End
            It 'passes both flags as one assignment before the actual executable'
                When call rendered_flags_are_preserved "$1"
                The status should be success
                The output should equal "3 provisioner commands preserve FEATURE_FLAGS"
            End
        End

        It 'detects the unsupported space-separated flag encoding'
            FEATURE_FLAGS="NVIDIA_CUDA_PREBAKE NVIDIA_DKMS_PARK"
            When call rendered_flags_are_preserved ./vhdbuilder/packer/vhd-image-builder-base.json
            The status should be failure
            The output should include "feature flags split across command arguments"
        End
    End

    Describe 'real install and validation consumers'
        NVIDIA_DRIVER_IMAGE="example/aks-gpu-cuda-lts"
        NVIDIA_DRIVER_IMAGE_TAG="580.126.09-test"
        CTR_GPU_INSTALL_CMD="ctr run"
        OS_VARIANT=""
        waitForContainerdReady() { :; }
        configureNvidiaCDIRefresh() { :; }
        isMarinerOrAzureLinux() { return 1; }
        isACL() { return 1; }
        systemctl() { :; }
        which() { return 0; }
        mkdir() {
            case "$*" in *"${TEST_DIR}"*) command mkdir "$@" ;; *) return 0 ;; esac
        }
        ctr() {
            case "$*" in *"images ls"*) echo "${NVIDIA_DRIVER_IMAGE}:${NVIDIA_DRIVER_IMAGE_TAG}" ;; esac
        }
        retrycmd_if_failure() {
            shift 3
            case "$1" in
                bash)
                    [ -f "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/source/dkms.conf" ] || return 99
                    echo "driver command: $*"
                    MODULE_LOAD_FAIL=false
                    ;;
                nvidia-modprobe) [ "${MODULE_LOAD_FAIL:-false}" != true ] ;;
                nvidia-smi) echo "healthy GPU" ;;
                ldconfig) : ;;
                *) return 99 ;;
            esac
        }

        It 'uses ordinary install after restoration, not install-skip-build'
            parked_registration
            When call ensureGPUDrivers
            The status should be success
            The output should include "event=dkms_restored"
            The output should include "gpuinstall /entrypoint.sh install"
            The output should not include "install-skip-build"
        End

        It 'restores before validation even when an installed module already loads'
            parked_registration
            CONFIG_GPU_DRIVER_IF_NEEDED=false
            When call ensureGPUDrivers
            The status should be success
            The line 1 of output should equal "AKS_GPU_PREBAKE event=dkms_restored"
            The output should include "gpu driver working fine"
            The output should not include "driver command:"
            The path "${GPU_DKMS_ACTIVE_DIR}/${DRIVER_VERSION}/source/dkms.conf" should be file
        End

        It 'preserves validation fallback to ordinary installation for a kernel without a loadable module'
            parked_registration
            CONFIG_GPU_DRIVER_IF_NEEDED=false
            KERNEL="6.8.0-new-azure"
            MODULE_LOAD_FAIL=true
            When call ensureGPUDrivers
            The status should be success
            The output should include "event=dkms_restored"
            The output should include "gpuinstall /entrypoint.sh install"
            The output should include "gpu driver working fine"
            The output should not include "install-skip-build"
        End
    End

    Describe 'PIS nodePrep dispatch'
        eval "$(sed -n '/^function nodePrep {/,/^}/p' ./parts/linux/cloud-init/artifacts/cse_main.sh)"
        should_skip_nvidia_drivers() { echo "${LIVE_SKIP}"; }
        reconcileVulnerableKernelModuleMitigation() { :; }
        isAmdAmaEnabledNode() { return 1; }
        retrycmd_if_failure() { return 0; }
        checkServiceHealth() { :; }
        systemctl() { return 1; }
        API_SERVER_NAME="127.0.0.1"
        ERR_NVIDIA_DRIVER_INSTALL=83

        # Execute the real GPU boundary and stop before unrelated hardware/services. For CPU
        # and opt-out paths continue through nodePrep to its real cleanup boundary instead.
        logs_to_events() {
            case "$1" in
                AKS.CSE.ensureGPUDrivers)
                    ensureGPUDrivers
                    exit "$?"
                    ;;
                AKS.CSE.cleanUpGPUDrivers)
                    cleanUpPrebakedGPUDriver
                    exit "$?"
                    ;;
                AKS.CSE.ensureGPUDrivers.*) shift; eval "$*" ;;
                *) return 0 ;;
            esac
        }
        configGPUDrivers() {
            [ -d "${GPU_DKMS_ACTIVE_DIR}" ] && [ ! -e "${GPU_DKMS_PARKED_DIR}" ] || return 99
            echo "nodePrep install"
        }
        validateGPUDrivers() {
            [ -d "${GPU_DKMS_ACTIVE_DIR}" ] && [ ! -e "${GPU_DKMS_PARKED_DIR}" ] || return 99
            echo "nodePrep validation"
        }
        basePrep() { echo "unexpected basePrep"; return 1; }

        provision_cached_image() {
            touch "${TEST_DIR}/base_prep.complete"
            # Run the shipped stage gates, redirecting only the marker path into the fixture.
            eval "$(sed -n '/^if \[ ! -f \/opt\/azure\/containers\/base_prep.complete \]/,/^echo "Custom script finished."/p' \
                ./parts/linux/cloud-init/artifacts/cse_main.sh |
                sed "s|/opt/azure/containers/base_prep.complete|${TEST_DIR}/base_prep.complete|g")"
        }

        Context 'managed CUDA'
            Parameters
                true 'nodePrep install'
                false 'nodePrep validation'
            End
            It 'skips basePrep and restores in nodePrep using the live opt-out decision'
                parked_registration
                skip_nvidia_driver_install=true
                LIVE_SKIP=false
                PRE_PROVISION_ONLY=false
                CONFIG_GPU_DRIVER_IF_NEEDED="$1"
                When run provision_cached_image
                The status should be success
                The output should include "Skipping basePrep"
                The output should include "event=dkms_restored"
                The output should include "$2"
                The output should not include "unexpected basePrep"
            End
        End

        Context 'CPU and opt-out'
            Parameters
                false false
                true true
            End
            It 'cleans the parked cache without activating it on PIS nodes'
                parked_registration
                GPU_NODE="$1"
                LIVE_SKIP="$2"
                skip_nvidia_driver_install=false
                PRE_PROVISION_ONLY=false
                When run provision_cached_image
                The status should be success
                The output should include "Skipping basePrep"
                The output should include "event=teardown"
                The output should include "parked_after=false"
                The output should not include "event=dkms_restored"
                The path "${GPU_DKMS_ACTIVE_DIR}" should not be exist
                The path "${GPU_DKMS_PARKED_DIR}" should not be exist
            End
        End

        It 'does not activate registration during pre-provision-only execution'
            parked_registration
            PRE_PROVISION_ONLY=true
            When run provision_cached_image
            The status should be success
            The output should include "Skipping nodePrep"
            The output should not include "event=dkms_restored"
            The path "${GPU_DKMS_PARKED_DIR}" should be directory
        End
    End
End
