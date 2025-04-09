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
    }
    BeforeAll 'setup'
    Include "./parts/linux/cloud-init/artifacts/mariner/cse_install_mariner.sh"
    Describe 'installDeps'
        It 'installs the required packages with installDeps for Mariner 2.0'
            OS_VERSION="2.0"
            When run installDeps
            The stdout should include "dnf install 30 1 600 mariner-repos-cloud-native"
        End
        It 'installs the required packages with installDeps for AzureLinux 3.0'
            OS_VERSION="3.0"
            When run installDeps
            The stdout should include "dnf install 30 1 600 azurelinux-repos-cloud-native"
        End
    End
End