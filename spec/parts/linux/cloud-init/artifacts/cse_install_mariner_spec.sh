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

        mv() {
            echo "mv $@"
        }

        BeforeEach 'setup_rpm_cache'
        AfterEach 'cleanup_rpm_cache'

        It 'installs cached dependency RPMs when they are present'
            desiredVersion="1.34.0-5.azl3"
            rpmDir="$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
            kubeletRpm="$rpmDir/kubelet-${desiredVersion}.x86_64.rpm"
            dependencyRpm="$rpmDir/containernetworking-plugins-1.7.1-4.azl3.x86_64.rpm"
            touch "$kubeletRpm"
            touch "$dependencyRpm"
            When call installRPMPackageFromFile kubelet "$desiredVersion"
            The output line 2 should include "Installing kubelet with cached dependency RPMs"
            The output line 2 should include "$dependencyRpm"
            The output line 2 should include "$kubeletRpm"
            The output line 3 should include "dnf install 30 1 600"
            The output line 4 should include "mv /usr/bin/kubelet /usr/local/bin/kubelet"
        End

        It 'installs only the requested RPM when no cached dependencies exist'
            desiredVersion="1.34.0-5.azl3"
            rpmDir="$RPM_PACKAGE_CACHE_BASE_DIR/kubelet/downloads"
            kubeletRpm="$rpmDir/kubelet-${desiredVersion}.x86_64.rpm"
            touch "$kubeletRpm"
            When call installRPMPackageFromFile kubelet "$desiredVersion"
            The output line 2 should include "dnf install 30 1 600 $kubeletRpm"
            The output line 3 should include "mv /usr/bin/kubelet /usr/local/bin/kubelet"
        End
    End
End