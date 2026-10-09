#!/bin/bash
# shellcheck disable=SC2317

Describe 'Azure Linux reconciliation failure isolation and events'
    setup() {
        Include ./parts/linux/cloud-init/artifacts/mariner/mariner-package-update.sh
        Include ./parts/linux/cloud-init/artifacts/mariner/security-update.sh
        TEST_DIR=$(mktemp -d)
        LIVE_PATCHING_STATE_FILE="${TEST_DIR}/current.json"
        KNEAD_EVENTS_LOGGING_DIR="${TEST_DIR}/events"
        export KUBECTL=kubectl
        TEST_NODE='{"metadata":{"name":"test-node","labels":{"kubernetes.azure.com/agentpool":"ap1"}}}'
    }
    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    Mock kubectl
        echo "kubectl called: $*"
    End

    read_events() {
        jq -c '.' "${KNEAD_EVENTS_LOGGING_DIR}"/*.json
    }

    run_internal_failure() {
        local result=0
        apply_components '{"components":[{"name":"securityPatch","nodeConfig":"first"},{"name":"securityPatch","nodeConfig":"second"}]}' "${TEST_NODE}" || result=1
        write_generic_status test-node goal || result=1
        return "${result}"
    }

    It 'continues later work after result recording fails and refuses incomplete success'
        updateSecurityPatch() { echo "handler ran: $1"; }
        securityPatchIsCurrent() { return 1; }
        Mock jq
            # shellcheck disable=SC2016
            case "$*" in
                *'.[$component]={code:$code}'*) exit 1 ;;
                *) /usr/bin/jq "$@" ;;
            esac
        End

        When call run_internal_failure
        The status should be failure
        The output should include 'handler ran: first'
        The output should include 'handler ran: second'
        The output should include 'failed to record component result:'
        The output should include 'refusing to write incomplete live-patching status'
        The output should not include 'kubectl called'
        The contents of file "${LIVE_PATCHING_STATE_FILE}" should include 'second'
    End

    It 'continues after a missing component name without claiming convergence'
        updateSecurityPatch() { echo 'handler ran'; }

        When call apply_components '{"components":[{"name":null,"nodeConfig":"{}"},{"name":"securityPatch","nodeConfig":"{}"}]}' "${TEST_NODE}"
        The status should be failure
        The output should include 'failed to read component name at index: 0'
        The output should include 'handler ran'
        The variable LIVE_PATCHING_COMPONENT_RESULTS_VALID should equal false
    End

    It 'emits reconciliation failure when result recording prevents status publication'
        read_generic_config() { printf '%s' '{"components":[{"name":"securityPatch","nodeConfig":"{}"}]}'; }
        Mock jq
            # shellcheck disable=SC2016
            case "$*" in
                *'.[$component]={code:$code}'*) exit 1 ;;
                *) /usr/bin/jq "$@" ;;
            esac
        End

        When call generic_main "${TEST_NODE}" aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
        The status should be failure
        The output should include 'refusing to write incomplete live-patching status'
        The output should not include 'kubectl called'
        The result of function read_events should include '"TaskName":"AKS.LivePatching.reconcile"'
        The result of function read_events should include 'Failed: goal='
    End

    It 'distinguishes checkpoint failure from handler failure'
        read_generic_config() { printf '%s' '{"components":[{"name":"securityPatch","nodeConfig":"{}"}]}'; }
        updateSecurityPatch() { echo 'handler succeeded'; }
        write_component_checkpoint() { return 1; }

        When call generic_main "${TEST_NODE}" aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
        The status should be failure
        The output should include 'handler succeeded'
        The output should include 'failed to persist component state: securityPatch'
        The output should not include 'component failed: securityPatch'
        The output should include '"securityPatch":{"code":"Failed"}'
        The result of function read_events should include 'checkpoint write failed: securityPatch'
    End

    It 'emits no-action with the standard Guest Agent schema'
        When call updateSecurityPatch '{}' "${TEST_NODE}"
        The status should be success
        The output should include 'no action needed'
        The result of function read_events should include '"TaskName":"AKS.LivePatching.securityPatch.NoAction"'
        The result of function read_events should include '"Version":"1.23"'
        The result of function read_events should include '"EventLevel":"Informational"'
        The result of function read_events should include '"OperationId":'
        The result of function read_events should include '"EventPid":"0"'
        The result of function read_events should include '"EventTid":"0"'
        The result of function read_events should include '"Timestamp":'
    End

    It 'emits a named timestamp validation failure'
        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"invalid"}}}' "${TEST_NODE}"
        The status should be failure
        The output should include 'goldenTimestamp is invalid'
        The result of function read_events should include 'reason=GoldenTimestampInvalid'
        The result of function read_events should include '"EventLevel":"Error"'
    End

    It 'does not turn an event write failure into a component failure'
        KNEAD_EVENTS_LOGGING_DIR=/dev/null/events
        When call updateSecurityPatch '{}' "${TEST_NODE}"
        The status should be success
        The output should include 'no action needed'
        The stderr should include 'Not a directory'
    End

    It 'replaces one checkpoint entry while preserving siblings and recording time'
        printf '%s' '{"components":[{"name":"securityPatch","nodeConfig":"old"},{"name":"other","nodeConfig":"keep"}]}' > "${LIVE_PATCHING_STATE_FILE}"
        write_component_checkpoint securityPatch new

        When run jq -e '.components == [{name:"other",nodeConfig:"keep"},{name:"securityPatch",nodeConfig:"new"}] and (.updatedAt | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"))' "${LIVE_PATCHING_STATE_FILE}"
        The status should be success
        The output should equal true
    End

    It 'rebuilds a pre-release object checkpoint using the shared array format'
        printf '%s' '{"components":{"securityPatch":{"nodeConfig":"{}"}}}' > "${LIVE_PATCHING_STATE_FILE}"
        write_component_checkpoint securityPatch '{}'

        When run jq -e '.components == [{name:"securityPatch",nodeConfig:"{}"}] and (.updatedAt | type == "string")' "${LIVE_PATCHING_STATE_FILE}"
        The status should be success
        The output should equal true
    End

    It 'does not turn a reconciliation event write failure into reconciliation failure'
        KNEAD_EVENTS_LOGGING_DIR=/dev/null/events
        read_generic_config() { printf '%s' '{"components":[]}'; }

        When call generic_main "${TEST_NODE}" aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
        The status should be success
        The output should include 'generic live-patching completed successfully'
        The stderr should include 'Not a directory'
    End
End
