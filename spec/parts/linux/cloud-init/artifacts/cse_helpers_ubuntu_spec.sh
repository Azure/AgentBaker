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

    Describe '_apt_get_update error detection'
        # Isolate the apt-get update retry/error-grep logic. sleep is stubbed so retries are fast.
        wait_for_apt_locks() { :; }
        dpkg() { :; }
        sleep() { :; }

        It "succeeds when apt-get update prints only the benign Ubuntu Pro/ESM hook line"
            # The ubuntu-pro-client apt hook (esm-cache/apt-news) writes this to STDERR during early
            # boot when the local systemd/D-Bus handshake isn't ready. The real code merges it via
            # 2>&1, and the apt operation itself still succeeds (Hit: ... InRelease), so it must NOT be
            # treated as a failure. Regression test for the exit-99 false positive: apt-get update
            # succeeded yet CSE exited 99 because the error grep matched "Error ...".
            apt-get() {
                case "$*" in
                    *update*)
                        echo "Hit:1 https://packages.microsoft.com/ubuntu/24.04/prod noble InRelease"
                        echo "Error connecting: Error sending credentials: Error sending message: Broken pipe" >&2
                        echo "Reading package lists..."
                        ;;
                esac
                return 0
            }
            When call _apt_get_update 3 ""
            The status should eq 0
            The stdout should include "Executed apt-get update 1 times"
        End

        It "succeeds on a clean apt-get update"
            apt-get() {
                case "$*" in
                    *update*)
                        echo "Hit:1 https://packages.microsoft.com/ubuntu/24.04/prod noble InRelease"
                        echo "Reading package lists..."
                        ;;
                esac
                return 0
            }
            When call _apt_get_update 3 ""
            The status should eq 0
            The stdout should include "Executed apt-get update 1 times"
        End

        It "still FAILS when PMC/Canonical is genuinely unreachable"
            # Guards against masking real remote-repo failures: an unreachable mirror surfaces as apt's
            # own E:/Err:/Failed-to-fetch lines, which do NOT match the benign filter and must still fail.
            apt-get() {
                case "$*" in
                    *update*)
                        echo "Err:1 https://packages.microsoft.com/ubuntu/24.04/prod noble InRelease" >&2
                        echo "  Could not connect to packages.microsoft.com:443 (13.107.9.104), connection timed out" >&2
                        echo "E: Failed to fetch https://packages.microsoft.com/ubuntu/24.04/prod/dists/noble/InRelease  Could not connect" >&2
                        echo "E: Some index files failed to download." >&2
                        ;;
                esac
                return 0
            }
            When call _apt_get_update 2 ""
            The status should eq 1
            The stdout should not include "Executed apt-get update"
        End
    End

    Describe 'apt_get_dist_upgrade error detection (VHD build)'
        # apt_get_dist_upgrade is VHD-build only; retries=10 is hardcoded, so stub sleep for speed.
        wait_for_apt_locks() { :; }
        dpkg() { :; }
        sleep() { :; }

        It "ignores the benign ESM/D-Bus line on dist-upgrade"
            apt-get() {
                case "$*" in
                    *dist-upgrade*)
                        echo "Calculating upgrade..."
                        echo "Error connecting: Error sending credentials: Error sending message: Broken pipe" >&2
                        echo "0 upgraded, 0 newly installed, 0 to remove and 0 not upgraded."
                        ;;
                esac
                return 0
            }
            When call apt_get_dist_upgrade
            The status should eq 0
            The stdout should include "Executed apt-get dist-upgrade 1 times"
        End

        It "still fails on a real apt error during dist-upgrade"
            apt-get() {
                case "$*" in
                    *dist-upgrade*)
                        echo "E: Failed to fetch https://packages.microsoft.com/... Could not connect to packages.microsoft.com:443" >&2
                        echo "E: Some index files failed to download." >&2
                        ;;
                esac
                return 0
            }
            When call apt_get_dist_upgrade
            The status should eq 1
            The stdout should not include "Executed apt-get dist-upgrade"
        End
    End
End
