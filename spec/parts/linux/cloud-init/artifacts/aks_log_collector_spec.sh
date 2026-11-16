#!/bin/bash

Describe 'aks-log-collector.sh'
    setup() {
        TEST_DIR="$(mktemp -d)"
        # shellcheck disable=SC1091
        __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/aks-log-collector.sh"
        LOG_BUNDLE_DIR="${TEST_DIR}/logcollector"
        LOG_BUNDLE="${LOG_BUNDLE_DIR}/logs.zip"
    }
    cleanup_test() { rm -rf "${TEST_DIR}"; }
    BeforeEach 'setup'
    AfterEach 'cleanup_test'

    Describe 'publishLogBundle'
        It 'publishes the bundle readable only by its owner'
            echo bundle > "${TEST_DIR}/aks_logs.zip"
            When call publishLogBundle "${TEST_DIR}/aks_logs.zip"
            The status should be success
            The contents of file "${LOG_BUNDLE}" should eq bundle
            The value "$(ls -l "${LOG_BUNDLE}" | cut -c1-10)" should eq '-rw-------'
        End

        It 'replaces a world-readable bundle left by a previous run'
            mkdir -p "${LOG_BUNDLE_DIR}"
            echo old > "${LOG_BUNDLE}"
            chmod 0644 "${LOG_BUNDLE}"
            echo new > "${TEST_DIR}/aks_logs.zip"
            When call publishLogBundle "${TEST_DIR}/aks_logs.zip"
            The status should be success
            The contents of file "${LOG_BUNDLE}" should eq new
            The value "$(ls -l "${LOG_BUNDLE}" | cut -c1-10)" should eq '-rw-------'
        End
    End

    Describe 'cleanup'
        It 'removes the published bundle'
            mkdir -p "${LOG_BUNDLE_DIR}"
            echo bundle > "${LOG_BUNDLE}"
            WORKDIR="$(mktemp -d /tmp/tmp.XXXXXXXX)"
            When run cleanup
            The status should be success
            The output should include 'Log collection finished.'
            The path "${LOG_BUNDLE}" should not be exist
            The path "${WORKDIR}" should not be exist
        End
    End
End
