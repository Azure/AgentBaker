#!/bin/bash

Describe 'IG diagnostic collector lifecycle'
    Include ./parts/linux/cloud-init/artifacts/inspektor-gadget/baseline/usr/share/inspektor-gadget/diagnostics.sh
    setup_collector() {
        TEST_DIR="$(mktemp -d)"
        export IG_DIAGNOSTICS_RUNTIME_DIR="${TEST_DIR}"
        export IG_DIAGNOSTICS_TRACKING_FILE="${TEST_DIR}/images"
        export TEST_CALLS="${TEST_DIR}/calls"
        printf '%s\n' 'mcr.microsoft.com/oss/v2/inspektor-gadget/gadget/top_process:v0.57.0' \
            'mcr.microsoft.com/oss/v2/inspektor-gadget/gadget/top_cpu_throttle:v0.57.0' > "${IG_DIAGNOSTICS_TRACKING_FILE}"
        export COLLECTOR_EXISTS=false CLIENT_FAIL=false COLLECTOR_STATUS=Running
        export COLLECTOR_STARTING=false
        export REPORTS_READY=true
    }
    cleanup_collector() { rm -rf "${TEST_DIR}"; }
    BeforeEach setup_collector
    AfterEach cleanup_collector

    ig_diagnostics_client() {
        printf '%s\n' "$*" >> "${TEST_CALLS}"
        [ "${CLIENT_FAIL}" != true ] || { echo 'daemon unreachable' >&2; return 1; }
        case "$1" in
            list) return 0 ;;
            show)
                if [ "${COLLECTOR_EXISTS}" = true ] || [ -f "${TEST_DIR}/$2" ]; then
                    if [ "${COLLECTOR_STARTING}" = true ] && [ ! -f "${TEST_DIR}/polled" ]; then
                        touch "${TEST_DIR}/polled"
                        printf 'ID: 0123456789abcdef0123456789abcdef\nNodeInstances: []\n'
                    else
                        printf 'ID: 0123456789abcdef0123456789abcdef\nNodeInstances:\n    - Status: %s\n' "${COLLECTOR_STATUS}"
                    fi
                else
                    echo "Error: instance \"$2\" not found" >&2
                    return 1
                fi
                ;;
            run)
                while [ "$1" != --name ]; do shift; done
                touch "${TEST_DIR}/$2"
                ;;
            delete) rm -f "${TEST_DIR}/$2" ;;
        esac
    }
    sleep() { :; }
    timeout() {
        printf '%s\n' "$*" >> "${TEST_CALLS}"
        [ ! -e "${TEST_DIR}/collectors.json" ] || return 1
        [ -s "${7}" ] || return 1
        [ "${REPORTS_READY}" = true ]
    }

    It 'starts CPU, memory and throttle collectors with no pulls and bounded arrays'
        When call ig_diagnostics_start
        The status should be success
        The output should include 'Continuous collectors started'
        The contents of file "${TEST_CALLS}" should include '--pull never --interval 5s --count 0 --sort -cpuRelative --max-entries 15'
        The contents of file "${TEST_CALLS}" should include '--sort -memRelative --max-entries 15'
        The contents of file "${TEST_CALLS}" should include '--sort -throttleRatio --max-entries 15'
        The contents of file "${TEST_CALLS}" should include '-k 1s 35s python3 /etc/node-problem-detector.d/plugin/ig_report_reader.py --check-startup '
        The contents of file "${TEST_DIR}/collectors.json" should include '"version": 1'
    End

    It 'replaces only exact owned names rather than parsing truncated list output'
        COLLECTOR_EXISTS=true
        When call ig_diagnostics_collector aks-npd-cpu-v1 test-image -cpuRelative
        The status should be success
        The output should equal 0123456789abcdef0123456789abcdef
        The contents of file "${TEST_CALLS}" should include 'delete aks-npd-cpu-v1'
    End

    It 'does not treat a failed show request as a missing collector'
        CLIENT_FAIL=true
        When call ig_diagnostics_collector aks-npd-cpu-v1 test-image -cpuRelative
        The status should be failure
        The stderr should include 'Cannot inspect'
        The contents of file "${TEST_CALLS}" should not include 'run '
    End

    It 'fails closed when a required image was not imported'
        printf 'old readiness\n' > "${TEST_DIR}/collectors.json"
        printf '' > "${IG_DIAGNOSTICS_TRACKING_FILE}"
        When call ig_diagnostics_start
        The status should be failure
        The stderr should include 'No imported image'
        The path "${TEST_DIR}/collectors.json" should not be exist
    End

    It 'waits for a detached collector to reach Running'
        COLLECTOR_STARTING=true
        When call ig_diagnostics_collector aks-npd-cpu-v1 test-image -cpuRelative
        The status should be success
        The output should equal 0123456789abcdef0123456789abcdef
        The stderr should include 'Waiting for aks-npd-cpu-v1'
    End

    It 'does not publish readiness for a failed collector'
        COLLECTOR_STATUS=Error
        When call ig_diagnostics_start
        The status should be failure
        The stderr should include 'is not running'
        The path "${TEST_DIR}/collectors.json" should not be exist
    End

    It 'does not trust Running when usable reports never arrive'
        REPORTS_READY=false
        printf 'old readiness\n' > "${TEST_DIR}/collectors.json"
        When call ig_diagnostics_start
        The status should be failure
        The stderr should include 'did not produce usable startup reports'
        The path "${TEST_DIR}/collectors.json" should not be exist
    End
End

Describe 'CSE default IG activation'
    Include ./parts/linux/cloud-init/artifacts/cse_helpers.sh
    Include ./parts/linux/cloud-init/artifacts/cse_config.sh
    setup_activation() {
        IG_DIAGNOSTICS_CONFIG_DIR="$(mktemp -d)"
        SYS_DIR="${IG_DIAGNOSTICS_CONFIG_DIR}/sys"
        mkdir -p "${SYS_DIR}/fs/cgroup"
        touch "${SYS_DIR}/fs/cgroup/cgroup.controllers"
        OS=UBUNTU
        OS_VARIANT=""
        IS_KATA=false
        MISSING_COMMAND=""
        printf '1\n' > "${IG_DIAGNOSTICS_CONFIG_DIR}/diagnostics-version"
        touch "${IG_DIAGNOSTICS_CONFIG_DIR}/skip_vhd_ig"
    }
    cleanup_activation() { rm -rf "${IG_DIAGNOSTICS_CONFIG_DIR}"; }
    BeforeEach setup_activation
    AfterEach cleanup_activation
    systemctl() { echo "systemctl $*"; }
    timeout() { shift 3; "$@"; }
    command() {
        case "$*" in
            '-v python3'|'-v jq') [ "$2" != "${MISSING_COMMAND}" ] ;;
            *) builtin command "$@" ;;
        esac
    }

    It 'enables the service by default without waiting for a report'
        When call configureIGDiagnostics
        The status should be success
        The output should include 'systemctl enable ig-diagnostics.service'
        The output should include 'systemctl start --no-block ig-diagnostics.service'
    End

    It 'keeps legacy collection on an older image'
        rm "${IG_DIAGNOSTICS_CONFIG_DIR}/diagnostics-version"
        When call configureIGDiagnostics
        The status should be success
        The output should include 'unavailable on this VHD'
        The output should not include 'systemctl enable'
    End

    It 'rejects unsupported capability versions'
        printf '2\n' > "${IG_DIAGNOSTICS_CONFIG_DIR}/diagnostics-version"
        When call configureIGDiagnostics
        The status should be failure
        The output should include 'unavailable on this VHD'
    End

    It 'does not activate the feature on Kata'
        IS_KATA=true
        When call configureIGDiagnostics
        The status should be success
        The output should equal ''
    End

    It 'keeps the legacy path on cgroup v1 nodes'
        rm "${SYS_DIR}/fs/cgroup/cgroup.controllers"
        When call configureIGDiagnostics
        The status should be success
        The output should include 'keeping legacy collection'
    End

    It 'reports a bounded activation failure for the nonfatal CSE caller'
        timeout() { return 1; }
        When call configureIGDiagnostics
        The status should be failure
        The output should include 'Failed to activate'
    End

    Describe 'missing runtime dependencies'
        Parameters
            python3
            jq
        End

        It 'reports missing prerequisites without activating the service'
            MISSING_COMMAND="$1"
            When call configureIGDiagnostics
            The status should be failure
            The output should include 'requires python3 and jq'
            The output should not include 'systemctl enable'
            The output should not include 'systemctl start'
        End
    End
End
