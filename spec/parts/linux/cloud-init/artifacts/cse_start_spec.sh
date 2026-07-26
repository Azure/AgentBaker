#!/usr/bin/env shellspec

Describe 'CSE completion markers'
    CSE_START_PATH="parts/linux/cloud-init/artifacts/cse_start.sh"

    setup() {
        TEST_DIR=$(mktemp -d)
        __SOURCED__=1
        Include "$CSE_START_PATH"
    }

    cleanup() {
        rm -rf "$TEST_DIR"
        unset __SOURCED__
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'creates the marker and its parent directory'
        marker_path="$TEST_DIR/nested/base_prep.complete"

        When call create_completion_marker "$marker_path"

        The status should be success
        The path "$marker_path" should be file
    End

    It 'returns failure when the marker cannot be created'
        touch "$TEST_DIR/not-a-directory"
        marker_path="$TEST_DIR/not-a-directory/provision.complete"

        When call create_completion_marker "$marker_path"

        The status should be failure
        The path "$marker_path" should not be exist
    End

    It 'handles provisioning failures before creating either marker'
        When run awk '
            /\[ "\$EXIT_CODE" -ne 0 \]/ { failure_check = NR }
            failure_check && !failure_exit && /exit "\$EXIT_CODE"/ { failure_exit = NR }
            /create_completion_marker "\/opt\/azure\/containers\/base_prep\.complete"/ { base_marker = NR }
            /create_completion_marker "\/opt\/azure\/containers\/provision\.complete"/ { provision_marker = NR }
            END {
                exit !(failure_check < failure_exit &&
                       failure_exit < base_marker &&
                       base_marker < provision_marker)
            }
        ' "$CSE_START_PATH"

        The status should be success
    End

    It 'fails closed when either marker cannot be created'
        When run awk '
            /if ! create_completion_marker "\/opt\/azure\/containers\/base_prep\.complete"/ { in_base = 1 }
            in_base && /exit 1/ { base_exit = 1; in_base = 0 }
            /if ! create_completion_marker "\/opt\/azure\/containers\/provision\.complete"/ { in_provision = 1 }
            in_provision && /exit 1/ { provision_exit = 1; in_provision = 0 }
            END { exit !(base_exit && provision_exit) }
        ' "$CSE_START_PATH"

        The status should be success
    End
End
