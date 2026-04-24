#!/bin/bash
set -uo pipefail

until [ "$(hostname)" = "$(cat /etc/hostname)" ]; do
   sleep 1
done

BIN_PATH="${BIN_PATH:-/opt/azure/containers/aks-node-controller}"
HOTFIX_BIN="${BIN_PATH}-hotfix"
HOTFIX_JSON="/opt/azure/containers/aks-node-controller-hotfix.json"
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

if [ -f "$HOTFIX_JSON" ]; then
    log "Downloading ANC hotfix from ${HOTFIX_JSON}"
    if "$BIN_PATH" download-hotfix; then
        log "Finished downloading ANC hotfix from ${HOTFIX_JSON}"
    else
        log "Failed to download ANC hotfix from ${HOTFIX_JSON}; falling back to staged binaries"
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
elif [ -f "$NBC_CMD_PATH" ]; then
    log "Launching aks-node-controller with nbc cmd ${NBC_CMD_PATH}"
    command+=("--nbc-cmd=$NBC_CMD_PATH")
else
    log "Gracefully exit aks-node-controller without provision config or nbc cmd"
    exit 0
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
