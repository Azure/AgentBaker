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

# check-hotfix reads the hotfix pointer from the LPS endpoint (IMDS-attested) and refreshes
# the on-disk hotfix pointer file (its own default path, which $HOTFIX_JSON mirrors) that the
# download-hotfix block below consumes, so it must run first.
# Gated default-off behind ENABLE_PROVISIONING_HOTFIX so existing VHDs behave exactly as
# before; only the literal string "true" enables it. This env var is the on-node terminal
# of the EnableProvisioningHotfix aks-rp region toggle (toggle -> absvc -> ANC), so regions
# where the toggle is off see no behavior change. check-hotfix is designed to be fail-open
# (its own error paths exit 0), but an older ANC binary that predates the subcommand exits
# non-zero, so we still wrap the call defensively to guarantee it can never block provisioning.
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
