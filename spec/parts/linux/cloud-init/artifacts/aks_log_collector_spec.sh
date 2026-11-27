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

    Describe 'collectLogs'
        setup_collection() {
            COLLECTOR_WORKDIR="$(command mktemp -d /tmp/tmp.XXXXXXXX)"
            export TEST_DIR COLLECTOR_WORKDIR
            export DEBUG=
            export UPLOAD_STATUS=0
            mkdir "${TEST_DIR}/bin"
            cat > "${TEST_DIR}/bin/collector-fixture" <<'EOF'
#!/bin/bash
case "${0##*/}" in
    curl)
        printf '%s\n' '{"tagsList":[]}'
        ;;
    jq)
        printf '%s\n' false
        exit 1
        ;;
    mktemp)
        printf '%s\n' "$COLLECTOR_WORKDIR"
        ;;
    zip)
        # Consume only relative fixture FIFOs, never the host's log files.
        for arg; do
            if [[ "$arg" == *.zip ]]; then
                archive="$arg"
            elif [[ "$arg" == collect/* && -p "$arg" ]]; then
                cat "$arg" >/dev/null
            fi
        done
        printf '%s\n' 'fixture bundle' > "$archive"
        ;;
    stat)
        printf '%s\n' 0
        ;;
    journalctl|crictl|ip)
        printf '%s\n' 'fixture command output'
        ;;
    python3)
        printf '%s\n' "$@" > "${TEST_DIR}/default-upload-args"
        exit "$UPLOAD_STATUS"
        ;;
esac
EOF
            chmod +x "${TEST_DIR}/bin/collector-fixture"
            local tool
            for tool in curl jq mktemp zip stat journalctl crictl ip python3; do
                ln -s collector-fixture "${TEST_DIR}/bin/${tool}"
            done
            export PATH="${TEST_DIR}/bin:${PATH}"
        }
        cleanup_collection() {
            rm -rf "${COLLECTOR_WORKDIR}"
        }
        collect_and_check_caller_directory() {
            local caller_directory="$PWD"
            collectLogs
            local status=$?
            [ "$PWD" = "$caller_directory" ] || return 1
            return "$status"
        }
        collect_with_caller_trap() (
            trap ':' EXIT
            local caller_trap
            caller_trap="$(trap -p EXIT)"
            collectLogs
            local status=$?
            [ "$(trap -p EXIT)" = "$caller_trap" ] || return 1
            return "$status"
        )
        BeforeEach 'setup_collection'
        AfterEach 'cleanup_collection'

        It 'keeps the existing Host uploader as the default'
            When call collect_and_check_caller_directory
            The status should be success
            The output should include 'Log collection finished.'
            The contents of file "${TEST_DIR}/default-upload-args" should eq '/opt/azure/containers/aks-log-collector-send.py'
            The path "${LOG_BUNDLE}" should not be exist
            The path "${COLLECTOR_WORKDIR}" should not be exist
        End

        It 'preserves default upload failure and still cleans up'
            UPLOAD_STATUS=23
            When call collect_and_check_caller_directory
            The status should eq 23
            The output should include 'Log collection finished.'
            The path "${LOG_BUNDLE}" should not be exist
            The path "${COLLECTOR_WORKDIR}" should not be exist
        End

        It 'does not replace the sourced caller EXIT trap'
            When call collect_with_caller_trap
            The status should be success
            The output should include 'Log collection finished.'
            The path "${COLLECTOR_WORKDIR}" should not be exist
        End

        It 'lets a synchronous uploader consume the private bundle before cleanup'
            uploadLogBundle() {
                [ -s "$1" ] || return 1
                ls -l "$1" | cut -c1-10 > "${TEST_DIR}/bundle-mode"
                cat "$1" > "${TEST_DIR}/uploaded"
                printf '%s\n' 'custom upload completed'
            }
            When call collect_and_check_caller_directory
            The status should be success
            The output should include 'custom upload completed'
            The contents of file "${TEST_DIR}/uploaded" should eq 'fixture bundle'
            The contents of file "${TEST_DIR}/bundle-mode" should eq '-rw-------'
            The path "${TEST_DIR}/default-upload-args" should not be exist
            The path "${LOG_BUNDLE}" should not be exist
            The path "${COLLECTOR_WORKDIR}" should not be exist
        End

        It 'returns custom upload failure and cleans up before returning to the caller'
            uploadLogBundle() {
                [ -s "$1" ] || return 1
                printf '%s\n' 'fixture upload failed' >&2
                return 42
            }
            When call collect_and_check_caller_directory
            The status should eq 42
            The output should include 'Log collection finished.'
            The stderr should eq 'fixture upload failed'
            The path "${LOG_BUNDLE}" should not be exist
            The path "${COLLECTOR_WORKDIR}" should not be exist
        End

        It 'cleans up when interrupted during a synchronous upload'
            uploadLogBundle() {
                [ -s "$1" ] || return 1
                sh -c 'kill -TERM "$PPID"'
            }
            When call collect_and_check_caller_directory
            The status should eq 1
            The output should include 'Log collection finished.'
            The path "${LOG_BUNDLE}" should not be exist
            The path "${COLLECTOR_WORKDIR}" should not be exist
        End

        It 'does not invoke the uploader if publishing the bundle fails'
            publishLogBundle() {
                return 1
            }
            uploadLogBundle() {
                printf '%s\n' 'unexpected uploader invocation'
                return 0
            }
            When call collect_and_check_caller_directory
            The status should eq 1
            The output should include 'ERROR: Failed to publish log bundle'
            The output should not include 'unexpected uploader invocation'
            The path "${COLLECTOR_WORKDIR}" should not be exist
        End
    End
End
