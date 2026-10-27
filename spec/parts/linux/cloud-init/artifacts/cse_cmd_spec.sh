#!/usr/bin/env shellspec

Describe 'CSE CustomData checksum verification'
    setup() {
        TEST_DIR="${SHELLSPEC_TMPBASE}/cse-checksums"
        mkdir -p "${TEST_DIR}"
        printf 'first\n' > "${TEST_DIR}/first"
        printf 'second\n' > "${TEST_DIR}/second"
        CHECKSUMS="'$(cksum "${TEST_DIR}/first")' '$(cksum "${TEST_DIR}/second")'"
        PROVISION_OUTPUT="${TEST_DIR}/provision.log"
        export PROVISION_OUTPUT
        : > "${PROVISION_OUTPUT}"
    }

    run_check() {
        {
            printf 'for checksum in %s; do\n' "${CHECKSUMS}"
            sed -n '/^for checksum in /,/^done;/p' parts/linux/cloud-init/artifacts/cse_cmd.sh | sed '1d'
            printf 'echo continued\n'
        } > "${TEST_DIR}/check.sh"
        sh "${TEST_DIR}/check.sh"
    }

    BeforeEach 'setup'

    It 'continues when every file matches'
        When run run_check
        The status should be success
        The output should eq 'continued'
        The contents of file "${PROVISION_OUTPUT}" should eq ''
    End

    It 'reports the changed file and stops even when its size is unchanged'
        printf 'seconx\n' > "${TEST_DIR}/second"
        When run run_check
        The status should eq 1
        The output should include "CustomData checksum verification failed: ${TEST_DIR}/second"
        The output should not include 'continued'
        The contents of file "${PROVISION_OUTPUT}" should include "${TEST_DIR}/second"
    End

    It 'reports a missing file and stops'
        mv "${TEST_DIR}/first" "${TEST_DIR}/missing"
        When run run_check
        The status should eq 1
        The output should include "CustomData checksum verification failed: ${TEST_DIR}/first"
        The output should not include 'continued'
        The contents of file "${PROVISION_OUTPUT}" should include "${TEST_DIR}/first"
    End
End
