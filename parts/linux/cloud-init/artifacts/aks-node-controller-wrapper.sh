#!/bin/bash
set -uo pipefail

BIN_PATH="${BIN_PATH:-/opt/azure/containers/aks-node-controller}"
CONFIG_PATH="${CONFIG_PATH:-/opt/azure/containers/aks-node-controller-config.json}"
LOGGER_TAG="aks-node-controller-wrapper"

IMDS_CUSTOM_DATA_URL="http://169.254.169.254/metadata/instance/compute/customData?api-version=2021-02-01&format=text"
MAX_RETRIES=10
RETRY_INTERVAL=2

log() {
    local message="$1"
    # Emit to both journal (via logger) and stdout so systemd captures it.
    logger -t "$LOGGER_TAG" "$message"
    echo "$message"
}

# Fetch the aks-node-controller config JSON from IMDS custom data.
# The custom data is a base64-encoded cloud-config YAML containing a write_files
# entry with the config JSON (also base64-encoded).
# See: aks-node-controller/pkg/nodeconfigutils/utils.go for the template.
fetch_config_from_imds() {
    local custom_data_b64=""
    local attempt

    for attempt in $(seq 1 $MAX_RETRIES); do
        custom_data_b64=$(curl -sS -H "Metadata: true" "$IMDS_CUSTOM_DATA_URL" 2>/dev/null) && break
        log "IMDS fetch attempt $attempt/$MAX_RETRIES failed, retrying in ${RETRY_INTERVAL}s..."
        sleep $RETRY_INTERVAL
    done

    if [ -z "${custom_data_b64:-}" ]; then
        log "ERROR: Failed to fetch custom data from IMDS after $MAX_RETRIES attempts"
        return 1
    fi

    # Decode the outer base64 to get the cloud-config YAML, then extract
    # the inner base64 blob (the config JSON) from the write_files entry.
    # The cloud-config format is:
    #   #cloud-config
    #   write_files:
    #   - path: /opt/azure/containers/aks-node-controller-config.json
    #     ...
    #     content: !!binary |
    #      <base64-encoded-config-json>
    local decoded
    decoded=$(echo "$custom_data_b64" | base64 -d 2>/dev/null)
    if [ $? -ne 0 ]; then
        log "ERROR: Failed to base64-decode IMDS custom data"
        return 1
    fi

    # Extract the base64 content block after the "!!binary |" line.
    # The content is indented with spaces under the write_files entry.
    local config_b64
    config_b64=$(echo "$decoded" | sed -n '/!!binary |/,$ p' | tail -n +2 | tr -d ' ')
    if [ -z "$config_b64" ]; then
        log "ERROR: Could not extract config JSON from cloud-config"
        return 1
    fi

    echo "$config_b64" | base64 -d > "$CONFIG_PATH"
    if [ $? -ne 0 ]; then
        log "ERROR: Failed to decode config JSON"
        return 1
    fi

    log "Successfully fetched config from IMDS and wrote to $CONFIG_PATH"
    return 0
}

# this is to ensure that shellspec won't interpret any further lines below
${__SOURCED__:+return}

# If the config file already exists (cloud-init wrote it), use it directly.
if [ -f "$CONFIG_PATH" ]; then
    log "Config found at $CONFIG_PATH"
else
    log "Config not found at $CONFIG_PATH, fetching from IMDS..."
    if ! fetch_config_from_imds; then
        # Neither IMDS nor config file available.
        # This is expected during VHD bake (packer) where there is no
        # custom data. Exit 0 so the service doesn't fail the build.
        log "IMDS fetch failed and no config file present, nothing to provision (packer build?). Exiting."
        exit 0
    fi
fi

log "Launching aks-node-controller with config ${CONFIG_PATH}"
"$BIN_PATH" provision --provision-config="$CONFIG_PATH" &
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
