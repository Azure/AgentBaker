#!/bin/bash
# shellcheck disable=SC2089,SC2090,SC2317

Describe 'ubuntu-snapshot-update.sh generic reconciliation'
    setup() {
        Include ./parts/linux/cloud-init/artifacts/ubuntu/ubuntu-snapshot-update.sh
        TEST_DIR="/tmp/knead-component-test"
        rm -rf "${TEST_DIR}"
        mkdir -p "${TEST_DIR}"

        KUBECONFIG="${TEST_DIR}/kubeconfig"
        touch "${KUBECONFIG}"
        KUBECTL="kubectl"
        KNEAD_COMPONENT_STATE_FILE="${TEST_DIR}/state/current.json"
        KNEAD_EVENTS_LOGGING_DIR="${TEST_DIR}/events"
        TEST_COMPONENTS_JSON_FILE="${TEST_DIR}/components.json"
        TEST_KUBECTL_ARGS_FILE="${TEST_DIR}/kubectl-args"

        TEST_STATUS=""
        TEST_GOAL=""
        TEST_AGENT_POOL="ap1"
        TEST_REPO_SERVICE=""
        printf '%s' '{"components":[]}' > "${TEST_COMPONENTS_JSON_FILE}"
        TEST_SECURITY_STATUS=0
        TEST_ANNOTATE_STATUS=0
        export KUBECTL KNEAD_COMPONENT_STATE_FILE KNEAD_EVENTS_LOGGING_DIR TEST_COMPONENTS_JSON_FILE TEST_KUBECTL_ARGS_FILE
        export TEST_STATUS TEST_GOAL TEST_AGENT_POOL TEST_REPO_SERVICE TEST_SECURITY_STATUS TEST_ANNOTATE_STATUS
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Mock hostname
        echo "AKS-NODE-1"
    End

    Mock kubectl
        case "$*" in
            *"annotate --overwrite node"*)
                echo "annotate mock called with args: $*"
                if [ "${TEST_ANNOTATE_STATUS}" -ne 0 ]; then
                    exit "${TEST_ANNOTATE_STATUS}"
                fi
                ;;
            *"get node"*)
                jq -nc \
                    --arg name "aks-node-1" \
                    --arg goal "${TEST_GOAL}" \
                    --arg status "${TEST_STATUS}" \
                    --arg agentPool "${TEST_AGENT_POOL}" \
                    --arg repoService "${TEST_REPO_SERVICE}" \
                    '{metadata: {name: $name, labels: {"kubernetes.azure.com/agentpool": $agentPool}, annotations: {"kubernetes.azure.com/live-patching-config-goal-hash": $goal, "kubernetes.azure.com/live-patching-status": $status, "kubernetes.azure.com/live-patching-repo-service": $repoService}}}'
                ;;
            *"get cm"*)
                printf '%s' "$*" > "${TEST_KUBECTL_ARGS_FILE}"
                cat "${TEST_COMPONENTS_JSON_FILE}"
                ;;
        esac
    End

    updateSecurityPatch() {
        echo "updateSecurityPatch called with args: $*"
        return "${TEST_SECURITY_STATUS}"
    }

    securityPatchIsCurrent() {
        local desired_payload="$1"
        local current_payload="$2"
        local node_json="$3"
        local agent_pool
        local desired_timestamp
        local current_timestamp

        agent_pool="$(printf '%s' "${node_json}" | jq -r '.metadata.labels["kubernetes.azure.com/agentpool"]')"
        desired_timestamp="$(printf '%s' "${desired_payload}" | jq -r --arg agentPool "${agent_pool}" '.agentPools[$agentPool].goldenTimestamp')"
        current_timestamp="$(printf '%s' "${current_payload}" | jq -r --arg agentPool "${agent_pool}" '.agentPools[$agentPool].goldenTimestamp')"
        [ "${desired_timestamp}" = "${current_timestamp}" ]
    }

    set_payload_goal() {
        printf '%s' "$1" > "${TEST_COMPONENTS_JSON_FILE}"
        TEST_GOAL="$(sha256sum "${TEST_COMPONENTS_JSON_FILE}" | awk '{print $1}')"
        export TEST_GOAL
    }

    fail_result_then_write_status() {
        KNEAD_COMPONENT_RESULTS='not-json'
        KNEAD_COMPONENT_RESULTS_VALID=true
        knead_set_component_result securityPatch Succeeded || true
        echo "results valid: ${KNEAD_COMPONENT_RESULTS_VALID}"
        knead_write_status aks-node-1 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    }

    fail_component_parse_then_write_status() {
        knead_apply_components 'not-json' '{}' || true
        echo "results valid: ${KNEAD_COMPONENT_RESULTS_VALID}"
        knead_write_status aks-node-1 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    }

    read_event() {
        local task="$1"

        jq -c --arg task "${task}" 'select(.TaskName == $task)' "${KNEAD_EVENTS_LOGGING_DIR}"/*.json
    }

    read_reconcile_events() {
        read_event 'AKS.LivePatching.reconcile'
    }

    event_file_names() {
        basename -a "${KNEAD_EVENTS_LOGGING_DIR}"/*.json
    }

    emit_event_ignoring_failure() {
        knead_emit_event 'AKS.LivePatching.test' 'test message' || true
    }

    It 'writes a Guest Agent event with the requested level and message'
        knead_emit_event 'AKS.LivePatching.test' 'test message' 'Error'

        When call read_event 'AKS.LivePatching.test'
        The status should be success
        The output should include '"TaskName":"AKS.LivePatching.test"'
        The output should include '"EventLevel":"Error"'
        The output should include '"Message":"test message"'
        The output should include '"OperationId":"'
        The path "${KNEAD_EVENTS_LOGGING_DIR}" should be directory
        The result of function event_file_names should match pattern '[0-9]*.json'
    End

    It 'allows callers to ignore event writing failures'
        KNEAD_EVENTS_LOGGING_DIR="/dev/null/events"
        export KNEAD_EVENTS_LOGGING_DIR

        When call emit_event_ignoring_failure
        The status should be success
        The stderr should include 'Not a directory'
    End

    It 'records kubeconfig wait failures'
        rm -f "${KUBECONFIG}"
        KNEAD_KUBECONFIG_WAIT_TIMEOUT_SECONDS=0
        export KNEAD_KUBECONFIG_WAIT_TIMEOUT_SECONDS

        When call knead_main
        The status should be failure
        The result of function read_reconcile_events should include 'Failed: kubeconfig wait failed'
        The result of function read_reconcile_events should include '"EventLevel":"Error"'
    End

    It 'does nothing when goal annotation is not set'
        TEST_GOAL=""
        export TEST_GOAL

        When call knead_main
        The status should be success
        The output should include 'live-patching goal is not set, skip knead-component'
        The output should not include 'updateSecurityPatch called'
        The output should not include 'annotate mock called'
    End

    It 'treats an absent annotations map as an unset annotation'
        When call knead_get_node_annotation '{"metadata":{}}' "${KNEAD_COMPONENT_GOAL_ANNOTATION}"
        The status should be success
        The output should equal ''
    End

    It 'fails after the kubeconfig wait deadline'
        rm -f "${KUBECONFIG}"
        KNEAD_KUBECONFIG_WAIT_TIMEOUT_SECONDS=0
        export KNEAD_KUBECONFIG_WAIT_TIMEOUT_SECONDS

        When call knead_main
        The status should be failure
        The error should include 'timed out waiting for kubelet kubeconfig after 0s'
        The output should not include 'updateSecurityPatch called'
    End

    It 'does nothing when status is converged for the goal'
        TEST_GOAL="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        TEST_STATUS='{"currentHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","components":{"securityPatch":{"code":"Succeeded"}}}'
        export TEST_GOAL TEST_STATUS

        When call knead_main
        The status should be success
        The output should include 'live-patching goal is already converged, nothing to apply'
        The output should not include 'updateSecurityPatch called'
        The output should not include 'annotate mock called'
    End

    It 'dispatches securityPatch and writes successful status'
        set_payload_goal '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"}]}'

        When call knead_main
        The status should be success
        The output should include 'applying component: securityPatch'
        The output should include 'updateSecurityPatch called with args: {"agentPools":{"ap1":{"goldenTimestamp":"20260623T000000Z"}}}'
        The output should include '"kubernetes.azure.com/agentpool":"ap1"'
        The output should include 'annotate mock called with args: annotate --overwrite node aks-node-1 kubernetes.azure.com/live-patching-status={"currentHash":"'
        The output should include '"components":{"securityPatch":{"code":"Succeeded"}}}'
        The output should include 'knead-component completed successfully'
        The contents of file "${KNEAD_COMPONENT_STATE_FILE}" should include '"securityPatch"'
        The result of function read_reconcile_events should include 'Succeeded: goal='
    End

    It 'reads the dotted ConfigMap key with an escaped JSONPath'
        set_payload_goal '{"components":[]}'

        When call knead_read_configmap "${TEST_GOAL}"
        The status should be success
        The output should equal '{"components":[]}'
        The contents of file "${TEST_KUBECTL_ARGS_FILE}" should equal 'get cm -n kube-system live-patching-config -o jsonpath={.data.live-patching-config\.json}'
    End

    It 'fails before dispatch when the goal hash does not match the ConfigMap payload'
        printf '%s' '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{}}"}]}' > "${TEST_COMPONENTS_JSON_FILE}"
        TEST_GOAL="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
        export TEST_GOAL

        When call knead_main
        The status should be failure
        The error should include 'live-patching goal hash does not match ConfigMap payload'
        The output should not include 'updateSecurityPatch called'
        The output should not include 'annotate mock called'
    End

    It 'rejects a prefixed goal hash'
        printf '%s' '{"components":[]}' > "${TEST_COMPONENTS_JSON_FILE}"
        TEST_GOAL="sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        export TEST_GOAL

        When call knead_main
        The status should be failure
        The error should include 'live-patching goal hash must be a 64-character lowercase sha256 digest'
        The output should not include 'annotate mock called'
    End

    It 'fails before dispatch when components is not an array'
        set_payload_goal '{"components":{}}'

        When call knead_main
        The status should be failure
        The error should include 'live-patching-config payload has invalid envelope'
        The output should not include 'updateSecurityPatch called'
        The output should not include 'annotate mock called'
    End

    It 'fails before dispatch when component names are duplicated'
        set_payload_goal '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{}}"},{"name":"securityPatch","nodeConfig":"{\"agentPools\":{}}"}]}'

        When call knead_main
        The status should be failure
        The error should include 'live-patching-config payload has invalid envelope'
        The output should not include 'updateSecurityPatch called'
        The output should not include 'annotate mock called'
    End

    It 'records a failed securityPatch result and advances currentHash'
        set_payload_goal '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"}]}'
        TEST_SECURITY_STATUS=1
        export TEST_SECURITY_STATUS

        When call knead_main
        The status should be failure
        The output should include 'updateSecurityPatch called'
        The output should include 'component failed: securityPatch'
        The output should include 'failed components: securityPatch'
        The output should include 'annotate mock called with args: annotate --overwrite node aks-node-1 kubernetes.azure.com/live-patching-status={"currentHash":"'
        The output should include '"securityPatch":{"code":"Failed"}'
        The result of function read_reconcile_events should include 'Failed: goal='
    End

    It 'continues reporting later components after securityPatch fails'
        set_payload_goal '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"},{"name":"npd","nodeConfig":"{\"version\":\"v20260623.0\"}"}]}'
        TEST_SECURITY_STATUS=1
        export TEST_SECURITY_STATUS

        When call knead_main
        The status should be failure
        The output should include 'component failed: securityPatch'
        The output should include 'unsupported component: npd'
        The output should include 'failed components: securityPatch'
        The output should include '"securityPatch":{"code":"Failed"}'
        The output should not include '"npd"'
    End

    It 'retries when currentHash matches but a component previously failed'
        set_payload_goal '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"}]}'
        TEST_STATUS="{\"currentHash\":\"${TEST_GOAL}\",\"components\":{\"securityPatch\":{\"code\":\"Failed\"}}}"
        export TEST_STATUS

        When call knead_main
        The status should be success
        The output should include 'updateSecurityPatch called'
        The output should include '"securityPatch":{"code":"Succeeded"}'
    End

    It 'ignores an unsupported component without blocking securityPatch dispatch'
        set_payload_goal '{"components":[{"name":"npd","nodeConfig":"{\"version\":\"v20260623.0\"}"},{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"}]}'

        When call knead_main
        The status should be success
        The output should include 'unsupported component: npd'
        The output should include 'updateSecurityPatch called'
        The output should not include 'failed components:'
        The output should not include '"npd"'
        The output should include '"securityPatch":{"code":"Succeeded"}'
        The contents of file "${KNEAD_COMPONENT_STATE_FILE}" should include '"securityPatch"'
        The contents of file "${KNEAD_COMPONENT_STATE_FILE}" should not include '"npd"'
    End

    It 'does not rerun a successful component when an unsupported sibling is ignored'
        set_payload_goal '{"components":[{"name":"npd","nodeConfig":"{\"version\":\"v20260623.0\"}"},{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"}]}'
        knead_main > /dev/null || true

        When call knead_main
        The status should be success
        The output should include 'unsupported component: npd'
        The output should include 'component is already current: securityPatch'
        The output should not include 'updateSecurityPatch called'
        The output should not include '"npd"'
        The output should include '"securityPatch":{"code":"Succeeded"}'
    End

    It 'does not rerun an unchanged component when another component changes'
        mkdir -p "$(dirname "${KNEAD_COMPONENT_STATE_FILE}")"
        printf '%s' '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"}]}' > "${KNEAD_COMPONENT_STATE_FILE}"
        set_payload_goal '{"components":[{"name":"npd","nodeConfig":"{\"version\":\"v20260701.0\"}"},{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"}]}'

        When call knead_main
        The status should be success
        The output should include 'unsupported component: npd'
        The output should include 'component is already current: securityPatch'
        The output should not include 'updateSecurityPatch called'
        The output should not include '"npd"'
        The output should include '"securityPatch":{"code":"Succeeded"}'
    End

    It 'does not rerun securityPatch when only another agent pool changes'
        mkdir -p "$(dirname "${KNEAD_COMPONENT_STATE_FILE}")"
        printf '%s' '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"},\"ap2\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"}]}' > "${KNEAD_COMPONENT_STATE_FILE}"
        set_payload_goal '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"},\"ap2\":{\"goldenTimestamp\":\"20260701T000000Z\"}}}"}]}'

        When call knead_main
        The status should be success
        The output should include 'component is already current: securityPatch'
        The output should not include 'updateSecurityPatch called'
        The output should include '"securityPatch":{"code":"Succeeded"}'
    End

    It 'preserves sibling state when recording a successful component'
        mkdir -p "$(dirname "${KNEAD_COMPONENT_STATE_FILE}")"
        printf '%s' '{"components":[{"name":"npd","nodeConfig":"{\"version\":\"v20260623.0\"}"}]}' > "${KNEAD_COMPONENT_STATE_FILE}"

        When call knead_write_component_state securityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"20260623T000000Z"}}}'
        The status should be success
        The contents of file "${KNEAD_COMPONENT_STATE_FILE}" should include '"npd"'
        The contents of file "${KNEAD_COMPONENT_STATE_FILE}" should include 'v20260623.0'
        The contents of file "${KNEAD_COMPONENT_STATE_FILE}" should include '"securityPatch"'
        The contents of file "${KNEAD_COMPONENT_STATE_FILE}" should include '20260623T000000Z'
    End

    It 'fails after handler success when status annotation update fails'
        set_payload_goal '{"components":[{"name":"securityPatch","nodeConfig":"{\"agentPools\":{\"ap1\":{\"goldenTimestamp\":\"20260623T000000Z\"}}}"}]}'
        TEST_ANNOTATE_STATUS=1
        export TEST_ANNOTATE_STATUS

        When call knead_main
        The status should be failure
        The output should include 'updateSecurityPatch called'
        The output should include 'failed to update live-patching status annotation'
        The output should not include 'knead-component completed successfully'
        The contents of file "${KNEAD_COMPONENT_STATE_FILE}" should include '"securityPatch"'
    End

    It 'fails before annotation when component results cannot be rendered'
        KNEAD_COMPONENT_RESULTS='not-json'
        export KNEAD_COMPONENT_RESULTS

        When call knead_write_status aks-node-1 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
        The status should be failure
        The output should include 'failed to render live-patching status annotation'
        The stderr should include 'parse error'
        The output should not include 'annotate mock called'
    End

    It 'fails before annotation when status rendering produces no output'
        KNEAD_COMPONENT_RESULTS=''
        export KNEAD_COMPONENT_RESULTS

        When call knead_write_status aks-node-1 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
        The status should be failure
        The output should include 'failed to render live-patching status annotation'
        The output should not include 'annotate mock called'
    End

    It 'preserves existing results and refuses status after a result update fails'
        When call fail_result_then_write_status
        The status should be failure
        The output should include 'failed to record component result: securityPatch=Succeeded'
        The output should include 'results valid: false'
        The output should include 'refusing to write incomplete live-patching status'
        The stderr should include 'parse error'
        The output should not include 'annotate mock called'
    End

    It 'refuses status after component parsing fails'
        When call fail_component_parse_then_write_status
        The status should be failure
        The output should include 'failed to read component count'
        The output should include 'results valid: false'
        The output should include 'refusing to write incomplete live-patching status'
        The stderr should include 'parse error'
        The output should not include 'annotate mock called'
    End
End
