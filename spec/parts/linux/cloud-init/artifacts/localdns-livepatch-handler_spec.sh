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
        LOCALDNS_FORWARD_IPS_PROM_FILE="${TEST_DIR}/localdns/forward_ips.prom"
        LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE="${TEST_DIR}/run/vnet-dns-forward-servers"
        LOCALDNS_AKS_NODE_CONTROLLER="${TEST_DIR}/aks-node-controller"
        LOCALDNS_RELOAD_TIMEOUT_SECONDS=5
        LOCALDNS_RELOAD_POLL_SECONDS=0
        LOCALDNS_READY_TIMEOUT_SECONDS=1
        LOCALDNS_LIMITS_SCRIPT=./parts/linux/cloud-init/artifacts/localdns-limits.sh
        Include ./parts/linux/cloud-init/artifacts/localdns-livepatch-handler.sh
        Include ./spec/parts/linux/cloud-init/artifacts/localdns_limits_fakes.sh
        # A running localdns.slice on the CSE limits of the e2e node config (2008 millicores, 128 MB).
        setup_fake_localdns_slice "${TEST_DIR}/slice" "200.8%" "128M"

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
# Like aks-node-controller, uncommit the old version before writing a changed Corefile, and print
# the version and the profile's limits before the outcome.
if [ "${TEST_ANC_OUTCOME}" = "rendered" ]; then
    rm -f "${output}.version"
    cp "${TEST_PATCH_SOURCE}" "${output}"
    if [ "${TEST_SIMULATE_RESTART}" = "true" ]; then
        # A localdns start right after the write builds the served Corefile from the new source.
        sed 's|168.63.129.16|10.1.0.4 10.1.0.5|g' "${output}" > "${TEST_SERVED_COREFILE}"
    fi
fi
[ -z "${TEST_ANC_ERROR:-}" ] || echo "error: ${TEST_ANC_ERROR}"
case "${TEST_ANC_OUTCOME}" in
    rendered|alreadyCurrent)
        echo "version: v2"
        [ -z "${TEST_ANC_CPU:-}" ] || echo "cpuQuota: ${TEST_ANC_CPU}"
        [ -z "${TEST_ANC_MEMORY:-}" ] || echo "memoryMax: ${TEST_ANC_MEMORY}"
        ;;
esac
echo "${TEST_ANC_OUTCOME}"
exit "${TEST_ANC_EXIT:-0}"
EOF
        chmod +x "${LOCALDNS_AKS_NODE_CONTROLLER}"

        TEST_ANC_OUTCOME="rendered"
        TEST_ANC_EXIT=0
        TEST_ANC_ERROR=""
        TEST_ANC_ARGS_FILE="${TEST_DIR}/anc-args"
        TEST_ANC_CPU=""
        TEST_ANC_MEMORY=""
        TEST_SIMULATE_RESTART=""
        TEST_SERVED_COREFILE="${LOCALDNS_SERVED_COREFILE}"
        TEST_EVENTS_FILE="${TEST_DIR}/events"
        TEST_RESTART_STATUS=0
        TEST_RELOAD_MODE="reload"
        TEST_READY_BODY="OK"
        export TEST_ANC_OUTCOME TEST_ANC_EXIT TEST_ANC_ERROR TEST_ANC_ARGS_FILE TEST_PATCH_SOURCE TEST_ANC_CPU TEST_ANC_MEMORY \
            TEST_SIMULATE_RESTART TEST_SERVED_COREFILE
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

    # Stands in for the reconciler's guest agent event writer: one "task|message|level" line per event.
    knead_emit_event() {
        printf '%s|%s|%s\n' "$1" "$2" "${3:-Informational}" >> "${TEST_EVENTS_FILE}"
    }

    systemctl() {
        case "$1" in
            show|set-property)
                fake_localdns_systemctl "$@"
                return
                ;;
        esac
        echo "systemctl $*"
        case "$1" in
            is-active) [ "${FAKE_LOCALDNS_SERVICE_STATE}" = active ] ;;
            restart) return "${TEST_RESTART_STATUS}" ;;
        esac
    }

    # Stands in for CoreDNS's ready and metrics endpoints. The reload plugin publishes a hash of the
    # config it loaded; here the served file's own hash plays that part, so a changed file reads as a
    # reload.
    curl() {
        local served_sha failed=0
        case "$*" in
            */ready)
                echo "${TEST_READY_BODY}"
                return 0
                ;;
        esac
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
            record_initial_served
            # A new version that only changed the limits renders the same Corefile.
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            # Any metrics read would fail, proving no reload was waited on.
            TEST_RELOAD_MODE="down"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS served corefile is already current'
            The output should not include 'systemctl restart'
            Assert served_unchanged
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v2'
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

        # A patch can add or drop zones, or move one between VNet DNS and CoreDNS, which changes the
        # forward targets the forward_info metrics report.
        It 'refreshes the forward_info metrics once CoreDNS has reloaded the patch'
            cat >> "${TEST_PATCH_SOURCE}" <<'EOF'
contoso.internal:53 {
    bind 169.254.10.10
    forward . 168.63.129.16 {
    }
}
EOF
            echo 'previous metrics' > "${LOCALDNS_FORWARD_IPS_PROM_FILE}"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'CoreDNS reloaded the localDNS corefile'
            The output should include "Successfully exported forward IPs to ${LOCALDNS_FORWARD_IPS_PROM_FILE}"
            The contents of file "${LOCALDNS_FORWARD_IPS_PROM_FILE}" should include 'localdns_vnetdns_forward_info{ip="10.1.0.4",block="contoso.internal:53",status="ok"} 1'
            The contents of file "${LOCALDNS_FORWARD_IPS_PROM_FILE}" should include 'localdns_vnetdns_forward_info{ip="10.1.0.5",block=".:53",status="ok"} 1'
            The contents of file "${LOCALDNS_FORWARD_IPS_PROM_FILE}" should include 'localdns_kubedns_forward_info{ip="10.0.0.10",block="cluster.local:53",status="ok"} 1'
        End

        It 'leaves the forward_info metrics alone when CoreDNS rejects the patch'
            echo 'previous metrics' > "${LOCALDNS_FORWARD_IPS_PROM_FILE}"
            TEST_RELOAD_MODE="reject"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'CoreDNS rejected the localDNS corefile'
            The output should not include 'exported forward IPs'
            The contents of file "${LOCALDNS_FORWARD_IPS_PROM_FILE}" should equal 'previous metrics'
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

        It 'leaves the revoke to the next run when a localdns start is in progress'
            echo "earlier-patch" > "${LOCALDNS_LIVEPATCHED_COREFILE}"
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            TEST_ANC_OUTCOME="noCorefileData"
            FAKE_LOCALDNS_SERVICE_STATE="activating"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'localdns.service is starting; leaving the patch to the next run'
            The output should not include 'systemctl restart'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'earlier-patch'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v1'
            Assert served_unchanged
        End
    End

    Describe 'other outcomes'
        It 'writes and commits the patch for the next start, but fails, when localdns is not running'
            FAKE_LOCALDNS_SERVICE_STATE="inactive"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'localdns.service is not running'
            The output should not include 'systemctl restart'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should be exist
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v2'
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

    Describe 'limits'
        saved_cpu() {
            sed -n 's/^CPUQuota=//p' "${LOCALDNS_LIMITS_SAVED_DIR}/50-CPUQuota.conf"
        }

        saved_memory() {
            sed -n 's/^MemoryMax=//p' "${LOCALDNS_LIMITS_SAVED_DIR}/50-MemoryMax.conf"
        }

        # A node an earlier patch (v1) left on its own Corefile and a CPU limit of 400%.
        write_earlier_patch() {
            echo "earlier-patch" > "${LOCALDNS_LIVEPATCHED_COREFILE}"
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            fake_localdns_apply_live "CPUQuota=400%"
            fake_localdns_save "CPUQuota=400%"
        }

        It 'applies the limits before serving the corefile, then commits the version'
            TEST_ANC_CPU="300.0%"
            TEST_ANC_MEMORY="512M"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'CoreDNS reloaded the localDNS corefile'
            The output should include 'committed localDNS corefileVersion v2'
            The output should include 'localDNS update completed successfully'
            The output should include 'systemctl set-property localdns.slice CPUQuota=300%'
            The output should include 'systemctl set-property localdns.slice MemoryMax=536870912'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal '300000 100000'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/memory.max" should equal '536870912'
            The result of function saved_cpu should equal '300%'
            The result of function saved_memory should equal '536870912'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v2'
        End

        It 'sets a limit the payload leaves out back to the baked value'
            write_earlier_patch
            TEST_ANC_MEMORY="512M"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS update completed successfully'
            The output should include 'CPUQuota=200.8%'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal '200800 100000'
        End

        It 'changes nothing when the preflight fails'
            write_earlier_patch
            FAKE_SYSTEMCTL_DOWN=true
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'localDNS limits preflight failed'
            The path "${TEST_ANC_ARGS_FILE}" should not be exist
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'earlier-patch'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v1'
            Assert served_unchanged
            The variable LOCALDNS_FAILURE_MESSAGE should equal 'systemctl did not respond; limits and corefile left unchanged'
        End

        It 'changes nothing when the baked limits cannot be read'
            sed -i 's/^CPUQuota=.*/CPUQuota=lots/' "${LOCALDNS_LIMITS_BAKED_SLICE_FILE}"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'localDNS limits preflight failed'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should not be exist
            The variable LOCALDNS_FAILURE_MESSAGE should equal "cannot read the baked limits from ${LOCALDNS_LIMITS_BAKED_SLICE_FILE}; limits and corefile left unchanged"
        End

        It 'reverts the limits and restores the previous corefile and version when a limit fails'
            write_earlier_patch
            TEST_ANC_CPU="300.0%"
            TEST_ANC_MEMORY="512M"
            FAKE_SET_PROPERTY_FAIL=MemoryMax
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'live patching the localDNS limits failed'
            The output should not include 'CoreDNS reloaded'
            The output should include 'CPUQuota=400%'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal '400000 100000'
            The result of function saved_cpu should equal '400%'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'earlier-patch'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v1'
            Assert served_unchanged
            The variable LOCALDNS_FAILURE_MESSAGE should equal 'live patching the memory limit failed (systemctl set-property MemoryMax=536870912 failed: Failed to set unit properties on localdns.slice: Access denied); reverted to the previous corefile and limits: cpu 400%, memory 128M'
        End

        It 'catches a limit that was applied but not saved'
            TEST_ANC_CPU="300.0%"
            FAKE_SET_PROPERTY_UNSAVED=CPUQuota
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'live patching the localDNS limits failed'
            The output should include 'CPUQuota=300%'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal '200800 100000'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should not be exist
            The variable LOCALDNS_FAILURE_MESSAGE should equal 'live patching the cpu limit failed (applied but not saved: live 300%, saved 200.8%); reverted to the previous corefile and limits: cpu 200.8%, memory 128M'
        End

        It 'removes the drop-ins and the version when the revert fails too'
            write_earlier_patch
            TEST_ANC_CPU="300.0%"
            TEST_ANC_MEMORY="512M"
            # CPUQuota is set, MemoryMax fails, and so does setting CPUQuota back.
            FAKE_SET_PROPERTY_FAIL_FROM=2
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'live patching the localDNS limits failed'
            The output should include 'reverting the localdns.slice limits failed'
            The path "${LOCALDNS_LIMITS_SAVED_DIR}/50-CPUQuota.conf" should not be exist
            The path "${LOCALDNS_LIMITS_SAVED_DIR}/50-MemoryMax.conf" should not be exist
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'earlier-patch'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should not be exist
            Assert served_unchanged
            The variable LOCALDNS_FAILURE_MESSAGE should equal 'live patching the memory limit failed (systemctl set-property MemoryMax=536870912 failed: Failed to set unit properties on localdns.slice: Access denied); previous corefile restored; reverting to the previous limits also failed; in effect: cpu 300%, memory 128M; saved: removed, baked applies after a reboot; baked: cpu 200.8%, memory 128M'
        End

        It 'restores the served corefile a localdns start rebuilt from the patch when a limit fails'
            write_earlier_patch
            TEST_SIMULATE_RESTART="true"
            TEST_ANC_MEMORY="512M"
            FAKE_SET_PROPERTY_FAIL=MemoryMax
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'live patching the localDNS limits failed'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'earlier-patch'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v1'
            Assert served_unchanged
        End

        It 'leaves the patch to the next run and reverts the limits when a localdns start is in progress'
            write_earlier_patch
            TEST_ANC_CPU="300.0%"
            FAKE_LOCALDNS_SERVICE_STATE="activating"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'localdns.service is starting; leaving the patch to the next run'
            The output should not include 'systemctl restart'
            The output should not include 'committed localDNS corefileVersion v2'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal '400000 100000'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'earlier-patch'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v1'
            Assert served_unchanged
            The variable LOCALDNS_FAILURE_MESSAGE should equal 'localdns.service was starting; the next run applies the patch; reverted to the previous corefile and limits: cpu 400%, memory 128M'
        End

        It 'reverts the limits too when CoreDNS rejects the corefile'
            write_earlier_patch
            TEST_ANC_CPU="300.0%"
            TEST_RELOAD_MODE="reject"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'CoreDNS rejected the localDNS corefile'
            The output should include 'CPUQuota=400%'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal '400000 100000'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'earlier-patch'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v1'
            Assert served_unchanged
            The variable LOCALDNS_FAILURE_MESSAGE should equal 'CoreDNS rejected the new corefile and kept serving the previous one; reverted to the previous corefile and limits: cpu 400%, memory 128M'
        End

        It 'drops the version when CoreDNS rejects the corefile and the limits cannot be reverted'
            write_earlier_patch
            TEST_ANC_CPU="300.0%"
            TEST_RELOAD_MODE="reject"
            FAKE_SET_PROPERTY_FAIL_FROM=2
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'CoreDNS rejected the localDNS corefile'
            The output should include 'reverting the localdns.slice limits failed'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'earlier-patch'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should not be exist
            The path "${LOCALDNS_LIMITS_SAVED_DIR}/50-CPUQuota.conf" should not be exist
            Assert served_unchanged
            The variable LOCALDNS_FAILURE_MESSAGE should include 'reverting to the previous limits also failed; in effect: cpu 300%, memory 128M'
        End

        It 'fails but keeps the committed version when CoreDNS does not become ready'
            TEST_ANC_CPU="300.0%"
            TEST_READY_BODY=""
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'CoreDNS did not report ready within 1s'
            The output should include 'CPUQuota=300%'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v2'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal '300000 100000'
            The variable LOCALDNS_FAILURE_MESSAGE should equal 'CoreDNS did not report ready within 1s; corefileVersion v2 applied, readiness checked again on the next run'
        End

        It 'reports a not running localdns once the patch is committed'
            FAKE_LOCALDNS_SERVICE_STATE="inactive"
            updateLocalDNS '{}' > /dev/null || true
            When call localDNSFailureMessage
            The output should equal 'localdns.service is not running; corefileVersion v2 applied, readiness checked again on the next run'
        End
    End

    Describe 'alreadyCurrent'
        # The node already has v2's Corefile and version on disk.
        write_current_patch() {
            cp "${TEST_PATCH_SOURCE}" "${LOCALDNS_LIVEPATCHED_COREFILE}"
            echo "v2" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            render_expected "${TEST_PATCH_SOURCE}" "10.1.0.4 10.1.0.5" > "${LOCALDNS_SERVED_COREFILE}"
            record_initial_served
            TEST_ANC_OUTCOME="alreadyCurrent"
            # Any metrics read would fail, proving CoreDNS is not touched.
            TEST_RELOAD_MODE="down"
        }

        It 'still checks the limits and readiness'
            write_current_patch
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS update completed successfully'
            The output should not include 'localDNS served corefile'
            The output should not include 'systemctl restart'
            The output should include 'already in effect'
            The contents of file "${FAKE_SET_PROPERTY_LOG}" should equal ''
            Assert served_unchanged
        End

        It 'sets limits that drifted without touching CoreDNS'
            write_current_patch
            TEST_ANC_CPU="300.0%"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS update completed successfully'
            The output should not include 'systemctl restart'
            The output should include 'CPUQuota=300%'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal '300000 100000'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v2'
            Assert served_unchanged
        End

        It 'drops the version when a drifted limit cannot be set'
            write_current_patch
            TEST_ANC_CPU="300.0%"
            FAKE_SET_PROPERTY_FAIL=CPUQuota
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'live patching the localDNS limits failed'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal "$(cat "${TEST_PATCH_SOURCE}")"
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should not be exist
            Assert served_unchanged
        End

        It 'fails when CoreDNS is not ready'
            write_current_patch
            TEST_READY_BODY=""
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'CoreDNS did not report ready within 1s'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'v2'
        End
    End

    Describe 'revoke limits'
        It 'resets the limits to the baked ones'
            echo "earlier-patch" > "${LOCALDNS_LIVEPATCHED_COREFILE}"
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            fake_localdns_apply_live "CPUQuota=400%"
            fake_localdns_save "CPUQuota=400%"
            TEST_ANC_OUTCOME="noCorefileData"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS has no config applicable to this node'
            The output should include 'CPUQuota=200.8%'
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal '200800 100000'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should not be exist
        End

        It 'still revokes the corefile, but fails, when the limits cannot be reset'
            echo "earlier-patch" > "${LOCALDNS_LIVEPATCHED_COREFILE}"
            fake_localdns_apply_live "CPUQuota=400%"
            fake_localdns_save "CPUQuota=400%"
            FAKE_SET_PROPERTY_FAIL=all
            TEST_ANC_OUTCOME="noCorefileData"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'failed to reset the localDNS limits to the baked ones'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should not be exist
            The path "${LOCALDNS_LIMITS_SAVED_DIR}/50-CPUQuota.conf" should not be exist
            The output should include 'CPUQuota=200.8%'
            The variable LOCALDNS_FAILURE_MESSAGE should equal 'resetting to the baked limits: the cpu limit failed (systemctl set-property CPUQuota=200.8% failed: Failed to set unit properties on localdns.slice: Access denied); in effect: cpu 400%, memory 128M; saved: removed, baked applies after a reboot; baked: cpu 200.8%, memory 128M'
        End
    End

    Describe 'node without LocalDNS'
        It 'succeeds when the payload has nothing for it'
            rm -f "${LOCALDNS_LIMITS_BAKED_SLICE_FILE}"
            FAKE_LOCALDNS_SERVICE_STATE="inactive"
            TEST_ANC_OUTCOME="noCorefileData"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'LocalDNS is not set up on this node'
            The contents of file "${FAKE_SET_PROPERTY_LOG}" should equal ''
        End

        It 'fails and writes nothing when the payload has a config for it'
            rm -f "${LOCALDNS_LIMITS_BAKED_SLICE_FILE}"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'localdns.slice is not set up here'
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}" should not be exist
            The variable LOCALDNS_FAILURE_MESSAGE should equal "cannot read the baked limits from ${LOCALDNS_LIMITS_BAKED_SLICE_FILE}; limits and corefile left unchanged"
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
            The output should equal 'CoreDNS rejected the new corefile and kept serving the previous one; reverted to the previous corefile and limits: cpu 200.8%, memory 128M'
        End

        It 'reports a failed fallback restart'
            TEST_RELOAD_MODE="down"
            TEST_RESTART_STATUS=1
            updateLocalDNS '{}' > /dev/null || true
            When call localDNSFailureMessage
            The output should equal 'localdns.service restart failed; reverted to the previous corefile and limits: cpu 200.8%, memory 128M'
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

    Describe 'events'
        It 'reports a patch served by a reload as Succeeded'
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'CoreDNS reloaded the localDNS corefile'
            The contents of file "${TEST_EVENTS_FILE}" should equal 'AKS.LivePatching.localDNS.Succeeded|path=runtime corefileVersion=v2 served=reload|Informational'
        End

        It 'reports a patch served by the fallback restart'
            rm -f "${LOCALDNS_VNET_DNS_FORWARD_SERVERS_FILE}"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'systemctl restart localdns.service'
            The contents of file "${TEST_EVENTS_FILE}" should equal 'AKS.LivePatching.localDNS.Succeeded|path=runtime corefileVersion=v2 served=restart|Informational'
        End

        It 'reports an already current patch'
            cp "${TEST_PATCH_SOURCE}" "${LOCALDNS_LIVEPATCHED_COREFILE}"
            echo "v2" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            TEST_ANC_OUTCOME="alreadyCurrent"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS update completed successfully'
            The contents of file "${TEST_EVENTS_FILE}" should equal 'AKS.LivePatching.localDNS.Succeeded|path=runtime corefileVersion=v2 served=alreadyCurrent|Informational'
        End

        It 'reports a failed patch with its message'
            TEST_RELOAD_MODE="reject"
            When call updateLocalDNS '{}'
            The status should be failure
            The output should include 'CoreDNS rejected the localDNS corefile'
            The contents of file "${TEST_EVENTS_FILE}" should equal 'AKS.LivePatching.localDNS.Failed|path=runtime CoreDNS rejected the new corefile and kept serving the previous one; reverted to the previous corefile and limits: cpu 200.8%, memory 128M|Error'
        End

        It 'reports a revoked patch'
            echo "earlier-patch" > "${LOCALDNS_LIVEPATCHED_COREFILE}"
            echo "v1" > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            render_expected "${TEST_PATCH_SOURCE}" "10.1.0.4 10.1.0.5" > "${LOCALDNS_SERVED_COREFILE}"
            record_initial_served
            TEST_ANC_OUTCOME="noCorefileData"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'CoreDNS reloaded the localDNS corefile'
            The contents of file "${TEST_EVENTS_FILE}" should equal 'AKS.LivePatching.localDNS.Revoked|path=runtime corefileVersion=v1 revoked; serving the baked corefile; served=reload|Informational'
        End

        It 'reports NoAction when there is no patch to revoke'
            TEST_ANC_OUTCOME="noCorefileData"
            TEST_RELOAD_MODE="down"
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS served corefile is already current'
            The contents of file "${TEST_EVENTS_FILE}" should equal 'AKS.LivePatching.localDNS.NoAction|path=runtime no localDNS config for this node|Informational'
        End

        It 'writes no event outside the reconciler'
            unset -f knead_emit_event
            When call updateLocalDNS '{}'
            The status should be success
            The output should include 'localDNS update completed successfully'
            The path "${TEST_EVENTS_FILE}" should not be exist
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

        # Without a version to compare, the handler runs and checks what is applied on disk.
        It "is not current when this pool's entry has no version, even if unchanged"
            When call localDNSIsCurrent '{"agentPools":{"ap1":{"localDnsProfile":{"enableLocalDns":true}}}}' \
                '{"agentPools":{"ap1":{"localDnsProfile":{"enableLocalDns":true}}}}' "${NODE_AP1}"
            The status should be failure
        End

        # A payload without agentPools has no entry for any pool -- what LPS serves when none is enabled.
        It 'is current when neither payload has agentPools'
            When call localDNSIsCurrent '{}' '{}' "${NODE_AP1}"
            The status should be success
        End

        It 'is not current when the agentPools disappear, so the handler revokes the patch'
            When call localDNSIsCurrent '{}' "{\"agentPools\":{${AP1_V1}}}" "${NODE_AP1}"
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

    Describe 'backup and restore'
        # Each file is backed up under its own name, so a restore puts back exactly the files it names,
        # whatever the order of localDNSPatchedFiles.
        prepare_backup() {
            echo 'prepared A' > "${LOCALDNS_LIVEPATCHED_COREFILE}"
            echo 'A' > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            echo 'served A' > "${LOCALDNS_SERVED_COREFILE}"
            TEST_BACKUP_DIR="${TEST_DIR}/backup"
            mkdir -p "${TEST_BACKUP_DIR}"
            localDNSBackup "${TEST_BACKUP_DIR}"
            echo 'prepared B' > "${LOCALDNS_LIVEPATCHED_COREFILE}"
            rm -f "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            echo 'served B' > "${LOCALDNS_SERVED_COREFILE}"
        }
        BeforeEach 'prepare_backup'

        It 'restores only the named files'
            When call localDNSRestore "${TEST_BACKUP_DIR}" "${LOCALDNS_LIVEPATCHED_COREFILE}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            The status should be success
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'prepared A'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'A'
            The contents of file "${LOCALDNS_SERVED_COREFILE}" should equal 'served B'
        End

        It 'restores every patched file when none is named'
            When call localDNSRestore "${TEST_BACKUP_DIR}"
            The status should be success
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'prepared A'
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should equal 'A'
            The contents of file "${LOCALDNS_SERVED_COREFILE}" should equal 'served A'
        End

        It 'removes a named file that did not exist when the backup was taken'
            rm -f "${TEST_BACKUP_DIR}/livepatched.localdns.corefile.version"
            echo 'B' > "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            When call localDNSRestore "${TEST_BACKUP_DIR}" "${LOCALDNS_LIVEPATCHED_COREFILE}.version"
            The status should be success
            The path "${LOCALDNS_LIVEPATCHED_COREFILE}.version" should not be exist
            The contents of file "${LOCALDNS_LIVEPATCHED_COREFILE}" should equal 'prepared B'
        End
    End
End
