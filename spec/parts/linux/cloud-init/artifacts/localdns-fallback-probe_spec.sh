#!/bin/bash

Describe 'localdns-fallback-probe.sh'
# Tests the pod-DNS fallback probe debounce state machine defined in
# parts/linux/cloud-init/artifacts/localdns-fallback-probe.sh.
#------------------------------------------------------------------------------

    Describe 'main (debounce + trigger)'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns-fallback-probe.sh"
            STATE_DIR=$(mktemp -d)
            FAIL_COUNTER_FILE="${STATE_DIR}/consecutive_fails"
            LOCALDNS_FALLBACK_PROBE_FAIL_THRESHOLD=3
            START_LOG="${STATE_DIR}/start.log"
            : > "${START_LOG}"

            # DNS_UP controls whether .11 answers.
            dig() {
                [ "${DNS_UP:-0}" = "1" ] && return 0 || return 9
            }
            # FALLBACK_ACTIVE controls is-active; record start invocations.
            systemctl() {
                if [ "$1" = "is-active" ]; then
                    [ "${FALLBACK_ACTIVE:-0}" = "1" ] && return 0 || return 3
                fi
                if [ "$1" = "start" ]; then
                    echo "start $*" >> "${START_LOG}"
                    return 0
                fi
                return 0
            }
        }
        cleanup() { rm -rf "${STATE_DIR}"; }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'does not start the fallback before the debounce threshold'
            DNS_UP=0 FALLBACK_ACTIVE=0

            When call main
            The status should be success
            The stdout should include "1/3"
            The contents of file "${FAIL_COUNTER_FILE}" should equal "1"
            The path "${START_LOG}" should be empty file
        End

        It 'starts the fallback once .11 is dark for the threshold'
            DNS_UP=0 FALLBACK_ACTIVE=0
            printf '2' > "${FAIL_COUNTER_FILE}"   # already 2 consecutive fails

            When call main
            The status should be success
            The stdout should include "starting localdns-fallback.service"
            The contents of file "${START_LOG}" should include "start --no-block localdns-fallback.service"
        End

        It 'resets the counter when .11 answers again'
            DNS_UP=1 FALLBACK_ACTIVE=1
            printf '2' > "${FAIL_COUNTER_FILE}"

            When call main
            The status should be success
            The stdout should include "resetting debounce counter"
            The contents of file "${FAIL_COUNTER_FILE}" should equal "0"
            The path "${START_LOG}" should be empty file
        End

        It 'does not restart the fallback when it is already active but .11 still dark'
            DNS_UP=0 FALLBACK_ACTIVE=1
            printf '5' > "${FAIL_COUNTER_FILE}"   # well past threshold

            When call main
            The status should be success
            The stdout should include "already active but .11 still dark"
            The path "${START_LOG}" should be empty file
        End

        It 'treats a missing counter file as zero'
            DNS_UP=0 FALLBACK_ACTIVE=0
            rm -f "${FAIL_COUNTER_FILE}"

            When call main
            The status should be success
            The stdout should include "1/3"
            The contents of file "${FAIL_COUNTER_FILE}" should equal "1"
        End
    End
End
