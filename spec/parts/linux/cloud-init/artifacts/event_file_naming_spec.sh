#!/bin/bash

Describe 'WALinuxAgent event file naming'
    ARTIFACTS_DIR="./parts/linux/cloud-init/artifacts"
    COMPLIANT_NAME='="\$\(date \+%s%N\)\$\{BASHPID\}(\.json)?"$'

    emitters() {
        grep -rlE 'EVENTS_LOGGING_(DIR|PATH)|events_logging_dir' "${ARTIFACTS_DIR}"/*.sh | sort
    }

    emitters_without_compliant_name() {
        local file
        for file in $(emitters); do
            if ! grep -qE "${COMPLIANT_NAME}" "${file}"; then
                echo "${file##*/}"
            fi
        done
        echo done
    }

    noncompliant_name_expressions() {
        grep -hE '=.*\$\(date \+%s%[0-9]*N\)' "${ARTIFACTS_DIR}"/*.sh \
            | grep -vE "${COMPLIANT_NAME}" \
            | sed 's/^[[:space:]]*//'
        echo done
    }

    compliant_name_count() {
        grep -hE "${COMPLIANT_NAME}" "${ARTIFACTS_DIR}"/*.sh | wc -l | tr -d ' '
    }

    It 'builds every emitted event file name from a nanosecond timestamp and the process id'
        When call emitters_without_compliant_name
        The output should equal done
    End

    It 'leaves no event file name built from a coarser or non-unique expression'
        When call noncompliant_name_expressions
        The output should equal done
    End

    It 'covers all known emission sites'
        When call compliant_name_count
        The output should equal 13
    End

    Describe 'repeated emission from a single process'
        setup_events_dir() {
            EVENTS_TEST_ROOT=$(mktemp -d)
            EVENTS_LOGGING_DIR="${EVENTS_TEST_ROOT}/"
        }

        cleanup_events_dir() {
            rm -rf "${EVENTS_TEST_ROOT}"
        }

        BeforeEach 'setup_events_dir'
        AfterEach 'cleanup_events_dir'

        Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

        emit_twice_then_count() {
            logs_to_events "AKS.Spec.first" true
            logs_to_events "AKS.Spec.second" true
            find "${EVENTS_TEST_ROOT}" -maxdepth 1 -type f -name '*.json' | wc -l | tr -d ' '
        }

        # Both calls run in the same shell, so they share a BASHPID and only the
        # timestamp separates them. This guards that contract end to end; it is
        # not a measurement of the resolution actually required.
        It 'does not overwrite the previous event when logs_to_events is called back to back'
            When call emit_twice_then_count
            The output should equal 2
        End
    End
End
