#!/bin/bash
set -uo pipefail

until [ "$(hostname)" = "$(cat /etc/hostname)" ]; do
   sleep 1
done

BIN_PATH="${BIN_PATH:-/opt/azure/containers/aks-node-controller}"
HOTFIX_BIN="${BIN_PATH}-hotfix"
# HOTFIX_JSON is only used by this wrapper for the -f gate/logs below. The check-hotfix and
# download-hotfix subcommands read/write their own internal default path and do NOT consume
# this variable, so overriding it does not change binary behavior (it exists mainly so
# shellspec can exercise the download-hotfix branch). Keep it aligned with the binary default.
HOTFIX_JSON="${HOTFIX_JSON:-/opt/azure/containers/aks-node-controller-hotfix.json}"
CONFIG_PATH="${CONFIG_PATH:-/opt/azure/containers/aks-node-controller-config.json}"
NBC_CMD_PATH="${NBC_CMD_PATH:-/opt/azure/containers/aks-node-controller-nbc-cmd.sh}"
# FEATURES_PATH is an optional KEY=VALUE feature-flag file and the on-node delivery channel for
# flags like ENABLE_PROVISIONING_HOTFIX (there is no systemd environment-variable delivery).
# Writer: the cloud-init boothook (producer side, PR #8717), running as root at provision time,
# writes it ONLY when the corresponding aks-rp toggle is on. It lands under /opt/azure/containers
# (0644, root-owned) like the other provisioning artifacts, so only root can populate it and the
# producer is the sole trusted writer. Parsed below at wrapper runtime; absent file (default-off,
# or an older VHD without the producer) is a no-op.
FEATURES_PATH="${FEATURES_PATH:-/opt/azure/containers/enabled_features.sh}"
LOGGER_TAG="aks-node-controller-wrapper"

log() {
    local message="$1"
    # Emit to both journal (via logger) and stdout so systemd captures it.
    logger -t "$LOGGER_TAG" "$message"
    echo "$message"
}

# this is to ensure that shellspec won't interpret any further lines below
${__SOURCED__:+return}

if [ ! -f "$CONFIG_PATH" ] && [ ! -f "$NBC_CMD_PATH" ]; then
    log "Gracefully exit aks-node-controller without provision config or nbc cmd"
    exit 0
fi

# Read the optional feature-flag file if present. The boothook writes it (KEY=VALUE lines only)
# at provision time BEFORE this wrapper runs, so reading it here rests on the same
# write-before-read ordering that config delivery already relies on - no systemd env-passing or
# boot-ordering assumption. Absent file (default-off, or an older VHD) is a no-op, preserving
# today's behavior exactly. We PARSE KEY=VALUE lines rather than sourcing the file, so a malformed
# file can never execute arbitrary shell or exit the wrapper (fail-open). The file is fully
# controlled by the producer, so any valid identifier=value is accepted (not a fixed key list);
# blank lines, comments, and non-identifier keys are skipped. The "|| [ -n "$_key" ]" guard
# ensures the final line is still parsed even if the file has no trailing newline (read returns
# non-zero at EOF but still populates the variables).
if [ -f "$FEATURES_PATH" ]; then
    log "Reading feature flags from ${FEATURES_PATH}"
    while IFS='=' read -r _key _val || [ -n "$_key" ]; do
        case "$_key" in
        ''|\#*) continue ;;
        [!a-zA-Z_]*|*[!a-zA-Z0-9_]*) continue ;;
        esac
        export "${_key}=${_val}"
    done <"$FEATURES_PATH"
fi

# check-hotfix refreshes the on-disk hotfix pointer (its own default path, mirrored by
# $HOTFIX_JSON) that download-hotfix reads below, so it must run first. Gated default-off
# behind ENABLE_PROVISIONING_HOTFIX (only the literal "true" enables it) - the on-node
# terminal of the EnableProvisioningHotfix aks-rp region toggle. Wrapped defensively: it is
# fail-open, but an older ANC binary predating the subcommand exits non-zero.
if [ "${ENABLE_PROVISIONING_HOTFIX:-}" = "true" ]; then
    log "ENABLE_PROVISIONING_HOTFIX=true; running check-hotfix to refresh hotfix pointer"
    if "$BIN_PATH" check-hotfix; then
        log "ANC check-hotfix completed; hotfix pointer refresh attempted"
    else
        log "ANC check-hotfix failed; continuing (fail-open)"
    fi
fi

if [ -f "$HOTFIX_JSON" ]; then
    log "Found ANC hotfix config at ${HOTFIX_JSON}; running download-hotfix"
    if "$BIN_PATH" download-hotfix; then
        log "ANC download-hotfix completed; binary selection follows"
    else
        log "ANC download-hotfix failed; binary selection follows"
    fi
fi

if [ -x "$HOTFIX_BIN" ]; then
    BIN_PATH="$HOTFIX_BIN"
    log "Using hotfix binary: $HOTFIX_BIN"
else
    log "Using VHD-baked binary: $BIN_PATH"
fi

command=("$BIN_PATH" provision)
if [ -f "$CONFIG_PATH" ]; then
    log "Launching aks-node-controller with config ${CONFIG_PATH}"
    command+=("--provision-config=$CONFIG_PATH")
fi
if [ -f "$NBC_CMD_PATH" ]; then
    log "Launching aks-node-controller with nbc cmd ${NBC_CMD_PATH}"
    command+=("--nbc-cmd=$NBC_CMD_PATH")
fi

"${command[@]}" &
child_pid=$!
log "Spawned aks-node-controller (pid ${child_pid})"

wait "$child_pid"
exit_code=$?

if [ "$exit_code" -eq 0 ]; then
    log "aks-node-controller completed successfully"
else
    log "aks-node-controller exited with code ${exit_code}"
fi

exit $exit_code
