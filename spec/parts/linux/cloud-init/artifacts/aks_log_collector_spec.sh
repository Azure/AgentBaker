#!/bin/bash
# shellcheck disable=SC2329 # Mocks are invoked by the sourced collector function.

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

Describe 'I/O recorder support bundle export'
    setup_export() {
        TEST_ROOT=$(mktemp -d)
        SOURCE_DIR="${TEST_ROOT}/captures"
        WORK_DIR="${TEST_ROOT}/work"
        SCRIPT="${TEST_ROOT}/collector.sh"
        GENERIC_SCRIPT="${TEST_ROOT}/generic.sh"
        mkdir -p "$SOURCE_DIR" "$WORK_DIR"
        sed -n '/^function collectIORecorderCaptures {$/,/^}$/p' \
            ./parts/linux/cloud-init/artifacts/aks-log-collector.sh > "$SCRIPT"
        # shellcheck disable=SC2016 # Match the literal array expression in the source.
        sed -n '/^for file in "${GLOBS\[@\]}"; do$/,/^done$/p' \
            ./parts/linux/cloud-init/artifacts/aks-log-collector.sh > "$GENERIC_SCRIPT"
        # shellcheck disable=SC1090
        . "$SCRIPT"
    }

    cleanup_export() {
        rm -rf "$TEST_ROOT"
    }

    export_captures() {
        cd "$WORK_DIR" || return 1
        collectIORecorderCaptures "$SOURCE_DIR" logs.zip
    }

    export_manifest() {
        unzip -p "${WORK_DIR}/logs.zip" collect/io-recorder-export.json | jq -r "$1"
    }

    capture() {
        printf '{"schema_version":1,"capture_id":"%s","collected_utc":"2026-01-01T00:00:00Z","history":[]}\n' \
            "$1" > "${SOURCE_DIR}/capture-$1.json"
    }

    sized_capture() {
        local bytes="$1"
        { printf '{"data":"'; head -c "$((bytes - 11))" /dev/zero | tr '\0' x; printf '"}'; } \
            > "${SOURCE_DIR}/capture-sized.json"
    }

    # shellcheck disable=SC2034 # Variables are consumed by the extracted generic loop.
    export_with_generic_collection() {
        export_captures || return 1
        local ZIP="${WORK_DIR}/logs.zip" IO_RECORDER_DIR="$SOURCE_DIR"
        local MAX_COLLECT_SIZE=10485760 MAX_SIZE=104857600 ZIP_STORE_SUFFIXES=".gz:.zip"
        local -a GLOBS=("$SOURCE_DIR" "$SOURCE_DIR"/* "${TEST_ROOT}/normal.log")
        stat() { wc -c < "${*: -1}"; }
        # shellcheck disable=SC1090
        . "$GENERIC_SCRIPT"
    }

    BeforeEach 'setup_export'
    AfterEach 'cleanup_export'

    It 'exports an immutable whole capture and a correlatable manifest without deleting the source'
        capture abc-123
        When call export_captures
        The status should be success
        The path "${SOURCE_DIR}/capture-abc-123.json" should be exist
        The path "${WORK_DIR}/collect/io-recorder/capture-abc-123.json" should not be exist
        The value "$(export_manifest .status)" should equal complete
        The value "$(export_manifest '.files[0].status')" should equal included
        The value "$(export_manifest '.files[0].sha256')" should equal "$(sha256sum "${SOURCE_DIR}/capture-abc-123.json" | cut -d ' ' -f1)"
        The value "$(unzip -p "${WORK_DIR}/logs.zip" collect/io-recorder/capture-abc-123.json)" should equal "$(cat "${SOURCE_DIR}/capture-abc-123.json")"
        The value "$(export_manifest '.collected_utc | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$")')" should equal true
    End

    It 'accepts exactly one MiB without truncation'
        sized_capture 1048576
        When call export_captures
        The status should be success
        The value "$(export_manifest '.files[0].bytes')" should equal 1048576
        The value "$(export_manifest '.files[0].status')" should equal included
        The value "$(unzip -p "${WORK_DIR}/logs.zip" collect/io-recorder/capture-sized.json | wc -c | tr -d ' ')" should equal 1048576
    End

    It 'rejects an oversized capture instead of tail-truncating it'
        sized_capture 1048577
        When call export_captures
        The status should be success
        The stderr should include 'oversized'
        The value "$(export_manifest .status)" should equal partial
        The value "$(export_manifest '.files[0].status')" should equal oversized
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip")" should equal collect/io-recorder-export.json
    End

    It 'bounds the capture count and reports unexamined files'
        for i in 1 2 3 4 5 6 7 8 9; do capture "$i"; done
        When call export_captures
        The status should be success
        The stderr should include 'file limit reached; 1 files not examined'
        The value "$(export_manifest '.files | length')" should equal 8
        The value "$(export_manifest .unexamined_files)" should equal 1
        The value "$(export_manifest .limits.max_total_bytes)" should equal 8388608
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip" | wc -l | tr -d ' ')" should equal 9
    End

    It 'exports all eight one-MiB captures within the snapshot byte budget'
        sized_capture 1048576
        for i in 1 2 3 4 5 6 7; do
            cp "${SOURCE_DIR}/capture-sized.json" "${SOURCE_DIR}/capture-$i.json"
        done
        When call export_captures
        The status should be success
        The value "$(export_manifest '[.files[].bytes] | add')" should equal 8388608
        The value "$(export_manifest .status)" should equal complete
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip" | wc -l | tr -d ' ')" should equal 9
    End

    It 'reports absence without requiring the recorder to be installed'
        rmdir "$SOURCE_DIR"
        When call export_captures
        The status should be success
        The value "$(export_manifest .status)" should equal not_present
        The value "$(export_manifest '.files | length')" should equal 0
    End

    It 'reports an empty recorder directory'
        When call export_captures
        The status should be success
        The value "$(export_manifest .status)" should equal empty
    End

    It 'ignores unpublished staging files with the production glob options'
        shopt -s nullglob nocaseglob extglob
        printf 'not published yet' > "${SOURCE_DIR}/.capture-pending.json"
        When call export_captures
        The status should be success
        The value "$(export_manifest .status)" should equal empty
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip")" should equal collect/io-recorder-export.json
    End

    It 'rejects symlinked directories'
        rmdir "$SOURCE_DIR"
        ln -s "$WORK_DIR" "$SOURCE_DIR"
        When call export_captures
        The status should be success
        The stderr should include 'not a regular directory'
        The value "$(export_manifest .status)" should equal invalid_directory
    End

    It 'rejects symlinks and named pipes without reading them'
        capture target
        ln -s "${SOURCE_DIR}/capture-target.json" "${SOURCE_DIR}/capture-link.json"
        mkfifo "${SOURCE_DIR}/capture-pipe.json"
        When call export_captures
        The status should be success
        The stderr should include 'capture-link.json: not_regular'
        The stderr should include 'capture-pipe.json: not_regular'
        The value "$(export_manifest '[.files[] | select(.status == "not_regular")] | length')" should equal 2
    End

    It 'reports invalid JSON and does not include it'
        printf '{"partial":' > "${SOURCE_DIR}/capture-bad.json"
        When call export_captures
        The status should be success
        The stderr should include 'invalid_json'
        The value "$(export_manifest '.files[0].status')" should equal invalid_json
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip")" should equal collect/io-recorder-export.json
    End

    It 'rejects JSON streams rather than treating them as one capture'
        printf '{}\n{}\n' > "${SOURCE_DIR}/capture-stream.json"
        When call export_captures
        The status should be success
        The stderr should include 'invalid_json'
        The value "$(export_manifest '.files[0].status')" should equal invalid_json
    End

    It 'reports a retention race at read time and keeps the partial file out of the archive'
        capture vanished
        timeout() {
            rm "${SOURCE_DIR}/capture-vanished.json"
            return 1
        }
        When call export_captures
        The status should be success
        The stderr should include 'read_failed'
        The value "$(export_manifest '.files[0].status')" should equal read_failed
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip")" should equal collect/io-recorder-export.json
    End

    It 'reports a capture evicted after enumeration rather than claiming an empty export'
        capture vanished
        function [ {
            if test "${1:-}" = "-L" && test "${2:-}" = "${SOURCE_DIR}/capture-vanished.json"; then
                rm -f "${SOURCE_DIR}/capture-vanished.json"
            fi
            builtin [ "$@"
        }
        When call export_captures
        The status should be success
        The stderr should include 'vanished'
        The value "$(export_manifest .status)" should equal partial
        The value "$(export_manifest '.files[0].status')" should equal vanished
    End

    It 'reports timed-out reads separately from other read errors'
        capture timeout
        timeout() { return 124; }
        When call export_captures
        The status should be success
        The stderr should include 'read_timeout'
        The value "$(export_manifest '.files[0].status')" should equal read_timeout
    End

    It 'rejects invalid capture filenames'
        printf '{}' > "${SOURCE_DIR}/capture-invalid name.json"
        When call export_captures
        The status should be success
        The stderr should include 'invalid_name'
        The value "$(export_manifest '.files[0].status')" should equal invalid_name
    End

    It 'reports failure to hash a capture instead of claiming inclusion'
        capture hash
        sha256sum() { return 1; }
        When call export_captures
        The status should be success
        The stderr should include 'hash_failed'
        The value "$(export_manifest '.files[0].status')" should equal hash_failed
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip")" should equal collect/io-recorder-export.json
    End

    It 'reports failed capture archiving in the export manifest'
        capture badzip
        zip() {
            case "${*: -1}" in
                */capture-*.json) return 1 ;;
                *) command zip "$@" ;;
            esac
        }
        When call export_captures
        The status should be success
        The stderr should include 'zip_failed'
        The value "$(export_manifest '.files[0].status')" should equal zip_failed
    End

    It 'returns failure if the export manifest cannot be archived'
        zip() { return 1; }
        When call export_captures
        The status should be failure
    End

    It 'preserves priority captures and never re-adds rejected files through the generic loop'
        capture valid
        sized_capture 1048577
        printf 'ordinary support log' > "${TEST_ROOT}/normal.log"
        When call export_with_generic_collection
        The status should be success
        The stderr should include oversized
        The stdout should include 'normal.log'
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip")" should include collect/io-recorder/capture-valid.json
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip")" should not include capture-sized.json
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip")" should not include '/captures/'
        The value "$(unzip -p "${WORK_DIR}/logs.zip" collect/io-recorder/capture-valid.json)" should equal "$(cat "${SOURCE_DIR}/capture-valid.json")"
        The value "$(unzip -Z1 "${WORK_DIR}/logs.zip" | head -n 1)" should equal collect/io-recorder/capture-valid.json
    End
End
