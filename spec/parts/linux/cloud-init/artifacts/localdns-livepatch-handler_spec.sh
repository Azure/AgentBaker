#!/bin/bash
# shellcheck disable=SC2034,SC2317

Describe 'localdns-livepatch-handler.sh'
    setup() {
        TEST_DIR="/tmp/localdns-livepatch-handler-test"
        rm -rf "${TEST_DIR}"
        mkdir -p "${TEST_DIR}/localdns" "${TEST_DIR}/run"

        LOCALDNS_LIVEPATCHED_COREFILE="${TEST_DIR}/localdns/livepatched.localdns.corefile"
        LOCALDNS_BAKED_COREFILE="${TEST_DIR}/localdns/localdns.corefile"
        LOCALDNS_SERVED_COREFILE="${TEST_DIR}/localdns/updated.localdns.corefile"
        LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE="${TEST_DIR}/run/vnet-dns-forward-servers"
        LOCALDNS_AKS_NODE_CONTROLLER="${TEST_DIR}/aks-node-controller"
        LOCALDNS_RELOAD_TIMEOUT_SECONDS=5
        LOCALDNS_RELOAD_POLL_SECONDS=0
        Include ./parts/linux/cloud-init/artifacts/localdns-livepatch-handler.sh

        # The baked corefile carries the Azure DNS placeholder for VnetDNS traffic and the CoreDNS
        # service IP for cluster.local, as the template renders them.
        cat > "${LOCALDNS_BAKED_COREFILE}" <<'EOF'
health-check.localdns.local:53 {
    bind 169.254.10.10 169.254.10.11
    reload
    whoami
}
.:53 {
    bind 169.254.10.10
    forward . 168.63.129.16 {
    }
    cache 3600
}
cluster.local:53 {
    bind 169.254.10.11
    forward . 10.0.0.10 {
    }
    cache 3600
}
EOF
        # The Corefile aks-node-controller renders for the patch: only the root cache duration changed.
        TEST_PATCH_SOURCE="${TEST_DIR}/patch.corefile"
        sed 's/cache 3600/cache 7200/' "${LOCALDNS_BAKED_COREFILE}" > "${TEST_PATCH_SOURCE}"
        sed -i '0,/cache 7200/! s/cache 7200/cache 3600/' "${TEST_PATCH_SOURCE}"

        # What localdns.sh recorded and served at startup on a custom-DNS VNet.
        echo "10.1.0.4 10.1.0.5" > "${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}"
        render_expected "${LOCALDNS_BAKED_COREFILE}" "10.1.0.4 10.1.0.5" > "${LOCALDNS_SERVED_COREFILE}"
        record_initial_served

        cat > "${LOCALDNS_AKS_NODE_CONTROLLER}" <<'EOF'
#!/bin/bash
cat > /dev/null
printf '%s\n' "$*" > "${TEST_ANC_ARGS_FILE}"
output=""
while [ "$#" -gt 0 ]; do
    case "$1" in
        --output) output="$2"; shift 2 ;;
        *) shift ;;
    esac
done
if [ "${TEST_ANC_OUTCOME}" = "applied" ]; then
    cp "${TEST_PATCH_SOURCE}" "${output}"
    echo "v2" > "${output}.version"
fi
[ -z "${TEST_ANC_ERROR:-}" ] || echo "error: ${TEST_ANC_ERROR}"
echo "${TEST_ANC_OUTCOME}"
exit "${TEST_ANC_EXIT:-0}"
EOF
        chmod +x "${LOCALDNS_AKS_NODE_CONTROLLER}"

        TEST_ANC_OUTCOME="applied"
        TEST_ANC_EXIT=0
        TEST_ANC_ERROR=""
        TEST_ANC_ARGS_FILE="${TEST_DIR}/anc-args"
        TEST_LOCALDNS_ACTIVE="true"
        TEST_RESTART_STATUS=0
        TEST_RELOAD_MODE="reload"
        export TEST_ANC_OUTCOME TEST_ANC_EXIT TEST_ANC_ERROR TEST_ANC_ARGS_FILE TEST_PATCH_SOURCE
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    render_expected() {
        sed -e "s|168.63.129.16|$2|g" "$1"
    }

    record_initial_served() {
        TEST_INITIAL_SERVED_SHA="$(sha256sum "${LOCALDNS_SERVED_COREFILE}" | awk '{print $1}')"
        cp "${LOCALDNS_SERVED_COREFILE}" "${TEST_DIR}/initial-served.corefile"
    }

    served_equals() {
        cmp -s "$1" "${LOCALDNS_SERVED_COREFILE}"
    }

    served_unchanged() {
        cmp -s "${TEST_DIR}/initial-served.corefile" "${LOCALDNS_SERVED_COREFILE}"
    }

    systemctl() {
        echo "systemctl $*"
        case "$1" in
            is-active) [ "${TEST_LOCALDNS_ACTIVE}" = "true" ] ;;
            restart) return "${TEST_RESTART_STATUS}" ;;
        esac
    }

    # Stands in for CoreDNS's metrics endpoint. The reload plugin publishes a hash of the config it
    # loaded; here the served file's own hash plays that part, so a changed file reads as a reload.
    curl() {
        local served_sha failed=0
        if [ "${TEST_RELOAD_MODE}" = "down" ]; then
            return 7
        fi
        served_sha="$(sha256sum "${LOCALDNS_SERVED_COREFILE}" | awk '{print $1}')"
        case "${TEST_RELOAD_MODE}" in
            stuck) served_sha="${TEST_INITIAL_SERVED_SHA}" ;;
            reject) [ "${served_sha}" != "${TEST_INITIAL_SERVED_SHA}" ] && failed=1 ;;
        esac
        echo "coredns_reload_failed_total ${failed}"
        echo "coredns_reload_version_info{hash=\"sha512\",value=\"${served_sha}\"} 1"
    }

    Describe 'in-place reload'
        It 'serves the patch through a CoreDNS reload without restarting localdns'
            When call updateLocalDNS '{"agentPools":{}}'
            The status should be success
            The output should include 'CoreDNS reloaded the localDNS corefile'
            The output should include 'localDNS update completed successfully'
            The output should not include 'systemctl restart'
            The contents of file "${LOCALDNS_SERVED_COREFILE}" should include 'cache 7200'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v2'
        End

        It 'keeps the VNet DNS servers bootstrap recorded and leaves cluster.local on CoreDNS'
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'CoreDNS reloaded the localDNS corefile'
            The contents of file "${LOCALDNS_SERVED_COREFILE}" should include 'forward . 10.1.0.4 10.1.0.5 {'
            The contents of file "${LOCALDNS_SERVED_COREFILE}" should include 'forward . 10.0.0.10 {'
            The contents of file "${LOCALDNS_SERVED_COREFILE}" should not include '168.63.129.16'
        End

        It 'keeps an Azure DNS VNet forwarding to 168.63.129.16'
            echo "168.63.129.16" > "${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}"
            render_expected "${LOCALDNS_BAKED_COREFILE}" "168.63.129.16" > "${LOCALDNS_SERVED_COREFILE}"
            record_initial_served
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'CoreDNS reloaded the localDNS corefile'
            The contents of file "${LOCALDNS_SERVED_COREFILE}" should include 'forward . 168.63.129.16 {'
            The contents of file "${LOCALDNS_SERVED_COREFILE}" should include 'cache 7200'
        End

        It 'leaves CoreDNS alone when the served corefile is already current'
            cp "${TEST_PATCH_SOURCE}" "${LOCALDNS_LIVEPATCHED_COREFILE}"
            render_expected "${TEST_PATCH_SOURCE}" "10.1.0.4 10.1.0.5" > "${LOCALDNS_SERVED_COREFILE}"
            TEST_ANC_OUTCOME="alreadyCurrent"
            # Any metrics read would fail, proving no reload was waited on.
            TEST_RELOAD_MODE="down"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS served corefile is already current'
            The output should not include 'systemctl restart'
        End

        It 'restores the previous corefiles when CoreDNS rejects the patch'
            TEST_RELOAD_MODE="reject"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'CoreDNS rejected the localDNS corefile and kept serving the previous one'
            The output should not include 'systemctl restart'
            Assert served_unchanged
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should not be exist
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should not be exist
        End

        It 'restores an earlier patch rather than dropping it when CoreDNS rejects a new one'
            echo "earlier-patch" > "${LOCALDNS_LIVEPATCHED_COREFILE}"
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            TEST_RELOAD_MODE="reject"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'CoreDNS rejected the localDNS corefile'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'earlier-patch'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v1'
        End
    End

    Describe 'restart fallback'
        It 'restarts localdns when no forward servers were recorded'
            rm -f "${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'no usable VNet DNS forward servers recorded'
            The output should include 'systemctl restart localdns.service'
            Assert served_unchanged
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should be exist
        End

        It 'restarts localdns when the recorded servers include a localdns listener'
            echo "169.254.10.10 10.1.0.4" > "${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'no usable VNet DNS forward servers recorded'
            The output should include 'systemctl restart localdns.service'
        End

        It 'restarts localdns when the recorded servers are not IP addresses'
            echo "not-an-ip" > "${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'no usable VNet DNS forward servers recorded'
            The output should include 'systemctl restart localdns.service'
        End

        It 'restarts localdns when the running corefile has no reload plugin'
            sed -i '/^    reload$/d' "${LOCALDNS_SERVED_COREFILE}"
            record_initial_served
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'running localDNS corefile has no reload plugin'
            The output should include 'systemctl restart localdns.service'
            Assert served_unchanged
        End

        It 'restarts localdns when CoreDNS metrics are unavailable'
            TEST_RELOAD_MODE="down"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'CoreDNS reload metrics are unavailable'
            The output should include 'systemctl restart localdns.service'
            Assert served_unchanged
        End

        It 'restarts localdns when CoreDNS does not reload in time'
            TEST_RELOAD_MODE="stuck"
            LOCALDNS_RELOAD_TIMEOUT_SECONDS=1
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'CoreDNS did not reload the localDNS corefile within 1s'
            The output should include 'systemctl restart localdns.service'
        End

        It 'restores the previous corefiles and restarts on them when the fallback restart fails'
            rm -f "${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}"
            TEST_RESTART_STATUS=1
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'failed to restart localdns.service'
            The output should include 'failed to restart localdns.service on the restored corefile'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should not be exist
            Assert served_unchanged
        End
    End

    Describe 'no config for this node'
        It 'revokes an earlier patch and serves the baked corefile again'
            echo "earlier-patch" > "${LOCALDNS_LIVEPATCHED_COREFILE}"
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            render_expected "${TEST_PATCH_SOURCE}" "10.1.0.4 10.1.0.5" > "${LOCALDNS_SERVED_COREFILE}"
            record_initial_served
            render_expected "${LOCALDNS_BAKED_COREFILE}" "10.1.0.4 10.1.0.5" > "${TEST_DIR}/expected.corefile"
            TEST_ANC_OUTCOME="noCorefileData"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS has no config applicable to this node; serving the baked corefile'
            The output should include 'CoreDNS reloaded the localDNS corefile'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should not be exist
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should not be exist
            Assert served_equals "${TEST_DIR}/expected.corefile"
        End

        It 'changes nothing when there was no patch to revoke'
            TEST_ANC_OUTCOME="noCorefileData"
            TEST_RELOAD_MODE="down"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS served corefile is already current'
            The output should not include 'systemctl restart'
            Assert served_unchanged
        End
    End

    Describe 'other outcomes'
        It 'writes the patch for the next start when localdns is not running'
            TEST_LOCALDNS_ACTIVE="false"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localdns.service is not running'
            The output should not include 'systemctl restart'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should be exist
            Assert served_unchanged
        End

        It 'fails without changing anything on notFound'
            TEST_ANC_OUTCOME="notFound"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'localDNS LPS config is not available'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should not be exist
            Assert served_unchanged
        End

        It 'fails and restores when aks-node-controller exits non-zero'
            TEST_ANC_EXIT=1
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'localDNS config apply failed'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should not be exist
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should not be exist
        End

        It 'fails when the aks-node-controller binary is missing'
            rm -f "${LOCALDNS_AKS_NODE_CONTROLLER}"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'aks-node-controller binary is required for localDNS live patching'
        End
    End

    Describe 'reported current'
        It 'reports the corefileVersion of the applied patch'
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            When call localDNSCurrent
            The output should equal 'v1'
        End

        It 'reports nothing when no live patch is applied'
            When call localDNSCurrent
            The output should equal ''
            The status should be success
        End

        It 'reports the new version once a patch is applied'
            updateLocalDNS '{}' > /dev/null
            When call localDNSCurrent
            The output should equal 'v2'
        End

        It 'reports nothing once a patch is revoked'
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            TEST_ANC_OUTCOME="noCorefileData"
            updateLocalDNS '{}' > /dev/null
            When call localDNSCurrent
            The output should equal ''
        End

        It 'keeps reporting the previous version when a patch fails'
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            TEST_ANC_EXIT=1
            updateLocalDNS '{}' > /dev/null || true
            When call localDNSCurrent
            The output should equal 'v1'
        End
    End

    Describe 'failure message'
        It 'reports nothing when the patch succeeds'
            updateLocalDNS '{}' > /dev/null
            When call localDNSFailureMessage
            The output should equal ''
        End

        It "passes on aks-node-controller's reason when the apply fails"
            TEST_ANC_EXIT=1
            TEST_ANC_ERROR="no bootstrap source on this node carries the CoreDNS service IP"
            updateLocalDNS '{}' > /dev/null || true
            When call localDNSFailureMessage
            The output should equal 'aks-node-controller apply-localdns-config failed: no bootstrap source on this node carries the CoreDNS service IP'
        End

        It 'reports the apply failure when aks-node-controller gives no reason'
            TEST_ANC_EXIT=1
            updateLocalDNS '{}' > /dev/null || true
            When call localDNSFailureMessage
            The output should equal 'aks-node-controller apply-localdns-config failed'
        End

        It 'reports that CoreDNS rejected the corefile'
            TEST_RELOAD_MODE="reject"
            updateLocalDNS '{}' > /dev/null || true
            When call localDNSFailureMessage
            The output should equal 'CoreDNS rejected the new corefile and kept serving the previous one'
        End

        It 'reports a failed fallback restart'
            TEST_RELOAD_MODE="down"
            TEST_RESTART_STATUS=1
            updateLocalDNS '{}' > /dev/null || true
            When call localDNSFailureMessage
            The output should equal 'localdns.service restart failed; restored the previous corefile'
        End

        It 'reports a missing aks-node-controller'
            rm -f "${LOCALDNS_AKS_NODE_CONTROLLER}"
            updateLocalDNS '{}' > /dev/null || true
            When call localDNSFailureMessage
            The output should equal 'aks-node-controller binary not found'
        End

        It 'reports an unexpected outcome'
            TEST_ANC_OUTCOME="bogus"
            updateLocalDNS '{}' > /dev/null || true
            When call localDNSFailureMessage
            The output should equal 'unexpected apply outcome: bogus'
        End

        It 'clears the message when a later run succeeds'
            TEST_RELOAD_MODE="reject"
            updateLocalDNS '{}' > /dev/null || true
            TEST_RELOAD_MODE="reload"
            updateLocalDNS '{}' > /dev/null
            When call localDNSFailureMessage
            The output should equal ''
        End
    End

    Describe 'agent pool selection'
        It "passes the Node object's agent pool to aks-node-controller"
            When call updateLocalDNS '{}' '{"metadata":{"labels":{"kubernetes.azure.com/agentpool":"ap1"}}}'
            The status should be success
            The output should include 'localDNS update completed successfully'
            The contents of file "${TEST_ANC_ARGS_FILE}" should equal "apply-localdns-config --config-file - --output ${LOCALDNS_LIVEPATCHED_COREFILE} --agent-pool ap1"
        End

        It 'leaves the agent pool to aks-node-controller when the Node object has no label'
            When call updateLocalDNS '{}' '{"metadata":{"labels":{}}}'
            The status should be success
            The output should include 'localDNS update completed successfully'
            The contents of file "${TEST_ANC_ARGS_FILE}" should equal "apply-localdns-config --config-file - --output ${LOCALDNS_LIVEPATCHED_COREFILE}"
        End
    End

    Describe 'is-current check'
        NODE_AP1='{"metadata":{"labels":{"kubernetes.azure.com/agentpool":"ap1"}}}'
        AP1_V1='"ap1":{"corefileVersion":"v1","localDnsProfile":{"enableLocalDns":true}}'
        AP1_V2='"ap1":{"corefileVersion":"v2","localDnsProfile":{"enableLocalDns":true}}'
        AP2_V1='"ap2":{"corefileVersion":"v1","localDnsProfile":{"enableLocalDns":true}}'
        AP2_V2='"ap2":{"corefileVersion":"v2","localDnsProfile":{"enableLocalDns":true}}'

        It 'is current when the payloads are identical'
            When call localDNSIsCurrent "{\"agentPools\":{${AP1_V1}}}" "{\"agentPools\":{${AP1_V1}}}" "${NODE_AP1}"
            The status should be success
        End

        It 'is current when only another agent pool changed'
            When call localDNSIsCurrent "{\"agentPools\":{${AP1_V1},${AP2_V2}}}" "{\"agentPools\":{${AP1_V1},${AP2_V1}}}" "${NODE_AP1}"
            The status should be success
        End

        It 'is current when this pool is absent from both and another pool changed'
            When call localDNSIsCurrent "{\"agentPools\":{${AP2_V2}}}" "{\"agentPools\":{${AP2_V1}}}" "${NODE_AP1}"
            The status should be success
        End

        It "is current when this pool's version is unchanged, whatever else in its entry changed"
            When call localDNSIsCurrent \
                '{"agentPools":{"ap1":{"corefileVersion":"v1","localDnsProfile":{"enableLocalDns":true,"cpuLimitInMilliCores":4000}}}}' \
                "{\"agentPools\":{${AP1_V1}}}" "${NODE_AP1}"
            The status should be success
        End

        It 'is not current when this pool changed'
            When call localDNSIsCurrent "{\"agentPools\":{${AP1_V2},${AP2_V1}}}" "{\"agentPools\":{${AP1_V1},${AP2_V1}}}" "${NODE_AP1}"
            The status should be failure
        End

        It 'is not current when this pool was removed, so the handler revokes the patch'
            When call localDNSIsCurrent "{\"agentPools\":{${AP2_V1}}}" "{\"agentPools\":{${AP1_V1},${AP2_V1}}}" "${NODE_AP1}"
            The status should be failure
        End

        It 'is not current when this pool was added'
            When call localDNSIsCurrent "{\"agentPools\":{${AP1_V1},${AP2_V1}}}" "{\"agentPools\":{${AP2_V1}}}" "${NODE_AP1}"
            The status should be failure
        End

        It 'selects this pool from the profiles key the same way'
            When call localDNSIsCurrent '{"profiles":{"ap1":{"configChecksum":"c1"},"ap2":{"configChecksum":"c2"}}}' \
                '{"profiles":{"ap1":{"configChecksum":"c1"},"ap2":{"configChecksum":"c1"}}}' "${NODE_AP1}"
            The status should be success
        End

        It 'is not current when this pool changed under the profiles key'
            When call localDNSIsCurrent '{"profiles":{"ap1":{"configChecksum":"c2"}}}' '{"profiles":{"ap1":{"configChecksum":"c1"}}}' "${NODE_AP1}"
            The status should be failure
        End

        # Without a version to compare, the handler runs and checks what is applied on disk.
        It "is not current when this pool's entry has no version, even if unchanged"
            When call localDNSIsCurrent '{"agentPools":{"ap1":{"localDnsProfile":{"enableLocalDns":true}}}}' \
                '{"agentPools":{"ap1":{"localDnsProfile":{"enableLocalDns":true}}}}' "${NODE_AP1}"
            The status should be failure
        End

        It 'is not current when the payload is not keyed by agent pool, even if unchanged'
            When call localDNSIsCurrent '{"corefile":"a"}' '{"corefile":"a"}' "${NODE_AP1}"
            The status should be failure
        End

        It 'is not current when the node has no agent pool label, even if the payload is unchanged'
            When call localDNSIsCurrent "{\"agentPools\":{${AP1_V1}}}" "{\"agentPools\":{${AP1_V1}}}" '{"metadata":{"labels":{}}}'
            The status should be failure
        End

        It 'is not current when the desired payload is malformed'
            When call localDNSIsCurrent '{"agentPools":"ap1"}' "{\"agentPools\":{${AP1_V1}}}" "${NODE_AP1}"
            The status should be failure
        End

        It 'is not current when the recorded payload is not JSON'
            When call localDNSIsCurrent "{\"agentPools\":{${AP1_V1}}}" 'not-json' "${NODE_AP1}"
            The status should be failure
        End
    End
End
