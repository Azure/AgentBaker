#!/bin/bash
# shellcheck disable=SC2034,SC2317

Describe 'localdns-limits.sh'
    setup() {
        TEST_DIR="/tmp/localdns-limits-test"
        rm -rf "${TEST_DIR}"
        mkdir -p "${TEST_DIR}"
        Include ./parts/linux/cloud-init/artifacts/localdns-limits.sh
        Include ./spec/parts/linux/cloud-init/artifacts/localdns_limits_fakes.sh
        # The provisioning limits of the e2e node config: 2008 millicores and 128 MB.
        setup_fake_localdns_slice "${TEST_DIR}" "200.8%" "128M"
        TEST_READY_BODY="OK"
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    systemctl() {
        fake_localdns_systemctl "$@"
    }

    curl() {
        echo "${TEST_READY_BODY}"
    }

    saved_cpu() {
        sed -n 's/^CPUQuota=//p' "${LOCALDNS_LIMITS_SAVED_DIR}/50-CPUQuota.conf"
    }

    saved_memory() {
        sed -n 's/^MemoryMax=//p' "${LOCALDNS_LIMITS_SAVED_DIR}/50-MemoryMax.conf"
    }

    Describe 'value parsing and formatting'
        Parameters
            "150.0%" 15000
            "150%" 15000
            "150.5%" 15050
            "33.33%" 3333
            "" max
            infinity max
        End
        It "parses CPUQuota=$1"
            When call localdns_limits_parse_cpu "$1"
            The output should equal "$2"
        End
    End

    Describe 'memory parsing'
        Parameters
            "512M" 536870912
            "536870912" 536870912
            "1G" 1073741824
            "64K" 65536
            infinity max
        End
        It "parses MemoryMax=$1"
            When call localdns_limits_parse_memory "$1"
            The output should equal "$2"
        End
    End

    Describe 'invalid values'
        It 'rejects a CPUQuota without a percent sign'
            When call localdns_limits_parse_cpu "150"
            The status should be failure
        End

        It 'rejects a percentage MemoryMax'
            When call localdns_limits_parse_memory "50%"
            The status should be failure
        End
    End

    Describe 'formatting'
        Parameters
            cpu 15000 "150%"
            cpu 15050 "150.5%"
            cpu 3333 "33.33%"
            cpu max unlimited
            memory 536870912 "512M"
            memory 1000 "1000 bytes"
            memory max unlimited
        End
        It "formats $1 $2"
            When call localdns_limits_format "$1" "$2"
            The output should equal "$3"
        End
    End

    Describe 'localdns_limits_preflight'
        It 'prints the baked limits and the running ones'
            fake_localdns_apply_live "CPUQuota=400%"
            When call localdns_limits_preflight
            The status should be success
            The output should equal "20080 134217728 40000 134217728"
        End

        It 'reports a node without localdns.slice separately'
            rm -f "${LOCALDNS_LIMITS_BAKED_SLICE_FILE}"
            When call localdns_limits_preflight
            The status should equal 2
            The output should equal "cannot read the baked limits from ${LOCALDNS_LIMITS_BAKED_SLICE_FILE}; limits and corefile left unchanged"
        End

        It 'fails when systemctl does not respond'
            FAKE_SYSTEMCTL_DOWN=true
            When call localdns_limits_preflight
            The status should equal 1
            The output should equal "systemctl did not respond; limits and corefile left unchanged"
        End

        It 'fails when the baked limits do not parse'
            sed -i 's/^CPUQuota=.*/CPUQuota=lots/' "${LOCALDNS_LIMITS_BAKED_SLICE_FILE}"
            When call localdns_limits_preflight
            The status should equal 1
            The output should equal "cannot read the baked limits from ${LOCALDNS_LIMITS_BAKED_SLICE_FILE}; limits and corefile left unchanged"
        End

        It 'fails when a baked limit is missing'
            sed -i '/^MemoryMax=/d' "${LOCALDNS_LIMITS_BAKED_SLICE_FILE}"
            When call localdns_limits_preflight
            The status should equal 1
            The output should include "cannot read the baked limits"
        End

        It 'fails when the running limits cannot be read'
            rm -f "${LOCALDNS_LIMITS_CGROUP_DIR}/memory.max"
            When call localdns_limits_preflight
            The status should equal 1
            The output should equal "cannot read the running limits from ${LOCALDNS_LIMITS_CGROUP_DIR}; limits and corefile left unchanged"
        End

        It 'records the saved limits as the previous ones when the slice is not running'
            fake_localdns_save "CPUQuota=300%"
            rm -rf "${LOCALDNS_LIMITS_CGROUP_DIR}"
            When call localdns_limits_preflight
            The status should be success
            The output should equal "20080 134217728 30000 134217728"
        End
    End

    Describe 'localdns_limits_apply'
        It 'sets both limits and saves them'
            When call localdns_limits_apply 30000 536870912 20080 134217728
            The status should be success
            The stderr should include "systemctl set-property localdns.slice CPUQuota=300%"
            The stderr should include "systemctl set-property localdns.slice MemoryMax=536870912"
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal "300000 100000"
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/memory.max" should equal "536870912"
            The result of function saved_cpu should equal "300%"
            The result of function saved_memory should equal "536870912"
        End

        It 'sets CPU before memory, one property per call'
            localdns_limits_apply 30000 536870912 20080 134217728 2> /dev/null
            When call fake_set_property_calls
            The line 1 of output should equal "CPUQuota=300%"
            The line 2 of output should equal "MemoryMax=536870912"
            The lines of output should equal 2
        End

        It 'skips limits already in effect'
            When call localdns_limits_apply 20080 134217728 20080 134217728
            The status should be success
            The stderr should include "already in effect"
            The contents of file "${FAKE_SET_PROPERTY_LOG}" should equal ""
        End

        It 'sets only the limit that differs'
            localdns_limits_apply 20080 536870912 20080 134217728 2> /dev/null
            When call fake_set_property_calls
            The output should equal "MemoryMax=536870912"
        End

        It 'accepts a fractional CPU limit saved truncated to a whole percent'
            When call localdns_limits_apply 15050 134217728 20080 134217728
            The status should be success
            The stderr should include "CPUQuota=150.5%"
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal "150500 100000"
            The result of function saved_cpu should equal "150%"
        End

        It 'treats a limit running and saved for the next boot as in effect'
            localdns_limits_apply 15050 134217728 20080 134217728 2> /dev/null
            : > "${FAKE_SET_PROPERTY_LOG}"
            When call localdns_limits_apply 15050 134217728 20080 134217728
            The status should be success
            The stderr should include "already in effect"
            The contents of file "${FAKE_SET_PROPERTY_LOG}" should equal ""
        End

        It 'reapplies a limit that runs but would not survive a reboot'
            # e.g. an earlier set-property that could not write its drop-in.
            fake_localdns_apply_live "CPUQuota=300%"
            When call localdns_limits_apply 30000 134217728 20080 134217728
            The status should be success
            The stderr should include "CPUQuota=300%"
            The result of function saved_cpu should equal "300%"
        End

        It 'reports which limit failed and why when set-property fails'
            FAKE_SET_PROPERTY_FAIL=MemoryMax
            When call localdns_limits_apply 30000 536870912 20080 134217728
            The status should be failure
            The output should equal "the memory limit failed (systemctl set-property MemoryMax=536870912 failed: Failed to set unit properties on localdns.slice: Access denied)"
            The stderr should include "CPUQuota=300%"
        End

        It 'catches set-property exiting 0 without saving the drop-in'
            FAKE_SET_PROPERTY_UNSAVED=CPUQuota
            When call localdns_limits_apply 30000 134217728 20080 134217728
            The status should be failure
            The output should equal "the cpu limit failed (applied but not saved: live 300%, saved 200.8%)"
            The stderr should include "CPUQuota=300%"
        End

        It 'reports both limits when neither was saved'
            FAKE_SET_PROPERTY_UNSAVED=all
            When call localdns_limits_apply 30000 536870912 20080 134217728
            The status should be failure
            The output should equal "the cpu and memory limits failed (cpu applied but not saved: live 300%, saved 200.8%; memory applied but not saved: live 512M, saved 128M)"
            The stderr should include "MemoryMax=536870912"
        End

        It 'saves the limits when the slice is not running'
            rm -rf "${LOCALDNS_LIMITS_CGROUP_DIR}"
            # Like systemd, the fake only touches the cgroup if it exists.
            fake_localdns_apply_live() { :; }
            When call localdns_limits_apply 30000 536870912 20080 134217728
            The status should be success
            The stderr should include "MemoryMax=536870912"
            The result of function saved_cpu should equal "300%"
        End

        It 'cuts a long reason to 200 characters'
            systemctl() {
                case "$1" in
                    set-property) printf 'x%.0s' $(seq 1 400); return 1 ;;
                esac
            }
            When call localdns_limits_apply 30000 134217728 20080 134217728
            The status should be failure
            The length of output should equal 223
            The output should start with "the cpu limit failed (systemctl set-property CPUQuota=300% failed: xxx"
            The stderr should include "CPUQuota=300%"
        End
    End

    Describe 'localdns_limits_revert'
        It 'sets the limits back and prints them'
            localdns_limits_apply 30000 536870912 20080 134217728 2> /dev/null
            When call localdns_limits_revert 40000 134217728 20080 134217728
            The status should be success
            The output should equal "cpu 400%, memory 128M"
            The stderr should include "CPUQuota=400%"
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal "400000 100000"
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/memory.max" should equal "134217728"
        End

        It 'resets to the baked limits'
            localdns_limits_apply 30000 536870912 20080 134217728 2> /dev/null
            When call localdns_limits_revert 20080 134217728 20080 134217728
            The status should be success
            The output should equal "cpu 200.8%, memory 128M"
            The stderr should include "MemoryMax=134217728"
            The contents of file "${LOCALDNS_LIMITS_CGROUP_DIR}/cpu.max" should equal "200800 100000"
            The result of function saved_memory should equal "134217728"
        End

        It 'removes the drop-ins and reports every layer when it fails'
            localdns_limits_apply 30000 536870912 20080 134217728 2> /dev/null
            FAKE_SET_PROPERTY_FAIL=all
            When call localdns_limits_revert 20080 134217728 20080 134217728
            The status should be failure
            The output should equal "in effect: cpu 300%, memory 512M; saved: removed, baked applies after a reboot; baked: cpu 200.8%, memory 128M"
            The stderr should include "reverting the localdns.slice limits failed: the cpu limit failed"
            The path "${LOCALDNS_LIMITS_SAVED_DIR}/50-CPUQuota.conf" should not be exist
            The path "${LOCALDNS_LIMITS_SAVED_DIR}/50-MemoryMax.conf" should not be exist
        End
    End

    Describe 'localdns_limits_from_anc_output'
        It 'reads the version and both limits'
            When call localdns_limits_from_anc_output "$(printf 'version: v2\ncpuQuota: 300.0%%\nmemoryMax: 512M\nrendered')" 20080 134217728
            The output should equal "v2 30000 536870912"
        End

        It 'uses the baked value for a limit the profile leaves out'
            When call localdns_limits_from_anc_output "$(printf 'version: v2\nmemoryMax: 512M\nalreadyCurrent')" 20080 134217728
            The output should equal "v2 20080 536870912"
        End

        It 'uses the baked values when the profile sets no limit'
            When call localdns_limits_from_anc_output "$(printf 'version: v2\nrendered')" 20080 134217728
            The output should equal "v2 20080 134217728"
        End

        It 'fails without a version'
            When call localdns_limits_from_anc_output "rendered" 20080 134217728
            The status should be failure
            The output should equal "aks-node-controller printed no corefileVersion"
        End

        It 'fails on a limit it cannot parse'
            When call localdns_limits_from_anc_output "$(printf 'version: v2\ncpuQuota: lots\nrendered')" 20080 134217728
            The status should be failure
            The output should equal "cannot parse the cpu limit lots from aks-node-controller"
        End
    End

    Describe 'localdns_limits_failure_message'
        It 'reports a reverted failure'
            When call localdns_limits_failure_message "live patching the memory limit failed (boom)" true "cpu 400%, memory 2048M"
            The output should equal "live patching the memory limit failed (boom); reverted to the previous corefile and limits: cpu 400%, memory 2048M"
        End

        It 'reports a failed revert'
            When call localdns_limits_failure_message "live patching the cpu limit failed (boom)" false "in effect: cpu 300%, memory 1024M; saved: removed, baked applies after a reboot; baked: cpu 200%, memory 1024M"
            The output should equal "live patching the cpu limit failed (boom); previous corefile restored; reverting to the previous limits also failed; in effect: cpu 300%, memory 1024M; saved: removed, baked applies after a reboot; baked: cpu 200%, memory 1024M"
        End

        It 'caps the message at 512 characters'
            When call localdns_limits_failure_message "$(printf 'y%.0s' $(seq 1 600))" true "cpu 400%, memory 2048M"
            The length of output should equal 512
        End
    End

    Describe 'localdns_write_corefile_version'
        It 'writes the version'
            When call localdns_write_corefile_version "${TEST_DIR}/corefile.version" v2
            The status should be success
            The contents of file "${TEST_DIR}/corefile.version" should equal "v2"
        End

        It 'fails when the directory does not exist'
            When call localdns_write_corefile_version "${TEST_DIR}/missing/corefile.version" v2
            The status should be failure
            The stderr should be present
        End
    End

    Describe 'readiness'
        It 'asks the node listener ready endpoint past any proxy'
            curl() {
                echo "$*" > "${TEST_DIR}/curl.args"
                echo OK
            }
            When call localdns_ready
            The status should be success
            The contents of file "${TEST_DIR}/curl.args" should equal "-s --noproxy 169.254.10.10 --connect-timeout 5 --max-time 10 http://169.254.10.10:8181/ready"
        End

        It 'is not ready when CoreDNS answers anything but OK'
            TEST_READY_BODY="NOTOK"
            When call localdns_ready
            The status should be failure
        End

        It 'waits for CoreDNS to report ready'
            When call wait_for_localdns_ready 100 5
            The status should be success
            The output should include "Waiting for localdns to start and be able to serve traffic."
            The output should include "Localdns is online and ready to serve traffic."
        End

        It 'gives up when CoreDNS is not ready in time'
            TEST_READY_BODY="NOTOK"
            When call wait_for_localdns_ready 1000 2
            The status should be failure
            The output should include "Localdns failed to come online after 2 seconds (timeout)."
        End

        It 'gives up when CoreDNS is not ready after max attempts'
            TEST_READY_BODY="NOTOK"
            When call wait_for_localdns_ready 2 50
            The status should be failure
            The output should include "Localdns failed to come online after 2 attempts."
        End
    End

    Describe 'served Corefile'
        write_source_corefile() {
            cat > "${TEST_DIR}/source.corefile" <<'EOF'
.:53 {
    bind 169.254.10.10
    forward . 168.63.129.16 {
    }
}
cluster.local:53 {
    bind 169.254.10.11
    forward . 10.0.0.10 {
    }
}
EOF
        }
        BeforeEach 'write_source_corefile'

        It 'replaces the Azure DNS IP with the VNet DNS servers'
            When call localdns_render_served_corefile "${TEST_DIR}/source.corefile" "${TEST_DIR}/served.corefile" "10.1.0.4 10.1.0.5"
            The status should be success
            The contents of file "${TEST_DIR}/served.corefile" should include 'forward . 10.1.0.4 10.1.0.5 {'
            The contents of file "${TEST_DIR}/served.corefile" should include 'forward . 10.0.0.10 {'
            The contents of file "${TEST_DIR}/served.corefile" should not include '168.63.129.16'
        End

        It 'serves the source as is on an Azure DNS VNet'
            When call localdns_render_served_corefile "${TEST_DIR}/source.corefile" "${TEST_DIR}/served.corefile" "168.63.129.16"
            The status should be success
            The contents of file "${TEST_DIR}/served.corefile" should equal "$(cat "${TEST_DIR}/source.corefile")"
        End

        It 'exports one forward_info line per forward, by listener and block'
            When call localdns_export_forward_ips "${TEST_DIR}/source.corefile" "${TEST_DIR}/forward_ips.prom"
            The status should be success
            The output should include "Successfully exported forward IPs to ${TEST_DIR}/forward_ips.prom"
            The contents of file "${TEST_DIR}/forward_ips.prom" should include 'localdns_vnetdns_forward_info{ip="168.63.129.16",block=".:53",status="ok"} 1'
            The contents of file "${TEST_DIR}/forward_ips.prom" should include 'localdns_kubedns_forward_info{ip="10.0.0.10",block="cluster.local:53",status="ok"} 1'
        End

        It 'reports a missing forward for a listener with none'
            sed -i '/forward \. 10\.0\.0\.10/d' "${TEST_DIR}/source.corefile"
            When call localdns_export_forward_ips "${TEST_DIR}/source.corefile" "${TEST_DIR}/forward_ips.prom"
            The status should be success
            The output should include 'Successfully exported forward IPs'
            The contents of file "${TEST_DIR}/forward_ips.prom" should include 'localdns_kubedns_forward_info{ip="unknown",block="none",status="missing"} 0'
        End

        It 'never fails the caller when the metrics cannot be written'
            When call localdns_export_forward_ips "${TEST_DIR}/source.corefile" "${TEST_DIR}/missing-dir/forward_ips.prom"
            The status should be success
            The output should include 'Metrics export skipped'
            The stderr should be present
        End
    End
End
