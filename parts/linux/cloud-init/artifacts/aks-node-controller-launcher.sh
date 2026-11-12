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
CLOUD_SETTINGS_PATH="${CLOUD_SETTINGS_PATH:-/opt/azure/containers/aks-node-controller-cloud-settings.sh}"
INIT_AKS_CLOUD_PATH="${INIT_AKS_CLOUD_PATH:-/opt/azure/containers/init-aks-cloud.sh}"
CLOUD_INIT_LOG="${CLOUD_INIT_LOG:-/var/log/azure/cluster-provision.log}"
LOGGER_TAG="aks-node-controller-launcher"

log() {
    local message="$1"
    # Emit to both journal (via logger) and stdout so systemd captures it.
    logger -t "$LOGGER_TAG" "$message"
    echo "$message"
}

# Keep the ordering-sensitive hotfix workflow in a separately callable script so tests can cover
# the sequence without also running the rest of the launcher/provision wrapper.
# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/aks-node-controller-hotfix.sh"

# this is to ensure that shellspec won't interpret any further lines below
${__SOURCED__:+return}

if [ ! -f "$CONFIG_PATH" ] && [ ! -f "$NBC_CMD_PATH" ]; then
    log "Gracefully exit aks-node-controller without provision config or nbc cmd"
    exit 0
fi

unset ANC_CLOUD_INIT_COMPLETE
skip_hotfix_acquisition=false
if [ -f "$CLOUD_SETTINGS_PATH" ]; then
    settings_ok=true
    # shellcheck source=/dev/null
    source "$CLOUD_SETTINGS_PATH" || settings_ok=false
    # These settings are node-specific; never leave them behind for a PIS image to bake in.
    rm -f "$CLOUD_SETTINGS_PATH" || settings_ok=false
    # Prepare trust and RepoDepot before ANC downloads its hotfix. Mark success only
    # in this process so ANC can skip the duplicate initializer during provisioning.
    if [ "$settings_ok" = true ] && [ -f "$INIT_AKS_CLOUD_PATH" ] &&
        ANC_EARLY_CLOUD_INIT=true LOCATION="$LOCATION" REPO_DEPOT_ENDPOINT="$REPO_DEPOT_ENDPOINT" "$INIT_AKS_CLOUD_PATH" >> "$CLOUD_INIT_LOG" 2>&1; then
        export ANC_CLOUD_INIT_COMPLETE=true
    else
        # Do not download with incomplete trust; let normal provisioning retry initialization.
        skip_hotfix_acquisition=true
        log "Early cloud initialization failed; skipping hotfix acquisition and retrying during provisioning"
    fi
fi

# Older VHDs lack the generated settings file and retain their existing hotfix flow.
anc_run_hotfix_flow "$BIN_PATH" "$HOTFIX_BIN" "$HOTFIX_JSON" "$FEATURES_PATH" "$skip_hotfix_acquisition"
BIN_PATH="$ANC_HOTFIX_SELECTED_BIN"

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
