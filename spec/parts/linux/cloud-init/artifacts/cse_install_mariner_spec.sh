#!/bin/bash

Describe 'cse_install_mariner.sh'
    setup() {
        # Mock the functions that are not needed to actually run for this test
        function dnf_makecache() {
            return 0
        }
        function dnf_update() {
            return 0
        }
        function dnf_install() {
            echo "dnf install $*"
            return 0
        }
        function systemctl() {
            return 0
        }
        function logs_to_events() {
            echo "$2"
            return 0
        }
        function fallbackToKubeBinaryInstall() {
            return 1
        }
    }
    BeforeAll 'setup'
    Include "./parts/linux/cloud-init/artifacts/cse_install.sh"
    Include "./parts/linux/cloud-init/artifacts/mariner/cse_install_mariner.sh"
    Describe 'installDeps'
        It 'installs the required packages with installDeps for Mariner 2.0'
            OS_VERSION="2.0"
            When call installDeps
            The output line 1 should include "Installing mariner-repos-cloud-native"
        End
        It 'installs the required packages with installDeps for AzureLinux 3.0'
            OS_VERSION="3.0"
            When call installDeps
            The output line 1 should include "Installing azurelinux-repos-cloud-native"
        End
    End

    Describe 'installRPMPackageFromFile'
        rpm_cache_root="$PWD/spec/tmp/rpm-cache"

        setup_rpm_cache() {
            RPM_PACKAGE_CACHE_BASE_DIR="$rpm_cache_root"
            mkdir -p "$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
        }

        cleanup_rpm_cache() {
            rm -rf "$rpm_cache_root"
        }

        BeforeEach 'setup_rpm_cache'
        AfterEach 'cleanup_rpm_cache'

        It 'extracts the requested RPM when cached dependency RPMs are present'
            desiredVersion="1.34.0-5.azl3"
            rpmDir="$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
            kubeletRpm="$rpmDir/kubelet-${desiredVersion}.x86_64.rpm"
            dependencyRpm="$rpmDir/containernetworking-plugins-1.7.1-4.azl3.x86_64.rpm"
            conflictRpm="$rpmDir/kubelet-1.34.1-4.azl3.x86_64.rpm"
            touch "$kubeletRpm"
            touch "$dependencyRpm"
            touch "$conflictRpm"
            When call installRPMPackageFromFile kubelet "$desiredVersion"
            The output should include "extractBinaryFromRPM $kubeletRpm kubelet /opt/bin/kubelet"
        End

        It 'extracts only the requested RPM when no cached dependencies exist'
            desiredVersion="1.34.0-5.azl3"
            rpmDir="$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
            kubeletRpm="$rpmDir/kubelet-${desiredVersion}.x86_64.rpm"
            touch "$kubeletRpm"
            When call installRPMPackageFromFile kubelet "$desiredVersion"
            The output should include "extractBinaryFromRPM $kubeletRpm kubelet /opt/bin/kubelet"
        End

        It 'selects the latest matching release when multiple cached RPMs exist'
            desiredVersion="1.34.3"
            rpmDir="$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
            release1="$rpmDir/kubelet-1.34.3-1.azl3.x86_64.rpm"
            release2="$rpmDir/kubelet-1.34.3-2.azl3.x86_64.rpm"
            touch "$release1"
            touch "$release2"
            When call installRPMPackageFromFile kubelet "$desiredVersion"
            The output should include "extractBinaryFromRPM $release2 kubelet /opt/bin/kubelet"
            The output should not include "$release1"
        End

        It 'returns failure when no cached RPM is found and dnf list finds no version'
            fallbackToKubeBinaryInstall() { return 1; }
            dnf() { echo ""; }
            desiredVersion="1.99.0"
            When call installRPMPackageFromFile kubelet "$desiredVersion"
            The output should include "Failed to find valid kubelet version for 1.99.0"
            The error should include "Failed to query kubelet versions (non-retryable error):"
            The status should equal 1
        End

        It 'strips RPM epoch before matching and downloading package version'
            fallbackToKubeBinaryInstall() { return 1; }
            dnf() {
                echo "kubelet.x86_64 1:1.34.8-2.azl3 azurelinux-official-cloud-native"
                return 0
            }
            desiredVersion="1.34.8"
            rpmDir="$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
            kubeletRpm="$rpmDir/kubelet-${desiredVersion}-2.azl3.x86_64.rpm"
            downloadPkgFromVersion() {
                echo "downloadPkgFromVersion $1 $2 $3"
                touch "$kubeletRpm"
            }

            When call installRPMPackageFromFile kubelet "$desiredVersion"

            The output should include "downloadPkgFromVersion kubelet 1.34.8-2.azl3 $rpmDir"
            The output should include "extractBinaryFromRPM $kubeletRpm kubelet /opt/bin/kubelet"
            The output should not include "1:1.34.8-2.azl3"
            The status should equal 0
        End

        It 'retries dnf list after a transient repo metadata GPG error'
            fallbackToKubeBinaryInstall() { return 1; }
            dnf_makecache() { echo "dnf makecache"; }
            sleep() { echo "sleep $1"; }
            dnfListCallsFile="$RPM_PACKAGE_CACHE_BASE_DIR/dnf-list-calls"
            echo 0 > "$dnfListCallsFile"
            dnf() {
                if [ "$1" = "clean" ]; then
                    echo "dnf clean $2"
                    return 0
                fi

                if [ "$1" = "list" ]; then
                    dnfListCalls=$(cat "$dnfListCallsFile")
                    dnfListCalls=$((dnfListCalls + 1))
                    echo "$dnfListCalls" > "$dnfListCallsFile"
                    if [ "$dnfListCalls" -eq 1 ]; then
                        echo "Error: Failed to download metadata for repo 'azurelinux-official-cloud-native': repomd.xml GPG signature verification error: Bad GPG signature"
                        return 1
                    fi
                    echo "kubelet.x86_64 1.34.8-2.azl3 azurelinux-official-cloud-native"
                    return 0
                fi
            }
            desiredVersion="1.34.8"
            rpmDir="$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
            kubeletRpm="$rpmDir/kubelet-${desiredVersion}-2.azl3.x86_64.rpm"
            downloadPkgFromVersion() { touch "$kubeletRpm"; }

            When call installRPMPackageFromFile kubelet "$desiredVersion"

            The output should include "sleep 10"
            The error should include "repo metadata error"
            The error should include "dnf clean metadata"
            The error should include "dnf makecache"
            The output should include "extractBinaryFromRPM $kubeletRpm kubelet /opt/bin/kubelet"
            The status should equal 0
        End
    End

    Describe 'should_use_nvidia_open_drivers'
        # Tests for the GPU driver selection logic
        # Returns 0 (true) for open driver (A100+, H100, H200, etc.)
        # Returns 1 (false) for proprietary driver (T4, V100)
        # Mocks get_compute_sku to return specific VM SKU for testing

        # Variable to hold mocked VM SKU
        MOCK_VM_SKU=""
        # Override get_compute_sku to return mocked value
        get_compute_sku() {
            echo "$MOCK_VM_SKU"
        }
        set_mock_sku() {
            MOCK_VM_SKU="$1"
        }

        It 'returns false (1) for T4 GPU SKU Standard_NC4as_T4_v3'
            set_mock_sku "Standard_NC4as_T4_v3"
            When call should_use_nvidia_open_drivers
            The status should equal 1
        End

        It 'returns false (1) for T4 GPU SKU Standard_NC64as_T4_v3'
            set_mock_sku "Standard_NC64as_T4_v3"
            When call should_use_nvidia_open_drivers
            The status should equal 1
        End

        It 'returns false (1) for T4 GPU SKU with lowercase standard_nc8as_t4_v3'
            set_mock_sku "standard_nc8as_t4_v3"
            When call should_use_nvidia_open_drivers
            The status should equal 1
        End

        It 'returns false (1) for V100 NDv2 SKU Standard_ND40rs_v2'
            set_mock_sku "Standard_ND40rs_v2"
            When call should_use_nvidia_open_drivers
            The status should equal 1
        End

        It 'returns false (1) for V100 NDv3 SKU Standard_ND40s_v3'
            set_mock_sku "Standard_ND40s_v3"
            When call should_use_nvidia_open_drivers
            The status should equal 1
        End

        It 'returns false (1) for V100 NCsv3 SKU Standard_NC6s_v3'
            set_mock_sku "Standard_NC6s_v3"
            When call should_use_nvidia_open_drivers
            The status should equal 1
        End

        It 'returns false (1) for V100 NCsv3 SKU Standard_NC24s_v3'
            set_mock_sku "Standard_NC24s_v3"
            When call should_use_nvidia_open_drivers
            The status should equal 1
        End

        It 'returns true (0) for A100 SKU Standard_ND96asr_v4'
            set_mock_sku "Standard_ND96asr_v4"
            When call should_use_nvidia_open_drivers
            The status should equal 0
        End

        It 'returns true (0) for A100 SKU Standard_NC24ads_A100_v4'
            set_mock_sku "Standard_NC24ads_A100_v4"
            When call should_use_nvidia_open_drivers
            The status should equal 0
        End

        It 'returns true (0) for A100 SKU Standard_NC96ads_A100_v4'
            set_mock_sku "Standard_NC96ads_A100_v4"
            When call should_use_nvidia_open_drivers
            The status should equal 0
        End

        It 'returns true (0) for H100 SKU Standard_ND96isr_H100_v5'
            set_mock_sku "Standard_ND96isr_H100_v5"
            When call should_use_nvidia_open_drivers
            The status should equal 0
        End

        It 'returns true (0) for H200 SKU Standard_ND96isr_H200_v5'
            set_mock_sku "Standard_ND96isr_H200_v5"
            When call should_use_nvidia_open_drivers
            The status should equal 0
        End

        It 'returns true (0) for NVadsA10 SKU Standard_NV36ads_A10_v5'
            set_mock_sku "Standard_NV36ads_A10_v5"
            When call should_use_nvidia_open_drivers
            The status should equal 0
        End

        It 'handles mixed case VM SKU names correctly'
            set_mock_sku "STANDARD_NC4AS_T4_V3"
            When call should_use_nvidia_open_drivers
            The status should equal 1
        End

        It 'handles lowercase VM SKU names correctly for open driver'
            set_mock_sku "standard_nd96asr_v4"
            When call should_use_nvidia_open_drivers
            The status should equal 0
        End
    End

    Describe 'downloadGPUDrivers grid vs cuda selection'
        # Tests the routing logic in downloadGPUDrivers():
        # NVIDIA_GPU_DRIVER_TYPE="grid" → downloadGridDrivers (converged A10 sizes)
        # NVIDIA_GPU_DRIVER_TYPE="cuda" → cuda/cuda-open path (all other GPU sizes)
        #
        # We mock downloadGridDrivers and the cuda download path to isolate
        # the selection logic without triggering actual downloads or exits.

        MOCK_VM_SKU=""
        get_compute_sku() { echo "$MOCK_VM_SKU"; }

        # Track which path was taken
        GRID_CALLED=""
        downloadGridDrivers() { GRID_CALLED="true"; }

        # Mock should_use_nvidia_open_drivers to avoid IMDS dependency
        MOCK_OPEN_RET=0
        should_use_nvidia_open_drivers() { return "$MOCK_OPEN_RET"; }

        # Mock uname to return a kernel version matching our fake package
        uname() { echo "6.6.121.1-1.azl3"; }

        # Mock dnf repoquery to return fake packages matching both cuda and cuda-open patterns
        dnf() {
            echo "cuda-open-570.195.03-1_6.6.121.1.1.azl3.x86_64"
            echo "cuda-570.195.03-1_6.6.121.1.1.azl3.x86_64"
        }

        It 'selects GRID driver path when NVIDIA_GPU_DRIVER_TYPE is grid'
            NVIDIA_GPU_DRIVER_TYPE="grid"
            MOCK_VM_SKU="Standard_NV36ads_A10_v5"
            GRID_CALLED=""
            When call downloadGPUDrivers
            The output should include "NVIDIA GRID driver (converged)"
            The variable GRID_CALLED should equal "true"
        End

        It 'selects GRID driver path for NCads_A10_v4 converged size'
            NVIDIA_GPU_DRIVER_TYPE="grid"
            MOCK_VM_SKU="Standard_NC8ads_A10_v4"
            GRID_CALLED=""
            When call downloadGPUDrivers
            The output should include "NVIDIA GRID driver (converged)"
            The variable GRID_CALLED should equal "true"
        End

        It 'fails fast for grid-v20 (Ubuntu-only) instead of installing a CUDA driver'
            # RTX PRO 6000 BSE v6 maps to grid-v20, which ships only as the
            # aks-gpu-grid-v20 container image consumed on Ubuntu. There is no
            # nvidia-vgpu-guest-driver v20 RPM for Mariner/AzureLinux, so the guard
            # must exit with ERR_NVIDIA_DRIVER_INSTALL rather than silently falling
            # through to the cuda path on a vGPU node. Use 'run' so the guard's exit
            # is captured as a status instead of aborting the example.
            ERR_NVIDIA_DRIVER_INSTALL=224
            NVIDIA_GPU_DRIVER_TYPE="grid-v20"
            MOCK_VM_SKU="Standard_NC128ds_xl_RTXPRO6000BSE_v6"
            When run downloadGPUDrivers
            The status should equal "$ERR_NVIDIA_DRIVER_INSTALL"
            The output should include "only supported on Ubuntu"
            The output should not include "converged"
        End

        It 'selects cuda-open path for A100 when NVIDIA_GPU_DRIVER_TYPE is cuda'
            NVIDIA_GPU_DRIVER_TYPE="cuda"
            MOCK_VM_SKU="Standard_ND96asr_v4"
            MOCK_OPEN_RET=0
            GRID_CALLED=""
            When call downloadGPUDrivers
            The output should include "NVIDIA OpenRM driver (cuda-open)"
            The variable GRID_CALLED should not equal "true"
        End

        It 'selects proprietary cuda path for T4 when NVIDIA_GPU_DRIVER_TYPE is cuda'
            NVIDIA_GPU_DRIVER_TYPE="cuda"
            MOCK_VM_SKU="Standard_NC4as_T4_v3"
            MOCK_OPEN_RET=1
            GRID_CALLED=""
            When call downloadGPUDrivers
            The output should include "NVIDIA proprietary driver (cuda)"
            The variable GRID_CALLED should not equal "true"
        End

        It 'does not select GRID path when NVIDIA_GPU_DRIVER_TYPE is empty'
            NVIDIA_GPU_DRIVER_TYPE=""
            MOCK_VM_SKU="Standard_ND96asr_v4"
            MOCK_OPEN_RET=0
            GRID_CALLED=""
            When call downloadGPUDrivers
            The output should not include "NVIDIA GRID driver"
            The variable GRID_CALLED should not equal "true"
        End
    End

    Describe 'installAznfsPackage'
        ERR_AZNFS_INSTALL_FAIL=242
        aznfs_test_dir="$PWD/spec/tmp/aznfs-test"

        setup_aznfs() {
            mkdir -p "${aznfs_test_dir}/opt/aznfs/downloads"
            # Create a fake aznfs RPM file
            touch "${aznfs_test_dir}/opt/aznfs/downloads/aznfs-3.0.15-1.x86_64.rpm"
        }

        cleanup_aznfs() {
            rm -rf "${aznfs_test_dir}"
        }

        # Mock gpg/rpm to avoid 'command not found' on CI
        gpg() {
            return 0
        }
        rpm() {
            return 0
        }

        BeforeEach 'setup_aznfs'
        AfterEach 'cleanup_aznfs'

        It 'skips install on non-AzureLinux 3.0'
            OS_VERSION="2.0"
            When call installAznfsPackage
            The output should include "only supported on Azure Linux 3.0"
        End

        It 'installs pre-downloaded RPM on AzureLinux 3.0'
            OS_VERSION="3.0"
            # Override findAznfsRpm to return our test RPM
            findAznfsRpm() {
                echo "${aznfs_test_dir}/opt/aznfs/downloads/aznfs-3.0.15-1.x86_64.rpm"
            }
            When call installAznfsPackage
            The output should include "Installing aznfs from pre-downloaded RPM"
        End

        It 'fails when pre-downloaded RPM is not found'
            OS_VERSION="3.0"
            # Override findAznfsRpm to return empty (no RPM found)
            findAznfsRpm() {
                echo ""
            }
            When call installAznfsPackage
            The output should include "aznfs RPM not found"
            The status should equal 242
        End
    End

    Describe 'managedGPUPackageList on Mariner'
        BeforeEach 'setup'
        setup() {
            ENABLE_MANAGED_GPU_EXPERIENCE=""
            ENABLE_MANAGED_GPU_EXPERIENCE_DRA=""
        }

        It 'returns base managed GPU packages by default'
            When call managedGPUPackageList

            The status should be success
            The output should equal 'datacenter-gpu-manager-4-core datacenter-gpu-manager-4-proprietary dcgm-exporter'
            The output should not include 'nvidia-device-plugin'
            The output should not include 'dra-driver-nvidia-gpu'
        End

        It 'includes nvidia-device-plugin when managed GPU experience is enabled'
            ENABLE_MANAGED_GPU_EXPERIENCE="true"

            When call managedGPUPackageList

            The status should be success
            The output should include 'datacenter-gpu-manager-4-core'
            The output should include 'datacenter-gpu-manager-4-proprietary'
            The output should include 'dcgm-exporter'
            The output should include 'nvidia-device-plugin'
            The output should not include 'dra-driver-nvidia-gpu'
        End

        It 'includes dra-driver-nvidia-gpu when DRA mode is enabled'
            ENABLE_MANAGED_GPU_EXPERIENCE_DRA="true"

            When call managedGPUPackageList

            The status should be success
            The output should include 'datacenter-gpu-manager-4-core'
            The output should include 'datacenter-gpu-manager-4-proprietary'
            The output should include 'dcgm-exporter'
            The output should include 'dra-driver-nvidia-gpu'
            The output should not include 'nvidia-device-plugin'
        End
    End
End
