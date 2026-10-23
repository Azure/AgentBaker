#!/bin/bash

Describe 'Azure Linux networkd recovery'
    setup() {
        Include ./parts/linux/cloud-init/artifacts/mariner/aks-networkd-recovery.sh
        TEST_DIR="${SHELLSPEC_TMPBASE}/networkd-recovery"
        mkdir -p "$TEST_DIR"
        TEST_STATE='s "failed"'
        TEST_IFINDEX=2
        TEST_QUERY_STATUS=0
        TEST_RECOVERY_STATUS=0
    }
    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    cat() {
        if [ "$1" = /sys/class/net/eth0/ifindex ]; then
            [ -n "$TEST_IFINDEX" ] || return 1
            printf '%s\n' "$TEST_IFINDEX"
        else
            command cat "$@"
        fi
    }
    busctl() {
        printf '%s\n' "$*" > "$TEST_DIR/query"
        [ "$TEST_QUERY_STATUS" -eq 0 ] || return "$TEST_QUERY_STATUS"
        printf '%s\n' "$TEST_STATE"
    }
    timeout() {
        printf '%s\n' "$*" > "$TEST_DIR/reconfigure"
        return "$TEST_RECOVERY_STATUS"
    }

    Describe 'non-failed states'
        Parameters
            pending
            initialized
            configuring
            configured
            unmanaged
            linger
        End
        It "leaves a $1 interface alone"
            TEST_STATE="s \"$1\""
            When call recover_networkd_link
            The status should be success
            The output should be blank
            The path "$TEST_DIR/reconfigure" should not be exist
        End
    End

    It 'immediately reconfigures only the failed interface'
        When call recover_networkd_link
        The status should be success
        The output should include 'eth0 is failed'
        The output should include 'connectivity recovery is not yet confirmed'
        The contents of file "$TEST_DIR/reconfigure" should equal '20s networkctl reconfigure 2'
        The contents of file "$TEST_DIR/query" should equal '--system --auto-start=no --timeout=5s get-property org.freedesktop.network1 /org/freedesktop/network1/link/_2 org.freedesktop.network1.Link AdministrativeState'
    End

    It 'reports a missing eth0 without attempting recovery'
        TEST_IFINDEX=''
        When call recover_networkd_link
        The status should be success
        The stderr should include 'Cannot find eth0'
        The path "$TEST_DIR/reconfigure" should not be exist
    End

    It 'does not treat a D-Bus error as a failed interface'
        TEST_QUERY_STATUS=1
        When call recover_networkd_link
        The status should be success
        The stderr should include 'Cannot query eth0 administrative state'
        The path "$TEST_DIR/reconfigure" should not be exist
    End

    It 'rejects unexpected administrative state output'
        TEST_STATE='unexpected'
        When call recover_networkd_link
        The status should be success
        The stderr should include 'Unexpected eth0 administrative state'
        The path "$TEST_DIR/reconfigure" should not be exist
    End

    Describe 'unsuccessful recovery requests'
        Parameters
            1
            124
        End
        It "reports a failed or timed-out request (exit $1)"
            TEST_RECOVERY_STATUS="$1"
            When call recover_networkd_link
            The status should be success
            The output should include 'requesting networkd reconfiguration'
            The stderr should include 'failed or timed out'
            The output should not include 'eth0 reconfiguration requested'
        End
    End
End
