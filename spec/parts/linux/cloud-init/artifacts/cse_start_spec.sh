#!/usr/bin/env shellspec

provision_json_mode() {
    stat -c '%a' "${PROVISION_JSON_FILE_PATH}" 2>/dev/null || stat -f '%Lp' "${PROVISION_JSON_FILE_PATH}"
}

provision_json_temp_count() {
    find "$(dirname "${PROVISION_JSON_FILE_PATH}")" -maxdepth 1 -name 'provision.json.tmp' | wc -l | tr -d ' '
}

Describe 'publishProvisionResponse'
    load_publish_provision_response() {
        eval "$(sed -n '/^publishProvisionResponse()/,/^}/p' parts/linux/cloud-init/artifacts/cse_start.sh)"
    }

    setup() {
        TEST_DIR="$(mktemp -d)"
        PROVISION_JSON_FILE_PATH="${TEST_DIR}/logs/provision.json"
        PROVISION_COMPLETE_FILE_PATH="${TEST_DIR}/state/provision.complete"
        BASE_PREP_COMPLETE_FILE_PATH="${TEST_DIR}/state/base_prep.complete"
        PRE_PROVISION_ONLY="false"
        load_publish_provision_response
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'atomically replaces the response before creating the completion marker'
        mkdir -p "$(dirname "${PROVISION_JSON_FILE_PATH}")"
        printf '%s\n' 'old response' > "${PROVISION_JSON_FILE_PATH}"

        When call publishProvisionResponse '{"ExitCode":"0","Output":"complete","Error":""}'

        The status should be success
        The output should equal '{"ExitCode":"0","Output":"complete","Error":""}'
        The contents of file "${PROVISION_JSON_FILE_PATH}" should equal '{"ExitCode":"0","Output":"complete","Error":""}'
        The path "${PROVISION_COMPLETE_FILE_PATH}" should be file
        The path "${BASE_PREP_COMPLETE_FILE_PATH}" should not be exist
        The result of function provision_json_mode should equal "644"
        The result of function provision_json_temp_count should equal "0"
    End

    It 'uses the base preparation marker for pre-provisioning'
        PRE_PROVISION_ONLY="true"

        When call publishProvisionResponse '{"ExitCode":"0"}'

        The status should be success
        The output should equal '{"ExitCode":"0"}'
        The path "${BASE_PREP_COMPLETE_FILE_PATH}" should be file
        The path "${PROVISION_COMPLETE_FILE_PATH}" should not be exist
    End

    It 'preserves the previous response and withholds completion when rename fails'
        mkdir -p "$(dirname "${PROVISION_JSON_FILE_PATH}")"
        printf '%s\n' 'old response' > "${PROVISION_JSON_FILE_PATH}"
        mv() {
            return 1
        }

        When call publishProvisionResponse '{"ExitCode":"0"}'

        The status should be failure
        The stdout should equal '{"ExitCode":"0"}'
        The contents of file "${PROVISION_JSON_FILE_PATH}" should equal "old response"
        The path "${PROVISION_COMPLETE_FILE_PATH}" should not be exist
        The result of function provision_json_temp_count should equal "0"
    End

    It 'keeps the response on stdout when staging cannot start'
        blocker="${TEST_DIR}/blocker"
        printf '%s\n' 'not a directory' > "${blocker}"
        PROVISION_JSON_FILE_PATH="${blocker}/provision.json"

        When call publishProvisionResponse '{"ExitCode":"0"}'
        The status should be failure
        The stdout should equal '{"ExitCode":"0"}'
        The stderr should include "mkdir:"
        The path "${PROVISION_COMPLETE_FILE_PATH}" should not be exist
    End
End
