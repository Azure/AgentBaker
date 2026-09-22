#!/bin/bash
# shellcheck disable=SC2317

Describe 'security-update.sh'
    security_patch_test_node_json() {
        local repo_service="${1:-}"

        jq -nc --arg repoService "${repo_service}" \
            '{metadata: {name: "aks-node-1", labels: {"kubernetes.azure.com/agentpool": "ap1"}, annotations: {"kubernetes.azure.com/live-patching-repo-service": $repoService}}}'
    }

    setup() {
        Include ./parts/linux/cloud-init/artifacts/ubuntu/security-update.sh
        TEST_DIR="/tmp/security-update-test"
        rm -rf "${TEST_DIR}"
        mkdir -p "${TEST_DIR}"

        SECURITY_PATCH_CONFIG_DIR="${TEST_DIR}/security-patch"
        TEST_NODE_JSON="$(security_patch_test_node_json)"
        TEST_APT_UPDATE_STATUS=0
        TEST_ANNOTATE_STATUS=0
        TEST_EVENT_LOG="${TEST_DIR}/events"
        KUBECTL="kubectl"
        export SECURITY_PATCH_CONFIG_DIR TEST_NODE_JSON TEST_EVENT_LOG KUBECTL
        export TEST_APT_UPDATE_STATUS TEST_ANNOTATE_STATUS
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Mock lsb_release
        echo "jammy"
    End

    Mock unattended-upgrade
        echo "unattended-upgrade called"
    End

    Mock sleep
        echo "sleep called"
    End

    Mock kubectl
        echo "kubectl called with args: $*"
        exit "${TEST_ANNOTATE_STATUS}"
    End

    apt_get_update_with_opts() {
        echo "apt_get_update_with_opts called with args: $*"
        return "${TEST_APT_UPDATE_STATUS}"
    }

    knead_emit_event() {
        mkdir -p "${TEST_EVENT_LOG}"
        printf '%s|%s|%s\n' "$1" "$2" "${3:-Informational}" >> "${TEST_EVENT_LOG}/events"
    }

    It 'applies the timestamp selected for the node agent pool'
        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"},"ap2":{"goldenTimestamp":"20260701T000000Z"}}}' "${TEST_NODE_JSON}"
        The status should be success
        The output should include 'apt_get_update_with_opts called with args: -o Acquire::http::Timeout=300 -o Acquire::https::Timeout=300 -o Acquire::Retries=3'
        The output should include 'unattended-upgrade called'
        The output should include 'kubectl called with args: annotate --overwrite node aks-node-1 kubernetes.azure.com/live-patching-current-timestamp=20260710T000000Z'
        The output should include 'securityPatch update completed successfully: 20260710T000000Z'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.Applied|goldenTimestamp=20260710T000000Z, agentPool=ap1|Informational'
        The contents of file "${SECURITY_PATCH_CONFIG_DIR}/sources.list" should include 'deb https://snapshot.ubuntu.com/ubuntu/20260710T000000Z jammy main restricted'
        The contents of file "${SECURITY_PATCH_CONFIG_DIR}/apt.conf" should include "Dir::Etc::sourcelist \"${SECURITY_PATCH_CONFIG_DIR}/sources.list\";"
    End

    It 'compares only the current node agent pool timestamp'
        desired_payload='{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"},"ap2":{"goldenTimestamp":"20260715T000000Z"}}}'
        current_payload='{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"},"ap2":{"goldenTimestamp":"20260701T000000Z"}}}'

        When call securityPatchIsCurrent "${desired_payload}" "${current_payload}" "${TEST_NODE_JSON}"
        The status should be success
    End

    It 'detects a changed timestamp for the current node agent pool'
        desired_payload='{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"}}}'
        current_payload='{"agentPools":{"ap1":{"goldenTimestamp":"20260701T000000Z"}}}'

        When call securityPatchIsCurrent "${desired_payload}" "${current_payload}" "${TEST_NODE_JSON}"
        The status should be failure
    End

    It 'retries unattended upgrade until it succeeds'
        eval 'unattended-upgrade() {
            TEST_UNATTENDED_ATTEMPT=$((TEST_UNATTENDED_ATTEMPT + 1))
            echo "unattended-upgrade called: ${TEST_UNATTENDED_ATTEMPT}"
            [ "${TEST_UNATTENDED_ATTEMPT}" -ge 3 ]
        }'
        TEST_UNATTENDED_ATTEMPT=0
        export TEST_UNATTENDED_ATTEMPT

        When call security_patch_unattended_upgrade
        The status should be success
        The output should include 'unattended-upgrade called: 3'
        The output should include 'executed unattended upgrade 3 times'
    End

    It 'fails after ten unattended upgrade attempts'
        eval 'unattended-upgrade() {
            TEST_UNATTENDED_ATTEMPT=$((TEST_UNATTENDED_ATTEMPT + 1))
            echo "unattended-upgrade called: ${TEST_UNATTENDED_ATTEMPT}"
            return 1
        }'
        TEST_UNATTENDED_ATTEMPT=0
        export TEST_UNATTENDED_ATTEMPT

        When call security_patch_unattended_upgrade
        The status should be failure
        The output should include 'unattended-upgrade called: 10'
    End

    It 'uses the private repository service for a network-isolated cluster'
        TEST_NODE_JSON="$(security_patch_test_node_json '10.0.0.1')"
        export TEST_NODE_JSON

        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"}}}' "${TEST_NODE_JSON}"
        The status should be success
        The output should include 'securityPatch update completed successfully: 20260710T000000Z'
        The contents of file "${SECURITY_PATCH_CONFIG_DIR}/sources.list" should include 'deb http://10.0.0.1/ubuntu jammy main restricted'
        The contents of file "${SECURITY_PATCH_CONFIG_DIR}/sources.list" should not include 'snapshot.ubuntu.com'
    End

    It 'uses the snapshot service when the annotations map is absent'
        When call security_patch_repo_endpoint '{"metadata":{}}'
        The status should be success
        The output should equal 'snapshot.ubuntu.com'
    End

    It 'falls back to the snapshot service for an invalid repository annotation'
        TEST_NODE_JSON="$(security_patch_test_node_json 'example.com')"
        export TEST_NODE_JSON

        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"}}}' "${TEST_NODE_JSON}"
        The status should be success
        The output should include 'securityPatch update completed successfully: 20260710T000000Z'
        The stderr should include 'ignoring invalid live patching repo service: example.com'
        The contents of file "${SECURITY_PATCH_CONFIG_DIR}/sources.list" should include 'snapshot.ubuntu.com/ubuntu/20260710T000000Z'
    End

    It 'succeeds without action when the component has no agent pool profiles'
        When call updateSecurityPatch '{}' "${TEST_NODE_JSON}"
        The status should be success
        The output should include 'securityPatch has no profile for agent pool ap1; no action needed'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.NoAction|No profile for agent pool ap1|Informational'
        The output should not include 'apt_get_update_with_opts called'
        The output should not include 'kubectl called'
    End

    It 'succeeds without action when the node agent pool has no profile'
        When call updateSecurityPatch '{"agentPools":{"ap2":{"goldenTimestamp":"20260710T000000Z"}}}' "${TEST_NODE_JSON}"
        The status should be success
        The output should include 'securityPatch has no profile for agent pool ap1; no action needed'
        The output should not include 'apt_get_update_with_opts called'
        The output should not include 'kubectl called'
    End

    It 'fails when the selected profile has no golden timestamp'
        When call updateSecurityPatch '{"agentPools":{"ap1":{}}}' "${TEST_NODE_JSON}"
        The status should be failure
        The output should include 'securityPatch goldenTimestamp is missing for agent pool: ap1'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.Failed|reason=GoldenTimestampMissing|Error'
        The output should not include 'apt_get_update_with_opts called'
    End

    It 'fails when the component configuration is malformed'
        When call updateSecurityPatch 'not-json' "${TEST_NODE_JSON}"
        The status should be failure
        The output should include 'securityPatch configuration is invalid'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.Failed|reason=ConfigurationInvalid|Error'
        The output should not include 'no action needed'
        The output should not include 'apt_get_update_with_opts called'
    End

    It 'fails when agentPools is not an object'
        When call updateSecurityPatch '{"agentPools":[]}' "${TEST_NODE_JSON}"
        The status should be failure
        The output should include 'securityPatch agentPools must be an object'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.Failed|reason=AgentPoolsInvalid|Error'
        The output should not include 'no action needed'
        The output should not include 'apt_get_update_with_opts called'
    End

    It 'fails when the selected golden timestamp is malformed'
        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"not-a-timestamp"}}}' "${TEST_NODE_JSON}"
        The status should be failure
        The output should include 'securityPatch goldenTimestamp is invalid: not-a-timestamp'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.Failed|reason=GoldenTimestampInvalid|Error'
        The output should not include 'apt_get_update_with_opts called'
    End

    It 'returns failure when apt metadata refresh fails'
        TEST_APT_UPDATE_STATUS=1
        export TEST_APT_UPDATE_STATUS

        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"}}}' "${TEST_NODE_JSON}"
        The status should be failure
        The output should include 'apt_get_update_with_opts failed'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.Failed|reason=AptMetadataRefreshFailed|Error'
        The output should not include 'unattended-upgrade called'
        The output should not include 'kubectl called'
    End

    It 'returns failure when the legacy status annotation cannot be updated'
        TEST_ANNOTATE_STATUS=1
        export TEST_ANNOTATE_STATUS

        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"}}}' "${TEST_NODE_JSON}"
        The status should be failure
        The output should include 'unattended-upgrade called'
        The output should include 'kubectl called with args: annotate --overwrite node aks-node-1 kubernetes.azure.com/live-patching-current-timestamp=20260710T000000Z'
        The output should include 'failed to update legacy securityPatch status annotation'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.Failed|reason=LegacyStatusAnnotationFailed|Error'
        The output should not include 'securityPatch update completed successfully'
    End

    It 'returns failure before apt update when apt configuration cannot be written'
        SECURITY_PATCH_CONFIG_DIR="/dev/null/security-patch"
        export SECURITY_PATCH_CONFIG_DIR

        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"}}}' "${TEST_NODE_JSON}"
        The status should be failure
        The output should include 'failed to generate securityPatch apt configuration'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.Failed|reason=AptConfigurationFailed|Error'
        The stderr should include 'Not a directory'
        The output should not include 'apt_get_update_with_opts called'
        The output should not include 'unattended-upgrade called'
    End

    It 'does not mask a sources list write failure when apt config is writable'
        mkdir -p "${SECURITY_PATCH_CONFIG_DIR}/sources.list"

        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"}}}' "${TEST_NODE_JSON}"
        The status should be failure
        The output should include 'failed to generate securityPatch apt configuration'
        The stderr should include 'Is a directory'
        The path "${SECURITY_PATCH_CONFIG_DIR}/apt.conf" should not be exist
        The output should not include 'apt_get_update_with_opts called'
        The output should not include 'unattended-upgrade called'
    End

    It 'returns failure before apt update when the Ubuntu codename is unavailable'
        Mock lsb_release
            exit 1
        End

        When call updateSecurityPatch '{"agentPools":{"ap1":{"goldenTimestamp":"20260710T000000Z"}}}' "${TEST_NODE_JSON}"
        The status should be failure
        The output should include 'failed to determine Ubuntu codename'
        The contents of file "${TEST_EVENT_LOG}/events" should include 'AKS.LivePatching.securityPatch.Failed|reason=UbuntuCodenameReadFailed|Error'
        The output should not include 'apt_get_update_with_opts called'
        The output should not include 'unattended-upgrade called'
    End
End
