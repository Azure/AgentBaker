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
            The stderr should include "Warning: CSE_STARTTIME_SECONDS environment variable is not set."
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
            The stderr should include "Warning: CSE_STARTTIME_SECONDS environment variable is not set."
        End
    End

    Describe '_apt_get_update CSE budget and per-attempt timeout'
        wait_for_apt_locks() { :; }
        dpkg() { :; }
        apt-get() { return 0; }
        # Report the per-attempt timeout value, then run the command. _apt_get_update redirects both
        # streams of the timeout invocation into its output file and cats it, so this lands on stdout.
        timeout() { echo "timeout arg: $1"; shift; "$@"; }

        It "caps each attempt at the default per-attempt timeout when CSE budget is plentiful"
            CSE_STARTTIME_SECONDS=$(date +%s)
            When call _apt_get_update 1 ""
            The status should eq 0
            The stdout should include "timeout arg: 180"
            The stdout should include "Executed apt-get update 1 times"
        End

        It "caps each attempt at the remaining CSE budget when it is smaller"
            CSE_MAX_DURATION_SECONDS=1000
            CSE_STARTTIME_SECONDS=$(( $(date +%s) - 900 ))
            # Allow a few seconds of drift between the setup above and the call.
            timeout() {
                if [ "$1" -le 100 ] && [ "$1" -ge 95 ]; then
                    echo "capped to remaining CSE budget"
                else
                    echo "not capped: $1"
                fi
                shift
                "$@"
            }
            When call _apt_get_update 1 ""
            The status should eq 0
            The stdout should include "capped to remaining CSE budget"
        End

        It "returns 2 when the CSE timeout is already exceeded before the first attempt"
            CSE_STARTTIME_SECONDS=$(( $(date +%s) - 800 ))
            When call _apt_get_update 3 ""
            The status should eq 2
            The stderr should include "CSE timeout approaching"
        End

        It "returns 2 when no CSE time remains"
            # Stub the guard so this test isolates the remaining-budget check, which would otherwise
            # only be reachable in the single second where elapsed time exactly equals the max.
            check_cse_timeout() { return 0; }
            CSE_MAX_DURATION_SECONDS=100
            CSE_STARTTIME_SECONDS=$(( $(date +%s) - 200 ))
            When call _apt_get_update 3 ""
            The status should eq 2
            The stderr should include "No CSE time remaining"
        End

        It "treats a timed-out apt-get as failure even when it emits no diagnostics"
            timeout() { return 124; }
            When call _apt_get_update 1 ""
            The status should eq 1
            The stderr should include "apt-get update timed out after"
        End

        It "still applies the per-attempt timeout during VHD build without CSE warnings"
            unset CSE_STARTTIME_SECONDS
            When call _apt_get_update 1 ""
            The status should eq 0
            The stdout should include "timeout arg: 180"
            The stderr should not include "CSE timeout approaching"
            The stdout should include "Executed apt-get update 1 times"
        End
    End

    Describe 'apt_get_dist_upgrade phased updates'
        wait_for_apt_locks() { :; }
        dpkg() { :; }

        It "passes APT::Get::Always-Include-Phased-Updates=true to dist-upgrade"
            apt-get() {
                # Only echo args for the dist-upgrade invocation so the assertion is unambiguous.
                for arg in "$@"; do
                    if [ "$arg" = "dist-upgrade" ]; then
                        echo "$@"
                        return 0
                    fi
                done
                return 0
            }
            When call apt_get_dist_upgrade
            The status should eq 0
            The stdout should include "APT::Get::Always-Include-Phased-Updates=true"
            The stdout should include "dist-upgrade"
        End
    End
End
