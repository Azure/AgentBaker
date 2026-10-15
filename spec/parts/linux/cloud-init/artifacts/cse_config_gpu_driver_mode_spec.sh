#!/bin/bash

Describe 'NVIDIA GPU driver mode configuration'
    Include './parts/linux/cloud-init/artifacts/cse_config_gpu.sh'

    setup() {
        command mkdir -p "${PWD}/tmp"
        TEST_DIR=$(command mktemp -d "${PWD}/tmp/nvidia-gpu-driver-mode.XXXXXX")
        MODE_FILE="${TEST_DIR}/containers/nvidia-gpu-driver-mode"
        DRIVER_MODE=DRA
        FAIL_STEP=''
        CHECK_ATOMIC_REPLACE=false
    }

    cleanup() {
        command rm -rf "${TEST_DIR:?}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    mkdir() {
        [ "${FAIL_STEP}" != mkdir ] || return 1
        command mkdir "$@"
    }

    mktemp() {
        [ "${FAIL_STEP}" != mktemp ] || return 1
        command mktemp "$@"
    }

    printf() {
        if [ "${FAIL_STEP}" = write ] && [ "$1" = 'DRA\n' ]; then
            return 1
        fi
        builtin printf "$@"
    }

    # Run without root on developer machines; validate the requested owner.
    chown() {
        [ "${FAIL_STEP}" != chown ] || return 1
        [ "$1" = root:root ]
    }

    chmod() {
        [ "${FAIL_STEP}" != chmod ] || return 1
        command chmod "$@"
    }

    mv() {
        [ "${FAIL_STEP}" != mv ] || return 1
        if [ "${CHECK_ATOMIC_REPLACE}" = true ]; then
            # The old value must remain visible until the final rename.
            [ "$(command cat "${MODE_FILE}")" = old-value ] || return 1
            [ "$(command cat "$2")" = DRA ] || return 1
        fi
        command mv "$@"
    }

    rm() {
        [ "${FAIL_STEP}" != rm ] || return 1
        command rm "$@"
    }

    seed_mode() {
        command mkdir -p "$(dirname "${MODE_FILE}")"
        builtin printf '%s\n' "$1" > "${MODE_FILE}"
    }

    mode_permissions() {
        stat -c '%a' "${MODE_FILE}" 2>/dev/null || stat -f '%Lp' "${MODE_FILE}"
    }

    temporary_files() {
        local candidate
        for candidate in "${MODE_FILE}".*; do
            [ ! -e "${candidate}" ] || builtin printf '%s\n' "${candidate}"
        done
    }

    It 'writes the exact DRA value for the selected DRA mode'
        When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
        The status should be success
        The contents of file "${MODE_FILE}" should equal 'DRA'
        The path "${MODE_FILE}" should be file
        The value "$(wc -c < "${MODE_FILE}" | tr -d '[:space:]')" should equal '4'
        The value "$(mode_permissions)" should equal '644'
        The value "$(temporary_files)" should equal ''
    End

    Describe 'non-DRA'
        Parameters
            DevicePlugin
            ''
        End

        It "leaves the file and directory absent for '$1'"
            DRIVER_MODE=$1
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be success
            The path "${MODE_FILE}" should not be exist
            The path "${TEST_DIR}/containers" should not be exist
        End

        It "removes a stale DRA value for '$1'"
            DRIVER_MODE=$1
            seed_mode DRA
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be success
            The path "${MODE_FILE}" should not be exist
        End
    End

    It 'does not infer the mode from the raw DRA flag'
        ENABLE_MANAGED_GPU_DRA=true
        DRIVER_MODE=''
        When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
        The status should be success
        The path "${MODE_FILE}" should not be exist
    End

    It 'replaces an old value only after the DRA value is complete'
        seed_mode old-value
        CHECK_ATOMIC_REPLACE=true
        When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
        The status should be success
        The contents of file "${MODE_FILE}" should equal 'DRA'
        The value "$(temporary_files)" should equal ''
    End

    It 'is safe to repeat on a DRA node'
        write_twice() {
            configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}" && configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
        }
        When call write_twice
        The status should be success
        The contents of file "${MODE_FILE}" should equal 'DRA'
        The value "$(temporary_files)" should equal ''
    End

    It 'does not change the existing managed-GPU marker'
        seed_mode DRA
        builtin printf 'unchanged\n' > "${TEST_DIR}/containers/managed-gpu-experience.enabled"
        DRIVER_MODE=DevicePlugin
        When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
        The status should be success
        The contents of file "${TEST_DIR}/containers/managed-gpu-experience.enabled" should equal 'unchanged'
    End

    Describe 'publication failures'
        Parameters
            mkdir
            mktemp
            write
            chown
            chmod
            mv
        End

        It "returns an error and does not publish a partial value when $1 fails"
            FAIL_STEP=$1
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be failure
            The path "${MODE_FILE}" should not be exist
            The value "$(temporary_files)" should equal ''
        End

        It "preserves the old value and cleans up when $1 fails"
            seed_mode old-value
            FAIL_STEP=$1
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be failure
            The contents of file "${MODE_FILE}" should equal 'old-value'
            The value "$(temporary_files)" should equal ''
        End
    End

    It 'returns an error if a stale DRA value cannot be removed'
        seed_mode DRA
        DRIVER_MODE=DevicePlugin
        FAIL_STEP=rm
        When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
        The status should be failure
        The contents of file "${MODE_FILE}" should equal 'DRA'
    End

    It 'rejects a directory at the destination instead of publishing inside it'
        command mkdir -p "${MODE_FILE}"
        When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
        The status should be failure
        The path "${MODE_FILE}" should be directory
        The value "$(temporary_files)" should equal ''
    End
End
