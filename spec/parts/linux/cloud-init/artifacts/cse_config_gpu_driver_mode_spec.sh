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
        EXPECTED_OLD_MODE=old-value
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
        if [ "${FAIL_STEP}" = write ] && [ "${1:-}" = '%s\n' ] &&
            { [ "${2:-}" = DRA ] || [ "${2:-}" = DevicePlugin ]; }; then
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
            [ "$(command cat "${MODE_FILE}")" = "${EXPECTED_OLD_MODE}" ] || return 1
            [ "$(command cat "$2")" = "${DRIVER_MODE}" ] || return 1
        fi
        command mv "$@"
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

    Describe 'valid modes'
        Parameters
            DRA 4
            DevicePlugin 13
        End

        It "writes the exact $1 value with a newline and readable permissions"
            DRIVER_MODE=$1
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be success
            The contents of file "${MODE_FILE}" should equal "${DRIVER_MODE}"
            The path "${MODE_FILE}" should be file
            The value "$(wc -c < "${MODE_FILE}" | tr -d '[:space:]')" should equal "$2"
            The value "$(mode_permissions)" should equal '644'
            The value "$(temporary_files)" should equal ''
        End

        It "replaces an old value only after $1 is complete"
            DRIVER_MODE=$1
            seed_mode old-value
            CHECK_ATOMIC_REPLACE=true
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be success
            The contents of file "${MODE_FILE}" should equal "${DRIVER_MODE}"
            The value "$(temporary_files)" should equal ''
        End

        It "is safe to repeat for $1"
            DRIVER_MODE=$1
            write_twice() {
                configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}" && configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            }
            When call write_twice
            The status should be success
            The contents of file "${MODE_FILE}" should equal "${DRIVER_MODE}"
            The value "$(temporary_files)" should equal ''
        End

        It "does not change the existing managed-GPU marker for $1"
            DRIVER_MODE=$1
            seed_mode DRA
            builtin printf 'unchanged\n' > "${TEST_DIR}/containers/managed-gpu-experience.enabled"
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be success
            The contents of file "${TEST_DIR}/containers/managed-gpu-experience.enabled" should equal 'unchanged'
        End

        It "rejects a directory at the destination for $1"
            DRIVER_MODE=$1
            command mkdir -p "${MODE_FILE}"
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be failure
            The path "${MODE_FILE}" should be directory
            The value "$(temporary_files)" should equal ''
        End
    End

    Describe 'mode changes'
        Parameters
            DRA DevicePlugin
            DevicePlugin DRA
        End

        It "replaces $1 with $2 without removing the existing file first"
            EXPECTED_OLD_MODE=$1
            DRIVER_MODE=$2
            seed_mode "${EXPECTED_OLD_MODE}"
            CHECK_ATOMIC_REPLACE=true
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be success
            The contents of file "${MODE_FILE}" should equal "${DRIVER_MODE}"
            The value "$(temporary_files)" should equal ''
        End
    End

    Describe 'unselected modes'
        Parameters
            ''
            false
            true
            dra
            deviceplugin
            None
            unsupported
        End

        It "skips '$1' without creating a file or directory"
            DRIVER_MODE=$1
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be success
            The path "${MODE_FILE}" should not be exist
            The path "${TEST_DIR}/containers" should not be exist
        End

        It "skips '$1' without removing or changing an existing file"
            DRIVER_MODE=$1
            seed_mode DRA
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be success
            The contents of file "${MODE_FILE}" should equal 'DRA'
            The value "$(temporary_files)" should equal ''
        End

        It "skips '$1' even when the destination is a directory"
            DRIVER_MODE=$1
            command mkdir -p "${MODE_FILE}"
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be success
            The path "${MODE_FILE}" should be directory
            The value "$(temporary_files)" should equal ''
        End
    End

    It 'succeeds without arguments under nounset and does not touch the default path'
        call_without_mode() (
            set -u
            # Block writes to the real default path if the guard regresses.
            mkdir() { echo 'unexpected directory creation' >&2; return 1; }
            rm() { echo 'unexpected file removal' >&2; return 1; }
            configureNvidiaGPUDriverMode
        )
        When run call_without_mode
        The status should be success
        The output should equal ''
        The stderr should equal ''
    End

    It 'does not infer the mode from the raw DRA flag'
        ENABLE_MANAGED_GPU_DRA=true
        DRIVER_MODE=DevicePlugin
        When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
        The status should be success
        The contents of file "${MODE_FILE}" should equal 'DevicePlugin'
    End

    Describe 'publication failures'
        Parameters
            DRA mkdir
            DRA mktemp
            DRA write
            DRA chown
            DRA chmod
            DRA mv
            DevicePlugin mkdir
            DevicePlugin mktemp
            DevicePlugin write
            DevicePlugin chown
            DevicePlugin chmod
            DevicePlugin mv
        End

        It "does not publish a partial $1 value when $2 fails"
            DRIVER_MODE=$1
            FAIL_STEP=$2
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be failure
            The path "${MODE_FILE}" should not be exist
            The value "$(temporary_files)" should equal ''
        End

        It "preserves the old value and cleans up when writing $1 fails at $2"
            seed_mode old-value
            DRIVER_MODE=$1
            FAIL_STEP=$2
            When call configureNvidiaGPUDriverMode "${DRIVER_MODE}" "${MODE_FILE}"
            The status should be failure
            The contents of file "${MODE_FILE}" should equal 'old-value'
            The value "$(temporary_files)" should equal ''
        End
    End

End
