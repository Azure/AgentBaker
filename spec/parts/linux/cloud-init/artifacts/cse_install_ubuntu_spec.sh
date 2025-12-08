#!/bin/bash

Describe 'cse_install_ubuntu.sh'
    setup() {
        function systemctl() {
            return 0
        }
    }
    BeforeAll 'setup'
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"

    Describe 'installToolFromLocalRepo'
        function rm() {
            return 0
        }
        function apt_get_install_from_local_repo() {
            return 0
        }
        setup_repo(){
            mkdir -p /tmp/to/repo
            touch /tmp/to/repo/Packages.gz
            mkdir -p /etc/apt/sources.list.d
        }
        BeforeAll 'setup_repo'
        AfterAll 'rm -rf /tmp/to'

        It 'apt install succeeded'
            When call installToolFromLocalRepo "test-tool" "/tmp/to/repo"
            The output should include "Successfully installed test-tool from local repository"
        End
        It 'apt install failed'
            function apt_get_install_from_local_repo() {
                return 1
            }
            When call installToolFromLocalRepo "test-tool" "/tmp/to/repo"
            The status should eq 1
            The output should include "Failed to install test-tool from local repository"
        End
    End
End

Describe 'cse_helpers_ubuntu.sh'
    setup() {
        # Mock the functions that are not needed to actually run for this test
        call_apt_get_install_count=0
        function apt-get() {
            if [[ "$1" == "install" ]]; then
                call_apt_get_install_count=$((call_apt_get_install_count + 1))
                echo "apt-get $*; times: $call_apt_get_install_count"
                if [ "$call_apt_get_install_count" -eq 3 ]; then
                    return 0
                else
                    return 1
                fi
            else
                return 0
            fi
        }
        function dpkg() {
            return 0
        }
        function wait_for_apt_locks() {
            return 0
        }
        function timeout() {
            shift 1
            "$@"
            return $?
        }
    }
    BeforeAll 'setup'
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh"
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"
    Describe '_apt_get_install'
        It 'succeeds after retries'
            When call _apt_get_install 3 1 "-y" "sample-package"
            The output should include "Executed apt-get install \"\" 3 times"
        End
        It 'fails after retries'
            When call _apt_get_install 2 1 "-y" "sample-package"
            The status should eq 1
            The output should not include "Executed apt-get install \"\" 3 times"
            The output should include "; times: 2"
        End
        It 'succeeds after retries but should not call apt-get update'
            function apt_get_update() {
                echo "unexpected apt_get_update call"
                return 1
            }
            When call _apt_get_install 3 1 "-y" "-o Dir::Etc::sourcelist=/tmp/custom-sources.list" "sample-package"
            The output should include "Executed apt-get install \"\" 3 times"
            The output should not include "unexpected apt_get_update call"
        End
    End

    Describe 'installToolFromLocalRepo'
        setup_repo(){
            mkdir -p /tmp/to/repo
            touch /tmp/to/repo/Packages.gz
            mkdir -p /etc/apt/sources.list.d

            function apt_get_update() {
                echo "unexpected apt_get_update call"
                return 1
            }
        }
        BeforeAll 'setup_repo'
        AfterAll 'rm -rf /tmp/to'

        It 'dnf install failed'
            When call installToolFromLocalRepo "test-tool" "/tmp/to/repo"
            The status should eq 0
            The output should not include "Failed to install test-tool from local repository"
            The output should not include "unexpected apt_get_update call"
        End
    End
End
