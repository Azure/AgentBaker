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
    }
    BeforeAll 'setup'
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

        ln() {
            echo "ln $@"
        }

        install() {
            echo "install $@"
        }

        BeforeEach 'setup_rpm_cache'
        AfterEach 'cleanup_rpm_cache'

        It 'installs cached dependency RPMs when they are present'
            desiredVersion="1.34.0-5.azl3"
            rpmDir="$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
            kubeletRpm="$rpmDir/kubelet-${desiredVersion}.x86_64.rpm"
            dependencyRpm="$rpmDir/containernetworking-plugins-1.7.1-4.azl3.x86_64.rpm"
            conflictRpm="$rpmDir/kubelet-1.34.1-4.azl3.x86_64.rpm"
            touch "$kubeletRpm"
            touch "$dependencyRpm"
            touch "$conflictRpm"
            When call installRPMPackageFromFile kubelet "$desiredVersion"
            The output should include "Skipping cached kubelet rpm $(basename "$conflictRpm") because it does not match desired version $desiredVersion"
            The output should include "Installing kubelet with cached dependency RPMs"
            The output should include "$dependencyRpm"
            The output should include "$kubeletRpm"
            The output should include "dnf install 30 1 600"
            The output should include "ln -snf /usr/bin/kubelet /opt/bin/kubelet"
            The output should include "install -m0755 -t /usr/local/bin /usr/bin/kubelet"
        End

        It 'installs only the requested RPM when no cached dependencies exist'
            desiredVersion="1.34.0-5.azl3"
            rpmDir="$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
            kubeletRpm="$rpmDir/kubelet-${desiredVersion}.x86_64.rpm"
            touch "$kubeletRpm"
            When call installRPMPackageFromFile kubelet "$desiredVersion"
            The output should include "dnf install 30 1 600 $kubeletRpm"
            The output should include "ln -snf /usr/bin/kubelet /opt/bin/kubelet"
            The output should include "install -m0755 -t /usr/local/bin /usr/bin/kubelet"
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
End
