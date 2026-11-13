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
CLOUD_SETTINGS_PATH="${CLOUD_SETTINGS_PATH:-/opt/azure/containers/init-aks-cloud.env}"
INIT_AKS_CLOUD_PATH="${INIT_AKS_CLOUD_PATH:-/opt/azure/containers/init-aks-cloud.sh}"
CLOUD_INIT_LOG="${CLOUD_INIT_LOG:-/var/log/azure/cluster-provision.log}"
CLOUD_INIT_DONE="${CLOUD_INIT_DONE:-/run/aks/init-aks-cloud.done}"
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

skip_hotfix_acquisition=false
settings_ok=true
rm -f "$CLOUD_INIT_DONE" || settings_ok=false
if [ -s "$CONFIG_PATH" ]; then
    # AKSNodeConfig is the live source of cloud settings; jq's @sh safely quotes values.
    jq -er '
        def field($snake; $camel):
            if has($snake) and has($camel) then error("duplicate field " + $snake)
            else .[$snake] // .[$camel] end;
        def safe_string:
            if type != "string" then error("invalid cloud settings")
            elif (explode | index(0)) != null then error("NUL in cloud settings")
            else . end;
        (field("cluster_config"; "clusterConfig") // {}) as $cluster
        | (field("custom_cloud_config"; "customCloudConfig") // {}) as $cloud
        | ($cluster.location // "" | safe_string) as $location
        | ($cloud | field("custom_cloud_env_name"; "customCloudEnvName") // "" | safe_string) as $name
        | (if ($name | ascii_downcase) == "akscustom" then
               ($cloud | field("repo_depot_endpoint"; "repoDepotEndpoint") // "" | safe_string)
           else "" end) as $endpoint
        | if $location == "" then error("missing location")
          else "LOCATION=\($location | @sh)",
               "REPO_DEPOT_ENDPOINT=\($endpoint | @sh)" end
    ' "$CONFIG_PATH" > "$CLOUD_SETTINGS_PATH" 2>> "$CLOUD_INIT_LOG" || settings_ok=false
fi
if [ -f "$CLOUD_SETTINGS_PATH" ]; then
    # shellcheck source=/dev/null
    if [ "$settings_ok" = true ] && [ -x "$INIT_AKS_CLOUD_PATH" ] && [ -s "$CLOUD_SETTINGS_PATH" ] &&
        ( set -a; source "$CLOUD_SETTINGS_PATH" && rm -f "$CLOUD_SETTINGS_PATH" && [ -n "${LOCATION:-}" ] &&
          [ "${REPO_DEPOT_ENDPOINT+x}" = x ] && ANC_EARLY_CLOUD_INIT=true "$INIT_AKS_CLOUD_PATH" ) >> "$CLOUD_INIT_LOG" 2>&1 &&
        mkdir -p "$(dirname "$CLOUD_INIT_DONE")" && touch "$CLOUD_INIT_DONE"; then
        log "Early cloud initialization completed"
    else
        rm -f "$CLOUD_SETTINGS_PATH" || log "Failed to remove cloud settings"
        # Do not download with incomplete trust; let normal provisioning retry initialization.
        skip_hotfix_acquisition=true
        log "Early cloud initialization failed; skipping hotfix acquisition and retrying during provisioning"
    fi
elif [ "$settings_ok" != true ]; then
    skip_hotfix_acquisition=true
    log "Early cloud initialization failed; skipping hotfix acquisition and retrying during provisioning"
fi

# Older payloads without cloud settings or AKSNodeConfig retain their existing hotfix flow.
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
