#!/bin/bash
Describe 'cse_start.sh completion markers'
    setup_completion_test() {
        TEST_DIR="$(mktemp -d)"
        BASE_PREP_COMPLETE_FILE="${TEST_DIR}/base_prep.complete"
        PROVISION_COMPLETE_FILE="${TEST_DIR}/provision.complete"
        PROVISION_LOG_FILE="${TEST_DIR}/cluster-provision.log"
        eval "$(sed -n '/^finalizeProvisioning()/,/^}/p' ./parts/linux/cloud-init/artifacts/cse_start.sh)"
    }
    cleanup_completion_test() { rm -rf "${TEST_DIR}"; }
    BeforeEach 'setup_completion_test'
    AfterEach 'cleanup_completion_test'
    touch() { [ "${FAIL_MARKER:-false}" != "true" ] && command touch "$@"; }
    completion_result() {
        finalizeProvisioning
        local rc=$?
        printf 'rc=%s base=%s provision=%s\n' \
            "${rc}" "$([ -f "${BASE_PREP_COMPLETE_FILE}" ] && echo true || echo false)" \
            "$([ -f "${PROVISION_COMPLETE_FILE}" ] && echo true || echo false)"
    }
    Parameters
        "true"  0  "false" "rc=0 base=true provision=false"
        "true"  84 "false" "rc=84 base=false provision=false"
        "false" 0  "false" "rc=0 base=false provision=true"
        "false" 84 "false" "rc=84 base=false provision=true"
        "true"  0  "true"  "rc=1 base=false provision=false"
        "false" 0  "true"  "rc=0 base=false provision=false"
    End
    Example "PreProvisionOnly=$1 exit=$2 marker-failure=$3"
        PRE_PROVISION_ONLY="$1"
        EXIT_CODE="$2"
        FAIL_MARKER="$3"
        When call completion_result
        The output should equal "$4"
    End

    It 'finalizes markers before status and event payloads'
        finalization_order() {
            grep -n -E '^finalizeProvisioning$|^JSON_STRING=|^message_string=' \
                ./parts/linux/cloud-init/artifacts/cse_start.sh
        }
        When call finalization_order
        The line 1 of output should include "finalizeProvisioning"
        The line 2 of output should include "JSON_STRING="
        The line 3 of output should include "message_string="
    End
End
