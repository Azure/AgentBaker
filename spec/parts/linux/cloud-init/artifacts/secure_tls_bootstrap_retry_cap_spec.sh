# shellcheck shell=bash
# shellcheck disable=SC2034  # SHELLSPEC_TMPBASE set by ShellSpec
#
# Tests for parts/linux/cloud-init/artifacts/secure-tls-bootstrap-retry-cap.sh
#
# These tests exercise the wrapper end-to-end against fake binaries / state
# directories so behavior is verified without touching real /var paths.

Describe 'secure-tls-bootstrap-retry-cap.sh'
    WRAPPER="parts/linux/cloud-init/artifacts/secure-tls-bootstrap-retry-cap.sh"

    # Helper: read filesystem mode portably (GNU stat -c on Linux,
    # BSD stat -f on macOS).
    state_dir_mode() {
        stat -c '%a' "${SCRATCH}/state" 2>/dev/null \
            || stat -f '%Lp' "${SCRATCH}/state" 2>/dev/null
    }

    # Helper: concatenated content of all event JSON files written under
    # ${SCRATCH}/events. Returns empty string if directory is absent.
    event_files_contents() {
        # shellcheck disable=SC2010
        if [ -d "${SCRATCH}/events" ]; then
            cat "${SCRATCH}"/events/*.json 2>/dev/null
        fi
    }

    # Helper: count of event JSON files written under ${SCRATCH}/events.
    event_files_count() {
        if [ -d "${SCRATCH}/events" ]; then
            find "${SCRATCH}/events" -mindepth 1 -maxdepth 1 -type f 2>/dev/null \
                | wc -l | tr -d '[:space:]'
        else
            printf '0'
        fi
    }

    # Helper: write a fake STLS binary that does not produce side effects
    # for --version invocations (the wrapper queries --version when
    # emitting the RetryCapReached event, and tests must not confuse that
    # invocation with a real bootstrap attempt).
    write_fake_binary() {
        local marker_path="${1:-}"
        cat > "${SCRATCH}/binary.sh" <<EOS
#!/bin/bash
if [ "\${1:-}" = "--version" ]; then
    echo "v1.1.4-fake"
    exit 0
fi
echo "fake-stls-binary called: \$*"
EOS
        if [ -n "${marker_path}" ]; then
            cat >> "${SCRATCH}/binary.sh" <<EOS
touch "${marker_path}"
EOS
        fi
        cat >> "${SCRATCH}/binary.sh" <<'EOS'
exit 0
EOS
        chmod +x "${SCRATCH}/binary.sh"
    }

    setup() {
        # Per-test scratch root inside the ShellSpec tmpbase (auto-cleaned).
        # shellcheck disable=SC2154
        SCRATCH="${SHELLSPEC_TMPBASE}/stls-retry-cap-$$-${RANDOM}"
        mkdir -p "${SCRATCH}"
        export SECURE_TLS_BOOTSTRAPPING_STATE_DIR="${SCRATCH}/state"
        export SECURE_TLS_BOOTSTRAPPING_EVENTS_DIR="${SCRATCH}/events"
        export SECURE_TLS_BOOTSTRAPPING_BOOT_ID_SRC="${SCRATCH}/boot_id"
        printf 'shellspec-boot-id-aaaa-bbbb-cccc' > "${SECURE_TLS_BOOTSTRAPPING_BOOT_ID_SRC}"

        write_fake_binary
        export SECURE_TLS_BOOTSTRAPPING_BINARY="${SCRATCH}/binary.sh"

        # Conservative defaults that keep tests fast.
        export SECURE_TLS_BOOTSTRAPPING_MAX_ATTEMPTS="3"
        export SECURE_TLS_BOOTSTRAPPING_MAX_TOTAL_SECONDS="9999"
        export SECURE_TLS_BOOTSTRAPPING_INITIAL_BACKOFF_SECONDS="1"
        export SECURE_TLS_BOOTSTRAPPING_MAX_BACKOFF_SECONDS="4"
    }

    cleanup() {
        [ -n "${SCRATCH:-}" ] && rm -rf "${SCRATCH}"
        unset SECURE_TLS_BOOTSTRAPPING_STATE_DIR
        unset SECURE_TLS_BOOTSTRAPPING_EVENTS_DIR
        unset SECURE_TLS_BOOTSTRAPPING_BOOT_ID_SRC
        unset SECURE_TLS_BOOTSTRAPPING_BINARY
        unset SECURE_TLS_BOOTSTRAPPING_MAX_ATTEMPTS
        unset SECURE_TLS_BOOTSTRAPPING_MAX_TOTAL_SECONDS
        unset SECURE_TLS_BOOTSTRAPPING_INITIAL_BACKOFF_SECONDS
        unset SECURE_TLS_BOOTSTRAPPING_MAX_BACKOFF_SECONDS
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'happy path'
        It 'runs the binary on first invocation and increments the attempts counter'
            When run script "${WRAPPER}"
            The status should be success
            The output should include "fake-stls-binary called"
            The contents of file "${SCRATCH}/state/attempts" should equal "1"
            The path "${SCRATCH}/state/first-attempt" should be exist
            The path "${SCRATCH}/state/last-attempt" should be exist
            The path "${SCRATCH}/state/capped" should not be exist
        End

        It 'creates the state directory with 0700 mode if missing'
            When run script "${WRAPPER}"
            The status should be success
            The path "${SCRATCH}/state" should be directory
            The output should include "fake-stls-binary called"
            The result of function state_dir_mode should equal "700"
        End

        It 'propagates the binary exit code on the non-capped path'
            cat > "${SCRATCH}/binary.sh" <<'EOS'
#!/bin/bash
if [ "${1:-}" = "--version" ]; then
    echo "v1.1.4-fake"
    exit 0
fi
exit 42
EOS
            chmod +x "${SCRATCH}/binary.sh"
            When run script "${WRAPPER}"
            The status should equal 42
            The contents of file "${SCRATCH}/state/attempts" should equal "1"
        End
    End

    Describe 'attempt cap'
        It 'emits a single RetryCapReached event when the attempts cap is hit and does NOT run the binary'
            # Seed the state as if we'd already done MAX_ATTEMPTS attempts.
            mkdir -p "${SCRATCH}/state"
            chmod 700 "${SCRATCH}/state"
            echo "3" > "${SCRATCH}/state/attempts"
            now_seed=$(date +%s)
            echo "$((now_seed - 10))" > "${SCRATCH}/state/first-attempt"
            echo "$((now_seed - 5))" > "${SCRATCH}/state/last-attempt"
            printf 'shellspec-boot-id-aaaa-bbbb-cccc' > "${SCRATCH}/state/boot-id"
            echo "GetNonceFailure" > "${SCRATCH}/state/last-final-error"
            # If the binary is invoked as a bootstrap attempt (not as
            # `--version`), it creates this marker — assert it doesn't.
            write_fake_binary "${SCRATCH}/binary-ran"

            When run script "${WRAPPER}"
            The status should be success
            The stderr should include "cap reached"
            The path "${SCRATCH}/binary-ran" should not be exist
            The path "${SCRATCH}/state/capped" should be exist
        End

        It 'writes a RetryCapReached event with the expected TaskName and FinalErrorType'
            mkdir -p "${SCRATCH}/state"
            chmod 700 "${SCRATCH}/state"
            echo "3" > "${SCRATCH}/state/attempts"
            now_seed=$(date +%s)
            echo "$((now_seed - 10))" > "${SCRATCH}/state/first-attempt"
            echo "$((now_seed - 5))" > "${SCRATCH}/state/last-attempt"
            printf 'shellspec-boot-id-aaaa-bbbb-cccc' > "${SCRATCH}/state/boot-id"
            echo "GetNonceFailure" > "${SCRATCH}/state/last-final-error"

            When run script "${WRAPPER}"
            The status should be success
            The stderr should include "cap reached"
            The path "${SCRATCH}/state/capped" should be exist
            The result of function event_files_contents should include "AKS.Bootstrap.SecureTLSBootstrapping.RetryCapReached"
            The result of function event_files_contents should include "RetryCapReached"
            The result of function event_files_contents should include "GetNonceFailure"
        End

        It 'no-ops on subsequent invocations once the capped sentinel exists'
            mkdir -p "${SCRATCH}/state"
            chmod 700 "${SCRATCH}/state"
            : > "${SCRATCH}/state/capped"
            write_fake_binary "${SCRATCH}/binary-ran"

            When run script "${WRAPPER}"
            The status should be success
            The stderr should include "cap already reached"
            The path "${SCRATCH}/binary-ran" should not be exist
            # No new event written on the no-op path.
            The result of function event_files_count should equal "0"
        End
    End

    Describe 'time-budget cap'
        It 'fires the cap when elapsed time exceeds MAX_TOTAL_SECONDS even if attempts are low'
            export SECURE_TLS_BOOTSTRAPPING_MAX_ATTEMPTS="999"
            export SECURE_TLS_BOOTSTRAPPING_MAX_TOTAL_SECONDS="60"
            mkdir -p "${SCRATCH}/state"
            chmod 700 "${SCRATCH}/state"
            echo "2" > "${SCRATCH}/state/attempts"
            now_seed=$(date +%s)
            # first-attempt was 5 minutes ago: budget exceeded.
            echo "$((now_seed - 300))" > "${SCRATCH}/state/first-attempt"
            echo "$((now_seed - 5))" > "${SCRATCH}/state/last-attempt"
            printf 'shellspec-boot-id-aaaa-bbbb-cccc' > "${SCRATCH}/state/boot-id"

            When run script "${WRAPPER}"
            The status should be success
            The stderr should include "cap reached"
            The path "${SCRATCH}/state/capped" should be exist
        End
    End

    Describe 'boot-id reset'
        It 'wipes state when the boot-id changes between invocations'
            mkdir -p "${SCRATCH}/state"
            chmod 700 "${SCRATCH}/state"
            echo "2" > "${SCRATCH}/state/attempts"
            echo "100" > "${SCRATCH}/state/first-attempt"
            echo "200" > "${SCRATCH}/state/last-attempt"
            # Old boot id is different from the one we'll present below.
            printf 'old-boot-id-xxx' > "${SCRATCH}/state/boot-id"

            When run script "${WRAPPER}"
            The status should be success
            The output should include "fake-stls-binary called"
            # Counter was wiped then incremented to 1 on this invocation.
            The contents of file "${SCRATCH}/state/attempts" should equal "1"
        End
    End

    Describe 'env-var overrides'
        It 'honors a custom MAX_ATTEMPTS value when seeding right at the override'
            export SECURE_TLS_BOOTSTRAPPING_MAX_ATTEMPTS="7"
            mkdir -p "${SCRATCH}/state"
            chmod 700 "${SCRATCH}/state"
            echo "7" > "${SCRATCH}/state/attempts"
            now_seed=$(date +%s)
            echo "$((now_seed - 60))" > "${SCRATCH}/state/first-attempt"
            echo "$((now_seed - 5))" > "${SCRATCH}/state/last-attempt"
            printf 'shellspec-boot-id-aaaa-bbbb-cccc' > "${SCRATCH}/state/boot-id"

            When run script "${WRAPPER}"
            The status should be success
            The stderr should include "cap reached"
            The path "${SCRATCH}/state/capped" should be exist
        End

        It 'clamps a non-numeric MAX_ATTEMPTS to the safe default (50) and runs normally'
            export SECURE_TLS_BOOTSTRAPPING_MAX_ATTEMPTS="not-a-number"
            When run script "${WRAPPER}"
            The status should be success
            The output should include "fake-stls-binary called"
            The contents of file "${SCRATCH}/state/attempts" should equal "1"
        End
    End

    Describe 'missing binary'
        It 'exits 127 when the binary is not present'
            export SECURE_TLS_BOOTSTRAPPING_BINARY="${SCRATCH}/does-not-exist"
            When run script "${WRAPPER}"
            The status should equal 127
            The stderr should include "binary not found"
        End
    End
End
