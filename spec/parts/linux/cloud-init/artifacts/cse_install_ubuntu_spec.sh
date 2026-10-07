#!/bin/bash

Describe 'cse_install_ubuntu.sh'
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"

    Describe 'cleanUpPrebakedGPUDriver'
        It 'is a no-op when the prebake marker is absent'
            GPU_DKMS_MARKER_FILE="$(mktemp)"; rm -f "${GPU_DKMS_MARKER_FILE}"
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should equal ""
        End

        It 'deregisters the nvidia DKMS module and removes baked artifacts (libs, binaries, marker) when present'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            lsmod() { echo ""; }  # no nvidia module loaded
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "Removing pre-baked NVIDIA driver"
            # deregisters via the DKMS source tree + built module removal (no slow dkms remove)
            The output should include "mock rm -rf /var/lib/dkms/nvidia"
            The output should include "mock rm -f /lib/modules"
            # relocated userspace libs
            The output should include "mock rm -rf /usr/bin/lib64"
            # driver userspace binaries so nvidia-smi becomes "command not found" on non-GPU nodes
            The output should include "mock rm -f /usr/bin/nvidia-smi"
            The output should include "mock ldconfig"
            # the slow per-version dkms remove --all must NOT be on the critical path anymore
            The output should not include "dkms remove"
            # stage-1 observability: a structured outcome line is emitted. Here rm is mocked, so the
            # marker is left in place and the cleanup correctly reports an incomplete (security-gap) result.
            The output should include "AKS_GPU_PREBAKE event=teardown"
            The output should include "status=incomplete"
        End

        It 'reports status=cleaned once the marker and DKMS state are actually gone'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            ldconfig() { echo "mock ldconfig"; }
            lsmod() { echo ""; }  # no nvidia module loaded (grid-style prebake)
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "AKS_GPU_PREBAKE event=teardown"
            The output should include "status=cleaned"
            The output should include "marker_after=false"
            # the setuid nvidia-modprobe is part of the security-coverage check
            The output should include "modprobe_after=false"
            The output should include "module_before=false"
            The output should include "module_after=false"
        End

        It 'unloads an idle prebaked nvidia module that auto-loaded at boot (cuda/cuda-lts SKUs)'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            ldconfig() { echo "mock ldconfig"; }
            # simulate a loaded-but-idle module: lsmod shows nvidia until rmmod is "run"
            _nvidia_loaded=true
            lsmod() { if [ "${_nvidia_loaded}" = true ]; then echo "nvidia 104165376 0"; else echo ""; fi; }
            cat() { echo "0"; }        # /sys/module/nvidia/refcnt = 0 (idle)
            ls() { return 1; }          # no /dev/nvidia* device nodes
            rmmod() { _nvidia_loaded=false; echo "mock rmmod $*"; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "mock rmmod nvidia"
            The output should include "module_before=true"
            The output should include "module_after=false"
            The output should include "status=cleaned"
        End

        It 'keeps the marker (incomplete) when a busy nvidia module cannot be unloaded'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            ldconfig() { echo "mock ldconfig"; }
            lsmod() { echo "nvidia 104165376 2"; }  # stays loaded (refcnt shows in-use)
            cat() { echo "2"; }                       # refcnt != 0 -> do not rmmod
            ls() { return 1; }
            rmmod() { echo "mock rmmod $*"; }         # should NOT be called
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should not include "mock rmmod"
            The output should include "module_before=true"
            The output should include "module_after=true"
            The output should include "status=incomplete"
        End

        It 'keeps the marker if any prebaked userspace artifact remains'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="$marker"
            rm() { echo "mock rm $*"; }
            ldconfig() { :; }
            lsmod() { :; }
            prebakedGPUDriverArtifactsRemain() { return 0; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "status=incomplete"
            The output should not include "mock rm -f $marker"
        End

        It 'keeps the marker when loaded-module inspection fails'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="$marker"
            rm() { echo "mock rm $*"; }
            ldconfig() { :; }
            lsmod() { return 1; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "status=incomplete"
            The output should not include "mock rm -f $marker"
        End
    End

    Describe 'cleanUpGPUDriversForBasePrep'
        GPU_DEST="${PWD}/.shellspec-nonexistent-gpu-dest"
        managedGPUPackageList() { :; }

        createGraceBlackwellFixture() {
            local wave="${1:-wave2}"
            GB_MAI_BOM_FILE="${PWD}/.shellspec-gb-bom-$$.json"
            GB200_MAI_BOM_FILE="${PWD}/.shellspec-gb200-bom-$$.json"
            GPU_DKMS_MARKER_FILE="${PWD}/.shellspec-gpu-marker-$$"
            GB_NVIDIA_PEERMEM_CONFIG_FILE="${PWD}/.shellspec-nvidia-peermem-$$.conf"
            GB_NVIDIA_MODPROBE_CONFIG_FILE="${PWD}/.shellspec-nvidia-modprobe-$$.conf"
            GB_NOUVEAU_MODPROBE_CONFIG_FILE="${PWD}/.shellspec-nouveau-$$.conf"
            GB_DRIVER_CLEANUP_PENDING_FILE="${PWD}/.shellspec-gb-cleanup-pending-$$"
            if [ "$wave" = wave1 ]; then
                printf '%s\n' '{"versions-wave1":{"libnvidia-common-580":"580.105.08-0ubuntu1","nvidia-dkms-580-open":"580.105.08-0ubuntu1","nvidia-driver-580-open":"580.105.08-0ubuntu1"}}' > "$GB200_MAI_BOM_FILE"
                printf '%s\n' 'options nvidia NVreg_RestrictProfilingToAdminUsers=0' \
                    'options nvidia NVreg_CreateImexChannel0=1' \
                    'options nvidia NVreg_CoherentGPUMemoryMode=driver' \
                    'options nvidia NVreg_RegistryDwords="RMBug5172204War=4"' > "$GB_NVIDIA_MODPROBE_CONFIG_FILE"
            else
                printf '%s\n' '{"versions-wave2":{"libnvidia-common-580":"580.159.04-1ubuntu1","nvidia-dkms-580-open":"580.159.04-1ubuntu1","nvidia-driver-580-open":"580.159.04-1ubuntu1"}}' > "$GB_MAI_BOM_FILE"
                cp parts/linux/cloud-init/artifacts/ubuntu/gb/nvidia-peermem.conf "$GB_NVIDIA_PEERMEM_CONFIG_FILE"
                cp parts/linux/cloud-init/artifacts/ubuntu/gb/modprobe-nvidia-parameters.conf "$GB_NVIDIA_MODPROBE_CONFIG_FILE"
                printf 'blacklist nouveau\noptions nouveau modeset=0\n' > "$GB_NOUVEAU_MODPROBE_CONFIG_FILE"
            fi
        }

        cleanupGraceBlackwellFixture() {
            rm -f "${GB_MAI_BOM_FILE:-}" "${GB200_MAI_BOM_FILE:-}" "${GB_DRIVER_CLEANUP_PENDING_FILE:-}" "${GPU_DKMS_MARKER_FILE:-}" \
                "${GB_NVIDIA_PEERMEM_CONFIG_FILE:-}" "${GB_NVIDIA_MODPROBE_CONFIG_FILE:-}" \
                "${GB_NOUVEAU_MODPROBE_CONFIG_FILE:-}"
        }

        runGraceBlackwellFailureAndRetainState() {
            cleanUpGPUDriversForBasePrep
            status=$?
            [ -f "$GPU_DKMS_MARKER_FILE" ] && echo "aks marker retained" || status=0
            [ -f "$GB_NVIDIA_PEERMEM_CONFIG_FILE" ] && echo "GB boot config retained"
            cleanupGraceBlackwellFixture
            return "$status"
        }

        mockGraceBlackwellPackageQueries() {
            dpkg-query() {
                if [ "$#" -eq 2 ]; then
                    [ "${gb_packages_purged:-false}" = true ] && return 0
                    printf 'libnvidia-common-580\tinstalled\nnvidia-dkms-580-open\tinstalled\nnvidia-driver-580-open\tinstalled\nnvidia-utils-580\tinstalled\nnvidia-modprobe\tinstalled\nnvidia-persistenced\tinstalled\n'
                    [ -n "${gb_extra_driver_package:-}" ] && printf '%s\tinstalled\n' "$gb_extra_driver_package"
                    return 0
                fi
                case "$3" in
                    libnvidia-common-580|nvidia-dkms-580-open|nvidia-driver-580-open)
                        if [ "${gb_packages_purged:-false}" = true ]; then
                            printf 'not-installed\t\n'
                        elif [ "${gb_version_mismatch:-false}" = true ] && [ "$3" = nvidia-driver-580-open ]; then
                            printf 'installed\t580.200.01-1ubuntu1\n'
                        else
                            printf 'installed\t%s\n' "${gb_driver_version:-580.159.04-1ubuntu1}"
                        fi
                        ;;
                    nvidia-utils-580)
                        [ "${gb_packages_purged:-false}" = true ] && printf 'not-installed\t\n' || printf 'installed\t%s\n' "${gb_utils_version:-580.159.04-1}"
                        ;;
                    nvidia-modprobe)
                        [ "${gb_packages_purged:-false}" = true ] && printf 'not-installed\t\n' || printf 'installed\t%s\n' "${gb_modprobe_version:-1.10.0-1ubuntu1}"
                        ;;
                    nvidia-persistenced)
                        [ "${gb_packages_purged:-false}" = true ] && printf 'not-installed\t\n' || printf 'installed\t%s\n' "${gb_persistenced_version:-1.10.0-1ubuntu1}"
                        ;;
                    *) printf 'not-installed\t\n' ;;
                esac
            }
            apt-mark() { printf 'nvidia-utils-580\nnvidia-modprobe\nnvidia-persistenced\n'; }
        }

        setupGraceBlackwellTeardown() {
            local wave="${1:-wave2}"
            createGraceBlackwellFixture "$wave"
            : > "$GPU_DKMS_MARKER_FILE"
            gb_driver_version=580.159.04-1ubuntu1
            gb_utils_version=580.159.04-1
            gb_modprobe_version=1.10.0-1ubuntu1
            gb_persistenced_version=1.10.0-1ubuntu1
            if [ "$wave" = wave1 ]; then
                gb_driver_version=580.105.08-0ubuntu1
                gb_utils_version=580.105.08-1
            fi
            gb_loaded_modules='nvidia_peermem nvidia_uvm nvidia_drm nvidia_modeset nvidia '
            gb_expected_modules='nvidia_peermem nvidia_uvm nvidia_drm nvidia_modeset nvidia '
            gb_services_stopped=false
            gb_packages_purged=false
            mockGraceBlackwellPackageQueries
        }

        It 'fails when the prebake marker survives cleanup'
            GPU_DKMS_MARKER_FILE="$(mktemp)"
            cleanUpGPUDrivers() { :; }
            lsmod() { echo ""; }
            When call cleanUpGPUDriversForBasePrep
            The status should be failure
            The stderr should include "GPU basePrep cleanup incomplete"
            The path "$GPU_DKMS_MARKER_FILE" should be exist
            AfterRun 'rm -f "$GPU_DKMS_MARKER_FILE"'
        End

        It 'fails when nvidia remains loaded without a prebake marker'
            GPU_DKMS_MARKER_FILE="${PWD}/.shellspec-absent-gpu-marker-$$"
            cleanUpGPUDrivers() { :; }
            lsmod() { echo "nvidia 104165376 0"; }
            When call cleanUpGPUDriversForBasePrep
            The status should be failure
            The stderr should include "GPU basePrep cleanup incomplete"
        End

        It 'fails when an unmarked prebake artifact remains'
            GPU_DKMS_MARKER_FILE="${PWD}/.shellspec-absent-gpu-marker-$$"
            cleanUpGPUDrivers() { :; }
            compgen() { [ "$2" = '/lib/modules/*/updates/dkms/nvidia*.ko*' ]; }
            lsmod() { echo ""; }
            When call cleanUpGPUDriversForBasePrep
            The status should be failure
            The stderr should include "GPU basePrep cleanup incomplete"
        End

        It 'tears down current and legacy BOM-owned Grace Blackwell drivers before marker cleanup'
            runGraceBlackwellTeardown() {
                local wave="${1:-wave2}" retry="${2:-false}"
                setupGraceBlackwellTeardown "$wave"
                systemctlDisableAndStop() {
                    echo "stop:$1"
                    if [ "$1" = openibd ]; then
                        gb_services_stopped=true
                    fi
                    return 0
                }
                systemctl() { return 1; }
                lsmod() {
                    echo "Module Size Used by"
                    for gb_module in $gb_loaded_modules; do
                        echo "$gb_module 123 0"
                    done
                }
                rmmod() {
                    [ "$gb_services_stopped" = true ] || return 1
                    [ "${gb_expected_modules%% *}" = "$1" ] || return 1
                    echo "rmmod:$1"
                    gb_expected_modules="${gb_expected_modules#* }"
                    gb_loaded_modules="${gb_loaded_modules#* }"
                }
                update-initramfs() {
                    [ "$gb_packages_purged" = true ] || return 1
                    [ ! -e "$GB_NVIDIA_PEERMEM_CONFIG_FILE" ] || return 1
                    [ ! -e "$GB_NVIDIA_MODPROBE_CONFIG_FILE" ] || return 1
                    [ ! -e "$GB_NOUVEAU_MODPROBE_CONFIG_FILE" ] || return 1
                    [ -f "$GB_DRIVER_CLEANUP_PENDING_FILE" ] || return 1
                    echo "update-initramfs"
                }
                apt_get_purge() {
                    [ "$1 $2 $3" = '10 5 300' ] || return 1
                    shift 3
                    local expected='libnvidia-common-580 nvidia-dkms-580-open nvidia-driver-580-open nvidia-utils-580 nvidia-modprobe nvidia-persistenced'
                    [ "$*" = "$expected" ] || return 1
                    [ -e "$GB_NVIDIA_MODPROBE_CONFIG_FILE" ] || return 1
                    [ "$gb_services_stopped" = true ] || return 1
                    [ -z "$gb_loaded_modules" ] || return 1
                    [ -z "$gb_expected_modules" ] || return 1
                    echo "purge:$*"
                    gb_packages_purged=true
                }
                cleanUpGPUDrivers() {
                    [ "$gb_packages_purged" = true ] || return 1
                    [ ! -e "$GB_DRIVER_CLEANUP_PENDING_FILE" ] || return 1
                    echo "aks-marker-cleanup"
                    rm -f "$GPU_DKMS_MARKER_FILE"
                }
                prebakedGPUDriverArtifactsRemain() { [ "$gb_packages_purged" != true ]; }
                cleanUpGPUDriversForBasePrep
                status=$?
                if [ "$status" -eq 0 ] && [ "$retry" = true ]; then
                    cleanUpGPUDriversForBasePrep
                    status=$?
                fi
                cleanupGraceBlackwellFixture
                return "$status"
            }

            runBothGraceBlackwellTeardowns() {
                runGraceBlackwellTeardown wave2 || return 1
                runGraceBlackwellTeardown wave1 true
            }

            When call runBothGraceBlackwellTeardowns
            The status should be success
            The output should include "rmmod:nvidia_peermem"
            The output should include "stop:nvidia-imex"
            The output should include "stop:openibd"
            The output should include "purge:"
            The output should include "aks-marker-cleanup"
        End

        It 'uses a BOM-listed dependency version instead of comparing it to the driver version'
            runGraceBlackwellBOMDependencyVersion() {
                createGraceBlackwellFixture
                gb_driver_version=580.159.04-1ubuntu1
                gb_modprobe_version=1.10.0-1ubuntu1
                printf '%s\n' '{"versions-wave2":{"libnvidia-common-580":"580.159.04-1ubuntu1","nvidia-dkms-580-open":"580.159.04-1ubuntu1","nvidia-driver-580-open":"580.159.04-1ubuntu1","nvidia-modprobe":"1.10.0-1ubuntu1"}}' > "$GB_MAI_BOM_FILE"
                mockGraceBlackwellPackageQueries
                entries=$(graceBlackwellDriverBOMEntries "$GB_MAI_BOM_FILE" versions-wave2) || return 1
                graceBlackwellInstalledDriverPackages "$entries"
                status=$?
                cleanupGraceBlackwellFixture
                return "$status"
            }
            When call runGraceBlackwellBOMDependencyVersion
            The status should be success
            The output should include "nvidia-modprobe=1.10.0-1ubuntu1"
        End

        It 'retains GB provenance and retries after config removal or initramfs failure'
            runGraceBlackwellTeardownRetry() {
                local status=0
                setupGraceBlackwellTeardown
                gb_loaded_modules=""
                mockGraceBlackwellPackageQueries
                systemctlDisableAndStop() { :; }
                systemctl() { return 1; }
                lsmod() { echo "Module Size Used by"; }
                apt_get_purge() { gb_packages_purged=true; }
                gb_fail_config_remove=true
                rm() {
                    if [ "${gb_fail_config_remove:-false}" = true ] &&
                        [ "$*" = "-f $GB_NVIDIA_PEERMEM_CONFIG_FILE $GB_NVIDIA_MODPROBE_CONFIG_FILE $GB_NOUVEAU_MODPROBE_CONFIG_FILE" ]; then
                        gb_fail_config_remove=false
                        return 1
                    fi
                    command rm "$@"
                }
                gb_initramfs_failures=1
                update-initramfs() {
                    [ ! -e "$GB_NVIDIA_PEERMEM_CONFIG_FILE" ] || return 1
                    [ ! -e "$GB_NVIDIA_MODPROBE_CONFIG_FILE" ] || return 1
                    [ ! -e "$GB_NOUVEAU_MODPROBE_CONFIG_FILE" ] || return 1
                    [ -f "$GB_DRIVER_CLEANUP_PENDING_FILE" ] || return 1
                    if [ "$gb_initramfs_failures" -gt 0 ]; then
                        gb_initramfs_failures=$((gb_initramfs_failures - 1))
                        return 1
                    fi
                }
                cleanUpGPUDrivers() {
                    [ "$gb_packages_purged" = true ] || return 1
                    [ ! -e "$GB_DRIVER_CLEANUP_PENDING_FILE" ] || return 1
                    rm -f "$GPU_DKMS_MARKER_FILE"
                }
                prebakedGPUDriverArtifactsRemain() { [ "$gb_packages_purged" != true ]; }

                cleanUpGPUDriversForBasePrep && status=1
                [ -f "$GPU_DKMS_MARKER_FILE" ] || status=1
                [ -f "$GB_DRIVER_CLEANUP_PENDING_FILE" ] || status=1
                [ -f "$GB_NVIDIA_MODPROBE_CONFIG_FILE" ] || status=1
                cleanUpGPUDriversForBasePrep && status=1
                [ -f "$GPU_DKMS_MARKER_FILE" ] || status=1
                [ -f "$GB_DRIVER_CLEANUP_PENDING_FILE" ] || status=1
                [ ! -e "$GB_NVIDIA_MODPROBE_CONFIG_FILE" ] || status=1
                cleanUpGPUDriversForBasePrep || status=1
                [ ! -e "$GB_DRIVER_CLEANUP_PENDING_FILE" ] || status=1
                [ ! -e "$GPU_DKMS_MARKER_FILE" ] || status=1
                cleanupGraceBlackwellFixture
                return "$status"
            }
            When call runGraceBlackwellTeardownRetry
            The status should be success
            The stderr should include "GPU basePrep cleanup incomplete: Grace Blackwell driver teardown failed"
        End

        It 'retains the marker on a legacy GB VHD without a BOM'
            runGraceBlackwellWithoutBOM() {
                status=0
                createGraceBlackwellFixture
                rm -f "$GB_MAI_BOM_FILE"
                : > "$GPU_DKMS_MARKER_FILE"
                cleanUpGPUDrivers() { echo "unexpected marker cleanup"; }
                runGraceBlackwellFailureAndRetainState || status=1
                createGraceBlackwellFixture
                rm -f "$GB_MAI_BOM_FILE" "$GB_NVIDIA_PEERMEM_CONFIG_FILE"
                : > "$GPU_DKMS_MARKER_FILE"
                dpkg-query() {
                    [ "$#" -eq 2 ] && printf 'nvidia-dkms-580-open\tinstalled\n'
                }
                runGraceBlackwellFailureAndRetainState || status=1
                return "$status"
            }
            When call runGraceBlackwellWithoutBOM
            The status should be failure
            The stderr should include "Grace Blackwell BOM is unavailable"
            The stderr should include "has no verifiable GB BOM ownership"
            The output should include "aks marker retained"
            The output should include "GB boot config retained"
            The output should not include "unexpected marker cleanup"
        End

        It 'retains the marker for mixed ownership and package-purge failures'
            runUnsafeGraceBlackwellTeardown() {
                status=0
                setupGraceBlackwellTeardown
                gb_extra_driver_package=nvidia-driver-535-server
                mockGraceBlackwellPackageQueries
                systemctlDisableAndStop() { echo "unexpected service stop"; }
                apt_get_purge() { echo "unexpected package purge"; }
                cleanUpGPUDrivers() { echo "unexpected marker cleanup"; }
                runGraceBlackwellFailureAndRetainState || status=1
                setupGraceBlackwellTeardown
                gb_packages_purged=false
                gb_extra_driver_package=""
                mockGraceBlackwellPackageQueries
                printf '\n# customer change\n' >> "$GB_NVIDIA_MODPROBE_CONFIG_FILE"
                systemctlDisableAndStop() { echo "unexpected service stop"; }
                systemctl() { return 1; }
                lsmod() { echo "nvidia 123 0"; }
                rmmod() { echo "unexpected module unload"; }
                apt_get_purge() { echo "unexpected package purge"; }
                cleanUpGPUDrivers() { echo "unexpected marker cleanup"; }
                runGraceBlackwellFailureAndRetainState || status=1
                setupGraceBlackwellTeardown
                gb_extra_driver_package=""
                mockGraceBlackwellPackageQueries
                lsmod() { echo "Module Size Used by"; }
                systemctlDisableAndStop() { :; }
                systemctl() { return 1; }
                update-initramfs() { :; }
                apt_get_purge() { return 1; }
                runGraceBlackwellFailureAndRetainState || status=1
                setupGraceBlackwellTeardown
                gb_version_mismatch=true
                mockGraceBlackwellPackageQueries
                apt_get_purge() { echo "unexpected package purge"; }
                cleanUpGPUDrivers() { echo "unexpected marker cleanup"; }
                cleanUpGPUDriversForBasePrep || status=1
                cleanupGraceBlackwellFixture
                return "$status"
            }
            When call runUnsafeGraceBlackwellTeardown
            The status should be failure
            The stderr should include "is not owned by the Grace Blackwell BOM"
            The stderr should include "module configuration"
            The stderr should include "Grace Blackwell driver package purge failed"
            The stderr should include "does not match the GB BOM"
            The output should include "aks marker retained"
            The output should include "GB boot config retained"
            The output should not include "unexpected service stop"
            The output should not include "unexpected module unload"
            The output should not include "unexpected package purge"
            The output should not include "unexpected marker cleanup"
        End

        It 'succeeds when the prebake marker and AKS paths are gone'
            GPU_DKMS_MARKER_FILE="$(mktemp)"
            cleanUpGPUDrivers() { rm -f "$GPU_DKMS_MARKER_FILE"; }
            lsmod() { echo ""; }
            When call cleanUpGPUDriversForBasePrep
            The status should be success
            The path "$GPU_DKMS_MARKER_FILE" should not be exist
        End
    End

    Describe 'installPackageFromCache version matching'
        deb_cache_root="${PWD}/.shellspec-deb-cache-$$"

        setup_deb_cache() {
            mkdir -p "$deb_cache_root"
        }

        cleanup_deb_cache() {
            rm -rf "$deb_cache_root"
        }

        BeforeEach 'setup_deb_cache'
        AfterEach 'cleanup_deb_cache'

        # Mock functions used by installPackageFromCache
        fallbackToKubeBinaryInstall() { return 1; }
        logs_to_events() { shift; eval "$@"; }
        extractDebBinaryFromFile() { echo "extractDebBinaryFromFile $1 $2 $3"; }

        It 'does not match version 1.34.10 when requesting 1.34.1'
            downloadDir="$deb_cache_root"
            packageName="kubelet"
            packageVersion="1.34.1"
            touch "$downloadDir/kubelet_1.34.1-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.10-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.11-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.12-1ubuntu22.04u1_amd64.deb"

            result() {
                ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1
            }
            When call result
            The output should equal "kubelet_1.34.1-1ubuntu22.04u1_amd64.deb"
        End

        It 'matches the latest release of the exact version requested'
            downloadDir="$deb_cache_root"
            packageName="kubelet"
            packageVersion="1.34.1"
            touch "$downloadDir/kubelet_1.34.1-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.1-2ubuntu22.04u1_amd64.deb"

            result() {
                ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1
            }
            When call result
            The output should equal "kubelet_1.34.1-2ubuntu22.04u1_amd64.deb"
        End

        It 'returns empty when no matching version exists'
            downloadDir="$deb_cache_root"
            packageName="kubelet"
            packageVersion="1.34.1"
            touch "$downloadDir/kubelet_1.34.10-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.2-1ubuntu22.04u1_amd64.deb"

            result() {
                ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1
            }
            When call result
            The output should equal ""
        End

        It 'matches version with plus suffix (azure convention)'
            downloadDir="$deb_cache_root"
            packageName="kubelet"
            packageVersion="1.34.1"
            touch "$downloadDir/kubelet_1.34.1+azure-1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.10+azure-1_amd64.deb"

            result() {
                ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1
            }
            When call result
            The output should equal "kubelet_1.34.1+azure-1_amd64.deb"
        End
    End

    Describe 'getLatestDebPackageVersion'
        getCPUArch() { echo "amd64"; }

        It 'selects the latest revision for the requested upstream version'
            apt() {
                cat <<'EOF'
Listing...
kubelet/repo 1.34.10-ubuntu22.04u1 amd64
kubelet/repo 1.34.10-ubuntu22.04u9 amd64
kubelet/repo 1.34.11-ubuntu22.04u1 amd64
EOF
            }

            When call getLatestDebPackageVersion kubelet 1.34.10
            The output should equal "1.34.10-ubuntu22.04u9"
        End

        It 'ignores revisions for other architectures'
            apt() {
                cat <<'EOF'
Listing...
kubelet/repo 1.34.10-ubuntu22.04u9 arm64
kubelet/repo 1.34.10-ubuntu22.04u8 amd64
EOF
            }

            When call getLatestDebPackageVersion kubelet 1.34.10
            The output should equal "1.34.10-ubuntu22.04u8"
        End

        It 'does not match a longer patch version'
            apt() {
                cat <<'EOF'
Listing...
kubelet/repo 1.34.1-ubuntu22.04u3 amd64
kubelet/repo 1.34.10-ubuntu22.04u9 amd64
EOF
            }

            When call getLatestDebPackageVersion kubelet 1.34.1
            The output should equal "1.34.1-ubuntu22.04u3"
        End
    End

    Describe 'installContainerdWithAptGet revision comparison'
        containerd_download_root="${PWD}/.shellspec-cse-install-ubuntu-containerd-$$"

        setup_containerd_revision() {
            mkdir -p "${containerd_download_root}"
            CONTAINERD_DOWNLOADS_DIR="${containerd_download_root}"
        }

        cleanup_containerd_revision() {
            rm -rf "${containerd_download_root}"
        }

        BeforeEach 'setup_containerd_revision'
        AfterEach 'cleanup_containerd_revision'

        semverCompare() {
            [ "$1" = "$2" ] && return 0
            [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -n 1)" = "$1" ]
        }
        dpkg() { echo "ii  moby-containerd"; }
        getLatestDebPackageVersion() { echo "1:1.7.35+azure-ubuntu22.04u2"; }
        removeContainerd() { echo "removeContainerd"; }
        downloadContainerdFromVersion() {
            touch "${CONTAINERD_DOWNLOADS_DIR}/moby-containerd_1.7.35+azure-ubuntu22.04u2_amd64.deb"
        }
        installDebPackageFromFile() { echo "installDebPackageFromFile $1"; }
        logs_to_events() {
            shift
            eval "$*"
        }

        It 'installs the latest revision when the installed upstream version is equal but stale'
            dpkg-query() { echo "1:1.7.35+azure-ubuntu22.04u1"; }

            When call installContainerdWithAptGet 1.7.35 "${CONTAINERD_DOWNLOADS_DIR}"

            The output should include "installed moby-containerd package version 1:1.7.35+azure-ubuntu22.04u1 does not match latest revision 1:1.7.35+azure-ubuntu22.04u2"
            The output should include "installDebPackageFromFile ${CONTAINERD_DOWNLOADS_DIR}/moby-containerd_1.7.35+azure-ubuntu22.04u2_amd64.deb"
        End

        It 'skips installation when the latest revision is already installed'
            dpkg-query() { echo "1:1.7.35+azure-ubuntu22.04u2"; }

            When call installContainerdWithAptGet 1.7.35 "${CONTAINERD_DOWNLOADS_DIR}"

            The output should include "currently installed containerd version 1:1.7.35+azure-ubuntu22.04u2 satisfies target version 1.7.35"
            The output should not include "removeContainerd"
            The output should not include "installDebPackageFromFile"
        End
    End

    Describe 'logResolvedPackageVersion'
        resolved_version_log="${PWD}/.shellspec-cse-install-ubuntu-resolved-version-$$"

        cleanup_resolved_version_log() {
            rm -f "${resolved_version_log}"
        }

        BeforeEach 'cleanup_resolved_version_log'
        AfterEach 'cleanup_resolved_version_log'

        It 'does not create the VHD completion marker during node provisioning'
            VHD_LOGS_FILEPATH="${resolved_version_log}"

            When call logResolvedPackageVersion moby-runc 1.4.3 1.4.3-1ubuntu22.04u1

            The output should equal "Resolved moby-runc package version 1.4.3 -> 1.4.3-1ubuntu22.04u1"
            The path "${resolved_version_log}" should not be exist
        End

        It 'appends the resolved version when the VHD completion marker exists'
            VHD_LOGS_FILEPATH="${resolved_version_log}"
            touch "${VHD_LOGS_FILEPATH}"

            When call logResolvedPackageVersion moby-runc 1.4.3 1.4.3-1ubuntu22.04u1

            The output should equal "Resolved moby-runc package version 1.4.3 -> 1.4.3-1ubuntu22.04u1"
            The contents of file "${resolved_version_log}" should include "moby-runc package version 1.4.3-1ubuntu22.04u1 (requested 1.4.3)"
        End
    End

    Describe 'ensureRunc repository fallback'
        runc_download_root="${PWD}/.shellspec-cse-install-ubuntu-runc-$$"

        setup_runc_fallback() {
            mkdir -p "${runc_download_root}"
            RUNC_DOWNLOADS_DIR="${runc_download_root}"
            VHD_LOGS_FILEPATH="${runc_download_root}/missing-vhd-marker"
        }

        cleanup_runc_fallback() {
            rm -rf "${runc_download_root}"
        }

        BeforeEach 'setup_runc_fallback'
        AfterEach 'cleanup_runc_fallback'

        isARM64() { echo 0; }
        getCPUArch() { echo "amd64"; }
        runc() { echo "runc version 1.4.2"; }
        getLatestDebPackageVersion() { echo "1.4.3-10ubuntu22.04u1"; }
        apt_get_install() { echo "apt_get_install $*"; }

        It 'installs the exact latest revision for a revisionless version'
            When call ensureRunc 1.4.3 "" "${RUNC_DOWNLOADS_DIR}"

            The output should include "Resolved moby-runc package version 1.4.3 -> 1.4.3-10ubuntu22.04u1"
            The output should include "apt_get_install 20 30 120 moby-runc=1.4.3-10ubuntu22.04u1 --allow-downgrades"
            The output should not include "moby-runc=1.4.3*"
        End

        It 'installs the latest revision when the installed upstream version is equal but stale'
            runc() { echo "runc version 1.4.3"; }
            dpkg() { echo "ii  moby-runc"; }
            dpkg-query() { echo "1.4.3-1ubuntu22.04u1"; }

            When call ensureRunc 1.4.3 "" "${RUNC_DOWNLOADS_DIR}"

            The output should include "installed moby-runc package version 1.4.3-1ubuntu22.04u1 does not match latest revision 1.4.3-10ubuntu22.04u1"
            The output should include "apt_get_install 20 30 120 moby-runc=1.4.3-10ubuntu22.04u1 --allow-downgrades"
        End

        It 'skips installation when the latest revision is already installed'
            runc() { echo "runc version 1.4.3"; }
            dpkg() { echo "ii  moby-runc"; }
            dpkg-query() { echo "1.4.3-10ubuntu22.04u1"; }

            When call ensureRunc 1.4.3 "" "${RUNC_DOWNLOADS_DIR}"

            The output should include "target moby-runc package version 1.4.3-10ubuntu22.04u1 is already installed"
            The output should not include "apt_get_install"
        End
    End
End
