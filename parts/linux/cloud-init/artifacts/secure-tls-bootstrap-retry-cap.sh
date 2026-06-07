#!/bin/bash
#
# secure-tls-bootstrap-retry-cap.sh
#
# Wrapper around the aks-secure-tls-bootstrap-client binary that enforces a
# per-provisioning-session retry cap. Without this wrapper, a single stuck VM
# (e.g. transient DNS failure) drives the binary in a kubelet-restart loop and
# emits thousands of failure events per VM, dominating per-event QoS metrics.
#
# Behavior per invocation:
#   - Acquires a flock on the state dir lock file
#   - On a different boot id (reboot), resets all state
#   - If the cap-sentinel already exists, exits 0 immediately (no event)
#   - If attempts >= max OR elapsed >= total budget, emits a single distinct
#     terminal event (TaskName=...SecureTLSBootstrapping.RetryCapReached) and
#     exits 0 so systemd marks the unit successful and stops re-triggering it
#   - Otherwise sleeps for max(0, backoff - elapsed_since_last) and runs the
#     binary as a subprocess; captures the binary's FinalErrorType from the
#     newest event file it produced
#   - Exits with the binary's exit code on the happy / non-capped path
#
# Configurable via env vars (overridable from BootstrappingConfig):
#   SECURE_TLS_BOOTSTRAPPING_MAX_ATTEMPTS         (default 50)
#   SECURE_TLS_BOOTSTRAPPING_MAX_TOTAL_SECONDS    (default 7200 = 2 h)
#   SECURE_TLS_BOOTSTRAPPING_INITIAL_BACKOFF_SECONDS (default 1)
#   SECURE_TLS_BOOTSTRAPPING_MAX_BACKOFF_SECONDS  (default 300)
#
# Per-provisioning-session reset: configureAndStartSecureTLSBootstrapping in
# cse_config.sh wipes the state dir before starting the unit. A reboot also
# resets state via the boot-id check below.

set -u

STATE_DIR="${SECURE_TLS_BOOTSTRAPPING_STATE_DIR:-/var/lib/aks-secure-tls-bootstrap}"
EVENTS_DIR="${SECURE_TLS_BOOTSTRAPPING_EVENTS_DIR:-/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events}"
BINARY="${SECURE_TLS_BOOTSTRAPPING_BINARY:-/opt/bin/aks-secure-tls-bootstrap-client}"
BOOT_ID_FILE="${SECURE_TLS_BOOTSTRAPPING_BOOT_ID_SRC:-/proc/sys/kernel/random/boot_id}"

MAX_ATTEMPTS="${SECURE_TLS_BOOTSTRAPPING_MAX_ATTEMPTS:-50}"
MAX_TOTAL_SECONDS="${SECURE_TLS_BOOTSTRAPPING_MAX_TOTAL_SECONDS:-7200}"
INITIAL_BACKOFF_SECONDS="${SECURE_TLS_BOOTSTRAPPING_INITIAL_BACKOFF_SECONDS:-1}"
MAX_BACKOFF_SECONDS="${SECURE_TLS_BOOTSTRAPPING_MAX_BACKOFF_SECONDS:-300}"

# Clamp non-numeric / negative values to safe defaults to harden against
# garbage env (e.g. apiserver renders an empty override).
sanitize_positive_int() {
    local val="$1"
    local fallback="$2"
    case "$val" in
        ''|*[!0-9]*) echo "$fallback" ;;
        *) echo "$val" ;;
    esac
}

MAX_ATTEMPTS=$(sanitize_positive_int "$MAX_ATTEMPTS" 50)
MAX_TOTAL_SECONDS=$(sanitize_positive_int "$MAX_TOTAL_SECONDS" 7200)
INITIAL_BACKOFF_SECONDS=$(sanitize_positive_int "$INITIAL_BACKOFF_SECONDS" 1)
MAX_BACKOFF_SECONDS=$(sanitize_positive_int "$MAX_BACKOFF_SECONDS" 300)
[ "$INITIAL_BACKOFF_SECONDS" -lt 1 ] && INITIAL_BACKOFF_SECONDS=1
[ "$MAX_BACKOFF_SECONDS" -lt "$INITIAL_BACKOFF_SECONDS" ] && MAX_BACKOFF_SECONDS="$INITIAL_BACKOFF_SECONDS"

ATTEMPTS_FILE="${STATE_DIR}/attempts"
FIRST_ATTEMPT_FILE="${STATE_DIR}/first-attempt"
LAST_ATTEMPT_FILE="${STATE_DIR}/last-attempt"
BOOT_ID_STATE_FILE="${STATE_DIR}/boot-id"
LAST_FINAL_ERROR_FILE="${STATE_DIR}/last-final-error"
CAPPED_SENTINEL="${STATE_DIR}/capped"
LOCK_FILE="${STATE_DIR}/.lock"

read_int_file() {
    local f="$1"
    local default="$2"
    if [ -r "$f" ]; then
        local v
        v=$(head -c 32 "$f" 2>/dev/null | tr -d '[:space:]')
        case "$v" in
            ''|*[!0-9]*) echo "$default" ;;
            *) echo "$v" ;;
        esac
    else
        echo "$default"
    fi
}

read_text_file() {
    local f="$1"
    local default="$2"
    if [ -r "$f" ]; then
        head -c 256 "$f" 2>/dev/null | tr -d '\n\r'
    else
        echo "$default"
    fi
}

compute_backoff() {
    # Loop-double from INITIAL_BACKOFF_SECONDS, n times, capped at MAX_BACKOFF_SECONDS.
    # Loop-based to avoid integer overflow on large attempts (2^63 etc.).
    local n="$1"
    local b="$INITIAL_BACKOFF_SECONDS"
    local i=0
    while [ "$i" -lt "$n" ]; do
        b=$((b * 2))
        if [ "$b" -ge "$MAX_BACKOFF_SECONDS" ]; then
            echo "$MAX_BACKOFF_SECONDS"
            return
        fi
        i=$((i + 1))
    done
    echo "$b"
}

emit_cap_event() {
    local attempts="$1"
    local elapsed="$2"
    local final_error="$3"
    local now_unix_ms
    now_unix_ms=$(date +%s%3N)
    local now_fmt
    now_fmt=$(date +"%F %T.%3N")

    mkdir -p "$EVENTS_DIR"

    local client_version="unknown"
    if [ -x "$BINARY" ]; then
        client_version=$("$BINARY" --version 2>/dev/null | head -1 | tr -d '\n\r' || echo "unknown")
        [ -z "$client_version" ] && client_version="unknown"
    fi

    # Build the inner Message JSON (a single string field per WALinuxAgent
    # schema). Keep all values as strings for parser robustness.
    local message
    if command -v jq >/dev/null 2>&1; then
        message=$(jq -nc \
            --arg status "Failure" \
            --arg reason "RetryCapReached" \
            --arg attempts "$attempts" \
            --arg elapsed "$elapsed" \
            --arg final "$final_error" \
            --arg maxAttempts "$MAX_ATTEMPTS" \
            --arg maxTotal "$MAX_TOTAL_SECONDS" \
            --arg client "$client_version" \
            '{Status:$status, Reason:$reason, Attempts:$attempts, ElapsedSeconds:$elapsed, FinalErrorType:$final, MaxAttempts:$maxAttempts, MaxTotalSeconds:$maxTotal, ClientVersion:$client}')
    else
        # Defensive fallback (jq is on the VHD, but if missing we still emit
        # a valid event so the cap signal isn't lost).
        message="{\"Status\":\"Failure\",\"Reason\":\"RetryCapReached\",\"Attempts\":\"$attempts\",\"ElapsedSeconds\":\"$elapsed\",\"FinalErrorType\":\"$final_error\",\"MaxAttempts\":\"$MAX_ATTEMPTS\",\"MaxTotalSeconds\":\"$MAX_TOTAL_SECONDS\",\"ClientVersion\":\"$client_version\"}"
    fi

    local event_json
    if command -v jq >/dev/null 2>&1; then
        event_json=$(jq -nc \
            --arg ts "$now_fmt" \
            --arg op "$now_fmt" \
            --arg ver "1.23" \
            --arg task "AKS.Bootstrap.SecureTLSBootstrapping.RetryCapReached" \
            --arg lvl "Error" \
            --arg msg "$message" \
            --arg pid "$$" \
            --arg tid "0" \
            '{Timestamp:$ts, OperationId:$op, Version:$ver, TaskName:$task, EventLevel:$lvl, Message:$msg, EventPid:$pid, EventTid:$tid}')
    else
        local esc_msg
        esc_msg=$(printf '%s' "$message" | sed 's/\\/\\\\/g; s/"/\\"/g')
        event_json="{\"Timestamp\":\"$now_fmt\",\"OperationId\":\"$now_fmt\",\"Version\":\"1.23\",\"TaskName\":\"AKS.Bootstrap.SecureTLSBootstrapping.RetryCapReached\",\"EventLevel\":\"Error\",\"Message\":\"$esc_msg\",\"EventPid\":\"$$\",\"EventTid\":\"0\"}"
    fi

    local event_file="${EVENTS_DIR}/${now_unix_ms}.json"
    umask 077
    printf '%s' "$event_json" > "$event_file"
    chmod 0600 "$event_file" 2>/dev/null || true
}

update_last_final_error() {
    # Best-effort: find the newest event JSON produced under EVENTS_DIR after
    # invocation start. Parse its Message field for FinalErrorType. Used to
    # propagate the binary's last failure reason into the cap event.
    local since_unix="$1"
    [ -d "$EVENTS_DIR" ] || return 0
    local newest
    newest=$(find "$EVENTS_DIR" -maxdepth 1 -type f -name '*.json' -newermt "@$since_unix" -printf '%T@ %p\n' 2>/dev/null \
        | sort -nr | head -1 | awk '{print $2}')
    [ -z "$newest" ] && return 0
    [ -r "$newest" ] || return 0

    local final=""
    if command -v jq >/dev/null 2>&1; then
        # Two layers: top-level Message is a string holding JSON.
        local inner
        inner=$(jq -r '.Message // ""' "$newest" 2>/dev/null)
        if [ -n "$inner" ]; then
            final=$(printf '%s' "$inner" | jq -r '.FinalErrorType // ""' 2>/dev/null)
        fi
    fi
    if [ -n "$final" ] && [ "$final" != "null" ]; then
        printf '%s' "$final" > "$LAST_FINAL_ERROR_FILE"
    fi
}

run_capped() {
    local current_boot
    current_boot=$(cat "$BOOT_ID_FILE" 2>/dev/null | tr -d '\n\r' || echo "")

    mkdir -p "$STATE_DIR"
    chmod 0700 "$STATE_DIR" 2>/dev/null || true

    # Boot-id reset: if we're on a different boot than the last persisted one,
    # this is a fresh recovery attempt — wipe state.
    if [ -f "$BOOT_ID_STATE_FILE" ]; then
        local persisted_boot
        persisted_boot=$(read_text_file "$BOOT_ID_STATE_FILE" "")
        if [ -n "$persisted_boot" ] && [ "$persisted_boot" != "$current_boot" ]; then
            rm -f "$ATTEMPTS_FILE" "$FIRST_ATTEMPT_FILE" "$LAST_ATTEMPT_FILE" \
                  "$LAST_FINAL_ERROR_FILE" "$CAPPED_SENTINEL" 2>/dev/null || true
        fi
    fi

    # If we've already capped in this boot, no-op (and exit 0 so systemd
    # marks the unit successful — important: this is what stops kubelet's
    # restart loop from continually re-triggering the unit).
    if [ -e "$CAPPED_SENTINEL" ]; then
        echo "secure-tls-bootstrap-retry-cap: cap already reached this boot; exiting 0" >&2
        return 0
    fi

    local now attempts first last
    now=$(date +%s)
    attempts=$(read_int_file "$ATTEMPTS_FILE" 0)
    first=$(read_int_file "$FIRST_ATTEMPT_FILE" 0)
    last=$(read_int_file "$LAST_ATTEMPT_FILE" 0)

    local elapsed=0
    [ "$first" -gt 0 ] && elapsed=$((now - first))
    [ "$elapsed" -lt 0 ] && elapsed=0

    if [ "$attempts" -ge "$MAX_ATTEMPTS" ] || \
       { [ "$first" -gt 0 ] && [ "$elapsed" -ge "$MAX_TOTAL_SECONDS" ]; }; then
        local final_error
        final_error=$(read_text_file "$LAST_FINAL_ERROR_FILE" "Unknown")
        echo "secure-tls-bootstrap-retry-cap: cap reached (attempts=$attempts/$MAX_ATTEMPTS, elapsed=${elapsed}s/${MAX_TOTAL_SECONDS}s, finalError=$final_error). Giving up." >&2
        emit_cap_event "$attempts" "$elapsed" "$final_error"
        : > "$CAPPED_SENTINEL"
        chmod 0600 "$CAPPED_SENTINEL" 2>/dev/null || true
        return 0
    fi

    # Backoff: sleep so we wait at least `backoff` between attempt N and N+1.
    local backoff sleep_for since_last
    backoff=$(compute_backoff "$attempts")
    if [ "$last" -gt 0 ]; then
        since_last=$((now - last))
        [ "$since_last" -lt 0 ] && since_last=0
        if [ "$since_last" -lt "$backoff" ]; then
            sleep_for=$((backoff - since_last))
            echo "secure-tls-bootstrap-retry-cap: backoff sleep ${sleep_for}s (attempt $((attempts + 1))/$MAX_ATTEMPTS, since_last=${since_last}s, target=${backoff}s)" >&2
            sleep "$sleep_for"
        fi
    fi

    # Persist updated counters before invoking the binary so a kill-mid-run
    # still counts as an attempt.
    now=$(date +%s)
    attempts=$((attempts + 1))
    printf '%s' "$attempts" > "$ATTEMPTS_FILE"
    printf '%s' "$now" > "$LAST_ATTEMPT_FILE"
    if [ "$first" -le 0 ]; then
        printf '%s' "$now" > "$FIRST_ATTEMPT_FILE"
    fi
    if [ -n "$current_boot" ]; then
        printf '%s' "$current_boot" > "$BOOT_ID_STATE_FILE"
    fi
    chmod 0600 "$ATTEMPTS_FILE" "$LAST_ATTEMPT_FILE" "$FIRST_ATTEMPT_FILE" \
                "$BOOT_ID_STATE_FILE" 2>/dev/null || true

    local invocation_start
    invocation_start=$(date +%s)

    # Run the binary as a subprocess (NOT exec) so we can post-process events.
    "$BINARY" "$@"
    local rc=$?

    update_last_final_error "$invocation_start"

    return $rc
}

main() {
    if [ ! -x "$BINARY" ]; then
        echo "secure-tls-bootstrap-retry-cap: binary not found or not executable at $BINARY" >&2
        # Exit non-zero so the systemd unit fails loudly during VHD validation.
        exit 127
    fi

    mkdir -p "$STATE_DIR"
    chmod 0700 "$STATE_DIR" 2>/dev/null || true

    # flock the entire wrapper body — systemd shouldn't run two ExecStart
    # instances concurrently, but this defends against manual systemctl
    # operations and unit-config changes. flock isn't available on every
    # platform (notably macOS in unit tests); when absent we proceed without
    # the lock — production targets (Ubuntu / Azure Linux) always have it
    # via util-linux.
    if command -v flock >/dev/null 2>&1; then
        exec 9>"$LOCK_FILE"
        if ! flock -n 9; then
            echo "secure-tls-bootstrap-retry-cap: another instance is running; exiting 0" >&2
            exit 0
        fi
    fi

    run_capped "$@"
    exit $?
}

# Only invoke main when executed directly. Sourcing (e.g. from ShellSpec) skips
# this so tests can exercise helpers in isolation.
if [ "${BASH_SOURCE[0]}" = "$0" ] || [ -z "${BASH_SOURCE[0]:-}" ]; then
    main "$@"
fi
