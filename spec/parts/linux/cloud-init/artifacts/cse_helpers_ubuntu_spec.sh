#!/bin/bash

Describe 'apt_get_install budget timeout'
    apt_install_precheck() {
        CSE_STARTTIME_SECONDS=$(date +%s)
    }
    BeforeEach apt_install_precheck

    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh"

    Describe '_apt_get_install with budget'
        # Mock apt-related commands and timeout to isolate budget logic
        wait_for_apt_locks() { :; }
        dpkg() { :; }
        apt_get_update() { :; }
        # Pass-through timeout mock: skip the timeout value, run the rest
        timeout() { shift; "$@"; }
        apt-get() {
            # Simulate install failure by default so retries are exercised
            if [ "$1" = "install" ]; then
                return 1
            fi
            # apt-get clean, etc.
            return 0
        }

        It "returns 0 when install succeeds within budget"
            apt-get() {
                if [ "$1" = "install" ]; then
                    return 0
                fi
                return 0
            }
            When call _apt_get_install 3 1 "-y" 60 fake-package
            The status should eq 0
            The stdout should include 'Executed apt-get install "fake-package"'
        End

        It "logs all package names when installing multiple packages"
            apt-get() {
                if [ "$1" = "install" ]; then
                    return 0
                fi
                return 0
            }
            When call _apt_get_install 1 0 "-y" 0 pkg-one pkg-two pkg-three
            The status should eq 0
            The stdout should include 'Executed apt-get install "pkg-one pkg-two pkg-three"'
        End

        It "returns 1 when install fails and retries exhausted (no budget)"
            When call _apt_get_install 2 0 "-y" 0 fake-package
            The status should eq 1
        End

        It "returns 2 when per-operation budget is exceeded"
            # Mock timeout to sleep so elapsed time exceeds the 1s budget
            timeout() {
                sleep 2
                return 1
            }
            When call _apt_get_install 5 0 "-y" 1 fake-package
            The status should eq 2
            The stderr should include "apt_get_install budget of 1s exceeded"
        End

        It "returns 2 when CSE timeout is already exceeded before first attempt"
            CSE_STARTTIME_SECONDS=$(( $(date +%s) - 800 ))
            When call _apt_get_install 3 1 "-y" 600 fake-package
            The status should eq 2
            The stderr should include "CSE timeout approaching"
        End

        It "does not apply budget when CSE_STARTTIME_SECONDS is unset"
            unset CSE_STARTTIME_SECONDS
            apt-get() {
                if [ "$1" = "install" ]; then
                    return 0
                fi
                return 0
            }
            # maxBudget=1 but since CSE_STARTTIME_SECONDS is unset, budget is ignored by apt_get_install wrapper
            # Here we test _apt_get_install directly with budget=0 (what the wrapper passes when unset)
            When call _apt_get_install 1 0 "-y" 0 fake-package
            The status should eq 0
            The stdout should include 'Executed apt-get install "fake-package"'
        End
    End

    Describe 'apt_get_install wrapper'
        wait_for_apt_locks() { :; }
        dpkg() { :; }
        apt_get_update() { :; }
        timeout() { shift; "$@"; }
        apt-get() {
            if [ "$1" = "install" ]; then
                return 0
            fi
            return 0
        }

        It "passes timeout as budget during CSE run"
            CSE_STARTTIME_SECONDS=$(date +%s)
            When call apt_get_install 1 0 60 fake-package
            The status should eq 0
            The stdout should include 'Executed apt-get install "fake-package"'
        End

        It "does not apply budget during VHD build (CSE_STARTTIME_SECONDS unset)"
            unset CSE_STARTTIME_SECONDS
            # Override timeout mock to fail if called — proves budget was not applied
            timeout() {
                echo "ERROR: timeout should not be called during VHD build" >&2
                return 1
            }
            When call apt_get_install 1 0 60 fake-package
            The status should eq 0
            The stdout should include 'Executed apt-get install "fake-package"'
        End
    End
End
