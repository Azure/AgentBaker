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
End