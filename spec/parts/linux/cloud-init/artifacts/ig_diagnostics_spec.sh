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
        SYSTEMCTL_FAIL=""
        printf '1\n' > "${IG_DIAGNOSTICS_CONFIG_DIR}/diagnostics-version"
        touch "${IG_DIAGNOSTICS_CONFIG_DIR}/skip_vhd_ig"
    }
    cleanup_activation() { rm -rf "${IG_DIAGNOSTICS_CONFIG_DIR}"; }
    BeforeEach setup_activation
    AfterEach cleanup_activation
    systemctl() {
        echo "systemctl $*"
        [ "$*" != "${SYSTEMCTL_FAIL}" ]
    }
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
        The output should include 'systemctl enable ig-diagnostics-recovery.timer'
        The output should include 'systemctl start --no-block ig-diagnostics-recovery.timer'
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

    Describe 'recovery timer failures'
        Parameters
            'enable ig-diagnostics-recovery.timer'
            'start --no-block ig-diagnostics-recovery.timer'
        End

        It 'still attempts ordinary activation when the recovery timer fails'
            SYSTEMCTL_FAIL="$1"
            When call configureIGDiagnostics
            The status should be failure
            The output should include 'Failed to activate IG diagnostics recovery timer'
            The output should include 'systemctl start --no-block ig-diagnostics.service'
        End
    End

    It 'keeps recovery armed when ordinary activation fails'
        SYSTEMCTL_FAIL='start --no-block ig-diagnostics.service'
        When call configureIGDiagnostics
        The status should be failure
        The output should include 'systemctl start --no-block ig-diagnostics-recovery.timer'
        The output should include 'Failed to activate IG continuous diagnostics'
    End

    It 'arms recovery even when the daemon unit needs repair'
        SYSTEMCTL_FAIL='cat ig-diagnostics.service'
        When call configureIGDiagnostics
        The status should be failure
        The output should include 'systemctl start --no-block ig-diagnostics-recovery.timer'
        The output should include 'service is unavailable on this VHD'
    End

    Describe 'missing runtime dependencies'
        Parameters
            python3
            jq
        End

        It 'arms recovery despite missing prerequisites without activating the daemon'
            MISSING_COMMAND="$1"
            When call configureIGDiagnostics
            The status should be failure
            The output should include 'requires python3 and jq'
            The output should include 'systemctl start --no-block ig-diagnostics-recovery.timer'
            The output should not include 'systemctl enable ig-diagnostics.service'
            The output should not include 'systemctl start --no-block ig-diagnostics.service'
        End
    End
End

Describe 'IG diagnostics bounded recovery'
    Include ./parts/linux/cloud-init/artifacts/inspektor-gadget/baseline/usr/share/inspektor-gadget/diagnostics-recovery.sh

    setup_recovery() {
        TEST_DIR="$(mktemp -d)"
        export IG_DIAGNOSTICS_CONFIG_DIR="${TEST_DIR}/config"
        export IG_DIAGNOSTICS_RUNTIME_DIR="${TEST_DIR}/runtime"
        export IG_DIAGNOSTICS_RECOVERY_DIR="${TEST_DIR}/recovery"
        export IG_DIAGNOSTICS_TRACKING_FILE="${TEST_DIR}/images"
        export IG_DIAGNOSTICS_READER="${TEST_DIR}/reader.py"
        export SYS_DIR="${TEST_DIR}/sys"
        mkdir -p "${IG_DIAGNOSTICS_CONFIG_DIR}" "${IG_DIAGNOSTICS_RUNTIME_DIR}" \
            "${IG_DIAGNOSTICS_RECOVERY_DIR}" "${SYS_DIR}/fs/cgroup"
        touch "${IG_DIAGNOSTICS_CONFIG_DIR}/skip_vhd_ig" "${IG_DIAGNOSTICS_READER}" \
            "${SYS_DIR}/fs/cgroup/cgroup.controllers"
        printf '1\n' > "${IG_DIAGNOSTICS_CONFIG_DIR}/diagnostics-version"
        printf '%s\n' 'mcr.microsoft.com/oss/v2/inspektor-gadget/gadget/top_process:v0.57.0' \
            'mcr.microsoft.com/oss/v2/inspektor-gadget/gadget/top_cpu_throttle:v0.57.0' > "${IG_DIAGNOSTICS_TRACKING_FILE}"
        printf '0 1000\n' > "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget"
        TEST_CALLS="${TEST_DIR}/calls"
        touch "${TEST_CALLS}"
        TEST_SERVICE_STATE=active
        TEST_STATE_CHANGED=100000000
        TEST_NEXT_STATE=""
        TEST_NEXT_CHANGED=100000000
        TEST_STATE_ERROR=0
        TEST_READER_STATUS=0
        TEST_COLLECTOR_STATE=Running
        TEST_UPTIME=2000
        TEST_MISSING_COMMAND=""
        TEST_RESET_STATUS=0
        TEST_ACTION_STATUS=0
    }
    cleanup_recovery() { rm -rf "${TEST_DIR}"; }
    BeforeEach setup_recovery
    AfterEach cleanup_recovery

    command() {
        if [ "$1" = -v ] && [ "$2" = "${TEST_MISSING_COMMAND}" ]; then return 1; fi
        case "$*" in
            '-v ig'|'-v gadgetctl') return 0 ;;
            *) builtin command "$@" ;;
        esac
    }
    timeout() {
        printf 'timeout %s\n' "$*" >> "${TEST_CALLS}"
        shift 3
        "$@"
    }
    systemctl() {
        printf 'systemctl %s\n' "$*" >> "${TEST_CALLS}"
        case "$1" in
            show)
                [ "${TEST_STATE_ERROR}" -eq 0 ] || return "${TEST_STATE_ERROR}"
                if [ -f "${TEST_DIR}/shown" ] && [ -n "${TEST_NEXT_STATE}" ]; then
                    printf 'ActiveState=%s\nStateChangeTimestampMonotonic=%s\n' "${TEST_NEXT_STATE}" "${TEST_NEXT_CHANGED}"
                else
                    touch "${TEST_DIR}/shown"
                    printf 'ActiveState=%s\nStateChangeTimestampMonotonic=%s\n' "${TEST_SERVICE_STATE}" "${TEST_STATE_CHANGED}"
                fi
                ;;
            reset-failed)
                printf x >> "${TEST_DIR}/resets"
                return "${TEST_RESET_STATUS}"
                ;;
            --no-block) return "${TEST_ACTION_STATUS}" ;;
        esac
    }
    python3() {
        printf 'reader %s\n' "$*" >> "${TEST_CALLS}"
        [ "$1" = "${IG_DIAGNOSTICS_READER}" ] &&
            [ "$2" = --check-startup ] &&
            [ "$3" = "${IG_DIAGNOSTICS_RUNTIME_DIR}/collectors.json" ] || return 2
        if [ "${TEST_READER_STATUS}" -ne 0 ]; then
            echo 'No fresh report' >&2
        fi
        return "${TEST_READER_STATUS}"
    }
    ig_diagnostics_client() {
        printf 'client %s\n' "$*" >> "${TEST_CALLS}"
        if [ "$2" = aks-npd-memory-v1 ]; then
            case "${TEST_COLLECTOR_STATE}" in
                missing) echo "Error: instance \"$2\" not found" >&2; return 1 ;;
                timeout) echo "Error: instance \"$2\" not found" >&2; return 124 ;;
                unreachable) echo 'daemon unreachable' >&2; return 1 ;;
                malformed) echo 'unexpected output'; return 0 ;;
            esac
            printf 'ID: 0123456789abcdef0123456789abcdef\nNodeInstances:\n    - Status: %s\n' "${TEST_COLLECTOR_STATE}"
        else
            printf 'ID: 0123456789abcdef0123456789abcdef\nNodeInstances:\n    - Status: Running\n'
        fi
    }
    ig_diagnostics_recovery_now() { printf '%s\n' "${TEST_UPTIME}"; }

    It 'requires all three fresh reports and preserves the consumed retry budget'
        printf '2 1000\n' > "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget"
        When call ig_diagnostics_recover
        The status should be success
        The output should include 'All three collectors have fresh reports'
        The contents of file "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget" should equal '2 0'
        The contents of file "${TEST_CALLS}" should include "-k 1s 35s python3 ${IG_DIAGNOSTICS_READER} --check-startup ${IG_DIAGNOSTICS_RUNTIME_DIR}/collectors.json"
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
    End

    Describe 'service transitions'
        Parameters
            activating
            deactivating
            reloading
        End
        It 'does not probe or interrupt an ongoing transition'
            TEST_SERVICE_STATE="$1"
            When call ig_diagnostics_recover
            The status should be success
            The output should include 'not interrupting it'
            The contents of file "${TEST_CALLS}" should not include 'reader '
            The contents of file "${TEST_CALLS}" should not include 'reset-failed'
        End
    End

    Describe 'stopped services'
        Parameters
            inactive
            failed
        End
        It 'enables and starts the daemon only after cooldown, reserving a recovery batch'
            TEST_SERVICE_STATE="$1"
            When call ig_diagnostics_recover
            The status should be success
            The output should include 'Requested recovery batch 1/2'
            The contents of file "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget" should equal '1 2000'
            The contents of file "${TEST_CALLS}" should include 'systemctl enable ig-diagnostics.service'
            The contents of file "${TEST_CALLS}" should include 'systemctl reset-failed ig-diagnostics.service'
            The contents of file "${TEST_CALLS}" should include 'systemctl --no-block --job-mode=fail start ig-diagnostics.service'
        End
    End

    Describe 'confirmed collector failures'
        Parameters
            missing
            Error
        End
        It 'recovers an active daemon only with positive collector failure evidence'
            TEST_READER_STATUS=1
            TEST_COLLECTOR_STATE="$1"
            When call ig_diagnostics_recover
            The status should be success
            The stderr should include 'No fresh report'
            The output should include 'Requested recovery batch 1/2'
            The contents of file "${TEST_CALLS}" should include 'client show aks-npd-memory-v1'
            The contents of file "${TEST_CALLS}" should include 'systemctl --no-block --job-mode=fail restart ig-diagnostics.service'
        End
    End

    It 'does not restart merely because reports are stale while collectors are running'
        TEST_READER_STATUS=1
        When call ig_diagnostics_recover
        The status should be failure
        The stderr should include 'not restarting on staleness alone'
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
        The contents of file "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget" should equal '0 1000'
    End

    Describe 'unconfirmed collector states'
        Parameters
            timeout
            unreachable
            malformed
            Starting
            Unknown
            Stopped
        End
        It 'does not mistake uncertainty or client timeouts for a failed collector'
            TEST_READER_STATUS=1
            TEST_COLLECTOR_STATE="$1"
            When call ig_diagnostics_recover
            The status should be failure
            The stderr should include 'No fresh report'
            The contents of file "${TEST_CALLS}" should not include 'reset-failed'
        End
    End

    It 'starts a full cooldown on the first failure without resetting the start limit'
        TEST_SERVICE_STATE=failed
        rm "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget"
        When call ig_diagnostics_recover
        The status should be success
        The output should include 'deferred for a 15-minute cooldown'
        The contents of file "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget" should equal '0 2000'
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
    End

    It 'does not recover before the full cooldown has elapsed'
        TEST_SERVICE_STATE=failed
        TEST_UPTIME=1899
        When call ig_diagnostics_recover
        The status should be success
        The output should include 'cooldown has not elapsed'
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
    End

    It 'does not restart when service state changes during the health check'
        TEST_READER_STATUS=1
        TEST_COLLECTOR_STATE=missing
        TEST_NEXT_STATE=activating
        When call ig_diagnostics_recover
        The status should be success
        The stderr should include 'No fresh report'
        The output should include 'Service state changed'
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
    End

    It 'detects an intervening restart even when ActiveState returns to active'
        TEST_READER_STATUS=1
        TEST_COLLECTOR_STATE=missing
        TEST_NEXT_STATE=active
        TEST_NEXT_CHANGED=1500000000
        When call ig_diagnostics_recover
        The status should be success
        The stderr should include 'No fresh report'
        The output should include 'Service state changed'
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
    End

    It 'preserves a recent start-rate window even when a previous failure cooldown expired'
        TEST_SERVICE_STATE=failed
        TEST_STATE_CHANGED=1800000000
        When call ig_diagnostics_recover
        The status should be success
        The output should include 'preserving its start-rate limit'
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
    End

    It 'keeps a failed reset attempt charged against the recovery budget'
        TEST_SERVICE_STATE=failed
        TEST_RESET_STATUS=1
        When call ig_diagnostics_recover
        The status should be failure
        The stderr should include 'Recovery batch 1/2 failed'
        The contents of file "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget" should equal '1 2000'
        The contents of file "${TEST_CALLS}" should not include '--job-mode=fail'
    End

    It 'does not replace conflicting systemd jobs and retains the reserved batch on refusal'
        TEST_SERVICE_STATE=failed
        TEST_ACTION_STATUS=1
        When call ig_diagnostics_recover
        The status should be failure
        The stderr should include 'Recovery batch 1/2 failed'
        The contents of file "${TEST_CALLS}" should include '--no-block --job-mode=fail start'
        The contents of file "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget" should equal '1 2000'
    End

    exercise_budget() {
        TEST_SERVICE_STATE=failed
        rm "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget"
        TEST_UPTIME=1000
        ig_diagnostics_recover || return
        TEST_UPTIME=1900
        ig_diagnostics_recover || return
        TEST_UPTIME=2200
        ig_diagnostics_recover || return
        TEST_UPTIME=2800
        ig_diagnostics_recover || return
        TEST_UPTIME=3700
        ig_diagnostics_recover
    }
    It 'permits only two additional recovery batches across repeated timer invocations'
        When call exercise_budget
        The status should be failure
        The output should include 'Recovery cooldown has not elapsed'
        The stderr should include 'Recovery budget exhausted'
        The contents of file "${TEST_DIR}/resets" should equal 'xx'
        The contents of file "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget" should equal '2 2800'
    End

    Describe 'invalid persisted budgets'
        Parameters
            '9 1000'
            '0 1+1'
            '0 1000 extra'
        End
        It 'never silently resets a malformed recovery budget'
            TEST_SERVICE_STATE=failed
            printf '%s\n' "$1" > "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget"
            When call ig_diagnostics_recover
            The status should be failure
            The stderr should include 'Invalid recovery budget'
            The contents of file "${TEST_CALLS}" should not include 'reset-failed'
        End
    End

    It 'does not act when systemd cannot report a known state'
        TEST_STATE_ERROR=124
        When call ig_diagnostics_recover
        The status should be failure
        The stderr should include 'Cannot determine service state'
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
    End

    It 'does not act on an unrecognized service state'
        TEST_SERVICE_STATE=unknown
        When call ig_diagnostics_recover
        The status should be failure
        The stderr should include 'Unknown service state'
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
    End

    It 'does not spend a recovery batch when an imported image is missing'
        TEST_SERVICE_STATE=failed
        printf '' > "${IG_DIAGNOSTICS_TRACKING_FILE}"
        When call ig_diagnostics_recover
        The status should be failure
        The stderr should include 'Recovery prerequisites are incomplete'
        The contents of file "${TEST_CALLS}" should not include 'reset-failed'
        The contents of file "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget" should equal '0 1000'
    End

    It 'reports a missing reader without enabling or resetting the daemon'
        rm "${IG_DIAGNOSTICS_READER}"
        When call ig_diagnostics_recover
        The status should be failure
        The stderr should include 'requires the packaged report reader'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    recover_after_prerequisite_repair() {
        TEST_SERVICE_STATE=failed
        TEST_MISSING_COMMAND=python3
        if ig_diagnostics_recover; then return 1; fi
        TEST_MISSING_COMMAND=""
        ig_diagnostics_recover
    }
    It 'can enable and start a previously disabled daemon after its prerequisites are repaired'
        When call recover_after_prerequisite_repair
        The status should be success
        The stderr should include 'Recovery requires python3'
        The output should include 'Requested recovery batch 1/2'
        The contents of file "${TEST_CALLS}" should include 'systemctl enable ig-diagnostics.service'
        The contents of file "${IG_DIAGNOSTICS_RECOVERY_DIR}/budget" should equal '1 2000'
    End

    It 'leaves old images on their existing collection path'
        rm "${IG_DIAGNOSTICS_CONFIG_DIR}/diagnostics-version"
        When call ig_diagnostics_recover
        The status should be success
        The output should include 'unsupported; not recovering'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    recover_with_lock_held() {
        exec 8>"${IG_DIAGNOSTICS_RECOVERY_DIR}/lock"
        flock 8
        ig_diagnostics_recover
        exec 8>&-
    }
    It 'does not race another recovery check against the per-boot budget'
        When call recover_with_lock_held
        The status should be success
        The output should include 'Another recovery check is running'
        The contents of file "${TEST_CALLS}" should equal ''
    End
End
