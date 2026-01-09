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

    Describe 'installToolFromLocalRepo'
        function rm() {
            return 0
        }
        function _dnf_makecache() {
            return 0
        }
        setup_repo(){
            mkdir -p /tmp/to/repo
            mkdir -p /tmp/to/repo/repodata
            mkdir -p /etc/yum.repos.d
        }
        BeforeAll 'setup_repo'
        AfterAll 'rm -rf /tmp/to'

        It 'dnf make cache and install both succeeded'
            When call installToolFromLocalRepo "test-tool" "/tmp/to/repo"
            The output should include "Successfully installed test-tool from local repository"
        End
        It 'dnf make cache failed'
            function _dnf_makecache() {
                return 1
            }
            When call installToolFromLocalRepo "test-tool" "/tmp/to/repo"
            The status should eq 1
            The output should include "Failed to update DNF cache for local repository"
        End
        It 'dnf install failed'
            function dnf_install() {
                return 1
            }
            When call installToolFromLocalRepo "test-tool" "/tmp/to/repo"
            The status should eq 1
            The output should include "Failed to install test-tool from local repository"
            The output should not include "dnf_makecache"
        End
    End
End

Describe 'cse_helpers_mariner.sh'
    setup() {
        # Mock the functions that are not needed to actually run for this test
        function dnf_makecache() {
            echo "dnf_makecache"
            return 0
        }
        function dnf_update() {
            echo "dnf_update"
            return 0
        }

        dnf_call_count=0
        dnf() {
            dnf_call_count=$((dnf_call_count + 1))
            if [ "$dnf_call_count" -eq 3 ]; then
                echo "dnf $*; times: $dnf_call_count"
                return 0
            else
                echo "dnf $*; times: $dnf_call_count"
                return 1
            fi
        }
    }
    BeforeAll 'setup'
    Include "./parts/linux/cloud-init/artifacts/mariner/cse_helpers_mariner.sh"
    Include "./parts/linux/cloud-init/artifacts/mariner/cse_install_mariner.sh"
    Describe 'dnf_install'
        It 'succeeds after retries'
            When call dnf_install 3 1 1 "sample-package"
            The output should include "dnf install -y sample-package"
            The output should include "Executed dnf install -y \"sample-package\" 3 times"
            The output should include "dnf_makecache"
        End
        It 'fails after retries'
            When call dnf_install 2 1 1 "sample-package"
            The output should include "dnf install -y sample-package"
            The output should include "dnf_makecache"
            The status should eq 1
        End
        It 'fails with disablerepo'
            When call dnf_install 2 1 1 "sample-package" --disablerepo='*' --enablerepo=localrepo
            The output should include "dnf install -y sample-package --disablerepo=* --enablerepo=localrepo"
            The output should not include "dnf_makecache"
            The status should eq 1
        End
    End

    Describe 'installToolFromLocalRepo'
        function rm() {
            return 0
        }
        function _dnf_makecache() {
            echo "_dnf_makecache"
            return 0
        }
        function dnf_makecache() {
            echo "unexpected dnf_makecache"
            return 0
        }
        setup_repo(){
            mkdir -p /tmp/to/repo
            mkdir -p /tmp/to/repo/repodata
            mkdir -p /etc/yum.repos.d
        }
        BeforeAll 'setup_repo'
        AfterAll 'rm -rf /tmp/to'

        It 'dnf install failed'
            When call installToolFromLocalRepo "test-tool" "/tmp/to/repo"
            The status should eq 0
            The output should not include "Failed to install test-tool from local repository"
            The output should not include "unexpected dnf_makecache"
        End
    End
End
