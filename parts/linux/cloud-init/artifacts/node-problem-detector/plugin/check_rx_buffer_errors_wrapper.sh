#!/bin/bash

# This is a wrapper script for the check_rx_buffer_errors.sh script
# It executes the check_rx_buffer_errors.sh script with a shorter timeout than NPD
# and manages exit codes appropriately.

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
MAIN_SCRIPT_PATH="${SCRIPT_DIR}/check_rx_buffer_errors.sh"
JSON_CONFIG_PATH="${SCRIPT_DIR}/../custom-plugin-monitor/custom-rx-buffer-errors-monitor.json"

# Default timeout value in case of any issues reading monitor
DEFAULT_TIMEOUT_SECONDS=25

if [ ! -x "$MAIN_SCRIPT_PATH" ]; then
    echo "ERROR: Check script not found or not executable: $MAIN_SCRIPT_PATH" >&2
    exit 0  # Still exit with 0 to avoid NPD errors
fi

if [ ! -f "$JSON_CONFIG_PATH" ]; then
    echo "Warning: JSON config not found: $JSON_CONFIG_PATH. Using default timeout of $DEFAULT_TIMEOUT_SECONDS." >&2
else
    # Check if jq is installed
    if ! command -v jq &> /dev/null; then
        echo "Warning: jq is not installed. Cannot parse JSON config. Using default timeout of $DEFAULT_TIMEOUT_SECONDS." >&2
    else
        NPD_TIMEOUT_STRING=$(jq -r '.pluginConfig.timeout' "$JSON_CONFIG_PATH")

        if [ -z "$NPD_TIMEOUT_STRING" ] || [ "$NPD_TIMEOUT_STRING" == "null" ]; then
            echo "Warning: Could not read timeout from $JSON_CONFIG_PATH. Using default timeout of $DEFAULT_TIMEOUT_SECONDS" >&2
        else
            # Remove 's' suffix and convert to integer
            NPD_TIMEOUT_SECONDS=${NPD_TIMEOUT_STRING%s}
            if ! [[ "$NPD_TIMEOUT_SECONDS" =~ ^[0-9]+$ ]]; then
                echo "Warning: Invalid timeout value '$NPD_TIMEOUT_STRING' in $JSON_CONFIG_PATH. Using default timeout of $DEFAULT_TIMEOUT_SECONDS." >&2
            fi
        fi
    fi
fi

# If NPD_TIMEOUT_SECONDS is not set, use the default timeout
if [ -z "$NPD_TIMEOUT_SECONDS" ]; then
    SCRIPT_TIMEOUT_SECONDS=$DEFAULT_TIMEOUT_SECONDS
else
    # Subtract 5 seconds from NPD timeout for the actual script timeout
    SCRIPT_TIMEOUT_SECONDS=$((NPD_TIMEOUT_SECONDS - 5))
fi

echo "Running check_rx_buffer_errors.sh with timeout of ${SCRIPT_TIMEOUT_SECONDS}s (NPD timeout: ${NPD_TIMEOUT_SECONDS}s)"

# Run with shorter timeout than the NPD timeout
timeout "${SCRIPT_TIMEOUT_SECONDS}s" "$MAIN_SCRIPT_PATH"
RETURN_CODE=$?

# Check for timeout - return code 124 indicates a timeout
if [ "$RETURN_CODE" -eq 124 ]; then
    echo "check_rx_buffer_errors.sh timed out (return code 124) after ${SCRIPT_TIMEOUT_SECONDS}s. Still exiting with 0." >&2
    exit 0
fi

echo "check_rx_buffer_errors.sh completed with return code $RETURN_CODE" >&2
# Always exit with 0 to avoid NPD events
exit 0